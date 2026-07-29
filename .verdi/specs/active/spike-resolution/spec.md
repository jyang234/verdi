---
id: spec/spike-resolution
kind: spec
title: "Spike Resolution"
owners: [platform-team]
class: story
status: draft
story: jira:VERDI-R54-4
problem: { text: "a spike answering open questions had no attribution home anywhere: which question a spike resolves lived in prose or memory, could not survive into the spec register, and gave the wall nothing to render when the claim was made — the only moment a duplicate claim is actionable", anchor: "#problem" }
outcome: { text: "a spike sticky's yarn to open questions is the resolution attribution; graduation mints a spike-flagged stub carrying the question ids it resolves under the round-5.4 fail-closed grammar; one spike answering many questions is normal, and a question claimed by multiple spikes renders a soft smell — an observation, never a rule", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "a spike sticky's yarn to open questions records resolution attribution, and graduation mints the spike stub — slug, spike: true, resolves: [the yarn'd oq-ids] — in the feature's frontmatter, surviving into the spec register", evidence: [behavioral], anchor: "#ac-1" }
  - { id: ac-2, text: "one spike may answer many questions; a question claimed by two or more spikes renders the soft smell on the wall — an observation, never a refusal or a lint error", evidence: [behavioral], anchor: "#ac-2" }
  - { id: ac-3, text: "the round-5.4 grammar fails closed: resolves requires spike: true, a spike stub declares resolves and no acceptance_criteria, a plain stub the reverse — and a resolves entry naming an undeclared open question of the same spec is a named lint refusal", evidence: [static, behavioral], anchor: "#ac-3" }
links:
  - { type: implements, ref: "spec/scoping-canvas#ac-5" }
decisions:
  - { id: dc-1, text: "retro-decomposition, disclosed: minted after its behavior merged (ledger R4-I-87); this spec describes the built slice and invents nothing new", anchor: "#dc-1" }
  - { id: dc-2, text: "attribution graduates into the stub itself (parent dc-2) — a spike-flagged stub carrying the oq-ids, one flag-discriminated list (parent dc-4) — and the multi-claim smell is the owner's cardinality ruling ratified in round 5.4: a norm-level observation, never an error", anchor: "#dc-2" }
constraints:
  - { id: co-1, text: "no network in any test: attribution, graduation, the smell, and the fail-closed grammar are exercised by artifact decode tests, the VL-006 sibling-check fixtures, workbench render tests, and hermetic Playwright specs, all under make verify", anchor: "#co-1" }
---
# Spike Resolution

## Problem

A spike answering open questions had no attribution home anywhere: which
question a spike resolves lived in prose or memory, could not survive
into the spec register, and gave the wall nothing to render when the
claim was made — the only moment a duplicate claim is actionable.

## Outcome

A spike sticky's yarn to open questions is the resolution attribution.
Graduation mints a spike-flagged stub carrying the question ids it
resolves under the round-5.4 fail-closed grammar. One spike answering
many questions is normal; a question claimed by multiple spikes renders
a soft smell — an observation, never a rule.

## AC-1

A spike sticky's yarn to open questions records resolution attribution,
and graduation mints the spike stub — slug, spike: true, resolves: [the
yarn'd oq-ids] — in the feature's frontmatter, surviving into the spec
register. Evidence: behavioral (e2e 32's "spike stickies → resolution
yarn → spike stubs" spec; the graduation action tests; the artifact
decode suite's spike-annotation cases).

## AC-2

One spike may answer many questions; a question claimed by two or more
spikes renders the soft smell on the wall — an observation, never a
refusal or a lint error (the owner's round-5.4 cardinality ruling).
Evidence: behavioral (the render suite's smell test; e2e 32's two-claims
spec).

## AC-3

The round-5.4 grammar fails closed: resolves requires spike: true, a
spike stub declares resolves and no acceptance_criteria, a plain stub
the reverse — and a resolves entry naming an undeclared open question of
the same spec is a named lint refusal (the VL-006 sibling check the
parent's dc-4 promised). Evidence: static (the strict-decoded stub
schema) and behavioral (the artifact negative-decode cases and VL-006's
spike-stub-resolves fixtures).

## DC-1

Retro-decomposition, disclosed: minted after its behavior merged (ledger
R4-I-87); this spec describes the built slice and invents nothing new.

## DC-2

Attribution graduates into the stub itself (parent dc-2) — a
spike-flagged stub carrying the oq-ids, one flag-discriminated list
(parent dc-4) — and the multi-claim smell is the owner's cardinality
ruling ratified in round 5.4: a norm-level observation, never an error.

## CO-1

No network in any test: attribution, graduation, the smell, and the
fail-closed grammar are exercised by artifact decode tests, the VL-006
sibling-check fixtures, workbench render tests, and hermetic Playwright
specs, all under make verify.
