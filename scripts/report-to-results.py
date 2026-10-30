#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Turn a test report into the organisation's interop-result-v1.

    scripts/report-to-results.py <report-interop.json> <results.json>

The report (schemas/report.schema.json) stays what scripts/interop.sh and
scripts/merge-reports.py write; results.json is derived from it. Waivers come
from tests/known-bugs.tsv and known-bugs-seen.tsv next to the report. The exit
status is 1 when the run failed: a FAIL or INFRA_ERROR, a known bug that no
longer reproduces, or no pass at all (scripts/interop_result.py).
"""

import json
import re
import sys
from pathlib import Path

import interop_result

ROOT = Path(__file__).resolve().parent.parent
OUTCOME = {"PASS": "pass", "FAIL": "fail", "SKIP_UNSUPPORTED": "unsupported",
           "KNOWN_ISSUE": "known-difference", "INFRA_ERROR": "infra-error"}


def load_waivers():
    waivers = []
    for line in (ROOT / "tests" / "known-bugs.tsv").read_text().splitlines():
        if not line.strip() or line.startswith("#"):
            continue
        spec, version, regex, evidence, summary = line.split("\t", 4)
        adapter, _, peer = spec.partition("@")
        adapter, _, role = adapter.partition(":")
        name = re.sub(r"[^a-z0-9]+", "-", (spec + "-" + regex).lower()).strip("-")
        waivers.append({"id": name, "spec": spec, "adapter": adapter, "role": role, "peer": peer, "version": version,
                        "regex": regex, "evidence": evidence, "summary": summary})
    return waivers


def applies(w, client, server, test_id):
    as_client = w["role"] != "server" and w["adapter"] == client and w["peer"] in ("", server)
    as_server = w["role"] != "client" and w["adapter"] == server and w["peer"] in ("", client)
    return (as_client or as_server) and re.search(w["regex"], test_id) is not None


def waiver_of(waivers, r):
    for w in waivers:
        if w["evidence"] == r.get("evidence") and applies(w, r["client"]["name"], r["server"]["name"], r["testId"]):
            return w
    return None


def convert(report, waivers, seen):
    results, implementations = [], {}
    for r in report["results"]:
        outcome = OUTCOME[r["status"]]
        res = {"id": r["testId"], "suite": r["testId"].split(".")[0], "feature": r["feature"], "protocolVersion": r["version"],
               "roles": {"client": r["client"]["name"], "server": r["server"]["name"], "initiator": r["initiator"]},
               "outcome": outcome, "durationMs": r["durationMs"]}
        if outcome != "pass":
            res["reason"] = r.get("error") or r.get("evidence") or r["status"]
        if outcome == "known-difference":
            w = waiver_of(waivers, r)
            if w is None:
                raise SystemExit("%s: KNOWN_ISSUE without a line in tests/known-bugs.tsv" % r["testId"])
            res["waiver"] = w["id"]
        results.append(res)
        for side in (r["client"], r["server"]):
            implementations.setdefault(side["name"], side)
    impls = []
    for name, side in sorted(implementations.items()):
        i = {"name": name, "version": side["version"], "independent": True}
        digest = side.get("image") if "@sha256:" in side.get("image", "") else interop_result.repo_digest(side.get("image", ""))
        if digest:
            i["image"] = digest
        impls.append(i)
    run = report["run"]
    return {"schemaVersion": "1.0", "repository": "otfabric/ocpp-interop",
            "interopVersion": interop_result.version_field(str(ROOT), "interop_version"),
            "contractVersion": interop_result.version_field(str(ROOT), "contract_version"),
            "gitSha": interop_result.git_sha(str(ROOT)), "startedAt": run["started"], "finishedAt": run["finished"],
            "host": interop_result.host(), "implementations": impls or [{"name": "none", "version": "none"}],
            "waivers": [{"id": w["id"], "scope": w["spec"], "evidence": w["evidence"], "reproduced": seen[w["summary"]]}
                        for w in waivers if w["summary"] in seen],
            "reproduce": "REPORT_DIR=reports make interop", "results": results}


def main(argv):
    if len(argv) != 2:
        print(__doc__, file=sys.stderr)
        return 2
    report_path, out = Path(argv[0]), Path(argv[1])
    seen = {}
    bugs = report_path.with_name("known-bugs-seen.tsv")
    if bugs.exists():
        for line in bugs.read_text().splitlines():
            state, _, text = line.partition("\t")
            seen[text] = seen.get(text, False) or state == "reproduced"
    doc = convert(json.loads(report_path.read_text()), load_waivers(), seen)
    interop_result.write(str(out), doc)
    bad = interop_result.problems(doc)
    print("%s: %s" % (out, ", ".join("%d %s" % (doc["summary"][o], o) for o in interop_result.OUTCOMES)))
    for p in bad:
        print("FAIL " + p, file=sys.stderr)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
