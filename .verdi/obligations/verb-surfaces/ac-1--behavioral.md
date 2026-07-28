---
id: obligation/verb-surfaces--ac-1--behavioral
kind: obligation
title: "scaffolded obligation: ac-1 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/verb-surfaces" }
frozen: { at: 2026-07-22, commit: fc77362bd91d5e77b199e22ddd22bd272132e4f7 }
---
# scaffolded obligation: ac-1 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-1's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/verb-surfaces was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/verb-surfaces ac-1 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

verdi waive <story-ref> <ac-id> --rationale <text> [--expires YYYY-MM-DD] resolves the (story, AC) pair through the same classifyPair seam verdi attest and verdi obligation author already share, refuses (exit 2) on a missing --rationale or a malformed --expires value, and otherwise writes a create-only WaiverFrontmatter record (status: active, reason, expiry when given, owners copied verbatim from the resolved story spec, a frozen stamp) at waivers/<story-slug>/<ac-id>.md, self-validated by decoding the exact rendered bytes before the first write — never overwriting a waiver already present at that path (that refusal names --reaffirm as the extension path); a lifecycle test proves the AC folds waived immediately after (verdi matrix's own STATUS column, evidence.Fold unmodified) and that the verb's own stdout surfaces the configured expiry (or discloses none was given)

