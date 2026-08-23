package experiment

import "fmt"

// StateDisclosureCode is the CLOSED vocabulary of authority conjuncts
// DeriveState reports as disclosed-unproven beside the state it derived
// (invention ledger SI-44). Each code names a fact AC-1's state table
// depends on that a filesystem reader structurally cannot establish from
// artifact bytes — CO-1 forbids assuming any of them silently, and equally
// forbids pretending the rung below is what the artifacts say.
//
// It is closed for the same reason every other vocabulary in this package
// is: an unknown code fails closed, so a consumer can never encounter a
// disclosure it has no handling for and treat it as noise. Exactly three
// codes are registered today; a fourth requires an explicit revision.
type StateDisclosureCode string

// The three registered disclosure codes.
const (
	// DisclosureRegistrationLockWitness is emitted at every rung from
	// registered upward. Locked (normalize.go) proves the lock block's
	// digest matches the definition it pins, which is an INTEGRITY fact;
	// AC-5 additionally requires a HUMAN moment behind that lock, witnessed
	// in solo mode by the pull-request merge — a git-layer transport fact
	// no bytes inside the experiment directory record. Wave-6's lock
	// surfaces and profile governance own converting this into proof or
	// refusal.
	DisclosureRegistrationLockWitness StateDisclosureCode = "registration-lock-human-witness"

	// DisclosureResultEnvironmentReceipt remains on V1 predecessor results:
	// they carry no durable receipt capable of proving the registered
	// environment-policy conjunct. A verified V2 result binds execution.json
	// and therefore does not emit this disclosure.
	DisclosureResultEnvironmentReceipt StateDisclosureCode = "result-environment-policy-receipt"

	// DisclosureRatificationActorResolution is emitted only at the ratified
	// rung. Ratification.Validate proves the actor field is a canonical
	// kernel principal id (grammar); it does not RESOLVE that id to an
	// authenticated principal, which OD-4 requires and which needs the
	// governance kernel's persisted-resolution seam SI-21 explicitly
	// defers. Wave-5's adapters own that resolution.
	DisclosureRatificationActorResolution StateDisclosureCode = "ratification-actor-principal-resolution"
)

// Validate fails closed on any disclosure code outside the vocabulary.
func (c StateDisclosureCode) Validate() error {
	switch c {
	case DisclosureRegistrationLockWitness, DisclosureResultEnvironmentReceipt, DisclosureRatificationActorResolution:
		return nil
	}
	return fmt.Errorf("experiment: unknown state disclosure code %q", string(c))
}

// StateDisclosure is one disclosed-unproven authority conjunct attached to
// a derived state: the code identifying WHICH conjunct, and a fixed detail
// saying why this layer cannot prove it and who owns proving it.
//
// Detail is a constant string, never a rendering of anything read from
// disk or the clock, so two derivations over the same bytes return equal
// disclosures (the determinism every output in this package holds to).
type StateDisclosure struct {
	Code   StateDisclosureCode
	Detail string
}

// Validate checks the code is registered and the detail is nonempty — a
// disclosure that names no reason discloses nothing.
func (d StateDisclosure) Validate() error {
	if err := d.Code.Validate(); err != nil {
		return err
	}
	if err := nonemptyString(fmt.Sprintf("state disclosure %q: detail", d.Code), d.Detail); err != nil {
		return err
	}
	return nil
}

// lockWitnessDisclosure, environmentReceiptDisclosure, and
// actorResolutionDisclosure are the three fixed disclosure values
// DeriveState emits. They are functions rather than package-level
// variables so no caller can mutate the shared value a later derivation
// would return.
func lockWitnessDisclosure() StateDisclosure {
	return StateDisclosure{
		Code: DisclosureRegistrationLockWitness,
		Detail: "the lock block's digest matches the registered definition, but the human moment AC-5 requires behind it " +
			// vocab:identity — "merge" names the git pull-request merge event AC-5 fixes as the solo-mode transport witness, a mechanism identity, not renameable display vocabulary.
			"is witnessed by the pull-request merge, a git-layer fact these artifact bytes cannot exhibit",
	}
}

// environmentReceiptDisclosure makes the V1 predecessor's missing durable
// execution proof visible at rest.
func environmentReceiptDisclosure() StateDisclosure {
	return StateDisclosure{
		Code: DisclosureResultEnvironmentReceipt,
		Detail: "the verdict rests on AC-2 step 1's environment-policy conjunct as well as the locked digests, but no " +
			"artifact at rest records the environment this run executed in; the execution unit's durable receipt owns proving it",
	}
}

func actorResolutionDisclosure() StateDisclosure {
	return StateDisclosure{
		Code: DisclosureRatificationActorResolution,
		Detail: "the ratification actor is a well-formed canonical principal id, but resolving it to an authenticated " +
			"principal (OD-4) needs the governance kernel's persisted-resolution seam, which SI-21 defers to a later unit",
	}
}
