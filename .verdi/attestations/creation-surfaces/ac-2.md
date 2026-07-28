---
id: attestation/creation-surfaces--ac-2
kind: attestation
title: "AC-2 attested: the board creates specs from template-driven fields through the one shared producer, overrides honored"
owners: [platform-team]
links:
  - { type: verifies, ref: spec/creation-surfaces }
frozen: { at: 2026-07-22, commit: 7c35d887ad9ce0ae355de82d6c3af90bf2fd73ec }
---
# AC-2 outcome attestation

Stand-in operator attests (Phase 2, 2026-07-22): the workbench creation
form generates its fields from the target class template's own
placeholders (the D-1 field contract, `designscaffold.Fields` yielding
ordered descriptors), renders through the same shared `designscaffold`
producer every other creation surface now shares — inheriting
`CheckClass` post-render validation — and `commit-to-design` was switched
to that identical producer, ending the third-producer divergence L-M12
named, with a byte-stable parity pin over the prior inputs.

Built by a Fable agent per the owner's UI directive; proven by
Playwright over a vocabulary-rename fixture store (form labels speak the
store's display words, the landed spec has the correct class on the
right branch). The judge round here surfaced three real fixes before
merge — an enumeration walker that let `$.Bogus` through the fail-closed
contract, a receipt that could call a filled tracker ref a placeholder,
and a graduation path that could outrun what an override template
actually carried — all fixed red-first, then a fourth-round CI catch on
a fixture git-identity gap. The OUTCOME — one field contract, both front
ends, overrides honored — holds and is enforced.
