# ocpp-interop Releases

## How a release is made

1. `make release-prep VERSION=vX.Y.Z` turns "## Unreleased" into
   "## vX.Y.Z" with today's date and sets `interop_version` in versions.yaml.
   Commit it through a pull request.
2. `make release-check VERSION=vX.Y.Z` on the merged `main` runs the checks
   the Release workflow will run on the tag.
3. Tag that commit `vX.Y.Z` and push the tag. The `Release` workflow
   validates the repository, runs the same checks, and creates the GitHub
   release with `manifest.json` and `checksums.txt`.

The release also builds and self-tests the images natively on amd64 and
arm64. It then publishes per-architecture tags and a multi-arch index to
GHCR, and lists the index digests in the release. Consumers pin the
digests. Every adapter with a Dockerfile must be in the Makefile's
`ADAPTERS`, or the release fails.

The `Upstream candidates` workflow runs every Monday. It compares each pin
with the latest upstream release and keeps an issue open while a pin is
behind. It never changes a pin.

## v0.4.0

**Date:** 2026-10-19

**Previous release:** v0.3.1

### Summary

A fifth reference: `citrineos`, a CSMS built on CitrineOS core
(TypeScript) for OCPP 1.6J, 2.0.1 and 2.1, the second independent OCPP 2.1
CSMS, with security profiles 0 to 3 and compression (contract 0.5).

### Added

- `citrineos` adapter (JavaScript on Node.js 24.16.0): CitrineOS core
  pinned to release v2.0.1 (`1a83638c31eebdb1dd8d6cdb0f5b8a9f142a1372`),
  built from the tag as its own `deploy.Dockerfile` does (pnpm 10.19.0, the
  release's lockfile). `server` only (CitrineOS has no station; `client`
  exits 5). The image runs PostgreSQL 15 with PostGIS 3 and RabbitMQ 3.10
  (Debian bookworm), which CitrineOS needs, next to CitrineOS; the database
  is migrated at build time. CitrineOS answers every CALL itself: the
  adapter writes the fixture's stations, 1.6 connectors, Basic
  authentication passwords and id tokens into CitrineOS's database with its
  own repositories, and sets its heartbeat interval and boot status from the
  fixture. CSMS CALLs go through CitrineOS's REST message API with a
  callback URL; events come from CitrineOS's router and network connection.
  See adapters/citrineos/README.md
- Capability feature `fixtureAnswers` (contract 0.5): `false` for an
  upstream that answers by itself (citrineos), whose `handles` lists only
  the actions whose answer is the fixture's; not declared means `true`.
  `scripts/interop.sh` reports the from-station `NotImplemented` test
  against such a CSMS as SKIP_UNSUPPORTED
- `scripts/smoke.sh`: an adapter without a client runs its server checks
  (events, unknown identity, security profiles 2 and 3, compression) with the
  client of `SMOKE_PEER`, and the CSMS's CALLs to a `client run` of it
  (answer, `invalid-request`, `timeout`, `not-connected`)
- `scripts/latest-upstream.py citrineos` (git tags of citrineos-core), and
  `citrineos` in the candidate workflow (`ADAPTER_COUNT` 5)
- `compose.yaml`: the `citrineos` CSMS on 127.0.0.1:9003
- `tests/known-bugs.tsv`: CitrineOS's TLS server presents its certificate
  only to a client that sends SNI, which the Java-OCA-OCPP stations do not
  (`v16` and `v201` `tls.profile2`, `tls.profile3`, chargetime with
  citrineos); the ocpp-go `statusInfo: null` defect also with the citrineos
  CSMS, which validates every response

### Changed

- Fixtures (2.0.1 and 2.1 baselines): the blocked and expired id tokens are
  valid ISO14443 UIDs, 7 bytes in hexadecimal as OCPP 2.x defines the type:
  `04BLOCKED00001` is now `04B10C0ED00001`, `04EXPIRED00001` is now
  `04E0B1D0000001` (CitrineOS refused the old ones). The unknown token of the
  2.x `authorize-unknown` test is `04000000000001`. **Consumers** that copied
  these values rather than reading them with `print-fixture` must update
  them. The 1.6 fixture is unchanged
