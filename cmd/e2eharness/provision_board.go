package main

// The v1 board's fixture provisioning (e2e/tests-v1/README.md "Harness
// obligations"): a draft spec on a design branch (the board opens it in
// AUTHORING mode), a spec under MR review whose comment feed is a canned
// local file (REVIEW mode, no network), the ADR the design spec's
// decision exempts, and a bare local "origin" so the board's
// commit-and-push affordance round-trips hermetically. Every name and
// body below is bound by e2e/tests-v1/fixtures.ts — change them together.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	designSpecName     = "refi-decline-flow"
	designBranch       = "design/" + designSpecName
	reviewSpecName     = "stale-decline-notices"
	emptySpecName      = "income-verification"
	obligationSpecName = "refi-decline-audit"
	replaySpecName     = "refi-decline-replay"

	// The wall-badge fixtures (spec/badge-computes ac-5): one badge-bearing
	// wall per board mode — authoring (draft on the design branch), review
	// (draft + an entry in the canned MR feed), and read-only (status
	// accepted-pending-build, which branch state cannot make authoring).
	// All three carry the same badge-triggering state (badgeSpecBody).
	badgeWallSpecName   = "decline-badge-wall"
	badgeReviewSpecName = "decline-badge-review"
	badgeSealedSpecName = "decline-badge-sealed"

	// The size-smell fixture pair (spec/case-file-flags ac-2/ac-3): two
	// authoring walls that differ ONLY in declared AC count, straddling
	// dc-1's deterministic proxy — estimated AC-column height
	// (boardlayout.ZoneOriginY 40 + count × RowPitch 176) vs the declared
	// reference-viewport-height constant 900. Five ACs estimate 920
	// (badge); four estimate 744 (no badge). If the declared layout
	// geometry or the reference constant is ever amended, these counts
	// move with it (the sizeSmellAC*Count constants below).
	sizeSmellWallSpecName = "decline-ac-sprawl"
	sizeFitWallSpecName   = "decline-ac-trim"
	sizeSmellACCount      = 5 // smallest count whose dc-1 estimate exceeds 900
	sizeFitACCount        = 4 // largest count whose dc-1 estimate fits

	// The judged-sweep fixtures (spec/derivation-drawer ac-3): three walls
	// each carrying a REAL decision-conflict-report.md whose covers pins
	// the design branch's first fixture commit — one fresh and complete
	// (no mismatch line), one stale (the spec was rewritten after that
	// commit, so covers no longer matches the wall's content digest), one
	// partial (decisions_scanned misses declared dc-2). See
	// provisionBoard's second commit.
	sweepFreshSpecName   = "decline-sweep-fresh"
	sweepStaleSpecName   = "decline-sweep-stale"
	sweepPartialSpecName = "decline-sweep-partial"

	// The evidence-slot fixture (spec/evidence-slot ac-1): a story wall
	// with REAL derived-tree state — a committed static record for ac-1
	// (fills exactly that kind's slot), an attestation file on disk
	// (fills the attestation slot), and a declared behavioral kind with
	// neither (the empty slot that badges). The no-derived-tree calm
	// state is proven on replaySpecName's wall, which has none.
	slotWallSpecName = "decline-slot-wall"
)

