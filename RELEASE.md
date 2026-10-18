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

### Summary

Phases 0, 1 and the first slice of Phase 4: design, foundations, and the
first two adapters for OCPP 1.6J. Nothing is released yet.

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
