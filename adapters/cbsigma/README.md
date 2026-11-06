# cbsigma adapter

OCPP 1.6J, 2.0.1 and 2.1 reference CSMS and station built on
[cB-Sigma/ocpp](https://github.com/cB-Sigma/ocpp), behind the
[container contract](../../docs/CONTAINER_CONTRACT.md). It is the first
reference of this repository with the OCPP 2.1 message types SEND and
CALLRESULTERROR.

| | |
|---|---|
| Upstream | cB-Sigma/ocpp (Go), its own protocol engine: the RPC connection, schema validation and the `csms` and `chargepoint` roles |
| Version | v0.2.0, released 2026-10-11 (git tag `v0.2.0`, commit `5af4302fb9830c4dfbb6fea048ff62a6c3bbbb2d`), the Go module `github.com/cB-Sigma/ocpp@v0.2.0` |
| Independence | Written for SigmaM2M from the published OCPP documents; its [PROVENANCE.md](https://github.com/cB-Sigma/ocpp/blob/5af4302fb9830c4dfbb6fea048ff62a6c3bbbb2d/PROVENANCE.md) says no code was ported from another implementation |
| Transport | [coder/websocket](https://github.com/coder/websocket) 1.8.15, through cB-Sigma's `ocpp.Conn` |
| Other modules in the binary | cB-Sigma/jsonschema 0.3.0 with santhosh-tekuri/jsonschema 6.0.3 (schema validation of the embedded OCA schemas), go-jose 4.1.4 (signed messages), golang.org/x/net and x/text. These are the versions cB-Sigma's go.mod requires |
| Licences | cB-Sigma/ocpp and cB-Sigma/jsonschema Apache-2.0 (with NOTICE files); coder/websocket ISC; santhosh-tekuri/jsonschema Apache-2.0; go-jose Apache-2.0; golang.org/x and the Go standard library BSD-3-Clause |
| Base images | `golang:1.27.2-trixie` (build), `debian:trixie-slim` (runtime), pinned by digest in the [Dockerfile](Dockerfile) |
| OCPP | 1.6J, 2.0.1 and 2.1, both roles, security profiles 0 to 3; in 2.1 SEND and CALLRESULTERROR. No compression (see below) |

A young project: v0.2.0 is its second commit, and its README says its
CALLRESULTERROR has not been tested against another implementation. Its own
`interop` module runs mobilityhouse/ocpp 2.1.0, ocpp-go v0.19.0 and
seanlee0923/ocpp v0.3.0 against it.

The adapter code is MIT. `/licenses` in the image holds the licence and
NOTICE files of every module in the binary (listed in
`/licenses/BUNDLED-MODULES.txt`), Go's licence, this repository's licence
and THIRD_PARTY_NOTICES.md.

## Build

[go.mod](go.mod) pins cB-Sigma/ocpp and, at the versions its go.mod
requires, every other module; [go.sum](go.sum) holds their checksums. A
release build runs `go mod download` and `go mod verify`, then `gofmt`,
`go vet` and `go build` with `CGO_ENABLED=0`, and checks that the commit the
Go module proxy records for the version (`Origin.Hash`) is `UPSTREAM_SHA`. A
candidate build (`make candidate-cbsigma CANDIDATE_VERSION=...
CANDIDATE_SHA=...`) moves the module to that version.

```bash
docker build -f adapters/cbsigma/Dockerfile -t ghcr.io/otfabric/ocpp-interop-cbsigma:dev .
docker run --rm -p 127.0.0.1:9000:9000 ghcr.io/otfabric/ocpp-interop-cbsigma:dev server --version 2.1
```

## How cB-Sigma is used

- **`server`**: `csms.Server` for the one version of `--version`, behind a
  plain `http.Server` (TLS for profiles 2 and 3 from `--tls-dir`). A handler
  for every action of the version (`HandleVersion`) answers from the
  fixture. `csms.Config.Authenticate` admits the stations of the fixture:
  Basic authentication with `ocpp.BasicAuthPassword` and `ocpp.PasswordOK`
  (profiles 1 and 2), the client certificate with `ocpp.ClientCertNames`
  (profile 3). `call` uses `Station.Call` and `Station.Send`.
- **`client`**: `chargepoint.Client` with `Connect`, the same handlers
  (`HandleVersion`), `Password` and `TLS`. `call` and `client <Action>` use
  `Client.Call` and `Client.Send`.
- **Validation**: cB-Sigma checks every request and answer against the OCA
  schemas it embeds. A request it refuses never reaches the handler (no
  `call-received`). A request the adapter sends is checked first with
  `ocpp.ValidateRequest` (`invalid-request`).
- **Answers**: the fixture's payload, or for a `callError` answer an
  `ocpp.RPCError` of the kind whose code it is (`ErrKind.Code`).
  `NotImplemented` for an action without an answer, `NotSupported` for an
  action the role does not receive.
- **Events** come from the handlers (`ocpp.MessageID`, `ocpp.IsSend`), from
  the calls the adapter makes (it gives each CALL its message id with
  `ocpp.WithMessageID`), from `csms.Config.OnConnect` and `OnDisconnect`, and
  from `OnResultError`. cB-Sigma has no frame hook.

### OCPP 2.1 SEND and CALLRESULTERROR

- **SEND**: NotifyPeriodicEventStream is sent with `Station.Send` or
  `Client.Send` and reaches the other end's handler with `ocpp.IsSend`; it is
  never answered (`send-sent`, `send-received`). cB-Sigma picks the message
  id of a SEND itself and does not say which: `messageId` is `null` on
  `send-sent` and left out of the result document.
- **CALLRESULTERROR**: cB-Sigma answers a CALLRESULT outside the schema with
  a CALLRESULTERROR (`Conn.CheckResult`) and returns an `ocpp.ResultError`:
  the result document has `error.code` `invalid-response` and
  `callResultError`. `OnResultError` reports one received
  (`call-result-error`).
- **Invalid answers**: cB-Sigma also checks the answers it sends, so with a
  fixture that has `invalidAnswers` for this end the adapter turns its
  checks off (`SkipValidation`) and checks requests itself
  (`ocpp.ValidateRequest`). That end then does not answer an invalid
  CALLRESULT with a CALLRESULTERROR; the self-tests give the
  `callresulterror` fixture to one end at a time.

## What it cannot do

- **Compression**: `csms.Config.AcceptOptions` accepts permessage-deflate,
  but cB-Sigma does not say whether it was negotiated (`compression` `null`
  in `connection-opened` when the station offers it), and
  `chargepoint.Config` cannot offer it: `client --compression` exits 5, and
  the feature `compression` is `false`.
- **HTTP 404** for an identity the fixture does not list: cB-Sigma answers
  every refusal of `Authenticate` with HTTP 401. The `connection-rejected`
  event has the reason `unknown-station`.
- **`--station ""`**: `chargepoint` appends the identity to the URL; exits 5.
- **`protocol-error` events**: cB-Sigma has no hook for a frame it cannot
  read.
