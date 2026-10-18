# Requirements

This file says what this repository must and must not do.
[PLAN.md](PLAN.md) gives the order of the work.
[docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md) and
[docs/FIXTURES.md](docs/FIXTURES.md) are the specifications that follow from
these requirements.

## Purpose

This repository provides independent, reproducible interoperability tests
for implementations of the Open Charge Point Protocol (OCPP). It packages
established third-party OCPP implementations as containers that all behave
the same way. Any OCPP library, in any language, can then test itself
against real implementation diversity without owning any of it.

It is not an OCPP implementation, not a charging management system, and not
a substitute for the OCA Compliance Testing Tool (OCTT).

## R1 - Repository isolation

- R1.1 This repository does not import, build, link against or depend on any
  library under test. It contains no test logic or expectation specific to
  one.
- R1.2 A consumer depends only on the published images and the documented
  contract. It does not need a checkout of this repository.
- R1.3 Each adapter drives its own upstream OCPP stack. Adapter glue
  (argument parsing, fixture loading, JSON output) is allowed. A shared OCPP
  engine that replaces part of an upstream stack is not.

| Responsibility | Here | Consumer |
|----------------|:----:|:--------:|
| Third-party implementations, adapters, Dockerfiles | yes | no |
| Fixtures, fixture schema and validation | yes | no |
| Container contract, capability declarations | yes | no |
| Image builds and publishing, licence compliance | yes | no |
| Self-tests of the reference stacks against each other | yes | no |
| Scenarios and assertions about a library under test | no | yes |
| Compatibility claims of a library under test | no | yes |

## R2 - Reference implementations

| Adapter | Upstream | Language | Versions to verify | Priority |
|---|---|---|---|---|
| `mobilityhouse` | [mobilityhouse/ocpp](https://github.com/mobilityhouse/ocpp) | Python | 1.6J, 2.0.1 (2.1 present since 2.1.0, to be verified) | P0 |
| `chargetime` | [ChargeTimeEU/Java-OCA-OCPP](https://github.com/ChargeTimeEU/Java-OCA-OCPP) | Java | 1.6J, 2.0.1, 2.1 | P0 |
| `lorenzodonini` | [lorenzodonini/ocpp-go](https://github.com/lorenzodonini/ocpp-go) | Go | 1.6J, 2.0.1 (upstream marks 2.0.1 as needing more real-world testing) | P1 |
| `citrineos` | [citrineos/citrineos-core](https://github.com/citrineos/citrineos-core) | TypeScript | 1.6J, 2.0.1, 2.1; CSMS only | P2; needs PostgreSQL and RabbitMQ, which its image runs |

- R2.1 Support that an upstream advertises is a candidate, not a fact. A
  capability is declared only after a self-test exercises it (R4).
- R2.2 Every reference is pinned to a release and a commit (R6).

## R3 - One contract

- R3.1 Every adapter image implements
  [docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md): the same
  commands, flags, documents and exit codes.
- R3.2 The contract is versioned. Breaking changes bump the major version;
  additions bump the minor version.
- R3.3 Every document an image writes validates against its schema in
  [schemas/](schemas).

## R4 - Honest capabilities

- R4.1 `print-capabilities` declares capabilities per protocol version and
  per role. A feature supported in one version is not assumed in another.
- R4.2 A capability is `true` only if the self-tests exercise it.
- R4.3 A scenario an adapter cannot run is reported as `SKIP_UNSUPPORTED`,
  never as `PASS`.
- R4.4 A known upstream defect is recorded in `tests/known-bugs.tsv` with a
  link to evidence. A known-bug entry that no longer reproduces fails the
  run, so that the list cannot go stale.

## R5 - Self-tests

- R5.1 Each image passes its own smoke tests before it is used against
  anything else.
- R5.2 Every client adapter runs against every server adapter, for every
  version both support, in both directions of request flow.
- R5.3 Results are written as report documents with the statuses `PASS`,
  `FAIL`, `SKIP_UNSUPPORTED`, `KNOWN_ISSUE` and `INFRA_ERROR`, and a
  compatibility matrix is generated from them.

## R6 - Reproducibility

- R6.1 [versions.yaml](versions.yaml) records, for each adapter:
  - the upstream repository, release, commit, licence and language;
  - the protocol versions and roles;
  - the base image digests.
  `scripts/check-versions.sh` fails when versions.yaml and the Dockerfiles
  disagree.
- R6.2 Nothing floats: base images are pinned by digest, and upstream
  dependencies by version and hash.
- R6.3 Qualification uses images pinned by digest. A separate scheduled
  workflow tests newer upstream releases as candidates and never changes a
  pin by itself.
- R6.4 Images are built for linux/amd64 and linux/arm64.

## R7 - Safety

- R7.1 Images run as a non-root user.
- R7.2 Servers listen on the container network. The compose file and the
  documented `docker run` examples publish ports on 127.0.0.1 only.
- R7.3 No private key is committed or baked into an image. TLS scenarios
  generate their keys and certificates at test time.
- R7.4 Nothing needs real charging equipment or network access beyond
  pulling images.

## R8 - Licensing

- R8.1 This repository's own code is MIT licensed. Each image carries the
  licences of everything it contains under `/licenses`, and
  [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
- R8.2 OCA specification documents and schemas are CC BY-ND 4.0. They are
  not committed here. Fixtures are written for this repository and are not
  taken from the specification. An upstream package that ships the OCA
  schemas (The Mobility House ocpp) is put into its image unmodified, and
  THIRD_PARTY_NOTICES.md says so.

## R9 - Scope discipline

- R9.1 Adapters implement the operations the contract defines, and nothing
  else. They contain no charging logic beyond the deterministic answers the
  fixture defines.
- R9.2 The test runner checks protocol behaviour between references. It
  does not grade conformance to the OCPP specification.
