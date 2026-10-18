# Coverage

What the self-tests exercise, by category and version. From Phase 4 on, this
file is kept in step with `tests/`.

| Category | 1.6J | 2.0.1 | 2.1 |
|---|---|---|---|
| Connectivity: subprotocol, identity, unexpected disconnect | planned | planned | planned |
| Provisioning: BootNotification, Heartbeat, StatusNotification | planned | planned | planned |
| Authorization | planned | planned | planned |
| Transactions (Start/StopTransaction; TransactionEvent) | planned | planned | planned |
| Meter values | planned | planned | planned |
| Remote commands, from the CSMS | planned | planned | planned |
| Configuration (1.6) or device model (2.x) | planned | planned | planned |
| Smart charging | planned | planned | planned |
| Errors: NotImplemented, invalid payload, timeout | planned | planned | planned |
| Security profiles 1–3 | planned | planned | planned |
| Compression | — | planned | planned |
| SEND and CALLRESULTERROR | — | — | planned |

## Not provided on purpose

- Grading conformance to the specification. That is the purpose of OCTT.
- Charging behaviour beyond the fixture's deterministic answers.
- Tests that need real charging hardware.
