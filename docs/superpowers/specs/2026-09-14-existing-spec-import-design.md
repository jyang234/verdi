# Import existing specifications into Verdi

Status: proposed design. The owner endorsed reviewed import and selected F13
as the reference/prototype on 2026-09-14. This document proposes the detailed
contract; it is not canonical ratification, implemented behavior, or acceptance
of a changed MVP release. The prototype is data only. Owner feedback subsequently narrowed import to
mechanical mapping; AI-assisted extraction is excluded from this version.

## Contents

1. Purpose and authority
2. Import workflow
3. Input and preservation rules
4. Mechanical mapping and source classification
5. Draft creation and failure behavior
6. F13 prototype and validation
7. Proposed decisions and delivery boundary

## Purpose and authority

An operator with an existing feature definition should be able to select its
Markdown sources, review a populated specification, and open its board without
retyping requirements. `spec.md` remains the semantic source of truth; cards
remain projections. Scratch annotations remain a separate non-authoritative
layer. The importer produces spec objects, never a competing card database.

Binding context: artifact contract §Object model (objects come from declared
frontmatter and resolved anchors, not inferred prose); AI-assisted spec design
(2026-07-30), especially shared mutations, human governance, model neutrality,
context integrity and provenance; merge-signaled acceptance (2026-08-01); the
adopted R3/R4 milestone boundary. Workspace PLAN §OQ-3 excludes import in v0.
This is a proposed explicit extension of that exclusion for the bounded flow
below. Archive migration, lifecycle import, general document conversion and
changes to acceptance/evidence rules remain outside it. No frozen authority is
edited or silently superseded by this proposal.

## Import workflow

1. **Choose sources.** A workbench entry point and a CLI equivalent accept a
   native draft spec, a Markdown file, or an explicit set of Markdown files.
   A file can have an explicitly chosen heading/line range, as F13 shares a
   plan with other features. Show the selection and destination before work.
2. **Prepare preview.** Decode native structured fields or apply an explicit,
   versioned mapping for a supported Markdown structure. Unrecognized sections
   remain attached source material for the user to map; do not guess. Show mapped
   problem/outcome, objects and relationships beside source passages, using the
   board's existing projection concepts and the project's display vocabulary.
   Preparation does not create a design branch or write a destination spec.
3. **Review coverage and gaps.** Distinguish copied passages, declared formatting
   transformations, user mappings/additions, retained supporting material, exclusions and
   unresolved content. Show conflicting requirements and ambiguous ownership.
   The user can edit the proposal or the selection and regenerate the preview.
4. **Create draft.** On explicit confirmation of the current preview, validate
   the complete candidate and commit a fresh design branch. Open its ordinary
   board, showing draft/proposed posture and the exact source/import record.

The UI must offer the choice to use existing material before asking someone to
supply new problem/outcome statements. A preview may be incomplete. Unresolved
source dispositions prevent final creation. A required statement absent from the
source can be mapped by the user or explicitly deferred through the existing
disclosed statement-deferral contract; that creates an incomplete draft, not
review readiness. Other invalid mandatory fields still block creation. Confirming
content mappings is draft authorship, not governance approval.

## Input and preservation rules

The first version targets one feature or one story per import. F13 is the
feature prototype. Multiple documents may describe that one target; a document
containing multiple features requires explicit selection. Bulk creation of many
features, recursive directory discovery, archives/ZIP, network URLs and automatic
link traversal are deferred. A bundle means the selected Markdown files, not an
executable package or a new instruction channel.

Use UTF-8 Markdown, explicit file identities and byte snapshots. Proposed initial
limits are 32 files, 2 MiB per file, 8 MiB selected source bytes in total; exceeding
one is an explicit refusal with the limit shown. Local path selection must remain
within the user-selected import root, reject traversal and symlink escapes, and
read regular files only. Browser upload filenames are labels, never destination
paths. Referenced files are not automatically fetched or included.

Retain selected source bytes with their path labels, byte digests and, when
available, repository/commit/blob identity. Selection ranges identify the original
file and offsets; excluded sections stay visible in the selection boundary.
Durable import provenance and source retention belong in a dedicated committed
sidecar area defined by the implementation contract, outside active spec scans
and default design/build context. `.verdi/data/` may stage disposable previews
but is never the sole durable home of provenance. No transcript or hidden
reasoning is retained. Exact sidecar schema/path is an implementation prerequisite,
not an invented extension to the current spec schema in this document.

Every selected source unit receives a disposition and destination, including
material kept only as notes and intentional omissions. Report the selection
total, mapped/retained/excluded/unresolved counts, and whether anything remains
unaccounted. Byte accounting proves retention, not semantic completeness. A
reviewed paraphrase is still a paraphrase. Users may add requirements, but these
are marked as additions and have no fabricated source passage.

