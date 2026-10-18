# Contributing

## Ground rules

1. **Nothing here may know a library under test** (REQUIREMENTS.md R1). CI
   fails on references to one in adapters, fixtures, schemas, scripts or
   tests.
2. **The contract is the product.** Change
   [docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md) first, then every
   adapter. After 1.0, an addition bumps the minor version and a breaking
   change bumps the major version.
3. **Declare only what is tested.** A capability or listed action needs a
   self-test that exercises it.
4. **Pin everything, and mirror it in versions.yaml.** `make versions`
   enforces this.
5. **Each adapter uses its own upstream engine.** Glue is fine. A shared OCPP
   implementation is not.

## Setup

- Docker with buildx
- Python 3.12 or later, with `jsonschema` (otherwise the checks run in a
  container)
- jq; shellcheck (otherwise it runs in a container)

```bash
make ci
```

## Adding a fixture

1. Create `fixtures/<v16|v201|v21>/<name>/fixture.json`.
2. Run `make fixtures`.
3. Document the fixture in [docs/FIXTURES.md](docs/FIXTURES.md).

Payloads must be valid for the OCPP version. Check them against the OCA
schema for that version before committing.

## Adding an adapter

`adapters/mobilityhouse` (Python) and `adapters/chargetime` (Java) show the
expected structure.

1. Read the upstream's source for what it really supports; its README is a
   candidate list, not a fact.
2. `adapters/<name>/`: the adapter, a `Dockerfile` in the style of the
   others (multi-stage, `ARG BASE_REGISTRY`, every base image as
   `ARG <NAME>_DIGEST=sha256:...`, `ARG UPSTREAM_VERSION` and
   `ARG UPSTREAM_SHA`, uid 10001, `/licenses`, OCI labels, `EXPOSE 9000`,
   the health check, `ENTRYPOINT ["/usr/local/bin/ocpp-interop"]`), a lock
   file that pins every dependency by hash, and a README that says how the
   upstream is used and what it cannot do.
3. Raw JSON becomes the upstream's types through the upstream's own
   (de)serialisation, and the upstream's engine sends and answers. Glue may
   parse arguments, load fixtures and write JSON; it may not implement
   OCPP.
4. Declare in `print-capabilities` only what `scripts/smoke.sh` and
   `scripts/interop.sh` exercise and pass.
5. Add the adapter to `versions.yaml` (status `built`), to `ADAPTERS` in the
   Makefile, to compose.yaml, and to THIRD_PARTY_NOTICES.md. Then
   `make ci`.
6. Record real upstream defects in `tests/known-bugs.tsv` with a link to the
   evidence (a permanent link to the upstream source or an issue).

## Style

- Shell: `set -euo pipefail`, clean under shellcheck at warning severity.
- Python: standard library only in scripts, except `jsonschema`.
- JSON: two-space indentation. Keys in fixtures follow OCPP's camelCase.

## License

Contributions are accepted under the MIT licence of this repository.
