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
fixtures/v21/<name>/fixture.json           OCPP 2.1
```

`make fixtures` validates each fixture against the schema, then checks the
cross-field rules (scripts/validate-fixtures.py):

- the name matches the directory, and the version directory matches
  `ocppVersion`;
- identities, id tokens, configuration keys, variables and EVSE IDs are
  unique;
- id tokens are at most 20 characters in 1.6, 36 in 2.0.1 and 255 in 2.1,
  and every 2.x token has a type;
- every answer is for an action the answering role actually receives in
  that version (for 2.1, the directions of OCPP 2.1 Edition 2, Part 2), and
  an action the version sends as a SEND (2.1: NotifyPeriodicEventStream) has
  no answer;
- every sample is sent by the role that sends that action;
- a derived answer names a table that fits the action and is not empty;
- a `baseline` fixture has a sample of every action of its version and an
  answer to every action that is not a SEND (a rule for this repository's
  baselines, which the adapters do not apply to the fixtures they load).

The adapters check a fixture when they load it, and exit 4 on a violation:
`mobilityhouse` with the schema and the code of scripts/validate-fixtures.py
itself, `chargetime` and `lorenzodonini` with a hand-written equivalent of
the rules for the versions they support (1.6 and 2.0.1; neither supports
2.1). A fixture with an
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

`fixtures/v16/baseline`, `fixtures/v201/baseline` and `fixtures/v21/baseline`
describe the station `OT-FABRIC-TEST-001`, vendor `OTFabric`, model
`TestStation`. It has two connectors in 1.6, and two EVSEs with one connector
each in 2.x. The id
tokens are one accepted, one blocked and one expired. Samples cover
provisioning, authorization, the transaction lifecycle, meter values,
remote start and stop, configuration or the device model, reset, triggers
and a TxDefault charging profile.

Each baseline fixture answers every action of its version that the
receiving role receives (the direction tables of
scripts/validate-fixtures.py), except a SEND, and has at least one sample of
every action, sent by the role that sends it: 39 actions in 1.6 (with the
Security Whitepaper messages), 64 in 2.0.1, 91 in 2.1 (one of them,
NotifyPeriodicEventStream, a SEND). Two actions have two samples in 1.6:
`securityEvent`/`securityEventMinimal` (SecurityEventNotification, with and
without the optional `techInfo`). To check the `NotImplemented` CALLERROR
from both ends, the self-tests use a copy of the baseline without one answer
on each side ([scripts/interop.sh](../scripts/interop.sh), `not_implemented`).

The certificate data is a throwaway test PKI, made once for these fixtures
with OpenSSL (EC P-256): a root CA `OTFabric Test Root CA` (serial 1001), a
station certificate `OT-FABRIC-TEST-001` it signed (serial 2001) and its
certificate signing request, a self-signed firmware signing certificate
(serial 3001), the base64 ECDSA signature of a test firmware image, an OCSP
response of the root CA for the station certificate (`good`), and the
`certificateHashData` of the certificates (SHA-256 of the issuer's name and
of its public key). The private keys were deleted after use and are in no
fixture. The EXI streams of Get15118EVCertificate are opaque base64 test
bytes, not ISO 15118 messages.

Every sample, every fixed answer and every derived answer for a sample
validates against the OCA JSON schemas of its version (1.6 including the
Security Whitepaper messages, 2.0.1, 2.1), as bundled with The Mobility
House ocpp 2.1.0. Its 2.1 schemas are identical to the OCA's
`OCPP-2.1_part3_JSON_schemas` of OCPP 2.1 Edition 2. The schemas are not part of this repository;
[scripts/validate-payloads.py](../scripts/validate-payloads.py) checks a
fixture against a copy of them:

```bash
python3 -m venv /tmp/oca && /tmp/oca/bin/pip install --index-url https://pypi.org/simple ocpp==2.1.0 jsonschema
schemas="$(/tmp/oca/bin/python -c 'import ocpp, os; print(os.path.dirname(ocpp.__file__))')"
/tmp/oca/bin/python scripts/validate-payloads.py "$schemas/v16/schemas" fixtures/v16/baseline/fixture.json
/tmp/oca/bin/python scripts/validate-payloads.py "$schemas/v201/schemas" fixtures/v201/baseline/fixture.json
/tmp/oca/bin/python scripts/validate-payloads.py "$schemas/v21/schemas" fixtures/v21/baseline/fixture.json
```

The 2.0.1 samples cover every action of the version: provisioning
(BootNotification, Heartbeat, StatusNotification, SetNetworkProfile),
authorization and the local list, the TransactionEvent lifecycle (Started,
Updated, Ended) and GetTransactionStatus, MeterValues, the device model
(NotifyReport, NotifyEvent, GetBaseReport, GetReport, GetVariables,
SetVariables) and monitoring, smart charging (including the EV's charging
needs and schedule, charging limits and reported profiles), reservations,
display messages, customer information, firmware and logs (including
publishing firmware), certificates (security and ISO 15118), CostUpdated,
DataTransfer from both ends and the remote commands.

The 2.1 baseline has every 2.0.1 sample and answer (all of them valid 2.1
payloads, except that a 2.1 monitor of NotifyMonitoringReport also has the
required `eventNotificationType`), and these OCPP 2.1 additions:

| Area | Station sends (CSMS answers) | CSMS sends (station answers) |
|---|---|---|
| Tariffs and cost | — | SetDefaultTariff, GetTariffs, ClearTariffs |
| DER control | NotifyDERAlarm, NotifyDERStartStop, ReportDERControl | SetDERControl, GetDERControl, ClearDERControl |
| Periodic event streams | OpenPeriodicEventStream, ClosePeriodicEventStream; NotifyPeriodicEventStream as a SEND (no answer) | GetPeriodicEventStream, AdjustPeriodicEventStream |
| Battery swap | BatterySwap | RequestBatterySwap |
| Payment and settlement | NotifySettlement, VatNumberValidation | NotifyWebPaymentStarted, ChangeTransactionTariff |
| Smart charging | NotifyPriorityCharging, PullDynamicScheduleUpdate | UsePriorityCharging, UpdateDynamicSchedule, NotifyAllowedEnergyTransfer, AFRRSignal |
| Certificates | GetCertificateChainStatus | — |

The answers are fixed: GetTariffs reports the tariff that SetDefaultTariff
installs, and GetPeriodicEventStream the stream that OpenPeriodicEventStream
opens, whatever happened before.

The station password (`OTFabricTestPass0001`) is a public test value. It
exists so that security profile 1 and 2 scenarios can run, and must never be
used anywhere else. It has 20 characters: Java-OCA-OCPP's 1.6 server accepts
Basic passwords of 16 to 20 bytes only.
