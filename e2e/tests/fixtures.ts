// fixtures.ts — the single constants module for every fixture ref the
// v1 board/dex specs use (authored as tests-v1/fixtures.ts, moved here
// with the V1-P6 flip-in).
//
// Spec files import fixture refs from HERE ONLY; no spec file names a
// fixture ref directly.
//
// BINDING NOTE (V1-P6, amended V1-P8): the workbench constants below are
// FINAL — they are provisioned verbatim by cmd/e2eharness/provisionv2.go
// (a draft spec on a design branch cannot live in the committed
// testdata/corpus tree, VL-004, so the harness authors the board fixtures
// onto a scratch design branch at startup; see tests-v1/README.md
// "Harness obligations"). The dex constants were finalized by V1-P8 to
// the v2 fixture overlay's real refs (testdata/corpus +
// testdata/dexoverlay). One V1-P6 constant moved with them: ADR_NAME —
// shared by the board's ref-card tests and the dex exemption-page test —
// now names the ADR the v2 fixture feature's dc-1 actually exempts
// (adr/0001-outbox-events, real on main), and provisionv2.go's design
// spec exempts the same one; recorded as a V1-P8 ledger deviation.

// ---------------------------------------------------------------------------
// Workbench (V1-P6 board v2) — authoring mode
// ---------------------------------------------------------------------------

// Draft spec on a design branch → the board opens in AUTHORING mode
// (05 §Workbench, "Two modes, keyed by branch state").
// Binds when V1-P1's fixture-v2 overlay merges.
export const DESIGN_SPEC = "refi-decline-flow";

// The design branch the harness provisions DESIGN_SPEC on, and the default
// branch the branch-switch guard test tries to switch to.
// Binds when V1-P1's fixture-v2 overlay merges (harness branch naming).
export const DESIGN_BRANCH = `design/${DESIGN_SPEC}`;
export const MAIN_BRANCH = "main";

// DESIGN_SPEC's declared object model (02 §Object model id conventions:
// ac-N acceptance criteria, co-N constraints, dc-N decisions, oq-N open
// questions). Binds when V1-P1's fixture-v2 overlay merges.
export const AC_IDS = ["ac-1", "ac-2", "ac-3"] as const;
export const CONSTRAINT_ID = "co-1";
// dc-1 carries a declared `links: [{type: exempts, ref: <ADR_REF>}]` edge
// (the PLAN-V1 §4 "one decision carrying an exempts edge against an ADR").
export const DECISION_WITH_EXEMPTS = "dc-1";
// dc-2 is the plain decision (no links) — the picker tests draw fresh yarn
// from it. Binds when V1-P1's fixture-v2 overlay merges.
export const DECISION_PLAIN = "dc-2";

// Substrings of DESIGN_SPEC's two required attributes (02 §Object model:
// `problem` / `outcome`, each { text, anchor }). The specs assert the
// placards CONTAIN these snippets. Bind when V1-P1's fixture-v2 overlay
// merges (the attribute texts the overlay authors).
export const PROBLEM_SNIPPET = "stale decline";
export const OUTCOME_SNIPPET = "declined applicants";

// The ADR that DECISION_WITH_EXEMPTS exempts — and the ADR whose dex
// exemption page 16-dex-v2 asserts (both the board fixture's dc-1 and
// the v2 corpus feature's dc-1 exempt this one real corpus ADR). Bound
// by V1-P8's fixture finalization.
export const ADR_NAME = "0001-outbox-events";
export const ADR_REF = `adr/${ADR_NAME}`;

// ---------------------------------------------------------------------------
// Workbench (V1-P6 board v2) — review mode
// ---------------------------------------------------------------------------

// Spec under MR review → the board opens in REVIEW mode as a mirror of the
// MR (05 §Workbench "Review" bullet; §Review stickies and forge
// round-trip). The harness provisions its comment feed through
// internal/forge's fake adapter (PLAN-V1 §5 V1-P6 "Stubs").
// Binds when V1-P1's fixture-v2 overlay merges.
export const REVIEW_SPEC = "stale-decline-notices";

