# Coverage

What the self-tests exercise, by category and version. Kept in step with
[scripts/smoke.sh](scripts/smoke.sh) and [scripts/interop.sh](scripts/interop.sh);
the results are in [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md). OCPP 2.1 is
tested with `mobilityhouse` only, the one adapter whose pinned upstream
release has it.

| Category | 1.6J | 2.0.1 | 2.1 |
|---|---|---|---|
| Connectivity: identity in the path, unknown identity refused, `client run` and one-shot clients, close seen by the CSMS, lost connection, handshake that fails | tested | tested | tested |
| Provisioning: BootNotification, Heartbeat, StatusNotification | tested | tested | tested |
| Authorization: accepted, blocked and unknown tokens (derived answers) | tested | tested | tested |
| Transactions: StartTransaction (derived), StopTransaction (1.6); TransactionEvent Started, Updated, Ended (2.x) | tested | tested | tested |
| Meter values | tested | tested | tested |
| Remote commands, from the CSMS: RemoteStart/StopTransaction (1.6) or RequestStart/StopTransaction (2.x), Reset, UnlockConnector, ChangeAvailability, TriggerMessage, ClearCache | tested | tested | tested |
| Configuration (1.6): GetConfiguration (derived, known and unknown keys), ChangeConfiguration | tested | — | — |
| Device model (2.x): GetVariables and SetVariables (derived: unknown component, unknown variable, case-insensitive names, read-only), GetBaseReport, NotifyReport, NotifyEvent | — | tested | tested |
| Smart charging: SetChargingProfile (TxDefaultProfile), ClearChargingProfile, GetCompositeSchedule; GetChargingProfiles, ReportChargingProfiles, NotifyChargingLimit, ClearedChargingLimit, NotifyEVChargingNeeds, NotifyEVChargingSchedule (2.x) | tested | tested (GetCompositeSchedule with lorenzodonini: known upstream bug) | tested |
| Reservations, local authorization list: ReserveNow, CancelReservation, ReservationStatusUpdate (2.x), GetLocalListVersion, SendLocalList | tested | tested | tested |
| Firmware and diagnostics: UpdateFirmware, GetDiagnostics and their status notifications (1.6); FirmwareStatusNotification, LogStatusNotification (2.x) | tested | tested | tested |
| Security events: SecurityEventNotification | tested | tested | tested |
| Certificates: SignCertificate, CertificateSigned, InstallCertificate, DeleteCertificate, GetInstalledCertificateIds (1.6 Security Whitepaper, 2.x); GetCertificateStatus, Get15118EVCertificate (2.x); GetCertificateChainStatus (2.1) | tested | tested | tested |
| Logs and secure firmware: GetLog, LogStatusNotification, SignedUpdateFirmware, SignedFirmwareStatusNotification, ExtendedTriggerMessage (1.6); GetLog, UpdateFirmware, PublishFirmware, UnpublishFirmware, PublishFirmwareStatusNotification (2.x) | tested | tested | tested |
| Monitoring (2.x): SetVariableMonitoring, ClearVariableMonitoring, SetMonitoringBase, SetMonitoringLevel, GetMonitoringReport, NotifyMonitoringReport | — | tested | tested |
| Display messages and customer information (2.x): SetDisplayMessage, GetDisplayMessages, ClearDisplayMessage, NotifyDisplayMessages, CustomerInformation, NotifyCustomerInformation | — | tested | tested |
| Provisioning beyond the basics (2.x): GetReport, SetNetworkProfile, GetTransactionStatus, CostUpdated | — | tested | tested |
| Data transfer, from both ends | tested | tested | tested |
| Errors: NotImplemented from both ends (a copy of the fixture without one answer per side), the fixture's CALLERROR answers, a request the upstream refuses, timeouts from both ends | tested | tested | tested (NotImplemented: known upstream bug) |
| Tariffs (2.1): SetDefaultTariff, GetTariffs, ClearTariffs | — | — | tested |
| DER control (2.1): SetDERControl, GetDERControl, ClearDERControl, NotifyDERAlarm, NotifyDERStartStop, ReportDERControl | — | — | tested |
| Periodic event streams (2.1): OpenPeriodicEventStream, ClosePeriodicEventStream, GetPeriodicEventStream, AdjustPeriodicEventStream | — | — | tested |
| Battery swap (2.1): RequestBatterySwap; BatterySwap | — | — | RequestBatterySwap tested; BatterySwap not sendable (upstream defect) |
| Payment and settlement (2.1): NotifySettlement, VatNumberValidation, NotifyWebPaymentStarted, ChangeTransactionTariff | — | — | tested |
| Priority and dynamic charging (2.1): UsePriorityCharging, NotifyPriorityCharging, UpdateDynamicSchedule, PullDynamicScheduleUpdate, NotifyAllowedEnergyTransfer, AFRRSignal | — | — | tested |
| Security profile 1 (Basic): accepted, no credentials, wrong password | tested | tested | tested |
| Security profiles 2 and 3 (TLS) | planned | planned | planned |
| Compression | — | planned | planned |
| SEND and CALLRESULTERROR (NotifyPeriodicEventStream as a SEND) | — | — | planned: no pinned upstream release implements them; a SEND action is refused (exit 5) |
| Container contract: usage errors, exit codes, fixture validation, readiness, events, SIGTERM, health check, uid, licences | tested | tested | tested |

## Actions

Every action of every version has at least one sample in the baseline
fixture, and an answer from the role that receives it (docs/FIXTURES.md).
Actions that appear in a sample, and the actions each adapter declares
(the same for the three adapters in 1.6 and 2.0.1; 2.1 is mobilityhouse
only):

| | 1.6 | 2.0.1 | 2.1 |
|---|---|---|---|
| Actions of the version | 39 | 64 | 91 |
| In a sample (before: v0.2.0) | 39 (30) | 64 (27) | 91 (43) |
| Station sends / handles | 14 / 26 (11 / 19) | 25 / 40 (13 / 14) | 35 / 55 (18 / 23) |
| CSMS sends / handles | 26 / 14 (20 / 10) | 40 / 25 (15 / 12) | 55 / 35 (24 / 17) |
| Exchanged with a PASS between at least one pairing | 39 | 64 | 89 |

Not exchanged by any pairing, with the reason:

| Version | Action | Why |
|---|---|---|
| 2.1 | BatterySwap | ocpp 2.1.0 (the only 2.1 adapter) cannot send it: `snake_to_camel_case` writes `soC` and `soH` as `SoC` and `soh` (adapters/mobilityhouse/README.md) |
| 2.1 | NotifyPeriodicEventStream | a SEND; no pinned upstream release implements SEND frames |

Exchanged, but not with every adapter: 2.0.1 GetCompositeSchedule fails
with lorenzodonini in either role (ocpp-go's response type does not have the
OCA structure; tests/known-bugs.tsv), and passes between mobilityhouse and
chargetime.

Every CALL is checked at both ends: the result document of the sender
against the fixture's answer, and the events of both (`call-sent`,
`call-received`, `call-answered`, `call-result`), with matching message
ids where both adapters expose them.

## Not provided on purpose

- Grading conformance to the specification. That is the purpose of OCTT.
- Charging behaviour beyond the fixture's deterministic answers: a station
  that accepts a RemoteStartTransaction or a TriggerMessage does not then
  start a transaction or send the triggered message.
- Tests that need real charging hardware.