Native mode preserves valid native content and stable IDs without model
extraction. It strict-validates identity, class, anchors, links, evidence kinds
and project model compatibility. The first version accepts draft/proposed native
specs; inputs asserting accepted/closed state, carrying frozen authority, or
requiring unsupported migration are refused with an explanation. They are not
silently stripped or rewritten. External prose may describe prior approval;
that statement is retained as a source claim, never translated into local
acceptance, attestation, waiver or gate evidence.

## Mechanical mapping and source classification

Import requires no model calls, prompt interpretation, inference service, or
AI-context bootstrap. It has three deterministic input cases:

- Native Verdi drafts: strict-decode existing fields and objects.
- Supported external Markdown structures: apply an explicit versioned mapping
  from known headings, keys, list items and source spans into Verdi fields.
- Unrecognized Markdown: retain the selected content and let the user assign
  existing sections/passages to fields/cards. Do not silently classify prose.

The same source bytes, mapping version/options and target inputs produce the
same candidate and provenance bytes. A mapping profile is data interpreted by
fixed importer code, not executable source-supplied code. Recognized structure
must be validated; changed/ambiguous headings or missing selectors cause an
explicit unresolved mapping, never a best-guess fallback. F13's section boundaries
and selected passages provide the first reference mapping, not a claim that
all Markdown plans share that structure. Support documents remain selected
sources; an implementation slice's Goal is not automatically the parent feature's
Outcome. Relationships require declared links or explicit user mapping.

### Explicit statement labels

Problem and Outcome are first-class import fields. When the selected target
contains properly labeled statements in a supported structure, the importer
must populate both fields mechanically from their existing content. Native
Verdi frontmatter uses the current `problem`/`outcome` contract. The Markdown
mapping contract must include explicit Problem and Outcome sections and define
its heading/scope rules, so ordinary labeled specifications do not require
manual re-entry or a per-document hand-authored mapping. No AI is involved.

Recognition is scoped to the selected feature/specification. Do not use a
supporting implementation slice's statement as the parent feature's merely
because it has the same label. Duplicate, empty or conflicting labeled fields
produce a named structural finding instead of choosing the first value. Preserve
multiline text and its source spans; formatting changes must be declared. Label
variants supported by a mapping profile must be explicit and tested rather than
an open-ended synonym or meaning inference.

F13's selected primary definition lacks explicitly labeled Problem and Outcome
sections. This is a structural finding against that source definition, not a
reason for Verdi to reject correctly labeled external specs. Report exactly
which labels are absent. Correcting F13's authoritative source is a separate
source-authoring/review action; this import proposal does not rewrite it. In an
incomplete import preview, the existing explicit deferral option remains
available without calling the source structurally complete.

Preserve wording. Allowed formatting transformations must be named and
reproducible (for example stripping a list marker or joining physical line wraps).
Retain exact source bytes/spans alongside the displayed text. Do not paraphrase,
summarize, manufacture problem/outcome statements, or derive feature completion
from task checkboxes. IDs absent from a source may be assigned deterministically
and must be identified as target-generated, not original source identifiers.
Existing IDs must be preserved or any conflict resolved explicitly.

No evidence kind or expected producer is inferred merely from a test command.
Use explicit source declarations, existing applicable model requirements, or
user selection with its origin recorded. Missing statement fields remain visibly
unresolved; explicit deferral uses existing draft rules and never weakens later
review/acceptance checks. Retained unclassified material stays visible in coverage;
source preservation is not a claim that every requirement was recognized.

Human-operated mechanical import does not need AI-assistance policy merely to
parse files; ordinary model, mutation, identity and lifecycle checks still apply.
If an agent calls an import mutation surface, it retains the existing governed
agent path and cannot declare itself a human to bypass policy. AI may later help
a user author or clarify content through the separate existing design workflow;
it is outside this importer version, and none is run by selection, preview or
apply. The development process's cross-model specification review is separate
from AI participation in the product's import path.

Source documents, including embedded agent instructions and commands, are task
data. Parsing never executes them, follows ambient links, reads secrets or
rewrites project instructions. Source text cannot select destination paths,
lifecycle state or executable authority. A reviewed mapping is bound to exact
bytes, so replay never depends on an external model or its availability.

## Draft creation and failure behavior

Use one shared import application service for workbench and CLI with the current
strict artifact decoding, typed mutation/validation, model vocabulary, Git and
board projection seams. Do not add a parallel spec parser or bypass the common
authority checks. The implementation plan must define the request/result and
provenance schemas before code, including every enum and error category.

A prepared proposal binds the chosen source bytes, reviewed candidate bytes,
source mapping, target identity and base repository revision/model identity.
At apply time revalidate the bound proposal and target; stale revisions or
changed candidate/selection require a fresh preview. Resolve linked objects
against the same validated repository context. A bundle with broken internal
references is a visible error, not partial success. Unsupported structural
features in a project model produce a concrete incompatibility, not data loss.

