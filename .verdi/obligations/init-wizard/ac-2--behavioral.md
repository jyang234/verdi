---
id: obligation/init-wizard--ac-2--behavioral
kind: obligation
title: "--wizard refuses without a TTY, and on one runs the vocabulary/template-set interview with live validation and an in-interview frontier refusal that never aborts"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/init-wizard" }
frozen: { at: 2026-07-21, commit: 2f5b1cae64f5345c42085e2794c2ad3d8f830976 }
---
# --wizard refuses without a TTY, and on one runs the vocabulary/template-set interview with live validation and an in-interview frontier refusal that never aborts

The behavioral evidence must show built-binary tests (`cmd/verdi/init_test.go`)
proving: `verdi init --wizard` with stdin wired to a plain pipe (never a real
terminal) and no test-only TTY override set exits 2 naming the missing TTY,
writing nothing to the target directory at all.

A second case sets the disclosed, test-only `VERDI_INIT_ASSUME_TTY=1`
override in the child process's environment and feeds a scripted answer
sequence over `cmd.Stdin` (a `strings.Reader`, the stdin-script harness this
story's spec discloses) that accepts every vocabulary-rename prompt's
default, answers `y` when the interview's one structural-request probe is
reached, and confirms the write at the end — the run must exit 0, its
combined output must contain a frontier-explanation line (naming that
structural configuration is locked to the canonical model and "unlocks
per-verb later") at the structural-request point, AND the interview must be
shown continuing past it to completion (later prompts still appear, the
final summary and write still happen) rather than the process exiting
early.

A third scripted run must supply real, non-default renames for at least one
class, one state, and one verb id and a "yes" to the template-set copy
question, and assert the promoted store's `.verdi/model.yaml` contains a
`vocabulary:` block with exactly those renames and `.verdi/templates/
feature.md` / `.verdi/templates/story.md` now exist as local override
copies.

Green in CI's test step, as part of `make verify`.
