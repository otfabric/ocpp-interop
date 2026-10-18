# SPDX-License-Identifier: MIT
"""One OCPP connection, driven by the upstream ChargePoint.

The Mobility House package has one class for both ends of a connection,
ocpp.v16.ChargePoint (ocpp.v201.ChargePoint for OCPP 2.0.1, ocpp.v21.ChargePoint
for 2.1): it routes
received CALLs to handlers, answers them, and sends CALLs with
ChargePoint.call(). The adapter subclasses it only to

- register a handler for every action the fixture answers, in the upstream's
  own route map (what its @on() decorator produces);
- observe the frames the upstream sends and receives, for the events;
- remember the raw response of a CALL, so that a CALLERROR can be told apart
  from a response the upstream rejects.

Raw JSON payloads become upstream types with the upstream's own helpers
(camel_to_snake_case, then the dataclasses of ocpp.v16.call and
ocpp.v16.call_result, or of ocpp.v201), and go back to JSON the way the
upstream serialises them (serialize_as_dict, remove_nones,
snake_to_camel_case). Validation against the OCPP JSON schemas, framing and
routing are the upstream's.
"""

from __future__ import annotations

import asyncio
import json
import time
import uuid
from dataclasses import is_dataclass

from ocpp.charge_point import camel_to_snake_case, remove_nones, serialize_as_dict, snake_to_camel_case
from ocpp.exceptions import OCPPError
from ocpp.messages import CallError, MessageType, unpack
from ocpp.v16 import ChargePoint as ChargePoint16
from ocpp.v16.enums import Action as Action16
from ocpp.v201 import ChargePoint as ChargePoint201
from ocpp.v201.enums import Action as Action201
from ocpp.v21 import ChargePoint as ChargePoint21
from ocpp.v21.enums import Action as Action21

from . import output
from .fixture import CallErrorAnswer

CHARGE_POINTS = {"1.6": ChargePoint16, "2.0.1": ChargePoint201, "2.1": ChargePoint21}
ACTIONS = {"1.6": {a.value for a in Action16}, "2.0.1": {a.value for a in Action201},
           "2.1": {a.value for a in Action21}}


def error_class(code: str):
    """The upstream exception class that the upstream turns into a CALLERROR
    with this error code, or None."""
    for cls in OCPPError.__subclasses__():
        if getattr(cls, "code", None) == code:
            return cls
    return None


def to_json(dataclass_instance) -> dict:
    """An upstream payload type as the upstream serialises it on the wire."""
    return snake_to_camel_case(remove_nones(serialize_as_dict(dataclass_instance)))


def request_type(version: str, action: str):
    """The upstream's request class of an action, or None."""
    if action not in ACTIONS[version]:
        return None
    cls = getattr(CHARGE_POINTS[version]._call, action, None)  # pylint: disable=protected-access
    return cls if isinstance(cls, type) and is_dataclass(cls) else None


