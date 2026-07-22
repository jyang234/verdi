---
id: obligation/obligation-seam--ac-4--behavioral
kind: obligation
title: "the board's existing obligation-graduate behavior is unchanged after the renderer moves behind the shared seam"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/obligation-seam" }
frozen: { at: 2026-07-21, commit: af0edd77237b6c52cffda3bc344c020ff5fad58e }
---
# the board's existing obligation-graduate behavior is unchanged after the renderer moves behind the shared seam

The behavioral evidence must show that
`internal/workbench/obligationauthor_test.go`'s existing suite
(`TestBoardSpec_ObligationGraduate`, `TestBoardSpec_ObligationGraduate_
StampIsCommitDerived`, `TestBoardSpec_ObligationGraduate_Negative`,
`TestBoardSpec_ObligationGraduate_FeatureWallRefused`,
`TestWriteObligationFileUsesAtomicWrite`,
`TestObligationAuthor_AtomicWrite_NoDirectCreateTemp`) passes UNMODIFIED
— no assertion loosened, no fixture reshaped to accommodate the
extraction — after `renderObligation`/`writeObligationFile` move behind
the shared `internal/evidence` seam, proving the rendered bytes (id,
title, owners, for_kind, links, frozen stamp, body) and the write
semantics (atomic, fsync'd, no leftover `.tmp` sibling, refuse-on-existing
preserved at the board's own call site) are byte-for-byte identical to
before the extraction. A second assertion must show accept's backstop and
`verdi obligation author` both produce output that round-trips through
the identical `artifact.DecodeObligation` self-validation the board's own
path already exercises — the same seam, exercised from three different
call sites, never three implementations that happen to agree today.
