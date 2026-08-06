package governanceprincipal

import "testing"

// TestReasonCodesAreClosedConstants pins every stable reason code to its
// exact wire value: consumers key on these strings, so a drifting value
// is a breaking change this test makes visible.
func TestReasonCodesAreClosedConstants(t *testing.T) {
	codes := []string{
		ReasonTrustSourceForbidden,
		ReasonTrustEvidenceInvalid,
		ReasonTrustEvidenceUnavailable,
		ReasonTrustSubjectMismatch,
		ReasonTrustSubjectVerified,
		ReasonTransitionNotApplicable,
		ReasonPrincipalViolated,
		ReasonPrincipalUnproven,
		ReasonRoleNotAuthorized,
		ReasonRequiredApproverMissing,
		ReasonDistinctnessViolated,
		ReasonSignatureViolated,
		ReasonSignatureUnproven,
		ReasonOwnershipViolated,
		ReasonOwnershipUnproven,
		ReasonEvidenceSourceForbidden,
		ReasonEvidenceSourceUnproven,
		ReasonEscalationRoleMissing,
		ReasonEscalationMetricUnavailable,
		ReasonExperimentalAuthorityForbidden,
		ReasonSoloRoleCollapse,
	}
	want := []string{
		"trust-source-forbidden",
		"trust-evidence-invalid",
		"trust-evidence-unavailable",
		"trust-subject-mismatch",
		"trust-subject-verified",
		"transition-not-applicable",
		"principal-violated",
		"principal-unproven",
		"role-not-authorized",
		"required-approver-missing",
		"distinctness-violated",
		"signature-violated",
		"signature-unproven",
		"ownership-violated",
		"ownership-unproven",
		"evidence-source-forbidden",
		"evidence-source-unproven",
		"escalation-role-missing",
		"escalation-metric-unavailable",
		"experimental-authority-forbidden",
		"solo-role-collapse",
	}
	if len(codes) != len(want) {
		t.Fatalf("code list length %d != expected %d", len(codes), len(want))
	}
	for i, w := range want {
		if codes[i] != w {
			t.Errorf("reason constant %d = %q, want %q", i, codes[i], w)
		}
	}
}
