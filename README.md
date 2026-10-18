# ocpp-interop — OCPP Interoperability Testing with Docker

[![CI](https://github.com/otfabric/ocpp-interop/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/ocpp-interop/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Independent OCPP implementations, packaged as containers that all speak the
same command-line contract. With them, any OCPP library can be tested
against real implementation diversity, in both roles and in both directions
of request flow, without owning any of that code.

> **Status: Phase 4 (first adapters).** `mobilityhouse` and `chargetime`
> implement the container contract, draft 0.2, for OCPP 1.6J in both roles,
> and pass the self-tests against each other in both directions of request
> flow ([results](docs/COMPATIBILITY.md)). No image is published yet: there
> is no release. See [PLAN.md](PLAN.md).

## Table of contents

- [What this repository is, and is not](#what-this-repository-is-and-is-not)
- [Reference implementations](#reference-implementations)
- [Using the images from a library's tests](#using-the-images-from-a-librarys-tests)
- [Repository layout](#repository-layout)
- [Development](#development)
- [Security](#security)
- [License](#license)

## What this repository is, and is not

It **is**:

- adapters around established OCPP stacks;
- [one container contract](docs/CONTAINER_CONTRACT.md);
- deterministic [fixtures](docs/FIXTURES.md);
- [honest capability declarations](docs/CAPABILITIES.md);
- reference-to-reference self-tests and their [reports](schemas/report.schema.json).

It is **not**:

- an OCPP implementation;
- a charging management system;
- a test suite for any particular library;
- a substitute for the OCA Compliance Testing Tool (OCTT).

Interoperability between two implementations does not establish conformance
to the specification. Neither does it establish OCA certification.

## Reference implementations

| Adapter | Upstream | Language | OCPP versions to verify | Status |
|---|---|---|---|---|
| `mobilityhouse` | [mobilityhouse/ocpp](https://github.com/mobilityhouse/ocpp) 2.1.0 | Python | 1.6J, 2.0.1 (2.1 candidate) | **built: 1.6J, both roles**, security profiles 0 and 1 |
| `chargetime` | [ChargeTimeEU/Java-OCA-OCPP](https://github.com/ChargeTimeEU/Java-OCA-OCPP) v2.0 | Java | 1.6J, 2.0.1, 2.1 | **built: 1.6J, both roles**, security profiles 0 and 1 |
| `lorenzodonini` | [lorenzodonini/ocpp-go](https://github.com/lorenzodonini/ocpp-go) | Go | 1.6J, 2.0.1 | planned (P1) |

The versions listed are what each upstream advertises. A version or feature
becomes a capability only when a self-test exercises it
([docs/CAPABILITIES.md](docs/CAPABILITIES.md)). Pins are recorded in
[versions.yaml](versions.yaml); each adapter's README says how it uses its
upstream and what the upstream cannot do.

## Using the images from a library's tests

Once images are published, a consumer pins them by digest and drives them
through the [contract](docs/CONTAINER_CONTRACT.md). Until then, build them
with `make images` (tag `dev`). For example, a library's CSMS can be tested
with a reference station, with CALLs in both directions:

```bash
IMAGE=ghcr.io/otfabric/ocpp-interop-mobilityhouse:dev    # or @sha256:... once published
docker run -d --name station "$IMAGE" \
  client run --version 1.6 --url ws://host.docker.internal:9000/ocpp
# station -> CSMS
docker exec station ocpp-interop call BootNotification \
  --payload "$(docker run --rm "$IMAGE" print-fixture --version 1.6 | jq -c .samples.boot.payload)"
# the library's CSMS sends Reset; the station answers from the fixture and reports it:
docker logs station | jq -c 'select(.event == "call-received")'
```

And a library's station can be tested against a reference CSMS, which also
sends CALLs on request:

```bash
docker run -d --name csms -p 127.0.0.1:9000:9000 "$IMAGE" server --version 1.6
# ... the library's station connects to ws://127.0.0.1:9000/ocpp/OT-FABRIC-TEST-001
docker exec csms ocpp-interop call --station OT-FABRIC-TEST-001 Reset --payload '{"type":"Soft"}'
```

`docker compose up -d --wait` runs both reference CSMSs on 127.0.0.1:9000
(mobilityhouse) and 127.0.0.1:9001 (chargetime).

## Repository layout

```text
adapters/<name>/      adapter source, Dockerfile, lock file and README
fixtures/<version>/   deterministic fixtures, one directory per fixture
fixtures/schema/      the fixture schema
schemas/              capability, result and report document schemas
scripts/              validation, smoke and cross-stack self-tests, release helpers
tests/                known upstream defects
docs/                 contract, capabilities, fixtures, compatibility
versions.yaml         every pin
```

## Development

```bash
make help        # list targets
make ci          # everything CI runs: validate, lint, images, smoke, interop, document schemas
make validate    # versions.yaml, schemas, fixtures
make images      # build every adapter image for this machine
make smoke       # contract smoke test of each image
make interop     # every station against every CSMS, CALLs from both ends
make documents   # smoke and interop, keeping every document in reports/, validated
make matrix      # the compatibility matrix of reports/report-interop.json
```

`make validate` needs Python 3 with `jsonschema`. If `jsonschema` is
missing, the target runs in a `python:3.12-slim` container instead. The
self-tests need Docker and jq.

## Security

The images are unauthenticated test fixtures by default. Run them on
private networks and publish ports on 127.0.0.1 only. See
[SECURITY.md](SECURITY.md).

## License

This repository's code is MIT ([LICENSE](LICENSE)). Each image carries the
licences of what it contains; see
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). OCA specification
documents and schemas are CC BY-ND 4.0 and are not part of this repository.
