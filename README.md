# ocpp-interop — OCPP Interoperability Testing with Docker

[![CI](https://github.com/otfabric/ocpp-interop/actions/workflows/ci.yml/badge.svg)](https://github.com/otfabric/ocpp-interop/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Independent OCPP implementations, packaged as containers that all speak the
same command-line contract. With them, any OCPP library can be tested
against real implementation diversity, in both roles and in both directions
of request flow, without owning any of that code.

> **Status: Phase 6 (OCPP 2.1).** `mobilityhouse`, `chargetime` and
> `lorenzodonini` implement the container contract, draft 0.5, for OCPP 1.6J
> and 2.0.1 in both roles, and `mobilityhouse` also for OCPP 2.1 (the pinned
> releases of the other two upstreams have no 2.1), with security profiles 0
> to 3 (TLS and mutual TLS) and, where the upstream can, compression. `everest` (EVerest
> libocpp) is a station for OCPP 1.6J, 2.0.1 and 2.1: the second
> independent OCPP 2.1 implementation. `citrineos` (CitrineOS) is a CSMS for
> the same three: the second independent OCPP 2.1 CSMS. They are self-tested
> against each other in both directions of request flow, with the upstream
> defects that shows recorded ([results](docs/COMPATIBILITY.md)). No pinned
> upstream release implements the OCPP 2.1 SEND and CALLRESULTERROR frames
> yet. See [PLAN.md](PLAN.md).

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
| `mobilityhouse` | [mobilityhouse/ocpp](https://github.com/mobilityhouse/ocpp) 2.1.0 | Python | 1.6J, 2.0.1, 2.1 | **built: 1.6J, 2.0.1 and 2.1, both roles**, security profiles 0 to 3, compression |
| `chargetime` | [ChargeTimeEU/Java-OCA-OCPP](https://github.com/ChargeTimeEU/Java-OCA-OCPP) v2.0 | Java | 1.6J, 2.0.1 (2.1 on master only, unreleased) | **built: 1.6J and 2.0.1, both roles**, security profiles 0 to 3, compression in 2.0.1 (known upstream defects) |
| `lorenzodonini` | [lorenzodonini/ocpp-go](https://github.com/lorenzodonini/ocpp-go) v0.19.0 | Go | 1.6J, 2.0.1 | **built: 1.6J and 2.0.1, both roles**, security profiles 0 to 3; no compression (its CSMS cannot accept it) |
| `everest` | [EVerest libocpp](https://github.com/EVerest/everest-core/tree/2026.02.1/lib/everest/ocpp), everest-core 2026.02.1 | C++ | 1.6J, 2.0.1, 2.1 (station only) | **built: 1.6J, 2.0.1 and 2.1, station (`client`) only**, security profiles 0 to 3; no compression (libocpp cannot offer it) |
| `citrineos` | [citrineos/citrineos-core](https://github.com/citrineos/citrineos-core) v2.0.1 | TypeScript | 1.6J, 2.0.1, 2.1 (CSMS only) | **built: 1.6J, 2.0.1 and 2.1, CSMS (`server`) only**, security profiles 0 to 3, compression; CitrineOS answers by itself (`fixtureAnswers: false`), with PostgreSQL and RabbitMQ in its image |

The versions listed are what each upstream advertises. A version or feature
becomes a capability only when a self-test exercises it
([docs/CAPABILITIES.md](docs/CAPABILITIES.md)). Pins are recorded in
[versions.yaml](versions.yaml); each adapter's README says how it uses its
upstream and what the upstream cannot do.

## Using the images from a library's tests

Each release publishes multi-arch images to GHCR
([RELEASE.md](RELEASE.md)). A consumer pins them by digest and drives them
through the [contract](docs/CONTAINER_CONTRACT.md); to test local changes,
build them with `make images` (tag `dev`). For example, a library's CSMS can
be tested with a reference station, with CALLs in both directions:

```bash
IMAGE=ghcr.io/otfabric/ocpp-interop-mobilityhouse@sha256:...    # or :dev for a local build
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

The same works with `--version 2.0.1` and the 2.0.1 fixture
(`print-fixture --version 2.0.1`), and, with the `mobilityhouse` image, with
`--version 2.1`. The `everest` image is a station only (`client run`,
`client <Action>`), for 1.6, 2.0.1 and 2.1. The `citrineos` image is a CSMS
only, for 1.6, 2.0.1 and 2.1; it answers as CitrineOS does, which is the
fixture's answer only for the actions it declares in `handles`.

Security profiles 2 and 3 run over TLS with a CA and certificates you
generate and mount read-only (`--tls-dir`; the files are listed in
[the contract](docs/CONTAINER_CONTRACT.md#transport-security)), and a
station offers compression with `--compression`:

```bash
docker run -d --name csms -p 127.0.0.1:9000:9000 -v "$PWD/tls:/tls:ro" "$IMAGE" \
  server --version 2.0.1 --security-profile 3 --tls-dir /tls/server
# ... the library's station connects to wss://localhost:9000/ocpp/OT-FABRIC-TEST-001
# with a client certificate whose CommonName is OT-FABRIC-TEST-001
```

`docker compose up -d --wait` runs the four reference CSMSs on
127.0.0.1:9000 (mobilityhouse), 127.0.0.1:9001 (chargetime),
127.0.0.1:9002 (lorenzodonini) and 127.0.0.1:9003 (citrineos), for OCPP 1.6 unless
`OCPP_INTEROP_OCPP=2.0.1 OCPP_INTEROP_FIXTURE_DIR=v201` is set. For OCPP 2.1
(`OCPP_INTEROP_OCPP=2.1 OCPP_INTEROP_FIXTURE_DIR=v21`), start the
`mobilityhouse` service, or `citrineos` (127.0.0.1:9003, which serves 1.6,
2.0.1 and 2.1 too): the other two exit 5. `everest` has no CSMS;
`docker compose --profile station up -d --wait mobilityhouse everest` runs
it as a station connected to the mobilityhouse CSMS.

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
self-tests need Docker, jq and OpenSSL (the TLS tests generate their CA and
certificates with it for every run). `OCPP_VERSIONS=2.1 make interop` limits the
cross-stack self-test to one OCPP version (default: 1.6, 2.0.1 and 2.1), and
`INTEROP_PAIRS="citrineos@2.1" REPORT_DIR=reports make interop` to the
pairings of one CSMS and version, as each CI job runs them
(`scripts/merge-reports.py` joins the reports of such runs).

## Security

The images are unauthenticated test fixtures by default. Run them on
private networks and publish ports on 127.0.0.1 only. See
[SECURITY.md](SECURITY.md).

## License

This repository's code is MIT ([LICENSE](LICENSE)). Each image carries the
licences of what it contains; see
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). OCA specification
documents and schemas are CC BY-ND 4.0 and are not part of this repository.
