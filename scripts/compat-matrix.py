#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Prints the compatibility matrix of a test report as Markdown.

    scripts/compat-matrix.py reports/report-interop.json

One table per OCPP version: a row per client (station) adapter, a column per
server (CSMS) adapter, and in each cell the counts of PASS, FAIL,
SKIP_UNSUPPORTED, KNOWN_ISSUE and INFRA_ERROR, first for the CALLs the
station sent, then for those the CSMS sent. Standard library only.
"""

import json
import sys
from collections import Counter, defaultdict

STATUSES = ("PASS", "FAIL", "SKIP_UNSUPPORTED", "KNOWN_ISSUE", "INFRA_ERROR")
SHORT = {"PASS": "pass", "FAIL": "fail", "SKIP_UNSUPPORTED": "skip", "KNOWN_ISSUE": "known", "INFRA_ERROR": "infra"}


def counts(results):
    c = Counter(r["status"] for r in results)
    return " ".join(f"{c[s]} {SHORT[s]}" for s in STATUSES if c[s]) or "—"


def main(argv):
    if len(argv) != 1:
        raise SystemExit(__doc__)
    with open(argv[0], encoding="utf-8") as handle:
        report = json.load(handle)
    by_version = defaultdict(list)
    for result in report["results"]:
        by_version[result["version"]].append(result)
    for version in sorted(by_version):
        results = by_version[version]
        clients = sorted({r["client"]["name"] for r in results})
        servers = sorted({r["server"]["name"] for r in results})
        upstream = {r["client"]["name"]: r["client"]["version"] for r in results}
        upstream.update({r["server"]["name"]: r["server"]["version"] for r in results})
        print(f"### OCPP {version}\n")
        print("Each cell: CALLs sent by the station (client) / by the CSMS (server).\n")
        print("| Station ↓ / CSMS → | " + " | ".join(f"{s} {upstream[s]}" for s in servers) + " |")
        print("|---|" + "---|" * len(servers))
        for client in clients:
            cells = []
            for server in servers:
                pair = [r for r in results if r["client"]["name"] == client and r["server"]["name"] == server]
                cells.append(" / ".join(counts([r for r in pair if r["initiator"] == i]) for i in ("client", "server")))
            print(f"| {client} {upstream[client]} | " + " | ".join(cells) + " |")
        print()
        special = [r for r in results if r["status"] not in ("PASS",)]
        if special:
            print("| Station | CSMS | Test | Status | Evidence or error |")
            print("|---|---|---|---|---|")
            for r in sorted(special, key=lambda r: (r["status"], r["testId"], r["client"]["name"], r["server"]["name"])):
                note = r.get("evidence") or r.get("error") or ""
                print(f"| {r['client']['name']} | {r['server']['name']} | `{r['testId']}` | {r['status']} | {note} |")
            print()
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
