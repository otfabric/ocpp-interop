# SPDX-License-Identifier: MIT
"""Command line of the adapter: the container contract (docs/CONTAINER_CONTRACT.md)."""

from __future__ import annotations

import asyncio
import json
import logging
import os
import re
import sys

from . import ADAPTER, CONTRACT_VERSION, SCHEMA_VERSION, output
from .fixture import FIXTURES_DIR, VERSION_DIRS, FixtureError, default_path, load

BUILD_INFO = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "build-info.json")
READY_FILE = "/run/ocpp-interop/ready"
CONTROL_SOCKET = "/run/ocpp-interop/control.sock"
SUPPORTED_VERSIONS = {"1.6", "2.0.1", "2.1"}
_NAME = re.compile(r"^[a-z0-9][a-z0-9-]*$")
_ACTION = re.compile(r"^[A-Z][A-Za-z0-9]+$")

USAGE = """usage:
  ocpp-interop server --version V [--fixture PATH] [--bind-address ADDR] [--port N]
                      [--security-profile N] [--tls-dir DIR] [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client run --version V --url URL [--station ID] [--fixture PATH] [--security-profile N]
                      [--tls-dir DIR] [--connect-timeout-ms N] [--timeout-ms N] [--hold-ms N]
                      [--ready-file PATH] [--control-socket PATH]
  ocpp-interop client <Action> --version V --url URL [--station ID] [--fixture PATH] [--payload JSON]
                      [--security-profile N] [--tls-dir DIR] [--connect-timeout-ms N] [--timeout-ms N]
  ocpp-interop call [--station ID] <Action> [--payload JSON] [--timeout-ms N] [--control-socket PATH]
  ocpp-interop print-capabilities
  ocpp-interop print-fixture --version V [NAME]
  ocpp-interop version"""

# What the self-tests exercise (scripts/smoke.sh, scripts/interop.sh), per
# version: every action of the version, in the direction the specification
# gives it (validate_fixtures.ACTIONS, scripts/validate-fixtures.py), less
# what ocpp 2.1.0 cannot exchange. The baseline fixtures have a sample and an
# answer for each. Nothing else is declared (REQUIREMENTS.md R4.2).
#
# Not sent, by the station of OCPP 2.1:
# - BatterySwap: ocpp 2.1.0 writes the battery fields soC and soH as SoC and
#   soh (charge_point.snake_to_camel_case), which its own validation refuses;
# - NotifyPeriodicEventStream: a SEND, which ocpp 2.1.0 has no frame for.
NOT_SENT = {"1.6": {"station": set(), "csms": set()},
            "2.0.1": {"station": set(), "csms": set()},
            "2.1": {"station": {"BatterySwap", "NotifyPeriodicEventStream"}, "csms": set()}}


def declared(version: str) -> dict:
    """{"station"|"csms": {"sends": [...], "handles": [...]}} for a version.

    An end handles what the other end sends: an action that no adapter sends
    (BatterySwap) is not exercised, so it is not declared as handled either.
    """
    from validate_fixtures import ACTIONS  # pylint: disable=import-outside-toplevel

    sends = {role: sorted(ACTIONS[version][role] - NOT_SENT[version][role]) for role in ("station", "csms")}
    return {"station": {"sends": sends["station"], "handles": sends["csms"]},
            "csms": {"sends": sends["csms"], "handles": sends["station"]}}


class UsageError(Exception):
    """Exit 2: nothing was sent."""


class Unsupported(Exception):
    """Exit 5: a version, role, profile or action this adapter does not support."""


def flags(args: list[str], known: set[str]) -> tuple[dict[str, str], list[str]]:
    """`--name value` pairs and the positional arguments. An unknown flag or a
    flag given twice is a usage error."""
    given: dict[str, str] = {}
    positional: list[str] = []
    i = 0
    while i < len(args):
        arg = args[i]
        if not arg.startswith("--"):
            positional.append(arg)
            i += 1
            continue
        name = arg[2:]
        if name not in known:
            raise UsageError(f"unknown flag {arg!r}")
        if i + 1 >= len(args):
            raise UsageError(f"{arg} needs a value")
        if name in given:
            raise UsageError(f"{arg} is given more than once")
        given[name] = args[i + 1]
        i += 2
    return given, positional


def number(given: dict, name: str, default: int, low: int, high: int) -> int:
    text = given.get(name, str(default))
    if not re.fullmatch(r"[0-9]+", text):
        raise UsageError(f"--{name}: {text!r} is not a number")
    value = int(text)
    if not low <= value <= high:
        raise UsageError(f"--{name}: {value} is not within {low}..{high}")
    return value


def version_of(given: dict) -> str:
    if "version" not in given:
        raise UsageError("--version is required")
    version = given["version"]
    if version not in VERSION_DIRS:
        raise UsageError(f"--version: {version!r} is not 1.6, 2.0.1 or 2.1")
    if version not in SUPPORTED_VERSIONS:
        raise Unsupported(f"OCPP {version} is not supported by this adapter")
    return version


