// Package specname is the one shared seam that validates a spec's proposed
// name before ANY branch-cutting creation surface composes, scaffolds, or
// commits anything under it (spec/uat-round-1 UAT-030/031/032, wave-3
// review fix round): the plain `verdi design start --kind/--name` path,
// `design start --supersedes`, and the board's create and Revise actions
// all call ValidateSuccessorName exactly once, before their own first
// write or branch cut.
//
// Moved out of internal/supersede, where the check began (the board's
// Revise action was the first non-CLI caller of the successor-side half of
// the supersede workflow — see that package's own resolve.go/compose.go
// doc comments). UAT-030/031/032's fix widens this predicate's callers to
// the plain `design start` path and the board's create action, NEITHER of
// which supersedes anything — a fresh spec's own name has no predecessor
// at all. Importing "supersede" purely to validate a brand-new name's
// shape and uniqueness would accumulate a second, unrelated concern onto
// that package (CLAUDE.md: "one package = one concern ... split before a
// package accumulates a second concern"), so the predicate lives here
// instead; internal/supersede keeps a thin re-export (its own validate.go)
// for the two callers that ARE supersede-flavored (`design start
// --supersedes`; the board's Revise action), so their code keeps reading
// supersede.ValidateSuccessorName without a forced rename.
package specname

