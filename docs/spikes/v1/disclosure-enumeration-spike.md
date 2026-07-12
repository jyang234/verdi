# Disclosure enumeration spike — answer

Resolves `spec/disclosure-legibility#oq-1`: "should disclosures be
machine-enumerable (MCP/audit surface), and what belongs in the
enumeration?"

## Recommendation: yes, build it — as a thin aggregator over what already
exists, not a new subsystem

Every disclosure-producing call site already exists and already computes
the same three-valued judgment (proven / violated / disclosed-unproven,
constitution 2) on demand from the current checkout. Building the
enumeration is not a new sensing problem — it is a *rendering and
collection* problem, which is exactly what DC-1 already commits this
feature to: "the rendered-state shape has to exist as a real seam other
producers can call into before any one view can enumerate through it."
That seam does not exist yet. Four call sites currently invent their own
shape independently:

| Producer | Shape today | Where |
|---|---|---|
| `internal/lint` | `Finding{Severity: SeverityDisclosure}`, rendered `"notice: VL-xxx path: message"` | `internal/lint/finding.go`, VL-017 |
| `cmd/verdi` gate | `gateCondition{Disclosed: true, Name, Reason}`, rendered `"[NOTICE] name\n       reason"` | `cmd/verdi/gate.go`, `closuregate.go`'s pending-supersession-on-nil-forge case |
| `internal/mcpserve` | ad hoc JSON string field, named per tool (`review_unavailable`) | `tool_list_annotations.go`, `tool_get_board.go` |
| `internal/workbench` | `proj.Notices []string`, free text, mixes review-unavailability with unrelated chrome banners (e.g. assumed default branch) | `boardspec.go` |
| `internal/align` | inferred from a sentinel `DecisionAbsenceFindingID` finding's presence in the judged section, surfaced only as the terse status label `disclosed-unproven-complete` | `internal/align/decision_report.go` |

Five surfaces, five shapes, none of them reusable by a sixth. This is
precisely the scattering ac-1 describes ("surface scattered across CLI
stderr, lint output, and alignment reports in ad hoc wording"). An
MCP-enumerable surface is achievable at low cost specifically *because*
every producer already does the hard part (deciding something is
disclosed-unproven); what's missing is a shared struct and one aggregator
function that calls the same decision points, not new instrumentation.

**Scope discipline for v1**: this spike recommends the shared shape and
the read surface. It does not recommend refactoring all five existing call
sites in the same change — that migration is real work belonging to
story-1 (`disclosure-seam`, already stubbed against ac-1) and is
explicitly out of the spike's timebox (spikes are exploration, not
delivery, 03 §Ceremony pricing). What this spike specifies is precise
enough that story-1/story-2 need make no further shape decisions.

## The enumeration item shape

```go
// Disclosure is the one shape every disclosure-producing call site emits
// or is refactored to emit (ac-1's "one vocabulary"). It carries at
// minimum what is unproven and why (DC-1).
type Disclosure struct {
    // ID is a deterministic, content-derived identifier — never a ULID or
    // wall-clock stamp (ground rules: "no wall-clock or randomness in
    // generated artifacts"). Computed as source + "/" + locus, so the same
    // disclosure re-derives the same ID on every call, letting a caller
    // diff two enumerations (did this disclosure appear/disappear?)
    // without persisting anything. Example: "lint:VL-017/spec/disclosure-legibility".
    ID string `json:"id"`

    // Source names the producing rule/verb/condition, reusing the id each
    // producer already has today rather than inventing a new taxonomy:
    // a lint rule id ("lint:VL-017"), a gate condition's existing Name
    // ("gate:pending-supersession"), align's judged-section label
    // ("align:decision-conflict"), or the review-feed's existing name
    // ("mcp:review-feed", "workbench:git-state"). Enumerable and grep-able
    // against the producer's own source file.
    Source string `json:"source"`

    // Scope is the artifact or ref this disclosure is about, when there is
    // one (a spec ref, a story ref) — omitted for checkout-wide
    // disclosures that name no single artifact (e.g. the assumed default
    // branch). This is what lets a board/panel filter to "disclosures for
    // the spec I'm looking at" (ac-2's "one view") without the caller
    // re-parsing Text.
    Scope string `json:"scope,omitempty"`

    // Text is the human-readable explanation — exactly the message each
    // producer already computes (Finding.Message, gateCondition.Reason,
    // the review_unavailable string, a Notices entry), never re-derived.
    Text string `json:"text"`

    // Severity is deliberately a closed one-value enum today:
    // "disclosed-unproven" is the only kind of disclosure the system
    // currently produces (constitution 2's three-valued honesty has
    // exactly one non-terminal state besides proven/violated, and
    // violated is never a disclosure — it is a verdict failure, reported
    // through a different channel entirely). The field exists, rather
    // than being omitted, because internal/lint's own Severity type
    // already anticipates more than one non-violation state existing in
    // principle; a fixed single value costs nothing and avoids a breaking
    // schema change if that ever stops being true. Nothing here invents a
    // grading scheme the codebase doesn't already have (CLAUDE.md:
    // "never resolve a spec ambiguity ... from what similar tools do").
    Severity string `json:"severity"` // always "disclosed-unproven" in v1
}
```

