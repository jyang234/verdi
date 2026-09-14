# Mechanical spec import: implementation contract

Status: main-authored implementation decisions under the owner's adopted
2026-09-14 existing-spec-import design, SI-200. These details receive independent
review before dispatch. No runtime completion or release acceptance is implied.
The reviewed parent design remains binding; this contract fixes its enumerated
prerequisites. No frozen spec is edited or canonical promotion claimed.

## Authority return and scope

The owner's “yes adopted” authorizes the bounded OQ-3 exception and R3 enlargement:
import one feature/story from selected native or Markdown sources, create a new
proposed branch, then use the existing board. This is the explicit Wave 6 authority
return for the routes, creation service and non-authoritative import sidecar below.
It follows R0–R2 and precedes import-dependent R3/R4 journeys. Outstanding Wave 6
units retain their ordering. No acceptance, lifecycle, tracker or evidence gate
changes. Hosted testing remains deferred.

A dedicated browser import action may explicitly defer both statements using the
existing CLI's disclosed placeholder representation. This allowance is confined
to the new import route; the existing creation form keeps its requirements.
CLI preview is read-only. CLI apply uses the existing delegated-agent actor with
`--harness` and policy enforcement; it does not acquire browser-human authority.
A human with no adopted assistance policy can complete import in the browser.
No new CLI `--human`, principal claim or MCP operation is introduced. The importer
itself invokes no AI, provider, forge, tracker or remote URL.

## Public operations and closed shapes

`verdi design import preview --request <path|->` and
`verdi design import apply --request <path|-> --preview <sha256> --harness <id>
[--session <id>]` exchange strict JSON. The request can be produced from files by
`verdi design import source --root <directory> --file <relative-path>
[--start-line <n> --end-line <n>]`; this emits one Source JSON value to compose
into a request. It reads only, never imports or executes content. Browser upload
produces the identical Source shape; labels are never filesystem destinations.

The Go API is owned by `internal/specimport`. All public structs use explicit
snake_case JSON tags; byte slices use standard base64 encoding. Closed enums and
field-dependent requirements below are validated after `artifact.DecodeExactJSON`.
Unknown fields/trailing values and null top-level requests fail closed.

```go
type Target struct {
    Slug string `json:"slug"`
    Class string `json:"class"`
    Title string `json:"title"`
    Story string `json:"story,omitempty"`
}
type Source struct {
    ID string `json:"id"`
    Label string `json:"label"`
    Data []byte `json:"data"`
    StartLine int `json:"start_line,omitempty"`
    EndLine int `json:"end_line,omitempty"`
}
type Mapping struct {
    Target string `json:"target"`
    SourceID string `json:"source_id,omitempty"`
    Start int `json:"start,omitempty"`
    End int `json:"end,omitempty"`
    Transform string `json:"transform,omitempty"`
    Text *string `json:"text,omitempty"`
    Evidence []string `json:"evidence,omitempty"`
}
type Link struct {
    Type string `json:"type"`
    Ref string `json:"ref"`
}
type Request struct {
    Schema string `json:"schema"`
    Target Target `json:"target"`
    Format string `json:"format"`
    Primary string `json:"primary"`
    Sources []Source `json:"sources"`
    Mappings []Mapping `json:"mappings,omitempty"`
    Links []Link `json:"links,omitempty"`
    DeferStatements bool `json:"defer_statements"`
    RetainUnmapped bool `json:"retain_unmapped"`
}
```

Request schema is `verdi.spec-import-request/v1`. Format is exactly `native`,
`markdown-v1`, `f13-reference-v1`, or `manual-v1`; no silent fallback. Primary names
exactly one Source ID. Source IDs use lowercase ASCII `[a-z0-9][a-z0-9-]{0,63}`,
unique per request. Labels are nonblank UTF-8 display strings, at most 1024 bytes.
Target slug follows the existing bare spec-name validator, class is feature/story,
title is nonblank UTF-8; optional story uses the existing scheme-qualified grammar.
External owners use the existing `[unassigned]` scaffold default; native owners
are preserved. Other target metadata requires source editing/reupload in native
mode, not silent ID/title/class substitution.

