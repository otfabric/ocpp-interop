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

To be written with the first adapter (PLAN.md, Phase 4). The adapters of
iec104-interop and modbus-interop show the expected structure.

## Style

- Shell: `set -euo pipefail`, clean under shellcheck at warning severity.
- Python: standard library only in scripts, except `jsonschema`.
- JSON: two-space indentation. Keys in fixtures follow OCPP's camelCase.

## License

Contributions are accepted under the MIT licence of this repository.
