# SPDX-License-Identifier: MIT
"""The control socket (docs/CONTAINER_CONTRACT.md, `call`).

A running `server` or `client run` listens on a Unix socket inside the
container. `ocpp-interop call` connects, writes one request line and reads
one result document line:

    {"station": "ID" | null, "action": "Reset", "payload": {...}, "timeoutMs": 10000}
"""

from __future__ import annotations

import asyncio
import json
import os
import socket
import sys

from . import output

MAX_LINE = 16 * 1024 * 1024


async def serve(path: str, version: str, find_endpoint) -> asyncio.AbstractServer:
    """Serve call requests. find_endpoint(station) returns the endpoint to
    send from, or None when there is no open connection for it."""

    async def handle(reader: asyncio.StreamReader, writer: asyncio.StreamWriter):
        try:
            line = await reader.readline()
            request = json.loads(line)
            action = request["action"]
            payload = request.get("payload") or {}
            timeout_s = int(request.get("timeoutMs", 10000)) / 1000
            endpoint = find_endpoint(request.get("station"))
            if endpoint is None:
                document = output.result(version, action, ok=False, request=payload, error_code="not-connected",
                                         error_message=f"no open connection for station {request.get('station')!r}")
            else:
                document = await endpoint.execute(action, payload, timeout_s)
            writer.write((output.dumps(document) + "\n").encode())
            await writer.drain()
        except Exception as e:  # pylint: disable=broad-except
            print(f"control socket: {e!r}", file=sys.stderr)
        finally:
            writer.close()

    if os.path.exists(path):
        os.unlink(path)
    os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
    return await asyncio.start_unix_server(handle, path=path, limit=MAX_LINE)


def call(path: str, station: str | None, action: str, payload: dict, timeout_ms: int) -> int:
    """The `call` command: one request over the control socket."""
    request = {"station": station, "action": action, "payload": payload, "timeoutMs": timeout_ms}
    try:
        with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as sock:
            sock.settimeout(timeout_ms / 1000 + 10)
            sock.connect(path)
            sock.sendall((json.dumps(request, separators=(",", ":")) + "\n").encode())
            data = b""
            while not data.endswith(b"\n"):
                chunk = sock.recv(65536)
                if not chunk:
                    break
                data += chunk
    except OSError as e:
        print(f"call: no running server or client run on {path}: {e}", file=sys.stderr)
        return 3
    try:
        document = json.loads(data)
    except ValueError:
        print("call: the running process did not answer", file=sys.stderr)
        return 1
    output.write_line(document)
    return output.exit_code(document)
