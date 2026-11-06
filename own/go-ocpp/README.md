# ocpp-interop-otfabric

[go-ocpp](https://github.com/otfabric/go-ocpp), OT Fabric's own OCPP stack,
behind the [container contract](../../docs/CONTAINER_CONTRACT.md), so that
other stacks can test against it: a CSMS (`server`) and a charging station
(`client`) for OCPP 1.6J, 2.0.1 and 2.1. Anything that uses an
`ocpp-interop-<adapter>` image can use this one in its place. Unlike those, it
is not a reference: its tags are go-ocpp versions, and no harness release or
`manifest.json` contains it.

**For testing only.** Its CSMS accepts the stations of the fixture with the
fixture's passwords, and the fixtures are public. Do not expose it to a network
you do not control. Old images stay available, so check
[go-ocpp advisories](https://github.com/otfabric/go-ocpp/security/advisories)
for the version you pull.

**Not an independent implementation.** The image declares
`upstream.independentEngine: false`, and `results.json` records
`independent: false`. A result against it says nothing independent about
go-ocpp when OT Fabric produces it. A result you publish, naming the
implementation and version, is independent evidence, and we would like to hear
about it.

## Use

```sh
docker run -d --name go-ocpp -p 127.0.0.1:9000:9000 ghcr.io/otfabric/ocpp-interop-otfabric:<tag> server --version 2.0.1
until docker exec go-ocpp test -f /run/ocpp-interop/ready; do sleep 0.1; done
# your station connects to ws://127.0.0.1:9000/ocpp/<identity of the fixture>;
# the CSMS sends a CALL to it with:
docker exec go-ocpp ocpp-interop call --station <identity> Reset --payload '{"type":"Immediate"}'
```

A station that connects to your CSMS:

```sh
docker run -d --name go-ocpp-station ghcr.io/otfabric/ocpp-interop-otfabric:<tag> \
  client run --version 2.0.1 --url ws://<your CSMS>/ocpp
docker exec go-ocpp-station ocpp-interop call BootNotification --payload "$(docker run --rm \
  ghcr.io/otfabric/ocpp-interop-otfabric:<tag> print-fixture --version 2.0.1 | jq -c .samples.boot.payload)"
```

`<tag>` is a go-ocpp version such as `v0.2.0`, or `latest`, the newest one
published. A rebuild of a version replaces its tag, so pin the digest if you need
an exact build. The digest is in the summary of the
[workflow run](https://github.com/otfabric/ocpp-interop/actions/workflows/own-go-ocpp.yml),
and `docker run --rm <image> version` prints the versions.

| | |
|---|---|
| Upstream | go-ocpp, the latest release when the image was built |
| Licence | MIT; every module's licence is in the image under `/licenses` |
| Adapter name | `otfabric` (the name in documents, as the schema allows) |
| Language | Go |
| Protocol engine | OT Fabric's own |
| OCPP | 1.6J, 2.0.1 and 2.1, both roles, security profiles 0 to 3, compression |

## How go-ocpp is used

Only its public API: `server.Server` (the CSMS) and `client.Client` (the
station), one OCPP version per process. Both take a raw-payload dispatcher
(`ocpp.DispatcherFunc`) that does what go-ocpp's typed dispatchers do: it
decodes and validates a request with the version package's `ValidateRequest`
(an invalid one is answered with the CALLERROR of its `ValidationError`), and
answers from the fixture after checking the answer with `ValidateResponse`.
An action of the version that the role does not receive is answered
`NotSupported`, an action the fixture does not answer or go-ocpp does not know
`NotImplemented`. A CALL from `client <Action>` or `call` is checked with
`ValidateRequest` (`invalid-request`) and sent with `Call` as raw JSON; its
response is checked with `ValidateResponse` (`invalid-response`). Which role
sends which action is the `Actions` table of `ocpp16`, `ocpp201` and `ocpp21`.

The events come from go-ocpp's frame observer (`WithFrameObserver`), the
message ids from a call interceptor (`ocpp.CallInfo.MessageID`), protocol
errors from `Metrics.OnProtocolError`, a lost connection from
`Metrics.OnDisconnect` and the session handler. The CSMS's authenticator
refuses an identity the fixture does not list (`server.ErrUnknownStation`, HTTP
404), and wrong Basic credentials or a client certificate of another identity
(HTTP 401). TLS (profiles 2 and 3) is `server.WithTLS` and `client.WithTLS`
with the files of `--tls-dir`. `OCPP_INTEROP_LOG_LEVEL` (`debug`, `info`,
`warn`, `error`) passes go-ocpp's `slog` logging to stderr.

## Why it is apart from the other adapters

`REQUIREMENTS.md` R1: nothing under `adapters/`, `scripts/`, `tests/` or the
top-level `Makefile` knows a library under test, and CI enforces that. This
directory and `.github/workflows/own-go-ocpp.yml` are the one exception. The
scripts take the image through `ADAPTER_IMAGE_otfabric`, and
`scripts/interop.sh` runs only the pairings with an image that declares
`independentEngine: false`: this stack against the references.

No release loop can form (`otfabric/engineering` ADR-0012). This image is built
by hand, from the latest go-ocpp release, and nothing is committed: this
repository has no pin of go-ocpp to bump. No release or required check of
go-ocpp needs the image, and go-ocpp commits no pin of it. It is not in a
harness release or `manifest.json`.

## Publishing

Run the **Own stack (go-ocpp)** workflow by hand, from `main`. It finds the
latest go-ocpp release and the latest harness release, and prints them. It
builds the image, runs the smoke test and the cross-stack self-test against the
references of that harness release on both architectures, then publishes
`:<go-ocpp version>` and `:latest`. Run from a branch, it stops before
publishing. Nothing runs it automatically, so after a go-ocpp release the image
stays on the old version until someone does.

```sh
make -f own/go-ocpp/Makefile image smoke interop    # from the repository root
```

## What it cannot do

`send` and `callResultError` are declared in OCPP 2.1 (contract 0.6,
[OCPP 2.1: SEND and CALLRESULTERROR](../../docs/CONTAINER_CONTRACT.md#ocpp-21-send-and-callresulterror)):
a SEND action (NotifyPeriodicEventStream) is sent with `Client.Send` or
`Session.Send` and received through `ocpp.SendDispatcher`; a response is read
into a type that validates itself with `ValidateResponse`, so go-ocpp answers
an invalid one with a CALLRESULTERROR; an `invalidAnswers` entry of the
fixture is sent as it is. The independent peer is the `cbsigma` reference
(cB-Sigma/ocpp); the other references know the message types 2 to 4 only, or
have no OCPP 2.1.

`compression` in `connection-opened` is what the handshake negotiated:
go-ocpp's `ocpp.ConnInfo.Compression` and `server.Session.Compression` (from
go-ocpp v0.3.0). Its CSMS accepts permessage-deflate whenever a station offers
it, and its station offers it with `--compression` (`client.WithCompression`).

Reported as `null`, because go-ocpp does not expose them:

- `code` and `reason` of `connection-closed` when the connection ended without
  a close frame, or by a local close of the CSMS.