The entire JSON envelope is limited to 12 MiB. There are 1–32 sources, each
nonempty valid UTF-8 and no more than 2 MiB, total supplied source bytes ≤8 MiB.
Line ranges are inclusive, 1-based, on physical LF-terminated lines with the
unterminated final line counted. Both zero selects the entire source; otherwise
both must be positive, ordered and within the file. Preserve CRLF and selected
bytes exactly. Record original file digest, selected digest and original line
coordinates. All mapping offsets refer to selected UTF-8 byte slices, half-open
`[start,end)`, and must land on rune boundaries. No source is fetched transitively.
The source reader rejects absolute paths, traversal components, non-regular files
and symlink components (including a symlink import root); limits apply before line
selection. It does not walk directories. Browser labels have no path authority.

## Deterministic structural mapping

Use the existing goldmark dependency for Markdown syntax, especially fenced code
and list boundaries. Only one selected target is recognized: the primary must
start with one ATX title heading; a peer/higher heading later means multiple
selected targets and is a named unresolved finding. Leading blank lines are
allowed. Metadata/source prose before the title is unrecognized, not guessed.
Field sections are direct child headings of that title, at exactly one deeper
level. Literal heading labels are case-insensitive after trimming whitespace:
`Problem`/`Problem Statement`, `Outcome`/`Outcome Statement`,
`Acceptance Criteria`, `Constraints`, `Decisions`, `Open Questions`.
No synonyms beyond these. Duplicate aliases for one field, empty fields,
unsupported nesting or multiple targets are reported; do not select the first.
Supporting sources are retained-only unless explicitly mapped by the user.

Problem/outcome select the entire section body excluding leading/trailing blank
lines, preserving all interior bytes and line breaks. A heading inside a fenced
code block is never a field. Object sections accept a flat Markdown bullet list:
one direct item = one object, in source order, with ac-/co-/dc-/oq- plus sequential
numbers generated per kind. Strip only the bullet marker and its following space;
multiline continuation indentation is a declared deterministic deindent transform.
Nested lists or mixed non-list prose make that section unresolved. No punctuation
or words are paraphrased. Existing IDs outside the supported list grammar are not
recognized implicitly; choose explicit mappings or native mode to preserve them.
Evidence is not inferred from text. The preview has a missing-evidence finding
until an explicit Mapping supplies kinds; feature attestation requirements are
still checked by existing lint. The UI can apply the user's selected kinds to
several criteria together, clearly marked as user selection.

`f13-reference-v1` binds the selected primary SHA-256
`7c6d95d4aa516cf682e6d4a33d1c260f1c861d938d46559241a7c764bffca577` and the eight
exact selectors in the reviewed `mechanical-field-map.json`. This is a named,
versioned reference profile, not a recognizer for all ATC plans. Changed primary
bytes refuse that profile with guidance to use manual mapping or labeled Markdown.
Supporting files stay whole retained-only units. Problem/outcome are absent.
`manual-v1` supplies no automatic mappings and retains all selected source.
Profiles are shipped data interpreted by fixed code, never uploaded executables.

Explicit Mappings override the automatic mapping for the same target. Duplicate
explicit targets fail. Targets are `problem`, `outcome` or valid ac-/co-/dc-/oq-
object IDs. With SourceID, Start/End select existing content and Transform is
`identity`, `trim-blank-lines`, `collapse-whitespace`, or `list-item`; list-item is
valid only for an actual supported direct list item span. If Text is omitted,
use the transformed source text and mark `copied-source`. If Text is supplied and
differs, mark `user-edited-source`; retain the original selection and transform.
Without SourceID, Text must be nonblank, offsets/transform absent: `user-added`.
Evidence accepts only static/behavioral/runtime/attestation, unique, on ACs only;
its origin is `user-selected`. Links are explicit user additions validated through
existing artifact/link/reference rules. They never arise from incidental URLs.
Text cannot select destination paths, actor, lifecycle or executable authority.