// designSpec is DESIGN_SPEC: the object model fixtures.ts binds (3 ACs,
// 1 constraint, dc-1 carrying the declared exempts edge to ADR_REF, dc-2
// plain, and oq-1 — the open question the scoping-canvas spike journey
// draws its resolution yarn to), with problem/outcome texts containing
// PROBLEM_SNIPPET ("stale decline") and OUTCOME_SNIPPET ("declined
// applicants").
//
// The problem text is deliberately long (several sentences): at the e2e
// viewport its case-file placard overflows the 3-line clamp, so it is the
// fixture for the board's click-to-expand affordance (33-board-expand) —
// a truncated placard shows the hint and opens the read-only expand
// dialog. Its "## Problem" body section is intentionally EMPTY, so the
// problem placard carries no hidden placard-full and its dialog falls back
// to the (long) headline text — the no-body path. Its "## Outcome" body,
// by contrast, is a RICHER-THAN-THE-HEADLINE section (a paragraph, a
// bulleted list, emphasis): the outcome placard's headline is short (it
// does NOT clamp at the wide e2e viewport), yet the placard is still
// expandable and its dialog renders that body HTML — the board-polish
// pass's always-expandable + show-body behavior, and the width-independence
// proof. EMPTY_SPEC (income-verification) keeps a short one-line problem
// headline AND an empty body section: no body, no clamp — the degenerate
// case (a short placard gets no affordance).
const designSpec = `---
id: spec/refi-decline-flow
kind: spec
class: feature
title: "Refinancing decline flow"
status: draft
owners: [platform-team]
problem: { text: "applicants keep acting on stale decline reasons after the underlying data changes, and the cost of that gap compounds at every touchpoint: they re-apply against a rule that no longer holds, they call support to contest a decision that has already been reversed, and some abandon the product entirely in the belief that a hard block still stands. Each of those paths generates avoidable rework for the servicing team, and every repetition erodes the applicant's trust that the decline they were shown is the decline that is actually real.", anchor: "#problem" }
outcome: { text: "declined applicants see the current decline state and a concrete next step", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a declined applicant sees the current decline reason within a minute of a data change", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "a reversed decline clears the notice everywhere it was shown", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "support can audit every decline notice ever shown", evidence: [static, attestation], anchor: "#ac-3" }
constraints:
  - { id: co-1, text: "decline notices never expose internal model scores", anchor: "#co-1" }
decisions:
  - { id: dc-1, text: "excuse decline events from the synchronous-write rule", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0001-outbox-events, note: "decline events are already async via the outbox" } ] }
  - { id: dc-2, text: "reuse the existing notification channel for decline updates", anchor: "#dc-2" }
open_questions:
  - { id: oq-1, text: "which decline reasons can legally be shown verbatim?", anchor: "#oq-1" }
---
# Refinancing decline flow

## Problem

## Outcome

The fix is a single source of decline truth: a borrower, a support agent, and
the audit log all read the *same* current decline state, sourced from the
servicing system of record rather than any cached copy. Concretely:

- a **reversal** propagates to every surface that showed the original
  decline, inside one refresh window;
- a **stale** decline retracts itself instead of standing until a human
  happens to notice;
- every notice ever shown stays **audit-visible**, so support can
  reconstruct exactly what the borrower saw, and when.

The wall below exists to close that gap. How the single source fans out
(an illustrative sketch — drawn, not verified; the body-figure fixture
for spec/illustrative-class, rendered by the placard body dialog):

` + "```mermaid\n" +
	"graph TD\n" +
	"  servicing --> decline-state\n" +
	"  decline-state --> borrower-ui\n" +
	"  decline-state --> support-desk\n" +
	"```" + `

## ac-1

Currency of the visible decline reason.

## ac-2

Reversal clears every surface.

## ac-3

Auditability of shown notices.

## co-1

Model scores stay internal.

## dc-1

The outbox already decouples decline events.

## dc-2

No second channel.

## oq-1

Legal review pending.
`

// designSpecLayout stores positions for a SUBSET of the objects (ac-1,
// dc-1), proving both the stored-verbatim path and the zoned fallback.
// The stored pixels sit exactly on their zones' first grid slot, so the
// zoned algorithm's occupancy check routes the unstored siblings to the
// next free slots — no overlap.
const designSpecLayout = `{
  "schema": "verdi.boardlayout/v1",
  "positions": { "ac-1": { "x": 40, "y": 40 }, "dc-1": { "x": 480, "y": 40 } }
}
`

// emptySpec is EMPTY_SPEC (fixtures.ts): the leanest VALID draft on the
// design branch — a story spec (class story requires problem, outcome,
// a tracker ref, and >=1 implements edge; no class permits zero of
// everything) with NOT ONE declared object. The newcomer's first board:
// its wall holds only the implements thread and must render the
// teaching empty-wall state rather than a void (the board-legibility
// contract). The implements target is the v2 corpus feature's ac-1,
// real on main.
const emptySpec = `---
id: spec/income-verification
kind: spec
class: story
title: "Income verification"
status: draft
owners: [platform-team]
story: jira:LOAN-2201
problem: { text: "income documents are verified by hand and applicants wait days", anchor: "#problem" }
outcome: { text: "verification completes the day the documents arrive", anchor: "#outcome" }
links:
  - { type: implements, ref: spec/escrow-autopay#ac-1 }
---
# Income verification

## Problem

## Outcome
`

