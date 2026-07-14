---
id: spec/obligation-wall
kind: spec
title: "Obligation Wall"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-7
problem: { text: "obligations now exist, are authored on the wall, and gate activation — but they are still not READ where an operator looks. `verdi matrix` shows only `kind:verdict` for a story AC's evidence; the board AC card renders no evidence at all. So the specific thing an AC demands — 'a Playwright test that drives the edit form and asserts persistence' — lives only in the obligation file, recovered by opening it, exactly the sidecar-illegibility the feature set out to end (feature co-3: legible-without-the-sidecar).", anchor: "#problem" }
outcome: { text: "a story AC's obligations are legible on the wall: `verdi matrix` renders, for each declared evidence kind, that kind's obligation (its title, read from `.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`), and the board AC card renders the same — so what an AC demands is read from the AC's own rendered obligations, never recovered from `verdi.bindings.yaml`. A declared kind with no obligation shows a disclosed badge (the wall-receipts posture: disclosure, not refusal), never blocking the render.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "`verdi matrix` renders a story AC's obligations: for each declared evidence kind, the kind's obligation title (read from `.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`), so what the AC demands is legible on matrix; a declared kind with no obligation shows a disclosed marker, never blocking", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "the board AC card renders its obligations — each declared kind's obligation on the card (its title/prose, with a disclosed badge for a kind that has no obligation yet), so an operator reads an AC's demands on the wall itself; proven on the board", evidence: [behavioral], anchor: "#ac-2" }
links:
  - { type: implements, ref: "spec/evidence-obligations#ac-4" }
decisions:
  - { id: dc-1, text: "obligations are LOADED by (spec-name, ac-id) — reading `.verdi/obligations/<spec-name>/<ac-id>--*.md` for a story AC, the spec-name keying obligation-artifact settled and obligation-gate consumes — mirroring how the fold's AttestationExists loads attestations by path. A small loader (internal/artifact or internal/evidence) returns an AC's obligations keyed by for_kind; both surfaces (matrix, board) consume it, not two readers", anchor: "#dc-1" }
  - { id: dc-2, text: "render is disclosure-only (feature co-2, wall-receipts posture): a declared kind WITH an obligation shows its title; a declared kind WITHOUT one shows a disclosed 'no obligation' badge — never a blocking error on the read surface. The activation GATE (obligation-gate) is what refuses at accept; the wall only DISCLOSES, so a draft in progress renders legibly", anchor: "#dc-2" }
  - { id: dc-3, text: "matrix (a CLI/text surface) is backend; the board card (browser markup) is a Fable front-end concern — both consume the one loader. `verdi matrix`'s existing Evidence column (the `kind:verdict` summary) gains the obligation title alongside the kind, and the board's cardView (which today carries no evidence field) gains an obligations projection rendered on the card", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "no network in any test: the matrix render is a CLI end-to-end over a hermetic fixture story with obligations on disk; the board card render is a Go render test plus a Playwright e2e over a fixture wall carrying an obligation and a kind-without-one (the disclosed badge)", anchor: "#co-1" }
  - { id: co-2, text: "legible-without-the-sidecar (feature co-3) is the bar: the obligation's own prose must be readable from the AC's rendered obligations on matrix AND the board, not only by opening the obligation file or `verdi.bindings.yaml`. The story satisfies this at both surfaces or it is not done", anchor: "#co-2" }
frozen: { at: 2026-07-13, commit: 54b01d9bedf2cc4389b46d8b09cbc5077b19c53b, stub_matched: true }
---
# Obligation Wall

## Problem

Obligations now exist, are authored on the wall, and gate activation — but they
are still not READ where an operator looks. `verdi matrix` shows only
`kind:verdict` for a story AC's evidence; the board AC card renders no evidence
at all. So the specific thing an AC demands — "a Playwright test that drives the
edit form and asserts persistence" — lives only in the obligation file,
recovered by opening it: exactly the sidecar-illegibility the feature set out to
end (feature co-3, legible-without-the-sidecar).

## Outcome

A story AC's obligations are legible on the wall. `verdi matrix` renders, for
each declared evidence kind, that kind's obligation (its title, read from
`.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`), and the board AC card
renders the same — so what an AC demands is read from the AC's own rendered
obligations, never recovered from `verdi.bindings.yaml`. A declared kind with no
obligation shows a disclosed badge (the wall-receipts posture: disclosure, not
refusal), never blocking the render.

## AC-1

`verdi matrix` renders a story AC's obligations: for each declared evidence
kind, the kind's obligation title (read from
`.verdi/obligations/<spec-name>/<ac-id>--<kind>.md`), so what the AC demands is
legible on matrix. A declared kind with no obligation shows a disclosed marker,
never blocking. Evidence: static (the loader + render are declared) + behavioral
(a CLI end-to-end over a fixture story with obligations on disk).

## AC-2

The board AC card renders its obligations — each declared kind's obligation on
the card (its title/prose, with a disclosed badge for a kind that has no
obligation yet) — so an operator reads an AC's demands on the wall itself.
Proven on the board. Evidence: behavioral (a Go render test + a Playwright e2e
over a fixture wall).

## DC-1

Obligations are LOADED by (spec-name, ac-id): reading
`.verdi/obligations/<spec-name>/<ac-id>--*.md` for a story AC — the spec-name
keying obligation-artifact settled and obligation-gate consumes — mirroring how
the fold's `AttestationExists` loads attestations by path. A small loader
(internal/artifact or internal/evidence) returns an AC's obligations keyed by
`for_kind`; both surfaces (matrix, board) consume it, not two readers.

## DC-2

Render is disclosure-only (feature co-2, the wall-receipts posture): a declared
kind WITH an obligation shows its title; a declared kind WITHOUT one shows a
disclosed "no obligation" badge — never a blocking error on the read surface.
The activation GATE (obligation-gate) is what refuses at accept; the wall only
DISCLOSES, so a draft in progress renders legibly.

## DC-3

`verdi matrix` (a CLI/text surface) is backend; the board card (browser markup)
is a Fable front-end concern — both consume the one loader. Matrix's existing
Evidence column (the `kind:verdict` summary) gains the obligation title
alongside the kind; the board's `cardView` (which today carries no evidence
field) gains an obligations projection rendered on the card.

## CO-1

No network in any test. The matrix render is a CLI end-to-end over a hermetic
fixture story with obligations on disk; the board card render is a Go render
test plus a Playwright e2e over a fixture wall carrying an obligation and a
kind-without-one (the disclosed badge).

## CO-2

Legible-without-the-sidecar (feature co-3) is the bar: the obligation's own
prose must be readable from the AC's rendered obligations on matrix AND the
board, not only by opening the obligation file or `verdi.bindings.yaml`. The
story satisfies this at both surfaces or it is not done.
