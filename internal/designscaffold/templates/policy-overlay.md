---
schema: verdi.policy-overlay/v1
id: policy-overlay/{{.Name}}
kind: policy-overlay
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
refines: {{safe .RefinesPolicy}}
scope: {phases: [], environments: [], paths: [], refs: []}
refinements:
  - claim: {{safe .ClaimName}}
    values: ["placeholder-value"]
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
TODO: replace with the real rationale before accept.