// The canned MR comment feed the harness serves for REVIEW_SPEC — three
// comments exercising every routing case of 02 §Record schemas'
// comment-token grammar. Bodies bind when V1-P1's fixture-v2 overlay
// merges (built from S6's committed captures, PLAN-V1 §4).
export const REVIEW_COMMENT_ANCHORED = {
  // Resolvable `[vd:<object-id>]` token → renders anchored to this card.
  objectId: "ac-2",
  body: "[vd:ac-2] this outcome AC reads as implementation-scoped — reword?",
} as const;
export const REVIEW_COMMENT_TOKEN_FREE = {
  // No token at all → inbox tray, never dropped.
  body: "overall direction looks right; one naming nit inline",
} as const;
export const REVIEW_COMMENT_UNRESOLVABLE = {
  // Token present but resolving to no declared object → inbox tray too
  // (02 §Record schemas: "a comment whose token does not resolve, or that
  // carries no token, renders in an unanchored inbox tray — never dropped").
  body: "[vd:zz-99] does this still apply after the split?",
} as const;
export const REVIEW_FEED_TOTAL = 3;

// ---------------------------------------------------------------------------
// Dex (V1-P8) — served statically on :4174 by cmd/e2eharness
// ---------------------------------------------------------------------------

export const DEX_BASE = "http://127.0.0.1:4174";

// The v2 fixture feature spec (three outcome ACs, three stubs, dc-1
// exempting ADR_NAME — PLAN-V1 §4's overlay, testdata/corpus). Finalized
// by V1-P8.
export const FEATURE_SPEC = "accepted-pending-build";

// The v2 fixture's two story specs (PLAN-V1 §4: one stub-matched, one
// deviating). STORY_STUB_MATCHED doubles as the realized stub's slug
// (R4-I-12: RefSlug(title) equals the stub's slug). Finalized by V1-P8.
export const STORY_STUB_MATCHED = "borrower-update-api";
export const STORY_DEVIATING = "borrower-update-mobile";

// Fixture stories carrying the ladder flags V1-P8's badges render
// (03 §The amendment ladder). Both resolve to the deviating story — the
// constants stay separate so the specs stay honest about which flag they
// assert: spec-stale comes from testdata/dexoverlay's living deviation
// report (accepted-deviation on the story's own ac-1, R4-I-18);
// pending-supersession from the fake forge's open MR whose candidate
// manifest amends accepted-pending-build's ac-2 (which this story's
// implements edges touch and STORY_STUB_MATCHED's do not).
export const STORY_WITH_SPEC_STALE = "borrower-update-mobile";
export const STORY_WITH_PENDING_SUPERSESSION = "borrower-update-mobile";

// The by-story axis's two archived quartets (05 §Verdi-dex IA): the
// round-four form archives layout.json in the board slot
// (testdata/dexoverlay); the grandfathered v0 form keeps its frozen
// board.json (testdata/corpus).
export const ARCHIVED_STORY_ROUND4 = "refi-rate-check-2024";
export const ARCHIVED_STORY_GRANDFATHERED = "loan-refi-2023";

// ---------------------------------------------------------------------------
// Route helpers (binding on the V1-P6/V1-P8 implementers — README.md)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Workbench (board polish) — read-only mode
// ---------------------------------------------------------------------------

// A spec that is NOT a draft on a design branch (it lives on main in the
// committed corpus), so its board renders READ-ONLY (05 §Workbench, "Two
// modes, keyed by branch state") — the fixture for the drag-refusal
// contract (a read-only board is never silently inert).
export const READONLY_SPEC = "stale-decline";

// A draft spec on the design branch with the two required attributes and
// NO declared objects — the newcomer's first board. Its board opens in
// authoring mode and must render the teaching empty-wall state, never a
// void (provisioned by cmd/e2eharness/provisionv2.go).
export const EMPTY_SPEC = "income-verification";

// EMPTY_SPEC is class: story — the harness's one story-class board
// fixture (every other board fixture is a feature) — and this is its
// `story:` tracker ref, which the case-file class tag wears as
// "story · <tracker-ref>" (provisioned by cmd/e2eharness/provisionv2.go).
export const EMPTY_SPEC_STORY_REF = "jira:LOAN-2201";

// ---------------------------------------------------------------------------
// Workbench (obligation authoring, spec/obligation-artifact ac-3)
// ---------------------------------------------------------------------------