// obligationSpec is OBLIGATION_STORY_SPEC (fixtures.ts): a STORY-class
// draft on the design branch — the wall on which a sticky graduates into an
// evidence obligation (spec/obligation-artifact ac-3). It declares two
// acceptance criteria (ac-1/ac-2, the obligation targets) and one decision
// (dc-1, a non-AC card the invalid-drop refusal lands on). Like every story
// it points up at a feature AC (escrow-autopay#ac-1, real on main).
const obligationSpec = `---
id: spec/refi-decline-audit
kind: spec
class: story
title: "Refinancing decline audit"
status: draft
owners: [platform-team]
story: jira:LOAN-2202
problem: { text: "decline notices are not auditable after the fact", anchor: "#problem" }
outcome: { text: "every decline notice shown is reconstructable", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "support can replay every decline notice shown to an applicant", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "the audit log is tamper-evident", evidence: [static], anchor: "#ac-2" }
decisions:
  - { id: dc-1, text: "reuse the outbox stream as the audit source", anchor: "#dc-1" }
links:
  - { type: implements, ref: spec/escrow-autopay#ac-1 }
---
# Refinancing decline audit

## Problem

## Outcome

## ac-1

Replayable decline notices.

## ac-2

Tamper-evident audit log.

## dc-1

Outbox as the audit source.
`

// replaySpec is OBLIGATION_WALL_SPEC (fixtures.ts): a STORY-class draft
// whose ac-1 declares TWO evidence kinds (behavioral, static). The wall's
// COMMITTED obligation (replayObligation, below) authors the behavioral one
// only, so ac-1's board card proves both halves of spec/obligation-wall ac-2
// at once — the authored obligation's title for behavioral, and the disclosed
// "no obligation" badge for the still-un-obligated static (dc-2). Distinct
// from obligationSpec (refi-decline-audit), whose ac-1 the graduate journey
// AUTHORS at runtime into an ephemeral store; this wall's obligation is
// committed up front, so the card reads it out on first load.
const replaySpec = `---
id: spec/refi-decline-replay
kind: spec
class: story
title: "Refinancing decline replay"
status: draft
owners: [platform-team]
story: jira:LOAN-2203
problem: { text: "a decline notice cannot be replayed after the fact, so support cannot see what an applicant actually saw", anchor: "#problem" }
outcome: { text: "every decline notice shown is reconstructable on demand", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "support can replay every decline notice shown to an applicant", evidence: [behavioral, static], anchor: "#ac-1" }
  - { id: ac-2, text: "the replay is tamper-evident", evidence: [static], anchor: "#ac-2" }
links:
  - { type: implements, ref: spec/escrow-autopay#ac-1 }
---
# Refinancing decline replay

## Problem

Once a decline notice reaches a borrower, its exact wording is gone from
any operator's view the moment the underlying account state moves on.
When a borrower disputes what they were told, or a QA review needs to
confirm a specific decline read the way the spec intended, support has no
way to reconstruct it — only whatever the current, possibly-already-
retracted state happens to be.

## Outcome

Every decline notice loansvc has ever shown is reconstructable on
demand, byte-for-byte, from the same event stream the outbox pattern
already durably records (adr/0002) — no separate notice-archival system,
no best-effort log line that might have rotated out.

## ac-1

Support can replay every decline notice shown to an applicant: given a
loan id and a rough time window, the replay view reconstructs the exact
notice text, channel, and timestamp from the outbox's own delivered
events — the same events the transactional outbox already commits
durably, read back rather than re-derived.

## ac-2

The replay is tamper-evident: each replayed notice carries a content
hash computed at delivery time, so a replay that no longer matches its
original hash is visibly flagged rather than silently trusted.
`

// replayObligation is the committed evidence-obligation artifact for
// replaySpec's ac-1 BEHAVIORAL kind — the obligation the board card reads out
// on the wall (spec/obligation-wall ac-2). Fully valid (id/for_kind
// agreement, exactly one verifies edge to the whole story spec, frozen), so
// evidence.Obligations strict-decodes it exactly as `verdi matrix` does. Its
// on-disk home is the loader's convention: .verdi/obligations/<spec>/<ac>--<kind>.md.
const replayObligation = `---
id: obligation/refi-decline-replay--ac-1--behavioral
kind: obligation
title: "a Playwright test drives the replay view and asserts the notice reappears"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/refi-decline-replay" }
frozen: { at: 2026-07-13, commit: 2f230011b192c5ac1c0ed5442be76fc401c4cbca }
---
# a Playwright test drives the replay view and asserts the notice reappears

Open a decline, mutate the underlying data, and assert the applicant-facing
notice reappears with the corrected reason — not a unit test of the projector.
`

