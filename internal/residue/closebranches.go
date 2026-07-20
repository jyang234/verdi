package residue

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// closeBranchPrefix is spec/close-verb dc-3's own closure-branch naming
// convention: close/<name>.
const closeBranchPrefix = "close/"

// CloseClassification is AC-2's total, two-outcome classification for an
// unmerged close/<name> branch (ac-2's static evidence: a single boolean —
// whether archive/<name> is already present at the audited (default
// branch) ref — with no third outcome reachable).
type CloseClassification int

const (
	// RitualIncomplete: archive/<name> is not yet present on the default
	// branch — the closure has not landed there, whether because this
	// branch's own tip already performed the local archive move and is
	// simply stuck unmerged (AC-1 pattern (a)'s own shape, cross-
	// referenced by this classification) or because it has not even
	// reached that step yet. Flags the run (dc-3).
	RitualIncomplete CloseClassification = iota
	// SupersededElsewhere: archive/<name> is ALREADY present on the
	// default branch, through a commit history independent of this
	// close/<name> branch — a redundant leftover. Reported only, never
	// flags (dc-3).
	SupersededElsewhere
)

// String renders c for disclosure — the exact "ritual-incomplete" /
// "superseded-elsewhere" vocabulary AC-2's own spec text names.
func (c CloseClassification) String() string {
	switch c {
	case RitualIncomplete:
		return "ritual-incomplete"
	case SupersededElsewhere:
		return "superseded-elsewhere"
	default:
		return "unknown"
	}
}

// CloseBranch is one unmerged close/<name> local branch — AC-2's own
// finding shape, and (when ArchivedOnOwnTip is also true and the matching
// active-zone spec is still status: accepted-pending-build) AC-1 pattern
// (a)'s witness too. As an implementation choice, patterna.go derives that
// finding from this same slice rather than re-running the tip-tree check,
// so AC-1 pattern (a) and AC-2 classify from one shared pass rather than
// two that could drift apart.
type CloseBranch struct {
	Name             string // "<name>" from close/<name>
	Branch           string // "close/<name>"
	Tip              string // the branch's own tip commit sha
	ArchivedOnOwnTip bool   // archive/<name> already present in THIS branch's own tip tree
	Class            CloseClassification
}

// scanCloseBranches is AC-1 pattern (a)'s and AC-2's SHARED single pass:
// every local close/* branch, restricted to the subset unmerged into
// defaultTip (a merged close/<name> branch is excluded entirely — never
// classified either way) and further restricted to exclude any branch
// whose <name> currently names an active-zone spec at status: superseded
// (dc-2: "status: superseded is explicitly OUT of scope for AC-1/AC-2" —
// both, not AC-1 alone; a superseded spec never goes through the
// archive/close ritual at all — 02 §Kind registry's own parallel-branch
// reading — so a leftover close/<name> branch for a name that has SINCE
// become superseded via the unrelated accept-time supersession flip is
// stale, not an actionable "ritual-incomplete" defect). supersededNames
// holds every CURRENT active-zone spec name at status: superseded — built
// from the UNFILTERED active-spec set, since this is precisely the
// lookup that decides the exclusion, not one that has already had it
// applied. Each classified branch carries the one shared tip-tree
// presence check (archiveExistsAt) — read against this branch's own tip
// (feeding ArchivedOnOwnTip, which patterna.go consumes) AND against
// defaultTip (feeding Class, AC-2's own classification) — so both callers
// read from one implementation of the archive-path-presence check, never
// two that could silently disagree. Sorted by name for a deterministic
// report.
func scanCloseBranches(ctx context.Context, root, defaultTip string, supersededNames map[string]bool) ([]CloseBranch, error) {
	branches, err := gitx.LocalBranches(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("residue: listing local branches: %w", err)
	}

	var out []CloseBranch
	for _, branch := range branches {
		name, ok := closeBranchName(branch)
		if !ok {
			continue
		}
		if supersededNames[name] {
			continue // dc-2: excluded before classification runs, for AC-2 same as AC-1
		}

		tip, err := gitx.RevParse(ctx, root, branch)
		if err != nil {
			return nil, fmt.Errorf("residue: resolving %s: %w", branch, err)
		}

		merged, err := gitx.IsAncestor(ctx, root, tip, defaultTip)
		if err != nil {
			return nil, fmt.Errorf("residue: checking %s merged: %w", branch, err)
		}
		if merged {
			continue // AC-2: a merged close/<name> branch is never classified either way
		}

		archivedOnOwnTip, err := archiveExistsAt(ctx, root, tip, name)
		if err != nil {
			return nil, fmt.Errorf("residue: checking %s's own tip for archive/%s: %w", branch, name, err)
		}
		archivedOnDefault, err := archiveExistsAt(ctx, root, defaultTip, name)
		if err != nil {
			return nil, fmt.Errorf("residue: checking default branch for archive/%s: %w", name, err)
		}

		class := RitualIncomplete
		if archivedOnDefault {
			class = SupersededElsewhere
		}

		out = append(out, CloseBranch{
			Name:             name,
			Branch:           branch,
			Tip:              tip,
			ArchivedOnOwnTip: archivedOnOwnTip,
			Class:            class,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// closeBranchName reports whether branch matches the close/<name>
// convention (spec/close-verb dc-3), returning name.
func closeBranchName(branch string) (name string, ok bool) {
	if !strings.HasPrefix(branch, closeBranchPrefix) {
		return "", false
	}
	name = strings.TrimPrefix(branch, closeBranchPrefix)
	return name, name != ""
}

// archiveExistsAt is the ONE tip-tree check AC-1 pattern (a) and AC-2's
// classification both read from — a single implementation shared by both
// callers (an engineering choice) so they cannot silently disagree:
// whether .verdi/specs/archive/<name>/spec.md exists in ref's tree, read
// via git plumbing (gitx.LsTree) — never a working-tree file check, since
// ref may be a branch tip that is not (and, being unmerged, cannot safely
// be) checked out.
func archiveExistsAt(ctx context.Context, root, ref, name string) (bool, error) {
	relPath := store.SpecRelPath(store.ZoneArchive, name)
	entries, err := gitx.LsTree(ctx, root, ref, relPath)
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}
