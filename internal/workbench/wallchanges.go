package workbench

// spec/wall-changes: the wall's uncommitted-changes classifier. The wall
// already knew only that the working tree was dirty (gitx.StatusDirty);
// this adds visibility into WHAT changed, in three states ac-1 requires —
// typed changes (a recognized internal/draftmutation operation with the
// object it touches), unclassified changes (a path with the reason the
// semantic diff does not recognize it), and an unreadable comparison
// (disclosed with its reason, never silently "no changes") — computed
// fresh per request from git and the working tree (co-1) and never
// persisted.
//
// The file splits the algorithm (classifyWallChanges, pure and git-free)
// from its git-backed inputs (loadWallChangesInputs) on purpose: the
// former is what the static obligations drive directly with literal byte
// and path tables, and the latter — the ONLY part that talks to git — is
// what the behavioral obligations prove against fixturegit repositories
// (co-2). computeWallChanges composes the two for loadBoard.
//
// This file never imports internal/draftmutation itself: the boundary
// witness (internal/draftmutation/boundary_test.go,
// TestLaterWorkbenchAdapterDoesNotImportDraftMutation) permits exactly one
// internal/workbench production file to do that, boardspecdesign.go, so
// the actual draftmutation.Diff call lives there (semanticDiffChanges) and
// this file consumes its result through the unguarded
// internal/designprovenance import instead — Change there is the exact
// same type (draftmutation.Change is a verbatim alias of it,
// internal/draftmutation/operation.go), so dc-1's "reused, never
// re-derived" holds exactly.

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// wallChangeReason is spec/wall-changes ac-1's closed vocabulary for why a
// path is unclassified — every difference the semantic diff does not turn
// into a typed operation. It is closed by design: a reason outside this
// set would be a new spec decision, not an implementation detail.
type wallChangeReason string

const (
	// wallReasonProse marks the spec's own body/prose text (everything
	// after the frontmatter) differing — the semantic diff only ever
	// compares frontmatter objects (dc-1), so body text is never typed.
	wallReasonProse wallChangeReason = "prose-or-body-text"
	// wallReasonLayout marks the spec's layout.json sidecar (card
	// positions) differing — a real uncommitted change with nothing for
	// the semantic diff to recognize, since it never reads that file.
	wallReasonLayout wallChangeReason = "layout"
	// wallReasonStagedPath marks any OTHER path git already tracks (a
	// pending change in the index or the worktree) — most commonly
	// another spec entirely, edited through its own board and never
	// staged, but still something the commit affordance's `git add -A`
	// (actionGitCommit) would sweep in. "Staged", here, is this
	// audience's plain-language "already known to git", not the
	// index-vs-worktree distinction gitx's own StagedPaths draws: the
	// wall's commit button stages and commits the whole tree regardless of
	// which column git status would put a path in, so the two column values
	// need one reason between them; ac-1's own reading (a PM/designer
	// audience, per gitx/worktree.go) leans the same way.
	wallReasonStagedPath wallChangeReason = "another-staged-path"
	// wallReasonUntracked marks a path git has never seen at all — a new
	// file, anywhere in the repository.
	wallReasonUntracked wallChangeReason = "untracked-file"
)

// wallUnclassifiedChange is one uncommitted change the semantic diff does
// not classify: a path plus the reason it falls outside the typed
// operation vocabulary (ac-1).
type wallUnclassifiedChange struct {
	Path   string           `json:"path"`
	Reason wallChangeReason `json:"reason"`
}

// wallChanges is the wall snapshot's uncommitted-changes summary (ac-1). A
// readable comparison carries Typed and Unclassified side by side (dc-2):
// a tree can hold both a recognized frontmatter edit and, say, an
// untracked screenshot at once. An unreadable comparison carries
// UnreadableReason instead of Typed, while Unclassified keeps listing
// every OTHER git-detected change (dc-2's "still lists what git status
// reports") — never collapsed to nothing just because the spec itself
// could not be compared.
type wallChanges struct {
	Typed            []designprovenance.Change `json:"typed"`
	Unclassified     []wallUnclassifiedChange  `json:"unclassified"`
	UnreadableReason string                    `json:"unreadableReason,omitempty"`
}

