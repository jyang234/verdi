package constitutionapp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
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
	Schema        string   `json:"schema"`
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
// requested Content is byte-identical to what is already COMMITTED at the
// resolved path on the proposal branch — Propose then reports success
// without creating an empty commit (Wave 6 Task 3's required "zero-effect"
// test). It is deliberately never measured against the working tree: an
// uncommitted edit carrying exactly the requested bytes is a REAL effect
// (the branch a proposal submits is its COMMITTED tree), and short-
// circuiting on it would report exit-0 success over a branch that never
// received the change.
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
// rebased over).
//
// Every request, CONTENT, path-custody, and branch/HEAD precondition check
// that can be decided WITHOUT touching the repository runs before the first
// repository mutation, so a refusal from any of them — input-invalid,
// corrupted-policy, name-mismatch, authority-invalid, the pre-mutation
// unsafe-path proof, and both stale-head variants — leaves branch, HEAD, the
// local-branch set, and the working tree byte-identical to what it found.
// That ordering is load-bearing rather than stylistic: this operation's
// refusal path returns only a *Error, whose Failure envelope carries no
// Identity at all, so a mutation performed before a refusal would be
// undisclosed — a caller could not learn from the refusal that its checkout
// had been moved onto a new, empty proposal branch.
//
// The guarantee is therefore exactly that scope, and no wider. Three classes
// of failure are decidable only AFTER the checkout has already selected (and,
// for a brand-new proposal, created) req.Branch, and each can leave the
// checkout there: the post-checkout unsafe-path proof retaken against the
// branch's own materialized tree, a divergent-worktree refusal, and any
// operational failure of the write/stage/commit sequence itself. Each is
// documented at its own site below. Closing that residual would take a
// repository-level transaction this operation does not have; it is disclosed
// rather than silently implied away.
//
// It stages and commits exactly the one requested artifact
// path via gitx.AddPaths — never gitx.AddAll — so an unrelated sibling
// file elsewhere in the checkout (a caller's own --request document,
// another in-progress edit) can never be swept into this commit. It never
// merges, approves, or writes anything outside that one path — merge/
// approval stay the normal Git pull-request boundary (design §7.1).
//
// Two custody refusals guard the write itself, because "one path" is a claim
// about the FILESYSTEM, not about a string:
//
//   - unsafe-path — a symlinked component of the destination would resolve
//     the write somewhere else entirely (safepath.go explains why the path
//     grammar alone cannot establish this);
//   - divergent-worktree — the working tree carries uncommitted content at
//     the destination that matches neither the committed blob nor the
//     request, so the write would erase an edit nobody asked to discard.
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

	// Content validation FIRST — decodeProposed owns every corrupted-policy,
	// name-mismatch, and authority-invalid refusal, and nothing below it has
	// run yet, so each of those refusals is provably side-effect-free.
	artifactID, digest, typed := decodeProposed(req.Kind, req.Name, req.Content)
	if typed != nil {
		return nil, typed
	}
	rel, relErr := policyRelPath(req.Kind, req.Name)
	if relErr != nil {
		return nil, inputInvalid("input-invalid", relErr.Error())
	}
	// gitPath is the repository-relative path; full is its absolute
	// filesystem twin. Both are derived here, before any mutation, so the
	// write path below performs no further derivation that could fail after
	// the checkout has already moved.
	gitPath := constitutionPolicyDir + "/" + rel
	full := filepath.Join(root, filepath.FromSlash(gitPath))

	// Path custody, taken BEFORE the first repository read or mutation so it
	// composes with the content-validation hoist above: every component of
	// the destination that already exists must be a real directory (and the
	// destination itself, if present, a real regular file). Without this,
	// filepath.Join + atomicfile.Write silently resolve a symlinked store
	// directory and land the artifact outside the checkout entirely — the
	// write happening before `git add` can even refuse the beyond-a-symlink
	// pathspec, so the escape is a completed side effect of a "failed"
	// Propose. See safepath.go for the full rationale.
	if err := checkNoSymlinkedComponent(root, gitPath); err != nil {
		return nil, verdict("unsafe-path", err.Error())
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
			// adapter) refuses a dirty working tree itself for a genuine
			// branch switch — a caller mid-edit on a DIFFERENT branch is
			// stopped here, before anything below runs.
			if err := s.Git.CheckoutExisting(ctx, root, req.Branch); err != nil {
				return nil, operational("io-failure", "checking out proposal branch", err)
			}
		}
	} else {
		if req.Expected.Head != "" {
			return nil, verdict("stale-head", fmt.Sprintf("branch %q does not exist, expected HEAD %s", req.Branch, req.Expected.Head))
		}
		if err := s.Git.CheckoutNewBranch(ctx, root, req.Branch); err != nil {
			return nil, operational("io-failure", "creating proposal branch", err)
		}
	}

	// Path custody is retaken here because the checkout above materialized the
	// proposal branch's own committed tree, and that tree can itself carry a
	// mode-120000 entry at any component of this path — a symlink this
	// operation never saw during the pre-mutation proof. Only the branch
	// selection can have happened in between (the pre-mutation proof already
	// refused everything else), and that movement is itself the same residual
	// the CreateCommit failure path already carries.
	if err := checkNoSymlinkedComponent(root, gitPath); err != nil {
		return nil, verdict("unsafe-path", err.Error())
	}

	// Zero-effect is decided against the proposal branch's COMMITTED blob.
	// A Show failure means the branch's committed tree does not carry this
	// path (a brand-new artifact) or Git could not read it at all; both are
	// resolved in the conservative direction — do the work — never as a
	// zero-effect success, and a genuinely broken Git surfaces immediately
	// at the AddPaths/CreateCommit calls below.
	//
	// The converse case — the committed blob already IS req.Content while the
	// working tree carries some further, uncommitted edit at that path — is
	// genuinely zero-effect for what was requested, so that edit is left
	// exactly where the caller put it rather than being reverted or swept
	// into a commit nobody asked for. Identity.Dirty on the returned result
	// discloses that the checkout is not clean.
	committed, showErr := s.Git.Show(ctx, root, req.Branch, gitPath)
	if showErr == nil && bytes.Equal(committed, req.Content) {
		identity, typedIdentity := s.resolveIdentity(ctx, root)
		if typedIdentity != nil {
			return nil, typedIdentity
		}
		return &ProposeResult{Schema: ProposeResultSchema, Identity: identity, Path: rel, ArtifactID: artifactID, Digest: digest, ZeroEffect: true}, nil
	}

	// The write is about to REPLACE whatever the working tree carries at this
	// path. A file there that matches neither the branch's committed blob nor
	// the requested content is an uncommitted edit nobody asked this
	// operation to discard: the stale-head precondition proves only that the
	// branch REF has not moved, so it passes clean while the replacement
	// erases that edit without ever disclosing it existed. Refuse and name
	// the divergence; the caller reconciles (commits, stashes, or discards)
	// its own edit first. The matching case — an uncommitted edit carrying
	// exactly the requested bytes — is not a divergence at all and still
	// commits, exactly as before.
	worktree, worktreeExists, readErr := readRegularFile(full)
	if readErr != nil {
		if errors.Is(readErr, errUnsafePathComponent) {
			return nil, verdict("unsafe-path", readErr.Error())
		}
		return nil, operational("io-failure", "reading the proposal artifact's working-tree state", readErr)
	}
	if worktreeExists && !bytes.Equal(worktree, req.Content) && (showErr != nil || !bytes.Equal(worktree, committed)) {
		return nil, verdict("divergent-worktree", fmt.Sprintf(
			"the working tree carries uncommitted content at %s that differs from both branch %q's committed state and the requested content; reconcile that edit before proposing over it", rel, req.Branch))
	}

	if err := ensureDirectoryChain(root, constitutionPolicyDir+"/"+path.Dir(rel)); err != nil {
		return nil, verdict("unsafe-path", err.Error())
	}
	if err := atomicfile.Write(full, req.Content, 0o644); err != nil {
		return nil, operational("io-failure", "writing proposal artifact", err)
	}
	if err := s.Git.AddPaths(ctx, root, full); err != nil {
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
