---
name: good-skill
description: Deliberately clean fixture skill for internal/specalign's clean-direction proof (spec/instruction-conformance ac-4). Never used by a real agent; committed testdata only.
---

# good-skill (fixture only)

This fixture models a correctly-scoped rewrite: real, recognized CLI verbs
only, and the retired ritual discussed honestly rather than taught as
current.

## Real verbs

Run `verdi lint` before every commit, and check `verdi audit` for
exemption drift.

## Historical note

Earlier versions of this skill instructed you to run
`verdi board commit <board-key> --name <spec-name>` to finish a spec. That
two-phase ritual is retired now; board editing on a design branch is spec
editing, with no separate commit step.
