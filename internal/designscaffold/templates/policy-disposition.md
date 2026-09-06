---
schema: verdi.policy-disposition/v1
id: policy-disposition/{{.Name}}
kind: policy-disposition
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
scope: {phases: [], environments: [], paths: [], refs: []}
witness:
  input_id: {{printf "%q" .InputID}}
  target_digest: {{printf "%q" .TargetDigest}}
  claims:
{{range .Claims}}    - id: {{safe .ID}}
      digest: {{printf "%q" .Digest}}
      category: {{safe .Category}}
      authority_digest: {{printf "%q" .AuthorityDigest}}
      scope: {phases: [{{range $i, $v := .Scope.Phases}}{{if $i}}, {{end}}{{safe $v}}{{end}}], environments: [{{range $i, $v := .Scope.Environments}}{{if $i}}, {{end}}{{safe $v}}{{end}}], paths: [{{range $i, $v := .Scope.Paths}}{{if $i}}, {{end}}{{safe $v}}{{end}}], refs: [{{range $i, $v := .Scope.Refs}}{{if $i}}, {{end}}{{safe $v}}{{end}}]}
      values: []
{{end}}  exemptions: [{{range $i, $e := .Exemptions}}{{if $i}}, {{end}}{id: {{printf "%q" $e.ID}}, digest: {{printf "%q" $e.Digest}}}{{end}}]
conclusion: {{.Conclusion}}
origin: {{.Origin}}
{{if .CompensatingControls}}compensating_controls:
{{range .CompensatingControls}}  - {{printf "%q" .}}
{{end}}{{end}}approvals:
{{range .Approvals}}  - role: {{safe .Role}}
    principal: {{safe .Principal}}
{{end}}expiry: {{printf "%q" .Expiry}}
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
TODO: replace with the real rationale before accept.
