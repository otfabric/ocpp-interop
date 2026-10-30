#!/usr/bin/env python3
"""Validate kept documents against the contract schemas.

Usage: validate-documents.py <directory>

The self-tests keep every document they produce when REPORT_DIR is set. A
file with "repository" is an interop-result-v1 document (results.json), one
with "versions" a capability document, one with "operation" a result document,
and one with "results" a test report. Needs the jsonschema package.
"""

import json
import sys
from pathlib import Path

import jsonschema

ROOT = Path(__file__).resolve().parent.parent
KINDS = (
    ("repository", "interop results", "interop-result-v1.schema.json"),
    ("results", "reports", "report.schema.json"),
    ("operation", "results", "client-result.schema.json"),
    ("versions", "capabilities", "capabilities.schema.json"),
)


def validator(name):
    schema = json.loads((ROOT / "schemas" / name).read_text())
    jsonschema.Draft202012Validator.check_schema(schema)
    return jsonschema.Draft202012Validator(schema, format_checker=jsonschema.FormatChecker())


def main(argv):
    if len(argv) != 1:
        print(__doc__, file=sys.stderr)
        return 2
    validators = {kind: validator(schema) for _, kind, schema in KINDS}
    counts = {kind: 0 for _, kind, _ in KINDS}
    failed = 0
    for path in sorted(Path(argv[0]).glob("*.json")):
        try:
            doc = json.loads(path.read_text())
        except ValueError as e:
            print(f"FAIL {path.name}: not JSON: {e}")
            failed += 1
            continue
        kind = next((k for key, k, _ in KINDS if isinstance(doc, dict) and key in doc), None)
        if kind is None:
            print(f"FAIL {path.name}: not a capability document, result document or report")
            failed += 1
            continue
        counts[kind] += 1
        for e in validators[kind].iter_errors(doc):
            where = "/".join(str(x) for x in e.absolute_path) or "(root)"
            print(f"FAIL {path.name}: {where}: {e.message[:200]}")
            failed += 1
    print(", ".join(f"{n} {k}" for k, n in counts.items()) + f", {failed} errors")
    if sum(counts.values()) == 0:
        print("no documents found", file=sys.stderr)
        return 1
    return 1 if failed else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
