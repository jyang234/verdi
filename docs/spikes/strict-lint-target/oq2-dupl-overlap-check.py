#!/usr/bin/env python3
"""Fix-round-1 evidence for R-4: a line-range-overlap check that actually
discriminates whether golangci-lint's dupl run found the hasDotDotElement
pair, replacing the broken `grep hasDotDotElement` witness (dupl's own
message text never carries a function name — it prints
"N-M lines are duplicate of `file:line-line`" — so that grep could not
have matched either way, correct or wrong).

Target ranges (the full function body, declaration through closing brace,
verified by `awk 'NR==485,NR==495{print NR": "$0}'` at commit 1be75d01 in
verdi-wt/scratch-lint-w1 — see oq2-retro-witness.txt):
  cmd/verdi/context.go:485-494
  internal/readinessload/conflict.go:232-241

Usage:
    python3 docs/spikes/strict-lint-target/oq2-dupl-overlap-check.py <dupl.json>

Exit 0 and prints "OVERLAP: <n> finding(s)" if any dupl finding's own range
or its cited partner's range overlaps either target range in either file;
exit 0 and prints "NO OVERLAP" otherwise (this script's own exit code is
always 0 — the overlap/no-overlap result is the first line of stdout, by
design, so it can be captured either way without relying on a second tool's
exit code as the signal, the exact class of mistake R-4 is about).
"""
import json
import re
import sys

TARGETS = {
    "cmd/verdi/context.go": (485, 494),
    "internal/readinessload/conflict.go": (232, 241),
}


def overlaps(a_lo, a_hi, b_lo, b_hi):
    return a_lo <= b_hi and b_lo <= a_hi


def strip_prefix(filename):
    return re.sub(r"^(\.\./)+", "", filename)


def main():
    path = sys.argv[1]
    with open(path) as f:
        data = json.load(f)

    hits = []
    for iss in data.get("Issues", []):
        primary_file = strip_prefix(iss["Pos"]["Filename"])
        m = re.match(r"(\d+)-(\d+) lines are duplicate of `([^:]+):(\d+)-(\d+)`", iss["Text"])
        if not m:
            continue
        p_lo, p_hi = int(m.group(1)), int(m.group(2))
        partner_file = m.group(3)
        q_lo, q_hi = int(m.group(4)), int(m.group(5))

        for fname, (t_lo, t_hi) in TARGETS.items():
            if primary_file == fname and overlaps(p_lo, p_hi, t_lo, t_hi):
                hits.append((primary_file, p_lo, p_hi, "primary"))
            if partner_file == fname and overlaps(q_lo, q_hi, t_lo, t_hi):
                hits.append((partner_file, q_lo, q_hi, "partner"))

    if hits:
        print(f"OVERLAP: {len(hits)} finding(s)")
        for h in hits:
            print("  ", h)
    else:
        print("NO OVERLAP")
    print(f"(checked {len(data.get('Issues', []))} total findings in {path})")


if __name__ == "__main__":
    main()
