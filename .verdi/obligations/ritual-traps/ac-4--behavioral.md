---
id: obligation/ritual-traps--ac-4--behavioral
kind: obligation
title: "VL-003 cross-checks a fragment-qualified bindings entry against the NAMED spec's own ACs; a typo'd #ac-9 reds by name, proven on the real root file"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/ritual-traps" }
frozen: { at: 2026-07-20, commit: 853f1c91ad9493e808eca1422d7991fa7d86692e }
---
# VL-003 cross-checks a fragment-qualified bindings entry against the NAMED spec's own ACs; a typo'd #ac-9 reds by name, proven on the real root file

The behavioral evidence must show `internal/lint/vl003_test.go` gaining a
case that introduces a fragment-qualified entry naming an AC the target
spec does not declare (e.g. `"spec/some-story#ac-9"` where that story's
own `acceptance_criteria:` list has no `ac-9`) and asserts `VL-003` reds,
naming both the offending entry and the target spec — never a silent
pass. A companion case with a CORRECT fragment-qualified entry — the
exact shape this design series' own `verdi.bindings.yaml` additions
already are (e.g. `"spec/judge-ergonomics#ac-1"`) — must continue to
pass, proving the check does not regress real, already-landed entries.
The obligation's evidence should include running `verdi lint` against
this repository's own real root `verdi.bindings.yaml` post-fix (once
ac-3's discovery path is in place) and confirming every fragment-
qualified entry this design series added resolves cleanly against its
named story's real acceptance criteria — the proof-on-the-real-file this
story's outcome promises, not only a synthetic fixture. Green in CI's
test step, as part of `make verify`.
