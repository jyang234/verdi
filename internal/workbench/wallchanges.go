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
// and path tables, and the latter — the ONLY part that talks to git, and
// only through the read-only wallGitReader port — is what the behavioral
// obligations prove against fixturegit repositories (co-2).
// computeWallChanges composes the two for the wall page and its snapshot
// (loadASD), and for nothing else.
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
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// wallChangeReason is the vocabulary for why a path is unclassified —
// every difference the semantic diff does not turn into a typed operation.
// The first four are spec/wall-changes ac-1's own; the fifth is SI-298's.
// It is a public JSON enum (the Commit and push popover consumes it): a
// new value is a recorded decision, never an implementation detail.
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
	// wallReasonStagedPath marks any OTHER tracked path whose index or
	// working tree differs from HEAD, anywhere in the repository,
	// including deletions and both sides of a rename (SI-299): Commit and
	// push stages everything (actionGitCommit's gitx.AddAll), so an
	// unstaged edit is swept into the commit exactly like a staged one.
	wallReasonStagedPath wallChangeReason = "another-staged-path"
	// wallReasonUntracked marks a path git has never seen at all — a new
	// file, anywhere in the repository.
	wallReasonUntracked wallChangeReason = "untracked-file"
	// wallReasonUnrecognizedSpec marks the spec file itself when git
	// reports it changed and the typed operations plus the body-text
	// comparison do not account for the whole change (SI-298): a
	// frontmatter field outside the semantic snapshot (title, owners,
	// impacts, ...), a formatting-only edit, a file-mode change, an index
	// state the working tree does not show (staged, then reverted), or an
	// unreadable comparison. When in doubt the spec is listed, beside any
	// typed operations; it is never dropped.
	wallReasonUnrecognizedSpec wallChangeReason = "unrecognized-spec-change"
)

// wallUnclassifiedChange is one uncommitted change the semantic diff does
// not classify: a repository-root-relative path plus the reason it falls
// outside the typed operation vocabulary (ac-1).
type wallUnclassifiedChange struct {
	Path   string           `json:"path"`
	Reason wallChangeReason `json:"reason"`
}

// wallChanges is the wall snapshot's uncommitted-changes summary (ac-1). A
// readable comparison carries Typed and Unclassified side by side (dc-2):
// a tree can hold both a recognized frontmatter edit and, say, an
// untracked screenshot at once. An unreadable comparison carries
// UnreadableReason instead of Typed (Typed is nil and absent from the
// JSON), while Unclassified keeps listing what git status reports (dc-2's
// "still lists what git status reports") — never collapsed to nothing
// just because the spec itself could not be compared.
type wallChanges struct {
	Typed            []designprovenance.Change `json:"typed"`
	Unclassified     []wallUnclassifiedChange  `json:"unclassified"`
	UnreadableReason string                    `json:"unreadableReason,omitempty"`
}

// MarshalJSON writes the summary's wire shape, which is keyed to its
// state rather than to nil-ness: a readable comparison always carries
// "typed" (an empty list when nothing is typed), an unreadable one never
// does (dc-2: its reason INSTEAD of a typed list), and "unclassified" is
// always a list.
func (c wallChanges) MarshalJSON() ([]byte, error) {
	wire := struct {
		Typed            *[]designprovenance.Change `json:"typed,omitempty"`
		Unclassified     []wallUnclassifiedChange   `json:"unclassified"`
		UnreadableReason string                     `json:"unreadableReason,omitempty"`
	}{Unclassified: c.Unclassified, UnreadableReason: c.UnreadableReason}
	if wire.Unclassified == nil {
		wire.Unclassified = []wallUnclassifiedChange{}
	}
	if c.UnreadableReason == "" {
		typed := c.Typed
		if typed == nil {
			typed = []designprovenance.Change{}
		}
		wire.Typed = &typed
	}
	return json.Marshal(wire)
}

// wallChangedPath is one OTHER repository path (never the spec's own
// spec.md — that path is classified through HeadSpec/WorkingSpec and
// SpecStatus instead) that git reports as differing from HEAD or as
// untracked.
type wallChangedPath struct {
	Path      string
	Untracked bool
}

// wallSpecStatus is what git status reports for the spec's own path — the
// facts a byte comparison of HEAD with the working tree cannot see.
type wallSpecStatus struct {
	// Reported: git lists the spec path at all (a tracked change in either
	// status column, the source of a rename, or an untracked file).
	Reported bool
	// Untracked: git has never seen the spec path.
	Untracked bool
	// IndexDiverged: the index holds a state that differs from both HEAD
	// and the working tree (both status columns set, or an unmerged
	// entry) — a staged edit later reverted or edited again.
	IndexDiverged bool
	// ModeChanged: the file mode differs between HEAD and the working tree.
	ModeChanged bool
}

