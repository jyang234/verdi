package contextcompile

import (
	"context"
	"errors"
	"fmt"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyauthority"
	"github.com/jyang234/verdi/internal/specstate"
)

// GitReader is the compiler's read-only Git port. Its methods expose only
// exact object reads and candidate discovery; no generic command runner or
// working-tree content reader crosses the compiler boundary.
type GitReader interface {
	Show(ctx context.Context, root, ref, path string) ([]byte, error)
	LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error)
	WorktreeChangedPaths(ctx context.Context, root string) ([]string, error)
}

// StateResolver projects exact candidate bytes through the one merge-signaled
// specification-state seam.
type StateResolver interface {
	Resolve(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
}

// AuthorityLoader loads and resolves the one sealed constitution authority.
type AuthorityLoader interface {
	Load(root string) (*policyauthority.Store, error)
	Resolve(store *policyauthority.Store) (*policyauthority.EffectivePolicy, error)
}

// ActorResolver supplies already-sealed governance-principal resolutions.
// The compiler records their exported projection; it never reconstructs a
// kernel seal from manifest data.
type ActorResolver interface {
	Resolutions(ctx context.Context) ([]governanceprincipal.PrincipalResolution, error)
}

type defaultAuthorityLoader struct{}

func (defaultAuthorityLoader) Load(root string) (*policyauthority.Store, error) {
	return policyauthority.Load(root)
}

func (defaultAuthorityLoader) Resolve(store *policyauthority.Store) (*policyauthority.EffectivePolicy, error) {
	return policyauthority.Resolve(store)
}

var (
	ErrNoConstitution             = errors.New("contextcompile: constitution not adopted")
	ErrAdapterMismatch            = errors.New("contextcompile: adapter mismatch")
	ErrAcceptedSpec               = errors.New("contextcompile: accepted specification required")
	ErrExpectedRepositoryMismatch = errors.New("contextcompile: expected repository mismatch")
	ErrDeclaredScope              = errors.New("contextcompile: declared phase or scope refusal")
	ErrProjectionDrift            = errors.New("contextcompile: instruction projection drift")
)

type contextRefusal interface {
	error
	contextRefusal()
}

// IsRefusal reports whether err is one of the closed exit-1 state-refusal
// families. Malformed input, invalid authority and broken ports return false.
func IsRefusal(err error) bool {
	var refusal contextRefusal
	return errors.As(err, &refusal)
}

// NoConstitutionRefusal reports explicit invocation against a store that has
// not adopted constitution authority.
type NoConstitutionRefusal struct{}

func (*NoConstitutionRefusal) Error() string   { return ErrNoConstitution.Error() }
func (*NoConstitutionRefusal) Unwrap() error   { return ErrNoConstitution }
func (*NoConstitutionRefusal) contextRefusal() {}

// AdapterMismatchRefusal reports a requested adapter pair absent from the
// resolved constitution. Registered is sorted by (id,version).
type AdapterMismatchRefusal struct {
	Requested  AdapterRef
	Registered []AdapterRef
}

func (e *AdapterMismatchRefusal) Error() string {
	return fmt.Sprintf("%s: requested %s/%s; registered %v", ErrAdapterMismatch, e.Requested.ID, e.Requested.Version, e.Registered)
}
func (*AdapterMismatchRefusal) Unwrap() error   { return ErrAdapterMismatch }
func (*AdapterMismatchRefusal) contextRefusal() {}

// AcceptedSpecRefusal reports a target or governing parent that lacks the
// merge-signaled accepted baseline required by the requested phase.
type AcceptedSpecRefusal struct {
	Ref         string
	State       specstate.State
	Relation    specstate.Relation
	Disclosures []string
}

func (e *AcceptedSpecRefusal) Error() string {
	return fmt.Sprintf("%s: %s is %s/%s (%v)", ErrAcceptedSpec, e.Ref, e.State, e.Relation, e.Disclosures)
}
func (*AcceptedSpecRefusal) Unwrap() error   { return ErrAcceptedSpec }
func (*AcceptedSpecRefusal) contextRefusal() {}

// ExpectedRepositoryMismatchRefusal reports an optional caller expectation
// that differs from computed repository facts. Expected never substitutes for
// ComputedBranch or ComputedHead.
type ExpectedRepositoryMismatchRefusal struct {
	Expected       Expected
	ComputedBranch string
	ComputedHead   string
	BranchKnown    bool
	HeadKnown      bool
}

func (e *ExpectedRepositoryMismatchRefusal) Error() string {
	return fmt.Sprintf("%s: expected branch=%q HEAD=%q; computed branch=%q known=%t HEAD=%q known=%t",
		ErrExpectedRepositoryMismatch, e.Expected.Branch, e.Expected.Head,
		e.ComputedBranch, e.BranchKnown, e.ComputedHead, e.HeadKnown)
}
func (*ExpectedRepositoryMismatchRefusal) Unwrap() error {
	return ErrExpectedRepositoryMismatch
}
func (*ExpectedRepositoryMismatchRefusal) contextRefusal() {}

// DeclaredScopeRefusal reports a state-valid target that the requested phase
// may not consume (for example, a feature as an authoritative build target).
type DeclaredScopeRefusal struct {
	Phase  Phase
	Ref    string
	Reason string
}

func (e *DeclaredScopeRefusal) Error() string {
	return fmt.Sprintf("%s: phase=%q ref=%q: %s", ErrDeclaredScope, e.Phase, e.Ref, e.Reason)
}
func (*DeclaredScopeRefusal) Unwrap() error   { return ErrDeclaredScope }
func (*DeclaredScopeRefusal) contextRefusal() {}

// ProjectionDriftRefusal reports managed instruction projection paths whose
// checked-in bytes do not match the one canonical renderer.
//
// Reasons carries the sorted, de-duplicated closed
// instructionprojection.Reason codes the verification report witnessed
// (authority design §10: "Existing generated projection drift | Exit-1 typed
// refusal with closed projection reason"). Paths may legally be empty — a
// walk- or manifest-level finding names no managed file — but Reasons and
// Paths are never BOTH empty: a refusal with no witness at all is not
// constructed (compiler.go's driftWitness fails closed operationally
// instead).
type ProjectionDriftRefusal struct {
	Paths   []string
	Reasons []string
}

func (e *ProjectionDriftRefusal) Error() string {
	return fmt.Sprintf("%s: reasons %v at %v", ErrProjectionDrift, e.Reasons, e.Paths)
}
func (*ProjectionDriftRefusal) Unwrap() error   { return ErrProjectionDrift }
func (*ProjectionDriftRefusal) contextRefusal() {}

// PhaseScopeRefusal is declared in validate.go because request validation
// produces it before any compiler port is called; this marker keeps it in the
// same closed IsRefusal family without coupling the request codec to this file.
func (*PhaseScopeRefusal) Unwrap() error   { return ErrDeclaredScope }
func (*PhaseScopeRefusal) contextRefusal() {}
