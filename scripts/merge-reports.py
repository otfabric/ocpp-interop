#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Merge the reports of interop shards into one report.

    scripts/merge-reports.py <out.json> <report.json> [<report.json> ...]

CI runs scripts/interop.sh in shards (INTEROP_PAIRS: one CSMS and OCPP
version each) and joins their report documents (schemas/report.schema.json)
here: the results of every shard, in the order of the shards given, under one
run that started with the first shard and finished with the last. A test id
that two shards report for the same pairing is an error: the shards overlap.

A shard cannot tell by itself whether a known bug (tests/known-bugs.tsv) is
stale: it may reproduce in another shard. Each shard writes, next to its
report, known-bugs-seen.tsv: "reproduced" or "absent", a tab, and the bug's
summary, for every bug whose tests ran in a pairing it applies to. A bug that
applied in some shard and reproduced in none is stale, as in a run without
shards, and fails the merge.
"""

import json
import sys
from pathlib import Path


def main(argv):
    if len(argv) < 2:
        print(__doc__, file=sys.stderr)
        return 2
    out, inputs = Path(argv[0]), [Path(a) for a in argv[1:]]
    reports = [json.loads(p.read_text()) for p in inputs]
    results, seen = [], {}
    for path, report in zip(inputs, reports):
        for result in report["results"]:
            key = (result["version"], result["client"]["name"], result["server"]["name"],
                   result["initiator"], result["testId"])
            if key in seen:
                print(f"{path}: {key} is also in {seen[key]}: the shards overlap", file=sys.stderr)
                return 1
            seen[key] = path
            results.append(result)
    runs = [r["run"] for r in reports]
    merged = {
        "schemaVersion": reports[0]["schemaVersion"],
        "run": {
            "id": "+".join(r["id"] for r in runs),
            "started": min(r["started"] for r in runs),
            "finished": max(r["finished"] for r in runs),
            "producer": f"{runs[0]['producer']}, {len(runs)} shards merged by scripts/merge-reports.py",
        },
        "results": results,
    }
    out.write_text(json.dumps(merged) + "\n")
    print(f"{out}: {len(results)} results from {len(reports)} shards")

    seen = {}
    for path in inputs:
        bugs = path.with_name("known-bugs-seen.tsv")
        if not bugs.exists():
            continue
        for line in bugs.read_text().splitlines():
            state, _, text = line.partition("\t")
            seen[text] = seen.get(text, False) or state == "reproduced"
    stale = [text for text, reproduced in seen.items() if not reproduced]
    for text in stale:
        print(f"FAIL a known bug did not reproduce in any shard: {text}", file=sys.stderr)
        print("     remove it from tests/known-bugs.tsv; the pin can move to this version", file=sys.stderr)
    print(f"known bugs: {len(seen) - len(stale)} reproduced, {len(stale)} stale")
    return 1 if stale else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