// A STORY-class draft on the design branch that DECLARES acceptance criteria
// — the wall on which a sticky graduates into an evidence obligation (a
// sticky's yarn dropped on a story AC). Distinct from EMPTY_SPEC, which is
// deliberately object-less; this one carries the AC targets and a non-AC
// decision card (OBLIGATION_STORY_NON_AC) the invalid-drop refusal lands on.
// Provisioned by cmd/e2eharness/provisionv2.go.
export const OBLIGATION_STORY_SPEC = "refi-decline-audit";
export const OBLIGATION_STORY_AC = "ac-1";
export const OBLIGATION_STORY_NON_AC = "dc-1";

// ---------------------------------------------------------------------------
// Workbench (obligation wall, spec/obligation-wall ac-2)
// ---------------------------------------------------------------------------

// A STORY-class draft on the design branch whose ac-1 declares TWO evidence
// kinds (behavioral, static) and carries a COMMITTED obligation for the
// behavioral one only — so its board AC card reads out both halves of ac-2 on
// first load: the authored obligation's title (behavioral) and the disclosed
// "no obligation" badge (static). Distinct from OBLIGATION_STORY_SPEC, whose
// obligation the graduate journey authors at runtime; this one is pre-authored
// so the card renders it without any interaction. Provisioned by
// cmd/e2eharness/provisionv2.go.
export const OBLIGATION_WALL_SPEC = "refi-decline-replay";
export const OBLIGATION_WALL_AC = "ac-1";
export const OBLIGATION_WALL_PRESENT_KIND = "behavioral";
export const OBLIGATION_WALL_MISSING_KIND = "static";
// A substring of the committed obligation's title — the specific demand the
// card reads out on the wall (feature co-3, legible-without-the-sidecar).
export const OBLIGATION_WALL_DEMAND = "drives the replay view";

// ---------------------------------------------------------------------------
// Workbench (wall badges, spec/badge-computes ac-5)
// ---------------------------------------------------------------------------

// Three walls carrying the SAME real badge-triggering state (a stub whose
// acceptance_criteria names an undeclared AC → VL-006 chip on the stub
// card; a decision exempting a nonexistent ADR → VL-003 chip on the
// decision card; a top-level depends-on to a nonexistent spec → VL-003
// stamp on the case file), one per board mode: a draft on the design
// branch (authoring), a draft with an entry in the canned MR feed
// (review), and an accepted-pending-build instance (read-only — branch
// state cannot make a non-draft wall authoring). Provisioned by
// cmd/e2eharness/provision_board.go (badgeSpec).
export const BADGE_WALL_SPEC = "decline-badge-wall";
export const BADGE_REVIEW_SPEC = "decline-badge-review";
export const BADGE_SEALED_SPEC = "decline-badge-sealed";
// The badge anchors: the decision card wearing the VL-003 chip and the
// stub card wearing the VL-006 chip.
export const BADGE_DECISION = "dc-1";
export const BADGE_STUB_SLUG = "badge-orphan";

// ---------------------------------------------------------------------------
// Workbench (size-smell, spec/case-file-flags ac-2/ac-3)
// ---------------------------------------------------------------------------

// The size-smell fixture pair: two authoring walls differing ONLY in
// declared AC count, straddling dc-1's deterministic proxy (estimated
// AC-column height = ZoneOriginY 40 + count × RowPitch 176, vs the
// declared reference-viewport-height constant 900 — never a client
// measurement). SIZE_SMELL_SPEC declares 5 ACs (estimate 920 → badge);
// SIZE_FIT_SPEC declares 4 (estimate 744 → no badge). Provisioned by
// cmd/e2eharness/provision_board.go (acCountSpec).
export const SIZE_SMELL_SPEC = "decline-ac-sprawl";
export const SIZE_FIT_SPEC = "decline-ac-trim";
// The badged wall's estimate operands, mirrored for the drawer-content
// assertions (constants disclosed by name and value, dc-1).
export const SIZE_SMELL_ESTIMATE = 920;
export const SIZE_SMELL_REFERENCE = 900;

