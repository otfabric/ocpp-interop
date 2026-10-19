#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""The job matrix of the self-tests, for the CI and Release workflows.

    scripts/ci-plan.py

Prints key=value lines for $GITHUB_OUTPUT:

    adapters=<JSON list>   the Makefile's ADAPTERS: one build and one smoke job each
    shards=<JSON list>     one interop job each: {"name", "pairs"}, where pairs
                           is the INTEROP_PAIRS of scripts/interop.sh

There is one shard per CSMS and OCPP version that versions.yaml says the
adapter serves (`roles` has server, `versions_verified` has the version), and
one more, `rest`, with every other CSMS and version: those pairings are one
SKIP_UNSUPPORTED each and take seconds, but keep the merged report complete.
The shards only split the work. What each pairing runs is still decided by
the adapters' capabilities, so a versions.yaml that is out of date makes a
shard slower, never a result wrong. CI time then grows with the slowest
shard, not with the number of adapters.
"""

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
VERSIONS = ["1.6", "2.0.1", "2.1"]
DIRS = {"1.6": "v16", "2.0.1": "v201", "2.1": "v21"}


def adapters():
    match = re.search(r"^ADAPTERS \?= (.+)$", (ROOT / "Makefile").read_text(), re.M)
    if not match:
        raise SystemExit("Makefile has no ADAPTERS ?= line")
    return match.group(1).split()


def served(adapter):
    """The OCPP versions versions.yaml says the adapter serves as a CSMS."""
    text = (ROOT / "versions.yaml").read_text()
    entry = re.search(rf"^  {adapter}:\n((?:    .*\n|\n)*)", text, re.M)
    if not entry:
        raise SystemExit(f"versions.yaml has no entry for {adapter}")
    roles = re.search(r"^    roles: \[([^\]]*)\]", entry.group(1), re.M)
    verified = re.search(r"^    versions_verified: \[([^\]]*)\]", entry.group(1), re.M)
    if not roles or "server" not in roles.group(1):
        return []
    return [v for v in VERSIONS if verified and f'"{v}"' in verified.group(1)]


def main():
    names = adapters()
    shards, rest = [], []
    for adapter in names:
        serves = served(adapter)
        for version in VERSIONS:
            if version in serves:
                shards.append({"name": f"{adapter}-{DIRS[version]}", "pairs": f"{adapter}@{version}"})
            else:
                rest.append(f"{adapter}@{version}")
    if rest:
        shards.append({"name": "rest", "pairs": " ".join(rest)})
    print(f"adapters={json.dumps(names)}")
    print(f"shards={json.dumps(shards)}")


if __name__ == "__main__":
    main()
