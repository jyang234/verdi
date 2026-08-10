package experimentdecision

import "github.com/jyang234/verdi/internal/experiment"

// EnvironmentAttestation is the EXECUTION LAYER's assertion, to this
// engine, that the run whose observations it is about to judge was
// executed under the locked definition's registered
// execution.environment_policy — AC-2 step 1's environment-policy
// conjunct, which no observation record carries a fingerprint for
// (invention ledger SI-41).
//
// It is an in-memory contract between the execution unit and this engine,
// not durable evidence. What it buys today is that the conjunct is never
// SILENTLY assumed: a caller with nothing to attest cannot obtain a
// verdict at all, because a zero or mismatched attestation is an
// operational error (CO-1, CO-6) rather than a verdict of any kind. The
// durable execution receipt that turns the assertion into checkable
// evidence belongs to spec/execution-workspace (Wave 3); at rest, the
// conjunct stays disclosed-unproven until that unit lands (SI-43).
//
// The zero value is deliberately useless: an empty PolicyID attests
// nothing.
type EnvironmentAttestation struct {
	// PolicyID is the environment-policy identifier the execution layer
	// enforced for the run. It must equal the locked definition's
	// execution.environment_policy EXACTLY; neither this package nor
	// internal/experiment interprets the identifier's content beyond that
	// equality (execution.environment_policy is a nonempty opaque id).
	PolicyID string
}

// verify checks att against def's registered environment policy. An empty
// PolicyID and a PolicyID naming any other policy are both operational
// errors: the first attests nothing, the second attests the wrong thing,
// and neither is a fact about the comparison the run produced.
func (att EnvironmentAttestation) verify(def experiment.Definition) error {
	if att.PolicyID == "" {
		return errf("environment-policy attestation is empty: definition %q registers environment policy %q, which the execution layer must attest to (AC-2 step 1)",
			def.ID, def.Execution.EnvironmentPolicy)
	}
	if att.PolicyID != def.Execution.EnvironmentPolicy {
		return errf("environment-policy attestation names policy %q, but definition %q registers %q",
			att.PolicyID, def.ID, def.Execution.EnvironmentPolicy)
	}
	return nil
}
