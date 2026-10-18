# SPDX-License-Identifier: MIT
"""`client run` and `client <Action>`: a station. The upstream ChargePoint
over a websockets client connection."""

from __future__ import annotations

import asyncio
import base64
import os
import signal
import sys
import time

from websockets.asyncio.client import connect
from websockets.exceptions import ConnectionClosed, InvalidHandshake
from websockets.typing import Subprotocol

from . import output
from .endpoint import make_endpoint_class
from .server import SUBPROTOCOLS


class ConnectFailed(Exception):
    """The WebSocket connection could not be opened, or the subprotocol was
    not accepted."""


def station_url(url: str, station: str) -> str:
    return url if not station else url.rstrip("/") + "/" + station


async def open_connection(version, url, station, fixture, security_profile, connect_timeout_s):
    headers = {}
    if security_profile == 1:
        password = fixture.stations[station]["password"]
        token = base64.b64encode(f"{station}:{password}".encode()).decode()
        headers["Authorization"] = f"Basic {token}"
    subprotocol = SUBPROTOCOLS[version]
    try:
        connection = await connect(station_url(url, station), subprotocols=[Subprotocol(subprotocol)],
                                   additional_headers=headers, open_timeout=connect_timeout_s)
    except (OSError, InvalidHandshake, asyncio.TimeoutError, TimeoutError, ValueError) as e:
        raise ConnectFailed(f"{e.__class__.__name__}: {e}") from e
    if connection.subprotocol != subprotocol:
        await connection.close()
        raise ConnectFailed(f"the server did not accept the subprotocol {subprotocol}")
    return connection


async def one_shot(version, url, station, fixture, security_profile, connect_timeout_s, timeout_s,
                   action, payload) -> dict:
    """`client <Action>`: connect, send one CALL, disconnect."""
    started = time.monotonic()
    try:
        connection = await open_connection(version, url, station, fixture, security_profile, connect_timeout_s)
    except ConnectFailed as e:
        return output.result(version, action, ok=False, request=payload, error_code="connect-failed",
                             error_message=str(e), elapsed_ms=(time.monotonic() - started) * 1000)
    endpoint = make_endpoint_class(version)(station, connection, fixture, "station", emit=False)
    reader = asyncio.ensure_future(_serve(endpoint, connection))
    try:
        document = await endpoint.execute(action, payload, timeout_s)
    finally:
        await connection.close()
        await asyncio.wait({reader}, timeout=2)
    document["elapsedMs"] = max(0, int((time.monotonic() - started) * 1000))
    return document


async def _serve(endpoint, connection):
    try:
        await endpoint.start()
    except ConnectionClosed:
        pass
    finally:
        if not endpoint.closed.done():
            endpoint.closed.set_result(True)


async def run(version, url, station, fixture, security_profile, connect_timeout_s, hold_ms, ready_file,
              control_path) -> int:
    """`client run`: stay connected, answer from the fixture, accept `call`."""
    from . import control  # pylint: disable=import-outside-toplevel

    stop = asyncio.Event()
    loop = asyncio.get_running_loop()
    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, stop.set)

    full_url = station_url(url, station)
    try:
        connection = await open_connection(version, url, station, fixture, security_profile, connect_timeout_s)
    except ConnectFailed as e:
        output.event("connection-failed", station, url=full_url, detail=str(e))
        output.event("stopped")
        return 3
    endpoint = make_endpoint_class(version)(station, connection, fixture, "station")
    output.event("connection-opened", station, subprotocol=connection.subprotocol, remote=full_url)
    reader = asyncio.ensure_future(_serve(endpoint, connection))

    def find(wanted):
        if wanted not in (None, "", station) or endpoint.closed.done():
            return None
        return endpoint

    try:
        control_server = await control.serve(control_path, version, find)
    except OSError as e:
        print(f"client: cannot open the control socket {control_path}: {e}", file=sys.stderr)
        await connection.close()
        return 3
    with open(ready_file, "w", encoding="utf-8") as handle:
        handle.write("ready\n")
    output.event("ready", station, version=version, url=full_url)

    waits = {asyncio.ensure_future(stop.wait()), reader}
    hold = None
    if hold_ms > 0:
        hold = asyncio.ensure_future(asyncio.sleep(hold_ms / 1000))
        waits.add(hold)
    await asyncio.wait(waits, return_when=asyncio.FIRST_COMPLETED)
    lost = reader.done() and not stop.is_set() and not (hold and hold.done())

    try:
        os.unlink(ready_file)
    except OSError:
        pass
    control_server.close()
    if not lost:
        await connection.close()
        await asyncio.wait({reader}, timeout=5)
    output.event("connection-closed", station, code=connection.close_code, reason=connection.close_reason)
    output.event("stopped")
    return 1 if lost else 0
