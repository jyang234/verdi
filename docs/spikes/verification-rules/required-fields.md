# oq-5 evidence — contract-required fields vs. decoder enforcement

Source: direct reads of `docs/design/specs/02-artifact-contract.md`
(read-only, workspace root — never edited) and
`internal/artifact/{common,spec,adr,attestation,waiver,conflict,diagram,
reaffirmation,obligation}.go` (this worktree). Every `grep`/`sed` command
and its output are reproducible from this file's citations, and from
`required-fields-evidence.out`'s own `grep -n "# required"` run over the
whole contract file (not scoped to one block); no output file was long
enough to need reduction (each snippet below is the literal function
body).

## Every `# required` marker in 02's frontmatter tables

`grep -n "# required" docs/design/specs/02-artifact-contract.md`
(`required-fields-evidence.out`) returns **9** hits, in two different
frontmatter tables: 8 in §Common frontmatter (the block every kind
shares, lines 78–104) and a 9th in §Kind registry's feature-spec-specific
frontmatter addition (line 328) — the story asks for "every `# required`
in 02's **frontmatter tables**" (plural), so both are in scope; an
earlier revision of this file counted only the 8-field Common-
frontmatter block without saying so (independent review VR-5), which
this revision corrects:

```
id: <kind>/<name>          # required, must agree with path
title: string              # required
status: <per-kind enum>    # required
owners: [string]            # required; team or CODEOWNERS-resolvable handles
problem: { text, anchor }   # required; feature/story only
outcome: { text, anchor }   # required; feature/story only
frozen: { at, commit }      # required iff temporal class is frozen
provenance: { ... }         # required iff generated
supersession:                # required only on a superseding revision (R4-I-4)
```

9 fields carry an explicit `# required` (unconditional or conditional)
marker. (`schema`, `links`, `acceptance_criteria`, `constraints`,
`decisions`, `open_questions` are explicitly `# optional` in the same
tables and are out of scope for this count.)

## The table

| # | field | 02's requirement | decoder field | accept or reject absence | evidence |
|---|---|---|---|---|---|
| 1 | `id` | required, every kind | `Base.ID` (`internal/artifact/common.go:298`) | **REJECT** — `ParseRef(b.ID)` errors on `""` (missing `/`) | `common.go` `validateBase`, line 316-319 |
| 2 | `title` | required, every kind | `Base.Title` | **REJECT** — explicit `if b.Title == ""` | `common.go:326-328` |
| 3 | `owners` | required, every kind | `Base.Owners` | **REJECT** — explicit `len(b.Owners)==0`, plus no-empty-entry check | `common.go:329-336` |
| 4 | `status` | required, every kind (no carve-out stated in the table text) | `SpecFrontmatter.Status` / each other kind's own `Status` field | **SPLIT** (see below) | see below |
| 5 | `problem` | required; feature/story only | `SpecFrontmatter.Problem *Attribute` | **SPLIT** (see below) | see below |
| 6 | `outcome` | required; feature/story only | `SpecFrontmatter.Outcome *Attribute` | **SPLIT** (see below) | see below |
| 7 | `frozen` | required iff temporal class is frozen | `Base.Frozen *Frozen` | **REJECT when required, and REJECT when present-but-not-required** — `requireFrozen(frozen, required, kind, status)`, both directions enforced | `adr.go:60-67`, called from every kind (`adr.go:51`, `attestation.go:31`, `conflict.go:52`, `diagram.go:125,131`, `obligation.go:247`, `reaffirmation.go:74`, `spec.go:408,500,536`, `waiver.go:43`) |
| 8 | `provenance` (top-level) | required iff generated | `Base.Provenance *Provenance` | **ACCEPT (absence never checked)** — `common.go:347` validates it only `if b.Provenance != nil`; no kind computes "is this artifact generated" and gates on it the way `requireFrozen` gates on temporal class (the one place `Frozen`+`Provenance` are jointly constrained is `board.go:80`'s `(Frozen==nil) != (Provenance==nil)` XOR check, which is board-record-specific pairing, not a generated-artifact-wide "provenance required" rule) | `common.go:342-350`; no `requireProvenance`-shaped helper exists anywhere in `internal/artifact` (`grep -rn "requireProvenance" internal/artifact` = 0 hits) |
| 9 | `supersession` | required only on a superseding revision (R4-I-4) — §Kind registry's feature-spec frontmatter addition, `02-artifact-contract.md:328`, a different table from rows 1-8's §Common frontmatter | `SpecFrontmatter.Supersession *Supersession` | **ACCEPT (absence never checked, at either layer)** — `spec.go:395` validates the block's own contents only `if fm.Supersession != nil`; nothing in `internal/artifact` computes "does this revision carry a `links: {type: supersedes}` edge, and if so, does it also carry a `supersession:` block" and rejects the mismatch. Checked whether `internal/lint` VL-015 backstops this at lint time instead (as row 7's `frozen` is backstopped by `requireFrozen`, both directions): it does not — `vl015.go:48`'s own entry guard is `if ... \|\| d.Spec.Supersession == nil { continue }`, so VL-015 **skips** a document outright when the block is absent; it only validates an ALREADY-PRESENT block's own completeness (every predecessor object classified exactly once) and never asks whether a revision with a `supersedes` link but no `supersession:` block at all should have one. `grep -rn "Supersession == nil\|Supersession != nil" internal/lint/*.go` = 2 hits total, neither doing that check. This is the same UAT-008 shape as rows 4-6 with **no backstop at either
layer** — narrower in scope than rows 4-6 (only feature-class specs may
carry a non-nil `supersession:` block at all: `validateStory`,
`spec.go:448`, and `validateComponent`, `spec.go:530`, both explicitly
**reject** a non-nil `Supersession` with the same
`fm.Supersession != nil` guard, so a story or component spec claiming to
supersede something is a decode error, not an accepted-but-unchecked
gap) — but for the one class where it is meaningful, the absence is
genuinely unchecked, not merely lint-deferred | `spec.go:390-398,
448, 530`; `internal/lint/vl015.go:45-49` (the guard that skips rather
than requires) |

