# ocpp-interop — OCPP Interoperability Testing with Docker

[![CI](https://github.com/otfabric/ocpp-interop/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/ocpp-interop/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Independent OCPP implementations, packaged as containers that all speak the
same command-line contract. With them, any OCPP library can be tested
against real implementation diversity, in both roles and in both directions
of request flow, without owning any of that code.

> **Status: Phase 1 (foundations).** The container contract is a draft
> (0.1), and no adapter image is published yet. See [PLAN.md](PLAN.md).

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
| `mobilityhouse` | [mobilityhouse/ocpp](https://github.com/mobilityhouse/ocpp) | Python | 1.6J, 2.0.1 (2.1 candidate) | planned (P0) |
| `chargetime` | [ChargeTimeEU/Java-OCA-OCPP](https://github.com/ChargeTimeEU/Java-OCA-OCPP) | Java | 1.6J, 2.0.1, 2.1 | planned (P0) |
| `lorenzodonini` | [lorenzodonini/ocpp-go](https://github.com/lorenzodonini/ocpp-go) | Go | 1.6J, 2.0.1 | planned (P1) |

The versions listed are what each upstream advertises. A version or feature
becomes a capability only when a self-test exercises it. Pins are recorded
in [versions.yaml](versions.yaml).

## Using the images from a library's tests

Once images are published, a consumer pins them by digest and drives them
through the contract. For example, a library's CSMS can be tested with a
reference station:

```bash
docker run -d --name station ghcr.io/otfabric/ocpp-interop-mobilityhouse@sha256:... \
  client run --version 1.6 --url ws://host.docker.internal:9000/ocpp
docker exec station ocpp-interop call BootNotification \
  --payload "$(docker run --rm IMAGE print-fixture --version 1.6 | jq -c .samples.boot.payload)"
```

## Repository layout

```text
adapters/<name>/      adapter source and Dockerfile (from Phase 4)
fixtures/<version>/   deterministic fixtures, one directory per fixture
fixtures/schema/      the fixture schema
schemas/              capability, result and report document schemas
scripts/              validation and test scripts
tests/                known upstream defects (and, from Phase 4, the test cases)
docs/                 contract, capabilities, fixtures, compatibility
versions.yaml         every pin
```

## Development

```bash
make help        # list targets
make ci          # everything CI runs
make validate    # versions.yaml, schemas, fixtures
```

`make validate` needs Python 3 with `jsonschema`. If `jsonschema` is
missing, the target runs in a `python:3.12-slim` container instead.

## Security

The images are unauthenticated test fixtures by default. Run them on
private networks and publish ports on 127.0.0.1 only. See
[SECURITY.md](SECURITY.md).

## License

This repository's code is MIT ([LICENSE](LICENSE)). Each image carries the
licences of what it contains; see
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). OCA specification
documents and schemas are CC BY-ND 4.0 and are not part of this repository.
