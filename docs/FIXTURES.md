# Fixtures

A fixture describes, for one OCPP version:

- the stations a CSMS accepts and the identity a station uses;
- how either end answers the CALLs it receives;
- named sample requests that tests can send.

Fixtures contain no assertions. Consumers decide what to check, using the
values in the fixture, which they read from the image with `print-fixture`.

## Location and validation

```text
fixtures/schema/ocpp-fixture.schema.json   the schema (draft 2020-12)
fixtures/v16/<name>/fixture.json           OCPP 1.6J
fixtures/v201/<name>/fixture.json          OCPP 2.0.1
fixtures/v21/<name>/fixture.json           OCPP 2.1 (from Phase 6)
```

`make fixtures` validates each fixture against the schema, then checks the
cross-field rules (scripts/validate-fixtures.py):

- the name matches the directory, and the version directory matches
  `ocppVersion`;
- identities, id tokens, configuration keys, variables and EVSE IDs are
  unique;
- id tokens are at most 20 characters in 1.6 and 36 in 2.x, and every 2.x
  token has a type;
- every answer is for an action the answering role actually receives in
  that version;
- every sample is sent by the role that sends that action;
- a derived answer names a table that fits the action and is not empty.

The adapters apply the same rules when they load a fixture, and exit 4 on a
violation.

## Answers

`answers.csms` holds what a CSMS answers to station CALLs, and
`answers.station` holds what a station answers to CSMS CALLs. Each entry
takes one of three forms:

| Form | Meaning |
|---|---|
| `{"payload": {...}}` | Answer with this payload |
| `{"derived": "idTokens"}` | Authorize, StartTransaction: `idTagInfo` (1.6) or `idTokenInfo` (2.x) with the status the `idTokens` table gives. An unknown token is `Invalid`. A `payload` alongside it is merged in |
| `{"derived": "configuration"}` | GetConfiguration: the requested keys from `configuration`. Unknown keys go in `unknownKey` |
| `{"derived": "variables"}` | GetVariables and SetVariables: the result for each requested component and variable from `variables`. Writing a `ReadOnly` variable is `Rejected`. Values change only for the lifetime of the process |
| `{"callError": {"code": ..., "description": ...}}` | Answer with this CALLERROR |

An action without an entry is answered with the CALLERROR `NotImplemented`.

The string `"${now}"` in a payload is replaced with the current UTC time in
RFC 3339 format. It is the only placeholder.

## Baseline fixtures

`fixtures/v16/baseline` and `fixtures/v201/baseline` describe the station
`OT-FABRIC-TEST-001`, vendor `OTFabric`, model `TestStation`. It has two
connectors in 1.6, and two EVSEs with one connector each in 2.0.1. The id
tokens are one accepted, one blocked and one expired. Samples cover
provisioning, authorization, the transaction lifecycle, meter values,
remote start and stop, configuration or the device model, reset, triggers
and a TxDefault charging profile.

The station password (`OTFabricTestPassword01`) is a public test value. It
exists so that security profile 1 and 2 scenarios can run, and must never be
used anywhere else.
