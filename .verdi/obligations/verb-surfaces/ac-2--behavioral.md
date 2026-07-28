---
id: obligation/verb-surfaces--ac-2--behavioral
kind: obligation
title: "scaffolded obligation: ac-2 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/verb-surfaces" }
frozen: { at: 2026-07-22, commit: fc77362bd91d5e77b199e22ddd22bd272132e4f7 }
---
# scaffolded obligation: ac-2 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-2's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/verb-surfaces was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/verb-surfaces ac-2 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

verdi waive <story-ref> <ac-id> --reaffirm --rationale <text> [--expires YYYY-MM-DD] refuses (exit 1, naming the plain create form) when no waiver yet exists at the convention path, and otherwise rewrites that SAME file in place: frontmatter reason/expiry/status(reset to active)/frozen are all refreshed to the fresh invocation, and the body's mechanically-owned reaffirmation log — delimited by a fixed marker so it is appended-to, never reparsed as prose — gains one new dated entry naming the fresh rationale and expiry, so a waiver reaffirmed more than once carries its full history legibly in one committed file; a lifecycle test proves a reaffirm round-trips (the file's frozen stamp and reason change, the prior log entry survives verbatim, a new one is appended) and that both the verb's own output and a lapsed prior expiry are disclosed when reaffirming after the recorded expiry has passed

