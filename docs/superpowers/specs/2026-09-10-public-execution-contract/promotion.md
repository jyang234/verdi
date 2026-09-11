# Public execution contract promotion record

**Owner decision:** “ratified”, 2026-09-10, in the current Codex task.
**Scope:** reviewed ATC design `a78ef6600246e6c5235162ed018ef0026958c6c2`,
tree `19538d011e0f1d84702c590ce8ee307e3a9d74a4`.
The independent review and its one closure completed before this decision;
the ATC design handoff records session `9324f883-26ed-4b11-add2-1762cbc391a2`
and closure SHA-256 `b6e66ccc559b5c6ff66115fa1f590fc0db78ab1002fcb9c5658b9c9e4ab522b2`.

The [story](../../../../.verdi/specs/active/public-execution-contract/spec.md)
is a proposed acceptance artifact. Ratification does not mean it has landed.
Runtime implementation waits for Git-derived acceptance on the configured
default branch. Push, PR, and merge require separate owner authorization.
This record claims no successful v2 runtime gate.

## Exact source preservation

Both files below are byte-exact UTF-8 text archives from the reviewed ATC head.
Relative Markdown links inside them are original-source locators, resolved
against ATC `docs/superpowers/specs/` at that head, not this archive directory.
Workspace PLAN remains the workspace source. Retained authorities stay at their
original locations; this is a bounded amendment, not a promotion of all of them.

| Archive | Original ATC path | Lines | SHA-256 |
|---|---|---:|---|
| [Ratified design](ratified-design.txt) | `docs/superpowers/specs/2026-09-10-public-execution-contract-design.md` | 360 | `556de816364d0c610e1bed4952b4d474874eff37a4b102c9e0096db83c4b0f30` |
| [Source witness](source-witness.txt) | `docs/superpowers/specs/2026-09-10-public-execution-contract-source-witness.md` | 141 | `7f99bcccfdb92c248f8cf4c04c982266d1d7757bc304e3af5f65725f94a8348e` |

## Coverage and transformations

| Source unit | Destination | Transformation or intentional omission |
|---|---|---|
| Design title, status, date, inspected heads, opening scope and navigation | Story metadata, DC-1/DC-4, this record, exact archive | Proposed status becomes owner-ratified design / proposed story; navigation and metadata adapted; historical bytes preserved |
| Design §1 | Story contract §1, AC-1/AC-5, CO-2 | Verbatim; ACs add indexing only |
| Design §2.1 | Story contract §2.1, AC-1/AC-2 | Verbatim; op-23 exception, SI-194, bounded read and capability usability retained |
| Design §2.2 | Story contract §2.2, AC-3 | Verbatim; legacy preimage scoped to ops 1–22 |
| Design §3 | Story contract §3, AC-2/AC-5, CO-3 | Verbatim; both endpoint invocation points and admission ordering retained |
| Design §4 | Story contract §4, AC-3 | Verbatim; every cut and same-epoch relaunch limitation retained |
| Design §5 | Story contract §5, AC-3/AC-4 | Verbatim; old actual-run limitation and quiescent rollback retained |
| Design §6 | Story contract §6, AC-1..5, CO-1 | Verbatim; two flights and every release gate retained |
| Design §7 | Story DC-3/DC-4, ATC IL-153, Verdi SI-197, this record | Requested ratification becomes recorded decision; completed design review stays historical; acceptance and external-action gates retained; stale ATC PLAN §7 ledger locator corrected to §10 |
| Witness preamble and §§1–4 | Exact witness archive, incorporated by story hash; this record | Historical status reads through ratification/DC-4; same stale ledger locator correction |
| Witness A01..A23 | Contract sections named by all 23 rows; AC-1..5 | All named supersessions retained; no expansion |
| Witness R01..R12 | Original sources and contract destinations named by all 12 rows | Retained by reference, not promoted or rewritten |
| Witness operations 1..23 | Contract §§2–3 and AC-1/AC-2/AC-3/AC-5 | All 22 request/result pairs and claims exception; 16 relation branches and six no-extra-relation arms retained |
| Witness limitations | Story CO-1, this record, future obligations | Unproven work remains unproven |

**Totals:** 360/360 design lines and
141/141 witness lines preserved exactly;
all design sections 1–7 and both preambles accounted for; 23/23 amendment units,
12/12 retained-source groups, 22/22 requests, 22/22 results, 1/1 local claims
operation mapped. Zero source text lost. Only historical metadata, navigation,
and the completed ratification request/review procedure are omitted from the
current story; all remain in their exact archive.

This is amendment-scope coverage, not clause-level promotion of retained
upstream sources or a completed runtime-validator migration. Implementation
must supply the separate function/invocation/mutation witness.

## Acceptance preparation

The story was created by `verdi design start` from Verdi
`8ab423fefa14f6cee3070ca96754928daf28c062` in an isolated worktree.
The existing Jira provider is configured `mode: fake`. The new bookkeeping
ref `jira:VERDI-ATC-7` did not resolve, so the CLI reported degraded title
resolution; the human title is authored here. No real tracker issue was
created or verified. The ref is reversible proposal metadata recorded in
SI-197, not evidence of external authorization.

This bounded refinement implements parent AC-4/5/6. Its original six planning
stubs are unchanged; this additional story adds no seventh feature outcome.
Static/behavioral obligations name future producers for the two flights, not
tests claimed to exist or pass. All contract §6 release checks remain mandatory
even where one obligation groups several checks.

The ten obligation producer identities `public-execution-contract:ac-N:static`
and `...:behavioral` and their `public-execution-contract-release` CI job are
future internal evidence plumbing for the already-ratified paired release
gate, not new public verbs or existing passing producers. A pair-bound report
is required because one repository's unit suite cannot establish the other
endpoint's invocation order or the paired crash/rollback outcomes. The checker
must reject missing, stale, skipped and unproven required evidence. Both
repositories' ordinary gates remain independently required.
