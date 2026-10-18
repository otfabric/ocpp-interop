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

`everest` (EVerest libocpp, everest-core 2026.02.1) is a station only.
Every pairing with `everest` as the CSMS is one `pairing` SKIP_UNSUPPORTED
(not in the tables). The tables are from one run of the four adapters
on contract 0.5 (2026-10-18, linux/arm64, `make ci`): 1689 PASS, 20
KNOWN_ISSUE, 257 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR. Most of the
everest skips are actions it does not declare (libocpp's message queue
knows a subset of the actions, adapters/everest/README.md) and compression.

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

Each pairing also runs the transport scenarios `tls.profile2`,
`tls.profile3` and `compression` (counted with the station's).
The transport scenarios keep a station connected over security profile 2
(TLS with RSA keys and Basic authentication), security profile 3 (mutual
TLS with ECDSA P-256 keys) or permessage-deflate, and send two CALLs from
each end; the CA and the certificates are generated for the run. They run
where both ends declare the profile or `compression`.

## OCPP 1.6J

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 27 pass 2 skip / 28 pass 1 skip | 28 pass 1 skip / 28 pass 1 skip | 28 pass 1 skip / 28 pass 1 skip |
| lorenzodonini v0.19.0 | 27 pass 2 skip / 29 pass | 28 pass 1 skip / 29 pass | 28 pass 1 skip / 29 pass |
| mobilityhouse 2.1.0 | 26 pass 2 skip 1 known / 29 pass | 27 pass 1 skip 1 known / 29 pass | 28 pass 1 known / 29 pass |
| everest 2026.02.1 | 24 pass 5 skip / 22 pass 7 skip | 25 pass 4 skip / 22 pass 7 skip | 25 pass 4 skip / 22 pass 7 skip |

Totals: 645 PASS, 3 KNOWN_ISSUE, 52 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
Security profiles 2 and 3 pass in all twelve pairings (every station,
everest included, with every CSMS).

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | all three | `v16.from-station.security-event-minimal` | KNOWN_ISSUE | [mobilityhouse/ocpp `v16/call.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/v16/call.py#L232-L236) |
| chargetime | all three | `v16.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all three | chargetime | `v16.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all but mobilityhouse with itself | | `v16.compression` | SKIP_UNSUPPORTED (8) | only mobilityhouse declares `compression` in 1.6: chargetime's 1.6 classes have no extensions, lorenzodonini's CSMS cannot accept it |

## OCPP 2.0.1

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 39 pass 1 skip 1 known / 43 pass 1 skip | 40 pass 1 skip / 42 pass 1 skip 1 known | 40 pass 1 known / 43 pass 1 skip |
| lorenzodonini v0.19.0 | 38 pass 2 skip 1 known / 43 pass 1 known | 39 pass 1 skip 1 known / 43 pass 1 known | 39 pass 1 skip 1 known / 41 pass 3 known |
| mobilityhouse 2.1.0 | 39 pass 1 skip 1 known / 44 pass | 40 pass 1 skip / 43 pass 1 known | 41 pass / 44 pass |
| everest 2026.02.1 | 27 pass 14 skip / 18 pass 26 skip | 28 pass 13 skip / 18 pass 26 skip | 28 pass 13 skip / 18 pass 26 skip |

Totals: 878 PASS, 13 KNOWN_ISSUE, 133 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
Security profiles 2 and 3 pass in all twelve pairings (every station,
everest included, with every CSMS).

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse, chargetime | chargetime | `v201.compression` | KNOWN_ISSUE (2) | the CSMS keeps its compression context although it announces `server_no_context_takeover`: [Java-WebSocket 1.6.0 `PerMessageDeflateExtension`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L335-L340) |
| chargetime | mobilityhouse | `v201.compression` | KNOWN_ISSUE | the station keeps its compression context although it offers `client_no_context_takeover`: [Java-WebSocket 1.6.0 `PerMessageDeflateExtension`](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L326-L333) |
| lorenzodonini | all three | `v201.from-station.notify-report` | KNOWN_ISSUE | [ocpp-go `notify_report.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |
| lorenzodonini | mobilityhouse | `v201.from-csms.reset-immediate`, `v201.from-csms.set-network-profile` | KNOWN_ISSUE (2) | [ocpp-go `reset.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| lorenzodonini, or any station with lorenzodonini | all three, or lorenzodonini | `v201.from-csms.get-composite-schedule` | KNOWN_ISSUE (5) | [ocpp-go `get_composite_schedule.go`](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/smartcharging/get_composite_schedule.go#L31-L50) |
| chargetime | all three | `v201.fixture-callerror.from-csms` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| all three | chargetime | `v201.fixture-callerror.from-station` | SKIP_UNSUPPORTED | chargetime does not declare `fixtureCallErrors` |
| lorenzodonini at either end | | `v201.compression` | SKIP_UNSUPPORTED (5) | lorenzodonini does not declare `compression`: ocpp-go's CSMS cannot accept it |

With compression, the first CALL of a connection to or from the chargetime
2.0.1 adapter works (BootNotification passes); the second message in the
direction of the defect cannot be decompressed and the connection closes.
Without `--compression` no station offers it, so every other 2.0.1 test is
unaffected (before contract 0.5 the mobilityhouse station always offered
it, and 34 tests against the chargetime CSMS were KNOWN_ISSUE).

## OCPP 2.1

`mobilityhouse` declares 2.1 in both roles and `everest` as a station: the
pinned releases of Java-OCA-OCPP (v2.0) and ocpp-go (v0.19.0) have no 2.1, so
every pairing with them is one `v21.pairing` SKIP_UNSUPPORTED. The everest
station against the mobilityhouse CSMS is the pairing of two independent
2.1 implementations.

| Station ↓ / CSMS → | chargetime v2.0 | lorenzodonini v0.19.0 | mobilityhouse 2.1.0 |
|---|---|---|---|
| chargetime v2.0 | 1 skip / — | 1 skip / — | 1 skip / — |
| lorenzodonini v0.19.0 | 1 skip / — | 1 skip / — | 1 skip / — |
| mobilityhouse 2.1.0 | 1 skip / — | 1 skip / — | 50 pass 3 skip 1 known / 58 pass 1 skip 1 known |
| everest 2026.02.1 | 1 skip / — | 1 skip / — | 31 pass 21 skip 2 known / 27 pass 33 skip |

Totals: 166 PASS, 4 KNOWN_ISSUE, 72 SKIP_UNSUPPORTED, 0 FAIL, 0 INFRA_ERROR.
`v21.tls.profile2` and `v21.tls.profile3` pass for both stations with the
mobilityhouse CSMS; `v21.compression` passes for mobilityhouse with itself
(everest cannot offer compression).

| Station | CSMS | Test | Status | Evidence |
|---|---|---|---|---|
| mobilityhouse | mobilityhouse | `v21.not-implemented.from-station`, `v21.not-implemented.from-csms` | KNOWN_ISSUE (2) | [mobilityhouse/ocpp `charge_point.py`](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse | mobilityhouse | `v21.from-station.battery-swap-in` | SKIP_UNSUPPORTED | BatterySwap is not declared: ocpp 2.1.0 cannot send it (see below) |
| mobilityhouse | mobilityhouse | `v21.send.notify-periodic-event-stream` | SKIP_UNSUPPORTED | no adapter declares `send` |
| mobilityhouse | mobilityhouse | `v21.rpc.call-result-error` (from each end) | SKIP_UNSUPPORTED (2) | no adapter declares `callResultError` |
| chargetime, lorenzodonini | all three | `v21.pairing` | SKIP_UNSUPPORTED (8) | 2.1 not declared by one of the two |
| everest | mobilityhouse | `v21.from-station.report-dercontrol` | KNOWN_ISSUE | [libocpp `ocpp::v2::MessageType`](https://github.com/EVerest/everest-core/blob/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/include/ocpp/v2/types.hpp#L15-L195) has no ReportDERControl |
| everest | mobilityhouse | `v21.not-implemented.from-station` | KNOWN_ISSUE | the mobilityhouse defect above (no answer to an unanswered 2.1 action) |
| everest | mobilityhouse | `v21.from-station.battery-swap-in`, `v21.send.notify-periodic-event-stream`, `v21.rpc.call-result-error` (from each end) | SKIP_UNSUPPORTED (4) | everest does not declare BatterySwap (no CSMS declares that it handles it, so no test would run it); libocpp has no SEND and no CALLRESULTERROR |

Every smoke test ([scripts/smoke.sh](../scripts/smoke.sh)) passes: 187
checks for lorenzodonini and 188 for chargetime (1.6 and 2.0.1; chargetime
also has its 2.0.1 compression session reported as a known upstream bug),
268 for mobilityhouse (1.6, 2.0.1 and 2.1). They include, per version,
security profiles 2 and 3 with RSA and ECDSA keys and what must not connect
(wrong password, a client certificate of another identity, from another CA
or none, a server certificate from another CA or for another host name,
`ws://` to a TLS server), and compression. everest: 147 (1.6, 2.0.1 and 2.1, station checks against
the mobilityhouse CSMS).

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
| chargetime (station, 2.0.1), with `--compression` | Java-OCA-OCPP v2.0 with Java-WebSocket 1.6.0 | `MultiProtocolJSONClient` offers permessage-deflate with `client_no_context_takeover` (Java-WebSocket always adds it), but keeps its compression context: `encodeFrame` resets the deflater only when `serverNoContextTakeover` is set, which the client sets false. A CSMS that takes the offer cannot decompress the station's second message | [source](https://github.com/TooTallNate/Java-WebSocket/blob/da3cf2a777aed862f2f5b5cf060cae7969958667/src/main/java/org/java_websocket/extensions/permessage_deflate/PerMessageDeflateExtension.java#L326-L333), [MultiProtocolJSONClient](https://github.com/ChargeTimeEU/Java-OCA-OCPP/blob/f27fe5bfb03e01491100597d6859080f5ae07545/ocpp-v2/src/main/java/eu/chargetime/ocpp/MultiProtocolJSONClient.java#L94-L100) |
| lorenzodonini (station, 2.0.1), with a CSMS that validates responses | ocpp-go v0.19.0 | `ResetResponse` and `SetNetworkProfileResponse` declare `statusInfo` without `omitempty`: the station sends `"statusInfo": null`, which the OCA schema does not allow | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/reset.go#L61-L64) |
| mobilityhouse (both roles, 2.1) | ocpp 2.1.0 | `charge_point._raise_key_error` raises NotImplemented or NotSupported for 1.6, 2.0 and 2.0.1 only: a 2.1 CALL whose action has no handler gets no answer, and the sender times out. Fixed on master (ae716a1, #744), unreleased | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L161-L193) |
| mobilityhouse (station, 2.1) | ocpp 2.1.0 | `snake_to_camel_case` writes the `soc` and `soh` fields of `BatteryDataType` as `SoC` and `soh` (the schema has `soC`, `soH`), so a BatterySwapRequest fails the upstream's own validation and cannot be sent. Not in known-bugs.tsv: BatterySwap is not declared, so no test runs it. Still so on master (2026-10-04) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L57-L64) |
| lorenzodonini (both roles, 2.0.1) | ocpp-go v0.19.0 | `GetCompositeScheduleResponse` does not have the OCA structure: a top-level `evseId`, always written, which the schema does not allow; no `statusInfo`; a `schedule` of `{startDateTime, chargingSchedule}` instead of `CompositeScheduleType`. The station cannot answer with a schedule, and the CSMS drops the schedule of a correct answer | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/smartcharging/get_composite_schedule.go#L31-L50) |
| mobilityhouse (both roles, 2.1) | ocpp 2.1.0 | `snake_to_camel_case` also misspells other optional 2.1 fields: `receiptUrl` (NotifySettlement, written `receiptURL`), the per-phase `limit_L2`-style fields (`limitL2`), `fixedPF...`, `prioritizedEMAIDs`, `taxRuleID`, `soC` of `LimitAtSoCType`; a payload with one fails the upstream's own validation. Not in known-bugs.tsv: the baseline's payloads use none of them (NotifySettlement is answered with `receiptId` only) | [source](https://github.com/mobilityhouse/ocpp/blob/91f4f89140a2198dc5a300cfea88d947ac91ae28/ocpp/charge_point.py#L46-L80) |
| everest (station, 2.1) | libocpp of everest-core 2026.02.1 | `ocpp::v2::MessageType` has no ReportDERControl, GetCertificateChainStatus or NotifyWebPaymentStarted, though libocpp has their message structs: `ocpp::MessageQueue` converts every CALL's action to that enum (`ControlMessage`, `receive`), so libocpp can neither send these CALLs nor read one it receives | [source](https://github.com/EVerest/everest-core/blob/b61bb12b8cb89ee9d0fab037b05c3bb8cea9b13e/lib/everest/ocpp/include/ocpp/v2/types.hpp#L15-L195) |
| lorenzodonini (station, 2.0.1) | ocpp-go v0.19.0 | `NotifyReportRequest.Tbc` is a `bool` with `omitempty`: an explicit `"tbc": false` is dropped from the request (false is the default) | [source](https://github.com/lorenzodonini/ocpp-go/blob/a1eec917af884db0585752f909a61893aa667480/ocpp2.0.1/provisioning/notify_report.go#L98-L104) |

See [tests/known-bugs.tsv](../tests/known-bugs.tsv).
