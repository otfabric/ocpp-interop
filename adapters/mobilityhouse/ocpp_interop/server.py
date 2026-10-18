# SPDX-License-Identifier: MIT
"""`server`: a CSMS. The upstream ChargePoint, one per station, over a
websockets server, which is the transport the upstream documents and tests
with (the ocpp package itself has no transport)."""

from __future__ import annotations

import asyncio
import base64
import binascii
import os
import signal
import sys
from http import HTTPStatus
from urllib.parse import unquote, urlsplit

from websockets.asyncio.server import serve
from websockets.exceptions import ConnectionClosed
from websockets.typing import Subprotocol

from . import ADAPTER, control, output
from .endpoint import make_endpoint_class
from .tls import compression_negotiated, peer_common_name

SUBPROTOCOLS = {"1.6": "ocpp1.6", "2.0.1": "ocpp2.0.1", "2.1": "ocpp2.1"}


def station_of(path: str) -> str:
    """The station identity: the last segment of the request path."""
    return unquote(urlsplit(path).path.rstrip("/").rsplit("/", 1)[-1])


def remote_of(connection) -> str | None:
    address = connection.remote_address
    return f"{address[0]}:{address[1]}" if address else None


def credentials(header: str | None):
    """(username, password) of a Basic Authorization header, or None."""
    if not header or not header.lower().startswith("basic "):
        return None
    try:
        user, _, password = base64.b64decode(header[6:].strip(), validate=True).decode().partition(":")
    except (binascii.Error, UnicodeDecodeError):
        return None
    return user, password


async def run(version, fixture, bind_address, port, security_profile, ssl_context, ready_file,
              control_path) -> int:
    endpoint_class = make_endpoint_class(version)
    subprotocol = SUBPROTOCOLS[version]
    sessions = {}
    stop = asyncio.Event()

    def process_request(connection, request):
        station = station_of(request.path)
        if station not in fixture.stations:
            output.event("connection-rejected", station, reason="unknown-station", remote=remote_of(connection))
            return connection.respond(HTTPStatus.NOT_FOUND, "Unknown station\n")
        if security_profile == 3 and peer_common_name(connection) != station:
            # The handshake verified the certificate against ca.crt; its
            # subject must name this station.
            output.event("connection-rejected", station, reason="authentication", remote=remote_of(connection))
            return connection.respond(HTTPStatus.UNAUTHORIZED, "Unauthorized\n")
        if security_profile in (1, 2):
            given = credentials(request.headers.get("Authorization"))
            if given != (station, fixture.stations[station]["password"]):
                output.event("connection-rejected", station, reason="authentication", remote=remote_of(connection))
                response = connection.respond(HTTPStatus.UNAUTHORIZED, "Unauthorized\n")
                response.headers["WWW-Authenticate"] = 'Basic realm="ocpp-interop"'
                return response
        offered = [p.strip() for value in request.headers.get_all("Sec-WebSocket-Protocol") for p in value.split(",")]
        if subprotocol not in offered:
            # websockets itself refuses the handshake (HTTP 400): it was
            # started with this one subprotocol.
            output.event("connection-rejected", station, reason="subprotocol", remote=remote_of(connection),
                         offered=offered)
        return None

    async def handler(connection):
        station = station_of(connection.request.path)
        endpoint = endpoint_class(station, connection, fixture, "csms")
        sessions[station] = endpoint
        output.event("connection-opened", station, subprotocol=connection.subprotocol, remote=remote_of(connection),
                     compression=compression_negotiated(connection))
        try:
            await endpoint.start()
        except ConnectionClosed:
            pass
        finally:
            if not endpoint.closed.done():
                endpoint.closed.set_result(True)
            if sessions.get(station) is endpoint:
                del sessions[station]
            output.event("connection-closed", station, code=connection.close_code, reason=connection.close_reason)

    def find(station):
        return sessions.get(station) if station else None

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, stop.set)

    try:
        # compression="deflate" (websockets' default, explicit here): permessage-deflate
        # is accepted whenever a station offers it.
        server = await serve(handler, bind_address, port, subprotocols=[Subprotocol(subprotocol)],
                             process_request=process_request, server_header=f"ocpp-interop-{ADAPTER}",
                             ssl=ssl_context, compression="deflate")
    except OSError as e:
        print(f"server: cannot listen on {bind_address}:{port}: {e}", file=sys.stderr)
        return 3
    try:
        control_server = await control.serve(control_path, version, find)
    except OSError as e:
        print(f"server: cannot open the control socket {control_path}: {e}", file=sys.stderr)
        server.close()
        return 3

    with open(ready_file, "w", encoding="utf-8") as handle:
        handle.write("ready\n")
    output.event("ready", adapter=ADAPTER, version=version, port=port, fixture=fixture.name,
                 securityProfile=security_profile)
    await stop.wait()

    try:
        os.unlink(ready_file)
    except OSError:
        pass
    control_server.close()
    server.close()
    try:
        await asyncio.wait_for(server.wait_closed(), 10)
    except asyncio.TimeoutError:
        pass
    # Let the connection handlers report their close.
    await asyncio.sleep(0)
    output.event("stopped")
    return 0
