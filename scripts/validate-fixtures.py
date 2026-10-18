#!/usr/bin/env python3
"""Validate every fixtures/<version>/<name>/fixture.json.

Two layers, both required:

  1. The structural rules of fixtures/schema/ocpp-fixture.schema.json, checked
     with the `jsonschema` package when it is installed. CI installs it, so a
     missing package is an error there (--require-schema).
  2. The cross-field rules a JSON Schema cannot express:
     - the name matches the directory and the version directory matches
       ocppVersion;
     - station identities, id tokens, configuration keys and variables are
       unique;
     - every answer is for an action that the answering role receives in
       that version, and every sample is sent by the role that sends it;
     - a derived answer names a table that fits the action.

The adapters apply the same rules when they load a fixture. This script makes
a broken fixture fail before any image is built.

Usage: validate-fixtures.py [--require-schema] [fixture.json ...]
"""

import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SCHEMA = ROOT / "fixtures" / "schema" / "ocpp-fixture.schema.json"
VERSION_DIRS = {"1.6": "v16", "2.0.1": "v201", "2.1": "v21"}

# Actions by the role that sends them. From the OCPP 1.6 specification with
# the Security Whitepaper extensions, and OCPP 2.0.1 Part 2. DataTransfer goes
# both ways.
V16_FROM_STATION = {
    "Authorize", "BootNotification", "DataTransfer", "DiagnosticsStatusNotification",
    "FirmwareStatusNotification", "Heartbeat", "MeterValues", "StartTransaction",
    "StatusNotification", "StopTransaction",
    "LogStatusNotification", "SecurityEventNotification", "SignCertificate",
    "SignedFirmwareStatusNotification",
}
V16_FROM_CSMS = {
    "CancelReservation", "ChangeAvailability", "ChangeConfiguration", "ClearCache",
    "ClearChargingProfile", "DataTransfer", "GetCompositeSchedule", "GetConfiguration",
    "GetDiagnostics", "GetLocalListVersion", "RemoteStartTransaction",
    "RemoteStopTransaction", "ReserveNow", "Reset", "SendLocalList", "SetChargingProfile",
    "TriggerMessage", "UnlockConnector", "UpdateFirmware",
    "CertificateSigned", "DeleteCertificate", "ExtendedTriggerMessage",
    "GetInstalledCertificateIds", "GetLog", "InstallCertificate", "SignedUpdateFirmware",
}
V201_FROM_STATION = {
    "Authorize", "BootNotification", "ClearedChargingLimit", "DataTransfer",
    "FirmwareStatusNotification", "Get15118EVCertificate", "GetCertificateStatus",
    "Heartbeat", "LogStatusNotification", "MeterValues", "NotifyChargingLimit",
    "NotifyCustomerInformation", "NotifyDisplayMessages", "NotifyEVChargingNeeds",
    "NotifyEVChargingSchedule", "NotifyEvent", "NotifyMonitoringReport", "NotifyReport",
    "PublishFirmwareStatusNotification", "ReportChargingProfiles",
    "ReservationStatusUpdate", "SecurityEventNotification", "SignCertificate",
    "StatusNotification", "TransactionEvent",
}
V201_FROM_CSMS = {
    "CancelReservation", "CertificateSigned", "ChangeAvailability", "ClearCache",
    "ClearChargingProfile", "ClearDisplayMessage", "ClearVariableMonitoring",
    "CostUpdated", "CustomerInformation", "DataTransfer", "DeleteCertificate",
    "GetBaseReport", "GetChargingProfiles", "GetCompositeSchedule", "GetDisplayMessages",
    "GetInstalledCertificateIds", "GetLocalListVersion", "GetLog", "GetMonitoringReport",
    "GetReport", "GetTransactionStatus", "GetVariables", "InstallCertificate",
    "PublishFirmware", "RequestStartTransaction", "RequestStopTransaction", "ReserveNow",
    "Reset", "SendLocalList", "SetChargingProfile", "SetDisplayMessage",
    "SetMonitoringBase", "SetMonitoringLevel", "SetNetworkProfile",
    "SetVariableMonitoring", "SetVariables", "TriggerMessage", "UnlockConnector",
    "UnpublishFirmware", "UpdateFirmware",
}
# 2.1 fixtures are rejected until the 2.1 action list is checked against
# OCPP 2.1 Edition 2 (PLAN.md, Phase 6).
ACTIONS = {
    "1.6": {"station": V16_FROM_STATION, "csms": V16_FROM_CSMS},
    "2.0.1": {"station": V201_FROM_STATION, "csms": V201_FROM_CSMS},
}
DERIVED = {
    "idTokens": {"Authorize", "StartTransaction"},
    "configuration": {"GetConfiguration"},
    "variables": {"GetVariables", "SetVariables"},
}
ID_TAG_MAX = {"1.6": 20, "2.0.1": 36}


