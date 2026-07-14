---
id: spec/public-showcase
kind: spec
class: feature
title: "Public showcase corpus, drift gate, and README"
status: draft
owners: [platform-team]
problem: { text: "verdi has no public README and no canonical example store; e2e fixtures sprawl per-feature with no gate keeping new capabilities showcased", anchor: "#problem" }
outcome: { text: "a vetted showcase store at examples/showcase is the e2e feature corpus, make verify fails on unshowcased capabilities, and the README quick-starts from it", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "examples/showcase exists, lints clean, and every artifact passes the three-column vetting bar", evidence: [static, behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "make verify fails with a named gap when a capability has no showcase-backed e2e coverage", evidence: [static, behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "README quick start commands reproduce verbatim against the showcase store", evidence: [behavioral], anchor: "#ac-3" }
stubs:
  - { slug: showcase-corpus-renovation, acceptance_criteria: [ac-1] }
  - { slug: showcase-drift-gate, acceptance_criteria: [ac-2] }
  - { slug: public-readme, acceptance_criteria: [ac-3] }
---
# Public showcase

## Problem

verdi has been dogfooding its own design process for six rounds, but that
depth has no public face. There is no README a newcomer can read to
understand what verdi is or run to see it work, and no single example
store: e2e fixtures live scattered across `testdata/corpus/` and
`e2e/tests-v1/README.md`'s contract fixtures, each shaped for one
feature's assertions rather than for a reader touring the tool. Nothing
stops a new capability from shipping without ever appearing in a
walkable example — coverage of *behavior* is gated (`make verify`'s e2e
step), coverage of *showcase* is not. The corpus this project would put
in front of a stranger does not exist as a corpus; it exists as an
argument implied by the sum of the test suite.

## Outcome

A single vetted store at `examples/showcase` becomes the public face of
verdi: a small, deliberately curated set of specs, ADRs, diagrams, and a
board that together demonstrate the two-level model, the amendment
ladder, and the workbench — every artifact in it earns its place against
a three-column vetting bar (accurate, minimal, teaches something the
README or dex text alone can't). `examples/showcase` also becomes load-
bearing, not decorative: it is (or backs) the e2e feature corpus, so
`make verify` can compute which shipped capabilities the showcase fails
to demonstrate and fail the build with a named gap rather than a passing
build silently going stale. The top-level README quick-starts a reader
by running commands directly against this store, and those commands are
verified to reproduce verbatim rather than drift from what the binary
actually does.

## AC-1

`examples/showcase` exists, lints clean under `verdi lint`, and every
artifact in it passes a three-column vetting bar before it is admitted:
**accurate** (matches current binary behavior, not aspirational),
**minimal** (earns its place — no artifact included merely because it
was convenient to copy), and **teaches** (demonstrates a concept the
README prose and dex-rendered text cannot carry on their own — a real
board, a real amendment, a real gate finding). The bar itself, and the
renovation of whatever pre-existing showcase-shaped fixtures the corpus
already has, is this AC's story-level concern (stub
`showcase-corpus-renovation`); this feature spec fixes only the outcome
and the bar's three columns.

## AC-2

`make verify` fails with a named gap — which capability, which showcase
artifact or e2e path is missing — when a shipped capability has no
showcase-backed e2e coverage. This closes the drift risk the problem
statement names: today a capability can ship, tests can stay green, and
the showcase can silently stop demonstrating it. The gate's mechanism
(what counts as "a capability," how the check enumerates them, where it
plugs into the `make verify` chain) is this AC's story-level concern
(stub `showcase-drift-gate`); this feature spec fixes only that the
failure is named, not silent, and that it is a `make verify` blocker
like the gates already there.

## AC-3

The top-level README's quick start section is a sequence of commands
that a reader can run verbatim against `examples/showcase` and get the
output the README shows — no hand-substitution, no "your results may
vary." This is checked, not merely written: the reproduction is
evidenced behaviorally, matching how this project already treats every
other CLI-behavioral claim (00 §Testing rules: "every browser-facing
behavioral path ... CLI behavioral paths: end-to-end Go tests driving
the built binary"). The README's own content and structure, and the
mechanism that keeps the commands from drifting out of sync with the
binary, are this AC's story-level concern (stub `public-readme`); this
feature spec fixes only that the commands are real, run against the
showcase store, and are held to verbatim reproduction.
