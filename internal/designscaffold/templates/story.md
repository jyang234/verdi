---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: story
status: draft
story: {{safe .StoryRef}}
{{if .Spike}}spike: true
{{end}}problem: { text: {{printf "%q" .Problem}}, anchor: problem }
outcome: { text: {{printf "%q" .Outcome}}, anchor: outcome }
{{if not .Spike}}acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static], anchor: ac-1 }
{{end}}links:
{{range .Links}}  - { type: {{.Type}}, ref: {{printf "%q" .Ref}} }
{{end}}---
# {{.Title}}

## Problem

TODO: design notes.

## Outcome

TODO: design notes.
{{if not .Spike}}
## Ac 1

TODO: design notes.
{{end}}