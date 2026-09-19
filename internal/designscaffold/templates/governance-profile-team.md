---
schema: verdi.governance-profile/v1
id: {{safe .ProfileID}}
class: team
applicable_transitions: [accept, policy-disposition-approval]
identity_trust_sources:
  - {id: forge, kind: forge}
role_mappings: []
ownership_sources: []
signature_requirements: []
required_approvers:
  - {transitions: [accept], roles: [reviewer], minimum: 1}
  - {transitions: [policy-disposition-approval], roles: [policy-owner], minimum: 1}
distinctness_rules:
  - {transitions: [accept], left_role: author, right_role: reviewer, relation: different-principal}
  - {transitions: [policy-disposition-approval], left_role: author, right_role: policy-owner, relation: different-principal}
evidence_source_restrictions: []
escalation_thresholds: []
template: {identity: {{printf "%q" .TemplateIdentity}}, digest: {{printf "%q" .TemplateDigest}}}
---
Starter team profile: acceptance needs one reviewer who is not the author,
and a policy disposition needs a policy owner who is not its author, all
authenticated through the forge. It maps no subjects yet — this file names
no one — so every role resolves unproven until the team adds its own
role_mappings through the project's review process. Nothing here is a
fabricated identity.
