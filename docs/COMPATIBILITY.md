# Compatibility

Results of the reference-to-reference self-tests
([scripts/interop.sh](../scripts/interop.sh)): every station (`client`)
against every CSMS (`server`), for OCPP 1.6J, 2.0.1 and 2.1, with CALLs sent by
the station and by the CSMS. The matrix below is the output of `make matrix` (from
`reports/report-interop.json`, written by `make documents`); CI writes the
same matrix for each architecture into its job summary.

Run: 2026-10-18, images built from this repository on linux/arm64
(macOS, Docker 29.8): `make validate lint`, then `make images smoke` and
`scripts/interop.sh` on images tagged `coverage`. The linux/amd64 images have not been
self-tested locally; CI does that natively.

Each cell: results of the CALLs and checks initiated by the station / by
the CSMS. The baseline fixtures have a sample of every action of their
version, so a pairing sends every action both ends declare. A 1.6 pairing
has 26 tests initiated by the station and 29 by the CSMS: one per fixture
sample, two derived answers (an unknown token, unknown configuration keys),
connectivity, security profile 1, the fixture's CALLERROR answers and
NotImplemented (from a fixture copy without one answer per side). A 2.0.1
pairing has 38 and 44: one per sample, the derived answers (an unknown
token; unknown component and variable with case-insensitive names; a
read-only variable), connectivity, security profile 1, the CALLERROR
answers and NotImplemented. A 2.1 pairing has 51 and 60: the 2.0.1 tests,
one per 2.1 sample, the SEND sample and CALLRESULTERROR (from each end).

Actions exchanged with a PASS in at least one pairing: 39 of 39 in 1.6, 64
of 64 in 2.0.1 and 89 of 91 in 2.1 (not BatterySwap and
NotifyPeriodicEventStream; [COVERAGE.md](../COVERAGE.md)). Before every
action had a sample (v0.2.0): 30, 27 and 43.

## OCPP 1.6J

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 25 pass 1 skip / 28 pass 1 skip | 26 pass / 28 pass 1 skip | 26 pass / 28 pass 1 skip |
| lorenzodonini v0.19.0 | 25 pass 1 skip / 29 pass | 26 pass / 29 pass | 26 pass / 29 pass |
| mobilityhouse 2.1.0 | 24 pass 1 skip 1 known / 29 pass | 25 pass 1 known / 29 pass | 25 pass 1 known / 29 pass |

