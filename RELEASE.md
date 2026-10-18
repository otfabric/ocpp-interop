# ocpp-interop Releases

## How a release is made

From Phase 4 on, a tag `vX.Y.Z` builds and self-tests the images natively on
amd64 and arm64. It then publishes per-architecture tags and a multi-arch
index to GHCR. The GitHub release lists the index digests together with a
`manifest.json` of every pin. Consumers pin the digests.

## Unreleased

### Summary

Phase 0 and Phase 1: design and foundations. No images are published yet.

### Added

- Container contract, draft 0.1
- JSON schemas for capabilities, result documents, test reports and
  fixtures
- `baseline` fixtures for OCPP 1.6J and 2.0.1, with cross-field validation
- versions.yaml, listing the candidate upstreams
- CI: validation of schemas, fixtures, versions and scripts