`RetainUnmapped` is the user's explicit disposition of all remaining source as
retained-only; false produces unresolved-coverage findings and prohibits creation.
It does not resolve missing/ambiguous mapped fields. The coverage record partitions
every selected source byte exactly once into the union of mapped spans and the
retained complement; overlapping field references can share a mapped interval,
with all destinations listed rather than double-counted. Report mapped/retained
byte totals, unresolved findings and field-value gaps separately. Whole retained
support documents are one declared unit each. This is byte coverage, not proof of
semantic completeness. Empty objects, missing ACs or unresolved duplicate headings
cannot be made valid merely by ticking RetainUnmapped.

## Candidate and validation

For external input, resolve the project class/template via `store.Open` and
`designscaffold.LoadTemplate`. Reuse the template/scaffold renderer and existing
`artifact/splice` typed draft mutation machinery to populate mapped attributes and
objects. Remove generated placeholder ACs/stubs; never retain them as real imported
requirements. Preserve template-defined custom fields; reject a template/model
that cannot express the candidate rather than dropping content. Every mapped text
appears in frontmatter and its body section. Generated anchors are bare `problem`,
`outcome` and object IDs with `## Problem`, `## Outcome`, `## ac-1`, etc.; validate
through `ResolveObjectAnchors`, whose current slug-symmetric semantics govern.
Do not write a second heading resolver. Map order: statements, then objects in
source/explicit insertion order; no map-iteration dependence. The scalar text
preserves selected wording after its declared transform, including multiline text.

Explicit deferral inserts both shared TODO constants and valid resolving sections,
marked `generated-deferral`; an available source statement stays retained if
replaced by deferral. Placeholders are visibly incomplete and not source quotations.
Other required fields/evidence must be valid. Native mode requires one selected
native primary whose valid ID/class/title/story match Target, status absent or
`draft`, and no frozen/supersession/lifecycle-migration metadata. Strict decode,
new-spec requiredness, anchors and project checks apply even to old native inputs;
no archive grandfathering. Native content and stable IDs remain byte-identical.
Mappings/Links/DeferStatements are refused in native mode; edit source/reupload or
use ordinary supported board editing after creation. Extra selected supports can
be retained. Native body is spec content; retained external prose is sidecar-only.

A shared read-only lint candidate seam checks a new in-memory Document in the
current corpus Snapshot using existing VL-002/003/005/006 logic plus strict decode,
ResolveObjectAnchors, requested class and model compatibility. Ref resolution,
configured tracker schemes, feature evidence floors and stub targets have one
owner, never copied into the importer. Creation does not claim review/merge gate
readiness. Stories require their existing valid parent/implements or spike/resolves
relationships and configured tracker; no TODO tracker is synthesized. Preview
findings explain the rule, target and next corrective action. Unrelated pre-existing
corpus findings are disclosed separately and cannot silently validate a candidate
whose dependencies fail to decode or resolve.

## Shared internal interfaces

`NewService() *Service` wires the production application dependencies. Public
DTOs and strict codecs stay in specimport; their JSON keys above/below are the
wire contract. `DecodeRequest([]byte) (Request, error)` is the sole request decoder.
`Normalize(Request) (Plan, error)` performs pure source selection/mapping and
returns `Plan{Sources []Snapshot, Fields []Field, Coverage []Coverage,
Findings []Finding, Native []byte}`. Snapshot carries ID/Label/Data,
OriginalDigest/Digest, StartLine/EndLine. Field carries Target/Text/Origin,
Spans []Span and Evidence []string; Span carries SourceID/Start/End/Transform.
Coverage carries SourceID, TotalBytes/MappedBytes/RetainedBytes/UnresolvedBytes,
and ordered Intervals with Start/End/Disposition/Targets. Finding carries
Code/Target/Message/Blocking. PreviewResult uses these same typed values rather
than independently serialized adapter DTOs. JSON keys are the snake_case forms
of these names, with `data`/`native` as base64. Internal Plan is not a public
request format. Maps are serialized through existing canonical JSON machinery.

