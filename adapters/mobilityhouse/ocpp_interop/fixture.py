# SPDX-License-Identifier: MIT
"""Fixture loading, validation and answers (docs/FIXTURES.md).

The structural rules are fixtures/schema/ocpp-fixture.schema.json, checked
with jsonschema (a dependency of the upstream package, so it is in the image
anyway). The cross-field rules are those of scripts/validate-fixtures.py,
which the image carries as validate_fixtures.py: one implementation of the
rules for the repository and this adapter. Neither is OCPP protocol logic.
"""

from __future__ import annotations

import copy
import json
from datetime import datetime, timezone
from pathlib import Path

FIXTURES_DIR = Path("/fixtures")
SCHEMA = FIXTURES_DIR / "schema" / "ocpp-fixture.schema.json"
VERSION_DIRS = {"1.6": "v16", "2.0.1": "v201", "2.1": "v21"}


class FixtureError(Exception):
    """The fixture is missing or invalid: exit 4."""


class CallErrorAnswer:
    """The fixture answers this action with a CALLERROR."""

    def __init__(self, code: str, description: str):
        self.code = code
        self.description = description


def default_path(version: str, name: str = "baseline") -> Path:
    return FIXTURES_DIR / VERSION_DIRS[version] / name / "fixture.json"


def now() -> str:
    """The current UTC time, RFC 3339, whole seconds: what "${now}" becomes."""
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _substitute(value):
    if value == "${now}":
        return now()
    if isinstance(value, dict):
        return {k: _substitute(v) for k, v in value.items()}
    if isinstance(value, list):
        return [_substitute(v) for v in value]
    return value


class Fixture:
    """A validated fixture."""

    def __init__(self, doc: dict, path: Path):
        self.doc = doc
        self.path = path
        self.name = doc["name"]
        self.version = doc["ocppVersion"]
        self.stations = {s["id"]: s for s in doc["stations"]}
        self.first_station = doc["stations"][0]["id"]
        self.id_tokens = {t["idToken"]: t["status"] for t in doc["idTokens"]}
        self.configuration = doc.get("configuration", [])

    def answers(self, role: str) -> dict:
        """The answer table of a role: 'csms' or 'station'."""
        return self.doc["answers"][role]

    def answer(self, role: str, action: str, request: dict):
        """The answer of `role` to a CALL: a payload dict, a CallErrorAnswer,
        or None when the fixture has no answer (NotImplemented)."""
        entry = self.answers(role).get(action)
        if entry is None:
            return None
        if "callError" in entry:
            return CallErrorAnswer(entry["callError"]["code"], entry["callError"]["description"])
        if "derived" not in entry:
            return _substitute(copy.deepcopy(entry["payload"]))
        derived = entry["derived"]
        if derived == "idTokens":
            # 1.6: idTagInfo with the status of the table; unknown is Invalid.
            answer = {"idTagInfo": {"status": self.id_tokens.get(request.get("idTag"), "Invalid")}}
        elif derived == "configuration":
            answer = self._configuration(request)
        else:
            # Only 1.6 fixtures are accepted (see load()); variables are 2.x.
            raise FixtureError(f"derived answer {derived!r} is not available for OCPP {self.version}")
        answer.update(_substitute(copy.deepcopy(entry.get("payload", {}))))
        return answer

    def _configuration(self, request: dict) -> dict:
        """GetConfiguration: the requested keys, or all of them when none is
        requested. Keys are compared case-insensitively (CiString50Type)."""
        by_key = {c["key"].lower(): c for c in self.configuration}
        wanted = request.get("key") or [c["key"] for c in self.configuration]
        known, unknown = [], []
        for key in wanted:
            item = by_key.get(str(key).lower())
            if item is None:
                unknown.append(key)
            else:
                known.append({"key": item["key"], "readonly": item["readonly"], "value": item["value"]})
        answer = {"configurationKey": known}
        if unknown:
            answer["unknownKey"] = unknown
        return answer


def load(path: Path, version: str) -> Fixture:
    """Read and validate a fixture; FixtureError on any violation."""
    try:
        doc = json.loads(Path(path).read_text(encoding="utf-8"))
    except (OSError, ValueError) as e:
        raise FixtureError(f"{path}: {e}") from e
    errors = []
    try:
        import jsonschema  # pylint: disable=import-outside-toplevel

        schema = json.loads(SCHEMA.read_text(encoding="utf-8"))
        validator = jsonschema.Draft202012Validator(schema)
        errors += [f"{'/'.join(map(str, e.absolute_path)) or '(root)'}: {e.message[:200]}"
                   for e in validator.iter_errors(doc)]
    except (OSError, ValueError) as e:
        raise FixtureError(f"cannot read the fixture schema {SCHEMA}: {e}") from e
    if not errors:
        from validate_fixtures import semantic_errors  # pylint: disable=import-outside-toplevel

        errors += semantic_errors(doc, Path(path).resolve())
    if not errors and doc.get("ocppVersion") != version:
        errors.append(f"ocppVersion is {doc.get('ocppVersion')!r}, but --version is {version}")
    if errors:
        raise FixtureError(f"{path}: " + "; ".join(errors))
    return Fixture(doc, Path(path))
