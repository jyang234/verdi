---
id: attestation/creation-surfaces--ac-1
kind: attestation
title: "AC-1 attested: verdi init reaches a working store by both paths, and can never leave a half-configured one"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/creation-surfaces }
frozen: { at: 2026-07-22, commit: 7c35d887ad9ce0ae355de82d6c3af90bf2fd73ec }
---
# AC-1 outcome attestation

Stand-in operator attests (Phase 2, 2026-07-22): `verdi init` ships both
paths — the bare non-interactive scaffold wrapper and the `--wizard`
configuring interview. The atomicity floor is real and stronger than the
frozen AC first named it: the wizard builds the complete candidate store
in a sibling temp dir, gates promotion on the FULL model-check core over
that staged root, and promotes by exactly one atomic rename-exclusive
syscall — so a mid-interview abort or crash leaves nothing at the real
root, and any existing `.verdi/` is refused, not silently replaced.

The strengthening was itself witnessed and adversarial: a fix agent
empirically REFUTED the judge's rename-race premise AND my own mkdir-claim
ruling with cross-platform probes (os.Rename returns EEXIST on APFS/
overlayfs; only raw ext4 silently replaces), and returned the safer
primitive. `TestCLIShowcaseInit` proves the create-only refusal against
the real examples/showcase store (byte-untouched) and the creation path
against a fresh scratch dir, both through the built binary. The
os.Rename/ENOTEMPTY wording in the frozen ac-1/ac-3 is stale relative to
the built verb and is queued for ratification precision; the OUTCOME —
a working store either way, never a broken one — holds.
