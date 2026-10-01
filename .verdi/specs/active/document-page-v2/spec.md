---
id: spec/document-page-v2
kind: spec
title: "Document page"
owners: [platform-team]
class: story
story: jira:VERDI-WR-6
problem: { text: "The Document page does not say plainly whether the reader is looking at a proposal or the accepted record, or which commit it shows, and nothing links a spec object in the document back to its card on the wall.", anchor: problem }
outcome: { text: "The Document page opens with a temporal stamp and an identity card and lists its contents in a rail, and each object's id chip opens the wall with that card selected. The document body stays byte-identical across the docs site, the CLI, MCP, and the board.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "The Document page carries a temporal stamp (proposed or accepted, the commit shown, and a refreshed time computed in the browser), an identity card (the ref, class, branch, owners, and the files behind the spec), and a contents rail listing the sections with their counts. The manual Refresh control and the not-authority stamp stay.", evidence: [behavioral, attestation], anchor: ac-1 }
  - { id: ac-2, text: "Each object in the document carries an id chip that opens the wall with that card selected, and the wall selects and reveals that card on arrival.", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "The document body is byte-identical across the docs site, verdi spec doc, MCP get_document, and the board's Document tab (spec-documents ac-6). The stamp, identity card, rail, and id chips are page chrome around the shared body and are never written into it.", evidence: [static], anchor: ac-3 }
  - { id: ac-4, text: "The Document page works at 320 px and 200 % zoom and before JavaScript runs: without JavaScript the stamp shows the proposed or accepted state and the commit, the rail and the body read, and no id chip is drawn that cannot work.", evidence: [behavioral], anchor: ac-4 }
constraints:
  - { id: co-1, text: "The shared document renderer (internal/specdocload and its callers) is not changed by this story.", anchor: co-1 }
decisions:
  - { id: dc-1, text: "The id chips are drawn by the Document page at the body's existing object anchors when JavaScript runs, so the shared body stays byte-identical across its four renders (spec-documents ac-6) and a docs-site page never carries a link to a wall it cannot serve. Without JavaScript the anchors still read, and no dead chip is drawn (SI-289).", anchor: dc-1 }
  - { id: dc-2, text: "The refreshed time is computed in the browser from the page's own load and refresh (parent co-3); the server writes no clock.", anchor: dc-2 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-5" }
  - { type: supersedes, ref: "spec/document-page" }
---
# Document page

## Problem

The Document page does not say plainly whether the reader is looking at a proposal or the accepted record, or which
commit it shows, and nothing links a spec object in the document back to its card on the wall.

## Outcome

The Document page opens with a temporal stamp and an identity card and lists its contents in a rail, and each object's
id chip opens the wall with that card selected. The document body stays byte-identical across the docs site, the CLI,
MCP, and the board.

## What changed from v1

v2 revises spec/document-page through rung 3 of the amendment ladder (conflict/document-page-playwright-producers). v1
declared its 3 Playwright-backed behavioral obligations as unresolved design debt (SI-288), because no producer grammar
existed for a Playwright test, and GLG v3 keeps design debt from crossing verdi build start. The Playwright producer now
exists (SI-292 to SI-294), so each of those obligations names its test as an elaborated producer, titled `document-page
› <obligation title>` in the file v1 named. The criteria, constraints, and decisions are v1's, unchanged.

## ac-1

The Document page carries a temporal stamp (proposed or accepted, the commit shown, and a refreshed time computed in the
browser), an identity card (the ref, class, branch, owners, and the files behind the spec), and a contents rail listing
the sections with their counts. The manual Refresh control and the not-authority stamp stay.

## ac-2

Each object in the document carries an id chip that opens the wall with that card selected, and the wall selects and
reveals that card on arrival.

## ac-3

The document body is byte-identical across the docs site, verdi spec doc, MCP get_document, and the board's Document tab
(spec-documents ac-6). The stamp, identity card, rail, and id chips are page chrome around the shared body and are never
written into it.

## ac-4

The Document page works at 320 px and 200 % zoom and before JavaScript runs: without JavaScript the stamp shows the
proposed or accepted state and the commit, the rail and the body read, and no id chip is drawn that cannot work.

## co-1

The shared document renderer (internal/specdocload and its callers) is not changed by this story.

## dc-1

The id chips are drawn by the Document page at the body's existing object anchors when JavaScript runs, so the shared
body stays byte-identical across its four renders (spec-documents ac-6) and a docs-site page never carries a link to a
wall it cannot serve. Without JavaScript the anchors still read, and no dead chip is drawn (SI-289).

## dc-2

The refreshed time is computed in the browser from the page's own load and refresh (parent co-3); the server writes no
clock.
