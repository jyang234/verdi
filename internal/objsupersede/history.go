package objsupersede

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// FactState is a history fact's three-valued state.
type FactState string

const (
	FactProven   FactState = "proven"
	FactAbsent   FactState = "absent"   // proven not to hold
	FactUnproven FactState = "unproven" // Witness names what is missing
)

// Fact is one acceptance or closure fact from the default branch's history:
// its commit and that commit's committer date (YYYY-MM-DD, UTC) when
// proven, or the missing witness when unproven. No date is ever guessed.
type Fact struct {
	State   FactState
	Commit  string
	Date    string
	Witness string
}

// History reads acceptance facts from the configured default branch
// (specstate.ResolveDefaultBranch) of the repository at a store root, at
// the branch's commit NewHistory pins: every fact of one History reads the
// same history. Copies share its memo, which is safe for concurrent use.
type History struct {
	root   string
	branch specstate.Branch
	ok     bool
	head   string // the default branch's commit, pinned by NewHistory
	unread string // why head could not be pinned, when it could not
	cache  *historyCache
}

// historyCache is a History's memo: each spec's first presence, each
// (successor, closed spec) walk list, the positions of the store's
// first-parent commits, and the engine that answers its establishments
// (per (S_k, T, head), SI-270 as amended).
type historyCache struct {
	mu     sync.Mutex          // guards firsts, walks, and order
	firsts map[string]Fact     // per spec
	walks  map[string]walkList // per (successor, closed spec)
	order  map[string]int      // nil until read
	orderF Fact                // unproven when order cannot be read
	engMu  sync.Mutex          // serializes Establishment over eng
	eng    *engine
}

// NewHistory resolves root's default branch and pins its commit. An
// unresolved branch or commit makes every later fact unproven rather than
// failing here.
func NewHistory(ctx context.Context, root string) History {
	h := History{root: root, cache: &historyCache{firsts: map[string]Fact{}, walks: map[string]walkList{}}}
	h.branch, h.ok = specstate.ResolveDefaultBranch(ctx, root)
	if h.ok {
		head, err := gitx.RevParse(ctx, root, h.branch.Ref+"^{commit}")
		if err != nil {
			h.unread = err.Error()
		}
		h.head = head
	}
	return h
}

func unproven(witness string) Fact { return Fact{State: FactUnproven, Witness: witness} }

// noBranch is the witness when the default branch does not resolve.
const noBranch = "the default branch could not be resolved (no CI_DEFAULT_BRANCH, no configured remote HEAD, and no single origin/main or origin/master)"

// Acceptance is spec's acceptance (02 §Kind registry: merging is
// acceptance): the earliest commit on the default branch's first-parent
// chain whose tree holds the spec's spec.md in either zone, dated by its
// committer date in UTC. A later in-place edit or archive move does not
// move it. Whether a supersession it issues is in force, and since when,
// is Establishment's answer (SI-270 as amended). A shallow history, an
// unresolved default branch, or an unreadable commit is unproven.
func (h History) Acceptance(ctx context.Context, spec string) Fact {
	f := h.first(ctx, spec)
	if f.State != FactProven {
		return f
	}
	return h.dated(ctx, f.Commit)
}

// first is spec's first presence on the pinned commit's first-parent
// chain: FactProven naming the earliest commit whose tree holds spec's
// spec.md in either zone, FactAbsent when none does, or FactUnproven with
// the witness; memoized per spec.
func (h History) first(ctx context.Context, spec string) Fact {
	if h.cache == nil {
		return h.readFirst(ctx, spec)
	}
	h.cache.mu.Lock()
	defer h.cache.mu.Unlock()
	f, ok := h.cache.firsts[spec]
	if !ok {
		f = h.readFirst(ctx, spec)
		h.cache.firsts[spec] = f
	}
	return f
}

// readFirst reads first's fact.
func (h History) readFirst(ctx context.Context, spec string) Fact {
	switch {
	case !h.ok:
		return unproven(noBranch)
	case h.unread != "":
		return unproven(h.unread)
	}
	paths := specPaths(spec)
	commits, err := gitx.FirstParentPathCommits(ctx, h.root, h.head, paths...)
	if err != nil {
		return h.historyError(err)
	}
	first, err := firstHolding(ctx, h.root, commits, paths)
	switch {
	case err != nil:
		return unproven(err.Error())
	case first == "":
		return Fact{State: FactAbsent}
	}
	return Fact{State: FactProven, Commit: first}
}

// specPaths are spec's spec.md paths in the active and archive zones.
func specPaths(spec string) []string {
	return []string{store.SpecRelPath(store.ZoneActive, spec), store.SpecRelPath(store.ZoneArchive, spec)}
}

// walk implements walkSource, memoized per (successor, closed spec).
func (h History) walk(ctx context.Context, successor, closed string) ([]string, Fact) {
	f := h.first(ctx, successor)
	if f.State != FactProven {
		return nil, f
	}
	if h.cache == nil {
		return h.readWalk(ctx, f.Commit, successor, closed)
	}
	key := successor + "\x00" + closed
	h.cache.mu.Lock()
	defer h.cache.mu.Unlock()
	w, ok := h.cache.walks[key]
	if !ok {
		w.commits, w.fact = h.readWalk(ctx, f.Commit, successor, closed)
		h.cache.walks[key] = w
	}
	return w.commits, w.fact
}