Totals: 486 PASS, 3 KNOWN_ISSUE, 6 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | all three | `v16.from-station.security-event-minimal` | KNOWN_ISSUE | [mobilityhouse/ocpp `v16/call.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |
| chargetime | all three | `v16.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all three | chargetime | `v16.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |

## OCPP 2.0.1

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 37 pass 1 skip / 43 pass 1 skip | 38 pass / 42 pass 1 skip 1 known | 38 pass / 43 pass 1 skip |
| lorenzodonini v0.19.0 | 36 pass 1 skip 1 known / 43 pass 1 known | 37 pass 1 known / 43 pass 1 known | 37 pass 1 known / 41 pass 3 known |
| mobilityhouse 2.1.0 | 8 pass 1 skip 29 known / 1 pass 43 known | 38 pass / 43 pass 1 known | 38 pass / 44 pass |

Totals: 650 PASS, 82 KNOWN_ISSUE, 6 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | chargetime | `v201.from-station.*` after the first CALL (28), `v201.from-csms.*` (42), `v201.not-implemented.*` (2), `v201.connect.close` | KNOWN_ISSUE (73) | [Java-WebSocket 1.6.0 `PerMessageDeflateExtension`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L335-L340) |
| lorenzodonini | all three | `v201.from-station.notify-report` | KNOWN_ISSUE | [ocpp-go `notify_report.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |
| lorenzodonini | mobilityhouse | `v201.from-csms.reset-immediate`, `v201.from-csms.set-network-profile` | KNOWN_ISSUE (2) | [ocpp-go `reset.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| lorenzodonini, or any station with lorenzodonini | all three, or lorenzodonini | `v201.from-csms.get-composite-schedule` | KNOWN_ISSUE (5) | [ocpp-go `get_composite_schedule.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/smartcharging/get_composite_schedule.go#L31-L50) |
| chargetime | all three | `v201.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all three | chargetime | `v201.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |

The mobilityhouse station and the chargetime 2.0.1 CSMS do connect, and
one-shot clients and the first CALL of a connection work (BootNotification
passes); every later CALL on the connection fails, because the station
cannot decompress the CSMS's second message. The chargetime station and the
lorenzodonini station do not offer compression and are not affected.

## OCPP 2.1

Only `mobilityhouse` declares 2.1: the pinned releases of Java-OCA-OCPP
(v2.0) and ocpp-go (v0.19.0) have no 2.1, so every pairing with them is one
`v21.pairing` SKIP_UNSUPPORTED.

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 1 skip / — | 1 skip / — | 1 skip / — |
| lorenzodonini v0.19.0 | 1 skip / — | 1 skip / — | 1 skip / — |
| mobilityhouse 2.1.0 | 1 skip / — | 1 skip / — | 47 pass 3 skip 1 known / 58 pass 1 skip 1 known |

Totals: 105 PASS, 2 KNOWN_ISSUE, 12 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | mobilityhouse | `v21.not-implemented.from-station`, `v21.not-implemented.from-csms` | KNOWN_ISSUE (2) | [mobilityhouse/ocpp `charge_point.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse | mobilityhouse | `v21.from-station.battery-swap-in` | SKIP_UNSUPPORTED | BatterySwap is not declared: ocpp 2.1.0 cannot send it (see below) |
| mobilityhouse | mobilityhouse | `v21.send.notify-periodic-event-stream` | SKIP_UNSUPPORTED | no adapter declares `send` |
| mobilityhouse | mobilityhouse | `v21.rpc.call-result-error` (from each end) | SKIP_UNSUPPORTED (2) | no adapter declares `callResultError` |
| chargetime, lorenzodonini | all three | `v21.pairing` | SKIP_UNSUPPORTED (8) | 2.1 not declared by one of the two |

Every smoke test ([scripts/smoke.sh](../scripts/smoke.sh)) passes: 128
checks for chargetime and lorenzodonini (1.6 and 2.0.1), 176 for
mobilityhouse (1.6, 2.0.1 and 2.1).

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
| mobilityhouse (station, 1.6) | ocpp 2.1.0 | `ocpp.v16.call.SecurityEventNotification` declares `tech_info` without a default: a SecurityEventNotification without the optional `techInfo` cannot be built, although the bundled OCA schema makes it optional. Still so on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |
| chargetime (CSMS, 2.0.1), with a station that offers permessage-deflate | Java-OCA-OCPP v2.0 with Java-WebSocket 1.6.0 | `MultiProtocolJSONServer` enables permessage-deflate by default and keeps its compression context (`setServerNoContextTakeover(false)`), but Java-WebSocket 1.6.0 always answers `server_no_context_takeover`; the station cannot decompress the second message and closes the connection (1002). Java-WebSocket's master announces the parameter only when set | [source](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L335-L340) |
| lorenzodonini (station, 2.0.1), with a CSMS that validates responses | ocpp-go v0.19.0 | `ResetResponse` and `SetNetworkProfileResponse` declare `statusInfo` without `omitempty`: the station sends `"statusInfo": null`, which the OCA schema does not allow | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| mobilityhouse (both roles, 2.1) | ocpp 2.1.0 | `charge_point._raise_key_error` raises NotImplemented or NotSupported for 1.6, 2.0 and 2.0.1 only: a 2.1 CALL whose action has no handler gets no answer, and the sender times out. Fixed on master (ae716a1, #744), unreleased | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse (station, 2.1) | ocpp 2.1.0 | `snake_to_camel_case` writes the `soc` and `soh` fields of `BatteryDataType` as `SoC` and `soh` (the schema has `soC`, `soH`), so a BatterySwapRequest fails the upstream's own validation and cannot be sent. Not in known-bugs.tsv: BatterySwap is not declared, so no test runs it. Still so on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L57-L64) |
| lorenzodonini (both roles, 2.0.1) | ocpp-go v0.19.0 | `GetCompositeScheduleResponse` does not have the OCA structure: a top-level `evseId`, always written, which the schema does not allow; no `statusInfo`; a `schedule` of `{startDateTime, chargingSchedule}` instead of `CompositeScheduleType`. The station cannot answer with a schedule, and the CSMS drops the schedule of a correct answer | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/smartcharging/get_composite_schedule.go#L31-L50) |
| mobilityhouse (both roles, 2.1) | ocpp 2.1.0 | `snake_to_camel_case` also misspells other optional 2.1 fields: `receiptUrl` (NotifySettlement, written `receiptURL`), the per-phase `limit_L2`-style fields (`limitL2`), `fixedPF...`, `prioritizedEMAIDs`, `taxRuleID`, `soC` of `LimitAtSoCType`; a payload with one fails the upstream's own validation. Not in known-bugs.tsv: the baseline's payloads use none of them (NotifySettlement is answered with `receiptId` only) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L46-L80) |
| lorenzodonini (station, 2.0.1) | ocpp-go v0.19.0 | `NotifyReportRequest.Tbc` is a `bool` with `omitempty`: an explicit `"tbc": false` is dropped from the request (false is the default) | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |

See [tests/known-bugs.tsv](../tests/known-bugs.tsv).
