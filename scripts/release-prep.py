#!/usr/bin/env python3
"""Prepares a release: turns the "## Unreleased" section of RELEASE.md into
"## <version>" with today's date, and sets interop_version in versions.yaml.

Usage: release-prep.py vX.Y.Z   (or make release-prep VERSION=vX.Y.Z)

Review the diff, commit it, merge it, then tag the merged commit.
"""

import datetime
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def main(argv):
    if len(argv) != 1 or not re.fullmatch(r"v\d+\.\d+\.\d+", argv[0]):
        print(__doc__, file=sys.stderr)
        return 2
    version = argv[0]
    notes = ROOT / "RELEASE.md"
    text = notes.read_text()
    if re.search(rf"^## {re.escape(version)}$", text, re.M):
        print(f"RELEASE.md already has a section {version}")
    else:
        m = re.search(r"^## Unreleased\n", text, re.M)
        if not m:
            print("RELEASE.md has no \"## Unreleased\" section to release", file=sys.stderr)
            return 1
        rest = text[m.end():]
        # Drop the paragraph that announces the release as planned, if any.
        rest = re.sub(r"\A\nPlanned as [^\n]*(?:\n[^\n#][^\n]*)*\n", "\n", rest)
        rest = "\n" + rest.lstrip("\n")
        today = datetime.date.today().isoformat()
        text = text[: m.start()] + f"## {version}\n\n**Date:** {today}\n" + rest
        notes.write_text(text)
        print(f"RELEASE.md: \"## Unreleased\" is now \"## {version}\" ({today})")
    versions = ROOT / "versions.yaml"
    v = versions.read_text()
    new, n = re.subn(r'^interop_version: "[^"]*"[^\n]*$',
                     f'interop_version: "{version[1:]}"', v, count=1, flags=re.M)
    if n != 1:
        print("versions.yaml has no interop_version line", file=sys.stderr)
        return 1
    versions.write_text(new)
    print(f"versions.yaml: interop_version {version[1:]}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