// slotWallSpec is SLOT_WALL_SPEC (fixtures.ts): a STORY-class draft on
// the design branch whose ac-1 declares THREE evidence kinds. The
// provisioned derived tree (writeSlotWallDerived) holds one CI static
// record bound to ac-1 at main's own sha — a real ancestor of the design
// branch's HEAD, so the fold's ancestry filter genuinely admits it — and
// slotWallAttestation sits at the fold's exact attestation path, so the
// wall renders held (static), held (attestation), and empty (behavioral)
// side by side on one card: spec/evidence-slot ac-1's filled-versus-
// empty proof, with the empty kind badging (ac-2).
const slotWallSpec = `---
id: spec/decline-slot-wall
kind: spec
class: story
title: "Decline slot wall"
status: draft
owners: [platform-team]
story: jira:LOAN-2204
problem: { text: "what each declared evidence kind already holds is invisible while authoring", anchor: "#problem" }
outcome: { text: "each declared kind's record state reads on its own obligation row", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "each declared kind shows what it holds", evidence: [static, behavioral, attestation], anchor: "#ac-1" }
links:
  - { type: implements, ref: spec/escrow-autopay#ac-1 }
---
# Decline slot wall

## Problem

While a story is still being authored, nobody can tell from the wall
alone which of its declared evidence kinds already have something behind
them and which are still open — the fold's own state lives only in the
derived tree and the obligation sidecar, invisible without opening both.

## Outcome

Each declared evidence kind reads its own current holding directly on the
AC card: a kind with a real derived record shows it, a kind with an
attestation on file shows that, and a kind with neither shows a calm,
literal "no record" — the same three-way read a "verdi matrix" run would
report, without ever leaving the wall.

## ac-1

Each declared kind on ac-1 shows what it currently holds: the static kind
reads the derived tree's own CI record, the attestation kind reads the
attestation file's mere presence, and the behavioral kind — genuinely
un-evidenced so far — reads as empty rather than silently omitted.
`

// slotWallAttestation fills decline-slot-wall ac-1's attestation slot:
// a fully valid attestation artifact at the fold's own on-disk home
// (.verdi/attestations/<story-slug>/<ac>.md, evidence.AttestationExists'
// exact path — story jira:LOAN-2204 slugs to jira-loan-2204).
func slotWallAttestation(commit string) string {
	return `---
id: attestation/jira-loan-2204--ac-1
kind: attestation
title: "ac-1 slot rendering attested by QA (fixture)"
owners: [qa-lead]
links:
  - { type: verifies, ref: spec/decline-slot-wall }
frozen: { at: 2026-07-14, commit: ` + commit + ` }
---
# ac-1 attestation

Existence is the record: this file filling the attestation slot IS the
fixture's claim.
`
}

// slotWallVerdicts is the derived-tree record set filling exactly ONE of
// decline-slot-wall ac-1's three declared kinds (static): a verdi.
// evidence/v1 CI record at commit (the harness store's main sha). The
// behavioral kind deliberately has no record — the empty slot under test.
func slotWallVerdicts(commit string) string {
	return `[
  { "schema": "verdi.evidence/v1", "evidence_for": ["ac-1"], "kind": "static", "verdict": "pass", "witness": "slotRenderer -> obligationRow", "producer": "slot-static-check", "provenance": { "source": "ci", "pipeline": "914", "job": "static-verify", "commit": "` + commit + `" }, "digest": "sha256:1f2e3d4c5b6a79881f2e3d4c5b6a79881f2e3d4c5b6a79881f2e3d4c5b6a7988" }
]
`
}

// writeSlotWallDerived writes decline-slot-wall's derived tree at the
// fold's conventional location (.verdi/data/derived/<ref-slug>/<commit>/
// verdicts.json). The data/ zone is gitignored (provisionStore's
// .verdi/.gitignore) and never committed — VL-013's rule, same as the
// corpus's own derived overlay.
func writeSlotWallDerived(storeRoot, commit string) error {
	dir := filepath.Join(storeRoot, ".verdi", "data", "derived", "spec--"+slotWallSpecName, commit)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating slot-wall derived tree: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "verdicts.json"), []byte(slotWallVerdicts(commit)), 0o644); err != nil {
		return fmt.Errorf("writing slot-wall verdicts.json: %w", err)
	}
	return nil
}

