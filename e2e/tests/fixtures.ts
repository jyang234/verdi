// fixtures.ts — the single constants module for every fixture ref the
// v1 board/dex specs use (authored as tests-v1/fixtures.ts, moved here
// with the V1-P6 flip-in).
//
// Spec files import fixture refs from HERE ONLY; no spec file names a
// fixture ref directly.
//
// ===========================================================================
// SHOWCASE / EDGE zoning (Task 2.2, story showcase-drift-gate)
// ===========================================================================
//
// Every fixture constant below (every flat const that isn't a route/testid
// helper function or a port-derived base URL) is a member of exactly one of
// the two exported, namespaced objects:
//
//   SHOWCASE — a showcased-capability fixture: committed examples/showcase
//     corpus content, the refi-decline-flow design-branch happy-path family
//     (provisionv2.go / provision_board.go's board-v2 fixtures), the
//     payoff-quote-portal live-draft showcase, or any other directory/dex/
//     board HAPPY-PATH demonstration of a real capability — what a README
//     reader or a dogfooding tour would be shown.
//   EDGE — a degenerate, mid-lifecycle, refusal, or stress fixture that
//     exists ONLY to exercise an edge/negative/error path: decline-badge-*,
//     decline-ac-sprawl/trim, decline-sweep-*, size-smell, editor error
//     cases, a misaimed-drop refusal, or a mid-session branch deletion.
//
// BINDING (ledger L-B, docs/design/plans/2026-07-14-public-rollout-plan.md):
// the literal substring "SHOWCASE." appearing in a spec file's TEXT is the
// showcase-coverage gate's marker (internal/showcasealign's coverage test,
// Task 3.2, greps spec files for the regexp /SHOWCASE\./). Consequently:
//
//   NEVER ALIAS SHOWCASE. Writing `const S = SHOWCASE;` (or renaming it via
//   `import { SHOWCASE as S }`) and then accessing `S.DESIGN_SPEC` from a
//   spec file defeats the gate — the coverage test never sees the literal
//   text "SHOWCASE." in that file, so a real capability demo silently stops
//   counting as coverage evidence. Every access MUST be spelled out in full,
//   `SHOWCASE.<NAME>`, at the call site. (EDGE carries no such rule — it is
//   never coverage evidence, so it may be destructured, aliased, or spread
//   freely.)
//
// Classification table (every constant, grouped by fixture family):
//
// | Members                                                          | Zone     | Why |
// |-------------------------------------------------------------------|----------|-----|
// | DESIGN_SPEC, DESIGN_BRANCH, MAIN_BRANCH, AC_IDS, CONSTRAINT_ID,    | SHOWCASE | refi-decline-flow design-branch happy path (rule-explicit); MAIN_BRANCH is the git-affordance branch-switch guard's target, exercised on this same wall |
// |   DECISION_WITH_EXEMPTS, DECISION_PLAIN, PROBLEM_SNIPPET,          |          | |
// |   OUTCOME_SNIPPET, ADR_NAME, ADR_REF                               |          | |
// | REVIEW_SPEC, REVIEW_COMMENT_ANCHORED, REVIEW_COMMENT_TOKEN_FREE,   | SHOWCASE | board review-mode happy path: a real MR mirror exercising the full comment-routing grammar, not a stress rig |
// |   REVIEW_COMMENT_UNRESOLVABLE, REVIEW_FEED_TOTAL                   |          | |
// | FEATURE_SPEC, STORY_STUB_MATCHED                                  | SHOWCASE | committed corpus (escrow-autopay, borrower-update-api) |
// | STORY_DEVIATING, STORY_WITH_SPEC_STALE,                            | SHOWCASE | the corpus's OWN genuine deviation scar (stale-decline breadth feature) — a deliberately showcased ladder-badge demo, not a synthetic rig |
// |   STORY_WITH_PENDING_SUPERSESSION                                  |          | |
// | ARCHIVED_STORY_ROUND4, ARCHIVED_STORY_GRANDFATHERED                | SHOWCASE | committed archived quartets (dex by-story happy path) |
// | READONLY_SPEC                                                      | SHOWCASE | stale-decline, the breadth feature — sealed-record happy path |
// | NO_CASEFILE_SPEC                                                   | SHOWCASE | committed corpus component (store-layout-notes): correct handling of a real, valid caseless artifact — not a stress rig |
// | EMPTY_SPEC, EMPTY_SPEC_STORY_REF                                   | SHOWCASE | the newcomer's-first-board teaching state — a genuine onboarding happy path |
// | OBLIGATION_STORY_SPEC, OBLIGATION_STORY_AC                        | SHOWCASE | sticky-graduates-to-obligation happy path |
// | OBLIGATION_STORY_NON_AC                                            | EDGE     | the misaimed-drop refusal case, on the same wall |
// | OBLIGATION_WALL_SPEC, OBLIGATION_WALL_AC,                          | SHOWCASE | disclosed-obligation / disclosed-"no obligation" happy path — three-valued honesty on display, not an error state |
// |   OBLIGATION_WALL_PRESENT_KIND, OBLIGATION_WALL_MISSING_KIND,      |          | |
// |   OBLIGATION_WALL_DEMAND                                           |          | |
// | SLOT_WALL_SPEC, SLOT_WALL_AC, SLOT_HELD_KIND, SLOT_EMPTY_KIND,     | SHOWCASE | evidence-slot happy path: held/empty/attested read together on one card |
// |   SLOT_ATTESTED_KIND                                               |          | |
// | BADGE_WALL_SPEC, BADGE_REVIEW_SPEC, BADGE_SEALED_SPEC,             | EDGE     | decline-badge-* — provisioned violation-chip rigs (rule-explicit) |
// |   BADGE_DECISION, BADGE_STUB_SLUG                                  |          | |
// | SIZE_SMELL_SPEC, SIZE_FIT_SPEC, SIZE_SMELL_ESTIMATE,               | EDGE     | size-smell (rule-explicit) |
// |   SIZE_SMELL_REFERENCE                                             |          | |
// | SWEEP_FRESH_SPEC, SWEEP_STALE_SPEC, SWEEP_PARTIAL_SPEC,            | EDGE     | decline-sweep-* (rule-explicit) |
// |   SWEEP_MISSING_DECISION                                           |          | |
// | PIN_ADR, PIN_DIAGRAM, PIN_TRASH_ADR                                | SHOWCASE | real corpus artifacts; pin/peek/drag/trash happy-path journeys (the trash's "pure pin" tier, not a stress case) |
// | MERMAID_SPEC, MERMAID_SPEC_REF, ILLUSTRATIVE_DIAGRAM,              | SHOWCASE | diagram-tier happy path: illustrative vs. full, correctly and distinctly rendered |
// |   PROPOSAL_DIAGRAM, ILLUSTRATIVE_FIGURE, ILLUSTRATIVE_CHIP,        |          | |
// |   PROPOSAL_FULL_FIGURE                                             |          | |
// | DOC_EDGE_TYPE, DOC_EDGE_TARGET                                     | SHOWCASE | READONLY_SPEC's document-level edge (real corpus ADR) |
// | OQ_ID, STUB_SLUGS, INSTANTIATE_SLUG                                | SHOWCASE | scoping-canvas happy path on real/committed stub content |
// | SUPERSEDED_FEATURE_SPEC, SUPERSEDED_STORY_SPEC                    | SHOWCASE | committed supersession chains (rate-lock, escrow-notify) |
// | DIR_LOCAL_DRAFT, DIR_REMOTE_DRAFT, DIR_INREVIEW_SPEC               | SHOWCASE | directory-home happy path: grouped listing, source disclosure, in-review chip |
// | DIR_EMPTY_BRANCH                                                   | EDGE     | degenerate branch: no draft spec at all |
// | DIR_DOOMED_DRAFT                                                   | EDGE     | mid-session branch deletion (stress/race path) |
// | DIR_CLOSED_AWAITING_ARCHIVE                                        | EDGE     | home-status-glance mid-lifecycle shape: closed, still in specs/active/ |
// | DIAGRAM_PROPOSAL, DIAGRAM_PROPOSAL_BODY, DIAGRAM_BASE_BODY,        | SHOWCASE | diagram-editor happy path: drafting, structural ops, byte preservation, verification rail, peek/reset |
// |   DIAGRAM_DERIVED, DIAGRAM_DERIVED_BODY, DIAGRAM_RAIL_TIER,        |          | |
// |   DIAGRAM_RAIL_FINDINGS                                            |          | |
// | DIAGRAM_OUTSIDE_OPS                                                | EDGE     | editor error case: disclosed-unavailable, outside the op grammar |
// | DIAGRAM_DERIVED_CORRUPT                                            | EDGE     | editor error case: corrupt digest, must fail visibly |
// | DB_TAB_A, DB_TAB_B, DB_SEALED_REMOTE, DB_SAME_SPEC,                | SHOWCASE | draft-boards happy path: per-branch boards, two-tab isolation, same-spec-two-modes |
// |   DB_SAME_SPEC_BRANCH, DB_SAME_SPEC_DRAFT_SNIPPET                  |          | |
// | SHOWCASE_DRAFT_SPEC, SHOWCASE_DRAFT_BRANCH,                        | SHOWCASE | payoff-quote-portal, the canonical live-draft showcase (rule-explicit) |
// |   SHOWCASE_DRAFT_PROBLEM_SNIPPET, SHOWCASE_DRAFT_OUTCOME_SNIPPET,  |          | |
// |   SHOWCASE_DRAFT_ACS, SHOWCASE_DRAFT_OQ_ID,                        |          | |
// |   SHOWCASE_DRAFT_OQ_CARRIED, SHOWCASE_DRAFT_OQ_RESOLVED,           |          | |
// |   SHOWCASE_DRAFT_DIAGRAM                                          |          | |
// | FORGE_KIND                                                         | SHOWCASE | examples/showcase's own committed verdi.yaml forge: value (disclosures happy path) |
//
// Top-level (never zoned — not fixtures, per this task's brief): boardPath
// and every other route/data-testid helper function, plus the port-derived
// bases DEX_BASE, CONTROL_URL, INSPECT_URL.
//
// NOTE (rudimentary provisioner prose — task-2.2-report.md flagged it,
// Task 3.4 upgraded three of the six once they became showcase-coverage
// evidence): refi-decline-replay, decline-slot-wall, and
// stale-decline-notices (cmd/e2eharness/provision_board.go's replaySpec/
// slotWallSpec/reviewSpec) were brought to the payoff-quote-portal prose
// bar (real `## Outcome`/`## ac-N` body prose, canon-consistent) when they
// became coverage evidence for wb:obligation-wall/wb:evidence-slot/
// wb:wall-receipts/wb:board-review-mode (showcase-coverage Task 3.4).
// stale-decline-notices' `## Problem` body stays DELIBERATELY empty —
// 33-board-expand.spec.ts's "affordance works in a non-authoring (review)
// room too" test asserts exactly that absence (the no-body →
// headline-fallback path), so upgrading it would break a passing test.
// income-verification, refi-decline-audit, and the draft-boards family
// (cmd/e2eharness/provision_board.go / provision_draftboards.go) remain
// rudimentary — still flagged, not fixed, since none is this task's
// showcase-coverage evidence (EMPTY_SPEC/OBLIGATION_STORY_SPEC/DB_* back no
// wb: gap Task 3.2 named). Follow-up: bring them to the same bar if/when
// they become showcase-coverage evidence themselves.
//
// ---------------------------------------------------------------------------
//
// BINDING NOTE (V1-P6, amended V1-P8): the workbench constants below are
// FINAL — they are provisioned verbatim by cmd/e2eharness/provisionv2.go
// (a draft spec on a design branch cannot live in the committed
// examples/showcase tree, VL-004, so the harness authors the board fixtures
// onto a scratch design branch at startup; see tests-v1/README.md
// "Harness obligations"). The dex constants were finalized by V1-P8 to
// the v2 fixture overlay's real refs (examples/showcase +
// testdata/dexoverlay). One V1-P6 constant moved with them: ADR_NAME —
// shared by the board's ref-card tests and the dex exemption-page test —
// now names the ADR the v2 fixture feature's dc-1 actually exempts
// (adr/0001-outbox-events, real on main), and provisionv2.go's design
// spec exempts the same one; recorded as a V1-P8 ledger deviation.

