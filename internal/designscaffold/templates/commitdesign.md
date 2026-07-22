---
id: {{safe .Ref}}
kind: spec
title: {{printf "%q" .Title}}
owners: {{safe .Owners}}
class: feature
status: draft
story: {{safe .StoryRef}}
{{if .Pins}}context:
{{range .Pins}}  - {{.Ref}}
{{end}}{{end}}acceptance_criteria:
  - { id: ac-1, text: "TODO: replace with real acceptance criteria before accept", evidence: [static] }
{{if .Dispositions}}dispositions:
{{range .Dispositions}}  - { sticky: {{.Sticky}}, disposition: {{.Disposition}} }
{{end}}{{end}}---
# {{.Title}}

TODO: design notes.

Drafted by commit-to-design from board {{printf "%q" .Ref}}. Every board sticky above is
carried as `open-question` until the commit-to-design skill (or a human)
promotes it to `incorporated` or `contradicted` (I-5).
