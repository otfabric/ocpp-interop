# Capabilities

`ocpp-interop print-capabilities` prints one JSON line, which validates
against [schemas/capabilities.schema.json](../schemas/capabilities.schema.json).
A consumer reads it to decide which scenarios to run against an image, and
reports the rest as `SKIP_UNSUPPORTED`.

## Rules

1. A capability is `true`, or an action is listed, only when the self-tests
   of this repository exercise it (REQUIREMENTS.md R4.2). An upstream's
   documentation alone is not enough.
2. Capabilities are declared per OCPP version and per role. Support for a
   feature in 1.6 says nothing about 2.0.1.
3. `sends` lists the actions the adapter can send in that role, and
   `handles` the actions it answers. An action missing from these lists is
   unsupported.
4. `notes` explains limitations in plain words, with a link to evidence
   where there is one.

## Example (illustrative; no adapter is published yet)

```json
{
  "schemaVersion": "1.0",
  "adapter": "mobilityhouse",
  "adapterVersion": "v0.1.0",
  "contractVersion": "1.0",
  "upstream": {
    "name": "The Mobility House ocpp",
    "repository": "https://github.com/mobilityhouse/ocpp",
    "version": "2.1.0",
    "revision": "0000000000000000000000000000000000000000",
    "license": "MIT",
    "language": "Python",
    "independentEngine": true
  },
  "versions": {
    "1.6": {
      "client": {"supported": true, "sends": ["BootNotification", "Heartbeat"], "handles": ["Reset"]},
      "server": {"supported": true, "sends": ["Reset"], "handles": ["BootNotification", "Heartbeat"]},
      "features": {"securityProfiles": [0, 1], "compression": false, "send": false, "callResultError": false}
    }
  }
}
```

## Feature keys

| Key | Meaning |
|---|---|
| `securityProfiles` | Profiles exercised: 0 (none), 1 (Basic over `ws://`), 2 (TLS + Basic), 3 (mutual TLS) |
| `compression` | `permessage-deflate` negotiated and used |
| `send` | OCPP 2.1 SEND frames sent and received |
| `callResultError` | OCPP 2.1 CALLRESULTERROR frames sent and received |

Further boolean keys may be added in a minor version of the contract.
