package supersede

import (
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
)

const (
	// ReasonInvalidName means the proposed successor name does not parse
	// as a spec ref (02 §Identity and references: kebab-case, and VL-002's
	// path-bound identity means the name IS the store directory).
	ReasonInvalidName Reason = "invalid-name"
	// ReasonSuccessorExists means the successor's own store directory is
	// already present in the active zone — the collision that must refuse
	// BEFORE anything is written or any branch is cut.
	ReasonSuccessorExists Reason = "successor-exists"
)

// NameError is ValidateSuccessorName's typed refusal, the successor-side
// twin of ResolveError: Reason classifies why, Name carries the rejected
// name, Path names the colliding directory (ReasonSuccessorExists only),
// and Unwrap exposes the underlying ref-parse failure
// (ReasonInvalidName only).
//
// Each caller renders its OWN message from these fields rather than
// relaying Detail verbatim: a CLI names the flag the operator typed
// (`--name`), the workbench names its own form field, and neither has to
// borrow the other's vocabulary to reuse this one check.
type NameError struct {
	Reason Reason
	Name   string
	Path   string
	Detail string
	Err    error
}

func (e *NameError) Error() string { return e.Detail }

func (e *NameError) Unwrap() error { return e.Err }

// ValidateSuccessorName proves the two successor-side preconditions every
// caller of this package shares before composing anything: name parses as
// a spec ref (returned, so the caller need not re-parse it to render the
// successor's canonical ref), and no spec directory of that name already
// exists in root's active zone.
//
// It lives here, beside Resolve's predecessor-side guard, because the
// board's Revise action (W3-C, spec/uat-round-1 ac-11's board half) needs
// exactly these two checks and must not re-implement them: a second copy
// is a second thing to drift, and this one is the seam where both surfaces
// meet — the same "one shared core, two callers" shape this package's own
// doc comment states.
//
// NOT covered here, deliberately: whether a `design/<name>` BRANCH already
// exists. That is a Git question this pure-filesystem check has no context
// or repository handle for, and each caller's own branch-creation
// primitive already owns it — the CLI's gitx.CheckoutNewBranchFrom refuses
// an existing branch before switching. A caller that updates a ref
// directly instead must make that check itself; see this lane's report.
func ValidateSuccessorName(root, name string) (artifact.Ref, error) {
	ref, err := artifact.ParseRef("spec/" + name)
	if err != nil {
		return artifact.Ref{}, &NameError{
			Reason: ReasonInvalidName,
			Name:   name,
			Detail: fmt.Sprintf("supersede: successor name %q is not a valid spec name: %v", name, err),
			Err:    err,
		}
	}

	dir := store.ActiveSpecDir(root, name)
	if _, statErr := os.Stat(dir); statErr == nil {
		return artifact.Ref{}, &NameError{
			Reason: ReasonSuccessorExists,
			Name:   name,
			Path:   dir,
			Detail: fmt.Sprintf("supersede: %s already exists", dir),
		}
	}

	return ref, nil
}
