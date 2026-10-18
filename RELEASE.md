# ocpp-interop Releases

## How a release is made

1. Set `interop_version` in versions.yaml to the new version, and add a
   `## vX.Y.Z` section to this file.
2. Push the tag `vX.Y.Z`. The `Release` workflow validates the repository,
   checks both of the above, and creates the GitHub release with
   `manifest.json` and `checksums.txt`.

From Phase 4 on, the release also builds and self-tests the images natively
on amd64 and arm64. It then publishes per-architecture tags and a multi-arch
index to GHCR, and lists the index digests in the release. Consumers pin the
digests. Until then a release has no images, and the workflow refuses to
release if an adapter exists that it would leave out.

The `Upstream candidates` workflow runs every Monday. It compares each pin
with the latest upstream release and keeps an issue open while a pin is
behind. It never changes a pin.

## v0.0.1

**Date:** 2026-10-18

### Summary

Phase 0 and Phase 1: design and foundations. No images are published yet.

### Added

- Container contract, draft 0.1
- JSON schemas for capabilities, result documents, test reports and
  fixtures
- `baseline` fixtures for OCPP 1.6J and 2.0.1, with cross-field validation
- versions.yaml, listing the candidate upstreams
- CI: validation of schemas, fixtures, versions and scripts
- Release workflow (tag `vX.Y.Z`): validation, `manifest.json`, GitHub
  release
- Upstream candidate workflow (weekly): pin-versus-latest table and a
  tracking issue, for mobilityhouse (PyPI), chargetime and lorenzodonini
  (git tags)