def security_profile_of(given: dict) -> int:
    profile = number(given, "security-profile", 0, 0, 3)
    if "tls-dir" in given and profile < 2:
        raise UsageError("--tls-dir is for security profiles 2 and 3")
    if profile >= 2:
        raise Unsupported(f"security profile {profile} (TLS) is not supported by this adapter yet")
    return profile


def payload_of(given: dict) -> dict:
    try:
        payload = json.loads(given.get("payload", "{}"))
    except ValueError as e:
        raise UsageError(f"--payload is not JSON: {e}") from e
    if not isinstance(payload, dict):
        raise UsageError("--payload must be a JSON object")
    return payload


def action_of(positional: list[str]) -> str:
    if len(positional) != 1:
        raise UsageError("exactly one action is required")
    if not _ACTION.match(positional[0]):
        raise UsageError(f"{positional[0]!r} is not an OCPP action name")
    return positional[0]


def fixture_of(given: dict, version: str):
    fixture = load(given.get("fixture", str(default_path(version))), version)
    from .endpoint import error_class  # pylint: disable=import-outside-toplevel

    # The upstream answers with a CALLERROR of a code only through its
    # OCPPError subclass of that code.
    for role in ("csms", "station"):
        for action, answer in fixture.answers(role).items():
            code = answer.get("callError", {}).get("code")
            if code is not None and error_class(code) is None:
                raise Unsupported(f"answers.{role}.{action}: the ocpp package has no CALLERROR {code!r}")
    return fixture


def build_info() -> dict:
    try:
        with open(BUILD_INFO, encoding="utf-8") as handle:
            return json.load(handle)
    except (OSError, ValueError):
        return {}


def capabilities() -> dict:
    from importlib.metadata import version as installed  # pylint: disable=import-outside-toplevel

    info = build_info()
    return {
        "schemaVersion": SCHEMA_VERSION,
        "adapter": ADAPTER,
        "adapterVersion": info.get("adapterVersion", "dev"),
        "contractVersion": CONTRACT_VERSION,
        "upstream": {
            "name": "The Mobility House ocpp",
            "repository": "https://github.com/mobilityhouse/ocpp",
            "version": installed("ocpp"),
            "revision": info.get("upstreamRevision", "0" * 40),
            "license": "MIT",
            "language": "Python",
            "independentEngine": True,
        },
        "versions": {version: version_capabilities(version) for version in sorted(SUPPORTED_VERSIONS)},
    }


def version_capabilities(version: str) -> dict:
    subprotocol = {"1.6": "ocpp1.6", "2.0.1": "ocpp2.0.1", "2.1": "ocpp2.1"}[version]
    notes = [
        "The ocpp package has no transport; the adapter runs it over the websockets package, "
        "as the upstream's documentation and examples do.",
        f"An endpoint answers an action of OCPP {version} without a fixture entry with NotImplemented, "
        f"and an action that is not part of OCPP {version} with NotSupported (upstream behaviour).",
        f"The server refuses a handshake that does not offer the subprotocol {subprotocol} (HTTP 400, "
        "websockets).",
    ]
    if version == "2.1":
        notes[1] = (
            "An endpoint does not answer an OCPP 2.1 CALL whose action has no fixture entry: ocpp 2.1.0 raises "
            "neither NotImplemented nor NotSupported for 2.1 (charge_point._raise_key_error), so the sender "
            "times out (tests/known-bugs.tsv; fixed on the upstream's master, unreleased).")
        notes += [
            "No SEND and no CALLRESULTERROR: ocpp 2.1.0 knows the message types 2, 3 and 4 only "
            "(ocpp.messages.MessageType). It has no NotifyPeriodicEventStream (a SEND), which is unsupported "
            "(exit 5), and drops a frame of type 5 or 6 it receives, which the adapter reports as protocol-error.",
            "BatterySwap is not sent: ocpp 2.1.0 writes the battery fields soC and soH as SoC and soh "
            "(charge_point.snake_to_camel_case), which its own schema validation refuses.",
        ]
    actions = declared(version)
    return {
        "client": {"supported": True, "sends": actions["station"]["sends"], "handles": actions["station"]["handles"]},
        "server": {"supported": True, "sends": actions["csms"]["sends"], "handles": actions["csms"]["handles"]},
        "features": {
            "securityProfiles": [0, 1],
            "compression": False,
            "send": False,
            "callResultError": False,
            "fixtureCallErrors": True,
            "messageIds": True,
        },
        "notes": notes,
    }


def command_server(args: list[str]) -> int:
    given, positional = flags(args, {"version", "fixture", "bind-address", "port", "security-profile",
                                     "tls-dir", "ready-file", "control-socket"})
    if positional:
        raise UsageError(f"unexpected argument {positional[0]!r}")
    port = number(given, "port", 9000, 1, 65535)
    version = version_of(given)
    profile = security_profile_of(given)
    fixture = fixture_of(given, version)
    from . import server  # pylint: disable=import-outside-toplevel

    return asyncio.run(server.run(version, fixture, given.get("bind-address", "0.0.0.0"), port, profile,
                                  given.get("ready-file", READY_FILE),
                                  given.get("control-socket", CONTROL_SOCKET)))