import { resolvePorts } from "../ports";

// ---------------------------------------------------------------------------
// Internal cross-references only (never exported): a few fixtures below are
// literal derivations of another fixture IN THE SAME ZONE (e.g. a branch name
// built from a spec name, or one fixture aliasing another's ref). An object
// literal can't reference its own export mid-construction, so the shared
// literal lives here once and every member that needs it reads from here.
// ---------------------------------------------------------------------------
const designSpecName = "refi-decline-flow";
const adrName = "0001-outbox-events";
const mermaidSpecName = "mermaid-demo";
const pinDiagramRef = "diagram/loansvc-topology";
const diagramBaseBody = `graph TD
  loansvc --> notification-svc
  loansvc --> charge-svc
`;
const showcaseDraftSpecName = "payoff-quote-portal";

// ===========================================================================
// SHOWCASE — showcased-capability fixtures
// ===========================================================================
export const SHOWCASE = {
  // -------------------------------------------------------------------------
  // Workbench (V1-P6 board v2) — authoring mode
  // -------------------------------------------------------------------------

  // Draft spec on a design branch → the board opens in AUTHORING mode
  // (05 §Workbench, "Two modes, keyed by branch state").
  // Binds when V1-P1's fixture-v2 overlay merges.
  DESIGN_SPEC: designSpecName,

  // The design branch the harness provisions DESIGN_SPEC on, and the default
  // branch the branch-switch guard test tries to switch to.
  // Binds when V1-P1's fixture-v2 overlay merges (harness branch naming).
  DESIGN_BRANCH: `design/${designSpecName}`,
  // The default branch the git-affordance branch-switch guard tries to
  // switch to, on the same DESIGN_SPEC wall.
  MAIN_BRANCH: "main",

  // DESIGN_SPEC's declared object model (02 §Object model id conventions:
  // ac-N acceptance criteria, co-N constraints, dc-N decisions, oq-N open
  // questions). Binds when V1-P1's fixture-v2 overlay merges.
  AC_IDS: ["ac-1", "ac-2", "ac-3"] as const,
  CONSTRAINT_ID: "co-1",
  // dc-1 carries a declared `links: [{type: exempts, ref: <ADR_REF>}]` edge
  // (the PLAN-V1 §4 "one decision carrying an exempts edge against an ADR").
  DECISION_WITH_EXEMPTS: "dc-1",
  // dc-2 is the plain decision (no links) — the picker tests draw fresh yarn
  // from it. Binds when V1-P1's fixture-v2 overlay merges.
  DECISION_PLAIN: "dc-2",

  // Substrings of DESIGN_SPEC's two required attributes (02 §Object model:
  // `problem` / `outcome`, each { text, anchor }). The specs assert the
  // placards CONTAIN these snippets. Bind when V1-P1's fixture-v2 overlay
  // merges (the attribute texts the overlay authors).
  PROBLEM_SNIPPET: "stale decline",
  OUTCOME_SNIPPET: "declined applicants",

  // The ADR that DECISION_WITH_EXEMPTS exempts — and the ADR whose dex
  // exemption page 16-dex-v2 asserts (both the board fixture's dc-1 and
  // the v2 corpus feature's dc-1 exempt this one real corpus ADR). Bound
  // by V1-P8's fixture finalization.
  ADR_NAME: adrName,
  ADR_REF: `adr/${adrName}`,

  // -------------------------------------------------------------------------
  // Workbench (V1-P6 board v2) — review mode
  // -------------------------------------------------------------------------

  // Spec under MR review → the board opens in REVIEW mode as a mirror of the
  // MR (05 §Workbench "Review" bullet; §Review stickies and forge
  // round-trip). The harness provisions its comment feed through
  // internal/forge's fake adapter (PLAN-V1 §5 V1-P6 "Stubs").
  // Binds when V1-P1's fixture-v2 overlay merges.
  REVIEW_SPEC: "stale-decline-notices",

  // The canned MR comment feed the harness serves for REVIEW_SPEC — three
  // comments exercising every routing case of 02 §Record schemas'
  // comment-token grammar. Bodies bind when V1-P1's fixture-v2 overlay
  // merges (built from S6's committed captures, PLAN-V1 §4).
  REVIEW_COMMENT_ANCHORED: {
    // Resolvable `[vd:<object-id>]` token → renders anchored to this card.
    objectId: "ac-2",
    body: "[vd:ac-2] this outcome AC reads as implementation-scoped — reword?",
  },
  REVIEW_COMMENT_TOKEN_FREE: {
    // No token at all → inbox tray, never dropped.
    body: "overall direction looks right; one naming nit inline",
  },
  REVIEW_COMMENT_UNRESOLVABLE: {
    // Token present but resolving to no declared object → inbox tray too
    // (02 §Record schemas: "a comment whose token does not resolve, or that
    // carries no token, renders in an unanchored inbox tray — never dropped").
    body: "[vd:zz-99] does this still apply after the split?",
  },
  REVIEW_FEED_TOTAL: 3,

  // -------------------------------------------------------------------------
  // Dex (V1-P8) — served statically on :4174 by default (VERDI_E2E_PORT_BASE,
  // D6-28, shifts it — see ../ports.ts) by cmd/e2eharness
  // -------------------------------------------------------------------------

  // The v2 fixture feature spec (three outcome ACs, three stubs, dc-1
  // exempting ADR_NAME — PLAN-V1 §4's overlay, examples/showcase). Finalized
  // by V1-P8.
  FEATURE_SPEC: "escrow-autopay",

  // The v2 fixture's two story specs (PLAN-V1 §4: one stub-matched, one
  // deviating). STORY_STUB_MATCHED doubles as the realized stub's slug
  // (R4-I-12: RefSlug(title) equals the stub's slug). Finalized by V1-P8.
  STORY_STUB_MATCHED: "borrower-update-api",
  STORY_DEVIATING: "borrower-update-mobile",

  // Fixture stories carrying the ladder flags V1-P8's badges render
  // (03 §The amendment ladder). Both resolve to the deviating story — the
  // constants stay separate so the specs stay honest about which flag they
  // assert: spec-stale comes from testdata/dexoverlay's living deviation
  // report (accepted-deviation on the story's own ac-1, R4-I-18);
  // pending-supersession from the fake forge's open MR whose candidate
  // manifest amends escrow-autopay's ac-2 (which this story's
  // implements edges touch and STORY_STUB_MATCHED's do not).
  STORY_WITH_SPEC_STALE: "borrower-update-mobile",
  STORY_WITH_PENDING_SUPERSESSION: "borrower-update-mobile",

  // The by-story axis's two archived quartets (05 §Verdi-dex IA): the
  // round-four form archives layout.json in the board slot
  // (testdata/dexoverlay); the grandfathered v0 form keeps its frozen
  // board.json (examples/showcase).
  ARCHIVED_STORY_ROUND4: "refi-rate-check-2024",
  ARCHIVED_STORY_GRANDFATHERED: "loan-refi-2023",

  // -------------------------------------------------------------------------
  // Workbench (board polish) — read-only mode
  // -------------------------------------------------------------------------

  // A spec that is NOT a draft on a design branch (it lives on main in the
  // committed corpus), so its board renders READ-ONLY (05 §Workbench, "Two
  // modes, keyed by branch state") — the fixture for the drag-refusal
  // contract (a read-only board is never silently inert).
  READONLY_SPEC: "stale-decline",

  // A committed spec (examples/showcase, class: component) that carries
  // NEITHER problem nor outcome — so its wall renders no case-file lockup
  // at all, and the class stamp has nowhere to hang (boardspecrender.go:
  // writeCaseTopline runs only inside the hasCaseFile header). The fixture
  // for the never-an-orphaned-stamp contract, now that READONLY_SPEC's
  // renovation gave it a full case file.
  NO_CASEFILE_SPEC: "store-layout-notes",

  // A draft spec on the design branch with the two required attributes and
  // NO declared objects — the newcomer's first board. Its board opens in
  // authoring mode and must render the teaching empty-wall state, never a
  // void (provisioned by cmd/e2eharness/provisionv2.go).
  EMPTY_SPEC: "income-verification",

  // EMPTY_SPEC is class: story — the harness's one story-class board
  // fixture (every other board fixture is a feature) — and this is its
  // `story:` tracker ref, which the case-file class tag wears as
  // "story · <tracker-ref>" (provisioned by cmd/e2eharness/provisionv2.go).
  EMPTY_SPEC_STORY_REF: "jira:LOAN-2201",

  // -------------------------------------------------------------------------
  // Workbench (obligation authoring, spec/obligation-artifact ac-3)
  // -------------------------------------------------------------------------

  // A STORY-class draft on the design branch that DECLARES acceptance criteria
  // — the wall on which a sticky graduates into an evidence obligation (a
  // sticky's yarn dropped on a story AC). Distinct from EMPTY_SPEC, which is
  // deliberately object-less; this one carries the AC targets (the non-AC
  // decision card OBLIGATION_STORY_NON_AC's invalid-drop refusal is an EDGE
  // fixture — see below). Provisioned by cmd/e2eharness/provisionv2.go.
  OBLIGATION_STORY_SPEC: "refi-decline-audit",
  OBLIGATION_STORY_AC: "ac-1",

  // -------------------------------------------------------------------------
  // Workbench (obligation wall, spec/obligation-wall ac-2)
  // -------------------------------------------------------------------------

  // A STORY-class draft on the design branch whose ac-1 declares TWO evidence
  // kinds (behavioral, static) and carries a COMMITTED obligation for the
  // behavioral one only — so its board AC card reads out both halves of ac-2 on
  // first load: the authored obligation's title (behavioral) and the disclosed
  // "no obligation" badge (static). Distinct from OBLIGATION_STORY_SPEC, whose
  // obligation the graduate journey authors at runtime; this one is pre-authored
  // so the card renders it without any interaction. Provisioned by
  // cmd/e2eharness/provisionv2.go.
  OBLIGATION_WALL_SPEC: "refi-decline-replay",
  OBLIGATION_WALL_AC: "ac-1",
  OBLIGATION_WALL_PRESENT_KIND: "behavioral",
  OBLIGATION_WALL_MISSING_KIND: "static",
  // A substring of the committed obligation's title — the specific demand the
  // card reads out on the wall (feature co-3, legible-without-the-sidecar).
  OBLIGATION_WALL_DEMAND: "drives the replay view",

  // -------------------------------------------------------------------------
  // Workbench (evidence slot, spec/evidence-slot)
  // -------------------------------------------------------------------------

  // A STORY-class draft on the design branch whose ac-1 declares THREE
  // evidence kinds, with REAL fold-visible state for two of them: a
  // derived-tree CI static record at main's sha (fills the static slot), an
  // attestation file at the fold's own path (fills the attestation slot),
  // and nothing for behavioral (the empty slot that badges). Provisioned by
  // cmd/e2eharness/provision_board.go (slotWallSpec / slotWallAttestation /
  // writeSlotWallDerived). The no-derived-tree CALM state (evidence-slot
  // dc-1) is proven on OBLIGATION_WALL_SPEC, which has no derived tree.
  SLOT_WALL_SPEC: "decline-slot-wall",
  SLOT_WALL_AC: "ac-1",
  SLOT_HELD_KIND: "static",
  SLOT_EMPTY_KIND: "behavioral",
  SLOT_ATTESTED_KIND: "attestation",

  // -------------------------------------------------------------------------
  // Workbench (import/pin toolbox, board-polish)
  // -------------------------------------------------------------------------

  // Corpus artifacts nothing on DESIGN_SPEC's wall names (real on main,
  // so real on the design branch) — the pin toolbox's import fixtures, and
  // (PIN_TRASH_ADR) the trash's "pure pin" removal-tier fixture.
  PIN_ADR: "adr/0002-outbox-events",
  PIN_DIAGRAM: pinDiagramRef,
  PIN_TRASH_ADR: "adr/0003-retry-policy",

  // -------------------------------------------------------------------------
  // Diagram tiers (spec/illustrative-class) — 39-diagram-tier
  // -------------------------------------------------------------------------

  // The spec whose markdown body carries a fenced ```mermaid block
  // (cmd/e2eharness/provision.go's mermaidDemoSpec — scratch-store only):
  // the body-figure fixture, illustrative BY LOCATION (dc-2).
  MERMAID_SPEC: mermaidSpecName,
  MERMAID_SPEC_REF: `spec/${mermaidSpecName}`,

  // The incumbent diagram-kind artifact (examples/showcase, no class:
  // discriminator): illustrative BY CLASS (dc-2). Same artifact PIN_DIAGRAM
  // names — aliased so the tier tests read in tier vocabulary.
  ILLUSTRATIVE_DIAGRAM: pinDiagramRef,

  // The class: proposal diagram (provision.go's proposalDiagram — scratch
  // store only): its surfaces carry the extractor-computed tier
  // (data-diagram-tier="full"; its body sits inside the declared grammar)
  // and must NEVER wear the illustrative badge (ac-2's negative case).
  PROPOSAL_DIAGRAM: "diagram/decline-flow-future",

  // The dc-1 badge grammar, verbatim: the machine-readable tier marker
  // (a selector over the figure wrapper) and the visible figcaption chip.
  ILLUSTRATIVE_FIGURE: 'figure[data-diagram-tier="illustrative"]',
  ILLUSTRATIVE_CHIP: "illustrative · not deterministically verifiable",
  PROPOSAL_FULL_FIGURE: 'figure[data-diagram-tier="full"]',

  // READONLY_SPEC's one closed-vocabulary DOCUMENT-LEVEL edge (02 §Object
  // model: frontmatter `links:` declared on the spec document itself, so
  // the projection emits it with From:"spec"). The document is not a card —
  // it hangs above the canvas as the placards header — so this edge's yarn
  // must tie to its one on-board endpoint (the target's reference card)
  // with a thread pointing off the board's top edge.
  DOC_EDGE_TYPE: "implements",
  DOC_EDGE_TARGET: "adr/0002-outbox-events",

  // -------------------------------------------------------------------------
  // Workbench (scoping canvas, spec/scoping-canvas) — the stubs band
  // -------------------------------------------------------------------------

  // DESIGN_SPEC's one open question (provisioned by provisionv2.go): the
  // spike proto-sticky's resolution-yarn target.
  OQ_ID: "oq-1",

  // FEATURE_SPEC (escrow-autopay, on main → sealed wall) declares
  // two stubs (public-rollout-plan Task 1.5 renamed them from the former
  // borrower-update-* trio once those stories were rewired onto
  // spec/stale-decline instead); its wall renders them as stub cards with
  // Instantiate. STUB_SLUGS mirrors the fixture's stubs: frontmatter
  // verbatim.
  STUB_SLUGS: ["autopay-mandate-api", "autopay-retry-policy"] as const,

  // The stub the instantiate journey cuts a branch for: it must have NO
  // realized story spec in the corpus — neither stub is realized any more
  // (Task 1.5: escrow-autopay's own implementing stories moved to
  // spec/stale-decline; the one residual borrower-update-mobile edge into
  // this feature only touches ac-2, whose AC-set does not equal either
  // stub's declared set), so design/<slug> carries a genuinely new
  // scaffold either way.
  INSTANTIATE_SLUG: "autopay-retry-policy",

  // The live corpus's other committed stub fixture: disclosure-legibility
  // (in this repo's own .verdi store) — asserted only through the Go
  // render tests; the e2e store's stub fixture is FEATURE_SPEC above.

  // -------------------------------------------------------------------------
  // Supersession terminal state (spec/feature-supersession-state ac-2)
  // -------------------------------------------------------------------------

  // A superseded FEATURE predecessor (rung 4) and a superseded STORY
  // predecessor (rung 3), committed on main via testdata/dexoverlay
  // (provision.go) — so their boards render READ-ONLY. Each is the superseded
  // predecessor of a v2 successor that carries the `supersedes` edge, and each
  // wears the terminal `superseded` status badge on its board head (and, on
  // dex, the same `.badge-superseded` status badge). The Go build/render tests
  // prove the same committed fixtures; these constants drive the Playwright
  // proof of the board surface.
  SUPERSEDED_FEATURE_SPEC: "rate-lock",
  SUPERSEDED_STORY_SPEC: "escrow-notify",

  // -------------------------------------------------------------------------
  // Directory home (spec/directory-home) — the whole-store directory at GET /
  // -------------------------------------------------------------------------

  // Directory fixture branches (cmd/e2eharness/provision_directory.go):
  // each name is both the design branch's slug (design/<name>) and the
  // draft spec's name.
  DIR_LOCAL_DRAFT: "audit-trail", // local branch only
  DIR_REMOTE_DRAFT: "vendor-onboarding", // remote-tracking only

  // The entry the control server's open-MR feed chips "in review": the board
  // suite's design branch (DESIGN_SPEC), which exists locally AND pushed.
  DIR_INREVIEW_SPEC: designSpecName,

  // -------------------------------------------------------------------------
  // Diagram editor (spec/board-editor) — the drafting/structural-ops/rail/
  // peek-reset happy paths (37-board-diagram-editor)
  // -------------------------------------------------------------------------

  // A from-scratch proposal WITHIN the op grammar's flowchart subset — the
  // structural-ops, save, and rail (canned report) journeys.
  DIAGRAM_PROPOSAL: "editor-proposal",
  DIAGRAM_PROPOSAL_BODY: `flowchart TD
  loansvc["Loan service"]
  billing["Billing"]
  %% drafted on the wall
  loansvc --> billing
`,

  // The pinned base and its two derived twins (ac-4): the good one's
  // derived_from.digest is computed with the REAL diagrambase formula at
  // provision time; the corrupt one (EDGE, below) pins sha256:000…0 and
  // must fail visible with no write.
  DIAGRAM_BASE_BODY: diagramBaseBody,
  DIAGRAM_DERIVED: "editor-derived",
  DIAGRAM_DERIVED_BODY: diagramBaseBody + `  loansvc --> audit-svc
`,

  // The canned verification report's rendered claims (provision_diagram.go's
  // cannedDiagramVerification, rendered VERBATIM by the rail — ac-5).
  DIAGRAM_RAIL_TIER: "partial",
  DIAGRAM_RAIL_FINDINGS: [
    ["loansvc", "exists"],
    ["billing", "proposed-new"],
    ["audit-log", "contradicted"],
    ["charge-svc", "stale-base"],
  ] as ReadonlyArray<[string, string]>,

  // -------------------------------------------------------------------------
  // Draft boards (spec/draft-boards) — the /b/<branch-escaped>/ routes
  // -------------------------------------------------------------------------

  // Draft-board fixture branches (cmd/e2eharness/provision_draftboards.go):
  // each name is both the design branch's slug (design/<name>) and its
  // draft spec's name.
  DB_TAB_A: "draft-tab-a", // local; tab A of the two-tab proof
  DB_TAB_B: "draft-tab-b", // local; tab B
  DB_SEALED_REMOTE: "sealed-remote", // remote-tracking ref ONLY (dc-4)
  // The same-spec-two-modes fixture (ac-3): DB_SAME_SPEC is landed
  // (accepted-pending-build) on main by cmd/e2eharness/provision.go AND
  // exists as a DRAFT edition on DB_SAME_SPEC_BRANCH — sealed at its
  // unprefixed address, authoring under /b/, simultaneously.
  DB_SAME_SPEC: "decline-ledger",
  DB_SAME_SPEC_BRANCH: "design/decline-ledger-v2",
  // A substring of the draft edition's outcome text — present under /b/,
  // absent from the landed record's unprefixed render.
  DB_SAME_SPEC_DRAFT_SNIPPET: "draft edition outcome (draft boards e2e)",

  // -------------------------------------------------------------------------
  // Showcase live-draft feature (cmd/e2eharness/provision_showcase_draft.go)
  // -------------------------------------------------------------------------

  // The canonical "one live draft on a design branch" lifecycle stage (public
  // rollout design §4.3): the payoff-quote-portal feature is authored on
  // design/payoff-quote-portal (jira:LOAN-1533) and never committed to main
  // (VL-004). The harness pre-cuts and seeds its managed worktree, so its
  // authoring board renders under /b/ with its object model AND its
  // open-question stickies. BINDING: every name/text below mirrors
  // provision_showcase_draft.go verbatim — change them together.
  SHOWCASE_DRAFT_SPEC: showcaseDraftSpecName,
  SHOWCASE_DRAFT_BRANCH: `design/${showcaseDraftSpecName}`,

  // Substrings of the draft's problem/outcome placards.
  SHOWCASE_DRAFT_PROBLEM_SNIPPET: "payoff quote",
  SHOWCASE_DRAFT_OUTCOME_SNIPPET: "good through a stated date",

  // The draft's two declared acceptance criteria (each declares evidence
  // kinds), rendered as object cards on the wall.
  SHOWCASE_DRAFT_ACS: ["ac-1", "ac-2"] as const,

  // The declared open question (rendered as an oq card) — VL-017's "carried"
  // path: the same text a still-open question sticky carries, formalized as a
  // real open_questions object on the spec.
  SHOWCASE_DRAFT_OQ_ID: "oq-1",
  SHOWCASE_DRAFT_OQ_CARRIED:
    "does a payoff quote's good-through date have to honor a rate lock that expires inside the quote window?",

  // VL-017's "resolved" path: a question sticky settled in place (status
  // resolved) rather than carried onto the spec.
  SHOWCASE_DRAFT_OQ_RESOLVED:
    "should the payoff quote require identity re-verification before it is shown?",

  // The proposal-tier diagram authored on the branch (VL-021: derived_from a
  // real corpus diagram + a well-formed sha256 digest).
  SHOWCASE_DRAFT_DIAGRAM: "payoff-quote-flow",

  // -------------------------------------------------------------------------
  // Disclosures (spec/disclosures-panel) — the checkout-wide seam
  // -------------------------------------------------------------------------

  // examples/showcase/.verdi/verdi.yaml's own committed `forge:` value — no
  // credentials are ever exported for it in this hermetic harness, so it is
  // the real, committed source of the workbench's one seeded
  // review-feed-unavailable disclosure (19-disclosures.spec.ts). A fact
  // about the corpus itself, not a process artifact.
  FORGE_KIND: "gitlab",
} as const;

