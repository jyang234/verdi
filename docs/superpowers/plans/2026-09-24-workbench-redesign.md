# Workbench redesign — plan

**Goal.** Implement the owner's workbench redesign so a person can define, agree on, and follow a change without the development
agent coaching them. This serves the north star directly (workspace `docs/design/concepts/2026-07-11-verdesign-spec-realignment.md`:
intuitive, human-legible tools; agreement; alignment; deviations forcing conversations), and it is the prerequisite the owner set
for any further user-acceptance testing (2026-09-23). The documented comprehension gaps it targets — what to do next, why the
steps come in this order, which work is human and which is mechanical (wave-3.5 pilot F-03..F-05) — were deferred to exactly this
design pass.

**Design source.** The owner's handoff, `design_handoff_workbench` (2026-09-18), kept outside git at the workspace path
`docs/design/handoffs/2026-09-18-workbench-redesign/` (README.md is the specification; `Wall v2.dc.html` and
`Workbench Screens.dc.html` are the interactive references; `tokens.css`, `baseline/`, `screenshots/`). Fidelity is high for
layout, spacing, type, colour, states, and copy, except where this plan's decisions say otherwise.

**Base.** origin/main `ef737e63` (PR #352 merged). Worktree `verdi-wt/wr-plan`, branch `design/workbench-redesign-plan`.

**What this plan does not claim.** Closing any story or feature is still blocked (backlog BL-44). The redesign's stories are built
through the story process so they can close once a close path exists.

## Owner decisions (2026-09-24; the owner accepted the controller's defaults)

- **D-WR-1 — one redesign feature supersedes what the design contradicts.** A new feature spec, `spec/workbench-redesign`, carries
  the design as acceptance criteria and supersedes, through the ratification flow, the accepted text it contradicts (listed in
  step 1). The design is not bent to fit those older criteria.
- **D-WR-2 — the readiness page follows the design.** The Wave 6 rule that keeps the readiness page's three-row preview
  byte-compatible (Wave 6 design §3.1, §8.2) is amended. The page keeps the per-request derivation stamp; the design's "startup
  snapshot · restart verdi serve" copy is out of date and is not used.
- **D-WR-3 — no "mine" filter and no identity pill yet.** Spec owners are team handles, not email addresses, and reading the
  operator's git identity for display collides with the ambient-identity rule (GLG v3 dc-22, SI-163). Both are deferred.
- **D-WR-4 — pull requests show as "PR #n open" only.** No new forge method for review state.
- **D-WR-5 — index card flags.** Age, source, "PR #n open", and disclosed are shown. The accepted-feature call to action
  "n AC unclaimed · ac-k · New story ›" is included, measured against the Wave 6 page budget and dropped if it breaks it. The
  "step n of 4" and "matrix verdict" flags are deferred.
- **D-WR-6 — where the rail's contents go.** The review-mode inbox tray stays docked and visible in review mode (05 §Review
  stickies: never dropped). Revise and New story become the top bar's primary action on sealed walls. Instantiate moves to the
  stub card's contextual toolbar. The branch switcher becomes a menu on the top bar's branch text, keeping the branch-switch
  guard. The policy setup guide moves into the drawer's Readiness tab. Case-file badges and disclosures become chips in the
  case-file strip.
- **D-WR-7 — absorbed follow-ups.** The wall-shell versus loader gap list that readiness-recovery ac-5 assigns to the post-design
  lane is included. The recovery view (BL-23) and attestation authoring stay out.
- **D-WR-8 — guidance first.** Readiness items keep the guidance sentence as the primary line, with the fact, timing, and
  blocking flag in the technical disclosure (spec-documents ac-12), inside the design's layout.
- **D-WR-9 — "New feature" links to the existing import page** with a one-line CLI hint. A browser feature-creation route is out
  of scope.
- **D-WR-10 — new behavior ships in new, small assets.** New JavaScript goes into new files of at most 64 KiB each; the 123 KB
  `boardspec.js` does not grow.