Rejected alternative: a free-text-only shape (just `Text`), matching
today's `proj.Notices []string`. Rejected because ac-2 requires
enumeration, not just concatenation — a caller needs `Source` and `Scope`
to group, filter, or diff disclosures, none of which a bare string
supports without re-parsing prose, which is the exact scattering ac-1
exists to end.

## Where enumeration lives, mechanically

A new `internal/disclosure` package owns the `Disclosure` struct and one
function, `Render(Disclosure) string`, producing the single vocabulary
ac-1 wants: `"disclosed-unproven [<source>]<scope suffix>: <text>"`. This
replaces the three different prefixing conventions above
(`"notice: "`, `"[NOTICE] "`, and the un-prefixed `review_unavailable`
field) with one — the seam DC-1 asks for. Existing producers are refactored
(story-1's job, not this spike's) to construct a `Disclosure` at their
existing decision point and either render it locally via `Render` (CLI
surfaces) or return it for collection (MCP/board surfaces); no producer's
*decision logic* changes, only what shape it hands to its caller.

A read-only aggregator, `internal/disclosure.Enumerate(ctx, root, scope
*artifact.Ref) ([]Disclosure, error)`, computed fresh on every call against
the current working tree (never persisted, matching ac-2: "reflects the
checkout's current state, not a historical log" and matching how
`proj.Notices` is already assembled synchronously per board render) —
calls the same functions lint/gate/align/workbench already call to decide
disclosedness, and collects their `Disclosure` values. An optional `scope`
narrows to one spec/story; nil enumerates the whole checkout.

**MCP tool**: `list_disclosures`, following the `get_board`/
`list_annotations` pattern exactly (`tool_list_disclosures.go`, registered
in `tooldefs.go`, read-only, carries `dataNeverInstructionsNote` since
`Text` is producer-authored free text): input `{ scope?: ref }`, output
`{ disclosures: Disclosure[] }`. This is additive to the MCP read surface
(05 §MCP server), not a replacement for `list_annotations`'s
`review_unavailable` field or `get_board`'s — those stay as they are for
backward compatibility (they are themselves *instances* of the shape this
spike specifies, expressible as `Disclosure{Source: "mcp:review-feed", ...}`
going forward, but migrating their wire shape is a breaking-change decision
this spike does not make).

**Board panel** (`disclosures-panel`, story-2's own stub, ac-1+ac-2):
renders `Enumerate` scoped to the spec being viewed, using `Render` for the
same text a CLI run would show — the literal cross-surface consistency
ac-1 asks for, provable by a behavioral test that asserts the panel's
rendered text and a CLI disclosure's rendered text agree byte-for-byte for
the same underlying `Disclosure`.

## What this spike does not answer (left to story-1/story-2 explicitly)

- Which of the five existing call sites get refactored first, and whether
  that is one story or several — a delivery-sequencing question, not a
  design one.
- Whether `list_disclosures` becomes part of the v1 MCP tool count (05
  currently names nine tools; this would be a tenth) — a scope/ceremony
  decision for story-2's own acceptance, not this spike's to make.
- Persistence/history of disclosures over time (an audit trail of what was
  disclosed on past commits) — explicitly out of scope: ac-2 states the
  enumeration is current-state-only, not historical, and DC-1 does not ask
  for a log.

## Answered

This closes oq-1 as specified in the feature spec: disclosures should be
MCP-enumerable (recommendation: yes), and the enumeration item shape is
`{id, source, scope?, text, severity}` as specified above, backed by a
shared `internal/disclosure` package and a `list_disclosures` MCP tool
following the existing `get_board`/`list_annotations` pattern. Story-2's
spec should cite this document directly rather than re-deriving the shape.
