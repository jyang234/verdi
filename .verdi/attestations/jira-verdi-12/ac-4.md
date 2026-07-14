---
id: attestation/jira-verdi-12--ac-4
kind: attestation
title: "AC-4 attested: the diagram-divergence review loop closes end to end through the existing disposition mechanism"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/alignment-section }
frozen: { at: 2026-07-14, commit: ac9bb0beb779a3ce04b48aeb6e1c42d333d808a5 }
---
# AC-4 attestation

Reviewer attests (round 6, 2026-07-14): the disposition loop for diagram
divergences closes end to end through the existing mechanism, exercised on
a genuinely accepted, genuinely diverged fixture proposal during the
alignment-section build (TestGenerate_DiagramFindingDispositionSurvives-
Regeneration: an accepted-deviation disposition with its note survives
report regeneration via the unchanged PreserveDispositions path), and the
same reviewer used that mechanism in anger throughout this round — every
build branch's deviation report was dispositioned by hand and each
disposition survived subsequent align refreshes. Diagram findings ride the
one findings list ComputeDigest already covers; no second digest field
exists (verified at review of PR #73, incl. the ADJ-10 remediation that
made 'realized' honest). The reviewer's judgment, not a computed record,
is the substance of this leg — which is exactly why this AC declares an
attestation floor.
