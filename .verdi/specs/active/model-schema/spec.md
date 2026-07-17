---
id: spec/model-schema
kind: spec
title: "Model Schema"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:VERDI-30
problem: { text: "the operating model has no declared artifact: lifecycle states, transitions, class structure, and verb bindings live as Go literals scattered across the codebase (seam map S1-S4, S7-S8) with no single source a reader or a tool can consult; nothing can validate a would-be model.yaml, and absence of one has no defined meaning", anchor: problem }
outcome: { text: "a new internal/model package decodes verdi.model/v1 through the store's single strict-decode seam with kernel validation (obligations lists required per transition, terminal-states freeze, reachability, catalog-only kinds) and a pinned frontier error for structural deviation; an embedded canonical.yaml is parity-tested against the code's own status enums and ritual verbs; store.Open resolves absent model.yaml to the embedded canonical; and verdi model check exposes all of it with 0/1/2 exit discipline, wired into make verify", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "DecodeModel enforces every kernel rule table-driven, one committed violation fixture per rule, unknown schemes/kinds fail closed naming the catalog", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "the embedded canonical.yaml decodes and is proven equivalent to the code's own state enums and ritual verb set by a parity test that fails on either side drifting", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "verdi model check exits 0 with an OK line (schema, counts, digest) on valid input including absent model.yaml, 1 with the pinned frontier text on structural deviation, 2 on operational trouble — proven by driving the built binary", evidence: [behavioral], anchor: ac-3 }
links:
  - { type: implements, ref: "spec/operating-model#ac-1" }
  - { type: implements, ref: "spec/operating-model#ac-2" }
frozen: { at: 2026-07-17, commit: 42b4bcd5d97ecbff00af5b98958202328162e9d6, stub_matched: true }
---
# Model Schema

## Problem

The operating model — every lifecycle state, every transition and the
obligation that gates it, the class hierarchy, and the verb bindings that
enact it — has no declared artifact today. It exists only as Go literals
scattered across roughly 45 files (extensibility audit @ 24214fd, seam map
S1-S4, S7-S8): the "is this accepted-pending-build" check alone is
independently re-implemented as near-identical regexes in
`cmd/verdi/close.go`, `supersede.go`, `accept.go`, and
`internal/lint/vl010.go`; the class hierarchy (feature decomposes into
stubs, a story's parent is feature) lives as unstated convention baked into
the scaffolds and the board's edge-legality table; the ritual-verb-to-
transition mapping lives in `cmd/verdi/dispatch.go` and the closure gate's
own condition lists, as code, not data. No single source exists that a
reader — or a tool — can consult to see the whole lifecycle at once, and
there is no way to validate a hand-written `model.yaml` against it, because
no schema for one exists yet. Absence of a `model.yaml` today has no defined
meaning either: nothing distinguishes "this store has no opinion, use the
built-in model" from "this store forgot to configure one" — both look
identical because neither state is representable.

## Outcome

A new `internal/model` package becomes the operating model's one declared
source. `verdi.model/v1` decodes through the store's single strict-decode
seam (`artifact.DecodeStrict`, the same wrapper pattern
`store.DecodeManifest` already uses) into typed `Model`/`Class`/
`Lifecycle`/`Transition`/`Obligation`/`Vocabulary` structures, and a kernel
validation pass enforces the rules that make a manifest well-formed
regardless of which concrete model it describes: every transition declares
an `obligations:` list (a present-but-empty list is distinct from an absent
key — decode must tell `nil` from `[]`), every `terminal` state is drawn
from `states`, every state is reachable, every transition's `from`/`to`
names a declared state, every class's `parent` names a declared class,
every class carries a non-empty `template`, and every obligation's
`scheme`/`kind` are drawn from the closed catalog (`author-vouch`,
`countersign`, `gate-pass`, `fold-green`, `hook`, `stubs-reconciled`), with
`count` legal only on `countersign` and `hook` legal only with a non-empty
`Hook`. A manifest that decodes cleanly but describes a *different*
lifecycle than today's — a different state set, transition set, class set,
or obligation set — fails closed with one pinned frontier error naming the
frontier crossed (operating-model dc-1's smallest reversible slice: v1
describes the model; it does not yet let the shape move).

An embedded `internal/model/canonical.yaml` expresses today's hard-coded
model exactly, and a parity test proves it rather than asserting it: the
canonical model's state set is checked equal to `internal/artifact/
status.go`'s own status enums, and its transition/verb set equal to
`cmd/verdi/dispatch.go`'s own ritual verbs, through exported helpers on
both sides rather than reflection on either side's private maps — so the
embedded default can never silently drift from the code it exists to
describe. `store.Open` resolves an absent `.verdi/model.yaml` to this
embedded canonical rather than treating absence as an error or a bare zero
value, so a store with no manifest at all changes nothing about how it
behaves today (the load-bearing parity claim the sibling stubs and the
phase's own exit gate depend on). `verdi model check` exposes all of this
at the CLI with the same three-valued discipline every other verdi verb
keeps: 0 with an OK line naming the schema, object counts, and the
resolved model's digest on a valid manifest (including the absent-file
case); 1 with the pinned frontier text on a structurally deviant one; 2 on
operational trouble (an unreadable store, a decode error). The check is
wired into `make verify`'s `lint-store` step, so this exit discipline is
exercised on every gate run, not only in this story's own tests.

## Ac 1

`DecodeModel` (`internal/model/decode.go`) is the schema's one entry
point: strict-decode via the shared `internal/artifact` seam, then a
kernel validation pass, table-driven over every rule the Outcome section
lists. Each rule is proven, not merely asserted, by a committed violation
fixture that trips exactly that rule and no other — one YAML file per rule
under `internal/model/testdata/`, alongside the canonical fixture that must
decode clean. An obligation's `scheme` or `kind` value outside the closed
catalog fails closed, and the error names the catalog itself — the legal
scheme/kind list — never a bare "invalid value", so an operator hand-editing
a manifest learns what *is* legal in the same breath as learning what is
not.

## Ac 2

The embedded `canonical.yaml` is not merely present — it is proven to agree
with the code it claims to describe. A parity test decodes it and compares
its state set against `internal/artifact/status.go`'s own status enums and
its ritual-verb set against `cmd/verdi/dispatch.go`'s own dispatch table.
The test is constructed so that drift on *either* side — a status added to
the Go enum with no matching state in the YAML, or the reverse — fails it;
the embedded default can never quietly diverge from the hard-coded model it
is supposed to be a truthful description of, which is the property the
rest of the phase's strangler sequencing (and every sibling stub of this
feature) depends on holding.

## Ac 3

`verdi model check` (`cmd/verdi/model.go`) gives the manifest the same
fail-closed feedback `verdi lint` already gives a spec. Driving the real
built binary end to end (mirroring `close_test.go`'s own style, not a
package-internal unit test standing in for it): with no `model.yaml`
present it exits 0 and prints an OK line naming the schema
(`verdi.model/v1`), the class/transition counts, and the resolved
canonical model's digest; with a valid hand-written `model.yaml`
(vocabulary/template changes only, dc-1's frontier) it exits 0 the same
way over that manifest's own counts and digest; with a structurally
deviant manifest it exits 1 and prints the pinned frontier error text
verbatim, never a paraphrase; with a missing store or an
unreadable/undecodable manifest it exits 2. `model check` is wired into
`make verify`'s `lint-store` step, so this exit discipline is exercised on
every gate run going forward, not only by this story's own tests.
