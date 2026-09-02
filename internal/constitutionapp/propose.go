package constitutionapp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/atomicfile"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// Expected is the exact branch/HEAD precondition Propose requires before it
// ever writes — mirrors internal/draftmutation.ExpectedIdentity's own
// stale-write refusal shape. Head is the caller's last-seen HEAD of Branch;
// an empty Head asserts "I expect Branch not to exist yet" (a brand-new
// proposal).
type Expected struct {
	Branch string `json:"branch"`
	Head   string `json:"head"`
}

// ProposeRequest is Propose's strict request: create or amend one
// Git-backed policy/overlay/exemption artifact on Branch. Content is the
// caller-supplied, already-rendered artifact bytes (frontmatter + body) —
// Propose never scaffolds default content itself, exactly as
// internal/designapp's mutate_draft never scaffolds a new spec (that is
// `design start`'s job, outside the six ASD application operations).
// Instantiating the configured artifact scaffold for a brand-new proposal
// is the workbench's own "Propose change" convenience (design §7.2),
// composing internal/humanartifact directly before calling this operation.
type ProposeRequest struct {
	Branch        string   `json:"branch"`
	Kind          string   `json:"kind"`
	Name          string   `json:"name"`
	Content       []byte   `json:"content"`
	Expected      Expected `json:"expected"`
	CommitMessage string   `json:"commit_message"`
}

func (r ProposeRequest) validate() error {
	if r.Branch == "" || r.Expected.Branch == "" {
		return errBranchRequired
	}
	if r.Branch != r.Expected.Branch {
		return fmt.Errorf("constitutionapp: branch %q does not match expected.branch %q", r.Branch, r.Expected.Branch)
	}
	switch r.Kind {
	case KindPolicy, KindOverlay, KindExemption:
	default:
		return errKindInvalid
	}
	if r.Name == "" {
		return errNameRequired
	}
	if len(r.Content) == 0 {
		return errContentEmpty
	}
	return nil
}

func policyRelPath(kind, name string) (string, error) {
	var dir string
	switch kind {
	case KindPolicy:
		dir = policyartifact.DirPolicies
	case KindOverlay:
		dir = policyartifact.DirOverlays
	case KindExemption:
		dir = policyartifact.DirExemptions
	default:
		return "", errKindInvalid
	}
	return dir + "/" + name + ".md", nil
}

// decodeProposed strict-decodes content against Kind, proving its own
// filename-stem/identity agreement exactly as internal/policyauthority.
// Load's own store.go cross-check does, and returns the decoded artifact's
// canonical (id, digest) pair. It reuses policyartifact.ClassifyPolicyPath
// as the single source of truth for the (kind, name) a relative path
// names, rather than trusting the request's own Kind/Name a second time —
// a caller-supplied Kind that disagrees with its own Name is caught here,
// before any write.
func decodeProposed(kind, name string, content []byte) (id, digest string, typed *Error) {
	rel, err := policyRelPath(kind, name)
	if err != nil {
		return "", "", inputInvalid("input-invalid", err.Error())
	}
	classifiedKind, classifiedName, err := policyartifact.ClassifyPolicyPath(rel)
	if err != nil {
		return "", "", inputInvalid("input-invalid", err.Error())
	}
	if classifiedKind != kind || classifiedName != name {
		return "", "", operational("authority-invalid", "constructed path does not classify back to the requested kind/name", nil)
	}

	switch kind {
	case KindPolicy:
		p, err := policyartifact.DecodePolicy(content)
		if err != nil {
			return "", "", verdictWithCause("corrupted-policy", "decoding proposed policy", err)
		}
		if p.Name() != name {
			return "", "", verdict("name-mismatch", fmt.Sprintf("content name %q does not match requested name %q", p.Name(), name))
		}
		digest, err := p.Digest()
		if err != nil {
			return "", "", operational("authority-invalid", "digesting proposed policy", err)
		}
		return p.ID, digest, nil
	case KindOverlay:
		o, err := policyartifact.DecodeOverlay(content)
		if err != nil {
			return "", "", verdictWithCause("corrupted-policy", "decoding proposed overlay", err)
		}
		if o.Name() != name {
			return "", "", verdict("name-mismatch", fmt.Sprintf("content name %q does not match requested name %q", o.Name(), name))
		}
		digest, err := o.Digest()
		if err != nil {
			return "", "", operational("authority-invalid", "digesting proposed overlay", err)
		}
		return o.ID, digest, nil
	case KindExemption:
		e, err := policyartifact.DecodeExemption(content)
		if err != nil {
			return "", "", verdictWithCause("corrupted-policy", "decoding proposed exemption", err)
		}
		if e.Name() != name {
			return "", "", verdict("name-mismatch", fmt.Sprintf("content name %q does not match requested name %q", e.Name(), name))
		}
		digest, err := e.Digest()
		if err != nil {
			return "", "", operational("authority-invalid", "digesting proposed exemption", err)
		}
		return e.ID, digest, nil
	default:
		return "", "", inputInvalid("input-invalid", errKindInvalid.Error())
	}
}

// ProposeResult is Propose's exact envelope. ZeroEffect is true when the
// requested Content is byte-identical to what is already committed at the
// resolved path — Propose then reports success without creating an empty
// commit (Wave 6 Task 3's required "zero-effect" test).
type ProposeResult struct {
	Schema     string   `json:"schema"`
	Identity   Identity `json:"identity"`
	Path       string   `json:"path"`
	ArtifactID string   `json:"artifact_id"`
	Digest     string   `json:"digest"`
	ZeroEffect bool     `json:"zero_effect"`
	Commit     string   `json:"commit,omitempty"`
}

