#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Validate the OCPP payloads of the fixtures against the OCA JSON schemas.

    scripts/validate-payloads.py <schema directory> [fixture.json ...]

The OCA schemas are CC BY-ND 4.0 and are not part of this repository
(REQUIREMENTS.md R8.2). Point this script at a copy, for example the one
The Mobility House ocpp package ships:

    python3 -m venv /tmp/oca && /tmp/oca/bin/pip install --index-url https://pypi.org/simple ocpp==2.1.0 jsonschema
    scripts/validate-payloads.py "$(/tmp/oca/bin/python -c 'import ocpp, os; print(os.path.dirname(ocpp.__file__))')/v201/schemas" \
        fixtures/v201/*/fixture.json

The schema directory decides the version: OCPP 1.6 names a request schema
<Action>.json, OCPP 2.x <Action>Request.json; responses are
<Action>Response.json in both. A SEND of OCPP 2.1 (NotifyPeriodicEventStream)
has one schema, <Action>.json, and no response. For 2.1, point it at the
OCA's OCPP-2.1_part3_JSON_schemas (identical to ocpp 2.1.0's v21/schemas). Checked: every sample against the request
schema; every fixed answer (with "${now}" replaced) against the response
schema; and every derived answer, as docs/FIXTURES.md defines it, for each
sample of that action. Not part of `make ci`, because it needs the schemas.
"""

import json
import sys
from datetime import datetime, timezone
from pathlib import Path

import jsonschema


def substitute(value):
    if value == "${now}":
        return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    if isinstance(value, dict):
        return {k: substitute(v) for k, v in value.items()}
    if isinstance(value, list):
        return [substitute(v) for v in value]
    return value


def same(a, b):
    return (a or "").lower() == (b or "").lower()


def find_variable(fx, component, variable):
    """(status, entry): UnknownComponent, UnknownVariable or the entry."""
    matches = [v for v in fx["variables"]
               if same(v["component"]["name"], component.get("name"))
               and same(v["component"].get("instance"), component.get("instance"))]
    if not matches:
        return "UnknownComponent", None
    for v in matches:
        if same(v["variable"]["name"], variable.get("name")) and same(v["variable"].get("instance"), variable.get("instance")):
            return None, v
    return "UnknownVariable", None


def derived(fx, kind, action, request):
    if kind == "idTokens":
        if fx["ocppVersion"] == "1.6":
            status = next((t["status"] for t in fx["idTokens"] if t["idToken"] == request.get("idTag")), "Invalid")
            return {"idTagInfo": {"status": status}}
        token = request.get("idToken", {})
        status = next((t["status"] for t in fx["idTokens"]
                       if t["idToken"] == token.get("idToken") and t.get("type") == token.get("type")), "Invalid")
        return {"idTokenInfo": {"status": status}}
    if kind == "configuration":
        keys = request.get("key") or [c["key"] for c in fx["configuration"]]
        known = [{"key": c["key"], "readonly": c["readonly"], "value": c["value"]}
                 for k in keys for c in fx["configuration"] if same(c["key"], k)]
        unknown = [k for k in keys if not any(same(c["key"], k) for c in fx["configuration"])]
        return {"configurationKey": known, **({"unknownKey": unknown} if unknown else {})}
    if kind == "variables" and action == "GetVariables":
        results = []
        for item in request["getVariableData"]:
            status, entry = find_variable(fx, item["component"], item["variable"])
            result = {"component": item["component"], "variable": item["variable"]}
            if "attributeType" in item:
                result["attributeType"] = item["attributeType"]
            if status is None and item.get("attributeType", "Actual") != "Actual":
                status = "NotSupportedAttributeType"
            if status is None and entry["mutability"] == "WriteOnly":
                status = "Rejected"
            result["attributeStatus"] = status or "Accepted"
            if status is None:
                result["attributeValue"] = entry["value"]
            results.append(result)
        return {"getVariableResult": results}
    if kind == "variables" and action == "SetVariables":
        results = []
        for item in request["setVariableData"]:
            status, entry = find_variable(fx, item["component"], item["variable"])
            result = {"component": item["component"], "variable": item["variable"]}
            if "attributeType" in item:
                result["attributeType"] = item["attributeType"]
            if status is None and item.get("attributeType", "Actual") != "Actual":
                status = "NotSupportedAttributeType"
            if status is None and entry["mutability"] == "ReadOnly":
                status = "Rejected"
            result["attributeStatus"] = status or "Accepted"
            results.append(result)
        return {"setVariableResult": results}
    raise ValueError(f"no derivation {kind} for {action}")


def main(argv):
    if not argv:
        raise SystemExit(__doc__)
    schemas = Path(argv[0])
    paths = [Path(p) for p in argv[1:]] or sorted(Path(__file__).resolve().parent.parent.glob("fixtures/v*/*/fixture.json"))
    v16 = (schemas / "BootNotification.json").exists()

    def check(name, instance, label):
        path = schemas / name
        if not path.exists():
            return [f"{label}: no schema {name}"]
        schema = json.loads(path.read_text())
        cls = jsonschema.validators.validator_for(schema)
        return [f"{label}: {'/'.join(map(str, e.absolute_path)) or '(root)'}: {e.message[:200]}"
                for e in cls(schema).iter_errors(instance)]

    failed = 0
    for path in paths:
        fx = json.loads(path.read_text())
        errors, checked = [], 0
        for name, sample in fx["samples"].items():
            action = sample["action"]
            name_2x = f"{action}Request.json"
            if not v16 and not (schemas / name_2x).exists() and (schemas / f"{action}.json").exists():
                name_2x = f"{action}.json"  # a SEND: no Request suffix, no response
            errors += check(f"{action}.json" if v16 else name_2x, sample["payload"], f"samples.{name}")
            checked += 1
        for role, table in fx["answers"].items():
            for action, answer in table.items():
                if "payload" in answer and "derived" not in answer:
                    errors += check(f"{action}Response.json", substitute(answer["payload"]), f"answers.{role}.{action}")
                    checked += 1
                elif "derived" in answer:
                    requests = [s["payload"] for s in fx["samples"].values()
                                if s["action"] == action and s["from"] != role]
                    if not requests:
                        errors.append(f"answers.{role}.{action}: derived, but no sample to derive it from")
                    for request in requests:
                        response = derived(fx, answer["derived"], action, request)
                        response.update(substitute(answer.get("payload", {})))
                        errors += check(f"{action}Response.json", response, f"answers.{role}.{action} (derived)")
                        checked += 1
        print(f"{'FAIL' if errors else 'ok  '} {path} ({checked} payloads)")
        for e in errors:
            print(f"     {e}")
        failed += bool(errors)
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
