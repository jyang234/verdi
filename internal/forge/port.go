// Package forge is the I-22 forge port: the tiny, consumer-defined surface
// v0 needs from a GitLab- or GitHub-hosted store (04 §port pattern applied
// to forges) — evidence-bundle fetch for `sync` (I-8), CI-context
// detection, and the generated-file attribute token VL-012 checks.
// Adapters (gitlab/, github/) implement Forge; fake/ is the hermetic test
// double; forgetest/ is the shared contract-test suite both adapters pass
// (mirroring internal/provider/providertest's pattern).
package forge

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNoBundle is returned, wrapped, by FetchEvidenceBundle when no
// successful verdi-evidence run exists for the given (ref, commit) — the
// trigger `verdi sync --or-regen` uses to fall back to local regeneration
// (05 §CLI: "regenerates locally when no bundle exists").
var ErrNoBundle = errors.New("forge: no evidence bundle available for this ref/commit")

// EvidenceBundle is the CI evidence bundle FetchEvidenceBundle retrieves:
// the raw bytes of each of the four derived-bundle files (I-8: verdi's own
// CI job "verdi-evidence" uploads the derived/<ref-slug>/<commit>/ tree as
// its artifact). Callers (cmd/verdi/sync.go) write these bytes straight to
// disk under data/derived/<ref-slug>/<commit>/ — this package does not
// decode them; that is internal/upstream's and internal/bundle's job.
type EvidenceBundle struct {
	Verdicts     []byte
	Tests        []byte
	Review       []byte
	BoundaryDiff []byte
}

// CIInfo is what CIContext detects from the forge's own CI environment:
// the default branch and, if the current run is building a merge/pull
// request, its target branch (feeds I-14's lint baselines in a later
// phase).
type CIInfo struct {
	DefaultBranch  string
	IsMergeRequest bool
	TargetBranch   string // "" if not in an MR/PR context
}

// Forge is the I-22 port.
type Forge interface {
	// FetchEvidenceBundle retrieves the latest successful verdi-evidence
	// CI run's artifact for (ref, commit) through the forge's own API.
	// Returns an error wrapping ErrNoBundle if no successful run exists
	// for that (ref, commit).
	FetchEvidenceBundle(ctx context.Context, ref, commit string) (*EvidenceBundle, error)
	// GeneratedAttribute returns the forge-appropriate git-attribute
	// token marking a path generated (02 §Repository plumbing, VL-012):
	// "gitlab-generated" or "linguist-generated".
	GeneratedAttribute() string
	// CIContext detects the current CI environment from the forge's own
	// CI-provided environment variables.
	CIContext(ctx context.Context) (CIInfo, error)
	// ListOpenMRs lists open (unmerged) merge/pull requests targeting
	// targetBranch (typically the store's default branch) through the
	// forge's own API — the rung-4 cascade fold's pending-supersession
	// input set (03 §The amendment ladder; openmr.go).
	ListOpenMRs(ctx context.Context, targetBranch string) ([]OpenMR, error)
	// FetchFileAtRef retrieves path's raw content as it stands at ref (a
	// branch name) through the forge's own API — enough to read a pending
	// supersession manifest's spec file from an OpenMR's SourceBranch
	// without a local clone of that branch. Returns an error wrapping
	// ErrFileNotFound when path does not exist at ref — the expected,
	// non-error-in-spirit outcome for most open MRs, which don't touch the
	// candidate spec path at all; callers distinguish that case from a
	// real transport error via errors.Is(err, ErrFileNotFound).
	FetchFileAtRef(ctx context.Context, ref, path string) ([]byte, error)

	// ListComments returns mrID's full comment feed — diff-anchored and
	// general/thread comments merged into one list (comments.go's doc
	// comment; 05 §Review stickies and forge round-trip: "the board pulls
	// the MR's full comment feed on every render"). Unfiltered: every
	// comment is returned regardless of whether its body carries a
	// resolvable [vd:<object-id>] token — classification into
	// anchored-vs-unanchored (the inbox tray split) is the caller's job
	// (ParseCommentToken, token.go), never dropped here.
	ListComments(ctx context.Context, mrID string) ([]Comment, error)
	// PostComment posts body as a new comment on mrID, anchored to target
	// if non-nil (a diff line) or as a general/thread comment if nil.
	// Returns the created Comment as the forge reports it back.
	PostComment(ctx context.Context, mrID, body string, target *CommentTarget) (Comment, error)
	// GetThreadResolution returns one entry per SUBSTANTIVE thread on
	// mrID — a thread the forge itself treats as resolvable (comments.go's
	// doc comment operationalizes 05's "substantive"). Threads with no
	// resolution concept at all (a general/individual comment) never
	// appear here, only in ListComments.
	GetThreadResolution(ctx context.Context, mrID string) ([]ThreadResolution, error)
}

// ErrFileNotFound is returned, wrapped, by FetchFileAtRef when path does
// not exist at ref — the expected, non-error outcome for most open MRs
// when probed against one candidate supersession-manifest path (most open
// MRs are not superseding the feature in question at all).
var ErrFileNotFound = errors.New("forge: file not found at ref")

// DetectKind decides which forge adapter kind applies: manifestForge (the
// store manifest's `forge:` key) if set, else auto-detected from
// remoteURL's host (I-22: "adapter selected via a verdi.yaml forge: key
// with auto-detect from the remote URL"). It returns "gitlab" or "github";
// constructing the concrete adapter for that kind is the caller's job
// (cmd/verdi/sync.go) — this package cannot import the gitlab/github
// subpackages without a dependency cycle, since they import this one.
func DetectKind(manifestForge, remoteURL string) (string, error) {
	if manifestForge != "" {
		return manifestForge, nil
	}
	lower := strings.ToLower(remoteURL)
	switch {
	case strings.Contains(lower, "gitlab"):
		return "gitlab", nil
	case strings.Contains(lower, "github"):
		return "github", nil
	default:
		return "", fmt.Errorf("forge: cannot auto-detect forge kind from remote URL %q and no forge: key set in verdi.yaml (I-22)", remoteURL)
	}
}