- CI: the self-test job may take 120 minutes (five adapters)

### Verification

`make ci VERSION=citrineos` with the five adapters on contract 0.5
(2026-10-19, linux/arm64): smoke 268 (mobilityhouse), 188 (chargetime, 1
known upstream bug), 187 (lorenzodonini), 147 (everest) and 242 (citrineos)
checks passed, 0 failed; interop 2196 PASS, 29 KNOWN_ISSUE, 558
SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR. With citrineos as the CSMS: 507
PASS, 9 KNOWN_ISSUE, 301 SKIP_UNSUPPORTED. OCPP 2.1 with citrineos: the
mobilityhouse station 29 / 41 PASS, the everest station 22 / 19 PASS
(station-initiated / CSMS-initiated), security profiles 2 and 3 with both.
The linux/amd64 image has not been built locally.

---

## v0.3.1

**Date:** 2026-10-18
**Previous release:** v0.3.0

### Summary

A fourth reference: `everest`, a station built on EVerest libocpp (C++) for
OCPP 1.6J, 2.0.1 and 2.1, the second independent OCPP 2.1 implementation,
with security profiles 0 to 3 (contract 0.5).

### Added

- `everest` adapter (C++): a station built on EVerest libocpp, pinned to
  everest-core release 2026.02.1 (`b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e`,
  libocpp 0.31.1 in `lib/everest/ocpp`; the standalone EVerest/libocpp
  repository is archived). OCPP 1.6J, 2.0.1 and 2.1, `client` only (libocpp
  has no CSMS; `server` exits 5), security profiles 0 to 3. It is
  the second independent OCPP 2.1 implementation in the self-tests. The
  adapter uses libocpp's protocol layer (`ocpp::Websocket` on libwebsockets,
  `ocpp::MessageQueue`, and libocpp's message structs for every payload),
  not its `ChargePoint`, which sends BootNotification and StatusNotification
  by itself; see adapters/everest/README.md. Security profiles 2 and 3 use
  libocpp's TLS with the files of `--tls-dir` (contract 0.5);
  `--compression` exits 5 (libocpp cannot offer permessage-deflate)
- `scripts/smoke.sh`: an adapter without a server runs its fixture checks
  with `client run` and its station checks against the server of
  `SMOKE_PEER` (default `mobilityhouse`); `server` and `client run` of an
  undeclared version are checked separately
- `scripts/latest-upstream.py everest` (git tags of everest-core), and
  `everest` in the candidate workflow
- `compose.yaml`: the `everest` station, profile `station`
- `tests/known-bugs.tsv`: libocpp's `ocpp::v2::MessageType` has no
  ReportDERControl (nor GetCertificateChainStatus, NotifyWebPaymentStarted),
  so its message queue cannot send it (`v21.from-station.report-dercontrol`)

### Changed

- CI: the self-test job may take 75 minutes (libocpp is compiled)

### Verification

`make ci` with the four adapters on contract 0.5 (2026-10-18, linux/arm64):
smoke 268 (mobilityhouse), 188 (chargetime, 1 known upstream bug), 187
(lorenzodonini) and 147 (everest) checks passed, 0 failed; interop 1689
PASS, 20 KNOWN_ISSUE, 257 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR. The
everest station passes security profiles 2 and 3 with every CSMS (1.6,
2.0.1; 2.1 with mobilityhouse).

---

## v0.3.0

**Date:** 2026-10-18
**Previous release:** v0.2.0

### Summary

Transport security and compression (PLAN.md, Phase 5): security profiles 2
and 3 in every adapter and every version it declares, and permessage-deflate
where the upstream can. The container contract is draft 0.5. The self-tests
generate a CA and certificates (RSA and ECDSA P-256) for every run.

Every action in a baseline sample: the adapters declare every action their
pinned upstream can exchange, so 39 of 39 OCPP 1.6 actions, 64 of 64 2.0.1
actions and 89 of 91 2.1 actions are now exchanged between the references
(before: 30, 27 and 43).

### Added

