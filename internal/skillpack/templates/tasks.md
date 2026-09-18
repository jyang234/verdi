---
name: verdi-tasks
description: Read a spec's tasks document — plan, evidence, and readiness — and turn it into a work list for the human without writing anything to the store.
---

# verdi-tasks

Use this skill when the user asks what to work on next for a spec. It reads and never writes: no `mutate_draft`, no `add_annotation`, no import.

## Steps

1. Call `get_document` with `ref` `spec/<slug>` and `kind` `tasks`, adding `proposed` true when the spec is a draft on its design branch (omit it for an accepted spec).
2. From the plan section, list each stub with the criteria it covers. From the evidence section, note each criterion's evidence state and what is still unproven. From the readiness section, list the concerns that need attention, blocking ones first; if readiness was not supplied for this render, say so.
3. Present a work list in this order: blocking readiness concerns, uncovered criteria (nothing covers them), stubs whose criteria have no evidence yet, then everything else. Quote ids so the human can find each item on the board.
4. If the human wants any of it changed, hand off to verdi-clarify or verdi-plan; this skill writes nothing.

```verdi-sequence
call get_document kind=tasks proposed=true
show
```
