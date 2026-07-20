---
id: obligation/instruction-conformance--ac-2--behavioral
kind: obligation
title: "Tests driving the real built verdi binary from an empty, rootless temp directory prove every extracted verdi <verb> reference is validated against dispatch.go's own recognized-verb set, in both directions"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/instruction-conformance" }
frozen: { at: 2026-07-19, commit: b4bbc327614200071401da65706b23119fbabd2d }
---
# Tests driving the real built verdi binary from an empty, rootless temp directory prove every extracted verdi <verb> reference is validated against dispatch.go's own recognized-verb set, in both directions

The behavioral evidence must show Go tests mirroring
`internal/specalign/helpers_test.go`'s `runBinary` + package `TestMain`
build-once precedent exactly (never `go run`, never importing `cmd/verdi`
as a package): for every `verdi <verb>` reference extracted from an
AC-1-enumerated fixture instruction file's backtick-delimited spans
(inline code and fenced code blocks alike), the once-built `verdi` binary
is exec'd with that word as its sole argument from a fresh, rootless temp
directory, and stderr is compared against dispatch.go's own top-level
unknown-verb usage banner (DC-2's exact-match classification rule — not
`verbs_test.go`/`helpers_test.go`'s existing `assertNotOutOfV0`, which asks
the different "known and implemented" question). It must show both
directions: a verb dispatch.go does not recognize at all fails, naming the
instruction file and the unrecognized verb text; and a verb dispatch.go
does recognize — including the two verbs explicitly out of v0 scope,
`waivers` and `verify-artifact`, which print dispatch.go's own distinct
"not implemented (out of v0 scope)" message rather than the unknown-verb
banner — passes, since prose accurately describing a real-but-unimplemented
verb is not stale. It must also show this check alone does not, and is not
expected to, catch the motivating `SKILL.md` defect (`board` is still
recognized) — that is AC-3's obligation. Green in CI's test step.
