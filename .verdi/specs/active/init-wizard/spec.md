---
id: spec/init-wizard
kind: spec
title: "Init Wizard"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-P2-6
problem: { text: "There is no verdi init at all: R4-I-56 already named the narrow, non-interactive scaffold-wrapper baseline as owed, and the owner-accepted guide's Part II already describes a full configuring wizard in detail — vocabulary renames, template-set selection, live validation previews — but neither exists, so a team starting fresh must hand-assemble a store by hand, exactly as the README's own manual bootstrap section still has to walk through today. The design wave that pressure-tested this story's own mechanism (Task 8, design doc §12) caught the plan's own draft under-specifying the one property this whole feature actually needs: 'pass verdi model check before any write' cannot be built literally, because the check path is disk-bound end to end, and a decode-only stand-in would leave a wizard-written template override unvalidated (W-1); and the plan's first-draft refusal predicate — an existing manifest — would let a stray, non-empty .verdi/ directory with no manifest inside it pass straight through into the single-rename promotion the wizard's own atomicity depends on, where it then fails outright with ENOTEMPTY (W-3b). Both gaps needed closing before either path could be trusted to leave nothing behind on refusal, abort, or a crash mid-write.", anchor: problem }
outcome: { text: "verdi init offers both paths the guide promises, at once. The bare form is non-interactive, R4-I-56's baseline: exactly the .verdi/ skeleton the README's manual steps already describe, and nothing more. --wizard opts explicitly into a guided interview, requiring a TTY and refusing without one; it walks every renameable vocabulary id and the template-set choice, each with a live validation preview, and explains the v1 frontier honestly rather than pretending to honor a structural request outside it. Neither path ever writes to the real store while working: both assemble the complete candidate store in a same-filesystem sibling temporary directory and gate promotion on the full runModelCheck core run over that staged root, promoting by exactly one os.Rename only once the staged store passes that check — and, for the wizard, once its model.yaml decode-compares equal to the interview's own in-memory intent. Both paths refuse on any existing .verdi/ directory at all, exit 2, naming what exists. A mid-interview abort, or a simulated crash mid-write, leaves nothing whatsoever at the real root.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Given an empty target directory, bare verdi init (no --wizard) writes exactly the .verdi/ skeleton the README's own manual bootstrap steps describe — a .verdi/verdi.yaml manifest carrying only schema: verdi.layout/v1 (R4-I-56's conservative scope: no invented forge or tracker defaults) — and nothing else: no model.yaml at all, since the canonical operating model governs by its absence exactly as an already-adopted store's does. Both the bare and the --wizard path refuse identically on ANY existing .verdi/ directory at all, not merely an existing verdi.yaml manifest inside it (W-3/W-3b — the single-rename promotion both paths share requires .verdi itself to be absent, or the rename fails outright with ENOTEMPTY): the refusal exits 2, naming exactly what already exists at that path, and points at hand-editing .verdi/model.yaml (validated by verdi model check) as the reconfigure-an-existing-store path, since that is explicitly out of v1 scope.", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "--wizard requires an attached terminal and refuses, exit 2, without one — never silently degrading to the bare path's defaults (tested via a stdin-script harness driving the built binary: a scripted sequence of answers fed over a real OS pipe, with a disclosed, test-only environment override standing in for the TTY predicate alone, chosen over a pty harness for hermetic, deterministic, dependency-free CI portability). On a real TTY it runs a guided interview over exactly the v1 frontier's two configurable axes (internal/model's checkFrontier: vocabulary display renames and a class's template-file choice, nothing else): each of the model's renameable class, state, and verb ids is offered a display rename, and copying the canonical templates into .verdi/templates/ for local customization is offered as a yes/no choice; every answer is previewed live by validating the in-progress candidate model in memory against the same kernel rules and frontier check verdi model check itself runs, before the interview moves on. A request for capability outside that frontier — restructuring the class hierarchy, lifecycle states, or per-transition obligations — is refused with an explanation naming the frontier (structural configuration 'unlocks per-verb later'; only vocabulary and template-file choices are configurable in v1), and the interview continues rather than aborting. The wizard writes nothing to the real store while interviewing: every write happens inside a same-filesystem sibling temporary directory created beside where .verdi will land — co-located with its eventual parent, never os.TempDir, so promotion can never cross a filesystem boundary.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "Promotion out of the staged temporary directory is gated on running the complete runModelCheck core (cmd/verdi/model.go) over the staged root exactly as verdi model check itself would — never a decode-only check that would leave a wizard-written template override unvalidated — and, when the interview diverged from canonical, on the staged model.yaml decoding back to a model that is identical to the interview's own in-memory candidate. Promotion itself is exactly one os.Rename of the staged store onto the real .verdi path; no other write ever touches the real root. A mid-interview abort (stdin ending before every prompt is answered) and a simulated crash injected partway through staging both leave nothing whatsoever at the real root — no .verdi/ directory at all — because the staged temporary directory is discarded on any pre-rename error and no real-store write ever precedes that single rename.", evidence: [behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/creation-surfaces#ac-1" }
frozen: { at: 2026-07-21, commit: a28bc2178eb7f481bc30b2b234a3d5e944b9591a, stub_matched: true }
---
# Init Wizard

## Problem

There is no `verdi init` at all: R4-I-56 already named the narrow,
non-interactive scaffold-wrapper baseline as owed, and the owner-accepted
guide's Part II already describes a full configuring wizard in detail —
vocabulary renames, template-set selection, live validation previews —
but neither exists, so a team starting fresh must hand-assemble a store
by hand, exactly as the README's own manual bootstrap section still has
to walk through today.

The design wave that pressure-tested this story's own mechanism (Task 8,
design doc §12) caught the plan's own draft under-specifying the one
property this whole feature actually needs: "pass `verdi model check`
before any write" cannot be built literally, because the check path is
disk-bound end to end, and a decode-only stand-in would leave a
wizard-written template override unvalidated (W-1); and the plan's
first-draft refusal predicate — an existing manifest — would let a
stray, non-empty `.verdi/` directory with no manifest inside it pass
straight through into the single-rename promotion the wizard's own
atomicity depends on, where it then fails outright with `ENOTEMPTY`
(W-3b). Both gaps needed closing before either path could be trusted to
leave nothing behind on refusal, abort, or a crash mid-write.