// ===========================================================================
// EDGE — degenerate / mid-lifecycle / stress fixtures
// ===========================================================================
export const EDGE = {
  // -------------------------------------------------------------------------
  // Workbench (obligation authoring, spec/obligation-artifact ac-3) — the
  // misaimed-drop refusal
  // -------------------------------------------------------------------------

  // The non-AC card on OBLIGATION_STORY_SPEC's wall (SHOWCASE) that the
  // invalid-drop refusal lands on: a drop on anything that is not an AC is
  // refused legibly, and nothing is written.
  OBLIGATION_STORY_NON_AC: "dc-1",

  // -------------------------------------------------------------------------
  // Workbench (wall badges, spec/badge-computes ac-5)
  // -------------------------------------------------------------------------

  // Three walls carrying the SAME real badge-triggering state (a stub whose
  // acceptance_criteria names an undeclared AC → VL-006 chip on the stub
  // card; a decision exempting a nonexistent ADR → VL-003 chip on the
  // decision card; a top-level depends-on to a nonexistent spec → VL-003
  // stamp on the case file), one per board mode: a draft on the design
  // branch (authoring), a draft with an entry in the canned MR feed
  // (review), and an accepted-pending-build instance (read-only — branch
  // state cannot make a non-draft wall authoring). Provisioned by
  // cmd/e2eharness/provision_board.go (badgeSpec).
  BADGE_WALL_SPEC: "decline-badge-wall",
  BADGE_REVIEW_SPEC: "decline-badge-review",
  BADGE_SEALED_SPEC: "decline-badge-sealed",
  // The badge anchors: the decision card wearing the VL-003 chip and the
  // stub card wearing the VL-006 chip.
  BADGE_DECISION: "dc-1",
  BADGE_STUB_SLUG: "badge-orphan",

  // -------------------------------------------------------------------------
  // Workbench (size-smell, spec/case-file-flags ac-2/ac-3)
  // -------------------------------------------------------------------------

  // The size-smell fixture pair: two authoring walls differing ONLY in
  // declared AC count, straddling dc-1's deterministic proxy (estimated
  // AC-column height = ZoneOriginY 40 + count × RowPitch 176, vs. the
  // declared reference-viewport-height constant 900 — never a client
  // measurement). SIZE_SMELL_SPEC declares 5 ACs (estimate 920 → badge);
  // SIZE_FIT_SPEC declares 4 (estimate 744 → no badge). Provisioned by
  // cmd/e2eharness/provision_board.go (acCountSpec).
  SIZE_SMELL_SPEC: "decline-ac-sprawl",
  SIZE_FIT_SPEC: "decline-ac-trim",
  // The badged wall's estimate operands, mirrored for the drawer-content
  // assertions (constants disclosed by name and value, dc-1).
  SIZE_SMELL_ESTIMATE: 920,
  SIZE_SMELL_REFERENCE: 900,

  // -------------------------------------------------------------------------
  // Workbench (derivation drawer, spec/derivation-drawer ac-3)
  // -------------------------------------------------------------------------

  // Three walls each carrying a REAL committed decision-conflict-report.md
  // (provisioned by cmd/e2eharness/provision_board.go, sweepSpec/sweepReport):
  // fresh (covers pins the commit whose spec bytes the wall still renders,
  // decisions_scanned complete → no mismatch line), stale (the spec was
  // rewritten after the covered commit → the drawer discloses the covers
  // contrast), and partial (declared dc-2 absent from decisions_scanned →
  // the drawer names it). Every report carries one dispositioned judged
  // finding (no-conflict + note) and one explicitly undispositioned one.
  SWEEP_FRESH_SPEC: "decline-sweep-fresh",
  SWEEP_STALE_SPEC: "decline-sweep-stale",
  SWEEP_PARTIAL_SPEC: "decline-sweep-partial",
  // The declared decision id the partial fixture's sweep misses.
  SWEEP_MISSING_DECISION: "dc-2",

  // -------------------------------------------------------------------------
  // Directory home (spec/directory-home) — degenerate/mid-session branches
  // -------------------------------------------------------------------------

  DIR_EMPTY_BRANCH: "uncharted-idea", // no draft spec → disclosed notice entry
  DIR_DOOMED_DRAFT: "doomed-draft", // deleted mid-session via CONTROL_URL

  // -------------------------------------------------------------------------
  // Home status glance (spec/home-status-glance) — the "closed awaiting
  // archive" mid-lifecycle shape (43-home-status-glance)
  // -------------------------------------------------------------------------

  // A feature spec whose status is closed while it is STILL physically in
  // .verdi/specs/active/ (cmd/e2eharness/provision.go's
  // closedAwaitingArchiveName/closedAwaitingArchiveSpec, landed on main
  // before the initial commit — scratch-store-only, never the committed
  // examples/showcase corpus): parent workbench-legibility dc-4's own
  // "closed awaiting archive" example, distinct from the archive-zone
  // ARCHIVED_SPEC (loan-refi-2023) the directory-home suite already
  // covers. The glance shows this one in settling; the archive-zone twin
  // is excluded entirely (dc-2/ADJ-32 f1).
  DIR_CLOSED_AWAITING_ARCHIVE: "rate-table-sunset",

  // -------------------------------------------------------------------------
  // Diagram editor (spec/board-editor) — the disclosed-unavailable and
  // corrupt-digest error cases (37-board-diagram-editor)
  // -------------------------------------------------------------------------

  // A renderer-legal proposal OUTSIDE the op subset (sequenceDiagram): the
  // disclosed-unavailable journey. Its rail also has NO canned report, so
  // it doubles as the verification-unavailable fixture (ac-5).
  DIAGRAM_OUTSIDE_OPS: "editor-illustrative-ops",

  // Pins sha256:000…0 against DIAGRAM_BASE_BODY's real derivation base and
  // must fail visible with no write (ac-4's negative case).
  DIAGRAM_DERIVED_CORRUPT: "editor-derived-corrupt",
} as const;

