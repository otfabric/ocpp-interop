# lorenzodonini adapter

OCPP 1.6J and 2.0.1 reference CSMS and station built on
[lorenzodonini/ocpp-go](https://github.com/lorenzodonini/ocpp-go), behind the
[container contract](../../docs/CONTAINER_CONTRACT.md).

| | |
|---|---|
| Upstream | lorenzodonini ocpp-go (Go), its own protocol engine |
| Version | v0.19.0, released 2025-02-02 (git tag `v0.19.0`, commit `a1eec917af884db0585752f909a61893aa667480`), the Go module `github.com/lorenzodonini/ocpp-go@v0.19.0` |
| Transport | ocpp-go's `ws` package, on [gorilla/websocket](https://github.com/gorilla/websocket) 1.5.3 and [gorilla/mux](https://github.com/gorilla/mux) 1.8.1 |
| Other modules in the binary | go-playground/validator.v9 9.30.0 (ocpp-go's message validation) with go-playground/locales 0.12.1, go-playground/universal-translator 0.16.0 and leodido/go-urn 1.1.0; relvacode/iso8601 1.6.0 (date-time parsing). These are the versions ocpp-go's go.mod requires |
| Licences | ocpp-go MIT; gorilla/websocket BSD-2-Clause; gorilla/mux BSD-3-Clause; the go-playground modules, go-urn and iso8601 MIT; the Go standard library BSD-3-Clause |
| Base images | `golang:1.27.2-trixie` (build), `debian:trixie-slim` (runtime), pinned by digest in the [Dockerfile](Dockerfile) |
| OCPP | 1.6J and 2.0.1, both roles, security profiles 0 to 3. No compression (see below). ocpp-go v0.19.0 has no OCPP 2.1 (`--version 2.1` exits 5) |

The adapter code is MIT. `/licenses` in the image holds the licence files of
every module in the binary (listed in `/licenses/BUNDLED-MODULES.txt`), Go's
licence, this repository's licence and THIRD_PARTY_NOTICES.md. go-urn 1.1.0
ships no licence file; [licenses/](licenses) has the MIT text the project
added with its next release.

## Build

[go.mod](go.mod) pins ocpp-go and, at the versions ocpp-go's own go.mod
requires, every other module; [go.sum](go.sum) holds their checksums. A
release build runs `go mod download` and `go mod verify`, then `gofmt`,
`go vet` and `go build` with `CGO_ENABLED=0`, and checks that the commit
the Go module proxy records for the version (`Origin.Hash`) is
`UPSTREAM_SHA`. A candidate build (`make candidate-lorenzodonini
CANDIDATE_VERSION=vX.Y.Z CANDIDATE_SHA=<commit>`) moves ocpp-go to that
version with `go get` (checked against the Go checksum database) and builds
the same way.

`OCPP_INTEROP_LOG_LEVEL=info` or `debug` passes ocpp-go's own logging
(`ocppj.SetLogger`, `ws.SetLogger`) to stderr; by default ocpp-go logs
nothing.

## How the upstream is used

The adapter uses ocpp-go's high-level API, as an application would:
`ocpp16.CentralSystem` and `ocpp16.ChargePoint` for 1.6, `ocpp2.CSMS` and
`ocpp2.ChargingStation` for 2.0.1. It creates their `ocppj` endpoints itself,
with the feature profiles and the dispatchers ocpp-go creates by default, so
that it can:

- register a handler for every feature profile. Each handler method
  ([handlers.go](handlers.go), one per action of ocpp-go's `*Handler`
  interfaces) answers from the fixture: the answer becomes ocpp-go's response
  type with `encoding/json` and ocpp-go's struct tags, as ocpp-go parses a
  message. An action without a fixture answer returns
  `ocpp.NewHandlerError(NotImplemented, ...)`, a `callError` answer the
  error with that code; ocpp-go validates and sends the response or turns
  the error into a CALLERROR;
- observe the frames ocpp-go's `ws.Server` and `ws.Client` read and write,
  for the events, by wrapping them (`ws.WsServer` and `ws.WsClient` are
  interfaces), and report what ocpp-go's invalid message hook sees as
  `protocol-error`;
- read the message id ocpp-go generates for a CALL, with its default
  generator installed through `ocppj.SetMessageIdGenerator`;
- set the dispatcher's request timeout to the CALL's `--timeout-ms`.

Sending: the payload becomes ocpp-go's request type the same way, and goes
out with `SendRequestAsync`, which validates it first (`invalid-request`
when that fails). The response is the response type ocpp-go parsed,
written back with `encoding/json`.

Server: `ws.Server` on the port, serving every path; ocpp-go takes the
station identity from the last path segment. The adapter's
`CheckClientHandler` refuses identities the fixture does not list and, with
profiles 1 and 2, wrong Basic credentials. Client: `ws.Client` with the
handshake timeout of `--connect-timeout-ms`; ocpp-go appends the identity to
the URL.

## Transport security and compression

Security profiles 2 and 3: `ws.NewTLSServer` and `ws.NewTLSClient`, with a
`crypto/tls` configuration ([tls.go](tls.go)), TLS 1.2 or later. The server
serves `server.crt` and `server.key` and, with profile 3, requires a client
certificate that verifies against `ca.crt` (`RequireAndVerifyClientCert`);
the connection check compares its subject CommonName with the identity (401
when it differs). The station trusts `ca.crt` only and presents
`client.crt` with profile 3; gorilla/websocket's dialer sets the server name
from the URL, so Go checks the server's host name. A client certificate that
does not verify fails in the TLS handshake, before ocpp-go sees a request:
no `connection-rejected` event.

Compression: the station offers permessage-deflate with `--compression`
(gorilla/websocket's `Dialer.EnableCompression`, set with
`ws.Client.AddOption`), but the CSMS cannot accept it: `ws.NewServer` and
`ws.NewTLSServer` create gorilla's `Upgrader` without `EnableCompression`
and keep it unexported
([source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ws/websocket.go#L284-L320)).
So the adapter does not declare `compression`, and the cross-stack tests do
not run its station with it. `ws.Client` does not expose whether compression
was negotiated: the station's `connection-opened` has `compression` `null`
with `--compression` (`false` without); the server's is always `false`.

## Actions

ocpp-go v0.19.0 has a feature, with request and response types, for every
OCPP 1.6 action (39: Core, Firmware Management, Local Auth List, Remote
Trigger, Reservation, Smart Charging and the Security Whitepaper profiles
logging, security, extendedtriggermessage, certificates and securefirmware)
and every OCPP 2.0.1 action (64, in 16 functional blocks), and the adapter
registers a handler for each. It therefore declares every action of both
versions in the direction the specification gives it, both roles; the
baseline fixtures have a sample and an answer for each.

## Limitations of the upstream

These are listed in the `notes` of the capabilities and in
[the contract's gaps](../../docs/CONTAINER_CONTRACT.md#gaps-before-10):

- ocpp-go's connection check can refuse a handshake with HTTP 401 only: an
  unknown identity gets 401, not 404 (the `connection-rejected` event has
  the right reason).
- `ws.Server.Start` takes a port and listens on all addresses:
  `--bind-address` other than `0.0.0.0` exits 5. A port it cannot bind is
  reported only in its log; the adapter checks the port first.
- The station does not see the negotiated subprotocol, and ocpp-go does not
  check that the server accepted it. The server does not see the close code
  of a connection; the station sees it only for a connection the CSMS
  closes.
- After a lost connection the station would reconnect by itself; the
  contract's `client run` reports the loss and exits 1 instead.
- No compression at the CSMS (above).

## Known upstream defects

Recorded in [tests/known-bugs.tsv](../../tests/known-bugs.tsv), with the
evidence:

- OCPP 2.0.1 `ResetResponse` and `SetNetworkProfileResponse` declare
  `statusInfo` without `omitempty`, so the station sends
  `"statusInfo": null`, which the OCA schema does not allow: a CSMS that
  validates responses (mobilityhouse) rejects the answer.
- OCPP 2.0.1 `GetCompositeScheduleResponse` does not have the OCA structure
  ([source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/smartcharging/get_composite_schedule.go#L31-L50)):
  a top-level `evseId`, always written, which the schema does not allow; no
  `statusInfo`; and a `schedule` of `{startDateTime, chargingSchedule}`
  where the schema has `CompositeScheduleType` `{evseId, duration,
  scheduleStart, chargingRateUnit, chargingSchedulePeriod}`. The station
  cannot answer with a schedule (mobilityhouse rejects the answer, the
  chargetime CSMS never completes the CALL, which times out, and ocpp-go
  reads an empty schedule), and the CSMS
  drops the schedule of a correct answer. GetCompositeSchedule stays
  declared: the request goes out and an answer comes back; the test is
  KNOWN_ISSUE with every peer.
- OCPP 2.0.1 `NotifyReportRequest` declares `tbc` as a `bool` with
  `omitempty`, so the station cannot send an explicit `"tbc": false`; the
  field is left out (false is its default).
