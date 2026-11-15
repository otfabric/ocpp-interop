#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Tests of the digest step of .github/workflows/release.yml and of
scripts/release-manifest.py: a release gets one digest for every adapter of
ADAPTERS, or it fails."""

import hashlib
import json
import os
import re
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
STEP = "Digests of the multi-arch images"


def adapters():
    with open(os.path.join(ROOT, "Makefile"), encoding="utf-8") as f:
        for line in f:
            if line.startswith("ADAPTERS ?= "):
                return line.split("?=", 1)[1].split()
    raise AssertionError("no ADAPTERS in the Makefile")


def interop_version():
    with open(os.path.join(ROOT, "versions.yaml"), encoding="utf-8") as f:
        return re.search(r'^interop_version: "([^"]+)"', f.read(), re.M).group(1)


def line(adapter):
    return f"{adapter} sha256:{hashlib.sha256(adapter.encode()).hexdigest()}\n"


def digest_step():
    """The shell command and the run script of the step, as GitHub runs them."""
    with open(os.path.join(ROOT, ".github", "workflows", "release.yml"), encoding="utf-8") as f:
        lines = f.read().replace("\r\n", "\n").split("\n")
    start = next(i for i, text in enumerate(lines) if text.strip() == f"- name: {STEP}")
    indent = len(lines[start]) - len(lines[start].lstrip())
    step = []
    for text in lines[start + 1:]:
        if text.strip() and len(text) - len(text.lstrip()) <= indent:
            break
        step.append(text)
    shell = next((text.split(":", 1)[1].strip() for text in step if text.strip().startswith("shell:")), None)
    run_at = next(i for i, text in enumerate(step) if text.strip() == "run: |")
    body = step[run_at + 1:]
    width = min(len(text) - len(text.lstrip()) for text in body if text.strip())
    script = "\n".join(text[width:] for text in body) + "\n"
    # GitHub's commands: shell bash is `bash --noprofile --norc -eo pipefail {0}`;
    # without a shell, a Linux runner uses `bash -e {0}`.
    command = ["bash", "--noprofile", "--norc", "-eo", "pipefail"] if shell == "bash" else ["bash", "-e"]
    return command, script


class Work(unittest.TestCase):
    def setUp(self):
        self.work = tempfile.mkdtemp()
        self.addCleanup(shutil.rmtree, self.work)


class DigestStep(Work):
    def run_step(self, output, make_exit=0):
        """Runs the step with a make that prints output and exits with make_exit."""
        bin_dir = os.path.join(self.work, "bin")
        os.makedirs(bin_dir)
        make = os.path.join(bin_dir, "make")
        with open(make, "w", encoding="utf-8", newline="\n") as f:
            f.write("#!/usr/bin/env bash\ncat <<'EOF'\n" + output + "EOF\nexit " + str(make_exit) + "\n")
        os.chmod(make, os.stat(make).st_mode | stat.S_IXUSR)
        command, script = digest_step()
        with open(os.path.join(self.work, "step.sh"), "w", encoding="utf-8", newline="\n") as f:
            f.write(script)
        env = dict(os.environ, PATH=bin_dir + os.pathsep + os.environ["PATH"])
        return subprocess.run(command + ["step.sh"], cwd=self.work, env=env, capture_output=True, text=True)

    def test_every_adapter_with_a_digest_passes(self):
        res = self.run_step("".join(line(a) for a in adapters()))
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)

    def test_a_make_that_stops_partway_fails(self):
        res = self.run_step("".join(line(a) for a in adapters()[:3]), make_exit=1)
        self.assertNotEqual(res.returncode, 0, res.stdout)


class Manifest(Work):
    def run_manifest(self, digests):
        path = os.path.join(self.work, "digests.txt")
        with open(path, "w", encoding="utf-8") as f:
            f.write(digests)
        self.manifest = os.path.join(self.work, "manifest.json")
        return subprocess.run([sys.executable, os.path.join(ROOT, "scripts", "release-manifest.py"),
                               "v" + interop_version(), "0" * 40, path, self.manifest,
                               os.path.join(self.work, "images.md")], capture_output=True, text=True)

    def test_every_adapter_gives_the_manifest(self):
        res = self.run_manifest("".join(line(a) for a in adapters()))
        self.assertEqual(res.returncode, 0, res.stderr)
        with open(self.manifest, encoding="utf-8") as f:
            images = json.load(f)["images"]
        self.assertEqual(sorted(images), sorted(adapters()))

    def test_a_partial_list_fails(self):
        res = self.run_manifest("".join(line(a) for a in adapters()[:3]))
        self.assertNotEqual(res.returncode, 0, res.stdout)
        self.assertIn(adapters()[-1], res.stderr)
        self.assertFalse(os.path.exists(self.manifest))

    def test_an_empty_list_fails(self):
        res = self.run_manifest("")
        self.assertNotEqual(res.returncode, 0, res.stdout)
        self.assertFalse(os.path.exists(self.manifest))

    def test_a_repeated_or_unknown_adapter_fails(self):
        complete = "".join(line(a) for a in adapters())
        for extra in (line(adapters()[0]), line("nonesuch")):
            res = self.run_manifest(complete + extra)
            self.assertNotEqual(res.returncode, 0, extra)
            self.assertFalse(os.path.exists(self.manifest), extra)


if __name__ == "__main__":
    unittest.main()