// readWalk reads the (successor, closed) walk (walkSource) from the pinned
// commit's first-parent chain: the commits that change successor's or
// closed's spec.md in either zone or .verdi/conflicts/ (SI-281's walk
// set), from first, the first whose tree holds successor's spec.md.
func (h History) readWalk(ctx context.Context, first, successor, closed string) ([]string, Fact) {
	paths := append(append(specPaths(successor), specPaths(closed)...), conflictsDir)
	all, err := gitx.FirstParentPathCommits(ctx, h.root, h.head, paths...)
	if err != nil {
		return nil, h.historyError(err)
	}
	i := slices.Index(all, first)
	if i < 0 {
		return nil, unproven(fmt.Sprintf("the commit %s that first holds spec/%s is missing from its walk", first, successor))
	}
	return all[i:], Fact{State: FactProven, Commit: first}
}

// position implements walkSource: commit's index among the pinned
// commit's first-parent commits that change a spec or a conflict, the
// commits every walk lists, oldest first; read once per History.
func (h History) position(ctx context.Context, commit string) (int, error) {
	if h.cache == nil {
		return 0, fmt.Errorf("objsupersede: a zero History has no first-parent order")
	}
	h.cache.mu.Lock()
	defer h.cache.mu.Unlock()
	if h.cache.order == nil && h.cache.orderF.State == "" {
		h.cache.order, h.cache.orderF = h.readOrder(ctx)
	}
	if h.cache.orderF.State == FactUnproven {
		return 0, errors.New(h.cache.orderF.Witness)
	}
	p, ok := h.cache.order[commit]
	if !ok {
		return 0, fmt.Errorf("objsupersede: commit %s is not on %s's first-parent chain of store records", commit, h.branch.Ref)
	}
	return p, nil
}

// readOrder reads position's commits.
func (h History) readOrder(ctx context.Context) (map[string]int, Fact) {
	switch {
	case !h.ok:
		return nil, unproven(noBranch)
	case h.unread != "":
		return nil, unproven(h.unread)
	}
	commits, err := gitx.FirstParentPathCommits(ctx, h.root, h.head, specsDir, conflictsDir)
	if err != nil {
		return nil, h.historyError(err)
	}
	order := make(map[string]int, len(commits))
	for i, c := range commits {
		order[c] = i
	}
	return order, Fact{State: FactProven}
}

// firstHolding returns the first of commits whose tree holds any of paths,
// or "" when none does.
func firstHolding(ctx context.Context, root string, commits, paths []string) (string, error) {
	for _, c := range commits {
		for _, p := range paths {
			present, err := gitx.PathExistsAt(ctx, root, c, p)
			if err != nil || present {
				return c, err
			}
		}
	}
	return "", nil
}

// historyError is the unproven fact for a failed first-parent read.
func (h History) historyError(err error) Fact {
	if errors.Is(err, gitx.ErrShallowHistory) {
		return unproven("shallow history: the first-parent chain of " + h.branch.Ref + " is incomplete")
	}
	return unproven(err.Error())
}

// Closed is SI-270's closed date of t: the committer date of the landing
// commit of t's specstate Closed baseline, read through
// internal/specstate. A spec specstate does not project Closed is absent;
// an Unproven projection, or a shallow history (whose first-parent chain
// cannot prove a landing), is unproven.
func (h History) Closed(ctx context.Context, t *Spec) Fact {
	if shallow, err := gitx.IsShallow(ctx, h.root); err != nil {
		return unproven(err.Error())
	} else if shallow {
		return unproven("shallow history: the default branch's first-parent chain is incomplete")
	}
	res, err := specstate.NewProjector().Resolve(ctx, h.root, specstate.Candidate{Path: t.Path, Content: t.Raw})
	switch {
	case err != nil:
		return unproven(err.Error())
	case res.State == specstate.Unproven:
		return unproven(strings.Join(res.Disclosures, "; "))
	case res.State != specstate.Closed || res.Baseline == nil:
		return Fact{State: FactAbsent}
	}
	return h.dated(ctx, res.Baseline.LandingCommit)
}

// dated returns a proven fact at commit, dated in UTC.
func (h History) dated(ctx context.Context, commit string) Fact {
	iso, err := gitx.CommitDate(ctx, h.root, commit)
	if err != nil {
		return unproven(err.Error())
	}
	day, err := utcDay(iso)
	if err != nil {
		return unproven(err.Error())
	}
	return Fact{State: FactProven, Commit: commit, Date: day}
}

// utcDay renders an RFC 3339 committer date as its UTC day, YYYY-MM-DD
// (SI-270), whatever offset the committer recorded.
func utcDay(iso string) (string, error) {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "", fmt.Errorf("objsupersede: committer date %q: %w", iso, err)
	}
	return t.UTC().Format("2006-01-02"), nil
}

// Establishment implements Establisher: the acceptance walk's answer
// (establish.go) over the pinned default-branch history, memoized for
// this History and its copies.
func (h History) Establishment(ctx context.Context, successor string, object artifact.Ref) Establishment {
	if h.cache == nil {
		return newEngine(h).establishment(ctx, successor, object)
	}
	h.cache.engMu.Lock()
	defer h.cache.engMu.Unlock()
	if h.cache.eng == nil {
		h.cache.eng = newEngine(h)
	}
	return h.cache.eng.establishment(ctx, successor, object)
}

// recordsAt implements walkSource: the records of commit's tree in h's
// repository.
func (h History) recordsAt(ctx context.Context, commit string) (*Records, error) {
	return ReadRecords(ctx, CommitTree{Root: h.root, Commit: commit})
}