Creation uses a fresh design branch with create-only collision checks and no
mutation of an existing spec. Both browser and CLI must leave the caller's
checkout/index untouched when creating that branch, and identify how to open
it through the existing worktree workflow. Write candidate, durable source
retention and provenance as one logical Git commit; publish the branch ref only
after validation and object creation succeed. Failure before publication leaves
no visible partial import. An uncertain response after publication must reconcile
the exact recorded import identity and commit before retry; it must not create
a second branch or silently reapply. Reimport/update/merge is deferred: show the
existing target and let the user cancel or explicitly choose a distinct identity.

A story still needs a supported configured tracker reference and existing parent
and acceptance constraints wherever they apply. An import must not turn file
lists, slice names or TODO tracker placeholders into accepted stories. Generated
reports and checkboxes in a source bundle are not fresh evidence. A successful
import proves only that a valid draft and its provenance were created.

## F13 prototype and validation

Reference: `../proposals/2026-09-14-spec-import-f13/`.

The pinned input is ATC revision `c346c005d117dfb090e1dedd5a89f45080875a5e`:
Stage 1 F13 lines 602–671 and the complete review-validation, transition-core
and review-journal slice contracts. Four selected sources / 668 selected lines
are retained byte-for-byte. Primary F13's 70 lines are partitioned into eight
accounting units with explicit transformations. This is not a claim that all
cited F13 governing authority is bundled or that semantic extraction is complete.

The expected preview distinguishes the full F13 feature from its receipt-adapter
prerequisite and narrower implementation slices. It retains review limits,
candidate invalidation, feature AwaitingUAT, G2 routing, blocking-finding
requirements, author adjudication, and the state catalog. File lists and test
commands remain notes. Slice exclusions do not delete parent requirements;
implementation status in a document is not current proof. Problem and outcome
remain unmapped because the selected primary source has no explicit named pair.
The former controller-written summaries are removed from this prototype. A user
may map existing source text or explicitly defer missing statements; neither is
an automatic semantic interpretation. No story decomposition is invented.

Required acceptance witnesses for the eventual implementation:

- F13 sources produce a reviewable mapping and board without retyping their
  declared requirements; every proposed card has truthful origin/classification.
- A positively labeled Markdown fixture imports both Problem and Outcome with
  matching content/source spans and no manual re-entry. Multiline statements
  survive import. Missing, empty, duplicate and cross-target labels have explicit
  results; F13's absent labels are a source-structure finding, not inferred prose.
- Native and supported external import preserve declared content and render it
  through the existing board without any model call; repeated inputs produce
  byte-identical candidate/mapping output. Unsupported structures remain explicit.
- User corrects a proposed mapping, creates the draft, edits through supported
  controls, reloads, and sees matching stored spec and board content.
- Unmapped source, invented source spans, conflicting content, missing required
  fields, broken links and model-incompatible objects cannot silently become a
  completed import. Explicit statement deferral remains an incomplete draft.
  Fake/external approval text cannot confer local acceptance.
- Duplicate/stale imports and failures before/after ref publication preserve
  existing data and support an exact, explained retry outcome.
- Bundle selections cannot escape their bounds or execute embedded instructions;
  import functions with model access absent and sends no source to a model.

Hermetic Go/built-binary tests exercise native/input/application behavior;
Playwright exercises the actual import and correction UI with recording disabled.
Full `make verify` and race gates run on the integrated candidate. This prototype
is not a test result or a replacement for those gates.

## Proposed decisions and delivery boundary

| Decision | Choice and alternative |
|---|---|
| D1 | Mechanical native decoding and explicit mappings for supported Markdown structures, with manual mapping for unknown structures. No inferred conversion. |
| D2 | Persist a canonical spec and source mapping; the existing board projects it. A second card store would introduce competing truth. |
| D3 | One target, explicit Markdown file set/selection. Archive and recursive/bulk migration remain separate. |
| D4 | No AI in the importer. Optional AI authoring remains a separate existing governed workflow and is not an import prerequisite. |
| D5 | Create-only first version. Updating existing specs requires a later explicit merge/provenance contract. |
| D6 | Preserve source snapshots and transformations; source coverage is not evidence or proof of meaning. |

The implementation prerequisites are the exact import/provenance and mapping
profile contracts, the native/external format recognition rules, and the explicit
amendment to the v0 import exclusion. Owner adoption of this detailed design precedes those plans.
The user approved F13 as reference, not an unreviewed change to release acceptance.
If import becomes part of the MVP release, both qualifying adoption journeys must
use the new identified release. The earlier paused/assisted operator session is
feedback, not an independent pass on that future release. Hosted testing remains
deferred. No current installation or user's ATC checkout changes in this step.

Implementation ownership remains Sonnet for backend, FABLE 5.1 for UI, and Opus 5
for assigned review/defect work, through genuine Claude Code and
`/fable-orchestration`. Main authors the authority/prototype and adjudicates the
single independent Opus specification review, with at most one correction and
same-reviewer closure. This document does not dispatch a runtime flight.