// wallChangesInputs is every fact classifyWallChanges needs, pre-fetched
// by the caller so the algorithm itself stays pure and git-free — exactly
// what TestWallChanges_Classify (ac-1's static obligation) and
// TestWallChanges_ZeroOperationsStayDirty (ac-2's) drive with literal
// tables. The real git integration lives in loadWallChangesInputs alone,
// proven separately by the behavioral obligations (co-2).
type wallChangesInputs struct {
	// SpecPath and LayoutPath are this spec's own repository-root-relative
	// paths (the store's prefix below the git top level, then
	// store.ActiveSpecRelPath and its layout.json sidecar) — ChangedPaths
	// never repeats SpecPath.
	SpecPath   string
	LayoutPath string

	// HeadExists is false when HEAD carries no revision of SpecPath — a
	// spec authored but never committed, or a HEAD that names no commit at
	// all (HeadUnborn) — the missing-spec unreadable case. HeadSpec is
	// meaningless when false.
	HeadExists bool
	HeadUnborn bool
	HeadSpec   []byte

	// WorkingSpec is the working tree's current spec.md bytes.
	WorkingSpec []byte

	// SpecStatus is what git status reports for SpecPath itself.
	SpecStatus wallSpecStatus

	// ChangedPaths is every OTHER path git reports as differing from HEAD
	// or as untracked (never SpecPath).
	ChangedPaths []wallChangedPath
}

// specRevision is one side of the comparison, strict-decoded: the same
// two-step validation loadBoard itself performs on the working tree's own
// bytes (artifact.SplitFrontmatter then artifact.DecodeSpec), reused here
// rather than re-derived so "readable" means the same thing on both sides.
type specRevision struct {
	frontmatter []byte
	body        []byte
	spec        *artifact.SpecFrontmatter
}

func readSpecRevision(raw []byte) (specRevision, error) {
	frontmatter, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return specRevision{}, err
	}
	spec, err := artifact.DecodeSpec(frontmatter)
	if err != nil {
		return specRevision{}, err
	}
	return specRevision{frontmatter: frontmatter, body: body, spec: spec}, nil
}

// outsideSemanticSnapshot returns a copy of fm with every field
// internal/draftmutation's semantic snapshot covers cleared — problem,
// outcome, acceptance criteria, constraints, decisions (with their links),
// open questions, stubs, context, and links — leaving exactly the fields
// no typed operation can describe (id, title, owners, impacts, status, and
// the rest). TestWallChanges_SnapshotCoverage pins the correspondence: an
// edit to each cleared field must yield a typed operation, so this list
// can never hide a change the diff does not type.
func outsideSemanticSnapshot(fm *artifact.SpecFrontmatter) artifact.SpecFrontmatter {
	rest := *fm
	rest.Problem, rest.Outcome = nil, nil
	rest.AcceptanceCriteria = nil
	rest.Constraints = nil
	rest.Decisions = nil
	rest.OpenQuestions = nil
	rest.Stubs = nil
	rest.Context = nil
	rest.Links = nil
	return rest
}

// specComparison is the result of comparing HEAD's spec.md with the
// working tree's.
type specComparison struct {
	// typed is draftmutation.Diff's own classification (dc-1); non-nil
	// whenever the comparison is readable.
	typed []designprovenance.Change
	// proseChanged: the body text differs.
	proseChanged bool
	// unaccounted: the bytes differ in a way neither typed nor
	// proseChanged describes (SI-298).
	unaccounted bool
	// identical: the two byte states are equal.
	identical bool
	// unreadableReason names which side could not be read, and why.
	unreadableReason string
}

// compareSpecRevisions computes the readable-comparison half of ac-1: the
// typed changes internal/draftmutation's Diff recognizes between two
// spec.md byte states (dc-1: reused, never re-derived, via
// semanticDiffChanges — boardspecdesign.go's one sanctioned draftmutation
// import site), whether their body text differs, and whether anything
// else differs that neither accounts for — or, if either side cannot be
// split/decoded as a spec, the reason naming which side and why.
//
// "Anything else" (SI-298) is any of: the frontmatter bytes changed while
// no operation is typed (a formatting-only edit, a field outside the
// snapshot, a reorder the snapshot ignores); a field outside the semantic
// snapshot changed, beside typed operations or not; or the bytes differ
// outside both the frontmatter and the body. A formatting change or a
// reorder made in the same edit as a typed operation is not separately
// detected: the typed operations are the account of that frontmatter.
func compareSpecRevisions(head, working []byte) (specComparison, error) {
	identical := bytes.Equal(head, working)
	headRev, err := readSpecRevision(head)
	if err != nil {
		return specComparison{identical: identical, unreadableReason: fmt.Sprintf("HEAD's spec.md could not be read: %v", err)}, nil
	}
	workingRev, err := readSpecRevision(working)
	if err != nil {
		return specComparison{identical: identical, unreadableReason: fmt.Sprintf("the working tree's spec.md could not be read: %v", err)}, nil
	}
	if identical {
		return specComparison{typed: []designprovenance.Change{}, identical: true}, nil
	}
	typed, err := semanticDiffChanges(head, working)
	if err != nil {
		// Both sides just decoded successfully above, so a Diff failure
		// here is an internal inconsistency, not a normal disclosed state.
		return specComparison{}, fmt.Errorf("workbench: wall changes: computing semantic diff: %w", err)
	}
	cmp := specComparison{typed: typed, proseChanged: !bytes.Equal(headRev.body, workingRev.body)}
	frontmatterChanged := !bytes.Equal(headRev.frontmatter, workingRev.frontmatter)
	cmp.unaccounted = (frontmatterChanged && len(typed) == 0) ||
		(!frontmatterChanged && !cmp.proseChanged) ||
		!reflect.DeepEqual(outsideSemanticSnapshot(headRev.spec), outsideSemanticSnapshot(workingRev.spec))
	return cmp, nil
}