// ===========================================================================
// Top-level exports — NOT fixtures: route/data-testid helper functions and
// port-derived base URLs (VERDI_E2E_PORT_BASE, D6-28 — see ../ports.ts).
// ===========================================================================

// Dex (V1-P8): served statically on :4174 by default.
export const DEX_BASE = `http://127.0.0.1:${resolvePorts().dex}`;

// The e2e control server (cmd/e2eharness/control.go): the hermetic open-MR
// feed `verdi serve` consults per render, plus the outage and delete-branch
// toggles the directory specs drive. Loopback only. :4177 by default.
export const CONTROL_URL = `http://127.0.0.1:${resolvePorts().control}`;

// The e2e inspection server (cmd/e2eharness/inspect.go): the suite's
// read-only window into the serving checkout's git state and the managed
// worktrees' files. Loopback only; it mutates nothing. :4178 by default.
export const INSPECT_URL = `http://127.0.0.1:${resolvePorts().inspect}`;

// Board v2 route: the board is a projection OF A SPEC (05 §Workbench,
// "Board as projection"), so it is addressed by spec name — distinct from
// v0's opaque board-key route (/board/<key>).
export function boardPath(spec: string): string {
  return `/board/spec/${spec}`;
}

// Dex artifact permalink (05 §Verdi-dex "Mechanics": /a/<kind>/<name>).
export function dexSpecPath(name: string): string {
  return `${DEX_BASE}/a/spec/${name}/`;
}