// badgeSpec renders one wall-badge fixture spec (spec/badge-computes
// ac-5): a feature wall carrying every badge-triggering shape the e2e
// suite asserts, all of them REAL lint-firing state, never a canned badge:
//
//   - stub "badge-orphan" names acceptance criterion ac-99, which the spec
//     does not declare → VL-006, object-anchored to the stub's own card
//     (dc-3) → a chip on the stub card;
//   - decision dc-1 exempts adr/0099-no-such-adr, which does not resolve
//     → VL-003, object-anchored to dc-1 → a chip on the decision card;
//   - the spec's own top-level depends-on names spec/no-such-parent,
//     which does not resolve → VL-003, spec-level (no single object)
//     → a stamp on the case-file lockup.
//
// name/status/frozenLine vary per mode fixture; frozenLine is "" for a
// draft and a full "frozen: {...}\n" line for the sealed instance (status
// accepted-pending-build requires the stamp, artifact.validateFeature).
func badgeSpec(name, status, frozenLine string) string {
	return `---
id: spec/` + name + `
kind: spec
class: feature
title: "Decline badge rack"
status: ` + status + `
owners: [platform-team]
` + frozenLine + `problem: { text: "the store computes receipts this wall cannot yet wear", anchor: "#problem" }
outcome: { text: "every computed receipt hangs on the exact surface it belongs to", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the wall wears its computed receipts", evidence: [behavioral], anchor: "#ac-1" }
decisions:
  - { id: dc-1, text: "excuse the badge rack from a rule recorded nowhere", anchor: "#dc-1",
      links: [ { type: exempts, ref: adr/0099-no-such-adr, note: "deliberately dangling: the object-anchored VL-003 badge fixture" } ] }
stubs:
  - { slug: badge-orphan, acceptance_criteria: [ac-99] }
links:
  - { type: depends-on, ref: spec/no-such-parent }
---
# Decline badge rack

## Problem

The badge fixtures need a wall whose lint findings are real.

## Outcome

Chips on cards, stamps on the case file.

## ac-1

The wall wears its computed receipts.

## dc-1

Deliberately dangling refs; see the frontmatter.
`
}

// acCountSpec renders one size-smell fixture spec (spec/case-file-flags
// ac-2/ac-3): a feature draft on the design branch whose ONE variable is
// its declared acceptance-criteria count — every AC individually valid,
// no other badge-triggering state, so the wall's only computed case-file
// state is whatever the AC COUNT drives. Both fixture walls render in
// authoring mode, so the Playwright suite can also drag an AC card and
// add a sticky on the badged wall (positions are not an operand; writes
// never blocked).
func acCountSpec(name string, n int) string {
	var sb strings.Builder
	sb.WriteString(`---
id: spec/` + name + `
kind: spec
class: feature
title: "Decline acceptance ledger"
status: draft
owners: [platform-team]
problem: { text: "every decline path accretes its own acceptance criterion and nobody watches the column grow", anchor: "#problem" }
outcome: { text: "the criteria ledger stays scoped to what one wall can carry", anchor: "#outcome" }
acceptance_criteria:
`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "  - { id: ac-%d, text: \"decline path %d keeps its promise\", evidence: [behavioral], anchor: \"#ac-%d\" }\n", i, i, i)
	}
	sb.WriteString("---\n# Decline acceptance ledger\n\n## Problem\n\n## Outcome\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "\n## ac-%d\n\nDecline path %d.\n", i, i)
	}
	return sb.String()
}

// sweepSpec renders one judged-sweep fixture spec (spec/derivation-drawer
// ac-3): a lint-quiet feature draft declaring TWO decisions — the
// currently-declared set dc-3's decisions_scanned comparison runs
// against. outcome parameterizes the document bytes so the stale fixture
// can be rewritten AFTER the commit its report covers (a real content
// drift, never a canned mismatch).
func sweepSpec(name, outcome string) string {
	return `---
id: spec/` + name + `
kind: spec
class: feature
title: "Decline sweep receipts"
status: draft
owners: [platform-team]
problem: { text: "a judged sweep's own inputs are invisible on the wall", anchor: "#problem" }
outcome: { text: "` + outcome + `", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "the case file wears the judged chip", evidence: [behavioral], anchor: "#ac-1" }
decisions:
  - { id: dc-1, text: "surface the sweep on the case file", anchor: "#dc-1" }
  - { id: dc-2, text: "compare, never verdict", anchor: "#dc-2" }
---
# Decline sweep receipts

## Problem

The sweep fixtures need walls whose reports are real.

## Outcome

` + outcome + `

## ac-1

The judged chip.

## dc-1

Case-file surface.

## dc-2

Comparison, not verdict.
`
}

