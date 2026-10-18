# everest adapter

OCPP 1.6J, 2.0.1 and 2.1 reference **station** built on
[EVerest libocpp](https://github.com/EVerest/everest-core/tree/2026.02.1/lib/everest/ocpp),
behind the [container contract](../../docs/CONTAINER_CONTRACT.md).

| | |
|---|---|
| Upstream | EVerest libocpp (C++), its own protocol engine |
| Version | everest-core release `2026.02.1`, released 2026-07-16 (commit `b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e`); its `lib/everest/ocpp` is libocpp project version 0.31.1 |
| Where libocpp lives | `lib/everest/ocpp` of the [everest-core](https://github.com/EVerest/everest-core) monorepo. The standalone [EVerest/libocpp](https://github.com/EVerest/libocpp) repository is archived; its last release, v0.31.1 (2025-12-11), is older code than the monorepo release pinned here |
| Built from the same tree | the EVerest libraries libocpp links: `lib/everest/log`, `timer`, `sqlite`, `evse_security`. Nothing else of everest-core |
| Transport | libocpp's `ocpp::Websocket` on [libwebsockets](https://github.com/warmcat/libwebsockets) 4.5.2 |
| Other dependencies | built from the tags libocpp's `dependencies.yaml` names, each checked against its commit: nlohmann/json 3.12.0, json-schema-validator 2.4.0, date 3.0.4, libwebsockets 4.5.2; Boost 1.83, OpenSSL 3.5 and SQLite 3.46 from Debian trixie. Only libwebsockets is a shared library in the image; OpenSSL, SQLite and the C++ runtime are the base image's |
| Build tool only | [everest-cmake](https://github.com/EVerest/everest-cmake) v0.6.0 (commit `e378100829014ad4c3a721a7aa1c02ef56736c61`), the release current when everest-core 2026.02.1 was tagged |
| Licences | libocpp and the EVerest libraries Apache-2.0; two WebSocket++ headers bundled in libocpp BSD-3-Clause; nlohmann/json, json-schema-validator, date and libwebsockets MIT; Boost BSL-1.0 |
| Base image | `debian:trixie-slim` (build and runtime), pinned by digest in the [Dockerfile](Dockerfile) |
| OCPP | 1.6J, 2.0.1 and 2.1, **station (`client`) only**; security profiles 0 to 3 |

The adapter code is MIT. `/licenses` in the image holds the licence of
everything in the executable (`/licenses/BUNDLED.txt` lists the sources by
tag and commit), this repository's licence and THIRD_PARTY_NOTICES.md.

## OCPP 2.1 in libocpp

The libocpp README of the pinned release says OCPP 2.1 support is "under
development": websocket connections and messages are supported, every
OCPP 2.0.1 functional block works with 2.1, and the 2.1 functional blocks are
being added. For this adapter, what matters is in the source of the tag:

- the subprotocol `ocpp2.1` (`ocpp::OcppProtocolVersion::v21`,
  [types.cpp](https://github.com/EVerest/everest-core/blob/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/lib/ocpp/common/types.cpp#L1063-L1093));
- the message structs of every OCPP 2.1 action: the 2.0.1 messages in
  `ocpp::v2` (generated from the 2.1 schemas, shared by 2.0.1 and 2.1) and
  the 27 messages 2.1 adds in `ocpp::v21`
  ([include/ocpp/v21/messages](https://github.com/EVerest/everest-core/tree/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/include/ocpp/v21/messages));
- **no SEND and no CALLRESULTERROR**: `ocpp::MessageTypeId` has CALL (2),
  CALLRESULT (3) and CALLERROR (4) only, and 5 is `UNKNOWN`
  ([call_types.hpp](https://github.com/EVerest/everest-core/blob/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/include/ocpp/common/call_types.hpp#L47-L52)).
  NotifyPeriodicEventStream, a SEND in 2.1, is one struct without a response
  in libocpp and nothing sends it as a SEND.

## How the upstream is used

libocpp implements the charging station. Its `ChargePoint` classes
(`ocpp::v16::ChargePoint`, `ocpp::v2::ChargePoint`) are station firmware:
`start()` sends BootNotification, they send StatusNotification and Heartbeat
by themselves, and they answer the CSMS from their configuration or device
model. The contract needs a station that sends only what a test asks for
(`client run` "does not send BootNotification by itself") and answers from the
fixture. So the adapter uses the parts of libocpp that `ChargePoint` itself
is built on, the way `ChargePoint` uses them:

- **`ocpp::Websocket`** ([websocket_libwebsockets.cpp](https://github.com/EVerest/everest-core/blob/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/lib/ocpp/common/websocket/websocket_libwebsockets.cpp)):
  the connection, with the subprotocol of `--version`, the station identity
  appended to the URL by libocpp (`ocpp::Uri::parse_and_validate`), HTTP
  Basic authentication in profiles 1 and 2 (user = identity, password = the
  fixture's), and TLS in profiles 2 and 3. One connection attempt
  (`max_connection_attempts` 0), no WebSocket pings.
- **`ocpp::MessageQueue<M>`** ([message_queue.hpp](https://github.com/EVerest/everest-core/blob/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/include/ocpp/common/message_queue.hpp)):
  sends CALLs one at a time (`push_call_async`), matches the CALLRESULT or
  CALLERROR to the CALL in flight, times it out, and sends the answers
  (`push_call_result`, `push_call_error`) after a received CALL. Its retries
  are set to one attempt, so nothing is ever resent; its database (for
  transaction messages that survive a restart) is replaced by one that keeps
  nothing.
- **the message structs**: every payload, sent or received, is read into
  libocpp's request or response struct of that action and version (its
  `from_json`) and written by it (`to_json`), framed by libocpp's
  `Call<T>` and `CallResult<T>`. [src/registry_v16.cpp](src/registry_v16.cpp)
  and [src/registry_v2.cpp](src/registry_v2.cpp) list every message header of
  the release; the image build fails when the release has others.

What the adapter decides is what `ChargePoint` would decide:

| Received CALL | Answer |
|---|---|
| An action of the version that libocpp has a struct for, and the fixture answers | the fixture's answer, read into libocpp's response struct and written by it (`call-answered` shows it as sent) |
| The fixture has a `callError` answer | that CALLERROR. libocpp's `CallError` takes any code; the adapter refuses a fixture with a code that is not an OCPP-J error code of the version (exit 5), as every adapter does |
| No fixture answer, or an action the station does not receive in that version | `NotImplemented`, as `ChargePoint::send_not_implemented_error` |
| A payload libocpp cannot read into the request struct | `FormationViolation`, the code `ChargePoint` uses in 1.6 and 2.x |
| An action libocpp does not know (its queue throws `StringToEnumException`) | `NotImplemented` (adapter's choice: `ChargePoint` answers such frames GenericError or RpcFrameworkError with message id `-1`) |

Every CALL libocpp's queue hands over reaches the adapter, so every received
CALL has `call-received` and `call-answered` events.

Sending (`client <Action>`, `call`): the payload is read into libocpp's
request struct (`invalid-request` when that throws); the request in the
result is what libocpp wrote, which drops fields its struct does not have
and writes date-times with milliseconds. The response is read into
libocpp's response struct (`invalid-response` when that throws) and written
back by it. The message id is libocpp's (a UUID). Only the actions the
station sends in the version are sent (others `unsupported`, exit 5).

## Security profiles and compression

Profiles 0 to 3 are declared. Profiles 2 and 3 use
libocpp's own TLS (OpenSSL through libwebsockets), fed with the files of
`--tls-dir` through a small `ocpp::EvseSecurity` implementation:
`ca.crt` is the only trust anchor for the CSMS (`use_ssl_default_verify_paths`
off), the CSMS's certificate must name the host of `--url`
(`verify_csms_common_name`, no wildcards), and in profile 3 `client.crt` and
`client.key` are the station's certificate (contract 0.5). The URL must be `wss://` with
profiles 2 and 3 and `ws://` otherwise (usage error).

`--compression` exits 5: libocpp creates its libwebsockets client context
without extensions, so it cannot offer permessage-deflate.
`connection-opened` reports `"compression": false`.

## Limitations

Listed in the `notes` of the capabilities:

- No CSMS: `server` exits 5, after the usage checks.
- No SEND, no CALLRESULTERROR (see above). A frame of type 5 or 6 is
  reported as `protocol-error` and not answered.
- libocpp does not pass the close code of a connection to the application:
  `connection-closed` has `code` and `reason` null.
- A BootNotification answered with a CALLERROR stays in libocpp's queue for
  a resend after `boot_notification_retry_interval_seconds`, which the
  adapter sets to a year; the result reports the CALLERROR from the frame.
- libocpp always appends the identity to the URL: `--station ""` exits 5.
- **Upstream defect**: `ocpp::v2::MessageType` has no `ReportDERControl`,
  `GetCertificateChainStatus` or `NotifyWebPaymentStarted`, although
  libocpp has their message structs. Its `MessageQueue` converts the action
  of every CALL to that enum (`ControlMessage`, `receive`), so libocpp can
  neither send these CALLs (`failed`: "could not be converted to
  MessageType") nor read one it receives (answered `NotImplemented`).
  ReportDERControl stays declared; `v21.from-station.report-dercontrol` is a
  KNOWN_ISSUE ([tests/known-bugs.tsv](../../tests/known-bugs.tsv)).

`OCPP_INTEROP_LOG_LEVEL` (`info`, `debug`) makes libocpp's logging on stderr
more verbose; the default is warnings. At `debug`, libocpp logs the Basic
authentication header, which holds the fixture's (public, test-only)
password.

## Build

The [Dockerfile](Dockerfile) clones every source at its tag and checks the
commit, builds the four third-party libraries with the options libocpp's
`dependencies.yaml` gives them, then libocpp, the EVerest libraries and the
adapter in one CMake project ([CMakeLists.txt](CMakeLists.txt)) with
EVerest's dependency manager off. The build takes about 10 minutes on four
cores and about 1 GiB of memory per compile job (`BUILD_JOBS`, default 4).
The image is about 150 MB.

A candidate build (`make candidate-everest CANDIDATE_VERSION=<everest-core
tag> CANDIDATE_SHA=<commit>`) builds another everest-core release the same
way.
