# Plan

The order of work for this repository. The overall plan, which also covers
otfabric/go-ocpp, is in that repository's IMPLEMENTATION_PLAN.md.

## Phase 0 - Design (done, 2026-10-18)

- [REQUIREMENTS.md](REQUIREMENTS.md)
- [docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md), draft 0.1
- Document schemas: capabilities, results, reports, fixtures

## Phase 1 - Foundation (done, 2026-10-18)

- [x] Layout, licence, Makefile, CI (validation job)
- [x] Fixture schema, and the `baseline` fixtures for 1.6 and 2.0.1, with
      cross-field validation
- [x] versions.yaml with the candidate upstreams (status `planned`)
- [x] Schema and document validators, shellcheck
- [x] CI green on GitHub
- [x] Release workflow (contract, schemas and fixtures; no images yet) and
      the weekly upstream candidate workflow

## Phase 4 - First adapters (open)

The phase number matches go-ocpp's plan, where Phase 4 introduces
interoperability. First slice: OCPP 1.6J only (2026-10-18).

- [x] Choose the test runner: Bash with jq, as in the sibling repositories.
      Bidirectional scenarios do not need more: `client run` holds the
      station's connection in one container, and `docker exec ... call`
      makes either end send a CALL through its control socket, so the
      runner only starts containers, runs commands and reads JSON
      (scripts/lib.sh, smoke.sh, interop.sh). No Go runner
- [x] `mobilityhouse` adapter: Python, both roles, 1.6J
- [x] `chargetime` adapter: Java, both roles, 1.6J
- [x] Control socket and `call`, `client run`, and the events
- [x] Dockerfiles: pinned bases, hashed dependency locks, non-root,
      licences, OCI labels, health check
- [x] Self-tests: smoke tests per image, then every client against every
      server, with requests from both ends
- [x] Reports (schemas/report.schema.json) and a generated compatibility
      matrix (scripts/compat-matrix.py, docs/COMPATIBILITY.md)
- [x] Per-architecture CI (native amd64 and arm64); image build, self-test
      and GHCR publish jobs in the release workflow; candidate builds in the
      candidate workflow (`make candidate-<adapter>`). Written and checked
      with actionlint, not yet run on GitHub; locally both images also build
      and start for linux/amd64 under emulation, but are self-tested on
      arm64 only
- [x] Contract 0.2: what implementing it with these upstreams showed
- [ ] Contract 1.0, once the adapters close the gaps the contract lists
- [ ] Release v0.1.0 (the maintainer decides when)

## Phase 5 - OCPP 2.0.1 and the third adapter (open)

- [x] 2.0.1 in `mobilityhouse` (ocpp.v201) and `chargetime` (ocpp2:
      MultiProtocolJSONServer and MultiProtocolJSONClient), both roles
- [x] `lorenzodonini` adapter (Go, ocpp-go v0.19.0), 1.6J and 2.0.1, both
      roles
- [x] The 2.0.1 baseline fixture complete: a sample for every action it
      answers in both directions, two it does not answer, `derived:
      variables` and `idTokenInfo`; every payload valid against the OCA
      2.0.1 schemas (scripts/validate-payloads.py)
- [x] Self-tests per version: smoke tests of every image for 1.6 and
      2.0.1, every station against every CSMS for both (3 x 3 each)
- [x] Contract 0.3: what implementing 2.0.1 and the third adapter required
- [x] Known bugs that show only with one peer (`adapter:role@peer`)
- [ ] TLS scenarios (profiles 2 and 3) with keys generated at test time
- [ ] Compression (`permessage-deflate`)

## Phase 6 - OCPP 2.1 (open)

- [x] The 2.1 action directions, from OCPP 2.1 Edition 2 Part 2, in
      scripts/validate-fixtures.py; NotifyPeriodicEventStream is a SEND and
      has no answer
- [x] `fixtures/v21/baseline`: the 2.0.1 baseline and the 2.1 additions
      tariffs, DER control, periodic event streams and battery swap; every
      payload valid against the OCA 2.1 schemas
- [x] 2.1 in `mobilityhouse` (ocpp.v21), both roles, self-tested (smoke,
      and interop with itself). Its upstream defects are recorded: no
      answer to an unanswered 2.1 action, BatterySwap cannot be sent
- [x] Contract 0.4: `--version 2.1`; SEND actions are refused without the
      `send` feature; type 5 and 6 frames an upstream cannot read are
      `protocol-error`
- [x] `everest` adapter (C++, EVerest libocpp of everest-core 2026.02.1):
      a station for 1.6J, 2.0.1 and 2.1 on libocpp's websocket, message
      queue and message types; the second independent 2.1 implementation.
      `scripts/smoke.sh` runs the station checks of an adapter without a
      server against `SMOKE_PEER` (default mobilityhouse)
- [ ] `everest`: declare security profiles 2 and 3 (implemented with
      libocpp's TLS) once the self-tests exercise TLS (contract 0.5)
- [ ] 2.1 in `chargetime`: Java-OCA-OCPP has 2.1 (with SEND and
      CALLRESULTERROR) on master since 2025-12-13, after its v2.0 release,
      the latest on Maven Central. Waits for a release (R2.2)
- [ ] SEND and CALLRESULTERROR: a contract operation and the features
      `send` and `callResultError`, once a pinned upstream release
      implements them (neither ocpp 2.1.0 nor libocpp of everest-core
      2026.02.1 does)

## Later

- CitrineOS as a CSMS integration target