// sweepReport renders one fixture decision-conflict-report.md: covers
// pins a real commit of this scratch repo's history, findings carry one
// dispositioned judged finding (disposition + note) and one explicitly
// undispositioned one, and decisions_scanned lists exactly `scanned`.
func sweepReport(covers, scanned string) string {
	return `---
schema: verdi.decisionconflict/v1
covers: ` + covers + `
findings:
  - { id: judged-dcf-1, kind: judged, text: "dc-1 may collide with an ADR nobody declared an edge against", disposition: no-conflict, note: "reviewed against the corpus; no ADR governs this surface" }
  - { id: judged-dcf-2, kind: judged, text: "dc-2 reads as a policy the parent may already own" }
sweep_provenance: { adr_corpus_digest: sha256:37517e5f3dc66819f61f5a7bb8ace1921282415f10551d2defa5c3eb0985b570, decisions_scanned: [` + scanned + `] }
---
# Decision-conflict report

## Judged (undeclared-conflict sweep)

- **judged-dcf-1** [no-conflict]: dc-1 may collide with an ADR nobody declared an edge against
- **judged-dcf-2** [UNDISPOSITIONED]: dc-2 reads as a policy the parent may already own
`
}

const sweepOutcomeV1 = "every case file wears its sweep provenance"
const sweepOutcomeStaleV2 = "the wall drifted after the sweep pinned it"

// reviewSpec is REVIEW_SPEC: the board opens it in review mode (its
// canned feed reports an open MR); ac-2 is the anchored comment's
// target.
const reviewSpec = `---
id: spec/stale-decline-notices
kind: spec
class: feature
title: "Stale decline notices"
status: draft
owners: [platform-team]
problem: { text: "decline notices linger long after the decline that produced them has gone stale, and because the mirror board that reviews this work is a non-authoring room, its placards must clamp and expand exactly like the live wall does: a reviewer who opens this spec sees the same three-line case file, the same fade-and-mark on an overflowing problem, and the same read-only expand dialog on a click — nothing about legibility depends on which room you are standing in.", anchor: "#problem" }
outcome: { text: "notices retract themselves when a decline goes stale", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a stale decline retracts its notice", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "the retraction reaches every notified channel", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "a retracted notice is audit-visible", evidence: [static, attestation], anchor: "#ac-3" }
---
# Stale decline notices

## Problem

## Outcome

A decline notice is a claim about the applicant's account state at the
moment it was generated. Once that state moves on — a retried charge
clears, an escrow adjustment lands, a same-day payoff closes the loan —
the notice is no longer true, and continuing to let it stand misleads
whoever reads it next. This feature retracts each decline notice at the
moment the state that produced it changes, across every channel it
reached, and keeps that retraction visible to audit indefinitely.

## ac-1

A stale decline retracts its own notice: the moment loansvc reclassifies
a decline as stale (the same detection stale-decline's own outcome
depends on), any notice already dispatched for it is marked retracted at
its origin — never silently left standing as if it were still current.

## ac-2

The retraction reaches every channel the original notice did: web,
mobile, and the servicing console alike read the retracted state on
their next fetch, so a borrower or an agent never sees the stale notice
on one surface and its retraction on another.

## ac-3

A retracted notice stays audit-visible: the retraction is itself a
recorded fact, not a deletion — an auditor can see both that the notice
fired and that it was later retracted, with the state change that caused
it.
`

// cannedReviewFeed is REVIEW_SPEC's MR comment feed — the three routing
// cases of 02 §Record schemas' comment-token grammar (fixtures.ts:
// anchored, token-free, unresolvable-token). Served to `verdi serve`
// through workbench.LoadCannedCommentFeed (VERDI_REVIEW_FEED).
const cannedReviewFeed = `{
  "stale-decline-notices": [
    { "id": "n-1", "author": "alice", "body": "[vd:ac-2] this outcome AC reads as implementation-scoped — reword?", "resolved": false },
    { "id": "n-2", "author": "bob", "body": "overall direction looks right; one naming nit inline", "resolved": false },
    { "id": "n-3", "author": "carol", "body": "[vd:zz-99] does this still apply after the split?", "resolved": true }
  ],
  "decline-badge-review": []
}
`

