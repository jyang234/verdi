---
id: obligation/close-preflight--ac-1--behavioral
kind: obligation
title: "A Go test proves --preflight names the exact condition, evidence kind, and path close would refuse on, for both story and feature scope"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/close-preflight" }
frozen: { at: 2026-07-16, commit: ba9347d6ff69b3a9654008e66e389dfa515c7ffc }
---
# A Go test proves --preflight names the exact condition, evidence kind, and path close would refuse on, for both story and feature scope

The behavioral evidence must show two Go test files driving the CLI's own
entry-point functions in-process — mirroring `cmd/verdi/close_test.go`'s and
`cmd/verdi/closuregate_test.go`'s existing convention (call the `runXxx`
function directly, over a fixturegit store, never a subprocess exec; this
is what "a Go end-to-end test driving the built binary" means at the
cmd/verdi package level — the identical code the binary ships, exercised
through its real signature) — never Playwright, and with no network in any
case (co-1).

`cmd/verdi/closepreflight_test.go` covers the story scope: one fixture, and
one subtest, per defect class named in ac-1 — an AC with no evidence at
all (no-signal), an AC with some-but-not-all declared kinds satisfied
(pending), an AC with a failing current record (violated), a flagged
spec-stale finding, and an open pending-supersession MR touching the
story's implemented objects — each asserting the preflight's stdout names
the exact unmet condition, the exact missing evidence kind, and the exact
on-disk path the fold reads (both the attestation and the derived-tree
cases), or, for spec-stale/pending-supersession, the exact finding id / MR
id / object id the real gate would print. A further subtest proves the
disclosed-vs-operational forge split (dc-5): a nil forge produces a
disclosed line, never exit 2; a forge double that returns a transport
error on `ListOpenMRs` produces exit 2.

`cmd/verdi/closepreflightfeature_test.go` covers the feature scope: one
fixture and one subtest per feature-specific defect class — a feature AC
not evidenced (including an unmet outcome-floor attestation at the
FeatureSlug path, dc-6, not the story-slug path), an unreconciled stub, and
an implementing story still open — each asserting the same exact-path/
exact-condition disclosure discipline as the story-scope file.

`cmd/verdi/closepreflight_test.go` must also cover ac-1's added
CI-guard-disclosure clause (dc-1, closing this story's own judged-dcj-2):
a ready fixture run with no CI environment variables set and no
`--force-local` prints the CI-only/`--force-local` guard's own condition
text alongside its ready verdict; the same ready fixture run with a CI
environment simulated (or with `--force-local`) does NOT print that line —
proving the disclosure is conditional on the real guard's own inputs, not
unconditionally appended. Per dc-1's follow-on fix (closing this story's
own second, re-swept judge finding), the test must also prove the
disclosure's condition is read from the same `lint.ReadCIEnv().InCI`/
`--force-local` inputs the guard itself reads — e.g. by driving both the
guard's own refusal (a real, non-`--preflight` close outside CI without
`--force-local`) and the preflight's disclosure from the identical
environment/flag setup in one test and asserting they agree — not by two
independently-hand-asserted expectations that could drift apart.