def duplicates(values):
    seen, dup = set(), set()
    for v in values:
        (dup if v in seen else seen).add(v)
    return sorted(dup)


def semantic_errors(fx, path):
    errors = []
    version = fx.get("ocppVersion")
    if fx.get("name") != path.parent.name:
        errors.append(f"name {fx.get('name')!r} is not the directory name {path.parent.name!r}")
    if VERSION_DIRS.get(version) != path.parent.parent.name:
        errors.append(f"ocppVersion {version!r} does not belong in {path.parent.parent.name}/")
    if version not in ACTIONS:
        errors.append(f"ocppVersion {version!r}: no action list yet, so fixtures cannot be checked")
        return errors

    for d in duplicates(s.get("id") for s in fx.get("stations", [])):
        errors.append(f"stations: identity {d!r} appears twice")
    for d in duplicates(t.get("idToken") for t in fx.get("idTokens", [])):
        errors.append(f"idTokens: {d!r} appears twice")
    for t in fx.get("idTokens", []):
        if len(t.get("idToken", "")) > ID_TAG_MAX[version]:
            errors.append(f"idTokens: {t.get('idToken')!r} is longer than {ID_TAG_MAX[version]} characters")
        if version != "1.6" and "type" not in t:
            errors.append(f"idTokens: {t.get('idToken')!r} has no type, which OCPP 2.x requires")
    for d in duplicates(c.get("key") for c in fx.get("configuration", [])):
        errors.append(f"configuration: key {d!r} appears twice")
    for d in duplicates(
        (v["component"].get("name"), v["component"].get("instance"), v["variable"].get("name"), v["variable"].get("instance"))
        for v in fx.get("variables", []) if "component" in v and "variable" in v
    ):
        errors.append(f"variables: {d!r} appears twice")
    for s in fx.get("stations", []):
        for d in duplicates(e.get("id") for e in s.get("evses", [])):
            errors.append(f"stations {s.get('id')!r}: EVSE {d} appears twice")

    # An answer is given by the role that receives the action.
    receives = {"csms": ACTIONS[version]["station"], "station": ACTIONS[version]["csms"]}
    for role, table in fx.get("answers", {}).items():
        for action, answer in table.items():
            if action not in receives.get(role, set()):
                errors.append(f"answers.{role}.{action}: the {role} does not receive {action} in OCPP {version}")
            derived = answer.get("derived") if isinstance(answer, dict) else None
            if derived and action not in DERIVED.get(derived, set()):
                errors.append(f"answers.{role}.{action}: cannot be derived from {derived}")
            if derived and not fx.get(derived):
                errors.append(f"answers.{role}.{action}: derived from {derived}, which is empty")

    for name, sample in fx.get("samples", {}).items():
        sender = sample.get("from")
        if sample.get("action") not in ACTIONS[version].get(sender, set()):
            errors.append(f"samples.{name}: the {sender} does not send {sample.get('action')} in OCPP {version}")
    return errors


def main(argv):
    require_schema = "--require-schema" in argv
    paths = [Path(a) for a in argv if not a.startswith("--")]
    if not paths:
        paths = sorted(ROOT.glob("fixtures/v*/*/fixture.json"))
    if not paths:
        print("no fixtures found", file=sys.stderr)
        return 1

    try:
        import jsonschema
    except ImportError:
        if require_schema:
            print("the jsonschema package is required (pip install jsonschema)", file=sys.stderr)
            return 1
        jsonschema = None
        print("warning: jsonschema is not installed; checking cross-field rules only", file=sys.stderr)

    schema = json.loads(SCHEMA.read_text())
    if jsonschema:
        jsonschema.Draft202012Validator.check_schema(schema)
        validator = jsonschema.Draft202012Validator(schema)

    failed = 0
    for path in paths:
        path = path.resolve()
        try:
            fx = json.loads(path.read_text())
        except (OSError, json.JSONDecodeError) as e:
            print(f"FAIL {path.relative_to(ROOT)}: {e}")
            failed += 1
            continue
        errors = []
        if jsonschema:
            errors += [f"{'/'.join(map(str, e.absolute_path)) or '(root)'}: {e.message}"
                       for e in sorted(validator.iter_errors(fx), key=lambda e: list(e.absolute_path))]
        # Messages of `not` and `oneOf` quote the whole instance: keep them short.
        errors = [e if len(e) <= 200 else e[:197] + "..." for e in errors]
        errors += semantic_errors(fx, path)
        rel = path.relative_to(ROOT)
        if errors:
            failed += 1
            print(f"FAIL {rel}")
            for e in errors:
                print(f"     {e}")
        else:
            print(f"ok   {rel}")
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