// classifyWallChanges is spec/wall-changes ac-1's pure classifier: given
// already-fetched HEAD/working-tree/status facts, it returns the
// three-state summary. It never touches git or the filesystem.
func classifyWallChanges(in wallChangesInputs) (*wallChanges, error) {
	result := &wallChanges{Unclassified: []wallUnclassifiedChange{}}

	// listSpec decides whether the spec file itself joins Unclassified.
	listSpec := false
	switch {
	case !in.HeadExists:
		result.UnreadableReason = fmt.Sprintf("%s has no revision at HEAD to compare against (authored but never committed)", in.SpecPath)
		if in.HeadUnborn {
			result.UnreadableReason = fmt.Sprintf("HEAD does not name a commit yet, so %s has no revision at HEAD to compare against", in.SpecPath)
		}
		listSpec = in.SpecStatus.Reported
	default:
		cmp, err := compareSpecRevisions(in.HeadSpec, in.WorkingSpec)
		if err != nil {
			return nil, err
		}
		if cmp.unreadableReason != "" {
			result.UnreadableReason = cmp.unreadableReason
			listSpec = in.SpecStatus.Reported || !cmp.identical
			break
		}
		result.Typed = cmp.typed
		if cmp.proseChanged {
			result.Unclassified = append(result.Unclassified, wallUnclassifiedChange{Path: in.SpecPath, Reason: wallReasonProse})
		}
		status := in.SpecStatus
		listSpec = cmp.unaccounted || (status.Reported && (cmp.identical || status.IndexDiverged || status.ModeChanged))
	}
	if listSpec {
		reason := wallReasonUnrecognizedSpec
		if !in.HeadExists && in.SpecStatus.Untracked {
			reason = wallReasonUntracked
		}
		result.Unclassified = append(result.Unclassified, wallUnclassifiedChange{Path: in.SpecPath, Reason: reason})
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
	result.Unclassified = dedupeUnclassified(result.Unclassified)
	return result, nil
}

// dedupeUnclassified drops adjacent identical entries from a sorted list —
// a path git names twice for one reason (a rename's source that is also
// edited again, say) is one change to list, not two.
func dedupeUnclassified(sorted []wallUnclassifiedChange) []wallUnclassifiedChange {
	out := make([]wallUnclassifiedChange, 0, len(sorted))
	for i, entry := range sorted {
		if i > 0 && entry == sorted[i-1] {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// wallGitReader is the read-only git the summary consults (the consumer-
// defined port; gitxWallReader is production). Every query runs from the
// repository's top level so every path means one thing: `git status`
// answers repository-root-relative wherever it runs, while `git ls-tree`
// and `git ls-files` answer relative to their working directory, and
// `git ls-files` sees only the directory it runs in.
type wallGitReader interface {
	Locate(ctx context.Context, dir string) (gitx.Location, error)
	PathExistsAt(ctx context.Context, dir, commit, path string) (bool, error)
	CommitExists(ctx context.Context, dir, commit string) (bool, error)
	Show(ctx context.Context, dir, commit, path string) ([]byte, error)
	TrackedChanges(ctx context.Context, dir string) ([]gitx.TrackedChange, error)
	UntrackedPaths(ctx context.Context, dir string) ([]string, error)
}

// gitxWallReader is wallGitReader over internal/gitx, all read-only
// queries: rev-parse, ls-tree, show, status with --no-optional-locks (so
// status never refreshes and writes the index, co-1), and ls-files.
type gitxWallReader struct{}

func (gitxWallReader) Locate(ctx context.Context, dir string) (gitx.Location, error) {
	return gitx.Locate(ctx, dir)
}

func (gitxWallReader) PathExistsAt(ctx context.Context, dir, commit, path string) (bool, error) {
	return gitx.PathExistsAt(ctx, dir, commit, path)
}

func (gitxWallReader) CommitExists(ctx context.Context, dir, commit string) (bool, error) {
	return gitx.CommitExists(ctx, dir, commit)
}

func (gitxWallReader) Show(ctx context.Context, dir, commit, path string) ([]byte, error) {
	return gitx.Show(ctx, dir, commit, path)
}

func (gitxWallReader) TrackedChanges(ctx context.Context, dir string) ([]gitx.TrackedChange, error) {
	return gitx.TrackedChanges(ctx, dir)
}

func (gitxWallReader) UntrackedPaths(ctx context.Context, dir string) ([]string, error) {
	return gitx.UntrackedPaths(ctx, dir)
}

// loadWallChangesInputs fetches classifyWallChanges' inputs from root's
// real git state through git (co-2: the ONLY function in this file that
// consults git). workingSpec is the caller's already-read working-tree
// spec.md bytes (loadBoard has them in hand as `raw`; this never re-reads
// the file). Every path is resolved against the git top level, so a store
// below it (a nested store) reads the same way as one at it.
func loadWallChangesInputs(ctx context.Context, git wallGitReader, root, specName string, workingSpec []byte) (wallChangesInputs, error) {
	loc, err := git.Locate(ctx, root)
	if err != nil {
		return wallChangesInputs{}, fmt.Errorf("locating the repository: %w", err)
	}
	top := loc.TopLevel
	specPath := loc.Prefix + store.ActiveSpecRelPath(specName)
	in := wallChangesInputs{
		SpecPath:    specPath,
		LayoutPath:  filepath.ToSlash(boardlayout.FilePath(path.Dir(specPath))),
		WorkingSpec: workingSpec,
	}

	exists, err := git.PathExistsAt(ctx, top, "HEAD", specPath)
	switch {
	case err != nil:
		// ls-tree fails outright when HEAD names no commit (an unborn
		// branch): that is the missing-at-HEAD state, disclosed, never an
		// error. Any other failure is operational.
		resolves, headErr := git.CommitExists(ctx, top, "HEAD")
		if headErr != nil {
			return wallChangesInputs{}, fmt.Errorf("checking HEAD for %s: %w", specPath, errors.Join(err, headErr))
		}
		if resolves {
			return wallChangesInputs{}, fmt.Errorf("checking HEAD for %s: %w", specPath, err)
		}
		in.HeadUnborn = true
	case exists:
		headSpec, err := git.Show(ctx, top, "HEAD", specPath)
		if err != nil {
			return wallChangesInputs{}, fmt.Errorf("reading HEAD's %s: %w", specPath, err)
		}
		in.HeadExists = true
		in.HeadSpec = headSpec
	}

	tracked, err := git.TrackedChanges(ctx, top)
	if err != nil {
		return wallChangesInputs{}, fmt.Errorf("listing changed paths: %w", err)
	}
	untracked, err := git.UntrackedPaths(ctx, top)
	if err != nil {
		return wallChangesInputs{}, fmt.Errorf("listing untracked paths: %w", err)
	}
	for _, change := range tracked {
		if change.Path == specPath {
			in.SpecStatus.Reported = true
			in.SpecStatus.IndexDiverged = in.SpecStatus.IndexDiverged || change.Unmerged || (change.Index != '.' && change.Worktree != '.')
			in.SpecStatus.ModeChanged = in.SpecStatus.ModeChanged || (!change.Unmerged && change.ModeHead != change.ModeWorktree)
		} else {
			in.ChangedPaths = append(in.ChangedPaths, wallChangedPath{Path: change.Path})
		}
		switch change.RenamedFrom {
		case "":
		case specPath:
			in.SpecStatus.Reported = true
		default:
			in.ChangedPaths = append(in.ChangedPaths, wallChangedPath{Path: change.RenamedFrom})
		}
	}
	for _, p := range untracked {
		if p == specPath {
			in.SpecStatus.Reported = true
			in.SpecStatus.Untracked = true
			continue
		}
		in.ChangedPaths = append(in.ChangedPaths, wallChangedPath{Path: p, Untracked: true})
	}
	return in, nil
}

// computeWallChanges composes loadWallChangesInputs and classifyWallChanges
// over the production git reader: root's real git state, classified. An
// error here is operational (git itself failed), never the
// unreadable-comparison disclosed state, which classifyWallChanges reports
// inside its result instead.
func computeWallChanges(ctx context.Context, root, specName string, workingSpec []byte) (*wallChanges, error) {
	in, err := loadWallChangesInputs(ctx, gitxWallReader{}, root, specName, workingSpec)
	if err != nil {
		return nil, fmt.Errorf("workbench: wall changes for %s: %w", specName, err)
	}
	return classifyWallChanges(in)
}
