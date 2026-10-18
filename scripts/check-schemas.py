#!/usr/bin/env python3
"""Check that every JSON schema in schemas/ and fixtures/schema/ is a valid
draft 2020-12 schema with the $id this repository publishes it under.

Usage: check-schemas.py
"""

import json
import sys
from pathlib import Path

import jsonschema

ROOT = Path(__file__).resolve().parent.parent
BASE = "https://github.com/otfabric/ocpp-interop/"


def main():
    paths = sorted([*ROOT.glob("schemas/*.json"), *ROOT.glob("fixtures/schema/*.json")])
    failed = 0
    for path in paths:
        rel = path.relative_to(ROOT).as_posix()
        try:
            schema = json.loads(path.read_text())
            jsonschema.Draft202012Validator.check_schema(schema)
        except (ValueError, jsonschema.SchemaError) as e:
            print(f"FAIL {rel}: {str(e)[:200]}")
            failed += 1
            continue
        if schema.get("$id") != BASE + rel:
            print(f"FAIL {rel}: $id is {schema.get('$id')!r}, expected {BASE + rel!r}")
            failed += 1
            continue
        print(f"ok   {rel}")
    if not paths:
        print("no schemas found", file=sys.stderr)
        return 1
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main())
