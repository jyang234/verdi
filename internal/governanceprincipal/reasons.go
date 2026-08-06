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
)
