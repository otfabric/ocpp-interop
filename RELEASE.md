# ocpp-interop Releases

## How a release is made

1. Set `interop_version` in versions.yaml to the new version, and add a
   `## vX.Y.Z` section to this file.
2. Push the tag `vX.Y.Z`. The `Release` workflow validates the repository,
   checks both of the above, and creates the GitHub release with
   `manifest.json` and `checksums.txt`.

The release also builds and self-tests the images natively on amd64 and
arm64. It then publishes per-architecture tags and a multi-arch index to
GHCR, and lists the index digests in the release. Consumers pin the
digests. Every adapter with a Dockerfile must be in the Makefile's
`ADAPTERS`, or the release fails.

The `Upstream candidates` workflow runs every Monday. It compares each pin
with the latest upstream release and keeps an issue open while a pin is
behind. It never changes a pin.

## Unreleased

Planned as v0.1.0 (`interop_version` in versions.yaml); the maintainer
decides when to release it.

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
