---
id: obligation/instruction-conformance--ac-5--static
kind: obligation
title: "The new AC-1/AC-2/AC-3 checks are unconditionally reachable by the existing bare go test ./internal/specalign/... target with no Makefile edit, and a source read of this repo's two real files, post build-phase disposition, shows no unresolved offense in either"
owners: [platform-team]
for_kind: static
links:
  - { type: verifies, ref: "spec/instruction-conformance" }
frozen: { at: 2026-07-19, commit: b4bbc327614200071401da65706b23119fbabd2d }
---
# The new AC-1/AC-2/AC-3 checks are unconditionally reachable by the existing bare go test ./internal/specalign/... target with no Makefile edit, and a source read of this repo's two real files, post build-phase disposition, shows no unresolved offense in either

The static evidence must show the new AC-1/AC-2/AC-3 checks are authored
as ordinary, unconditional Go test functions committed under
`internal/specalign` — no `-run` filter, no build tag, no `t.Skip`
guarding any of them — so `spec-align`'s existing bare `go test
./internal/specalign/...` Makefile target (left unedited) genuinely
reaches them, the structural claim the outcome text itself makes: "the new
test file(s) join `make verify` with no Makefile edit." It must also show
a source-level read of this repo's own two real enumerated files —
`.claude/skills/commit-to-design/SKILL.md` and the repo-root `CLAUDE.md` —
as they stand after the build phase applies a disposition to the former
(DC-4's retirement, the file no longer existing, or DC-3's
rewrite-to-disclose alternative, every current-ritual mention now paired
with a retirement disclosure in the same file), confirming by inspection
that no `verdi <verb>` reference in either file names a verb dispatch.go
does not recognize, and no retired-ritual phrase in either file lacks a
disclosure. This half of AC-5 is a content/wiring audit, provable by
reading the check's registration and the two files' final text — the
sibling `ac-5--behavioral` obligation covers the run-it-for-real claim
this AC's text also makes.
