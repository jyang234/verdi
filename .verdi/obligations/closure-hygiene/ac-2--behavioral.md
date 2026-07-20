---
id: obligation/closure-hygiene--ac-2--behavioral
kind: obligation
title: "Two unmerged close/<name> branches — one ritual-incomplete, one superseded-elsewhere — plus a clean GREEN fixture"
owners: [platform-team]
for_kind: behavioral
links:
  - { type: verifies, ref: "spec/closure-hygiene" }
frozen: { at: 2026-07-20, commit: f8c298d3ad712ead9c108d707a10c49547a440ce }
---
# Two unmerged close/<name> branches — one ritual-incomplete, one superseded-elsewhere — plus a clean GREEN fixture

The behavioral evidence must drive a RED-direction fixturegit repository
carrying two unmerged `close/*` branches: `close/alpha`, whose tip tree
does NOT yet contain `.verdi/specs/archive/alpha/spec.md`
(ritual-incomplete), and `close/beta`, whose own tip tree DOES already
contain `.verdi/specs/archive/beta/spec.md` while the default branch,
separately, ALSO already carries `.verdi/specs/archive/beta/spec.md`
through its own independent commit history (superseded-elsewhere — the
redundant-leftover shape named by the spec's own `close/attest-helper`-style
witnesses).

The test asserts both a `close/alpha: ritual-incomplete`-shaped witness
line and a `close/beta: superseded-elsewhere`-shaped witness line appear
in the `== Closure hygiene audit ==` section, and that the run exits 1 —
then, with `close/alpha` removed from the fixture so only the
superseded-elsewhere branch remains, asserts the run exits 0 despite
`close/beta`'s line still printing.

A second, GREEN-direction fixture with no unmerged `close/*` branches at
all (none exist, or every one is already merged) asserts the section
reports clean with no branch lines at all.
