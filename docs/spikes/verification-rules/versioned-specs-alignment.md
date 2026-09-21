# oq-4 evidence — versioned-specs alignment

Source: `versioncompare.out` (full command output;
`go run ./docs/spikes/verification-rules/_scratch/versioncompare .`, exit
0) for the mechanical ac/constraint alignment, plus direct `grep`/`diff`
reads of the four spec files for the `supersession:` manifest and the two
real `amended` decision entries the corpus contains (below).

## 1. Is the predecessor "actually marked superseded"?

| spec | own `status:`/`state:` field | successor's `links: {type: supersedes}` |
|---|---|---|
| guided-lifecycle-governance (v1) | none | (n/a — is the predecessor) |
| guided-lifecycle-governance-v2 | none | → `spec/guided-lifecycle-governance` |
| guided-lifecycle-governance-v3 | none | → `spec/guided-lifecycle-governance-v2` |
| comparative-spike-experiments (v1) | none | (n/a — is the predecessor) |
| comparative-spike-experiments-v2 | none | → `spec/comparative-spike-experiments` |
| comparative-spike-experiments-v3 | none | → `spec/comparative-spike-experiments-v2` |

**No.** None of the six files carries any self-referential
`status`/`state` field at all (verified: `awk` frontmatter dump of each,
and `grep -n -iE '^(state|status):' ` on all six — zero hits). Supersession
is recorded exactly once, forward, on the successor's own `links:` block —
never written back onto the predecessor.

This is not an oversight: `internal/lint/vl010.go`'s own doc comment
(lines 26–33) names the reason directly — a round-5 mechanism used to
flip a predecessor's `status:` line to `superseded` in place (the accept
ritual's "predecessor flip", `cmd/verdi/accept.go`'s now-deleted
`supersede.go`); `docs/superpowers/specs/2026-08-01-merge-signals-spec-
acceptance-design.md` ("Task 7") deleted that mutation entirely —
"supersession is now derived purely from Git reachability
(internal/specstate), never written to a predecessor's own bytes" — so a
status-only edit to `superseded` is now an ordinary, illegal frozen-file
modification (VL-010), not a sanctioned exception. `grep -c "status:
superseded" .verdi/specs/active/disclosure-seam/spec.md` = 1 — the one
spec in the corpus that DOES self-mark — predates Task 7 and is frozen at
that byte forever (A8: grandfathered artifacts are never rewritten); it is
not evidence the mechanism still runs. Today "is X superseded" is a
computed fact (`internal/specstate`, git-reachability + the newest
in-reachable-history `supersedes` link), never a byte in X's own file —
checking a predecessor's own frontmatter for `superseded` is the wrong
test post-Task-7 and will always read "not marked" for every future
supersession too.

## 2. Mechanical alignment + identical/amended classification (ac/constraint scope)

Every acceptance-criterion and constraint id aligns 1:1 across all four
available version-transitions with **zero** additions, removals, or text
differences:

| transition | aligned ac/co pairs | identical | different (text) | only-in-predecessor | only-in-successor |
|---|---|---|---|---|---|
| GLG v1 → v2 | 15 | 15 | 0 | 0 | 0 |
| GLG v2 → v3 | 15 | 15 | 0 | 0 | 0 |
| CSE v1 → v2 | 14 | 14 | 0 | 0 | 0 |
| CSE v2 → v3 | 14 | 14 | 0 | 0 | 0 |
| **total** | **58** | **58 (100%)** | **0 (0%)** | 0 | 0 |

This matches each successor's own `supersession:` manifest exactly: every
transition's `carried:` list names every existing ac-/co- id with
`amended: []` and `amended_advisory: []` for that object class every
time. The only structural change any transition makes at the ac/co level
is none at all — every observed version bump adds new **decisions** only
(GLG v2 adds dc-16..dc-26; GLG v3 adds dc-27; CSE v2 adds dc-20..dc-28 and
removes oq-1; CSE v3 adds nothing new) — never a criterion or constraint.

## 3. The only two real `amended` examples in the corpus (both decisions, not ac/co)

`clauses:` (oq-3) only applies to `acceptance_criteria`/`constraints`, so
these are out of that field's scope, but they are the ONLY real
predecessor-changed-text examples anywhere in the four transitions, so
they are the only material for hand-classifying reworded-same-claim vs.
different-claim:

**dc-10, comparative-spike-experiments → v2** (declared `amended`, not
`amended_advisory`):
- v1: "...provenance, trust, **and** decision invariants are not
  configurable, extension happens only through..."
- v2: "...provenance, trust, decision, **observation, receipt, and
  result** invariants are not configurable; extension happens only
  through..."
- **Classification: different-claim.** Three new invariant categories
  (observation, receipt, result — concepts v2 introduces) are added to
  what the kernel protects; the claim's scope genuinely grew, this is not
  a paraphrase of the same set.

**dc-21, comparative-spike-experiments-v2 → v3** (declared `amended`):
- v2: "...evaluator output cannot claim harness identity or
  harness-measured values, and warmup failures remain visible
  non-decision diagnostics..."
- v3: "...evaluator output cannot claim harness identity or
  harness-measured values; **every zero-exit measured attempt carries
  only the fixed harness process measurements in addition to any
  completed evaluator evidence, while failure-outcome process
  measurements remain diagnostic and never restore eligibility**; and
  warmup failures remain visible non-decision diagnostics..."
- **Classification: different-claim.** A whole new rule is inserted
  mid-sentence (fixed-vs-diagnostic process measurement eligibility) that
  v2 never stated.

Both real `amended` entries in the corpus are unambiguous scope
additions — neither is a borderline "is this just a rewording" case.
`amended_advisory` is declared empty (`[]`) in every transition; it has
never actually been used. See README.md oq-4 for the ruling this
supports and its caveat (no real "hard to classify" example exists to
test the judge-task question against).
