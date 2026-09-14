# Spec-import proposal review adjudication

Status: main-authored adjudication of the single independent Opus 5 review of
`8fd72841f5faf2e7ea9133d686dacdc046de116c`; one correction pass, closure pending.
This report does not ratify authority or authorize runtime dispatch.

## Findings and disposition

| Finding | Main ruling and correction |
|---|---|
| B1 candidate composition and retained-source destination | Accept the concrete ambiguity. The broad import/mapping-contract prerequisite could encompass composition, so an implementation violation is not yet demonstrated. Nevertheless make composition, body/anchor validation and context boundaries explicit prerequisites. Retained-only bytes stay sidecar-only; mapped passages are spec content. Native body content is preserved. |
| B2 allegedly contradictory deferral | Reject the claimed authority contradiction: archived `spec/cli-creation` AC-1 explicitly permits disclosed TODO deferral, and R1 cites and preserves it. Accept the missing-citation/representation gap. Cite it, name existing constants/anchors and distinguish CLI authority from the proposed import UI allowance. A focused existing test passes. No required attribute is omitted and no lint grandfathering is used. |
| B3 Wave 6 authority return and ordering | Accept as a runtime prerequisite. Explicitly require authority return for the new route/schema/seam, record proposed R3 scope enlargement after R0–R2, and preserve outstanding Wave 6 units. OQ-3 exception alone is insufficient. |
| O1 selection omits Prove/separators | Accept disclosure gap. Whitespace-only applies inside selected spans; selection itself is distinct. Add a complete primary-byte partition and retain all unselected bytes. |
| O2 criterion grouping | Accept as an implementation prerequisite, not a demonstrated generic parser. The eight explicit prototype selectors fix its grouping; versioned profiles must define their own grouping. |
| O3 missing evidence declaration accounting | Accept. Expose eight unsettled evidence slots plus two missing statements, total 10; identify this as a limited value-gap count, not exhaustive candidate validation. Do not infer evidence kinds from commands. |
| O4 imported origin and provenance transaction | Accept as a prerequisite. Require truthful vocabulary and exact composition with existing provenance before runtime; no guessed enum or fabricated human/AI attribution. All required records publish with the candidate. |
| O5 asymmetric unit granularity | Accept clarification. Three support documents are explicitly whole-document retained-only units; no claim that their requirements have been classified. Primary bytes receive finer accounting. |
| O6 prerequisite label/exclusion wording | Accept. Name validation, transition-core and persistence/replay slices, and call the scope change a bounded exception to OQ-3. |
| O7 stale report | Accept. Update deliverables, review/model evidence and fresh verification. Preserve earlier refusal and original source-whitespace evidence as history. |
| O8 preserve strengths | Retain full selected parent conditions, slice boundaries, missing-statement honesty and target-generated ID disclosure; no correction changes snapshot bytes or criterion text. |

## Binding evidence for B2

- `.verdi/specs/archive/cli-creation/spec.md`, AC-1/AC-2: explicit pair deferral
  retains disclosed TODOs; noninteractive omission without deferral refuses.
- `docs/superpowers/plans/2026-08-29-wave-6-workbench-presentation.md`, §R1:
  the archived specification remains binding; statement sourcing is validated
  before branch creation and existing rendering behavior is preserved.
- `internal/designscaffold/designscaffold.go` and `templates/feature.md`:
  both required attributes remain present with TODO text and resolving sections.
- `go test ./cmd/verdi -run '^TestRunDesignStart_DeferStatements_DisclosesAndKeepsPlaceholders$' -count=1`:
  exit 0. This is existing CLI behavior, not runtime importer evidence.

## Closure boundary

The same reviewer receives the consolidated correction and the previously omitted
CLI authority excerpt. Main retains authorship and adjudication. Closure may
establish that this direction is suitable for detailed contracts; it cannot
substitute for adopting those contracts, the required authority return, runtime
implementation, or two independent/same-release MVP journey witnesses.