- **D-WR-11 — build through the story process.** The redesign feature's stories are instantiated from its stubs and built on
  `verdi build start` branches (as the process-hardening plan's D-PH-1).

## Binding constraints that the design does not show

- **Responsiveness, no-JS, budget (Wave 6 §5.2, §5.3).** The design is drawn at 1440 px. Every screen must also work at 320 px
  width and 200 % zoom, be usable from the initial server response before JavaScript runs, and stay within the page budget.
- **Accessibility.** Existing axe checks, keyboard reachability, and focus order stay green; the new selection model, drawer,
  toolbar, and type picker are keyboard-operable.
- **Plain vocabulary.** Labels use the store's vocabulary preset ("planned story", "research spike" in new stores); the design's
  hard-coded "story"/"spike"/"New story" are substituted.
- **Honesty on screen.** Every state shown is proven, violated with a witness, or disclosed as unproven. The document's
  not-authority stamp stays; "refreshed n s ago" is client-side only; a manual Refresh control stays reachable on the wall and
  the Document page (Wave 6 §5.1, spec-documents dc-2).
- **Existing semantics kept.** The type picker still offers only the edge types legal for the (source kind, target kind) pair,
  with each type's consequence label and the confirmation step on gate-bearing types (05 element taxonomy); the design's six
  rows are its styling. The derivation drawer (badge-computes) keeps its dialog semantics beside the new record drawer. Card
  receipts (obligation rows, evidence slots, attestation chips, coverage chips with their exact texts) keep a place on AC cards.
  Provenance stays on-demand: the `⋯` menu's counts load lazily.
- **Tokens only.** New rules use tokens (`--wall-edge`, `--scrim` added), no literal colours except the pushpin highlights and
  shadows; dark mode follows the existing overrides. The workbench chrome is scoped so the dex, which shares `style.css`, does
  not change unless intended.
- **Evidence stays intact.** Playwright files that obligations name as evidence (`43-home-status-glance`, `37-directory-home`,
  `43-tool-view-exit`, `43-family-board-links`, `35-board-obligation-graduate`, `36-board-obligation-wall`, and the behavioral
  obligations of derivation-drawer, case-file-flags, creation-form, directory-home, draft-boards, badge-computes, evidence-slot)
  are never renamed; their assertions change only as the superseding criteria of step 1 require. The showcase map
  (`internal/showcasealign/coverage_test.go`) and its `SHOWCASE.` markers keep resolving.

## Step 1 — authority (controller-authored; one independent cross-model review of each unit's exact head, at most one correction pass, one closure check, then the owner's merge)

| Unit | Content |
|---|---|
| A1 `spec/workbench-redesign` (feature) and the amendments it needs | Acceptance criteria for the five screens and the global chrome, including the decisions above and the binding constraints; stubs aligned to the stories below. It resolves each conflicting source through the route the ratification flow allows for it: the closed stories home-status-glance (ac-2, dc-5), directory-home (ac-1, dc-1, dc-4), badge-computes (ac-5, dc-4 presentation), and case-file-flags (ac-1, dc-3, dc-4 presentation) by story supersession; the Wave 6 design's §3.1 and §8.2 readiness rule and workbench-directory dc-5's in-review grouping by a ledgered design amendment; spec-documents ac-12 and readiness-recovery ac-5 are kept, not superseded. Ambiguities the survey found (the scoping band's handwritten sticky, the trash refusal for declared stubs, pins on stub cards, the attribution and obligation yarn outside the typed picker) are decided and recorded here, not in a lane |
| A2 the story batch | Stories instantiated from the merged feature's stubs (`verdi design start --from-stub workbench-redesign <stub>`), reviewed once, merged; then `verdi build start` for each |

## Step 2 — lanes

Frontend lanes (F) are built by FABLE (`model: fable`, the workspace rule for UI work), for fixes as well; reviews are
independent Opus 5.5. Backend data lanes (B) are Tier 2, implemented by Sonnet 5 and reviewed by Opus 5.5. Each lane: a brief with
its frozen interface, write set, and the Playwright files it owns; RED then GREEN; controller pre-review with the full GREEN list;
independent review; integration only when the integrated patch equals the reviewed patch.

| Lane | Story | Scope | Depends on |
|---|---|---|---|
| F1 | chrome-and-tokens | One top bar replacing `.site-head`, `.board-head`, `.asd-posture` on every workbench page (wall, document, diagram editor, shared layout); class and mode chips; no rotations, no hand font; `--wall-edge`, `--scrim`; workbench scoping so the dex is unchanged | A2 |
| F2 | wall-canvas | Object, stub, reference, and sticky cards at the new footprint with their receipts; pins; yarn in two layers; selection model; contextual toolbar; drag-to-thread with the legal-pair type picker; add-in-place slots; card edit; keyboard; minimap. New JavaScript in new assets | F1 |
| F3 | wall-strip-and-drawer | Case-file strip with inline problem/outcome edit and badge chips; Commit & push with the change popover; readiness pill; the record drawer (Readiness with the policy guide and gap list, Provenance, Review, Context, Repo, Moves, Keys) rendering projections instead of JSON; `⋯` menu with lazy counts; rail removal with every rail item re-homed per D-WR-6; readiness targets retargeted from rail anchors | F2, B4 |
| F4 | readiness-page | The readiness page per D-WR-2 and D-WR-8 | F1 |
| F5 | document-page | Temporal stamp, identity card, id chips deep-linking to the selected wall card, contents rail; Refresh kept | F1 |
| F6 | new-story-dialog | The dialog per the design, prefilled from the index call to action | F3 |
| F7 | index | Pipeline and list views, filter row, the "other records" strip, keyboard; the D-WR-5 flags and call to action | B1, B3, F1 |
| B1 | index-data | Last commit date per design branch through the refindex port, test doubles, and the e2e harness; a clock seam for "quiet 14 d" | A2 |
| B3 | index-data | AC coverage per accepted feature as a pure function extracted from the wall projection; the disclosures count | A2 |
| B4 | wall-changes | The uncommitted typed changes (semantic diff from HEAD to the working tree) on the wall snapshot | A2 |

**Order and concurrency.** At most three lanes in flight. F1 first. Then F2, F4, F5, B1, B3, and B4 in parallel within the cap.
Then F3 (after F2 and B4), F6, and F7 (after B1 and B3).

**Shared files.** `internal/dex/assets/style.css` is one file: F1 sets the token and section structure, and each later F lane owns
only its sections (canvas, strip and drawer, readiness, document, dialog, index). `boardspecrender.go` is split by region between
F2 (canvas) and F3 (strip, rail, drawer, dialogs). `e2e/tests/helpers.ts` changes only in F2 (rail and toolbox helpers) and
`fixtures.ts` only in F7. A lane that must rename a Playwright file or a showcase marker stops and reports.

**Gates.** Each lane runs its own Playwright files serially and the Go render tests it touches. The wave gate is a serial
`make verify` with no other agent running. Playwright keeps screenshots, traces, and video off; visual fidelity is judged by the
owner at each risk gate on a running `verdi serve`.

## Owner actions

- Review and merge A1 and A2.
- Visual acceptance at the risk gate, on a running workbench.
- After the redesign lands: the real-use checkpoint (a real task, two timed journeys, the second unassisted).

## Out of scope

The recovery view (BL-23); attestation authoring; the "mine" filter and identity pill; pull-request review state; "step n of 4"
and "matrix verdict" index flags; a browser feature-creation route; real search; the design's "deliberately not built" list
(Sticky/Card/Pin toolbar actions reuse the existing dialogs; the Graduate menu; the full case-file dialog).

## Risks

- **The wall is the largest surface.** F2 and F3 may each split at brief time if they exceed a reviewable size.
- **Playwright churn.** About thirty spec files pin today's UI; lanes update them in place and never weaken what an evidence file
  proves.
- **Budget.** The index call to action computes coverage per accepted feature; B3 measures it against the Wave 6 page budget.
- **Load.** Serial gates, the three-lane cap, and the test-speed lane T1 (backlog BL-63) keep runs reliable.

## Source coverage

| Source | Destination |
|---|---|
| Owner's design handoff (README screens 1–5, global chrome) | Goal; lanes F1–F7 |
| Impact survey: authority conflicts | D-WR-1, D-WR-2; A1 |
| Impact survey: drift since the design base (guidance-first cards, per-request readiness, obligation rows, plain vocabulary, policy guide, gap list) | D-WR-2, D-WR-6, D-WR-7, D-WR-8; binding constraints |
| Impact survey: Playwright evidence producers and the showcase map | Binding constraints; shared-files rules |
| Impact survey: data gaps | D-WR-3, D-WR-4, D-WR-5; B1, B3, B4 |
| Owner decisions 2026-09-24 (eleven defaults) | D-WR-1..D-WR-11 |

Coverage: every item mapped. Intentional omissions: the items under "Out of scope".