`ReadSource(ctx context.Context, importRoot, relativePath string, startLine,
endLine int) (Source, error)` supplies the safe read-only CLI file helper.
`Compose(ctx context.Context, root string, request Request, plan Plan)
([]byte, []Finding, error)` owns the existing renderer/splice integration and
shared candidate lint checks. It does not write. `ReadRecord(ctx context.Context,
root, branch, slug string) (RecordView, error)` owns committed provenance reads;
RecordView includes the decoded record, original import commit and an explicit
current-spec-match boolean/disclosure. A moved but valid descendant branch does
not erase the historical import revision; malformed/missing records refuse or
disclose missing proof. No claimed verification derives solely from self-reported
record hashes without checking referenced Git bytes.

## Preview, identity and atomic publication

`(*Service).Preview(ctx context.Context, root string, request Request) (PreviewResult, error)` is read-only. Its result has schema
`verdi.spec-import-preview/v1`, `digest`, `base_commit`, `model_digest`,
`config_digest`, `request_digest`, `spec_ref`, `candidate` (base64; omitted when
not constructible), `fields`, `sources`, `coverage`, `findings`, `ready`.
Field views report target/text/origin/source spans/evidence without claiming a
native candidate exists when mandatory data is missing. Findings have code,
target, message and blocking boolean. Every field is deterministic; digests are
SHA-256 lowercase hex over canonical JSON excluding the digest itself. Request
order is significant; the same bytes/options/project context reproduce the result.
The base is current HEAD. Model digest uses `model.Model.Digest`; config digest
binds the committed manifest, model/template overrides and dependency corpus used
in validation. Prepare requires a clean tracked checkout/index and no untracked
corpus/config inputs (ignored `.verdi/data` is allowed); it refuses with correction
guidance instead of reading an uncommitted context that cannot be reproduced.
This first version may refuse unrelated tracked edits; it never resets them.

`(*Service).Apply(ctx context.Context, root string, request Request, expectedDigest string, actor draftmutation.Actor) (Result, error)`
recomputes preview and rejects a changed digest or any blocking finding before
Git publication. Client-supplied candidate/provenance/actor fields do not exist.
Export the existing draftmutation actor-policy dispatcher as a shared function,
without changing its authorization matrix. Browser adapters alone construct the
explicit browser-human actor. CLI builds NewDelegatedAgent from harness/session.
Record real attribution/policy posture; neither principal nor source author is
inferred. Invalid adopted policy remains an operational refusal, not non-adoption.

Commit only `.verdi/specs/active/<slug>/spec.md` plus
`.verdi/imports/<slug>/<preview-digest>/record.json` and
`.verdi/imports/<slug>/<preview-digest>/sources/<source-id>.md`.
Use fixed trusted path constructors in `internal/store`. `.verdi/imports` is an
admitted top-level provenance area, ignored by artifact index classification and
normal design/build context. Its sole decoder belongs to specimport. Record schema
`verdi.spec-import-record/v1` stores preview/base/model/config/request/candidate
digests, spec ref, normalized source identities/ranges/digests, mappings/origins,
coverage, actor attribution and policy posture. No clock/randomness in this record.
This new creation record supplies import provenance atomically; it does not forge
an ASD mutation entry for a nonexistent prior draft. Subsequent ordinary mutations
use existing design provenance unchanged. Source origins are copy claims, not
third-party authorship proofs. Inspector must validate record/snapshot/candidate
bindings and disclose unavailable or mismatching proof.