// Corpus artifacts nothing on DESIGN_SPEC's wall names (real on main,
// so real on the design branch) — the pin toolbox's import fixtures.
export const PIN_ADR = "adr/0002-outbox-events";
export const PIN_DIAGRAM = "diagram/loansvc-topology";
export const PIN_TRASH_ADR = "adr/0003-retry-policy";

// READONLY_SPEC's one closed-vocabulary DOCUMENT-LEVEL edge (02 §Object
// model: frontmatter `links:` declared on the spec document itself, so
// the projection emits it with From:"spec"). The document is not a card —
// it hangs above the canvas as the placards header — so this edge's yarn
// must tie to its one on-board endpoint (the target's reference card)
// with a thread pointing off the board's top edge.
export const DOC_EDGE_TYPE = "implements";
export const DOC_EDGE_TARGET = "adr/0002-outbox-events";

// ---------------------------------------------------------------------------
// Workbench (scoping canvas, spec/scoping-canvas) — the stubs band
// ---------------------------------------------------------------------------

// DESIGN_SPEC's one open question (provisioned by provisionv2.go): the
// spike proto-sticky's resolution-yarn target.
export const OQ_ID = "oq-1";

// FEATURE_SPEC (accepted-pending-build, on main → sealed wall) declares
// three stubs; its wall renders them as stub cards with Instantiate.
// STUB_SLUGS mirrors the fixture's stubs: frontmatter verbatim.
export const STUB_SLUGS = [
  "borrower-update-api",
  "borrower-update-ui",
  "borrower-update-audit-log",
] as const;

// The stub the instantiate journey cuts a branch for: it must have NO
// realized story spec in the corpus (borrower-update-api is realized;
// the audit log story does not exist yet), so design/<slug> carries a
// genuinely new scaffold.
export const INSTANTIATE_SLUG = "borrower-update-audit-log";

// The live corpus's other committed stub fixture: disclosure-legibility
// (in this repo's own .verdi store) — asserted only through the Go
// render tests; the e2e store's stub fixture is FEATURE_SPEC above.

// ---------------------------------------------------------------------------
// Supersession terminal state (spec/feature-supersession-state ac-2)
// ---------------------------------------------------------------------------

// A superseded FEATURE predecessor (rung 4) and a superseded STORY
// predecessor (rung 3), committed on main via testdata/dexoverlay
// (provision.go) — so their boards render READ-ONLY. Each is the superseded
// predecessor of a v2 successor that carries the `supersedes` edge, and each
// wears the terminal `superseded` status badge on its board head (and, on
// dex, the same `.badge-superseded` status badge). The Go build/render tests
// prove the same committed fixtures; these constants drive the Playwright
// proof of the board surface.
export const SUPERSEDED_FEATURE_SPEC = "rate-lock";
export const SUPERSEDED_STORY_SPEC = "escrow-notify";

// data-testid helpers for the scoping surface (binding selector
// contract, like refCardTestId above).
export function stubCardTestId(slug: string): string {
  return `stub-card-${slug}`;
}
export function coverageChipTestId(acId: string): string {
  return `coverage-${acId}`;
}

// ---------------------------------------------------------------------------
// Directory home (spec/directory-home) — the whole-store directory at GET /
// ---------------------------------------------------------------------------

// The e2e control server (cmd/e2eharness/control.go): the hermetic open-MR
// feed `verdi serve` consults per render, plus the outage and delete-branch
// toggles the directory specs drive. Loopback only.
export const CONTROL_URL = "http://127.0.0.1:4177";

// Directory fixture branches (cmd/e2eharness/provision_directory.go):
// each name is both the design branch's slug (design/<name>) and the
// draft spec's name.
export const DIR_LOCAL_DRAFT = "audit-trail"; // local branch only
export const DIR_REMOTE_DRAFT = "vendor-onboarding"; // remote-tracking only
export const DIR_EMPTY_BRANCH = "uncharted-idea"; // no draft spec → disclosed notice entry
export const DIR_DOOMED_DRAFT = "doomed-draft"; // deleted mid-session via CONTROL_URL

// The entry the control server's open-MR feed chips "in review": the board
// suite's design branch (DESIGN_SPEC), which exists locally AND pushed.
export const DIR_INREVIEW_SPEC = DESIGN_SPEC;

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
