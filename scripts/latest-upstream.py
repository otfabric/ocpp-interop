#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Compares the upstream pin of one adapter with the latest upstream release.

    scripts/latest-upstream.py <adapter>

Prints key=value lines (the format of $GITHUB_OUTPUT):

    pin=<version in the adapter Dockerfile, or the release recorded in versions.yaml>
    pin_source=dockerfile|versions.yaml
    built=yes|no          whether adapters/<adapter>/Dockerfile exists
    latest=<latest stable upstream release>
    latest_sha=<its commit, where it is looked up in git>
    update=yes|no
    source=<where the latest release was looked up>

While an adapter is planned and has no Dockerfile, its pin is the release
recorded as `latest:` in versions.yaml: "update=yes" then means that record is
stale. Nothing is modified. Pre-releases (rc, beta, ...) are never "latest".
"""

import json
import re
import subprocess
import sys
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
AGENT = "ocpp-interop latest-upstream (https://github.com/otfabric/ocpp-interop)"


def fetch(url):
    request = urllib.request.Request(url, headers={"User-Agent": AGENT, "Accept": "application/json, */*"})
    with urllib.request.urlopen(request, timeout=30) as response:
        return response.read().decode()


def key(version):
    """Sort key of a stable version; None for anything else (pre-releases)."""
    match = re.fullmatch(r"v?(\d+(?:\.\d+)*)", version)
    return tuple(int(part) for part in match.group(1).split(".")) if match else None


def newest(versions):
    stable = [v for v in versions if key(v) is not None]
    if not stable:
        raise SystemExit("no stable release found upstream")
    return max(stable, key=key)


def git_tags(repository):
    """Tags of repository as {tag: commit}; an annotated tag gives its commit."""
    out = subprocess.check_output(["git", "ls-remote", "--tags", repository], text=True)
    commits = {}
    for line in out.splitlines():
        sha, ref = line.split()
        tag = ref.removeprefix("refs/tags/")
        if tag.endswith("^{}"):
            commits[tag[:-3]] = sha
        else:
            commits.setdefault(tag, sha)
    return commits


def mobilityhouse():
    releases = json.loads(fetch("https://pypi.org/pypi/ocpp/json"))["releases"]
    live = [v for v, files in releases.items() if files and not all(f.get("yanked") for f in files)]
    return newest(live), "", "PyPI"


def chargetime():
    tags = git_tags("https://github.com/ChargeTimeEU/Java-OCA-OCPP.git")
    latest = newest(tags)
    return latest, tags[latest], "git tags of ChargeTimeEU/Java-OCA-OCPP"


def lorenzodonini():
    tags = git_tags("https://github.com/lorenzodonini/ocpp-go.git")
    latest = newest(tags)
    return latest, tags[latest], "git tags of lorenzodonini/ocpp-go"


RESOLVERS = {
    "mobilityhouse": mobilityhouse,
    "chargetime": chargetime,
    "lorenzodonini": lorenzodonini,
}


def recorded(adapter):
    """The `latest:` value of the adapter's entry in versions.yaml."""
    entry = re.search(rf"^  {adapter}:\n((?:    .*\n|\n)*)", (ROOT / "versions.yaml").read_text(), re.M)
    match = entry and re.search(r'^    latest: "([^"]+)"', entry.group(1), re.M)
    if not match:
        raise SystemExit(f"versions.yaml has no latest: for {adapter}")
    return match.group(1)


def main():
    if len(sys.argv) != 2 or sys.argv[1] not in RESOLVERS:
        raise SystemExit(f"usage: latest-upstream.py {'|'.join(RESOLVERS)}")
    adapter = sys.argv[1]
    dockerfile = ROOT / "adapters" / adapter / "Dockerfile"
    if dockerfile.exists():
        match = re.search(r"^ARG UPSTREAM_VERSION=(.+)$", dockerfile.read_text(), re.M)
        if not match:
            raise SystemExit(f"adapters/{adapter}/Dockerfile has no ARG UPSTREAM_VERSION=<pin>")
        pin, pin_source, built = match.group(1).strip(), "dockerfile", "yes"
    else:
        pin, pin_source, built = recorded(adapter), "versions.yaml", "no"
    latest, latest_sha, source = RESOLVERS[adapter]()
    print(f"pin={pin}")
    print(f"pin_source={pin_source}")
    print(f"built={built}")
    print(f"latest={latest}")
    print(f"latest_sha={latest_sha}")
    print(f"update={'yes' if key(latest) != key(pin) else 'no'}")
    print(f"source={source}")


if __name__ == "__main__":
    main()
