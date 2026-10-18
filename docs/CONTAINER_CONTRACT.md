# Container contract

Version: **0.5 (draft)**. `contract_version` in [versions.yaml](../versions.yaml).

> Implemented by `mobilityhouse`, `chargetime` and `lorenzodonini` for OCPP
> 1.6J and 2.0.1, and by `mobilityhouse` for OCPP 2.1, with security
> profiles 0 to 3 everywhere; compression as each adapter declares it; and by `everest`
> (a station only: `client` and `call`) for OCPP 1.6J, 2.0.1 and 2.1. It stays a draft (0.x)
> until every adapter implements every part of it;
> [Gaps before 1.0](#gaps-before-10) lists what is missing. Until 1.0 it may
> change without a major version bump.

Every adapter image exposes the same commands, flags, JSON documents and exit
codes. Consumers depend on this document, not on any adapter's source. After
1.0, a breaking change bumps the major version and an addition bumps the
minor version. Documents carry `schemaVersion` `"1.0"`, which names the
compatible line and changes only with the major version.

## Contents

- [Image layout](#image-layout)
- [How OCPP shapes this contract](#how-ocpp-shapes-this-contract)
- [Commands](#commands)
- [`server`](#server)
- [`client`](#client)
- [`call`](#call)
- [Answers](#answers)
- [OCPP 2.1: SEND and CALLRESULTERROR](#ocpp-21-send-and-callresulterror)
- [Transport security](#transport-security)
- [Compression](#compression)
- [Events](#events)
- [Result document](#result-document)
- [`print-capabilities`](#print-capabilities)
- [`print-fixture`](#print-fixture)
- [Exit codes](#exit-codes)
- [Output rules](#output-rules)
- [Gaps before 1.0](#gaps-before-10)
- [Changes](#changes)

## Image layout

| Item | Value |
|------|-------|
| Image | `ghcr.io/otfabric/ocpp-interop-<adapter>` |
| Entrypoint | `ocpp-interop` (`/usr/local/bin/ocpp-interop`) |
| Default command | `server --version 1.6` |
| Protocol | OCPP-J over WebSocket: `ws://` with security profiles 0 and 1, `wss://` (TLS) with profiles 2 and 3, with keys and certificates the test generates and mounts at runtime ([Transport security](#transport-security)) |
| Port | 9000/tcp |
| Path | `/ocpp/<station identity>`; a server takes the identity from the last path segment |
| User | non-root, uid 10001 |
| Fixtures | `/fixtures/<version>/<name>/fixture.json`, baked in; mount over `/fixtures` to replace, or pass `--fixture` |
| Readiness file | `/run/ocpp-interop/ready` |
| Control socket | `/run/ocpp-interop/control.sock` (see [`call`](#call)) |
| Health check | built in: `test -f /run/ocpp-interop/ready` |
| Licences | `/licenses/` |

`<version>` is `v16`, `v201` or `v21`. On the command line, versions are
written as `1.6`, `2.0.1` and `2.1`.

## How OCPP shapes this contract

The contracts of the sibling repositories (Modbus, IEC 104) have a
long-running server and a one-shot client. OCPP needs more than that:

1. **Both ends send requests.** A CSMS must be able to send
   RemoteStartTransaction, Reset or GetConfiguration to a station that is
   connected to it. A test must therefore be able to make a running
   endpoint, client or server, send a CALL on an open connection. The
   [`call`](#call) command does this through a control socket inside the
   container (`docker exec <container> ocpp-interop call ...`).
2. **The station stays connected.** A test of a CSMS sending to a station
   needs a reference station that connects and stays connected. This is
   `client run`. One-shot operations are `client <Action>`.
3. **Requests are raw JSON.** An operation is an OCPP action with a JSON
   payload. The adapter turns that payload into the upstream library's own
   request type using the upstream's own deserialisation, sends it with the
   upstream engine, and prints the response as the upstream serialises it.
   This keeps the contract the same for every action and every version. It
   also uses no OCPP code other than the upstream's, which R1.3 requires.
4. **Answers come from the fixture.** When an endpoint receives a CALL, it
   answers with the response the fixture defines for that action
   ([Answers](#answers)). Every CALL and answer is reported as an event.

## Commands

```text
ocpp-interop server --version V [flags]
ocpp-interop client run --version V --url URL [flags]
ocpp-interop client <Action> --version V --url URL [--payload JSON] [flags]
ocpp-interop call [--station ID] <Action> [--payload JSON] [--timeout-ms N] [--control-socket PATH]
ocpp-interop print-capabilities
ocpp-interop print-fixture --version V [NAME]
ocpp-interop version
```

Flags have the form `--name value`, except `--compression`, a switch
without a value. An unknown flag, a flag given twice, a missing required
flag, a value that is not valid for the flag (including a `--tls-dir`
without the files its profile needs, or with files that are not valid PEM),
or a payload that is not a JSON object is a usage error (exit 2) and is
never ignored.
An action name must match `^[A-Z][A-Za-z0-9]+$`. `client` takes `run` or the
action as its first argument.

Checks happen in this order, before any network access: usage (exit 2), then
what the adapter supports (exit 5: version, security profile, action), then
the fixture (exit 4).

## `server`

Runs a CSMS for one version and one fixture until it receives SIGTERM or
SIGINT.

| Flag | Default | Meaning |
|------|---------|---------|
| `--version V` | required | `1.6`, `2.0.1` or `2.1`: the subprotocol offered (`ocpp1.6`, `ocpp2.0.1`, `ocpp2.1`) |
| `--fixture PATH` | `/fixtures/<version>/baseline/fixture.json` | Fixture to serve |
| `--bind-address ADDR` | `0.0.0.0` | Listen address |
| `--port N` | 9000 | Listen port |
| `--security-profile N` | `0` | `0`: no authentication; `1`: HTTP Basic authentication over `ws://`: the user name is the station identity and the password is the fixture's password of that station; `2`: TLS and Basic authentication as profile 1; `3`: TLS with a client certificate ([Transport security](#transport-security)) |
| `--tls-dir DIR` | — | `server.crt` and `server.key`, and for profile 3 `ca.crt`, generated by the test and mounted read-only. Required with profiles 2 and 3, a usage error with 0 and 1 |
| `--ready-file PATH` | `/run/ocpp-interop/ready` | Readiness file |
| `--control-socket PATH` | `/run/ocpp-interop/control.sock` | Control socket for [`call`](#call) |

Lifecycle:

1. Validate the fixture. A broken fixture exits 4; a fixture the adapter
   cannot serve (see [Answers](#answers)) exits 5.
2. Listen. A port that cannot be bound exits 3.
3. Write the readiness file and the `ready` event.
4. Accept stations whose identity the fixture lists. Other identities are
   refused during the handshake (HTTP 404) with the `connection-rejected`
   event, reason `unknown-station`. With security profiles 1 and 2, wrong
   or missing credentials are refused (HTTP 401), reason `authentication`;
   with profile 3, a client certificate that is not the station's (see
   [Transport security](#transport-security)).
5. Answer CALLs from the fixture, and send the CALLs that `call` asks for.
6. On SIGTERM or SIGINT, remove the readiness file, close the sessions,
   write `stopped`, and exit 0.

## `client`

`client run` connects as one station and stays connected. It answers CSMS
requests from the fixture, accepts [`call`](#call) commands, and reports
[events](#events). It does not send BootNotification by itself: the test
sends it with `call`, so that every CALL on the wire is one the test asked
for. It ends:

- on SIGTERM, SIGINT or at the end of `--hold-ms`: it closes the connection,
  writes `connection-closed` and `stopped`, removes the readiness file and
  exits 0;
- when the CSMS closes the connection: `connection-closed`, `stopped`,
  exit 1;
- when it cannot connect: `connection-failed`, `stopped`, exit 3.

`client <Action>` connects, sends one CALL, prints one
[result document](#result-document), disconnects, and writes no events. It
answers CALLs the CSMS sends meanwhile from the fixture.

| Flag | Default | Meaning |
|------|---------|---------|
| `--version V` | required | Subprotocol to offer; the CSMS must accept it |
| `--url URL` | required | `ws://host:port/path` with security profiles 0 and 1, `wss://host:port/path` with 2 and 3; any other combination is a usage error. `/<station>` is appended unless `--station` is empty |
| `--station ID` | the fixture's first station | Station identity |
| `--fixture PATH` | `/fixtures/<version>/baseline/fixture.json` | Fixture used for answers and credentials |
| `--payload JSON` | `{}` | Request payload (`client <Action>` only) |
| `--security-profile N` | `0` | As for `server`; profiles 1 and 2 send the station identity and the fixture's password of that station |
| `--tls-dir DIR` | — | `ca.crt`, and for profile 3 `client.crt` and `client.key`. Required with profiles 2 and 3, a usage error with 0 and 1 |
| `--compression` | off | Offer permessage-deflate, and use it when the CSMS accepts it ([Compression](#compression)). An adapter whose upstream cannot offer it in that version exits 5 |
| `--connect-timeout-ms N` | 5000 | Upper bound for the WebSocket handshake |
| `--timeout-ms N` | 10000 | Upper bound for the CALL of `client <Action>` |
| `--hold-ms N` | 0 | `client run`: disconnect after N ms; 0 means until SIGTERM |
| `--ready-file PATH` | `/run/ocpp-interop/ready` | `client run`: written once the connection is open |
| `--control-socket PATH` | `/run/ocpp-interop/control.sock` | `client run`: control socket for [`call`](#call) |

## `call`

```text
docker exec <container> ocpp-interop call [--station ID] <Action> [--payload JSON] [--timeout-ms N]
```

`call` sends one CALL from a running `server` (to the station with that
identity) or from `client run` (to its CSMS; `--station` may be omitted, and
must be the station's own identity if given). It returns when the response
arrives or after `--timeout-ms` (default 10000), and prints one
[result document](#result-document). A server without an open connection to
that station, or a `client run` asked for another station, answers with
`error.code` `not-connected`.

`call` talks to the running process through the control socket, which
serves only this command and only inside the container. Its protocol is
internal to each image and may change with the image; for reference, every
adapter uses one JSON line each way: the request
`{"station": "ID" | null, "action": "...", "payload": {...}, "timeoutMs": N}`
and the result document.

When no `server` or `client run` listens on the control socket, `call` writes
nothing to stdout, a diagnostic to stderr, and exits 3.

## Answers

When an endpoint receives a CALL, it answers as [the fixture](FIXTURES.md)
defines: with the payload, with a derived payload (`idTokens`,
`configuration` in 1.6, `variables` in 2.x), with `"${now}"` replaced by the
current UTC time, or with the fixture's CALLERROR. The adapter turns the
answer into the upstream's confirmation type with the upstream's
deserialisation, and the upstream sends it.

- An action of the version that the fixture does not answer is answered with
  the CALLERROR `NotImplemented`, by the upstream.
- An action that is not part of the version is answered as the upstream
  decides (`NotImplemented` or `NotSupported`).
- An adapter whose upstream cannot answer with a chosen CALLERROR declares
  the feature `fixtureCallErrors: false` and refuses a fixture with
  `callError` answers (exit 5).
- A `callError` answer whose code the upstream cannot send (every reference
  upstream sends only the error codes of the OCPP-J specification) is
  refused with exit 5, also by an adapter that declares `fixtureCallErrors`.
- An upstream may answer a CALL it cannot route itself, without the adapter
  seeing it. Such CALLs have no `call-received` or `call-answered` event.
- An action that the version sends as a SEND (OCPP 2.1:
  NotifyPeriodicEventStream) has no answer; a fixture that gives one is
  invalid (exit 4).

Known exceptions are listed per adapter in the `notes` of its capabilities.

## OCPP 2.1: SEND and CALLRESULTERROR

OCPP 2.1 (Part 4, OCPP-J) adds two message types to CALL (2), CALLRESULT (3)
and CALLERROR (4):

- **CALLRESULTERROR** (5): the answer to a CALLRESULT that the receiver
  cannot process;
- **SEND** (6): a message that is never answered and may be sent while a
  CALL is outstanding. NotifyPeriodicEventStream is sent as a SEND.

The features `send` and `callResultError` say whether an adapter sends and
receives them ([CAPABILITIES.md](CAPABILITIES.md)). No upstream release this
repository pins implements either, so every adapter declares both `false`,
and this version of the contract has no operation that sends a SEND or
provokes a CALLRESULTERROR (see [Gaps before 1.0](#gaps-before-10)). Until
it has:

- an adapter without `send` never sends a SEND action as a CALL: `client
  <Action>` and `call` refuse it with `error.code` `unsupported` (exit 5);
- a frame of type 5 or 6 that the upstream cannot read is reported as
  `protocol-error`, where the upstream exposes it.

## Transport security

Security profiles 2 and 3 of OCPP (1.6 Security Whitepaper, 2.0.1 and 2.1
Part 2, Security) run OCPP-J over TLS. The test generates the CA, the keys
and the certificates for each run and mounts them read-only; nothing of it
is baked into an image (REQUIREMENTS.md R7.3).

`--tls-dir` holds PEM files:

| File | Who | Content |
|---|---|---|
| `server.crt` | server | the server certificate, then any intermediate certificates |
| `server.key` | server | its private key |
| `ca.crt` | station; server with profile 3 | the CA certificates to trust |
| `client.crt` | station with profile 3 | the client certificate, then any intermediate certificates |
| `client.key` | station with profile 3 | its private key |

Private keys are unencrypted PKCS #8 (`-----BEGIN PRIVATE KEY-----`). Keys
may be RSA (2048 bits or more) or ECDSA on P-256; adapters accept both.

- **TLS**: version 1.2 or later, at both ends.
- **The station** connects to a `wss://` URL, trusts only the certificates
  of `ca.crt`, and verifies that the server's certificate names the host of
  the URL (a DNS name or an IP address in its subject alternative names).
  A server certificate that does not verify fails the connection
  (`connect-failed`, or `connection-failed` for `client run`). The test
  issues server certificates with subject alternative names for every name
  it connects by (for example `localhost`, `127.0.0.1`,
  `host.docker.internal`, the container names on the test network).
- **Profile 2**: the server presents `server.crt`; the station sends HTTP
  Basic credentials exactly as with profile 1 (the user name is the station
  identity, the password the fixture's password of that station), and the
  server checks them as with profile 1.
- **Profile 3**: the server also requires a client certificate that
  verifies against its `ca.crt`, and the station identity in the URL must
  equal the subject CommonName of that certificate. A certificate that does
  not verify fails the TLS handshake, before an identity is known: the
  server may then write no event. A verified certificate of another
  identity is refused with HTTP 401 and the event `connection-rejected`,
  reason `authentication`. There is no Basic authentication with
  profile 3: the station sends none and the server ignores any.
- A station without a client certificate, or a `ws://` connection to a TLS
  server, fails like any other handshake.

An adapter or upstream that cannot do a profile leaves it out of
`features.securityProfiles` and exits 5 for it, and says why in its
`notes`.

## Compression

Compression is the WebSocket extension permessage-deflate (RFC 7692),
negotiated in the handshake.

- **The CSMS** accepts permessage-deflate whenever a station offers it. It
  is mandatory for a CSMS in OCPP 2.0.1 (Part 4, 3.3) and 2.1 (Part 4,
  3.4); in OCPP 1.6 a CSMS accepts it where its upstream can.
- **The station** offers it only with `--compression`, and then uses it
  if the CSMS accepts. Without the switch it offers no extension.
- The extension's parameters (context takeover, window sizes) are the
  upstream's, as negotiated.
- `features.compression` is `true` for a version only when the adapter's
  station can offer compression and its CSMS accepts it. An upstream that
  cannot do it in one role is named in the adapter's `notes`; a known
  defect is listed in [tests/known-bugs.tsv](../tests/known-bugs.tsv), so
  that the test reports it as `KNOWN_ISSUE`.
- `connection-opened` reports whether compression was negotiated where the
  upstream exposes it ([Events](#events)).

## Events

`server` and `client run` write [JSON Lines](https://jsonlines.org/) to
stdout. Every event has these fields:

- `event`
- `time` (RFC 3339, UTC)
- `station`, when the event concerns one station

| `event` | Additional fields |
|---|---|
| `ready` | server: `adapter`, `version`, `port`, `fixture`, `securityProfile`; `client run`: `version`, `url` |
| `connection-opened` | `subprotocol`, `remote`; `compression` (`true` or `false`: permessage-deflate negotiated; `null` where the upstream does not expose it) |
| `connection-rejected` | `reason` (`unknown-station`, `authentication`, `subprotocol`), `remote` |
| `connection-failed` | `client run` only: `url`, `detail` |
| `connection-closed` | `code`, `reason` |
| `call-received` | `messageId`, `action`, `payload` |
| `call-answered` | `messageId`, `action`, `payload` or `error` |
| `call-sent` | `messageId`, `action`, `payload` |
| `call-result` | `messageId`, `action`, `payload` or `error` |
| `protocol-error` | `detail`, and the raw frame up to 4 KiB in `frame` |
| `stopped` | — |

`error` is `{code, description, details}`. A field whose value the upstream
does not expose is `null`: for example the message id of a CALL the upstream
sends, the negotiated subprotocol or the close code (see each adapter's
`notes`, and the `messageIds` feature). Payloads are as the upstream reads or
writes them: an upstream may write a timestamp with milliseconds, a decimal
as `11000.0`, or add an optional field with its default value.

Tests assert on events, so events are written as soon as they happen and
stdout is flushed after each line.

## Result document

`client <Action>` and `call` print exactly one line, a JSON object that
validates against
[schemas/client-result.schema.json](../schemas/client-result.schema.json):

```json
{
  "schemaVersion": "1.0",
  "adapter": "mobilityhouse",
  "version": "1.6",
  "operation": "BootNotification",
  "ok": true,
  "messageId": "3c3d1f2e-...",
  "request": {"chargePointVendor": "OTFabric", "chargePointModel": "TestStation"},
  "response": {"status": "Accepted", "currentTime": "2026-10-18T10:00:00Z", "interval": 300},
  "callError": null,
  "error": null,
  "elapsedMs": 12
}
```

`request` is the payload as the upstream sent it (or, if it was not sent,
as given). `response` is the response as the upstream parsed it, written
back to JSON with the upstream's serialisation. `messageId` is left out when
the upstream does not expose it.

`callError` is `{code, description, details}` when the peer answered with a
CALLERROR. That is still a protocol success, so the exit code is 1 with
`error.code` `call-error`.

| `error.code` | Exit code | Meaning |
|---|---|---|
| `connect-failed` | 3 | Handshake failed, refused, or subprotocol not accepted |
| `timeout` | 1 | No response within `--timeout-ms` |
| `call-error` | 1 | The peer answered with a CALLERROR |
| `invalid-request` | 1 | The upstream refused the request payload before sending it |
| `invalid-response` | 1 | The upstream rejected the response |
| `connection-lost` | 1 | The connection closed while waiting |
| `not-connected` | 1 | `call`: no open connection for that station |
| `unsupported` | 5 | The upstream library has no such action in this version or role |
| `failed` | 1 | Anything else; `error.message` says what |

## `print-capabilities`

This command prints one JSON line that validates against
[schemas/capabilities.schema.json](../schemas/capabilities.schema.json). It
is described in [CAPABILITIES.md](CAPABILITIES.md). Capabilities are given
per version and per role.

## `print-fixture`

`print-fixture --version V [NAME]` writes the fixture `NAME` (default
`baseline`) baked into the image, byte for byte. A consumer reads the
expected values from the image it tests against, so that they cannot drift
from what the image serves. A fixture the image does not have exits 1.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | The operation failed (see `error.code`); `client run` lost its connection |
| 2 | Usage error |
| 3 | Connection failed: handshake, a port that cannot be bound, or no endpoint behind the control socket |
| 4 | Invalid fixture |
| 5 | Not supported by this adapter (version, role, security profile, action, or a fixture answer it cannot give) |

## Output rules

- stdout carries JSON only. The exceptions are `print-fixture`, which writes
  the fixture file, and `version`, which writes one line of text.
- stderr is free-form diagnostics, including the upstream library's logs.
  `OCPP_INTEROP_LOG_LEVEL` (for example `DEBUG`) makes them more verbose.
- Fields may be added within a major version. Consumers ignore fields they do
  not know.
- Every wait is bounded by a flag with a documented default.

## Gaps before 1.0

1.0 needs every adapter to implement all of the above. What is missing:

| Gap | Adapter |
|---|---|
| Compression (`features.compression`) | lorenzodonini: ocpp-go's `ws.Server` creates its WebSocket upgrader without compression and does not expose it, so the CSMS cannot accept it ([source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ws/websocket.go#L284-L320)); its station offers it with `--compression`. chargetime, OCPP 1.6: JSONServer and JSONClient have no extensions ([source](https://github.com/ChargeTimeEU/Java-OCA-OCPP/blob/f27fe5bfb03e01491100597d6859080f5ae07545/ocpp-v1_6-json/src/main/java/eu/chargetime/ocpp/JSONServer.java#L70)). chargetime, OCPP 2.0.1: declared, but Java-WebSocket 1.6.0 keeps its compression context against what it announces, in both roles (tests/known-bugs.tsv) |
| `compression` in `connection-opened` | chargetime: not exposed (`null` when it may have been negotiated). lorenzodonini: the station does not see it (`null` with `--compression`) |
| HTTP 401 for refused credentials and certificates | chargetime: Java-WebSocket 1.6.0 answers every handshake the adapter refuses with HTTP 404 ([source](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/WebSocketImpl.java#L426-L428)); the event has the right reason |
| OCPP 2.1 | chargetime: the pinned release (v2.0, the latest on Maven Central) has no 2.1; the upstream's master added it after that release. lorenzodonini: ocpp-go v0.19.0 has no 2.1 |
| An operation to send a SEND and to provoke a CALLRESULTERROR; the features `send` and `callResultError` | all: The Mobility House ocpp 2.1.0 and EVerest libocpp (everest-core 2026.02.1) know the message types 2 to 4 only; the other upstreams have no 2.1 in their pinned releases |
| `NotImplemented` for an OCPP 2.1 action the fixture does not answer | mobilityhouse: ocpp 2.1.0 does not answer such a CALL at all (tests/known-bugs.tsv) |
| `callError` answers from the fixture (`fixtureCallErrors`) | chargetime: Java-OCA-OCPP answers a failed handler with `InternalError` only |
| `NotImplemented` for a Core profile action the fixture does not answer | chargetime, OCPP 1.6: JSONServer and JSONClient always register the Core profile, so such an action reaches the adapter, which can only fail it (`InternalError`) |
| Message ids of sent CALLs (`messageIds`), subprotocol and close code in events | chargetime: not exposed by Java-OCA-OCPP. lorenzodonini: the station does not see the negotiated subprotocol, the server not the close code |
| `protocol-error` events | chargetime: the upstream logs unparsable frames without a hook |
| `connection-rejected` with reason `subprotocol` | chargetime: the 1.6 JSONServer also accepts a handshake without a subprotocol; mobilityhouse and lorenzodonini report it |
| HTTP 404 for an identity the fixture does not list | lorenzodonini: ocpp-go's connection check refuses with 401 only (the event's reason is right) |
| `--bind-address` other than `0.0.0.0` | lorenzodonini: ocpp-go's ws.Server listens on all addresses; another address exits 5 |
| `server` (a CSMS) | everest: libocpp implements the charging station only; `server` exits 5 after the usage checks |
| Close code in `connection-closed` | everest: libocpp's websocket does not pass it to the application |
| `--station ""` | everest: libocpp always appends the identity to the URL; exits 5 |

## Changes

- **0.5** (unreleased): transport security and compression, in every
  adapter. The section [Transport security](#transport-security) makes
  security profiles 2 (TLS and Basic authentication) and 3 (mutual TLS, the
  identity is the client certificate's CommonName) normative: the files of
  `--tls-dir`, PKCS #8 keys, RSA or ECDSA P-256, TLS 1.2 or later, host name
  verification by the station, `wss://` URLs for profiles 2 and 3 only.
  `--tls-dir` is required with profiles 2 and 3 (usage error otherwise). The
  section [Compression](#compression): the client switch `--compression`
  (the first flag without a value), a CSMS accepts permessage-deflate
  whenever it is offered, `features.compression` needs both roles. The
  optional field `compression` of `connection-opened`.
- **0.4** (unreleased): OCPP 2.1, in `mobilityhouse`. The section
  [OCPP 2.1: SEND and CALLRESULTERROR](#ocpp-21-send-and-callresulterror):
  without the `send` feature a SEND action is refused (exit 5), never sent as
  a CALL; a frame of type 5 or 6 the upstream cannot read is a
  `protocol-error`; a fixture answer to a SEND is invalid. No operation for
  SEND or CALLRESULTERROR yet: no pinned upstream release implements them.
- **0.3** (unreleased): OCPP 2.0.1, and the third adapter. The derived
  answer `variables` (GetVariables, SetVariables) is part of the answers. A
  `callError` answer whose code the upstream cannot send is refused (exit
  5). The gaps are listed per adapter and version.
- **0.2** (unreleased): first implementation. Added `--control-socket`, the
  `connection-failed` event, `remote` on `connection-rejected`, the error
  codes `invalid-request` and `not-connected`, the [Answers](#answers)
  rules, `null` for values an upstream does not expose, the exit codes of
  `client run`, `call` without an endpoint, and a port that cannot be bound.
  `call` takes `--station` before or after the action. The default command
  is `server --version 1.6`.
- **0.1**: Phase 0 design.
