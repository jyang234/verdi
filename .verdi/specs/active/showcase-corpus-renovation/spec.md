---
id: spec/showcase-corpus-renovation
kind: spec
title: "Showcase Corpus Renovation"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-22
problem: { text: "the fixture corpus that will become examples/showcase is scattered across testdata/corpus (the primary e2e feature corpus) and testdata/dexoverlay (a second, separately-layered tree grafted on for dex/supersession coverage), named after test concerns (accepted-pending-build, new-feature-x) rather than a coherent domain, and written to pass lint and assertions rather than to read well — no artifact in it has ever been vetted against a reader-facing bar. Public-showcase#ac-1 requires a single vetted store at examples/showcase where every artifact earns its place; today there is no such store, no domain, and no vetting record.", anchor: "#problem" }
outcome: { text: "testdata/corpus and testdata/dexoverlay are relocated and merged into one committed store at examples/showcase, renamed onto a coherent LoanServ domain (services, dates, jira keys, and prose drawn from a single canon), and every artifact in it is renovated to and recorded against the three-column vetting bar — lint-clean, editorially exemplary, and narrative-coherent — with the store lint-clean end to end as a provisioned checkout.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "examples/showcase exists, lints clean, and every artifact passes the three-column vetting bar", evidence: [static, behavioral], anchor: "#ac-1" }
links:
  - { type: implements, ref: "spec/public-showcase#ac-1" }
decisions:
  - { id: dc-1, text: "relocation is a pure git-mv followed by content renovation, never a rebuild from scratch: testdata/corpus's fixturegit history and pinned SHAs are load-bearing for other suites (04's fixture discipline) and must survive the move unchanged except where a frozen artifact genuinely needs a content edit, which forces a full re-pin (00 §Provenance discipline: never silently reshape a fixture's history)", anchor: "#dc-1" }
  - { id: dc-2, text: "testdata/dexoverlay folds into the same tree as new layers.txt entries rather than staying a second overlay copy step in the e2e harness — one committed store, one construction path, matching public-showcase's outcome that examples/showcase is load-bearing, not decorative", anchor: "#dc-2" }
  - { id: dc-3, text: "renaming (accepted-pending-build to escrow-autopay, dropping new-feature-x) and prose renovation follow one canon table (dates, service names, jira keys) fixed once at the story level, so every renovated artifact in the corpus stays internally consistent rather than each file inventing its own detail", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "no network in any test: the relocated, renovated tree is exercised over fixturegit stable SHAs and a provisioned scratch checkout exactly as the e2e harness constructs one — never against a live service", anchor: "#co-1" }
  - { id: co-2, text: "every artifact under examples/showcase gets a row in verdi/docs/showcase-vetting.md recording all three bar columns or an explicit cut; a frozen archived artifact that needs a content edit is never edited in place — it goes through layers.txt content replacement and a full re-pin, noted in the vetting doc", anchor: "#co-2" }
frozen: { at: 2026-07-14, commit: e046518e66ec45c9a89a47f289aa7fa3cd992139, stub_matched: true }
---
# Showcase Corpus Renovation

## Problem

The fixture corpus that will become `examples/showcase` exists today only as
two disconnected trees: `testdata/corpus`, the primary e2e feature corpus,
and `testdata/dexoverlay`, a second tree layered on top purely to exercise
dex and supersession behavior, copied into place by the e2e harness at
provision time rather than committed as part of the store itself. Neither
tree was ever written for a reader. Names come from what a test needed
(`accepted-pending-build`, `new-feature-x`) rather than a domain a stranger
could follow, prose is minimal-to-pass-lint rather than production quality,
and nothing records whether any given artifact belongs in a corpus meant to
represent verdi in public. `spec/public-showcase#ac-1` requires exactly this
store to exist, lint clean, and pass a three-column vetting bar — none of
which the current scattered fixtures satisfy.

## Outcome

`testdata/corpus` and `testdata/dexoverlay` are relocated (`git mv`, history
preserved) and merged into one committed store at `examples/showcase`, with
the overlay's separate copy-at-provision-time step retired in favor of
ordinary `layers.txt` layers. The corpus is renamed onto a single coherent
LoanServ domain — real-sounding services, a fixed chronological timeline,
one jira-key scheme — so every artifact reads as part of one story rather
than a grab-bag of test fixtures. Every artifact in the tree, including the
two pre-existing archived quartets and the ADR roster, is renovated to
production-quality prose and recorded in `verdi/docs/showcase-vetting.md`
against the three-column bar: **lint-clean** (passes `verdi lint` with zero
unrecorded findings), **editorially exemplary** (no dead prose links, no
filler, prose a team would actually ship), and **narrative-coherent +
depth-justified** (consistent with the fixed canon, earning its place or cut
if it doesn't). The store lints clean end to end when provisioned exactly as
the e2e harness provisions a checkout — not just in the working tree.

## AC-1

`examples/showcase` exists as a single committed store — the merge of
`testdata/corpus` and `testdata/dexoverlay`, renamed off test-shaped names
onto the LoanServ domain — and lints clean under `verdi lint` run from a
scratch checkout provisioned the same way the e2e harness provisions one
(temp dir, `.verdi` copied, git init and commit, `mutable/`/`derived/`
materialized into `.verdi/data/`). Every artifact under the tree — specs,
ADRs, diagrams, boards, obligations, attestations, the two archived
quartets — has a row in `verdi/docs/showcase-vetting.md` recording all three
vetting-bar columns (lint-clean, editorially exemplary, narrative-coherent
+ depth-justified) or an explicit, reasoned cut; there is no artifact left
unrecorded. A frozen archived file that genuinely needs a content edit is
never hand-edited in place — it is replaced through `layers.txt` and a full
re-pin, and that re-pin is itself noted in the vetting doc so the history
rewrite is visible, not silent.

Evidence: **static** (the relocated, renamed tree exists at the right path,
`layers.txt` reflects the merged construction, and the vetting doc has a
complete row set with no unrecorded artifact — checked by inspection of the
committed tree and the doc's coverage against a filesystem walk) + **behavioral**
(`verdi lint` exits 0 against a store provisioned exactly as the e2e harness
provisions one, over the fully renovated tree).

## DC-1

Pure relocation, then renovation. `testdata/corpus`'s fixturegit history and
pinned SHAs are load-bearing for other suites (04 §Fixture discipline); the
move preserves them unchanged except where a frozen artifact genuinely needs
a content edit, which forces a full re-pin rather than a silent history
rewrite (00 §Provenance discipline).

## DC-2

One tree, one construction path. `testdata/dexoverlay` folds into
`examples/showcase` as new `layers.txt` layers instead of staying a second
copy-at-provision-time step in the e2e harness, matching public-showcase's
outcome that the showcase store is load-bearing infrastructure, not a
decorative sample.

## DC-3

One canon, fixed once. Renaming (`accepted-pending-build` to
`escrow-autopay`, dropping `new-feature-x`) and all prose renovation draw
from a single canon table — dates, service names, jira keys — fixed at the
story level so every renovated artifact stays internally consistent rather
than each file inventing its own detail.

## CO-1

No network in any test. The relocated, renovated tree is exercised over
fixturegit stable SHAs and a provisioned scratch checkout constructed
exactly as the e2e harness constructs one — never against a live service.

## CO-2

Every artifact recorded, nothing silent. Every file under `examples/showcase`
gets a row in `verdi/docs/showcase-vetting.md` recording all three vetting-bar
columns or an explicit cut. A frozen archived artifact that needs a content
edit is never edited in place — it goes through `layers.txt` content
replacement and a full re-pin, noted in the vetting doc so the rewrite is
visible.
