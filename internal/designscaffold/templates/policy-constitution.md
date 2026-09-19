---
schema: verdi.policy-constitution/v1
id: policy-constitution/constitution
kind: policy-constitution
title: {{printf "%q" .Title}}
owners: [{{range $i, $o := .Owners}}{{if $i}}, {{end}}{{safe $o}}{{end}}]
selected_profile: {{safe .ProfileID}}
environments: [local]
catalog:
  roles: [author, reviewer, policy-owner]
  transitions: [accept, policy-disposition-approval]
  evidence_sources: []
  escalation_metrics: []
subjects:
  action: []
  configuration: []
  capability: []
  resource: []
  identity: []
  evidence: []
adapters: []
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Starter constitution written by `verdi policy adopt --starter`. It selects
the {{safe .ProfileID}} governance profile and registers the minimal
catalog that profile needs: three roles, the accept and
policy-disposition-approval transitions, one local environment, no
constraint subjects, and no harness adapters. Add subjects, environments,
and adapters through the project's own review process; acceptance of this
file is the owner's merge to the default branch.
