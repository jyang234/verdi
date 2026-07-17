---
id: {{.Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{.Owners}}
class: feature{{if .StoryRef}}
story: {{.StoryRef}}{{end}}
status: draft
problem: { text: "{{.Problem}}", anchor: problem }
outcome: { text: "{{.Outcome}}", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static, attestation], anchor: ac-1 }
stubs:
  - { slug: todo-replace-stub-slug, acceptance_criteria: [ac-1] }
---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.

## Ac 1

TODO: design notes.
