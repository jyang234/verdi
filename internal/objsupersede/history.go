package objsupersede

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
// (specstate.ResolveDefaultBranch) of the repository at a store root.
type History struct {
	root   string
	branch specstate.Branch
	ok     bool
}

// NewHistory resolves root's default branch. An unresolved branch makes
// every later fact unproven rather than failing here.
func NewHistory(ctx context.Context, root string) History {
	b, ok := specstate.ResolveDefaultBranch(ctx, root)
	return History{root: root, branch: b, ok: ok}
}

func unproven(witness string) Fact { return Fact{State: FactUnproven, Witness: witness} }

// noBranch is the witness when the default branch does not resolve.
const noBranch = "the default branch could not be resolved (no CI_DEFAULT_BRANCH, no configured remote HEAD, and no single origin/main or origin/master)"

// Acceptance is SI-270's acceptance point of spec: the earliest commit on
// the default branch's first-parent chain whose tree holds the spec's
// spec.md in either zone, dated by its committer date in UTC. A later
// in-place edit or archive move does not move it. A shallow history, an
// unresolved default branch, or an unreadable commit is unproven.
func (h History) Acceptance(ctx context.Context, spec string) Fact {
	if !h.ok {
		return unproven(noBranch)
	}
	paths := []string{store.SpecRelPath(store.ZoneActive, spec), store.SpecRelPath(store.ZoneArchive, spec)}
	commits, err := gitx.FirstParentPathCommits(ctx, h.root, h.branch.Ref, paths...)
	if errors.Is(err, gitx.ErrShallowHistory) {
		return unproven("shallow history: the first-parent chain of " + h.branch.Ref + " is incomplete")
	}
	if err != nil {
		return unproven(err.Error())
	}
	for _, c := range commits {
		for _, p := range paths {
			present, err := gitx.PathExistsAt(ctx, h.root, c, p)
			if err != nil {
				return unproven(err.Error())
			}
			if present {
				return h.dated(ctx, c)
			}
		}
	}
	return Fact{State: FactAbsent}
}

// Closed is SI-270's closed date of t: the committer date of the landing
// commit of t's specstate Closed baseline, read through
// internal/specstate. A spec specstate does not project Closed is absent;
// an Unproven projection is unproven with its disclosures.
func (h History) Closed(ctx context.Context, t *Spec) Fact {
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

// Establishment implements Establisher: successor's supersession of object
// is in force when successor is accepted and design §3's new-replacement
// match held on its acceptance commit's tree (SI-270).
func (h History) Establishment(ctx context.Context, successor string, object artifact.Ref) Establishment {
	acc := h.Acceptance(ctx, successor)
	switch acc.State {
	case FactAbsent:
		return Establishment{Reason: ReasonEstablisherNotAccepted}
	case FactUnproven:
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: acc.Witness}
	}
	recs, err := ReadRecords(ctx, CommitTree{Root: h.root, Commit: acc.Commit})
	if err != nil {
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: err.Error()}
	}
	if detail := recs.newMatchFails(ctx, successor, object); detail != "" {
		return Establishment{Reason: ReasonEstablisherNotInForce, Detail: detail}
	}
	return Establishment{Commit: acc.Commit, Date: acc.Date}
}

// newMatchFails evaluates successor's edges to object under the
// new-replacement rules, plus completeness for object's spec, and returns
// the first failing reason's text, or "" when the match holds.
func (recs *Records) newMatchFails(ctx context.Context, successor string, object artifact.Ref) string {
	s := recs.Specs[successor]
	if s == nil {
		return fmt.Sprintf("spec/%s does not decode in its acceptance commit's tree", successor)
	}
	found := false
	for _, d := range s.FM.Decisions {
		for _, l := range d.Links {
			if ref, ok := fragmentEdge(l); ok && ref == object {
				found = true
				if r := recs.evaluateEdge(ctx, s, d.ID, l.Ref, ref, false, nil); r.Outcome != ResolvedNew {
					return textOrError(r)
				}
			}
		}
	}
	if !found {
		return fmt.Sprintf("spec/%s carries no edge to %s", successor, object)
	}
	for _, r := range recs.completeness(s) {
		if ref, err := artifact.ParseRef(r.Edge); err != nil || ref.Name == object.Name {
			return textOrError(r)
		}
	}
	return ""
}

// textOrError renders a result this package built; a rendering error is
// reported inline, never dropped.
func textOrError(r Result) string {
	text, err := r.Text()
	if err != nil {
		return err.Error()
	}
	return text
}