// wallChangedPath is one OTHER repository path (never the spec's own
// spec.md — that path is classified through HeadSpec/WorkingSpec instead)
// that git reports as differing from HEAD or as untracked.
type wallChangedPath struct {
	Path      string
	Untracked bool
}

// wallChangesInputs is every fact classifyWallChanges needs, pre-fetched
// by the caller so the algorithm itself stays pure and git-free — exactly
// what TestWallChanges_Classify (ac-1's static obligation) and
// TestWallChanges_ZeroOperationsStayDirty (ac-2's) drive with literal
// tables. The real git integration lives in loadWallChangesInputs alone,
// proven separately by the behavioral obligations (co-2).
type wallChangesInputs struct {
	// SpecPath and LayoutPath are this spec's own repository-relative
	// paths (store.ActiveSpecRelPath and its layout.json sidecar) —
	// ChangedPaths never repeats SpecPath; the caller classifies it
	// through HeadSpec/WorkingSpec below instead.
	SpecPath   string
	LayoutPath string

	// HeadExists is false when HEAD carries no revision of SpecPath at
	// all (a spec authored but never committed) — the "missing spec"
	// unreadable case. HeadSpec is meaningless when false.
	HeadExists bool
	HeadSpec   []byte

	// WorkingSpec is the working tree's current spec.md bytes.
	WorkingSpec []byte

	// ChangedPaths is every OTHER path git reports as differing from HEAD
	// or as untracked (never SpecPath).
	ChangedPaths []wallChangedPath
}

// splitSpecBody strict-decodes raw as a spec document and returns its body
// (everything after the frontmatter) — the same two-step validation
// loadBoard itself already performs on the working tree's own bytes
// (artifact.SplitFrontmatter then artifact.DecodeSpec), reused here rather
// than re-derived so "readable" means the same thing on both sides of the
// comparison.
func splitSpecBody(raw []byte) ([]byte, error) {
	frontmatter, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return nil, err
	}
	if _, err := artifact.DecodeSpec(frontmatter); err != nil {
		return nil, err
	}
	return body, nil
}

// diffSpecBytes computes the readable-comparison half of ac-1: the typed
// changes internal/draftmutation's Diff recognizes between two spec.md
// byte states (dc-1: reused, never re-derived, via semanticDiffChanges —
// boardspecdesign.go's one sanctioned draftmutation import site), and
// whether their body text differs (wallReasonProse) — or, if either side
// cannot be split/decoded as a spec, the reason naming which side and why.
func diffSpecBytes(head, working []byte) (typed []designprovenance.Change, proseChanged bool, unreadableReason string, err error) {
	headBody, headErr := splitSpecBody(head)
	if headErr != nil {
		return nil, false, fmt.Sprintf("HEAD's spec.md could not be read: %v", headErr), nil
	}
	workingBody, workingErr := splitSpecBody(working)
	if workingErr != nil {
		return nil, false, fmt.Sprintf("the working tree's spec.md could not be read: %v", workingErr), nil
	}
	typed, diffErr := semanticDiffChanges(head, working)
	if diffErr != nil {
		// Both sides just decoded successfully above, so a Diff failure
		// here is an internal inconsistency, not a normal disclosed state.
		return nil, false, "", fmt.Errorf("workbench: wall changes: computing semantic diff: %w", diffErr)
	}
	return typed, !bytes.Equal(headBody, workingBody), "", nil
}