// The per-ADR exemption page (05 §Lenses: "A per-ADR exemption page ...
// lists an ADR's active exemptions and the exempting specs").
export function dexAdrExemptionsPath(adrName: string): string {
  return `${DEX_BASE}/a/adr/${adrName}/exemptions/`;
}

// A by-story quartet page (05 §Verdi-dex IA's by-story axis, V1-P8).
export function dexByStoryPath(name: string): string {
  return `${DEX_BASE}/by-story/${name}/`;
}

// data-testid for the reference card an external yarn target (e.g. an ADR)
// renders as on the board: "ref-card-" + the ref with "/" flattened to "-".
export function refCardTestId(ref: string): string {
  return `ref-card-${ref.replace(/\//g, "-")}`;
}

// data-testid helper for a kind's record-state chip on its obligation row
// (binding selector contract, like coverageChipTestId).
export function slotChipTestId(acId: string, kind: string): string {
  return `slot-${acId}-${kind}`;
}

// data-testid helpers for the scoping surface (binding selector
// contract, like refCardTestId above).
export function stubCardTestId(slug: string): string {
  return `stub-card-${slug}`;
}
export function coverageChipTestId(acId: string): string {
  return `coverage-${acId}`;
}

// The per-branch board address grammar the directory emits for a
// design-branch entry (draft-boards dc-1: the branch rides one path
// segment, its slash percent-encoded). The routes themselves are the
// draft-boards story's; the directory only emits them.
export function draftBoardHref(name: string): string {
  return `/b/design%2F${name}/board/spec/${name}`;
}