Use shared Git plumbing to build the base tree with the complete write set in
sorted path order, create one child commit, then create-only publish
`refs/heads/design/<slug>`. No checkout/index change. Reject any active/archive
spec identity or target branch collision. Recheck HEAD/context before publication;
cooperating import operations use one checkout lock, and create-only ref CAS is
final authority for collisions. Arbitrary simultaneous out-of-process repository
rewrites are outside this local tool's concurrency guarantee, never a claimed
serializable global Git transaction. Git object/commit timestamps follow existing
Git machinery; only candidate/provenance bytes are deterministic.

Before ordinary collision/stale handling on retry, an existing target can be
reported `already-created` only if its tip is exactly the recorded base plus the
expected import write set, the recorded preview/request/actor match this call,
and every candidate/source/record binding verifies. No matching record or a moved
branch returns collision; never overwrite, reset or mint another identity. Failure
before ref publication has no visible branch/import; unreachable Git objects are
ordinary disposable plumbing. An uncertain response is resolved by the same request
and preview digest. Result schema `verdi.spec-import-result/v1` has status `created`
or `already-created`, branch, commit, spec_ref, preview_digest and board_path using
the existing `/b/design%2F<slug>/board/spec/<slug>` branch-board route convention.

## Errors and browser behavior

Operational errors (CLI exit 2) have codes `invalid-request`, `invalid-source`,
`unsupported-format`, `invalid-model`, `identity-unavailable`, `authority-invalid`,
`io-failure`. Completed refusals (exit 1) use `unresolved`, `invalid-candidate`,
`dirty-context`, `stale-preview`, `target-exists`, `policy-forbidden`, `actor-forbidden`,
`provenance-mismatch`. Preserve underlying shared refusal reasons. A preview with
blocking findings is returned as structured output with exit 1; success exit 0.
No branch is created by preview. HTTP errors map malformed inputs to 400, oversized
to 413, policy/actor to 403, stale/collision/dirty to 409, operational I/O to 500.
A completed preview uses 200 with `ready:false` and its explicit findings.

Browser routes: GET `/design/import`; POST `/design/import/preview` and
`/design/import/apply`; GET `/design/import/record?branch=<branch>&spec=<slug>`.
Mutation routes retain existing same-origin/method/body-size protections. No source
is rendered as raw HTML or executable Markdown. The page is discoverable from home
before new statements are requested, with labeled file/range/target selection,
automatic fields, editable explicit mappings, evidence selection, retained-source
coverage, explicit pair deferral and a review step bound to the current digest.
Any source/field edit clears the confirmation and requires fresh preview. Successful
creation opens the existing branch board and a source-record link. The record view
is read-only, works on that branch's committed bytes and exposes missing/tampered
proof truthfully. It remains non-authoritative after later supported draft edits;
show that the record describes the imported revision when current spec has changed,
not that ordinary edits corrupted original provenance.

## Acceptance and coverage witness

Parent design sections map without omission: workflow → preview/page/apply; input
preservation → bounded Source/ranges/record; mechanical mapping → formats/Mapping;
creation → candidate/lint/actor/Git; F13 → pinned profile plus negative statements;
positive labels → markdown fixture; ownership/gates → implementation plan tasks 1–5.
All six D decisions remain intact. Transformation: abstract prerequisites become
concrete grammar/types/paths. Intentional exclusions remain archive/bulk/reimport,
new lifecycle/acceptance/AI/MCP/hosted integration. This is a design-to-contract
coverage witness, not canonical promotion or evidence of implemented behavior.

Run hermetic pure/Go binary/handler tests and recording-disabled Playwright. Prove
4/4 F13 source snapshots, 8/8 mapped spans, 2409 primary bytes accounted once;
positive multiline statements; false/duplicate labels; explicit mappings/corrections;
no model/provider/network calls; strict native compatibility; retained-source context
exclusion; policy boundaries; failure/retry/dirty-index preservation; source-record
inspection after later edits; and final `make verify` plus `go test -race ./...`.
