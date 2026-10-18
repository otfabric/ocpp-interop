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
| Smart charging: SetChargingProfile (TxDefaultProfile), ClearChargingProfile; GetCompositeSchedule (1.6) | tested | tested | tested |
| Reservations, local authorization list: ReserveNow, CancelReservation, GetLocalListVersion, SendLocalList (1.6); GetLocalListVersion, ReservationStatusUpdate and CancelReservation without an answer (2.x) | tested | tested | tested (no answer at all: known upstream bug) |
| Firmware and diagnostics: UpdateFirmware, GetDiagnostics and their status notifications (1.6); FirmwareStatusNotification, LogStatusNotification (2.x) | tested | tested | tested |
| Security events: SecurityEventNotification | tested (not answered) | tested | tested |
| Data transfer, from both ends | tested | tested | tested |
| Errors: NotImplemented from both ends, the fixture's CALLERROR answers, a request the upstream refuses, timeouts from both ends | tested | tested | tested (NotImplemented: known upstream bug) |
| Tariffs (2.1): SetDefaultTariff, GetTariffs, ClearTariffs | — | — | tested |
| DER control (2.1): SetDERControl, GetDERControl, ClearDERControl, NotifyDERAlarm, NotifyDERStartStop, ReportDERControl | — | — | tested |
| Periodic event streams (2.1): OpenPeriodicEventStream, ClosePeriodicEventStream, GetPeriodicEventStream, AdjustPeriodicEventStream | — | — | tested |
| Battery swap (2.1): RequestBatterySwap; BatterySwap | — | — | RequestBatterySwap tested; BatterySwap not sendable (upstream defect) |
| Security profile 1 (Basic): accepted, no credentials, wrong password | tested | tested | tested |
| Security profiles 2 and 3 (TLS) | planned | planned | planned |
| Compression | — | planned | planned |
| SEND and CALLRESULTERROR (NotifyPeriodicEventStream as a SEND) | — | — | planned: no pinned upstream release implements them; a SEND action is refused (exit 5) |
| Container contract: usage errors, exit codes, fixture validation, readiness, events, SIGTERM, health check, uid, licences | tested | tested | tested |

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
