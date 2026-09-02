package constitutionapp

// The ONE place every constitutionapp result envelope's version constant is
// declared (mirrors internal/designapp/schema.go's own convention). Each
// operation returns exactly one envelope carrying exactly one of these
// constants in its own `schema` field.
const (
	InspectResultSchema           = "verdi.constitution-inspect/v1"
	ProposeResultSchema           = "verdi.constitution-propose/v1"
	ValidateResultSchema          = "verdi.constitution-validate/v1"
	ImpactReviewResultSchema      = "verdi.constitution-impact-review/v1"
	SubmitPreparationResultSchema = "verdi.constitution-submit-preparation/v1"

	// FailureSchema versions the ONE typed failure envelope every
	// constitutionapp operation's application failure projects into
	// (outcome.go's Failure).
	FailureSchema = "verdi.constitution-failure/v1"
)

// Artifact kind identifiers Propose accepts, restated from
// internal/policyartifact's own kernel constants (kernel.go's KindPolicy/
// KindOverlay/KindExemption) so callers of this package's wire request never
// need to import policyartifact themselves for these three literal strings.
// The values are byte-identical to policyartifact's own constants; Validate
// always dispatches through policyartifact.ClassifyPolicyPath and
// policyartifact.Decode*, never a parallel enum interpretation.
const (
	KindPolicy    = "policy"
	KindOverlay   = "policy-overlay"
	KindExemption = "policy-exemption"
)
