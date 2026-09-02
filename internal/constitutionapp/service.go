package constitutionapp

// Service is the sole application consumer for the Constitution's five
// operations (doc.go). Every port has a production default wired by
// NewService; tests substitute fakes/fixturegit-backed adapters at each
// named port, never by reimplementing an owner's algorithm inside this
// package.
type Service struct {
	// Git resolves every raw Git identity/branch/commit primitive an
	// operation needs (identity.go).
	Git GitReader

	// Authority is the one loader+resolver over the constitution store
	// (authority.go), composing internal/policyauthority unchanged.
	Authority AuthorityStore

	// Conflict compiles the exact accepted-context manifest and runs mechanical
	// and semantic conflict evaluation over the same request through the one
	// existing compiler and conflict gate (conflict.go).
	Conflict ConflictEvaluator
}

// NewService returns the production Service: real Git primitives, the real
// policyauthority loader/resolver, and the real policyconflict evaluator.
func NewService() Service {
	return Service{
		Git:       gitxReader{},
		Authority: policyauthorityStore{},
		Conflict:  localConflictEvaluator{},
	}
}
