---
id: spec/index-data
kind: spec
title: "Index data"
owners: [platform-team]
class: story
story: jira:VERDI-WR-8
problem: { text: "The index cannot say how old a card's work is. It has no commit date for a draft's branch or for an accepted spec, so it can neither show a card's age nor mark a draft that has gone quiet.", anchor: problem }
outcome: { text: "Every index entry carries the date of its last change, read through the index's git port, and a draft is marked quiet after fourteen days against an injected clock, so tests and the e2e harness stay deterministic.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "Each design-branch entry carries its branch tip's committer date, and each default-branch entry carries the committer date of the commit where its current bytes landed on the default branch. Both are read through refindex's git port in the same per-render computation. A date that cannot be read is disclosed on that entry, never shown as zero or as the current date.", evidence: [static, behavioral], anchor: ac-1 }
  - { id: ac-2, text: "A draft is quiet when its last change is more than fourteen days before now. The drafts are the entries in refindex's drafts-in-progress group (the On the desk column): every design-branch entry, and any default-branch entry whose status groups there. Now comes from an injected clock: production passes the wall clock at render time, and tests and the e2e harness set it, so no test depends on the real date.", evidence: [static], anchor: ac-2 }
  - { id: ac-3, text: "The git port's test doubles and the e2e harness give every fixture entry a date, and the harness can set the clock, so later stories' Playwright files can assert ages and the quiet mark deterministically.", evidence: [static, behavioral], anchor: ac-3 }
constraints:
  - { id: co-1, text: "Dates are read only through refindex's port, in the one index computation per render; nothing is persisted, and the default-branch walk is not repeated.", anchor: co-1 }
  - { id: co-2, text: "Every git integration is proven against fixturegit repositories, never the live store, and no test uses the network.", anchor: co-2 }
decisions:
  - { id: dc-1, text: "The design shows an age on every card, not only on drafts. A default-branch entry's age is the landing commit of its current bytes (specstate's baseline), the date the reader's version reached the default branch; it is already resolved per render, and a later in-place edit that lands moves it. A design-branch entry's age is its branch tip.", anchor: dc-1 }
  - { id: dc-2, text: "The quiet threshold, fourteen days, is one named constant shared by the card mark and the quiet filter. Quiet applies to the drafts the parent names (dc-5), which are the entries in the drafts-in-progress group whatever their source; no entry outside that group reads quiet.", anchor: dc-2 }
  - { id: dc-3, text: "The clock is passed to the consumer that decides quiet, never read from a package variable; the index computation itself reads no clock.", anchor: dc-3 }
links:
  - { type: implements, ref: "spec/workbench-redesign#ac-7" }
---
# Index data

## Problem

The index cannot say how old a card's work is. It has no commit date for a draft's branch or for an accepted spec, so it
can neither show a card's age nor mark a draft that has gone quiet.

## Outcome

Every index entry carries the date of its last change, read through the index's git port, and a draft is marked quiet
after fourteen days against an injected clock, so tests and the e2e harness stay deterministic.

## ac-1

Each design-branch entry carries its branch tip's committer date, and each default-branch entry carries the committer
date of the commit where its current bytes landed on the default branch. Both are read through refindex's git port in
the same per-render computation. A date that cannot be read is disclosed on that entry, never shown as zero or as the
current date.

## ac-2

A draft is quiet when its last change is more than fourteen days before now. The drafts are the entries in refindex's
drafts-in-progress group (the On the desk column): every design-branch entry, and any default-branch entry whose status
groups there. Now comes from an injected clock: production passes the wall clock at render time, and tests and the e2e
harness set it, so no test depends on the real date.

## ac-3

The git port's test doubles and the e2e harness give every fixture entry a date, and the harness can set the clock, so
later stories' Playwright files can assert ages and the quiet mark deterministically.

## co-1

Dates are read only through refindex's port, in the one index computation per render; nothing is persisted, and the
default-branch walk is not repeated.

## co-2

Every git integration is proven against fixturegit repositories, never the live store, and no test uses the network.

## dc-1

The design shows an age on every card, not only on drafts. A default-branch entry's age is the landing commit of its
current bytes (specstate's baseline), the date the reader's version reached the default branch; it is already resolved
per render, and a later in-place edit that lands moves it. A design-branch entry's age is its branch tip.

## dc-2

The quiet threshold, fourteen days, is one named constant shared by the card mark and the quiet filter. Quiet applies to
the drafts the parent names (dc-5), which are the entries in the drafts-in-progress group whatever their source; no
entry outside that group reads quiet.

## dc-3

The clock is passed to the consumer that decides quiet, never read from a package variable; the index computation itself
reads no clock.
