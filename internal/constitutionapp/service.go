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

	// Conflict runs mechanical and semantic conflict evaluation over one
	// governed target through the one existing conflict gate (conflict.go),
	// composing internal/policyconflict unchanged.
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
