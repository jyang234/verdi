package draftmutation

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// IdentityReader is the consumer-local Git fact port. The kernel owns
// canonicalization and validation; implementations supply only raw facts.
type IdentityReader interface {
	CheckoutRoot(ctx context.Context, start string) (string, error)
	CurrentBranch(ctx context.Context, root string) (string, error)
	Head(ctx context.Context, root string) (string, error)
}

type GitIdentityReader struct{}

func (GitIdentityReader) CheckoutRoot(ctx context.Context, start string) (string, error) {
	command := exec.CommandContext(ctx, "git", "-C", start, "rev-parse", "--show-toplevel")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("draftmutation: resolving Git checkout root: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func (GitIdentityReader) CurrentBranch(ctx context.Context, root string) (string, error) {
	return gitx.CurrentBranch(ctx, root)
}

func (GitIdentityReader) Head(ctx context.Context, root string) (string, error) {
	return gitx.RevParse(ctx, root, "HEAD")
}

// ResolveCanonicalIdentity constructs the request-independent identity once.
func ResolveCanonicalIdentity(ctx context.Context, start, spec string, reader IdentityReader) (Identity, error) {
	if reader == nil {
		return Identity{}, fmt.Errorf("draftmutation: identity reader is nil")
	}
	rawRoot, err := reader.CheckoutRoot(ctx, start)
	if err != nil {
		return Identity{}, err
	}
	if rawRoot == "" || strings.Contains(rawRoot, `\`) {
		return Identity{}, fmt.Errorf("draftmutation: checkout root %q is empty or non-POSIX", rawRoot)
	}
	absolute, err := filepath.Abs(rawRoot)
	if err != nil {
		return Identity{}, fmt.Errorf("draftmutation: making checkout root absolute: %w", err)
	}
	absolute = filepath.Clean(absolute)
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return Identity{}, fmt.Errorf("draftmutation: resolving checkout root symlinks: %w", err)
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.IsDir() {
		return Identity{}, fmt.Errorf("draftmutation: checkout root %q is not a directory", resolved)
	}
	canonical := filepath.ToSlash(filepath.Clean(resolved))
	branch, err := reader.CurrentBranch(ctx, resolved)
	if err != nil {
		return Identity{}, fmt.Errorf("draftmutation: resolving current branch: %w", err)
	}
	if branch == "" {
		branch = "DETACHED"
	}
	head, err := reader.Head(ctx, resolved)
	if err != nil {
		return Identity{}, fmt.Errorf("draftmutation: resolving HEAD: %w", err)
	}
	identity := Identity{Checkout: canonical, Branch: branch, Head: head, Spec: spec}
	if err := identity.Validate(); err != nil {
		return Identity{}, err
	}
	return identity, nil
}

// VerifyExpected byte-compares all three caller assertions with the one
// canonical service identity. Spec is request.Spec and already part of it.
func VerifyExpected(identity Identity, expected ExpectedIdentity) *Error {
	if expected.Checkout != identity.Checkout || expected.Branch != identity.Branch || expected.Head != identity.Head {
		return NewError(CodeIdentityInvalid, identity, fmt.Sprintf("expected checkout/branch/HEAD %q/%q/%q does not match canonical identity", expected.Checkout, expected.Branch, expected.Head))
	}
	return nil
}

// StateProjector is the consumer-local effective-state port.
type StateProjector interface {
	ResolveState(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error)
}

type GitStateProjector struct{ Projector specstate.Projector }

func NewGitStateProjector() GitStateProjector {
	return GitStateProjector{Projector: specstate.NewProjector()}
}

func (p GitStateProjector) ResolveState(ctx context.Context, root string, candidate specstate.Candidate) (specstate.Result, error) {
	return p.Projector.Resolve(ctx, root, candidate)
}

// AuthorizeState permits only the matching design/<spec-name> branch and a
// Git-projected proposal. Persisted status fields are never authority.
func AuthorizeState(ctx context.Context, root string, identity Identity, content []byte, projector StateProjector) (specstate.Result, *Error) {
	ref, err := artifact.ParseRef(identity.Spec)
	if err != nil {
		return specstate.Result{}, WrapError(CodeAuthorityInvalid, identity, "invalid canonical spec identity", err)
	}
	wantBranch := "design/" + ref.Name
	if identity.Branch != wantBranch {
		return specstate.Result{}, NewError(CodeStateForbidden, identity, fmt.Sprintf("branch %q is not mutable design branch %q", identity.Branch, wantBranch))
	}
	if projector == nil {
		return specstate.Result{}, NewError(CodeAuthorityInvalid, identity, "state projector is nil")
	}
	candidate := specstate.Candidate{Path: store.SpecRelPath(store.ZoneActive, ref.Name), Content: append([]byte(nil), content...)}
	result, projectErr := projector.ResolveState(ctx, root, candidate)
	if projectErr != nil {
		return specstate.Result{}, WrapError(CodeAuthorityInvalid, identity, "projecting Git-derived spec state", projectErr)
	}
	if result.State != specstate.Proposed {
		return result, NewError(CodeStateForbidden, identity, fmt.Sprintf("Git-derived state %q is not mutable proposal state", result.State))
	}
	return result, nil
}
