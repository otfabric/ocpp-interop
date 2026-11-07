# seanlee adapter

OCPP 1.6J, 2.0.1 and 2.1 reference CSMS and station built on
[seanlee0923/ocpp](https://github.com/seanlee0923/ocpp), behind the
[container contract](../../docs/CONTAINER_CONTRACT.md).

| | |
|---|---|
| Upstream | seanlee0923/ocpp (Go): its own OCPP-J framing (package `protocol`), message types and validation code generated from the OCA JSON schemas (`v16`, `v201`, `v21`), the CSMS (`csms`) and the station (`station`) |
| Version | v0.3.0, released 2026-08-13 (git tag `v0.3.0`, commit `7e3759d106605a11e4e3f1ce45fc44e6f595cd3c`), the Go module `github.com/seanlee0923/ocpp@v0.3.0` |
| Independence | Its own engine; the message types are generated from the OCA schemas by its own generator ([seanlee0923/ocpp-schema-gen](https://github.com/seanlee0923/ocpp-schema-gen)). No code of another reference here |
| Transport | [gorilla/websocket](https://github.com/gorilla/websocket) 1.5.3 |
| Other modules in the binary | google/uuid 1.6.0 (message ids). These are the versions seanlee0923/ocpp's go.mod requires |
| Licences | seanlee0923/ocpp MIT; gorilla/websocket BSD-2-Clause; google/uuid and the Go standard library BSD-3-Clause |
| Base images | `golang:1.27.2-trixie` (build), `debian:trixie-slim` (runtime), pinned by digest in the [Dockerfile](Dockerfile) |
| OCPP | 1.6J (without the Security Whitepaper messages), 2.0.1 and 2.1, both roles, security profiles 0 to 3. No compression, no SEND from either role, no CALLRESULTERROR (see below) |

The adapter code is MIT. `/licenses` in the image holds the licence files of
every module in the binary (listed in `/licenses/BUNDLED-MODULES.txt`), Go's
licence, this repository's licence and THIRD_PARTY_NOTICES.md. The OCA JSON
schemas are not in the binary: seanlee0923/ocpp ships the Go types and
validation code generated from them.

## Build

[go.mod](go.mod) pins seanlee0923/ocpp and, at the versions its go.mod
requires, every other module; [go.sum](go.sum) holds their checksums. A
release build runs `go mod download` and `go mod verify` and checks that the
commit the Go module proxy records for the version (`Origin.Hash`) is
`UPSTREAM_SHA`. It then regenerates [actions_gen.go](actions_gen.go)
(`go run ./gen`) and, at the pin, fails if the committed file differs; then
`gofmt`, `go vet` and `go build` with `CGO_ENABLED=0`. A candidate build
(`make candidate-seanlee CANDIDATE_VERSION=... CANDIDATE_SHA=...`) moves
the module to that version and builds with a fresh actions_gen.go.

```bash
docker build -f adapters/seanlee/Dockerfile -t ghcr.io/otfabric/ocpp-interop-seanlee:dev .
docker run --rm -p 127.0.0.1:9000:9000 ghcr.io/otfabric/ocpp-interop-seanlee:dev server --version 2.1
```

## How seanlee0923/ocpp is used

- **Typed calls and handlers**: the library's `csms.Call`, `station.Call`,
  `csms.Handle`, `csms.HandleSend` and `station.Handle` are generic over its
  generated payload types. The fixtures and the contract are JSON, so
  [actions_gen.go](actions_gen.go), written by [gen](gen/main.go) from the
  library's `v16`, `v201` and `v21` packages, instantiates them for every
  action it has types for (28, 64 and 91), and [ops.go](ops.go) turns a JSON
  request into the action's request type and the confirmation back into
  JSON. Every request and answer goes through those types and the generated
  validation.
- **`server`**: `csms.Server` (an `http.Handler`) for the one version of
  `--version`, served by the adapter's `http.Server` (TLS for profiles 2 and
  3). `csms.SecurityConfig` has the profile and an `Authenticator` that
  admits the stations of the fixture: an identity it does not list is
  refused, profiles 1 and 2 compare the Basic password (the library checks
  that the user name is the identity), profile 3 compares the CommonName of
  the verified client certificate with the identity. A router middleware
  gives the handlers the session's identity. `OnConnect` and `OnDisconnect`
  give `connection-opened` and `-closed`; the adapter writes
  `connection-rejected` for a refused handshake from the HTTP status.
- **`client`**: `station.Station` with one connection attempt (no
  `ReconnectPolicy`), `BasicCredentials` for profiles 1 and 2 and a
  `tls.Config` for 2 and 3. `call` and `client <Action>` use `station.Call`.
- **Answers**: a typed handler for every action the role receives answers
  from the fixture, a `callError` answer as a `csms.CallError` or
  `station.CallError` with the fixture's code. `NotImplemented` for an
  action without an answer. An action the role does not receive has no
  handler: the library answers it `NotImplemented`.
- **Events** come from the handlers and the calls the adapter makes. The
  library gives the message id of a CALL neither to its handler nor to the
  caller: `messageIds` is `false` and `messageId` is `null`.

## What it cannot do

- **Compression**: neither role negotiates permessage-deflate;
  `client --compression` exits 5.
- **SEND**: the CSMS takes the OCPP 2.1 SEND NotifyPeriodicEventStream
  (`csms.HandleSend`), but neither role can send one, and the station
  ignores one it receives. `send` is `false`.
- **CALLRESULTERROR**: the library has no message type 5; `callResultError`
  is `false`.
- **OCPP 1.6 Security Whitepaper messages** (SecurityEventNotification,
  SignCertificate, CertificateSigned, InstallCertificate, ...): the `v16`
  package has the 28 actions of OCPP 1.6 only. Not declared.
- **An unknown identity** is refused with HTTP 401 (403 at profile 3), not
  the 404 the contract asks for; the event has the right reason.
- **`--station ""`**: `station.Config` requires an identity and appends it
  to the URL; exits 5.
- **The close frame of the station**: `Station.Stop` closes the TCP
  connection without a close frame; `client run` reports `connection-closed`
  with `code` and `reason` `null`.
- **`protocol-error` events**: the library has no hook for a frame it cannot
  read.
- **Integers with a zero fraction**: the CSMS validates `20000.0` as an
  integer (draft-06) but cannot decode it into its Go `int`, and answers
  `FormatViolation`; the everest station sends NotifyEVChargingNeeds so in
  2.0.1 (tests/known-bugs.tsv,
  [seanlee0923/ocpp#2](https://github.com/seanlee0923/ocpp/issues/2)).
