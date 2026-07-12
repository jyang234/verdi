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
