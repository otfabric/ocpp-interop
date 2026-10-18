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

The adapters check a fixture when they load it, and exit 4 on a violation:
`mobilityhouse` with the schema and the code of scripts/validate-fixtures.py
itself, `chargetime` and `lorenzodonini` with a hand-written equivalent of
the rules for the versions they support (1.6 and 2.0.1). A fixture with an
answer the adapter cannot give (`callError` without the `fixtureCallErrors`
capability, or with a code that is not an OCPP-J error code) exits 5.

## Answers

`answers.csms` holds what a CSMS answers to station CALLs, and
`answers.station` holds what a station answers to CSMS CALLs. Each entry
takes one of three forms:

| Form | Meaning |
|---|---|
| `{"payload": {...}}` | Answer with this payload |
| `{"derived": "idTokens"}` | Authorize, StartTransaction: `idTagInfo` (1.6) or `idTokenInfo` (2.x) with the status the `idTokens` table gives. In 2.x a token matches by `idToken` and `type`. An unknown token is `Invalid`. A `payload` alongside it is merged in |
| `{"derived": "configuration"}` | GetConfiguration: the requested keys from `configuration`, or every key when none is requested. Keys compare case-insensitively (CiString50Type). Unknown keys go in `unknownKey`, which is left out when empty |
| `{"derived": "variables"}` | GetVariables and SetVariables: one result per requested component and variable, in the order of the request, from `variables` (see below) |
| `{"callError": {"code": ..., "description": ...}}` | Answer with this CALLERROR |

An action without an entry is answered with the CALLERROR `NotImplemented`.

The string `"${now}"` in a payload is replaced with the current UTC time in
RFC 3339 format, in whole seconds (`2026-10-18T10:00:00Z`). It is the only
placeholder. An upstream may write it with milliseconds on the wire.

ChangeConfiguration does not change what GetConfiguration reports: 1.6
answers are fixed, apart from the derived ones.

### Device model variables (2.x)

A `variables` entry is a component (`name`, optional `instance`), a
variable (`name`, optional `instance`), the `value` of its `Actual`
attribute and its `mutability`. Names and instances compare
case-insensitively. For each item of `getVariableData` or
`setVariableData`, the result echoes `component`, `variable` and, if the
request has one, `attributeType`, with `attributeStatus`:

| Status | When |
|---|---|
| `UnknownComponent` | no entry has that component |
| `UnknownVariable` | the component exists, but not with that variable |
| `NotSupportedAttributeType` | the request names an `attributeType` other than `Actual` |
| `Rejected` | GetVariables of a `WriteOnly` variable, SetVariables of a `ReadOnly` one |
| `Accepted` | otherwise; GetVariables adds `attributeValue`, the current value |

An accepted SetVariables changes the value that GetVariables reports, for
the lifetime of the process. Nothing else changes values.

## Baseline fixtures

`fixtures/v16/baseline` and `fixtures/v201/baseline` describe the station
`OT-FABRIC-TEST-001`, vendor `OTFabric`, model `TestStation`. It has two
connectors in 1.6, and two EVSEs with one connector each in 2.0.1. The id
tokens are one accepted, one blocked and one expired. Samples cover
provisioning, authorization, the transaction lifecycle, meter values,
remote start and stop, configuration or the device model, reset, triggers
and a TxDefault charging profile.

Each baseline fixture has a sample for every action it answers, in both
directions, and samples for actions it does not answer, so that a test can
check the `NotImplemented` CALLERROR from both ends. In 1.6:
`securityEvent`/`securityEventMinimal` (SecurityEventNotification, with and
without the optional `techInfo`) and `extendedTriggerHeartbeat`
(ExtendedTriggerMessage). In 2.0.1: `reservationStatusUpdate`
(ReservationStatusUpdate) and `cancelReservation` (CancelReservation).

Every sample, every fixed answer and every derived answer for a sample
validates against the OCA JSON schemas of its version (1.6 including the
Security Whitepaper messages, 2.0.1), as bundled with The Mobility House
ocpp 2.1.0. The schemas are not part of this repository;
[scripts/validate-payloads.py](../scripts/validate-payloads.py) checks a
fixture against a copy of them:

```bash
python3 -m venv /tmp/oca && /tmp/oca/bin/pip install --index-url https://pypi.org/simple ocpp==2.1.0 jsonschema
schemas="$(/tmp/oca/bin/python -c 'import ocpp, os; print(os.path.dirname(ocpp.__file__))')"
/tmp/oca/bin/python scripts/validate-payloads.py "$schemas/v16/schemas" fixtures/v16/baseline/fixture.json
/tmp/oca/bin/python scripts/validate-payloads.py "$schemas/v201/schemas" fixtures/v201/baseline/fixture.json
```

The 2.0.1 samples cover provisioning (BootNotification, Heartbeat,
StatusNotification), authorization, the TransactionEvent lifecycle (Started,
Updated, Ended), MeterValues, the device model (NotifyReport, NotifyEvent,
GetBaseReport, GetVariables, SetVariables), SecurityEventNotification,
FirmwareStatusNotification, LogStatusNotification, DataTransfer from both
ends, RequestStart/StopTransaction, Reset, TriggerMessage, ChangeAvailability,
ClearCache, UnlockConnector, SetChargingProfile (TxDefaultProfile),
ClearChargingProfile and GetLocalListVersion.

The station password (`OTFabricTestPass0001`) is a public test value. It
exists so that security profile 1 and 2 scenarios can run, and must never be
used anywhere else. It has 20 characters: Java-OCA-OCPP's 1.6 server accepts
Basic passwords of 16 to 20 bytes only.
