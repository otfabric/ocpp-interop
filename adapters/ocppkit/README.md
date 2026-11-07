# ocppkit adapter

OCPP 1.6J, 2.0.1 and 2.1 reference CSMS and station built on
[hupe1980/ocpp-kit](https://github.com/hupe1980/ocpp-kit), behind the
[container contract](../../docs/CONTAINER_CONTRACT.md). It is the second
reference with the OCPP 2.1 message types SEND and CALLRESULTERROR, the
first that offers and accepts permessage-deflate in both roles, and the
first with a Local Controller (`local-controller`, contract 0.8).

| | |
|---|---|
| Upstream | hupe1980/ocpp-kit (Rust): its own codec (typed messages generated from the OCA JSON schemas), sans-I/O protocol engine, RFC 6455 WebSocket and RFC 7692 compression, and the `transport::Station` and `transport::Csms` roles |
| Version | 0.3.0, released 2026-09-15 on crates.io (git tag `v0.3.0`, commit `b64ce83ab46c742dfbd99e925cd0acfacbf2ddc9`) |
| Independence | Its own engine and WebSocket; written from the OCPP documents its [SPEC_EDITIONS.md](https://github.com/hupe1980/ocpp-kit/blob/b64ce83ab46c742dfbd99e925cd0acfacbf2ddc9/SPEC_EDITIONS.md) lists. No code of another reference here |
| Transport | ocpp-kit's own WebSocket over tokio, TLS with rustls 0.23 (ring) |
| Other crates in the binary | tokio, rustls with ring and webpki-roots, serde and serde_json, and, for the adapter, jsonschema (the fixture schema), regex and x509-parser (the CommonName of a client certificate). `/licenses/BUNDLED-CRATES.txt` lists all of them |
| Licences | ocpp-kit MIT OR Apache-2.0; the other crates MIT, Apache-2.0, ISC, BSD-3-Clause, Unicode-3.0, Zlib or CDLA-Permissive-2.0 (webpki-roots), each as its Cargo.toml declares |
| Base images | `rust:1.99.1-slim-trixie` (build), `debian:trixie-slim` (runtime), pinned by digest in the [Dockerfile](Dockerfile) |
| OCPP | 1.6J, 2.0.1 and 2.1, both roles. CSMS: security profiles 0 to 3; station: 1 to 3 (see below). Compression in both roles; in 2.1 SEND and CALLRESULTERROR |

The adapter code is MIT. `/licenses` in the image holds the licence files of
every crate in the binary (two crates without one get a `LICENSE-DECLARED`
with the licence and authors their Cargo.toml declares), this repository's
licence and THIRD_PARTY_NOTICES.md. The OCA JSON schemas ocpp-kit's crate
vendors are used to generate its code; they are not in the binary.

## Build

[Cargo.toml](Cargo.toml) requires exactly ocpp-kit 0.3.0, and
[Cargo.lock](Cargo.lock) pins every crate by version and checksum. A release
build runs `cargo fetch --locked` and checks that the commit `cargo publish`
recorded in the crate (`.cargo_vcs_info.json`) is `UPSTREAM_SHA`, then
`cargo fmt --check`, `cargo clippy -D warnings` and `cargo build --release
--locked --offline`. A candidate build (`make candidate-ocppkit
CANDIDATE_VERSION=... CANDIDATE_SHA=...`) moves ocpp-kit to that version
(`cargo update --precise`).

```bash
docker build -f adapters/ocppkit/Dockerfile -t ghcr.io/otfabric/ocpp-interop-ocppkit:dev .
docker run --rm -p 127.0.0.1:9000:9000 ghcr.io/otfabric/ocpp-interop-ocppkit:dev server --version 2.1
```

## How ocpp-kit is used

- **`server`**: `transport::Csms` for the one version of `--version`, on a
  listener the adapter binds (`serve_on`), with `ServerTls` for profiles 2
  and 3. The authenticator admits the stations of the fixture: an identity
  it does not list is `AuthOutcome::Unknown` (HTTP 404); profiles 1 and 2
  compare the Basic password with `constant_time_eq`; profile 3 compares
  the CommonName of the verified client certificate with the identity.
  `SessionEvent` gives `connection-opened`, `-closed` and `-rejected`; `call`
  uses `CsmsHandle::session` and the session's `Handle`.
- **`client`**: `transport::Station` with one connection attempt
  (`reconnect(false)`), `SecurityProfile`, `BasicAuthPassword` and
  `ClientTls`. `call` and `client <Action>` use the station's `Handle`.
- **Engine**: no automatic Heartbeat (`HeartbeatPolicy::Manual`); every CALL
  of the station is sent as triggered (`CallOptions::triggered`), so that the
  boot state machine never holds back what a test sends; the CSMS boot gate
  is off (`enforce_boot_gate`). The deadline of a CALL is its `--timeout-ms`.
- **Validation**: ocpp-kit decodes every request into its typed messages and
  answers one it cannot decode with a CALLERROR before the handler runs (no
  `call-received`). A request the adapter sends is decoded first
  (`transcode_request`, `invalid-request`); an answer it receives is decoded
  (`transcode_response`, `invalid-response`).
- **Answers**: one `Handler` for every action answers from the fixture, a
  `callError` answer as a `CallError` with the fixture's code
  (`ErrorCode::parse`, the version's spelling). `NotImplemented` for an
  action without an answer, `NotSupported` for an action the role does not
  receive.
- **Events** come from the handler (`IncomingRequest`: message id, action,
  `MessageKind`), from the calls the adapter makes and from each session's
  event channel (`Event::ResultRejected`). ocpp-kit does not return the id of
  a CALL it sends: `messageIds` is `false`.

### Local Controller (contract 0.8)

`local-controller` runs ocpp-kit's `transport::LocalController` (OCPP 2.0.1
and 2.1 Part 4 chapter 6) on a listener the adapter binds: one CSMS
connection per station under the station's identity and path, every frame
forwarded unchanged, close propagation both ways, a server certificate
southbound (`--tls-dir`) and a client identity northbound
(`--upstream-tls-dir`). At profiles 1 and 2 the station's `Authorization`
header goes upstream (`UpstreamCredentials::PassThrough`); at 3 none
(`UpstreamCredentials::None`), the controller's client certificate. A
`Relay` that forwards every frame writes a `relayed` event for each; the
controller has no hook for a leg that opens or closes. Not in 1.6, which
has no Local Controller (exit 5).

### OCPP 2.1 SEND and CALLRESULTERROR

- **SEND**: NotifyPeriodicEventStream is sent with `Handle::call_raw` as a
  SEND and reaches the other end's handler as `MessageKind::Send`; it is
  never answered (`send-sent`, `send-received`).
- **CALLRESULTERROR**: a CALL of 2.1 is sent with its typed request
  (`Handle::call_with`); ocpp-kit answers a CALLRESULT it cannot decode with
  a CALLRESULTERROR, and the result document has `error.code`
  `invalid-response` and `callResultError`. A CALLRESULTERROR received is an
  `Event::ResultRejected` (`call-result-error`). With the `callresulterror`
  fixture the handler sends the invalid answer as it is.

## What it cannot do

- **Security profile 0 as a station**: ocpp-kit's station always sends
  credentials (profiles 1 to 3, as OCPP 2.0.1 and the 1.6 Security
  Whitepaper define them). The capabilities say so per role
  (`client.securityProfiles` `[1, 2, 3]`, contract 0.7): `client
  --security-profile 0` exits 5, and the self-tests run the pairings of this
  station with HTTP Basic authentication at both ends. At profiles 1 and 2 a
  `--station` the fixture does not list is sent with a password of the
  adapter's own, `OTFABRIC-NO-PASS` (16 characters, within the lengths OCPP
  1.6 and 2.x allow, so that a server refuses the identity, not the
  password's length).
- **`compression` in `connection-opened`**: ocpp-kit does not say whether
  permessage-deflate was negotiated; `null` when it was offered.
- **The close frame**: ocpp-kit reports why a session ended for logs only;
  `connection-closed` has `code` and `reason` `null`.
- **`--station ""`**: the station appends its identity to the URL; exits 5.
- **OCPP 2.1 NotifyWebPaymentStarted**: OCPP 2.1 Part 2 (use case C) has the
  CSMS send it; ocpp-kit 0.3.0 has it as a station message
  ([`Origin::ChargingStation`](https://github.com/hupe1980/ocpp-kit/blob/b64ce83ab46c742dfbd99e925cd0acfacbf2ddc9/src/v2_1/action.rs#L578)),
  so its CSMS refuses to send it and its station refuses it before the
  handler. Left out of the CSMS's `sends` and the station's `handles`
  ([hupe1980/ocpp-kit#1](https://github.com/hupe1980/ocpp-kit/issues/1)).
- **Integers with a zero fraction**: ocpp-kit reads the integer fields of
  the OCA types as `i32`, so it refuses `20000.0`, which the draft-06 OCA
  schemas allow. The everest station sends NotifyEVChargingNeeds that way in
  2.0.1 (tests/known-bugs.tsv,
  [hupe1980/ocpp-kit#2](https://github.com/hupe1980/ocpp-kit/issues/2)).
- **`protocol-error` events**: ocpp-kit has no hook for a frame it cannot
  read.
