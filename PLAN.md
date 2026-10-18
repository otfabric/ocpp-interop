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
- [ ] Contract 1.0, once both adapters close the gaps the contract lists
- [ ] Release v0.1.0 (the maintainer decides when)

## Phase 5 - OCPP 2.0.1 and the third adapter (open)

- [ ] 2.0.1 in `mobilityhouse` and `chargetime`
- [ ] `lorenzodonini` adapter (Go), 1.6J and 2.0.1
- [ ] TLS scenarios (profiles 2 and 3) with keys generated at test time
- [ ] Compression (`permessage-deflate`)

## Phase 6 - OCPP 2.1 (open)

- [ ] 2.1 fixtures, once the action list is verified against Edition 2
- [ ] 2.1 in `chargetime`, and in `mobilityhouse` if its 2.1.0 support
      holds up in tests
- [ ] SEND and CALLRESULTERROR features in the capabilities

## Later

- CitrineOS as a CSMS integration target
