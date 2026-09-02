package constitutionapp

import (
	"context"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
)

// Identity is the one accepted/proposed Git identity every constitutionapp
// operation reports (Wave 6 Task 3: "resolve one exact accepted tree and one
// proposal identity per operation"; mirrors internal/designapp's own
// draftmutation.Identity precedent). Checkout/Branch/Head/Dirty name the
// PROPOSED state: whatever is currently checked out at Checkout — the same
// "current checkout is the draft" posture internal/draftmutation already
// uses for ASD. DefaultBranch/AcceptedHead name the ACCEPTED state, read
// without a second checkout via a Git-tree-backed fs.FS (acceptedSource,
// authority.go).
type Identity struct {
	Checkout      string `json:"checkout"`
	Branch        string `json:"branch"`
	Head          string `json:"head"`
	Dirty         bool   `json:"dirty"`
	DefaultBranch string `json:"default_branch,omitempty"`
	AcceptedHead  string `json:"accepted_head,omitempty"`
	// AcceptedKnown is false when the default branch cannot be resolved at
	// all (a hermetic repository with no configured/inferable default —
	// specstate.ResolveDefaultBranch's own disclosed failure). This is
	// distinct from Checkout==DefaultBranch: a caller must never infer "no
	// accepted tree" from an empty string when it could instead mean "the
	// accepted tree IS this same checkout."
	AcceptedKnown bool `json:"accepted_known"`
}

// GitReader is constitutionapp's own consumer-owned Git port (04 §port
// pattern): every raw Git primitive an operation needs, backed by
// internal/gitx in production. Read and write concerns share one interface
// deliberately — mirrors internal/draftmutation's own single IdentityReader
// port — since every method is a thin, individually well-tested
// internal/gitx wrapper, never an algorithm this package owns.
type GitReader interface {
	CurrentBranch(ctx context.Context, root string) (string, error)
	RevParse(ctx context.Context, root, rev string) (string, error)
	StatusDirty(ctx context.Context, root string) (bool, error)
	HasLocalBranch(ctx context.Context, root, name string) (bool, error)
	CheckoutNewBranch(ctx context.Context, root, name string) error
	CheckoutExisting(ctx context.Context, root, branch string) error
	AddAll(ctx context.Context, root string) error
	CreateCommit(ctx context.Context, root, message string) (string, error)
	LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error)
	Show(ctx context.Context, root, ref, path string) ([]byte, error)
}

// gitxReader is the production GitReader: every method is a direct,
// unmodified internal/gitx call.
type gitxReader struct{}

func (gitxReader) CurrentBranch(ctx context.Context, root string) (string, error) {
	return gitx.CurrentBranch(ctx, root)
}
func (gitxReader) RevParse(ctx context.Context, root, rev string) (string, error) {
	return gitx.RevParse(ctx, root, rev)
}
func (gitxReader) StatusDirty(ctx context.Context, root string) (bool, error) {
	return gitx.StatusDirty(ctx, root)
}
func (gitxReader) HasLocalBranch(ctx context.Context, root, name string) (bool, error) {
	return gitx.HasLocalBranch(ctx, root, name)
}
func (gitxReader) CheckoutNewBranch(ctx context.Context, root, name string) error {
	return gitx.CheckoutNewBranch(ctx, root, name)
}
func (gitxReader) CheckoutExisting(ctx context.Context, root, branch string) error {
	// gitx.Checkout, never gitx.CheckoutExisting: this is a genuine
	// user-initiated branch switch (amending an existing proposal branch).
	// gitx.CheckoutExisting's own doc comment reserves gitx.Checkout's
	// dirty-working-tree branch-switch guard for exactly this case and
	// deliberately skips it only for one narrow internal unwind path.
	return gitx.Checkout(ctx, root, branch)
}
func (gitxReader) AddAll(ctx context.Context, root string) error {
	return gitx.AddAll(ctx, root)
}
func (gitxReader) CreateCommit(ctx context.Context, root, message string) (string, error) {
	return gitx.CreateCommit(ctx, root, message)
}
func (gitxReader) LsTreeEntries(ctx context.Context, root, ref string) ([]gitx.TreeEntry, error) {
	return gitx.LsTreeEntries(ctx, root, ref)
}
func (gitxReader) Show(ctx context.Context, root, ref, path string) ([]byte, error) {
	return gitx.Show(ctx, root, ref, path)
}

// resolveIdentity is the shared prologue every operation starts from: the
// current checkout's proposed Git state plus, when resolvable, the accepted
// default branch's exact HEAD. An unresolved default branch is disclosed via
// AcceptedKnown=false rather than failing the whole operation — the same
// non-fatal-disclosure posture internal/designapp's own resolveReviewBaseline
// uses for "default branch is unresolved."
func (s Service) resolveIdentity(ctx context.Context, root string) (Identity, *Error) {
	if s.Git == nil {
		return Identity{}, operational("git-reader-unavailable", "git reader is not configured", nil)
	}
	branch, err := s.Git.CurrentBranch(ctx, root)
	if err != nil {
		return Identity{}, operational("io-failure", "resolving current branch", err)
	}
	head, err := s.Git.RevParse(ctx, root, "HEAD")
	if err != nil {
		return Identity{}, operational("io-failure", "resolving current HEAD", err)
	}
	dirty, err := s.Git.StatusDirty(ctx, root)
	if err != nil {
		return Identity{}, operational("io-failure", "resolving working-tree cleanliness", err)
	}
	identity := Identity{Checkout: root, Branch: branch, Head: head, Dirty: dirty}

	def, ok := specstate.ResolveDefaultBranch(ctx, root)
	if !ok {
		return identity, nil
	}
	acceptedHead, err := s.Git.RevParse(ctx, root, def.Ref)
	if err != nil {
		return Identity{}, operational("io-failure", "resolving accepted default-branch HEAD", err)
	}
	identity.DefaultBranch = def.Name
	identity.AcceptedHead = acceptedHead
	identity.AcceptedKnown = true
	return identity, nil
}
