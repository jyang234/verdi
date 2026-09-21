---
id: conflict/readiness-parity-request-input-unspecified
kind: conflict
title: "readiness-recovery ac-4 promises four-surface byte parity the CLI cannot meet without a context request"
status: superseded
frozen: { at: 2026-09-21, commit: d72c55c66838d0b64b253681f10b94df6ce67b8d }
owners: [platform-team]
links:
  - { type: challenges, ref: spec/readiness-recovery }
---
# Conflict: ac-4's byte parity does not qualify the context-request input

## What is disputed

`spec/readiness-recovery` ac-4 requires readiness for the same ref at the
same HEAD to read byte-identically on the CLI, the board readiness page, the
board Document tab, and MCP get_document. ac-3 of the same spec keeps
`verdi serve --context-request` as an optional, served-only startup pre-run
of the full policy-conflict evaluation, and ac-4's own parenthetical shows
the startup spec on the readiness page when that request is given. `verdi
spec doc` takes no request: spec/spec-documents ac-2 declares its flag set
and carries none, and verdi-surfaces §CLI has no row for the verb. For the
startup request's own spec the served surfaces therefore evaluate the
check-context area through the request and the CLI cannot, so the two
readings differ in exactly the Readiness section. Yesterday's accepted truth
— unqualified four-way byte parity — is contested as unmeetable by any
conforming implementation.

## Witness

The independent review of readiness-recovery waves 1 and 2
(`docs/superpowers/reports/2026-09-21-readiness-recovery-independent-review.md`,
finding R4) showed that the shipped parity test
`TestDocumentParity_ServedWithContextRequest` (`cmd/verdi/document_parity_e2e_test.go`)
passes only by asserting that the CLI and served readings of the startup
spec differ. Ledger row SI-217 (`docs/superpowers/invention-ledger.md`)
records the conflict, the four options (keep and disclose; drop the request
from the Document tab and MCP; add a request flag to `verdi spec doc`; amend
ac-4 to define parity over equivalent request inputs), and the interim
disclosed-as-unproven posture. The independent closure check
(`docs/superpowers/reports/2026-09-21-readiness-recovery-independent-closure.md`)
confirmed the record accurate and left the choice to the owner.

## Resolution

Feature supersession (`spec/verdi-evidence-model` §The amendment ladder rung
4): `spec/readiness-recovery-v2` supersedes `spec/readiness-recovery`,
amending ac-4 to define parity over equivalent context-request inputs — the
startup request's own spec may differ between a served surface deriving
through the request and the request-free CLI in exactly the Readiness
section, disclosed by the no-request and stale-request witnesses — and
carrying every other object byte-identical. The owner chose this option on
2026-09-21 over a `verdi spec doc` request flag, which remains available as
a later feature. Cascade fold: zero affected stories — the only frontmatter
edges into `spec/readiness-recovery` are whole-spec `depends-on` links on
five feature specs (`ritual-write-scope`, `ritual-write-scope-v2`,
`self-governance`, `mutation-ratchet`, `verification-rules`); their spike
stories mention the spec only in body prose and declare no edge, and no
story anywhere carries an object-level edge into ac-4 — so the single-owner
acceptance price applies.
