---
id: obligation/verb-surfaces--ac-4--behavioral
kind: obligation
title: "scaffolded obligation: ac-4 behavioral evidence"
owners: ["johnyang"]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/verb-surfaces" }
frozen: { at: 2026-07-22, commit: fc77362bd91d5e77b199e22ddd22bd272132e4f7 }
---
# scaffolded obligation: ac-4 behavioral evidence

This obligation was scaffolded at accept; not elaborated. It is a placeholder for ac-4's behavioral evidence, written by accept's
freeze-moment backstop because no obligation existed for this pair
when spec/verb-surfaces was accepted (spec/creation-surfaces#ac-4). Replace this body
with a first-person statement of what that evidence must specifically
show before relying on it — by hand, or via `verdi obligation author
spec/verb-surfaces ac-4 behavioral` on a design branch before the replacement itself freezes.
The acceptance criterion's own declared text, for reference:

internal/specalign/vocabprose_test.go's TestVocabProseWitness word list extends to every canonical model verb id (mdl.Lifecycle[*].Transitions[*].Verb — today accept and close, derived from model.Canonical() exactly as the existing class/state derivation already is, never hand-maintained) alongside its existing class and state words; run at head after seeding, the witness is green — every unrouted, unmarked bare hit of a verb word the extended list newly catches across the whole cmd/ and internal/ production tree, including this story's own waive.go and every verb-speaking surface the merged sibling stories already shipped, is either routed through model.DisplayVerb (or an equivalent already-routed local per the witness's own ROUTED heuristic) or marked // vocab:identity at its producing site with a stated reason; a dedicated mutation-witness test — mirroring the existing class/state witness's own convention — proves the extended scanner is RED against a synthetic, deliberately-bare, unrouted verb word and GREEN once seeded, so the new category is proven to bite rather than merely present

