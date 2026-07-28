---
id: attestation/creation-surfaces--ac-3
kind: attestation
title: "AC-3 attested: design start gains flags, interview, and --from-stub through the same seams as the board"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/creation-surfaces }
frozen: { at: 2026-07-22, commit: 7c35d887ad9ce0ae355de82d6c3af90bf2fd73ec }
---
# AC-3 outcome attestation

Stand-in operator attests (Phase 2, 2026-07-22): `design start` reached
the CLI parity the board already had. `--problem`/`--outcome` produce a
statement-filled scaffold; `--defer-statements` commits deliberate TODOs
only with an explicit disclosure line; a TTY interview prompts when the
flags are absent, deriving its prompts from the SAME
`designscaffold.Fields` descriptors the board form uses — one field
contract, two front ends, never a second hand-rolled list. `--owners`
stays deliberately out (the I-10/X-4 posture).

`--from-stub` closed the ADJ-65 asymmetry at the mechanism, not the
surface: the stub-instantiate core was extracted into a shared
`internal/stubinstantiate` package that both the board action and the
CLI path call, proven behavior-preserving by the board's own handler
tests passing unmodified and by a parity test asserting the two paths
land byte-identical committed specs. Statement fields are required
content, so an empty value routes to the interview or a named refusal,
never a silently-placeholdered spec. The frozen "TODO-free" wording is a
cross-surface scaffold-body-template limitation queued for ratification;
the OUTCOME — create a tailored spec from the CLI, symmetric with the
board — holds.
