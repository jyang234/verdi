package governanceprincipal

// Stable reason codes. Every kernel finding, witness, and disclosure
// carries one of these closed constants; human detail may accompany a
// code but never replaces it.
const (
	// Principal-resolution witnesses.
	ReasonTrustSourceForbidden     = "trust-source-forbidden"
	ReasonTrustEvidenceInvalid     = "trust-evidence-invalid"
	ReasonTrustEvidenceUnavailable = "trust-evidence-unavailable"
	ReasonTrustSubjectMismatch     = "trust-subject-mismatch"
	// ReasonTrustSubjectVerified is the positive witness an authenticated
	// resolution carries: the claimed subject was observed in valid
	// evidence, with the evidence digest recorded.
	ReasonTrustSubjectVerified = "trust-subject-verified"

	// Authorization findings.
	ReasonTransitionNotApplicable = "transition-not-applicable"
	ReasonPrincipalViolated       = "principal-violated"
	ReasonPrincipalUnproven       = "principal-unproven"
	ReasonRoleNotAuthorized       = "role-not-authorized"
	ReasonRequiredApproverMissing = "required-approver-missing"
	ReasonDistinctnessViolated    = "distinctness-violated"
	// ReasonDistinctnessUnproven marks an applicable distinctness rule
	// with an unfilled role side: the relation cannot be evaluated, so it
	// is unproven, never vacuously satisfied.
	ReasonDistinctnessUnproven    = "distinctness-unproven"
	ReasonSignatureViolated       = "signature-violated"
	ReasonSignatureUnproven       = "signature-unproven"
	ReasonOwnershipViolated       = "ownership-violated"
	ReasonOwnershipUnproven       = "ownership-unproven"
	ReasonEvidenceSourceForbidden = "evidence-source-forbidden"
	ReasonEvidenceSourceUnproven  = "evidence-source-unproven"
	ReasonEscalationRoleMissing   = "escalation-role-missing"
	// ReasonEscalationMetricUnavailable marks an applicable escalation
	// threshold whose metric value was not supplied: the threshold cannot
	// be evaluated, so the decision is unproven, never a silent pass.
	ReasonEscalationMetricUnavailable    = "escalation-metric-unavailable"
	ReasonExperimentalAuthorityForbidden = "experimental-authority-forbidden"

	// Disclosures.
	ReasonSoloRoleCollapse = "solo-role-collapse"
)
