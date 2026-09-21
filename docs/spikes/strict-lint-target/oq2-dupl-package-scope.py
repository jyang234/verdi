#!/usr/bin/env python3
"""Fix-round-1 evidence for R-1: verifies, from this lane's own
oq1-raw/dupl.json (37 findings, full ./... run), that every dupl finding
pairs two files in the same directory (== the same Go package in this
tree; no directory here splits into multiple packages). Re-runnable:

    python3 docs/spikes/strict-lint-target/oq2-dupl-package-scope.py

Cited by README.md's oq-2 section (R-1 fix) instead of the withdrawn
"reach gap" framing.
"""
import json
import os
import re

with open("docs/spikes/strict-lint-target/oq1-raw/dupl.json") as f:
    data = json.load(f)

issues = data["Issues"]
same_dir = []
cross_dir = []
for iss in issues:
    fn = re.sub(r"^(\.\./)+", "", iss["Pos"]["Filename"])
    m = re.search(r"duplicate of `([^:]+):(\d+)-(\d+)`", iss["Text"])
    if not m:
        raise SystemExit(f"unparsed Text: {iss['Text']!r}")
    partner = m.group(1)
    dir_a, dir_b = os.path.dirname(fn), os.path.dirname(partner)
    (same_dir if dir_a == dir_b else cross_dir).append((fn, partner))

print(f"total dupl findings: {len(issues)}")
print(f"same-directory (same-package) pairs: {len(same_dir)}")
print(f"cross-directory (cross-package) pairs: {len(cross_dir)}")
for c in cross_dir:
    print("  CROSS:", c)
