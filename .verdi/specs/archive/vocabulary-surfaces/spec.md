---
id: spec/vocabulary-surfaces
kind: spec
title: "Vocabulary Surfaces"
owners: [platform-team]
class: story
status: closed
story: jira:VERDI-32
problem: { text: "display vocabulary is hard-coded across every surface (seam S10): CLI verdict strings, workbench board labels and columns, dex lens and badge labels, and MCP tool descriptions each carry their own class/state/verb literals, so a team's rename is a Go change per surface — while the model's Vocabulary block (verdi.model/v1, live since model-schema) decodes and validates but nothing consumes it", anchor: problem }
outcome: { text: "the resolved model's vocabulary drives display naming on all four surfaces — CLI verdicts, board, dex, MCP tool descriptions — through DisplayState/DisplayVerb/class display lookups with id fallback, while ids in refs, branches, commits, and history never change; a store with no model.yaml renders byte-identical output to today (the parity floor), and a store with vocabulary renames shows them everywhere at once, so a rename can never leak partially", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "CLI verdict and status output resolves display names through the model: a vocab-rename fixture store shows the renamed state/verb labels, and with no model.yaml the output is byte-identical to today's, proven against existing golden expectations", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "the workbench board and dex render the model's display names for states, verbs, and classes (columns, chips, badges, lens labels), proven by render tests and a Playwright spec over a vocab-rename fixture store", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "MCP tool descriptions speak the model's class display names, proven by driving the stdio server against a vocab-rename store", evidence: [behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/operating-model#ac-4" }
frozen: { at: 2026-07-17, commit: 2eb8e805a0c838ec2fd5a1b00b8c7e7c88ac94da, stub_matched: true }
---
# Vocabulary Surfaces

## Problem

Display vocabulary is hard-coded across every surface that turns a
state, verb, or class id into a word a person reads — the cluster the
extensibility audit's seam map names S10. `cmd/verdi/buildstart.go`'s
status-mismatch verdict (`"build start: %s status is %q, not
accepted-pending-build"`), `cmd/verdi/supersede.go`'s flipped-
predecessor status line, and `cmd/verdi/close.go`'s own verdict strings
each print class/state/verb literals baked straight into `fmt.Fprintf`
calls. The workbench board's zone labels and column chips
(`internal/workbench/boardspecrender.go`), dex's lens data
(`internal/dex/lens.go`) and its ladder badges
(`internal/wallbadge/ladder.go`), and the MCP server's own tool catalog
(`internal/mcpserve/tooldefs.go` — `get_context_bundle`'s description
alone says "a feature spec's `context:` field" as a literal string) each
carry an independent copy of the same words. A team that wants to call
a `story` a "Change Request," or `accepted-pending-build` "Ready to
build," has four surfaces to patch in Go — and a fifth, disjoint set
(refs, branches, commit trailers) it must leave alone, with nothing in
any of the four telling it which is which.

The gap is not that the model can't express the rename. `verdi.model/v1`
already declares a `vocabulary:` block (table C-2, live since
model-schema), and `internal/model.Model` already carries
`DisplayState`/`DisplayVerb` methods with exactly the fallback-to-id
behavior a cosmetic-only rename needs — the frontier check even
deliberately exempts `Vocabulary` and a class's own `Display` label from
dc-1's structural comparison (`judged-frontier-display-structural`), so
a rename alone can never trip it. But those two methods have zero
callers outside `internal/model`'s own tests today: the block decodes,
validates, round-trips, and then goes nowhere. `store.Open` already
resolves `Config.Model` for every store — the embedded canonical default
when `.verdi/model.yaml` is absent, a decoded manifest otherwise — so the
resolved model already sits at the door of every verb, every render
path, and every served response. Nobody has opened that door.

## Outcome

The resolved model's vocabulary drives display naming on all four
surfaces at once, through the same small set of lookups. CLI verdicts
(`build start`, `accept`/`supersede`, `close`) resolve the state and
verb words they print through `DisplayState`/`DisplayVerb` before
writing them to stdout/stderr; the workbench board and dex resolve
state, verb, and class words the same way for column headers, card
chips, ladder badges, and lens labels; the MCP server's tool
descriptions interpolate the model's class display names in place of
the literal `"feature spec"`/`"story spec"` text they carry today.
Class display resolution is new plumbing, not a copy of the other two:
a class's word comes from `Vocabulary.Classes[id]` when the model
declares a rename there, else the class's own declared `Class.Display`
(already structural per-class data since model-schema), else the id
itself — the same fallback-to-id shape `DisplayState`/`DisplayVerb`
already established, extended one level deeper because a class carries
two independent sources of a display word instead of one.

Two properties hold everywhere this reaches, non-negotiably. First, the
parity floor: a store with no `model.yaml` resolves to the embedded
canonical model, whose vocabulary is empty, so every lookup falls back
to the bare id and every surface prints byte-identical output to
today — the same "absence changes nothing" posture operating-model's
own ac-1 proved for the model as a whole, proved again here for its
display layer specifically. Second: ids in refs, branches, commits, and
history never change. `DisplayState`/`DisplayVerb` already only ever
return a label without touching the id passed in, and class display
resolution is designed to the identical contract — a rename is
provably cosmetic, not merely intended to be, because nothing in this
story's surface touches the identity layer at all.

Seam S10 is exactly why this is one story reaching four surfaces rather
than four smaller ones: a vocabulary is not "mostly renamed," it is
renamed or it is not. A store that renames `accepted-pending-build` to
"Ready to build" and sees that label on the CLI and the board but still
reads "accepted-pending-build" on a dex badge or inside an MCP tool's
description has not received a rename — it has received a new,
surface-specific inconsistency, indistinguishable from a bug, on top of
the old hard-coded one. A rename that reaches three of four surfaces has
leaked, not landed. That is the rationale behind splitting AC-2 (board
and dex) and AC-3 (MCP) out as their own independently-proven criteria
rather than folding every surface into AC-1's CLI proof: not because the
non-CLI surfaces are technically harder, but because "reaches every
surface at once, or it hasn't reached any of them" is the property this
story exists to establish, and one AC covering all four surfaces would
let any single one of them regress silently without failing the gate
built to catch exactly that.

## Ac 1

CLI verdict and status output resolves display names through the
model. Over a vocab-rename fixture store — `internal/model/testdata/
vocab-rename.yaml`, model-schema's own fixture, renaming `accept` to
"Sign off" and `accepted-pending-build` to "Ready to build," reused
here rather than duplicated — verdicts that name a state or a verb
(`build start`'s "not accepted-pending-build" refusal, `accept`/
`supersede`'s flipped-status confirmation, `close`'s own verdict lines)
must print the renamed label, driving the built binary the way
`feature_test.go`'s existing exact-substring assertions already do for
today's literals. And with no `model.yaml` present at all, the output
must be byte-identical to today's: this half of the AC is proven by
showing the CLI's existing golden/substring expectations — the ones
already pinned throughout `cmd/verdi`'s test suite — keep passing
unmodified, so the parity floor is not a new test asserting sameness,
it is the old tests never having to change.

## Ac 2

The workbench board and dex render the model's display names for
states, verbs, and classes — column headers and card chips
(`boardspecrender.go`), lens labels (`dex/lens.go`), and ladder badges
(`wallbadge/ladder.go`) all resolve through the identical lookups AC-1's
CLI half uses, never a second, independently hand-rolled rename table
of their own. This AC and AC-3 are what make Outcome's "reaches every
surface at once" property checkable rather than merely asserted, so
each is proven at two levels: Go render tests over each surface's
label-producing function, taking a resolved model directly (unit-level,
no server required, one test per surface so a regression on any single
one fails on its own), and one Playwright spec
(`e2e/vocabulary.spec.ts`, following the existing e2e fixture-store
convention) driving a served board over the vocab-rename fixture store
and asserting the rendered page shows "Ready to build," never
"accepted-pending-build," on the column chip a real browser renders.

## Ac 3

MCP tool descriptions speak the model's class display names. The tool
catalog `internal/mcpserve/tooldefs.go` returns today is assembled at
serve time, not compile time — this AC is that assembly step reading
`store.Config.Model` and resolving class display words into the
description text tools already carry (`get_context_bundle`'s "feature
spec" reference among them), rather than a new tool or a new field on
the wire protocol. As with AC-2, this is proven against the real
server, not a package-internal stand-in: driving the stdio MCP server
end to end, mirroring `internal/mcpserve/server_errlog_test.go`'s
existing `ServeConn`-driving convention, against a vocab-rename store,
and asserting the tool list response's description text carries the
renamed class label in place of today's literal.