### Row 4 — `status`, split by kind/class

Mechanically confirmed, `internal/artifact/{adr,attestation,waiver,
conflict,diagram}.go`: every non-spec kind rejects an empty/absent status
— `!xStatuses[fm.Status]` is true for `fm.Status==""` too, since `""` is
never a map key in `adrStatuses`/`waiverStatuses`/`conflictStatuses`/
`diagramStatuses`, so absence fails exactly like an unknown value
(`adr.go:36-38`, `waiver.go:34-36`, `conflict.go:35-37`,
`diagram.go:122-124,127-129`). Attestation and reaffirmation carry no
`Status` field at all — 02's own per-kind registry already documents
"(none — existence is the record)" for attestation, so this is a
pre-existing, table-consistent carve-out, not a new divergence
(`attestation.go:3-7`).

Spec kind is where the split lives, and it is class-conditional:
- **spec/component**: unconditionally required —
  `internal/artifact/spec.go:194-196` (the `SpecFrontmatter.Status` field
  comment), quoted verbatim: "Status is REQUIRED for the component class
  (validateComponent still checks specComponentStatuses unconditionally)
  but OPTIONAL for the feature and story classes"; enforced at
  `spec.go:519`, `if !specComponentStatuses[fm.Status] { ...error... }`
  (unconditional — no `fm.Status != ""` guard, unlike the two classes
  below). `status.go:8-16`'s own package-doc comment makes the same
  point in different words ("this enum only governs an EXPLICIT status
  value on those two classes... since a feature/story's authoritative
  proposed-versus-accepted state is Git-derived") but never uses the
  word "component" or "requires" — the earlier draft of this row
  fabricated a continuation attributed to that comment; corrected per
  independent review VR-4 to cite spec.go instead, where the component
  contrast is actually written.
- **spec/feature, spec/story**: **REJECT explicit-unknown, ACCEPT
  absence** — `spec.go` `validateFeature`/`validateStory` both open with
  `if fm.Status != "" && !specFeatureStatuses[fm.Status] { ...error... }`
  (`spec.go:330-333` feature, `spec.go:419-422` story) — an omitted
  `status:` line never reaches that branch at all.

**This is UAT-008** (`docs/design/uat/uat-findings.md:47,133-138`,
open/low/docs, witness cited there is this same file): "02 §Common
frontmatter lists `status: <per-kind enum> # required`. The imported spec
has no `status:` line and lint passes." This spike's own read confirms
UAT-008's witness exactly and narrows it: the acceptance is not a general
decoder laxity, it is scoped to exactly two spec classes (feature, story)
— every other kind and spec/component still enforces 02's table as
written.

### Rows 5/6 — `problem`/`outcome`, split by spec class

`validateObjectBlocks` (`spec.go:543-553`, shared by both classes)
validates `Problem`/`Outcome` **only if non-nil** — it never itself
requires either. The two classes diverge in what they layer on top:
- **spec/story**: explicit unconditional checks, `if fm.Problem == nil {
  ...error("story spec requires a problem attribute")... }` and the
  matching check for `Outcome` (`spec.go:424-434`) — **REJECT**. The
  function's own doc comment states why: "story is wholly new as of round
  four — no v0 fixture ever carried class: story — so Problem, Outcome,
  and Story are all required unconditionally here, with no grandfathering
  tension" (`spec.go:410-412`).
- **spec/feature**: `validateFeature` (`spec.go:327-408`) calls
  `validateObjectBlocks(fm.Problem, fm.Outcome, ...)` and adds no
  additional nil check of its own anywhere in the function — **ACCEPT**.
  Grep confirms: `grep -n "fm.Problem == nil\|fm.Outcome == nil"
  internal/artifact/spec.go` matches only inside `validateStory`, never
  inside `validateFeature`.

This is a second, previously-unrecorded contract/decoder gap of the same
UAT-008 shape (02's table marks `problem`/`outcome` "required; feature/
story only" with no class carve-out inside that clause, but only the
story half of "feature/story" is actually enforced) — found by this
spike, not yet in `uat-findings.md`; the spike does not open a new
UAT-0xx entry itself (out of this brief's write set) but names it here so
the ratification request can decide whether to fold it in alongside
UAT-008 or file it separately.

## Total

Of the **9** explicitly `# required`-marked fields across 02's two
frontmatter tables: **3 unconditionally enforced everywhere** (id, title,
owners), **1 conditionally enforced correctly and bidirectionally
everywhere** (frozen), **2 never enforced at either the decoder or lint
layer despite an unconditional- or conditional-sounding table entry**
(provenance's top-level presence; supersession's presence on a
superseding revision), and **3 split by kind/class**, two of which
(status, problem/outcome) diverge from 02's stated text specifically for
spec/feature — the corpus's most common class.
