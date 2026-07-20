---
id: obligation/instruction-conformance--ac-4--behavioral
kind: obligation
title: "A dirty fixture carrying both an unrecognized verb and an undisclosed retired-ritual phrase fails naming the exact file and offense; a clean fixture with real verbs and a disclosed mention passes with zero findings"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/instruction-conformance" }
frozen: { at: 2026-07-19, commit: b4bbc327614200071401da65706b23119fbabd2d }
---
# A dirty fixture carrying both an unrecognized verb and an undisclosed retired-ritual phrase fails naming the exact file and offense; a clean fixture with real verbs and a disclosed mention passes with zero findings

The behavioral evidence must show two committed fixture instruction files
driven through the real AC-1/AC-2/AC-3 checks end to end — never a
synthetic assertion standing in for actually running them. The first,
dirty fixture carries both a `verdi <verb>` reference naming a verb
dispatch.go does not recognize and a retired-ritual phrase from AC-3's
tripwire set with no disclosure anywhere in the file; driving the checks
against it must fail, with the failure output naming the exact fixture
file path and the exact offending verb or phrase text — never a bare
boolean. The test proving this must itself never be structured as a `go
test -run` invocation that would exit 0 by matching nothing if the
underlying test function were ever renamed or deleted — the vacuous-pass
class this package's own ADJ-47/ADJ-50 history already found and fixed
once for `docsync_test.go`, and the exact standard this story's own
outcome text sets ("this gate cannot silently vanish the way this
package's own ADJ-47/ADJ-50 history already found and fixed once"). The
second, clean fixture carries only real, recognized verbs and a `board
commit` mention paired with a retirement disclosure in the same file;
driving the checks against it must pass with zero findings, proving the
checks do not also false-positive on legitimate content that discusses
the retired ritual honestly. Green in CI's test step.
