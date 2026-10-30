#!/usr/bin/env python3
# SPDX-License-Identifier: MIT
"""Tests of scripts/report-to-results.py that need no Docker."""

import importlib.util
import os
import unittest
from unittest import mock

import interop_result

HERE = os.path.dirname(os.path.abspath(__file__))
spec = importlib.util.spec_from_file_location("report_to_results", os.path.join(HERE, "report-to-results.py"))
rr = importlib.util.module_from_spec(spec)
spec.loader.exec_module(rr)

RUN = {"id": "r", "started": "2026-10-30T00:00:00Z", "finished": "2026-10-30T00:00:01Z", "producer": "test"}
URL = "https://example.com/bug"


def result(status, **kw):
    r = {"testId": "v201.from-csms.reset", "version": "2.0.1", "client": {"name": "a", "version": "1", "image": "i"},
         "server": {"name": "b", "version": "2", "image": "j"}, "initiator": "server", "feature": "provisioning",
         "status": status, "durationMs": 3}
    r.update(kw)
    return r


WAIVERS = [{"id": "w", "spec": "a:client", "adapter": "a", "role": "client", "peer": "", "version": "1",
            "regex": r"^v201\.from-csms\.reset$", "evidence": URL, "summary": "the bug"}]


def convert(results, seen=None):
    with mock.patch.object(interop_result, "repo_digest", return_value=None):
        doc = rr.convert({"run": RUN, "results": results}, WAIVERS, seen or {})
    doc["summary"] = interop_result.summarize(doc["results"])
    return doc


class Convert(unittest.TestCase):
    def test_outcomes(self):
        doc = convert([result("PASS"), result("FAIL", error="e"), result("SKIP_UNSUPPORTED", evidence="no capability"),
                       result("KNOWN_ISSUE", evidence=URL, error="e"), result("INFRA_ERROR", error="e")], {"the bug": True})
        self.assertEqual([r["outcome"] for r in doc["results"]],
                         ["pass", "fail", "unsupported", "known-difference", "infra-error"])
        self.assertEqual(doc["results"][3]["waiver"], "w")
        self.assertEqual(interop_result.schema_errors(doc), [])

    def test_stale_known_bug_fails(self):
        doc = convert([result("PASS")], {"the bug": False})
        self.assertFalse(doc["waivers"][0]["reproduced"])
        self.assertTrue(interop_result.problems(doc))

    def test_all_skipped_fails(self):
        self.assertTrue(interop_result.problems(convert([result("SKIP_UNSUPPORTED", evidence="no capability")])))

    def test_every_known_bug_line_parses(self):
        self.assertTrue(rr.load_waivers())
        ids = [w["id"] for w in rr.load_waivers()]
        self.assertEqual(len(ids), len(set(ids)))


if __name__ == "__main__":
    unittest.main()