// classifyWallChanges is spec/wall-changes ac-1's pure classifier: given
// already-fetched HEAD/working-tree/changed-path facts, it returns the
// three-state summary. It never touches git or the filesystem.
func classifyWallChanges(in wallChangesInputs) (*wallChanges, error) {
	result := &wallChanges{
		Typed:        []designprovenance.Change{},
		Unclassified: []wallUnclassifiedChange{},
	}

	switch {
	case !in.HeadExists:
		result.UnreadableReason = fmt.Sprintf("%s has no revision at HEAD to compare against (authored but never committed)", in.SpecPath)
	case !bytes.Equal(in.HeadSpec, in.WorkingSpec):
		typed, proseChanged, unreadableReason, err := diffSpecBytes(in.HeadSpec, in.WorkingSpec)
		if err != nil {
			return nil, err
		}
		switch {
		case unreadableReason != "":
			result.UnreadableReason = unreadableReason
		default:
			result.Typed = typed
			if proseChanged {
				result.Unclassified = append(result.Unclassified, wallUnclassifiedChange{Path: in.SpecPath, Reason: wallReasonProse})
			}
		}
	}

	for _, cp := range in.ChangedPaths {
		reason := wallReasonStagedPath
		switch {
		case cp.Path == in.LayoutPath:
			reason = wallReasonLayout
		case cp.Untracked:
			reason = wallReasonUntracked
		}
		result.Unclassified = append(result.Unclassified, wallUnclassifiedChange{Path: cp.Path, Reason: reason})
	}
	sort.Slice(result.Unclassified, func(i, j int) bool {
		if result.Unclassified[i].Path != result.Unclassified[j].Path {
			return result.Unclassified[i].Path < result.Unclassified[j].Path
		}
		return result.Unclassified[i].Reason < result.Unclassified[j].Reason
	})
	return result, nil
}

// loadWallChangesInputs fetches classifyWallChanges' inputs from root's
// real git state (co-2: the ONLY function in this file that runs git).
// workingSpec is the caller's already-read working-tree spec.md bytes
// (loadBoard has them in hand as `raw`; this never re-reads the file).
func loadWallChangesInputs(ctx context.Context, root, specName string, workingSpec []byte) (wallChangesInputs, error) {
	specPath := store.ActiveSpecRelPath(specName)
	in := wallChangesInputs{
		SpecPath:    specPath,
		LayoutPath:  path.Join(path.Dir(specPath), "layout.json"),
		WorkingSpec: workingSpec,
	}

	exists, err := gitx.PathExistsAt(ctx, root, "HEAD", specPath)
	if err != nil {
		return wallChangesInputs{}, fmt.Errorf("checking HEAD for %s: %w", specPath, err)
	}
	in.HeadExists = exists
	if exists {
		headSpec, err := gitx.Show(ctx, root, "HEAD", specPath)
		if err != nil {
			return wallChangesInputs{}, fmt.Errorf("reading HEAD's %s: %w", specPath, err)
		}
		in.HeadSpec = headSpec
	}

	changed, err := gitx.WorktreeChangedPaths(ctx, root)
	if err != nil {
		return wallChangesInputs{}, fmt.Errorf("listing changed paths: %w", err)
	}
	untrackedList, err := gitx.UntrackedPaths(ctx, root)
	if err != nil {
		return wallChangesInputs{}, fmt.Errorf("listing untracked paths: %w", err)
	}
	untracked := make(map[string]bool, len(untrackedList))
	for _, p := range untrackedList {
		untracked[p] = true
	}
	for _, p := range changed {
		if p == specPath {
			continue // classified through HeadSpec/WorkingSpec instead
		}
		in.ChangedPaths = append(in.ChangedPaths, wallChangedPath{Path: p, Untracked: untracked[p]})
	}
	return in, nil
}

// computeWallChanges composes loadWallChangesInputs and classifyWallChanges
// for loadBoard: root's real git state, classified. An error here is
// operational (git itself failed), never the unreadable-comparison
// disclosed state, which classifyWallChanges reports inside its result
// instead.
func computeWallChanges(ctx context.Context, root, specName string, workingSpec []byte) (*wallChanges, error) {
	in, err := loadWallChangesInputs(ctx, root, specName, workingSpec)
	if err != nil {
		return nil, fmt.Errorf("workbench: wall changes for %s: %w", specName, err)
	}
	return classifyWallChanges(in)
}
