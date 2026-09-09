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

// Every REQUEST envelope is versioned too, and the version travels ON THE
// WIRE rather than only in a doc comment. A request document is the input
// half of a public, versioned contract: with no schema field, a future v2
// request and a v1 request are the same bytes to a v1 decoder, so a caller
// (or an agent composing one) is silently misread instead of refused. Each
// Decode*Request requires an EXACT match — a missing, empty, or differing
// value is refused, never defaulted, since defaulting is precisely the silent
// misreading the field exists to prevent.
//
// The requirement is a WIRE requirement. A Go caller constructing a request
// value in process (internal/mcpserve building an empty InspectRequest; a
// test table) is not decoding a document and is not subject to it — exactly
// as a result envelope's own Schema is set by the producer rather than
// demanded of it.
const (
	InspectRequestSchema           = "verdi.constitution-inspect-request/v1"
	ProposeRequestSchema           = "verdi.constitution-propose-request/v1"
	ValidateRequestSchema          = "verdi.constitution-validate-request/v1"
	ImpactReviewRequestSchema      = "verdi.constitution-impact-review-request/v1"
	SubmitPreparationRequestSchema = "verdi.constitution-submit-preparation-request/v1"
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
