---
id: spec/workbench-redesign
kind: spec
title: "Workbench Redesign"
owners: [platform-team]
class: feature
problem: { text: "The workbench asks too much of a reader who has not been coached. A header stack of three rows stands before the wall; the wall's write affordances hide in a rail of eight panels; stamps, tilts, and handwriting read as gimmicks; on-demand panels print raw JSON; threading is a drag nobody discovers; and the index is one column of lists. Pilot readers could not tell what to do next, why the steps come in their order, or which work is theirs and which is mechanical (wave-3.5 pilot F-03 to F-05), and no one has yet completed a journey on the workbench without the development agent's help.", anchor: problem }
outcome: { text: "A person can define a change, reach agreement on it, and follow it on the workbench without coaching. Every page has one top bar. Clicking a card on the wall puts everything that can be done to it in front of the reader. Reading aids live in one drawer that renders projections as prose and tables. The index shows every spec in four status columns with its next move. Every state shown stays proven, violated with a witness, or disclosed as unproven.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Every workbench page — the wall, the Document page, the diagram editor, the readiness page, the index, and the pages on the shared layout — carries one top bar in place of today's site header, board header, and posture row: the wordmark, the page title, the class and mode chips, the branch and posture text, and the page's own controls. Chips replace stamps and no workbench chrome is rotated; handwriting survives only on a story or spike sticky parked in the stubs band (dc-13). The displayed bytes and the clean or dirty state stay in the bar, and the full posture (checkout, branch, worktree head, accepted head, ahead and behind) stays one action away. The docs site renders unchanged.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "On the wall, clicking a card selects it: the card and every card threaded to it are emphasized, everything else recedes, a status pill names its threads, and a contextual toolbar offers exactly the actions legal for that card in the current mode. Dragging a pin from one spec object to another opens a type picker offering only the edge types legal for that source and target, each with its consequence, and gate-bearing types ask for confirmation. A slot at the foot of each object column declares a new object with the next id in place. Cards keep their receipts (obligation rows, evidence slots, attestation chips, and coverage chips with their existing texts). The keyboard reaches every card and action (arrows, Enter, Escape, Delete), and a minimap shows and moves the viewport.", evidence: [behavioral, attestation], anchor: ac-2 }
  - { id: ac-3, text: "The case-file strip shows the problem and the outcome on one line each, each editable in place as one typed operation, with badges and disclosures as chips and a disclosure drawn distinctly from a flag. Commit and push shows the uncommitted changes in three distinct states — typed changes, unclassified changes (prose, layout, another staged path, an untracked file), and an unreadable comparison — and a comparison with no recognized operation never clears the uncommitted indicator while any other change remains. The branch switcher is a menu on the branch text and keeps the branch-switch guard. A readiness pill opens a record drawer whose tabs (Readiness, Provenance, Review, Context, Repo, Moves, Keys) render their projections as prose and tables, never raw JSON, with counts loaded only on demand. The rail is gone and every item it held has a home (dc-6); in review mode the inbox tray stays docked and visible.", evidence: [behavioral, attestation], anchor: ac-3 }
  - { id: ac-4, text: "The readiness page follows the design's layout — where you are, the four-step stepper, Focus next, Known problems in later steps, and Completed checks — with each item's guidance sentence as its primary line, its fact, timing, and blocking flag in the technical disclosure, and the per-request derivation stamp in place of the design's startup-snapshot line.", evidence: [behavioral, attestation], anchor: ac-4 }
  - { id: ac-5, text: "The Document page carries a temporal stamp (proposed or accepted, the commit, and a refreshed time computed in the browser), an identity card, id chips that open the wall with that card selected, and a contents rail; the manual Refresh control and the not-authority stamp stay.", evidence: [behavioral, attestation], anchor: ac-5 }
  - { id: ac-6, text: "The New story dialog shows the branch it will cut as the name is typed, reports a name that breaks the grammar inline, lists each acceptance criterion with its existing coverage text, and, when opened from an index card's call to action, starts with that uncovered criterion claimed; nothing is written until Create.", evidence: [behavioral, attestation], anchor: ac-6 }
  - { id: ac-7, text: "The index shows every spec on the default branch and every draft on a design branch exactly once, in four status columns — On the desk, Accepted, Active components, and On the shelf (dc-12) — each card with its title, ref, status badge, working links, age, source, and any disclosure. A draft whose branch has an open pull request carries an in-review chip, and the in-review filter selects those drafts; when the forge cannot be reached, the chip and the filter say review status is unavailable and every card stays in its column. An accepted feature with an uncovered criterion offers a New story call to action. Archived specs, record kinds, services, and boards stay reachable below the columns. A list view shows the same groups, and the keyboard moves between cards.", evidence: [behavioral, attestation], anchor: ac-7 }
  - { id: ac-8, text: "A person new to the store works a real task through two timed journeys on the redesigned workbench — defining a change, reaching agreement on it, and following it — and completes the second without coaching from the development agent; the record of each journey names where they hesitated and what the workbench showed them there.", evidence: [attestation], anchor: ac-8 }
constraints:
  - { id: co-1, text: "Every screen works at 320 px width and at 200 % zoom, is usable from the initial server response before any JavaScript runs, and stays within the Wave 6 page budget (Wave 6 workbench presentation design §5.2, §5.3). New behavior ships in new JavaScript assets of at most 64 KiB each; boardspec.js does not grow.", anchor: co-1 }
  - { id: co-2, text: "The existing accessibility checks, keyboard reachability, and focus order stay green, and the selection model, the toolbar, the type picker, the record drawer, and the dialogs are operable from the keyboard alone.", anchor: co-2 }
  - { id: co-3, text: "Three-valued honesty holds on every screen: each state shown is proven, violated with a witness, or disclosed as unproven. The Document page's not-authority stamp stays, a refreshed time is computed only in the browser, and a manual Refresh control stays reachable on the wall and on the Document page (Wave 6 §5.1; spec-documents dc-2).", anchor: co-3 }
  - { id: co-4, text: "Labels use the store's vocabulary preset, so the design's hard-coded story, spike, and New story are substituted; class and status words route through the model's display chain, and the vocabulary witness is never weakened.", anchor: co-4 }
  - { id: co-5, text: "New rules use tokens only (--wall-edge and --scrim are added) except the pushpin highlights and shadows; dark mode follows the existing overrides; the workbench chrome is scoped so the docs site, which shares style.css, changes only where intended.", anchor: co-5 }
  - { id: co-6, text: "The Playwright files that obligations name as evidence (43-home-status-glance, 37-directory-home, 43-tool-view-exit, 43-family-board-links, 35-board-obligation-graduate, 36-board-obligation-wall, and the behavioral obligations of derivation-drawer, case-file-flags, creation-form, directory-home, draft-boards, badge-computes, and evidence-slot) are never renamed, and their assertions change only where a superseded object requires it; the showcase map and its SHOWCASE markers keep resolving.", anchor: co-6 }
  - { id: co-7, text: "Existing semantics are kept: the type picker offers only the legal pairs with their consequence labels and gate confirmation (05 element taxonomy); the derivation drawer keeps its dialog semantics beside the record drawer; provenance stays on demand; and the workbench performs no forge write.", anchor: co-7 }
  - { id: co-8, text: "All workbench markup, CSS, and browser behavior is implemented by Fable subagents; browser paths are proven by Playwright under e2e/ and CLI paths by end-to-end Go tests on the built binary; screenshots, traces, video, and screen recording stay disabled; no network in any test.", anchor: co-8 }
decisions:
  - { id: dc-1, text: "One redesign feature carries the design and replaces the accepted text it contradicts; the design is not bent to fit older criteria. Accepted text it can honor is kept. The objects it cannot honor are closed specs' criteria and decisions (workbench-legibility dc-4; home-status-glance ac-1 to ac-3 and dc-1 to dc-5; badge-computes ac-5 and dc-4; case-file-flags ac-1, dc-3, and dc-4), and they are replaced only through closed-spec object supersession (docs/superpowers/specs/2026-09-24-closed-spec-object-supersession-design.md) once it is ratified and its tooling is proven, by edges on dc-12 and dc-6 and one conflict per closed spec. Until then this spec is provisional and declares no supersedes edge.", anchor: dc-1 }
  - { id: dc-2, text: "The readiness page follows the design. The Wave 6 design's rule keeping a three-row, byte-compatible preview (§3.1, §8.2) is changed by a ledgered design amendment; the per-request derivation stamp stays, and the design's startup-snapshot copy is not used.", anchor: dc-2 }
  - { id: dc-3, text: "No mine filter and no identity pill: spec owners are team handles rather than email addresses, and reading the operator's git identity for display collides with the ambient-identity rule (GLG v3 dc-22; SI-163).", anchor: dc-3 }
  - { id: dc-4, text: "A pull request shows as open, with its number, and no review state; no forge method is added, and the workbench opens no pull request — the Review tab names the branch and the command instead of the design's Open pull request button.", anchor: dc-4 }
  - { id: dc-5, text: "Index cards show age (quiet after fourteen days for a draft), source, the in-review chip, and any disclosure. The New story call to action counts acceptance criteria with no implementing stub or story — family structure from implements edges, the projection workbench-legibility ac-2 already renders on boards, not evidence-bearing state — and is dropped rather than overriding workbench-legibility dc-4 if review rejects that reading or it breaks the page budget. The step and matrix-verdict flags are deferred.", anchor: dc-5 }
  - { id: dc-6, text: "Where the rail's items go: the review-mode inbox tray stays docked and visible in review mode (05 §Review stickies); Revise and New story become the top bar's primary action on a sealed wall; Instantiate moves to the stub card's toolbar; the branch switcher becomes a menu on the branch text; the policy setup guide moves into the drawer's Readiness tab; and case-file badges and disclosures become chips in the case-file strip.", anchor: dc-6 }
  - { id: dc-7, text: "This feature absorbs the wall-shell versus loader gap list that readiness-recovery ac-5 assigns to the post-design workbench lane; the recovery view (BL-23) and attestation authoring stay out.", anchor: dc-7 }
  - { id: dc-8, text: "Guidance first: readiness items keep their guidance sentence as the primary line, with the fact, timing, and blocking flag in the technical disclosure (spec-documents ac-12), inside the design's layout.", anchor: dc-8 }
  - { id: dc-9, text: "New feature links to the existing import page with a one-line CLI hint; a browser route that creates a feature is out of scope.", anchor: dc-9 }
  - { id: dc-10, text: "The design source is the owner's handoff of 2026-09-18, identified by its archive digest (sha256 966ebcee92ec1cbf04243c26500fb132716a3b2b158fdd4451a2cd6f9d2978d1) and the per-file manifest in the redesign plan; fidelity is high for layout, spacing, type, colour, states, and copy except where these decisions differ.", anchor: dc-10 }
  - { id: dc-11, text: "Out of scope: the recovery view; attestation authoring; the mine filter and identity pill; pull-request review state and pull-request creation; the step and matrix-verdict flags; a browser feature-creation route; real search; and the design's deliberately-not-built list (the toolbar's Sticky, Card, and Pin artifact actions reuse the existing dialogs, and the Graduate menu and the full case-file dialog stay as they are).", anchor: dc-11 }
  - { id: dc-12, text: "The index has four status columns — On the desk, Accepted, Active components, and On the shelf — which are workbench-directory dc-2's four status groups in that order. The third is labelled Active components, not the design's Built, because the group comes from a component's authored active status, which does not show that anything was built. In review is a forge-sourced chip and a filter, never a column: the forge never moves a card, and unavailable review status is disclosed, never shown as zero. On the shelf shows terminal specs still in the active zone and folds archived specs into a collapsed list at its foot.", anchor: dc-12 }
  - { id: dc-13, text: "Four readings of closed stories, each honored rather than superseded. The story or spike sticky parked in the stubs band keeps its handwritten face and graduation typesets it (scoping-canvas dc-6). The trash stays as a drop target, and it and the Delete key both refuse a declared stub in plain language (stub-cards ac-3). A stub card's pin anchors its projected coverage yarn but is not a threading handle, and its toolbar offers no graduate, delete, or retype action (stub-cards ac-2). A sticky's attribution yarn and its graduation drops keep their existing paths and meanings (scoping-canvas ac-2 and ac-5; obligation-artifact ac-3); the type picker opens only for a drag between two spec objects.", anchor: dc-13 }
stubs:
  - { slug: chrome-and-tokens, acceptance_criteria: [ac-1] }
  - { slug: wall-canvas, acceptance_criteria: [ac-2] }
  - { slug: wall-changes, acceptance_criteria: [ac-3] }
  - { slug: wall-strip-and-drawer, acceptance_criteria: [ac-3] }
  - { slug: readiness-page, acceptance_criteria: [ac-4] }
  - { slug: document-page, acceptance_criteria: [ac-5] }
  - { slug: new-story-dialog, acceptance_criteria: [ac-6] }
  - { slug: index-data, acceptance_criteria: [ac-7] }
  - { slug: index-coverage, acceptance_criteria: [ac-7] }
  - { slug: index, acceptance_criteria: [ac-7] }
  - { slug: real-use-checkpoint, acceptance_criteria: [ac-8] }
---
# Workbench Redesign

**Provisional.** This spec is a draft. Its replacement of closed specs' criteria and decisions (dc-1) waits on the closed-spec
object supersession amendment and on tooling that proves that route. Until both land, it declares no `supersedes` edge and files no
conflict, and it is not proposed for acceptance.

## Problem

The workbench asks too much of a reader who has not been coached. Before the wall come three header rows: the posture, the
readiness ladder, and the case-file placards. The wall's write affordances hide in a rail of eight panels. Stamps, tilts, and
handwriting read as gimmicks. On-demand panels print raw JSON, threading is a drag nobody discovers, and the index is one column
of lists. In the wave-3.5 pilot, readers could not tell what to do next, why the steps come in their order, or which work was
theirs (F-03 to F-05). No one has yet completed a journey on the workbench without the development agent's help.

## Outcome

A person can define a change, reach agreement on it, and follow it on the workbench without coaching. The murder-board concept
stays: the cork wall, the index cards, the pushpins and yarn, the stickies, the three temporal colours, and the five fold
statuses. The chrome is rebuilt around a selection model: click a card and everything you can do to it is in front of you.

## ac-1

One top bar on every workbench page, chips instead of stamps, and nothing rotated. The posture stays in view; the detail is one
action away.

## ac-2

The selection model, the contextual toolbar, drag-to-thread with the legal-pair type picker, add-in-place slots, the keyboard, and
the minimap. Receipts stay on the cards.

## ac-3

The case-file strip, Commit and push with its three change states, the branch menu, and the record drawer. Every item the rail
held has a home.

## ac-4

The readiness page in the design's layout, guidance first, with the per-request stamp.

## ac-5

The Document page's stamp, identity card, deep-linking id chips, and contents rail; Refresh stays.

## ac-6

The New story dialog, including the prefill from the index's call to action.

## ac-7

The index's four status columns, the in-review chip and filter, the call to action, the other-records strip, and the list view.

## ac-8

The redesign is judged by use: a real task, two timed journeys, the second without coaching.

## co-1

320 px, 200 % zoom, no JavaScript required, the page budget, and small new assets.

## co-2

Keyboard and accessibility.

## co-3

Three-valued honesty on every screen.

## co-4

The store's vocabulary, not the design's hard-coded words.

## co-5

Tokens only, dark mode, and the docs site unchanged.

## co-6

Evidence files are never renamed; the showcase map keeps resolving.

## co-7

Existing semantics are kept, and the workbench makes no forge write.

## co-8

Frontend work by Fable subagents; tests without network.

## dc-1

What the redesign replaces, and the route it waits on.

## dc-2

The readiness page follows the design, under a ledgered amendment of the Wave 6 rule.

## dc-3

No mine filter and no identity pill.

## dc-4

Pull requests show as open; the workbench creates none.

## dc-5

Index card flags, and why the call to action is family structure rather than evidence.

## dc-6

Where the rail's items go.

## dc-7

The gap list is absorbed; recovery and attestation authoring stay out.

## dc-8

Guidance first.

## dc-9

New feature links to import.

## dc-10

The pinned design source.

## dc-11

What is out of scope.

## dc-12

The four status columns, and why the third is Active components.

## dc-13

Four readings of closed stories, honored rather than superseded.