// provisionBoard sets the store up for the v1 board specs: a local bare
// origin (push target), the design branch carrying both draft specs plus
// the exempted ADR, and the canned review feed file. It runs AFTER the dex
// site is built, so the static site keeps reflecting main. Returns the feed
// file's path for the serve subprocess's env.
func provisionBoard(scratch, storeRoot string) (feedPath string, err error) {
	// The board's commit affordance uses the checkout's own identity.
	if err := runGit(storeRoot, nil, "config", "user.name", "verdi-e2e"); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "config", "user.email", "e2e@verdi.invalid"); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "config", "commit.gpgsign", "false"); err != nil {
		return "", err
	}

	// A bare local origin makes "Commit & push" a real round-trip with no
	// network.
	originDir := filepath.Join(scratch, "origin.git")
	if err := runGit("", nil, "init", "--bare", "--quiet", "--initial-branch=main", originDir); err != nil {
		return "", fmt.Errorf("git init --bare: %w", err)
	}
	if err := runGit(storeRoot, nil, "remote", "add", "origin", originDir); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "push", "--quiet", "--set-upstream", "origin", "main"); err != nil {
		return "", err
	}
	// origin/HEAD is the default-branch resolution gitx.DefaultBranch (and
	// so the directory home's ref index) keys off; `git remote add` alone
	// never sets it, so pin it to main explicitly (spec/directory-home's
	// harness obligation — without it the whole-store directory would
	// honestly render an empty default-branch walk).
	if err := runGit(storeRoot, nil, "remote", "set-head", "origin", "main"); err != nil {
		return "", err
	}

	// The sealed badge fixture freezes at the store's own main tip — a
	// real commit in this scratch repo's history (frozen.commit must be
	// sha-shaped and honest; a made-up sha would be a lie the fixtures
	// don't need to tell). Resolved BEFORE the design branch is cut, so
	// the sha is main's.
	mainSHA, err := gitOutput(storeRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolving main HEAD for the sealed badge fixture: %w", err)
	}

	// The design branch: both draft specs (draft never lands on main —
	// VL-004); the ADR dc-1 exempts is the corpus's own adr/0001-outbox-events.
	if err := runGit(storeRoot, nil, "checkout", "--quiet", "-b", designBranch); err != nil {
		return "", err
	}
	// ADR_REF's target (adr/0001-outbox-events, V1-P8's fixtures.ts
	// finalization) is the corpus's own real ADR — already on main and so
	// on this branch; nothing to author here.
	files := map[string]string{
		filepath.Join(".verdi", "specs", "active", designSpecName, "spec.md"):         designSpec,
		filepath.Join(".verdi", "specs", "active", designSpecName, "layout.json"):     designSpecLayout,
		filepath.Join(".verdi", "specs", "active", reviewSpecName, "spec.md"):         reviewSpec,
		filepath.Join(".verdi", "specs", "active", emptySpecName, "spec.md"):          emptySpec,
		filepath.Join(".verdi", "specs", "active", obligationSpecName, "spec.md"):     obligationSpec,
		filepath.Join(".verdi", "specs", "active", replaySpecName, "spec.md"):         replaySpec,
		filepath.Join(".verdi", "obligations", replaySpecName, "ac-1--behavioral.md"): replayObligation,
		// The evidence-slot wall (spec/evidence-slot ac-1): the story spec
		// plus the attestation that fills its attestation slot; its derived
		// tree is written UNTRACKED below (data/ is gitignored, VL-013).
		filepath.Join(".verdi", "specs", "active", slotWallSpecName, "spec.md"): slotWallSpec,
		filepath.Join(".verdi", "attestations", "jira-loan-2204", "ac-1.md"):    slotWallAttestation(mainSHA),
		// The three wall-badge fixtures (spec/badge-computes ac-5): one per
		// board mode, same badge-triggering state.
		filepath.Join(".verdi", "specs", "active", badgeWallSpecName, "spec.md"):   badgeSpec(badgeWallSpecName, "draft", ""),
		filepath.Join(".verdi", "specs", "active", badgeReviewSpecName, "spec.md"): badgeSpec(badgeReviewSpecName, "draft", ""),
		filepath.Join(".verdi", "specs", "active", badgeSealedSpecName, "spec.md"): badgeSpec(badgeSealedSpecName, "accepted-pending-build", "frozen: { at: 2024-01-01, commit: "+mainSHA+" }\n"),
		// The size-smell pair (spec/case-file-flags): same wall, one AC
		// count over dc-1's estimate and one under it.
		filepath.Join(".verdi", "specs", "active", sizeSmellWallSpecName, "spec.md"): acCountSpec(sizeSmellWallSpecName, sizeSmellACCount),
		filepath.Join(".verdi", "specs", "active", sizeFitWallSpecName, "spec.md"):   acCountSpec(sizeFitWallSpecName, sizeFitACCount),
		// The three judged-sweep fixtures (spec/derivation-drawer ac-3):
		// committed HERE so the branch's first fixture commit is the real
		// spec revision their reports' covers pins.
		filepath.Join(".verdi", "specs", "active", sweepFreshSpecName, "spec.md"):   sweepSpec(sweepFreshSpecName, sweepOutcomeV1),
		filepath.Join(".verdi", "specs", "active", sweepStaleSpecName, "spec.md"):   sweepSpec(sweepStaleSpecName, sweepOutcomeV1),
		filepath.Join(".verdi", "specs", "active", sweepPartialSpecName, "spec.md"): sweepSpec(sweepPartialSpecName, sweepOutcomeV1),
	}
	for rel, content := range files {
		path := filepath.Join(storeRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return "", fmt.Errorf("creating %s: %w", filepath.Dir(rel), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", rel, err)
		}
	}
	if err := runGit(storeRoot, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: refi-decline-flow + stale-decline-notices fixtures"); err != nil {
		return "", err
	}

	// Second fixture commit (spec/derivation-drawer ac-3): each sweep
	// report's covers pins the FIRST fixture commit — resolved from this
	// branch's real history, never invented — and the stale wall's spec is
	// rewritten in the same commit, so at serve time the fresh/partial
	// walls' content still matches their covers while the stale wall's
	// genuinely drifted. Committing (rather than leaving the tree dirty)
	// keeps the git affordance's uncommitted-changes indicator honest for
	// the other board specs.
	coversSHA, err := gitOutput(storeRoot, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolving the design fixture commit for sweep covers: %w", err)
	}
	fullScan := func(name string) string {
		return "spec/" + name + "#dc-1, spec/" + name + "#dc-2"
	}
	sweepFiles := map[string]string{
		filepath.Join(".verdi", "specs", "active", sweepFreshSpecName, "decision-conflict-report.md"): sweepReport(coversSHA, fullScan(sweepFreshSpecName)),
		filepath.Join(".verdi", "specs", "active", sweepStaleSpecName, "decision-conflict-report.md"): sweepReport(coversSHA, fullScan(sweepStaleSpecName)),
		// The partial sweep: declared dc-2 is missing from decisions_scanned.
		filepath.Join(".verdi", "specs", "active", sweepPartialSpecName, "decision-conflict-report.md"): sweepReport(coversSHA, "spec/"+sweepPartialSpecName+"#dc-1"),
		// The stale wall's drift: same spec, rewritten outcome.
		filepath.Join(".verdi", "specs", "active", sweepStaleSpecName, "spec.md"): sweepSpec(sweepStaleSpecName, sweepOutcomeStaleV2),
	}
	for rel, content := range sweepFiles {
		if err := os.WriteFile(filepath.Join(storeRoot, rel), []byte(content), 0o644); err != nil {
			return "", fmt.Errorf("writing %s: %w", rel, err)
		}
	}
	if err := runGit(storeRoot, nil, "add", "-A"); err != nil {
		return "", err
	}
	if err := runGit(storeRoot, nil, "commit", "--quiet", "--no-verify", "-m", "design: judged-sweep report fixtures (fresh/stale/partial)"); err != nil {
		return "", err
	}

	if err := runGit(storeRoot, nil, "push", "--quiet", "--set-upstream", "origin", designBranch); err != nil {
		return "", err
	}

	// The slot wall's derived tree: keyed by main's sha (a real ancestor
	// of the design branch HEAD the board renders at), written after the
	// commit purely for narrative order — data/ is untracked either way.
	if err := writeSlotWallDerived(storeRoot, mainSHA); err != nil {
		return "", err
	}

	feedPath = filepath.Join(scratch, "review-feed.json")
	if err := os.WriteFile(feedPath, []byte(cannedReviewFeed), 0o644); err != nil {
		return "", fmt.Errorf("writing review feed: %w", err)
	}
	return feedPath, nil
}
