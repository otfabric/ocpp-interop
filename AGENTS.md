# Agent instructions

Instructions for AI coding agents (and humans) working in this repository.

## Read first

1. [REQUIREMENTS.md](REQUIREMENTS.md): the rules this repository is held to.
2. [docs/CONTAINER_CONTRACT.md](docs/CONTAINER_CONTRACT.md): the product.
3. [PLAN.md](PLAN.md): what is done and what is next.

## Hard rules

- Never reference or depend on a library under test (`otfabric/go-ocpp` or
  any other) from adapters, fixtures, schemas, scripts or tests.
- Never write OCPP protocol logic shared between adapters. Each adapter
  drives its own upstream engine.
- Never mark a capability true, or list an action, without a self-test that
  exercises it.
- Never report an unsupported scenario as passed, and never widen a skip
  rule to make a run green.
- Never commit private keys, OCA PDFs or OCA schema files.
- Pin every upstream and base image, and mirror the pin in versions.yaml.
- Do not invent OCPP behaviour. Fixture payloads must be valid for their
  version, and an answer that is not in the fixture is `NotImplemented`.

## Before finishing

```bash
make ci
```

Update the document that owns the topic, and add an entry to RELEASE.md
under "Unreleased" (create it above the newest version if it is missing,
and close it with a `---` line: the section up to `---` becomes the body of
the GitHub release).

## Organisation standards

This repository follows **otfabric/engineering policy 1.2**, profile **interoperability** (`.otfabric/policy.yaml`, mode `enforce`). Where this file and the organisation standards disagree, this file wins here — say so in your report.
