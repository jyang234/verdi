---
id: attestation/residue-reclamation--ac-1
kind: attestation
title: "AC-1 attested: verdi gc --reclaim-unmanaged reclaims provably-dead unmanaged branches/worktrees within the ratified predicate, with disclosed second guards and recorded deviations"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/residue-reclamation }
frozen: { at: 2026-07-20, commit: cdc4154dd70e08667d0b820c529486607244f16e }
---
Attested by the controller as the owner's delegated stand-in (2026-07-19
directive; reclamation design owner-accepted and amendment owner-ratified
2026-07-20): the outcome holds. `verdi gc --reclaim-unmanaged` plans and,
on `--apply`, executes reclamation of provably-dead unmanaged
branches/worktrees strictly within the ratified predicate — merged,
clean, never primary or invoking, explicit double opt-in, git's own
refusals as independent second guards, every action and every kept row
disclosed one line each with tip SHAs for recovery. Validated against
real fixtures end-to-end; the dry-run/apply transcript and both
second-guard refusals are witnessed in the build's evidence.

Bounded honestly: the R4-I-80 branch-only corner is caught at apply time
not plan time (disclosed in dc-2); derived/cache pruning bullets remain
unimplemented (co-1); reclamation of anything unmerged — including
redundant superseded-elsewhere close branches — remains out of scope by
the amendment's own terms. The unmerged-branch-only disclosure gap, the
dry-run tip omission, and the conservative dc-2 managed-guard widening
are recorded accepted-deviations (R4-I-82/83).
