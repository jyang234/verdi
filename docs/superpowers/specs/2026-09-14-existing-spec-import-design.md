# Import existing specifications into Verdi

Status: proposed design. The owner endorsed reviewed import and selected F13
as the reference/prototype on 2026-09-14. This document proposes the detailed
contract; it is not canonical ratification, implemented behavior, or acceptance
of a changed MVP release. The prototype is data only.

## Contents

1. Purpose and authority
2. Import workflow
3. Input and preservation rules
4. AI assistance and review
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
2. **Prepare preview.** Native structured fields are decoded. Ordinary Markdown
   can be mapped manually or with governed external AI assistance. Show proposed
   problem/outcome, objects and relationships beside source passages, using the
   board's existing projection concepts and the project's display vocabulary.
   Preparation does not create a design branch or write a destination spec.
3. **Review coverage and gaps.** Distinguish copied passages, summaries, inferred
   suggestions, user additions, retained supporting material, exclusions and
   unresolved content. Show conflicting requirements and ambiguous ownership.
   The user can edit the proposal or the selection and regenerate the preview.
4. **Create draft.** On explicit confirmation of the current preview, validate
   the complete candidate and commit a fresh design branch. Open its ordinary
   board, showing draft/proposed posture and the exact source/import record.

The UI must offer the choice to use existing material before asking someone to
supply new problem/outcome statements. A preview may be incomplete; unresolved
mandatory fields or source dispositions prevent final creation. Confirming
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

## AI assistance and review

AI assistance uses an explicitly selected external harness through Verdi's
existing governed design/context boundary; Verdi does not embed a model or send
source files to a provider merely because they were selected. Native parsing,
manual mapping and draft creation remain possible without AI. A missing policy,
context capability or permitted harness produces a named refusal for AI
assistance, with manual mapping available; confirming a preview cannot launder
agent authorship into authenticated human authorship or bypass policy.

The existing bootstrap limitation matters: if governed design assistance cannot
compile context for a not-yet-created or unaccepted target, expose that missing
capability. Do not introduce a temporary accepted spec, fake policy, raw model
fallback or owner-vouch shortcut. Whether an import-specific pre-draft context
can reuse the existing contract must be resolved before the AI runtime lane.
The data-only prototype below is controller-authored specification work, not
proof that this runtime bootstrap already works.

Source documents, including embedded agent instructions and commands, are task
data. Extraction cannot execute them, follow ambient project links, read secrets
or rewrite project instructions. Model output is strictly validated data: it
cannot supply paths, lifecycle decisions or executable authority. Preserve
harness/model/session attribution and source classification where supported;
missing identity remains disclosed. No promise of deterministic model prose is
made; a reviewed proposal is frozen to exact bytes and can be replayed without
rerunning the model.

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
implementation status in a document is not current proof. The inferred problem
and outcome must be labeled as summaries, since the selected primary source
contains no verbatim named pair. No implicit story decomposition is invented.

Required acceptance witnesses for the eventual implementation:

- F13 sources produce a reviewable mapping and board without retyping their
  declared requirements; every proposed card has truthful origin/classification.
- Native draft import preserves declared objects and renders them through the
  existing board, without running a model.
- User corrects a proposed mapping, creates the draft, edits through supported
  controls, reloads, and sees matching stored spec and board content.
- Unmapped source, invented source spans, conflicting content, missing required
  fields, broken links and model-incompatible objects cannot silently become a
  completed import. Fake/external approval text cannot confer local acceptance.
- Duplicate/stale imports and failures before/after ref publication preserve
  existing data and support an exact, explained retry outcome.
- Bundle selections cannot escape their bounds or execute embedded instructions;
  unavailable governed AI bootstrap has an explicit honest result.

Hermetic Go/built-binary tests exercise native/input/application behavior;
Playwright exercises the actual import and correction UI with recording disabled.
Full `make verify` and race gates run on the integrated candidate. This prototype
is not a test result or a replacement for those gates.

## Proposed decisions and delivery boundary

| Decision | Choice and alternative |
|---|---|
| D1 | Reviewed conversion plus native import. Native-only is smaller but does not address F13's external plan format. |
| D2 | Persist a canonical spec and source mapping; the existing board projects it. A second card store would introduce competing truth. |
| D3 | One target, explicit Markdown file set/selection. Archive and recursive/bulk migration remain separate. |
| D4 | External governed AI assistance, optional. Automatic ungoverned model calls would break existing design authority. |
| D5 | Create-only first version. Updating existing specs requires a later explicit merge/provenance contract. |
| D6 | Preserve source snapshots and transformations; source coverage is not evidence or proof of meaning. |

The implementation prerequisites are the exact import/provenance contracts, the
pre-draft governed AI-context decision, and the explicit amendment to the v0
import exclusion. Owner adoption of this detailed design precedes those plans.
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