## Outcome

`verdi init` offers both paths the guide promises, at once. The bare
form is non-interactive, R4-I-56's baseline: exactly the `.verdi/`
skeleton the README's manual steps already describe, and nothing more.
`--wizard` opts explicitly into a guided interview, requiring a TTY and
refusing without one; it walks every renameable vocabulary id and the
template-set choice, each with a live validation preview, and explains
the v1 frontier honestly rather than pretending to honor a structural
request outside it.

Neither path ever writes to the real store while working: both assemble
the complete candidate store in a same-filesystem sibling temporary
directory and gate promotion on the full `runModelCheck` core run over
that staged root, promoting by exactly one `os.Rename` only once the
staged store passes that check — and, for the wizard, once its
`model.yaml` decode-compares equal to the interview's own in-memory
intent. Both paths refuse on any existing `.verdi/` directory at all,
exit 2, naming what exists. A mid-interview abort, or a simulated crash
mid-write, leaves nothing whatsoever at the real root.

## Ac 1

Given an empty target directory, bare `verdi init` (no `--wizard`)
writes exactly the `.verdi/` skeleton the README's own manual bootstrap
steps describe — a `.verdi/verdi.yaml` manifest carrying only
`schema: verdi.layout/v1` (R4-I-56's conservative scope: no invented
forge or tracker defaults) — and nothing else: no `model.yaml` at all,
since the canonical operating model governs by its absence exactly as
an already-adopted store's does.

Both the bare and the `--wizard` path refuse identically on ANY existing
`.verdi/` directory at all, not merely an existing `verdi.yaml` manifest
inside it (W-3/W-3b — the single-rename promotion both paths share
requires `.verdi` itself to be absent, or the rename fails outright with
`ENOTEMPTY`): the refusal exits 2, naming exactly what already exists at
that path, and points at hand-editing `.verdi/model.yaml` (validated by
`verdi model check`) as the reconfigure-an-existing-store path, since
that is explicitly out of v1 scope.

## Ac 2

`--wizard` requires an attached terminal and refuses, exit 2, without
one — never silently degrading to the bare path's defaults (tested via a
stdin-script harness driving the built binary: a scripted sequence of
answers fed over a real OS pipe, with a disclosed, test-only environment
override standing in for the TTY predicate alone, chosen over a pty
harness for hermetic, deterministic, dependency-free CI portability).

On a real TTY it runs a guided interview over exactly the v1 frontier's
two configurable axes (`internal/model`'s `checkFrontier`: vocabulary
display renames and a class's template-file choice, nothing else): each
of the model's renameable class, state, and verb ids is offered a
display rename, and copying the canonical templates into
`.verdi/templates/` for local customization is offered as a yes/no
choice; every answer is previewed live by validating the in-progress
candidate model in memory against the same kernel rules and frontier
check `verdi model check` itself runs, before the interview moves on.

A request for capability outside that frontier — restructuring the class
hierarchy, lifecycle states, or per-transition obligations — is refused
with an explanation naming the frontier (structural configuration
"unlocks per-verb later"; only vocabulary and template-file choices are
configurable in v1), and the interview continues rather than aborting.

The wizard writes nothing to the real store while interviewing: every
write happens inside a same-filesystem sibling temporary directory
created beside where `.verdi` will land — co-located with its eventual
parent, never `os.TempDir`, so promotion can never cross a filesystem
boundary.

## Ac 3

Promotion out of the staged temporary directory is gated on running the
complete `runModelCheck` core (`cmd/verdi/model.go`) over the staged
root exactly as `verdi model check` itself would — never a decode-only
check that would leave a wizard-written template override unvalidated —
and, when the interview diverged from canonical, on the staged
`model.yaml` decoding back to a model that is identical to the
interview's own in-memory candidate.

Promotion itself is exactly one `os.Rename` of the staged store onto the
real `.verdi` path; no other write ever touches the real root. A
mid-interview abort (stdin ending before every prompt is answered) and a
simulated crash injected partway through staging both leave nothing
whatsoever at the real root — no `.verdi/` directory at all — because
the staged temporary directory is discarded on any pre-rename error and
no real-store write ever precedes that single rename.

## Note

Disclosed ritual-order deviation: this story's build commits were
authored directly on this one `design/init-wizard` branch (per this
build's own dispatch instruction, adapted for concurrent sibling-story
worktrees), before `verdi build start` ever ran — `verdi build start
spec/init-wizard` was run afterward, once `verdi align`'s build-mode
branch-name resolution (`storyresolve.ResolveBuildSpec`, requiring a
`feature/<name>` branch) surfaced that the alignment step needed it.
