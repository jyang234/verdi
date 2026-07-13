---
id: spec/close-verb
kind: spec
title: "Close Verb"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-2
problem: { text: "`verdi close` is a phase-0 stub (I-23): recognized, exits 2 'not implemented'. No story has ever reached a true, archived closure, and — because the verdi repo is not a flowmap service of itself — the `verdi-evidence` bundle is EMPTY (D6-4), so verdi's own stories have no CI-produced static/behavioral evidence to fold. Without both the verb and a self-hosted evidence producer, true-closure#ac-1 ('archived closure on authoritative CI-produced evidence alone') is unreachable in this arena, and #ac-2 ('rollup published and readable') has no verb reaching the publish step.", anchor: "#problem" }
outcome: { text: "`verdi close <story>` drives a merged verdi story to a true, archived quartet on `source: ci` evidence alone and publishes its rollup to the configured tracker, readably — and verdi's self-hosted stories earn real CI static+behavioral evidence, so the fold reaches evidenced rather than stalling empty.", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "`verdi close <story>` folds only `source: ci` evidence, verifies eligibility, freezes the alignment report, generates a valid `rollup.json`, and archives the frozen quartet (spec, layout.json, rollup.json, deviation-report.md) to specs/archive/ — demonstrated end to end on a real merged verdi story", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "the closed story's rollup publishes to the configured tracker and reads back through it (the round-6 hermetic fake provider), reaching the publish step for real", evidence: [behavioral, attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "verdi's self-hosted stories earn CI static + behavioral evidence — a `make verify`-derived behavioral record and a static record, each `source: ci` and bound by (story, AC) through verdi.bindings.yaml — so the fold reaches evidenced and D6-4 is closed (the runtime kind is completed by the runtime-evidence story)", evidence: [static, behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/true-closure#ac-1" }
  - { type: implements, ref: "spec/true-closure#ac-2" }
decisions:
  - { id: dc-1, text: "the self-hosted evidence producer captures `make verify` results (go test + e2e pass) as `source: ci` BEHAVIORAL records and a build/vet-clean STATIC record, bound by (story, AC) via verdi.bindings.yaml — 03 §Pluggable evidence admits 'any test suite' as a behavioral producer; verdi earns its own CI evidence honestly rather than falling back to human attestation (which is not CI-produced)", anchor: "#dc-1" }
  - { id: dc-2, text: "the round-6 fake tracker is config-selectable in the real binary (verdi.yaml providers.jira gains a fake mode) so ac-2's publish + read-back is hermetic — real Jira stays a config change (true-closure dc-2); this also stops routine verbs egressing to example.atlassian.net (D6-2)", anchor: "#dc-2" }
  - { id: dc-3, text: "`verdi close` prepares the closure branch, commits the archived quartet + the active→archive move, and prints the push/open-MR instruction — it does NOT widen the forge port with CreateMR (the phase-7 precedent: verbs stop at the branch; MR creation is the human's act), the smallest reversible option", anchor: "#dc-3" }
constraints:
  - { id: co-1, text: "authoritative evidence only: every record folded into the decision to close carries `provenance.source: ci`; no local/advisory record is load-bearing (true-closure dc-1, constitution 4)", anchor: "#co-1" }
  - { id: co-2, text: "the archived quartet is frozen and the active→archive move is a pure rename (VL-009/VL-010); no network in any test — closure is exercised hermetically over fixturegit + httptest forge doubles + canned source:ci bundles + the fake provider", anchor: "#co-2" }
---
# Close Verb

## Problem

`verdi close` is ratified in 05 §CLI but the binary declines to run it —
phase-0 stub (I-23), exit 2. No story in this store has reached a true,
archived closure; the quartet exists only as a described shape. And because
the verdi repo is not a flowmap-bound service of itself, `verdi sync
--produce` assembles an EMPTY derived bundle (D6-4): verdi's own stories
declare `static`/`behavioral` evidence that the CI trust root, as built by
remote-and-ci, produces nothing for. So even with the verb, a verdi story
would fold to `pending` forever. Both gaps must close together for
true-closure#ac-1 and #ac-2 to be reachable in the self-hosted arena.

## Outcome

`verdi close <story>` drives a merged verdi story to a true, archived
quartet on `source: ci` evidence alone, then publishes its rollup to the
configured tracker where it reads back. And verdi's self-hosted stories
earn real CI evidence: a `make verify`-derived behavioral record and a
static record, bound by (story, AC), so the fold reaches evidenced instead
of stalling on an empty bundle. The first real archived closure in this
system's history — on its own CI, its own store.

## AC-1

`verdi close <story>` implements 03 §Closure ritual: resolve the story,
fold ONLY `source: ci` records (the existing authoritative filter),
`runClosureGate` for eligibility, `runAlign(freeze=true)` for the frozen
deviation report, build and digest a real `rollup.json`, and move the
quartet — spec, `layout.json`, `rollup.json`, `deviation-report.md` — into
`specs/archive/<name>/` as frozen, git-committed artifacts. Proven end to
end by closing a real merged verdi story (remote-and-ci is a candidate) and
confirming the archived quartet exists, is frozen, and traces to
`source: ci` records only. Evidence: behavioral (an exerciser drives the
close) + attestation (an operator affirms no local record was load-bearing).

## AC-2

The closed story's rollup reaches the publish step for real: `rollup
--publish` (already contract-proven) is invoked by close against the
configured provider, and the published rollup reads back through the
tracker's own surface. The round-6 tracker is the hermetic fake provider
(dc-2), so this is proven without a live Jira while exercising the real
publish path. Evidence: behavioral (publish + read-back) + attestation (the
read-back rollup reflects the story's final fold).

## AC-3

The D6-4 closure. A CI-provenance producer captures verdi's own
`make verify` outcome — go test + e2e passing — as a `source: ci`
BEHAVIORAL evidence record, and the build/vet-clean check as a STATIC
record, each bound to its (story, AC) targets through `verdi.bindings.yaml`
(03 §Pluggable evidence: "any test suite" is a behavioral producer). With
these, a verdi self-hosted story's fold reaches `evidenced` on authoritative
CI evidence rather than an empty bundle. This AC delivers static + behavioral
for the self-hosted arena; the `runtime` kind is completed by the
runtime-evidence story (true-closure#ac-3's remaining leg). Evidence: static
(the producer's own binding is declared, not inferred) + behavioral (a story
declaring these kinds folds to evidenced from CI records).

## DC-1

Capture `make verify`, don't fabricate. The honest way verdi earns CI
static/behavioral evidence is to bind its own gate's result — the same
`make verify` CI already runs — as evidence records, per the pluggable
producer seam. The rejected alternative, closing self-hosted stories on
human attestation, is dishonest against true-closure#ac-1's "CI-produced
evidence alone": attestation is a human oracle, not CI. The binding lives
in `verdi.bindings.yaml` (producer IDs → AC IDs), strict-decoded like every
other record.

## DC-2

Config-selectable fake tracker. `buildProviderRegistry` wires only real
Jira today (example.atlassian.net), so ac-2 could not be proven hermetically
and routine verbs egress to a real host (D6-2). A `fake` mode on
`providers.jira` (or a `fake:` scheme) selects the in-process fake adapter,
keeping the proof hermetic; switching to real Jira stays exactly the config
change true-closure dc-2 promises — no code path change, because the adapter
boundary is what 04 already proved.

## DC-3

Close stops at the branch. The forge port has no create-MR method and this
story does not add one (the phase-7 precedent: verbs prepare the branch and
print the push/open-MR instruction; MR creation is the human's act). Close
commits the archived quartet and the active→archive rename on a closure
branch and prints how to open its MR. Smallest reversible; widening the port
is a later decision if a verb ever needs to open an MR itself.

## CO-1

Authoritative only. Every record folded into the decision to close carries
`provenance.source: ci`; `internal/evidence.Fold`'s existing filter
(SourceCI unconditionally, SourceLocal only under Preview) is consumed
unchanged. No local or advisory record is load-bearing in a real close
(true-closure dc-1, constitution 4). A `--force-local` style escape, if
any, is disclosed and never gates.

## CO-2

Frozen, hermetic. The archived quartet is frozen (VL-009/VL-010) and the
active→archive move is a pure rename (VL-010 admits it). No network in any
test: closure is exercised over fixturegit stores, httptest forge doubles,
canned `source: ci` bundles, and the fake provider. Only the real-remote
proof (closing a story against a real `verdi-evidence` run) is left to the
controller, disclosed as the one thing the hermetic gate cannot cover.