def make_endpoint_class(version: str):
    base = CHARGE_POINTS[version]

    class Endpoint(base):
        """A ChargePoint that answers from the fixture and reports events."""

        def __init__(self, station_id, connection, fixture, role, emit=True):
            super().__init__(station_id, connection)
            self.version = version
            self.fixture = fixture
            self.role = role  # the answering role: "csms" (server) or "station" (client)
            self.emit = emit
            self.received = {}  # messageId -> (action, payload) of CALLs received
            self.sent = {}  # messageId -> (action, payload) of CALLs sent
            self.responses = {}  # messageId -> raw CallResult or CallError
            self.closed = asyncio.get_running_loop().create_future()
            for action in fixture.answers(role):
                self.route_map[action] = {
                    "_on_action": self._handler(action),
                    "_skip_schema_validation": False,
                }

        def _event(self, name, **fields):
            if self.emit:
                output.event(name, self.id, **fields)

        def _handler(self, action):
            async def handler(call_unique_id, **_payload):
                _, request = self.received.get(call_unique_id, (action, {}))
                answer = self.fixture.answer(self.role, action, request)
                if isinstance(answer, CallErrorAnswer):
                    # The upstream answers an OCPPError with a CALLERROR of its code.
                    raise error_class(answer.code)(description=answer.description)
                return getattr(self._call_result, action)(**camel_to_snake_case(answer))

            return handler

        # ---- observation of the upstream's traffic ----

        async def route_message(self, raw_msg):
            self._observe_incoming(raw_msg)
            await super().route_message(raw_msg)

        def _observe_incoming(self, raw):
            try:
                msg = unpack(raw)
            except OCPPError as e:
                self._event("protocol-error", detail=f"{e.__class__.__name__}: {e.description}", frame=str(raw)[:4096])
                return
            if msg.message_type_id == MessageType.Call:
                self.received[msg.unique_id] = (msg.action, msg.payload)
                self._event("call-received", messageId=msg.unique_id, action=msg.action, payload=msg.payload)
            elif msg.message_type_id == MessageType.CallResult:
                action = self.sent.get(msg.unique_id, (None, None))[0]
                self._event("call-result", messageId=msg.unique_id, action=action, payload=msg.payload)
            elif msg.message_type_id == MessageType.CallError:
                action = self.sent.get(msg.unique_id, (None, None))[0]
                self._event("call-result", messageId=msg.unique_id, action=action, error={
                    "code": msg.error_code, "description": msg.error_description, "details": msg.error_details})

        async def _send(self, message):
            # Reported before the frame is handed over, so that the event
            # precedes the peer's answer in the stream.
            self._observe_outgoing(message)
            await super()._send(message)

        def _observe_outgoing(self, message):
            frame = json.loads(message)
            kind, unique_id = frame[0], frame[1]
            if kind == MessageType.Call:
                self.sent[unique_id] = (frame[2], frame[3])
                self._event("call-sent", messageId=unique_id, action=frame[2], payload=frame[3])
            elif kind == MessageType.CallResult:
                action = self.received.pop(unique_id, (None, None))[0]
                self._event("call-answered", messageId=unique_id, action=action, payload=frame[2])
            elif kind == MessageType.CallError:
                action = self.received.pop(unique_id, (None, None))[0]
                self._event("call-answered", messageId=unique_id, action=action, error={
                    "code": frame[2], "description": frame[3], "details": frame[4]})

        async def _get_specific_response(self, unique_id, timeout):
            response = await super()._get_specific_response(unique_id, timeout)
            self.responses[response.unique_id] = response
            return response

        # ---- sending a CALL for the contract ----

        async def execute(self, action: str, payload: dict, timeout_s: float) -> dict:
            """Send one CALL with the upstream and return a result document."""
            started = time.monotonic()

            def done(**kwargs):
                return output.result(self.version, action, elapsed_ms=(time.monotonic() - started) * 1000, **kwargs)

            cls = request_type(self.version, action)
            if cls is None:
                return done(ok=False, request=payload, error_code="unsupported",
                            error_message=f"ocpp {self.version} has no request type for {action}")
            try:
                request = cls(**camel_to_snake_case(payload))
            except TypeError as e:
                return done(ok=False, request=payload, error_code="invalid-request", error_message=str(e))

            unique_id = str(uuid.uuid4())
            self._response_timeout = timeout_s
            task = asyncio.ensure_future(self.call(request, suppress=False, unique_id=unique_id))
            finished, _ = await asyncio.wait({task, self.closed}, timeout=timeout_s + 1,
                                             return_when=asyncio.FIRST_COMPLETED)
            sent = self.sent.get(unique_id, (None, payload))[1]
            if task not in finished:
                task.cancel()
                if self.closed.done():
                    return done(ok=False, request=sent, message_id=unique_id, error_code="connection-lost",
                                error_message="the connection closed while waiting for the response")
                return done(ok=False, request=sent, message_id=unique_id, error_code="timeout",
                            error_message=f"no response within {int(timeout_s * 1000)} ms")
            error = task.exception()
            if error is None:
                return done(ok=True, request=sent, message_id=unique_id, response=to_json(task.result()))
            raw = self.responses.pop(unique_id, None)
            if isinstance(raw, CallError):
                return done(ok=False, request=sent, message_id=unique_id,
                            call_error={"code": raw.error_code, "description": raw.error_description,
                                        "details": raw.error_details},
                            error_code="call-error", error_message=f"CALLERROR {raw.error_code}")
            if unique_id not in self.sent:
                # The upstream validates the request against the OCPP schema
                # before it sends anything.
                return done(ok=False, request=payload, error_code="invalid-request",
                            error_message=f"{error.__class__.__name__}: {error}")
            if isinstance(error, asyncio.TimeoutError):
                return done(ok=False, request=sent, message_id=unique_id, error_code="timeout",
                            error_message=f"no response within {int(timeout_s * 1000)} ms")
            if raw is not None:
                return done(ok=False, request=sent, message_id=unique_id, error_code="invalid-response",
                            error_message=f"{error.__class__.__name__}: {error}")
            return done(ok=False, request=sent, message_id=unique_id, error_code="failed",
                        error_message=f"{error.__class__.__name__}: {error}")

    return Endpoint