// data-testid helpers for the directory surface (binding selector contract).
export function dirEntryTestId(name: string): string {
  return `dir-entry-${name}`;
}
export function dirGroupTestId(group: string): string {
  return `dir-group-${group}`;
}

// data-testid helpers for the home status glance (spec/home-status-glance
// dc-5's binding selector contract, mirroring dirEntryTestId/dirGroupTestId
// above verbatim): NEW, additional testids that never replace or repurpose
// dir-entry-*/dir-group-*.
export function glanceEntryTestId(name: string): string {
  return `glance-entry-${name}`;
}
export function glanceGroupTestId(slug: string): string {
  return `glance-group-${slug}`;
}

// Provisioned by cmd/e2eharness/provision_diagram.go on the design branch
// (BINDING: names and bodies mirror that file verbatim — change them
// together). The editor page for a class: proposal diagram:
export function diagramEditorPath(name: string): string {
  return `/board/diagram/${name}`;
}

// The per-branch board address (draft-boards dc-1): the branch rides one
// path segment with its slashes percent-encoded; beneath the prefix the
// existing board addresses apply unchanged.
export function branchBoardPath(branch: string, spec: string): string {
  return `/b/${encodeURIComponent(branch)}${boardPath(spec)}`;
}

// The managed worktree's deterministic store-relative home (worktree-manager
// dc-1: design/<name> maps to .verdi/data/worktrees/<name>/) — where the
// inspection server reads a branch's tree.
export function worktreeSpecPath(name: string, spec: string): string {
  return `.verdi/data/worktrees/${name}/.verdi/specs/active/${spec}/spec.md`;
}

// The managed worktree's deterministic store-relative home for a diagram —
// where the inspection server reads the branch's committed proposal.
export function worktreeDiagramPath(name: string, diagram: string): string {
  return `.verdi/data/worktrees/${name}/.verdi/diagrams/${diagram}.mermaid`;
}