def command_client(args: list[str]) -> int:
    if not args or args[0].startswith("--"):
        raise UsageError("client needs 'run' or an action")
    mode, rest = args[0], args[1:]
    known = {"version", "url", "station", "fixture", "security-profile", "tls-dir",
             "connect-timeout-ms", "timeout-ms"}
    known |= {"hold-ms", "ready-file", "control-socket"} if mode == "run" else {"payload"}
    given, positional = flags(rest, known)
    if positional:
        raise UsageError(f"unexpected argument {positional[0]!r}")
    action = None if mode == "run" else action_of([mode])
    if "url" not in given:
        raise UsageError("--url is required")
    url = given["url"]
    if not url.startswith(("ws://", "wss://")):
        raise UsageError("--url must start with ws:// or wss://")
    connect_timeout_s = number(given, "connect-timeout-ms", 5000, 1, 3600000) / 1000
    timeout_s = number(given, "timeout-ms", 10000, 1, 3600000) / 1000
    hold_ms = number(given, "hold-ms", 0, 0, 2**31) if mode == "run" else 0
    payload = payload_of(given) if action else {}
    version = version_of(given)
    profile = security_profile_of(given)
    if url.startswith("wss://"):
        raise Unsupported("wss:// (security profiles 2 and 3) is not supported by this adapter yet")
    fixture = fixture_of(given, version)
    station = given.get("station", fixture.first_station)
    if profile == 1 and station not in fixture.stations:
        raise UsageError(f"--station {station!r} is not in the fixture: security profile 1 needs its password")
    from . import client  # pylint: disable=import-outside-toplevel
    from .endpoint import request_type  # pylint: disable=import-outside-toplevel

    if action is None:
        return asyncio.run(client.run(version, url, station, fixture, profile, connect_timeout_s, hold_ms,
                                      given.get("ready-file", READY_FILE),
                                      given.get("control-socket", CONTROL_SOCKET)))
    if request_type(version, action) is None:
        output.write_line(output.result(version, action, ok=False, request=payload, error_code="unsupported",
                                        error_message=f"ocpp {version} has no request type for {action}"))
        return 5
    document = asyncio.run(client.one_shot(version, url, station, fixture, profile, connect_timeout_s, timeout_s,
                                           action, payload))
    output.write_line(document)
    return output.exit_code(document)


def command_call(args: list[str]) -> int:
    given, positional = flags(args, {"station", "payload", "timeout-ms", "control-socket"})
    action = action_of(positional)
    payload = payload_of(given)
    timeout_ms = number(given, "timeout-ms", 10000, 1, 3600000)
    from . import control  # pylint: disable=import-outside-toplevel

    return control.call(given.get("control-socket", CONTROL_SOCKET), given.get("station"), action, payload,
                        timeout_ms)


def command_print_fixture(args: list[str]) -> int:
    given, positional = flags(args, {"version"})
    if "version" not in given or given["version"] not in VERSION_DIRS:
        raise UsageError("--version 1.6, 2.0.1 or 2.1 is required")
    if len(positional) > 1:
        raise UsageError("at most one fixture name")
    name = positional[0] if positional else "baseline"
    if not _NAME.match(name):
        raise UsageError(f"{name!r} is not a fixture name")
    path = FIXTURES_DIR / VERSION_DIRS[given["version"]] / name / "fixture.json"
    try:
        data = path.read_bytes()
    except OSError:
        print(f"print-fixture: no fixture {name!r} for OCPP {given['version']} in this image", file=sys.stderr)
        return 1
    sys.stdout.buffer.write(data)
    sys.stdout.flush()
    return 0


def main(argv: list[str]) -> int:
    logging.basicConfig(stream=sys.stderr, level=os.environ.get("OCPP_INTEROP_LOG_LEVEL", "WARNING").upper(),
                        format="%(asctime)s %(levelname)s %(name)s: %(message)s")
    if not argv:
        print(USAGE, file=sys.stderr)
        return 2
    command, args = argv[0], argv[1:]
    try:
        if command == "server":
            return command_server(args)
        if command == "client":
            return command_client(args)
        if command == "call":
            return command_call(args)
        if command == "print-capabilities":
            if args:
                raise UsageError("print-capabilities takes no arguments")
            output.write_line(capabilities())
            return 0
        if command == "print-fixture":
            return command_print_fixture(args)
        if command == "version":
            if args:
                raise UsageError("version takes no arguments")
            from importlib.metadata import version as installed  # pylint: disable=import-outside-toplevel

            print(f"ocpp-interop-{ADAPTER} {build_info().get('adapterVersion', 'dev')} "
                  f"(ocpp {installed('ocpp')}, websockets {installed('websockets')}, contract {CONTRACT_VERSION})")
            return 0
        raise UsageError(f"unknown command {command!r}")
    except UsageError as e:
        print(f"usage error: {e}\n{USAGE}", file=sys.stderr)
        return 2
    except Unsupported as e:
        print(f"unsupported: {e}", file=sys.stderr)
        return 5
    except FixtureError as e:
        print(f"invalid fixture: {e}", file=sys.stderr)
        return 4
