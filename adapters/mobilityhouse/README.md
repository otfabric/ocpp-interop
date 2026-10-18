# mobilityhouse adapter

OCPP 1.6J reference CSMS and station built on
[The Mobility House ocpp](https://github.com/mobilityhouse/ocpp), behind the
[container contract](../../docs/CONTAINER_CONTRACT.md).

| | |
|---|---|
| Upstream | The Mobility House ocpp (Python), its own protocol engine |
| Version | 2.1.0, released 2025-07-16 (git tag `2.1.0`, commit `91f4f89140a2198dc5a300cfea88d947ac91ae28`) |
| Artifact | `ocpp-2.1.0-py3-none-any.whl` from PyPI, `sha256:0e452c14c21e2995431334ce1513d5ad8246ecb5c8fb90ed4143e2ecdd9e2845`. Its `ocpp/` tree is identical to the git tag |
| Transport | [websockets](https://github.com/python-websockets/websockets) 17.2: the ocpp package has none of its own; its documentation and examples use websockets |
| Other dependencies | jsonschema 4.26.0 (the upstream's only dependency) with attrs, jsonschema-specifications, referencing and rpds-py |
| Licences | ocpp MIT; websockets BSD-3-Clause; jsonschema and its dependencies MIT; Python PSF-2.0. The OCA JSON schemas inside the ocpp package are CC BY-ND 4.0 and are used unmodified |
| Base image | `python:3.14.8-slim-trixie`, pinned by digest in the [Dockerfile](Dockerfile) |
| OCPP | 1.6J, both roles, security profiles 0 and 1 |

The adapter code is MIT. `/licenses` in the image holds the licence of every
installed package, of Python and of this repository, and
THIRD_PARTY_NOTICES.md.

## Build

[requirements.txt](requirements.txt) pins every package by version and by
the sha256 of its wheels, and the build installs it with `--require-hashes`.
`UPSTREAM_VERSION` must equal the ocpp pin for a release build. A candidate
build (`make candidate-mobilityhouse CANDIDATE_VERSION=x.y.z
CANDIDATE_SHA=<commit of its tag>`) installs that ocpp release and its
dependencies from PyPI without hashes, keeps websockets at the pin, and
fails if the installed version is not the requested one. `UPSTREAM_SHA` is
recorded as `upstream.revision`; PyPI cannot prove it.

`OCPP_INTEROP_LOG_LEVEL=INFO` shows the upstream's frame log on stderr.

## How the upstream is used

`ocpp.v16.ChargePoint` is one class for both ends of a connection. The
adapter ([ocpp_interop/endpoint.py](ocpp_interop/endpoint.py)) subclasses it
only to:

- register a handler for every action the fixture answers, in the upstream's
  route map (what its `@on()` decorator builds). The handler builds the
  answer as the upstream's `ocpp.v16.call_result` dataclass, with the
  upstream's `camel_to_snake_case`; a `callError` answer raises the
  upstream's `OCPPError` subclass of that code. Schema validation of the
  request and of the answer, serialisation, and the CALLERRORs for unknown
  actions (`NotImplemented`, `NotSupported`) or invalid payloads are the
  upstream's;
- observe the frames the upstream sends (`_send`) and receives
  (`route_message`), for the events;
- keep the raw response of a CALL (`_get_specific_response`), so that a
  CALLERROR can be told apart from a response the upstream rejects.

Sending: the payload becomes the upstream's `ocpp.v16.call` dataclass and
goes out with `ChargePoint.call(..., suppress=False)`, which validates it
against the OCA schema first (`invalid-request` when that fails). The
response is the upstream's `call_result` dataclass, written back with the
upstream's own serialisation. Message ids are the upstream's (UUID 4),
passed explicitly so that they can be reported.

Server: `websockets.asyncio.server.serve` with the subprotocol of the
version; `process_request` refuses unknown identities (404) and, with
profile 1, wrong Basic credentials (401). websockets itself refuses a
handshake without the subprotocol (400). Client:
`websockets.asyncio.client.connect`.

## Known upstream defects

- `ocpp.v16.call.SecurityEventNotification` declares `tech_info` without a
  default, so a SecurityEventNotification without the optional `techInfo`
  cannot be built. Recorded in
  [tests/known-bugs.tsv](../../tests/known-bugs.tsv); the station sends it
  fine with `techInfo`.
