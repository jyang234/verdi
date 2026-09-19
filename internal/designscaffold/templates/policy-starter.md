---
schema: verdi.policy/v1
id: policy/{{.Name}}
kind: policy
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
scope: {phases: [], environments: [], paths: [], refs: []}
claims: []
instructions: []
payloads:
  design_assistance: {mode: {{safe .DesignAssistanceMode}}, layout: false}
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Starter policy: one real rule and nothing else. The design_assistance
payload sets mode {{safe .DesignAssistanceMode}} — what delegated agents
may do on a design branch. No constraint claims, no instruction lines, and
no other payloads are declared; add them through the project's own review
process. Acceptance of this policy is the owner's merge to the default
branch.
