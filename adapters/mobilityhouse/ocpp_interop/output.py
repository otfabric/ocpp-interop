# SPDX-License-Identifier: MIT
"""JSON on stdout: events (JSON Lines) and result documents."""

from __future__ import annotations

import decimal
import json
import sys
from datetime import datetime, timezone

from . import ADAPTER, SCHEMA_VERSION


class _Encoder(json.JSONEncoder):
    """The upstream turns some numbers into decimal.Decimal while validating."""

    def default(self, o):
        if isinstance(o, decimal.Decimal):
            return int(o) if o == o.to_integral_value() else float(o)
        return super().default(o)


def dumps(value) -> str:
    return json.dumps(value, separators=(",", ":"), cls=_Encoder, ensure_ascii=False)


def timestamp() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="milliseconds").replace("+00:00", "Z")


def write_line(document: dict) -> None:
    sys.stdout.write(dumps(document) + "\n")
    sys.stdout.flush()


def event(name: str, station: str | None = None, **fields) -> None:
    document = {"event": name, "time": timestamp()}
    if station is not None:
        document["station"] = station
    document.update(fields)
    write_line(document)


def result(version: str, operation: str, *, ok: bool, request=None, response=None, message_id=None,
           call_error=None, error_code=None, error_message="", elapsed_ms=0) -> dict:
    """A result document (schemas/client-result.schema.json)."""
    document = {
        "schemaVersion": SCHEMA_VERSION,
        "adapter": ADAPTER,
        "version": version,
        "operation": operation,
        "ok": ok,
    }
    if message_id is not None:
        document["messageId"] = message_id
    if request is not None:
        document["request"] = request
    document.update({
        "response": response,
        "callError": call_error,
        "error": None if error_code is None else {"code": error_code, "message": error_message},
        "elapsedMs": max(0, int(elapsed_ms)),
    })
    return document


EXIT_CODES = {
    None: 0,
    "connect-failed": 3,
    "unsupported": 5,
}


def exit_code(document: dict) -> int:
    """The exit code that goes with a result document."""
    error = document.get("error")
    code = error.get("code") if error else None
    return EXIT_CODES.get(code, 1)