- Security profiles 2 (TLS and Basic authentication) and 3 (mutual TLS, the
  identity is the client certificate's CommonName) in `mobilityhouse`
  (websockets over Python's ssl), `chargetime` (the upstream's `enableWSS`,
  Java-WebSocket's `SSLParametersWebSocketServerFactory`) and
  `lorenzodonini` (ocpp-go's `ws.NewTLSServer` and `ws.NewTLSClient`), in
  both roles
- Compression: the client switch `--compression`; a CSMS accepts
  permessage-deflate whenever it is offered. Declared by `mobilityhouse`
  (1.6, 2.0.1, 2.1) and `chargetime` (2.0.1); `lorenzodonini`'s CSMS cannot
  accept it (ocpp-go does not expose its upgrader), and the chargetime 1.6
  classes have no extensions
- `connection-opened` reports `compression` (true, false, or null where the
  upstream does not expose it)
- `scripts/lib.sh`: `make_pki`, a throwaway CA, server and client
  certificates and the wrong ones (another identity, another CA) with
  OpenSSL; nothing is committed or baked into an image
- `scripts/smoke.sh`: per adapter and version, profiles 2 and 3 with RSA and
  ECDSA keys, what must not connect (wrong password, a client certificate of
  another identity or from another CA, none, a server certificate from
  another CA or for another host name, `ws://` to a TLS server), `client
  run` over TLS with CALLs from both ends, the CSMS's answer to a
  permessage-deflate offer, and a compressed session; usage errors of
  `--tls-dir`, `wss://` and `--compression`. Known upstream bugs of an
  adapter with itself are reported as such, and fail when they do not show
- `scripts/interop.sh`: the scenarios `<v>.tls.profile2`,
  `<v>.tls.profile3` and `<v>.compression` for every station and CSMS pair
- Every action the pinned upstreams support, declared from the direction
  tables in both roles (`NOT_SENT` / `notSent` for the exceptions), with an
  answer and a sample in the baseline fixtures of 1.6, 2.0.1 and 2.1;
  certificate data from a throwaway P-256 test PKI (no private keys)
- `validate-fixtures.py`: a fixture named `baseline` must have a sample of
  every action and an answer to every non-SEND action
- `scripts/interop.sh`: NotImplemented tested from both ends
  (`<v>.not-implemented.from-station`, `.from-csms`) with a fixture copy
  that leaves one action per side unanswered
- `tests/known-bugs.tsv`: ocpp-go v0.19.0 GetCompositeScheduleResponse does
  not have the OCA structure; its SetNetworkProfileResponse sends
  `"statusInfo": null`
- mobilityhouse: payload keys mapped to the dataclass's own field names
  (`iso_15118_*`), which fixes Get15118EVCertificate in 2.1
- `tests/known-bugs.tsv`: the chargetime 2.0.1 station keeps its
  compression context although it offers `client_no_context_takeover`
  (Java-WebSocket 1.6.0)

### Changed

- Container contract 0.5: the sections "Transport security" and
  "Compression"; `--tls-dir` is required with profiles 2 and 3; `wss://`
  with profiles 2 and 3 only; flags other than `--compression` keep the
  form `--name value`; the gaps before 1.0 for compression and for
  chargetime's HTTP 404 on refused handshakes
- The mobilityhouse station no longer offers permessage-deflate unless
  asked (`--compression`), so the chargetime 2.0.1 CSMS's context takeover
  defect now shows in `v201.compression` only, with any station that offers
  compression; its tests/known-bugs.tsv entry is no longer limited to the
  mobilityhouse station
- versions.yaml: `interop_version` 0.3.0, the next planned release;
  `contract_version` 0.5

---

## v0.2.0

**Date:** 2026-10-18
**Previous release:** v0.1.0

### Summary

OCPP 2.1 (PLAN.md, Phase 6) in `mobilityhouse`, the one adapter whose pinned
upstream release has it, with a 2.1 baseline fixture and self-tests. The
container contract is draft 0.4. Java-OCA-OCPP has 2.1 on its master only,
after the pinned v2.0 release; ocpp-go v0.19.0 has none. No pinned upstream
release implements the 2.1 SEND and CALLRESULTERROR frames, so no adapter
declares them.

### Added

- OCPP 2.1 in `mobilityhouse` (ocpp.v21 of ocpp 2.1.0), CSMS and station,
  security profiles 0 and 1, subprotocol `ocpp2.1`
- `fixtures/v21/baseline`: every 2.0.1 baseline sample and answer, and the
  2.1 additions tariffs (SetDefaultTariff, GetTariffs, ClearTariffs), DER
  control (SetDERControl, GetDERControl, ClearDERControl, NotifyDERAlarm,
  NotifyDERStartStop, ReportDERControl), periodic event streams
  (OpenPeriodicEventStream, ClosePeriodicEventStream, GetPeriodicEventStream,
  AdjustPeriodicEventStream, and NotifyPeriodicEventStream as a SEND) and
  battery swap (BatterySwap, RequestBatterySwap). Every payload validates
  against the OCA OCPP 2.1 Edition 2 JSON schemas
- `scripts/validate-fixtures.py`: the 2.1 action directions from OCPP 2.1
  Edition 2 Part 2; an answer to a SEND action is an error; id tokens of
  2.1 may have 255 characters
- `scripts/validate-payloads.py`: the schema of a SEND (`<Action>.json`)
- Self-tests for 2.1: `scripts/smoke.sh` runs its version checks for 2.1
  (new: a fixture that answers a SEND, a SEND action refused with exit 5,
  `print-fixture --version 2.1`); `scripts/interop.sh` runs 2.1 by default
  (`OCPP_VERSIONS` default `1.6 2.0.1 2.1`), with the test ids
  `v21.send.*` and `v21.rpc.call-result-error`, which run only where both
  ends declare `send` or `callResultError`
- `tests/known-bugs.tsv`: ocpp 2.1.0 does not answer a 2.1 CALL whose
  action has no handler (no NotImplemented; fixed on its master)

### Changed

- Container contract 0.4: `--version 2.1`; the section "OCPP 2.1: SEND and
  CALLRESULTERROR" (a SEND action is refused without the `send` feature,
  never sent as a CALL; frames of type 5 and 6 an upstream cannot read are
  `protocol-error`); a fixture answer to a SEND is invalid; the gaps for
  2.1 per adapter
- The fixture schema allows id tokens of up to 255 characters (the limit
  per version is a cross-field rule) and limits a token type to 20
- versions.yaml: `interop_version` 0.2.0, the next planned release;
  `mobilityhouse` verifies 2.1; the `chargetime` note says where its 2.1 is

---

## v0.1.0

**Date:** 2026-10-18

### Summary

OCPP 2.0.1 in every adapter, and the third adapter, `lorenzodonini` (Go,
lorenzodonini/ocpp-go v0.19.0), for 1.6J and 2.0.1 (PLAN.md, Phase 5). The
container contract is draft 0.3.

### Added

- Adapter `lorenzodonini`: ocpp-go v0.19.0 (Go module, pinned with go.mod
  and go.sum), OCPP 1.6J and 2.0.1, CSMS and station, security profiles 0
  and 1. It uses ocpp-go's high-level API on its ocppj endpoints and ws
  transport; compose.yaml runs its CSMS on 127.0.0.1:9002
- OCPP 2.0.1 in `mobilityhouse` (ocpp.v201) and `chargetime` (artifact
  `eu.chargetime.ocpp:ocpp2:2.0`, MultiProtocolJSONServer and
  MultiProtocolJSONClient; the dependency lock adds ocpp2 and jsr305)
- The derived answers of 2.0.1: `idTokenInfo` (a token matches by value
  and type) and `variables` for GetVariables and SetVariables, in every
  adapter
- The 2.0.1 baseline fixture: samples for every action it answers in both
  directions (NotifyReport, NotifyEvent, SecurityEventNotification,
  FirmwareStatusNotification, LogStatusNotification, DataTransfer,
  GetBaseReport, ChangeAvailability, ClearCache, UnlockConnector,
  ClearChargingProfile, GetLocalListVersion are new), and two it does not
  answer (ReservationStatusUpdate, CancelReservation)
- `scripts/validate-payloads.py`: checks the fixtures' samples and answers
  against a copy of the OCA JSON schemas (not in `make ci`, because the
  schemas are not in the repository)
- Self-tests per OCPP version: `scripts/smoke.sh` runs its version
  dependent checks for every version an image declares (new: an id token
  without a type, a CALLERROR code that is not an OCPP-J error code);
  `scripts/interop.sh` runs every station against every CSMS for 1.6 and
  2.0.1 (new for 2.0.1: unknown device model component and variable,
  case-insensitive names, a read-only variable). `OCPP_VERSIONS` limits a
  run to some versions
- `tests/known-bugs.tsv`: an entry may name the peer a defect shows with
  (`adapter:role@peer`). New entries: the chargetime 2.0.1 CSMS with a
  station that offers permessage-deflate (Java-WebSocket 1.6.0), and the
  lorenzodonini 2.0.1 station's `"statusInfo": null` in ResetResponse
- `make run-<adapter> RUN_OCPP=2.0.1`

### Changed

- Container contract 0.3: the `variables` derived answer is part of the
  answers; a `callError` answer whose code the upstream cannot send is
  refused with exit 5; the gaps before 1.0 are listed per adapter
- The capability documents declare 2.0.1 next to 1.6, in both roles
- `scripts/interop.sh`: a station of the CALLERROR test starts only once
  its CSMS listens (the Go station connected before the server was ready)
- smoke.sh compares the CSMS's Reset answer as interop.sh compares answers
  (every field of the fixture's answer), not for equality
- versions.yaml: `interop_version` 0.1.0, the next planned release;
  `lorenzodonini` built, with its base images

## v0.0.1

**Date:** 2026-10-18

### Summary

The first release: design, foundations, and the first two adapters for
OCPP 1.6J (Phases 0, 1 and the first slice of Phase 4). The container
contract is still a draft (0.2), so consumers should expect it to change
before 1.0.

### Added

- Container contract, draft 0.2: first implemented. Adds `--control-socket`,
  the `connection-failed` event, the error codes `invalid-request` and
  `not-connected`, the rules for answers, and lists the gaps before 1.0
- JSON schemas for capabilities, result documents, test reports and
  fixtures
- `baseline` fixtures for OCPP 1.6J and 2.0.1, with cross-field validation.
  The 1.6 fixture has a sample for every action it answers, in both
  directions, and two for actions it does not answer; every 1.6 sample and
  answer validates against the OCA 1.6 schemas
- Adapter `mobilityhouse`: The Mobility House ocpp 2.1.0 over websockets
  17.2, OCPP 1.6J, CSMS and station, security profiles 0 and 1
- Adapter `chargetime`: ChargeTimeEU Java-OCA-OCPP v2.0 from Maven Central,
  OCPP 1.6J, CSMS and station, security profiles 0 and 1
- Self-tests: `scripts/smoke.sh` (the contract, per image) and
  `scripts/interop.sh` (every station against every CSMS, CALLs from both
  ends), report documents, `scripts/compat-matrix.py`
- `tests/known-bugs.tsv`: SecurityEventNotification without `techInfo` in
  mobilityhouse 2.1.0. An entry may name the role it applies to
- Makefile targets for images, smoke, interop, documents, candidates and
  multi-arch publishing; compose.yaml with both CSMSs on 127.0.0.1
- versions.yaml, listing the pins of the built adapters and the candidate
  upstreams
- CI: validation, and a native self-test job per architecture (amd64,
  arm64)
- Release workflow (tag `vX.Y.Z`): native per-architecture build, self-test
  and GHCR push, a multi-arch manifest, `manifest.json`, GitHub release
- Upstream candidate workflow (weekly): pin-versus-latest table, a tracking
  issue, and a candidate build with self-tests for built adapters

### Changed

- The station password of the baseline fixtures is now
  `OTFabricTestPass0001` (20 characters): Java-OCA-OCPP's 1.6 server
  accepts Basic passwords of 16 to 20 bytes only
