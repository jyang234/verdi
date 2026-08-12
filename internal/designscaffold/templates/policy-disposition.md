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
    - id: {{safe .ClaimID}}
      digest: {{printf "%q" .ClaimDigest}}
      category: {{safe .Category}}
      authority_digest: {{printf "%q" .AuthorityDigest}}
      scope: {phases: [], environments: [], paths: [], refs: []}
      values: []
  exemptions: []
conclusion: no-conflict
origin: judge-result
approvals:
  - role: {{safe .ApprovalRole}}
    principal: {{safe .ApprovalPrincipal}}
expiry: {{printf "%q" .Expiry}}
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
TODO: replace with the real rationale before accept.
