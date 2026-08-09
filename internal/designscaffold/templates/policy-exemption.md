---
schema: verdi.policy-exemption/v1
id: policy-exemption/{{.Name}}
kind: policy-exemption
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
scope: {phases: [], environments: [], paths: [], refs: []}
witnesses:
  - policy: {{safe .WitnessPolicy}}
    claim: {{safe .WitnessClaim}}
    claim_digest: {{printf "%q" .WitnessClaimDigest}}
compensating_controls:
  - "TODO: replace with a real compensating control before accept."
approvals:
  - role: {{safe .ApprovalRole}}
    principal: {{safe .ApprovalPrincipal}}
expiry: {{printf "%q" .Expiry}}
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
TODO: replace with the real rationale before accept.
