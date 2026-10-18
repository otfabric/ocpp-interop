# Coverage

What the self-tests exercise, by category and version. Kept in step with
[scripts/smoke.sh](scripts/smoke.sh) and [scripts/interop.sh](scripts/interop.sh);
the results are in [docs/COMPATIBILITY.md](docs/COMPATIBILITY.md).

| Category | 1.6J | 2.0.1 | 2.1 |
|---|---|---|---|
| Connectivity: identity in the path, unknown identity refused, `client run` and one-shot clients, close seen by the CSMS, lost connection, handshake that fails | tested | planned | planned |
| Provisioning: BootNotification, Heartbeat, StatusNotification | tested | planned | planned |
| Authorization: accepted, blocked and unknown tokens (derived answers) | tested | planned | planned |
| Transactions: StartTransaction (derived), StopTransaction | tested | planned | planned |
| Meter values | tested | planned | planned |
| Remote commands, from the CSMS: RemoteStart/StopTransaction, Reset, UnlockConnector, ChangeAvailability, TriggerMessage, ClearCache | tested | planned | planned |
| Configuration: GetConfiguration (derived, known and unknown keys), ChangeConfiguration | tested | planned | planned |
| Smart charging: SetChargingProfile (TxDefaultProfile), ClearChargingProfile, GetCompositeSchedule | tested | planned | planned |
| Reservations, local authorization list | tested | planned | planned |
| Firmware and diagnostics: UpdateFirmware, GetDiagnostics, their status notifications | tested | planned | planned |
| Data transfer, from both ends | tested | planned | planned |
| Errors: NotImplemented from both ends, the fixture's CALLERROR answers, a request the upstream refuses, timeouts from both ends | tested | planned | planned |
| Security profile 1 (Basic): accepted, no credentials, wrong password | tested | planned | planned |
| Security profiles 2 and 3 (TLS) | planned | planned | planned |
| Compression | — | planned | planned |
| SEND and CALLRESULTERROR | — | — | planned |
| Container contract: usage errors, exit codes, fixture validation, readiness, events, SIGTERM, health check, uid, licences | tested | planned | planned |

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
