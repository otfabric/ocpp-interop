# Compatibility

Results of the reference-to-reference self-tests
([scripts/interop.sh](../scripts/interop.sh)): every station (`client`)
against every CSMS (`server`), with CALLs sent by the station and by the
CSMS. The matrix below is the output of `make matrix` (from
`reports/report-interop.json`, written by `make documents`); CI writes the
same matrix for each architecture into its job summary.

Run: 2026-10-18, images built from this repository on linux/arm64
(macOS, Docker 29.8). The linux/amd64 images have not been self-tested yet;
CI does that natively.

## OCPP 1.6J

Each cell: results of the CALLs and checks initiated by the station /
by the CSMS. A pairing has 22 of each: one per fixture sample, two derived
answers (an unknown token, unknown configuration keys), connectivity,
security profile 1 and the fixture's CALLERROR answers.

| Station ↓ / CSMS → | chargetime v2.0 | mobilityhouse 2.1.0 |
|---|---|---|
| chargetime v2.0 | 21 pass 1 skip / 21 pass 1 skip | 22 pass / 21 pass 1 skip |
| mobilityhouse 2.1.0 | 20 pass 1 skip 1 known / 22 pass | 21 pass 1 known / 22 pass |

Totals: 170 PASS, 2 KNOWN_ISSUE, 4 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | chargetime | `v16.from-station.security-event-minimal` | KNOWN_ISSUE | [mobilityhouse/ocpp `v16/call.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |
| mobilityhouse | mobilityhouse | `v16.from-station.security-event-minimal` | KNOWN_ISSUE | same |
| chargetime | chargetime | `v16.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| chargetime | mobilityhouse | `v16.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| chargetime | chargetime | `v16.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| mobilityhouse | chargetime | `v16.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |

Both smoke tests ([scripts/smoke.sh](../scripts/smoke.sh)) pass: 80 checks
each.

### What a PASS means

The sender's result document is the answer the fixture defines (every field
of the fixture's answer is there with an equal value; `"${now}"` is a date
and time), the request on the wire keeps every field of the sample, and
both ends report the CALL in their events: `call-sent` and `call-result` at
the sender, `call-received` with the request and `call-answered` with the
fixture's answer at the receiver, with the same message id where both
adapters expose it. Interoperability between two references says nothing
about conformance to the specification.

### Observations that are not failures

- Java-OCA-OCPP writes date-times with milliseconds
  (`2026-01-01T10:00:00.000Z`), decimals as `11000.0`, and adds
  `numberPhases: 3`, the default of the field, to a charging schedule
  period that has none. The other end reads all of them; the tests compare
  date-times and numbers by value.
- mobilityhouse writes the decimals of the SetChargingProfile and
  RemoteStartTransaction requests and of the GetCompositeSchedule response
  it sends with one fraction digit (`ocpp.messages._DecimalEncoder`). The
  fixture's limits are whole numbers, so the tests do not exercise it.

## Known upstream defects

| Adapter | Upstream | Defect | Evidence |
|---|---|---|---|
| mobilityhouse (station) | ocpp 2.1.0 | `ocpp.v16.call.SecurityEventNotification` declares `tech_info` without a default: a SecurityEventNotification without the optional `techInfo` cannot be built, although the bundled OCA schema makes it optional. Still so on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |

See [tests/known-bugs.tsv](../tests/known-bugs.tsv).