// Propose creates or amends one Git-backed policy/overlay/exemption
// proposal artifact on req.Branch, guarded by req.Expected's exact
// branch/HEAD precondition (a stale-head refusal, verdict, never silently
// rebased over). It never merges, approves, or writes anything outside the
// one requested artifact path — merge/approval stay the normal Git
// pull-request boundary (design §7.1).
func (s Service) Propose(ctx context.Context, root string, req ProposeRequest) (*ProposeResult, *Error) {
	if root == "" {
		return nil, inputInvalid("input-invalid", errRootRequired.Error())
	}
	if err := req.validate(); err != nil {
		return nil, inputInvalid("input-invalid", err.Error())
	}
	if s.Git == nil {
		return nil, operational("git-reader-unavailable", "git reader is not configured", nil)
	}

	currentBranch, err := s.Git.CurrentBranch(ctx, root)
	if err != nil {
		return nil, operational("io-failure", "resolving current branch", err)
	}
	exists, err := s.Git.HasLocalBranch(ctx, root, req.Branch)
	if err != nil {
		return nil, operational("io-failure", "checking for existing proposal branch", err)
	}

	if exists {
		branchHead, err := s.Git.RevParse(ctx, root, req.Branch)
		if err != nil {
			return nil, operational("io-failure", "resolving proposal branch HEAD", err)
		}
		if branchHead != req.Expected.Head {
			return nil, verdict("stale-head", fmt.Sprintf("branch %q is at %s, expected %s", req.Branch, branchHead, req.Expected.Head))
		}
		if currentBranch != req.Branch {
			// gitx.Checkout (constitutionapp's own GitReader.CheckoutExisting
			// adapter) refuses a dirty working tree itself, so no separate
			// guard is needed on this branch of the dispatch.
			if err := s.Git.CheckoutExisting(ctx, root, req.Branch); err != nil {
				return nil, operational("io-failure", "checking out proposal branch", err)
			}
		} else if dirty, err := s.Git.StatusDirty(ctx, root); err != nil {
			return nil, operational("io-failure", "checking working-tree cleanliness", err)
		} else if dirty {
			// Already on req.Branch, so no checkout guard runs at all —
			// without this explicit check, AddAll below would sweep any
			// pre-existing uncommitted change into this commit, silently
			// widening the write footprint beyond the one requested
			// artifact path (design §7.1's "prepare submission without
			// merging or inventing approval" boundary starts here: Propose
			// must never durably absorb content the caller did not ask it
			// to write).
			return nil, operational("checkout-dirty", "the working tree has uncommitted changes unrelated to this proposal; commit or discard them first", nil)
		}
	} else {
		if req.Expected.Head != "" {
			return nil, verdict("stale-head", fmt.Sprintf("branch %q does not exist, expected HEAD %s", req.Branch, req.Expected.Head))
		}
		// gitx.CheckoutNewBranch carries no dirty-tree guard of its own
		// (unlike gitx.Checkout): it only ever moves HEAD to a fresh branch
		// at the SAME commit, so no tracked file changes and no data is at
		// risk from that operation alone. But a dirty tree here would still
		// let AddAll below sweep unrelated uncommitted content into the
		// first commit on a brand-new proposal branch — the same
		// write-footprint concern as the already-on-branch case above.
		if dirty, err := s.Git.StatusDirty(ctx, root); err != nil {
			return nil, operational("io-failure", "checking working-tree cleanliness", err)
		} else if dirty {
			return nil, operational("checkout-dirty", "the working tree has uncommitted changes unrelated to this proposal; commit or discard them first", nil)
		}
		if err := s.Git.CheckoutNewBranch(ctx, root, req.Branch); err != nil {
			return nil, operational("io-failure", "creating proposal branch", err)
		}
	}

	artifactID, digest, typed := decodeProposed(req.Kind, req.Name, req.Content)
	if typed != nil {
		return nil, typed
	}

	rel, relErr := policyRelPath(req.Kind, req.Name)
	if relErr != nil {
		return nil, inputInvalid("input-invalid", relErr.Error())
	}
	full := filepath.Join(root, ".verdi", "policy", filepath.FromSlash(rel))

	existing, readErr := os.ReadFile(full)
	if readErr == nil && bytes.Equal(existing, req.Content) {
		identity, typedIdentity := s.resolveIdentity(ctx, root)
		if typedIdentity != nil {
			return nil, typedIdentity
		}
		return &ProposeResult{Schema: ProposeResultSchema, Identity: identity, Path: rel, ArtifactID: artifactID, Digest: digest, ZeroEffect: true}, nil
	}
	if readErr != nil && !os.IsNotExist(readErr) {
		return nil, operational("io-failure", "reading existing proposal artifact", readErr)
	}

	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, operational("io-failure", "creating proposal artifact directory", err)
	}
	if err := atomicfile.Write(full, req.Content, 0o644); err != nil {
		return nil, operational("io-failure", "writing proposal artifact", err)
	}
	if err := s.Git.AddAll(ctx, root); err != nil {
		return nil, operational("io-failure", "staging proposal artifact", err)
	}
	message := req.CommitMessage
	if message == "" {
		message = fmt.Sprintf("propose %s: %s", req.Kind, artifactID)
	}
	commit, err := s.Git.CreateCommit(ctx, root, message)
	if err != nil {
		return nil, operational("io-failure", "committing proposal artifact", err)
	}

	identity, typedIdentity := s.resolveIdentity(ctx, root)
	if typedIdentity != nil {
		return nil, typedIdentity
	}
	return &ProposeResult{Schema: ProposeResultSchema, Identity: identity, Path: rel, ArtifactID: artifactID, Digest: digest, Commit: commit}, nil
}
