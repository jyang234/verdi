---
id: obligation/instruction-conformance--ac-3--static
kind: obligation
title: "A closed handEditPhrasings-style phrase set, applied to the same AC-1-enumerated files, fires only on the conjunction of an undisclosed current-ritual phrasing and the absence of a retirement disclosure in that same file"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/instruction-conformance" }
frozen: { at: 2026-07-19, commit: b4bbc327614200071401da65706b23119fbabd2d }
---
# A closed handEditPhrasings-style phrase set, applied to the same AC-1-enumerated files, fires only on the conjunction of an undisclosed current-ritual phrasing and the absence of a retirement disclosure in that same file

The static evidence must show a source-level check, following
`internal/specalign/docsync_test.go`'s `handEditPhrasings` idiom exactly —
a closed, named, case-insensitive substring set — applied to the same set
of files AC-1's enumeration produces. Mirroring that file's own
`TestFindHandEditPhrasing` repair (ADJ-50), every phrasing in both the
current-ritual set and the retirement-disclosure set must itself be proven
to match a realistic sentence using it, never a dead pattern (containing,
say, an un-matchable placeholder) that implies protection it does not
actually have — the ADJ-47/ADJ-50 defect class this story must not
reintroduce. It must show the rule fires only on DC-3's conjunction: (a)
presence of a phrasing that instructs or describes `verdi board commit` /
a frozen `board.json` as the current step to finish a design-branch spec,
**and** (b) absence, anywhere in that same file, of a retirement-disclosure
phrasing (e.g. "retired," "grandfathered," "superseded") — never on
presence alone, since `verdi-surfaces/spec.md`'s own CLI table and
"Superseded" section, and this very story's Problem/Outcome prose,
legitimately repeat both strings while correctly explaining the
retirement, and a presence-only rule would make DC-4's rewrite-to-disclose
alternative structurally impossible to ever pass. It must carry the same
disclosed limit `docsync_test.go`'s own header states verbatim: a
lexical/substring tripwire, not a semantic guarantee against a future
paraphrase of either phrase set (ADJ-50's inherited residual, not
re-litigated here) — never silently overclaiming more than it checks.