import (
	"context"
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// Reason is a refusal's classification — a distinguishable, typed value
// (never only a prose-matched string) so a caller can branch on WHY a name
// was refused without parsing Error() text.
type Reason string

const (
	// ReasonInvalidName means the proposed name does not parse as a spec
	// ref (02 §Identity and references: kebab-case, and VL-002's
	// path-bound identity means the name IS the store directory), or it
	// parses but carries a "#fragment" or "@commit" decoration a bare
	// spec NAME must never have (UAT-030: those decorate a REFERENCE to a
	// spec — 02 §Identity and references' "Fragment ref"/pinned forms —
	// never a spec's own name, which is exactly what ParseRef alone
	// cannot tell apart, since a bare name is itself a valid, undecorated
	// reference).
	ReasonInvalidName Reason = "invalid-name"
	// ReasonSuccessorExists means a spec directory of this name already
	// exists in root's ACTIVE zone — the collision that must refuse
	// BEFORE anything is written or any branch is cut.
	ReasonSuccessorExists Reason = "successor-exists"
	// ReasonArchivedExists means a spec directory of this name already
	// exists in root's ARCHIVE zone (UAT-032: names are unique across
	// active and archived specs, guide 6.1).
	ReasonArchivedExists Reason = "archived-exists"
	// ReasonExistsOnBase means the name is absent from the CURRENT
	// checkout's working tree (both checks above passed) but is already
	// present — active or archived — on the ref the caller's new branch
	// will actually be cut from (UAT-031). A caller's checkout can be
	// behind that ref (a serving checkout not yet fetched/merged past a
	// spec that landed on the default branch, or a managed worktree cut
	// earlier): the branch-cutting machinery bases the new branch on THAT
	// ref, never the checkout's own working tree or HEAD, so a
	// working-tree-only collision check would pass while the cut would
	// silently replace the same-named spec already landed there.
	ReasonExistsOnBase Reason = "exists-on-base"
)

// NameError is ValidateSuccessorName's typed refusal: Reason classifies
// why, Name carries the rejected name exactly as submitted, Path names the
// colliding directory or (ReasonExistsOnBase) the store-relative path that
// was found on the base ref, and — ReasonInvalidName only — Unwrap exposes
// the plain "why" as its own error (either the real artifact.ParseRef
// failure, or, for a name that parsed fine but carries a "#fragment"/
// "@commit" decoration, a same-shaped error naming that instead), so every
// ReasonInvalidName case can be rendered the same way: name the flag, then
// %v the unwrapped reason.
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

// ExistsOnBaseDetail renders ReasonExistsOnBase's operator-facing "why" —
// exported (F1, the wave-3 review's own fix round on this lane) so every
// caller's own switch-case rendering reuses the identical wording instead
// of each re-deriving it (as all four originally did, byte-for-byte
// identical apart from their own verb-name prefix — this is that shared
// text, now the single place it is spelled).
//
// baseRef == "HEAD" is dc-7's own disclosed fallback (stubinstantiate.
// ResolvedBase.HeadDisclosed / resolveDesignStartBase's "HEAD" return): no
// "origin" remote is configured at all, so the branch's base IS the
// calling checkout's own current HEAD. In that shape "this checkout is
// behind HEAD" is not just imprecise, it is backwards — the checkout IS at
// HEAD by definition — and "fetch/pull" names a remote that does not
// exist. The reachable witness: a fresh, remote-less store where the spec
// is committed at HEAD but its working-tree directory was since deleted
// without committing that deletion — store.ActiveSpecDir's stat (the
// second check, above) finds nothing, yet the name is still exactly as
// taken as it was, one commit ago, at this branch's own base. Every OTHER
// baseRef (a real branch or remote-tracking ref name, e.g. "main" or
// "origin/main") keeps the original wording byte-for-byte: there the
// checkout genuinely can be behind that ref, and fetch/pull is the correct
// remedy.
func ExistsOnBaseDetail(name, baseRef string) string {
	if baseRef == "HEAD" {
		return fmt.Sprintf("spec/%s already exists at this branch's base (HEAD) though it is absent from the working tree; commit or restore it before reusing this name", name)
	}
	return fmt.Sprintf("spec/%s already exists on %s — this checkout is behind %s; fetch/pull before starting a new spec of this name", name, baseRef, baseRef)
}

// ValidateSuccessorName proves every precondition a branch-cutting creation
// surface shares before composing or scaffolding anything under name: it
// parses as a plain (unpinned, unfragmented) spec name — returned as an
// artifact.Ref so the caller need not re-parse it — and no spec directory
// of that name already exists in root's active OR archive zone.
//
// ctx and baseRef add UAT-031's third check, OPTIONAL: when baseRef is
// non-empty, ValidateSuccessorName additionally refuses if name is already
// present — active or archived — at baseRef, the ref the caller's new
// branch will actually be cut from (specstate.ResolveDefaultBranch's own
// resolution, or dc-7's disclosed HEAD fallback — never assumed to be the
// current checkout's HEAD). A caller with no base ref in hand yet passes
// "" and skips this third check. ctx is required unconditionally (even
// though it is unused when baseRef == "") because every function in this
// codebase that CAN do I/O takes one as its first parameter (CLAUDE.md);
// gitx.BlobAt is the only I/O this function ever performs.
//
// NOT covered here, deliberately: whether a `design/<name>` BRANCH already
// exists. That is a Git question this check's own base-ref probe does not
// answer either (a branch can exist with no spec of that name landed on
// its own base yet) — each caller's own branch-creation primitive already
// owns it: the CLI's gitx.CheckoutNewBranchFrom refuses an existing branch
// before switching, and stubinstantiate.CommitScaffoldBranch's UpdateRef is
// create-only. A caller that updates a ref directly some other way must
// make that check itself.
func ValidateSuccessorName(ctx context.Context, root, name, baseRef string) (artifact.Ref, error) {
	ref, err := artifact.ParseRef("spec/" + name)
	if err != nil {
		return artifact.Ref{}, &NameError{
			Reason: ReasonInvalidName,
			Name:   name,
			Detail: fmt.Sprintf("specname: %q is not a valid spec name: %v", name, err),
			Err:    err,
		}
	}
	// UAT-030: "#fragment" and "@commit" decorate a REFERENCE to a spec
	// (02 §Identity and references), never a spec's own name. Checked
	// after ParseRef succeeds, as its own distinct case (rather than
	// folded into the error above), so a name that fails to parse for an
	// unrelated reason keeps ITS OWN wrapped error available via Unwrap.
	if ref.Fragment() || ref.Pinned() {
		decoration := "a \"#fragment\" suffix"
		switch {
		case ref.Fragment() && ref.Pinned():
			decoration = "a \"#fragment\" suffix and an \"@commit\" pin"
		case ref.Pinned():
			decoration = "an \"@commit\" pin"
		}
		decorationErr := fmt.Errorf("a spec's own name must not carry %s (02 §Identity and references)", decoration)
		return artifact.Ref{}, &NameError{
			Reason: ReasonInvalidName,
			Name:   name,
			Detail: fmt.Sprintf("specname: %q is not a valid spec name: %v", name, decorationErr),
			Err:    decorationErr,
		}
	}

	if dir := store.ActiveSpecDir(root, name); direxists(dir) {
		return artifact.Ref{}, &NameError{
			Reason: ReasonSuccessorExists,
			Name:   name,
			Path:   dir,
			Detail: fmt.Sprintf("specname: %s already exists", dir),
		}
	}
	if dir := store.ArchiveSpecDir(root, name); direxists(dir) {
		return artifact.Ref{}, &NameError{
			Reason: ReasonArchivedExists,
			Name:   name,
			Path:   dir,
			Detail: fmt.Sprintf("specname: spec %s already exists under specs/archive/ — names are unique across active and archived specs (guide 6.1)", name),
		}
	}

	if baseRef != "" {
		for _, relPath := range []string{store.ActiveSpecRelPath(name), store.SpecRelPath(store.ZoneArchive, name)} {
			_, found, err := gitx.BlobAt(ctx, root, baseRef, relPath)
			if err != nil {
				return artifact.Ref{}, fmt.Errorf("specname: checking %s for an existing spec/%s: %w", baseRef, name, err)
			}
			if found {
				return artifact.Ref{}, &NameError{
					Reason: ReasonExistsOnBase,
					Name:   name,
					Path:   relPath,
					Detail: "specname: " + ExistsOnBaseDetail(name, baseRef),
				}
			}
		}
	}

	return ref, nil
}

// direxists reports whether path exists at all — a directory or a plain
// file, matching os.Stat's own "something is there" semantics (the store
// layout always uses directories, but a caller-corrupted or hand-crafted
// checkout with a plain FILE at the spec directory's path must refuse
// exactly the same way, never be silently treated as free).
func direxists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
