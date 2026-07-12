---
id: spec/disclosure-legibility
kind: spec
title: "Disclosure Legibility"
owners: [platform-team]
class: feature
status: draft
problem: { text: "verdi's own disclosed-unproven states — VL-017's mutable-zone-absent notice, align's synthetic-absence findings, evidence-model's three-valued honesty markers — surface scattered across CLI stderr, lint output, and alignment reports in ad hoc wording, so an operator cannot answer \"what is verdi currently not proving, in total?\" without grepping several surfaces by hand and reading each one's own phrasing", anchor: "#problem" }
outcome: { text: "an operator can see, in one vocabulary and one place, every claim verdi is currently not proving", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "every disclosed-unproven state verdi emits reads in one consistent vocabulary wherever it appears", evidence: [behavioral, attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "an operator can enumerate every current disclosure for a checkout in one view", evidence: [behavioral, attestation], anchor: "#ac-2" }
decisions:
  - { id: dc-1, text: "disclosures are a first-class rendered state, not log lines", anchor: "#dc-1" }
open_questions:
  - { id: oq-1, text: "should disclosures be machine-enumerable (MCP/audit surface), and what belongs in the enumeration?", anchor: "#oq-1" }
stubs:
  - { slug: disclosure-seam, acceptance_criteria: [ac-1] }
  - { slug: disclosures-panel, acceptance_criteria: [ac-1, ac-2] }
---
# Disclosure Legibility

## Problem

verdi's own constitution requires three-valued honesty: every claim is
proven, violated-with-witness, or disclosed-as-unproven, and silence is
never a pass (constitution 2). The system already produces disclosures that
honor this — VL-017's printed notice when the mutable zone is absent in a
CI clone, `align`'s synthetic-absence finding when no judge is configured,
evidence-model's advisory/`--preview` markers, waiver expiry notices, and
more to come as the system grows. But each of these was invented at its own
call site, in its own words, discoverable only by whoever happens to be
staring at that particular command's output at that particular moment.

There is no single vocabulary an operator can learn once and recognize
everywhere, and no single place to go stand and ask "what, right now, is
this checkout honestly *not* proving?" A reviewer approving a merge, an
auditor doing a periodic sweep, or an agent deciding whether to trust a
result all have to reconstruct that answer by hand, surface by surface —
exactly the silent-failure shape the constitution's three-valued honesty
rule exists to close off at the level of any *one* claim, but that nothing
currently closes off at the level of the checkout as a whole.

## Outcome

An operator can see, in one vocabulary and one place, every claim verdi is
currently not proving. "One vocabulary" means every disclosure — regardless
of which verb or lint rule produced it — is rendered through the same
phrasing convention, so recognizing one teaches you to recognize all of
them. "One place" means there is a single view (see ac-2) that enumerates
the checkout's current disclosures rather than requiring a tour of every
surface that might emit one.

This feature does not invent new things to disclose; it gives the
disclosures the system already produces (and will keep producing as it
grows) a consistent face and a single point of assembly.

## AC-1

Every disclosed-unproven state verdi emits reads in one consistent
vocabulary wherever it appears — CLI stderr, lint findings, alignment
reports, and any future surface. A reader who has learned to recognize one
disclosure recognizes all of them, without needing to know which internal
component produced it.

Evidence: behavioral (an exerciser confirms the shared rendering path is
actually used, not merely defined) and attestation (an operator affirms the
vocabulary reads as consistent in practice, not just in the code that
renders it).

## AC-2

An operator can enumerate every current disclosure for a checkout in one
view, without knowing in advance which commands or lint rules might have
produced one. The enumeration reflects the checkout's current state, not a
historical log.

Evidence: behavioral (an exerciser confirms the view actually surfaces
disclosures a running checkout emits) and attestation (an operator affirms
the enumerated view is where they'd actually look, and that nothing they
know to be currently disclosed is missing from it).

## DC-1

Disclosures are a first-class rendered state, not log lines. A disclosure
is not merely something printed once to stderr and then gone; it is a
recognizable, structured state — carrying at minimum what is unproven and
why — that can be rendered consistently (ac-1) and collected into an
enumeration (ac-2). Treating disclosures as an ephemeral logging concern
would make ac-2 impossible to build honestly: you cannot enumerate what was
never structured enough to collect. This decision is why the two stubs
below split along a seam (disclosure-seam) and a presentation surface
(disclosures-panel) rather than shipping as one undifferentiated change:
the rendered-state shape has to exist as a real seam other producers can
call into before any one view can enumerate through it.

## OQ-1

Should disclosures be machine-enumerable (MCP/audit surface), and what
belongs in the enumeration? A human-facing panel (ac-2) is the outcome
floor this feature commits to, but agents acting on this checkout have the
same "what is currently unproven" question a human reviewer does, and MCP
already ships read tools for comparable surfaces (`list_annotations`,
`get_matrix`). Whether disclosure-legibility should grow a machine-readable
form, and if so what the enumeration's item shape should carry (source
verb, disclosure text, severity, a stable id?), is open. This is the
spike's target (see the round-5 protocol's phase B): the spike's answer
lands in story-2's spec and its own record, never as an amendment to this
frozen feature spec.
