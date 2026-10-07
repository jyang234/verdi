package workbench

// The wall's readiness marks, projection side (spec/wall-canvas-v2 ac-1,
// dc-1; ledger SI-350 (1)–(2), SI-352, SI-360): which card carries which
// mark, and the one unavailable notice when the marks' input cannot be
// read. These are non-presentation facts (owner F-3) over the readiness
// snapshot a composed refresh loaded in its own read session
// (projectWallRefresh); the dot, chip and notice markup are a renderer's.
// Nothing here is cached or kept past the request (readiness-recovery-v2
// co-2), and the readiness snapshot gains no field (SI-360 (1)).

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/journey"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// wallMarks is one wall's readiness marks. When Unavailable is set the
// marks' input could not be read: it is the notice's one reason, and no
// card carries a mark (SI-350 (2)). Otherwise Objects maps an object
// card's id to the Focus next concerns whose Object names it (SI-345), in
// Focus next order, and Stubs maps a stub card's slug to its
// stub-unreconciled concern; a card named by nothing has no entry.
type wallMarks struct {
	Objects     map[string][]wallMark `json:"objects,omitempty"`
	Stubs       map[string]wallMark   `json:"stubs,omitempty"`
	Unavailable string                `json:"unavailable,omitempty"`
}

// wallMark is one Focus next concern naming a card, and its chip word.
type wallMark struct {
	Concern string `json:"concern"`
	Chip    string `json:"chip"`
}

// The marks' chip words (SI-350 (1), SI-360 (4)): "no stub" for the
// coverage family, "unresolved" for every other family.
const (
	markChipNoStub     = "no stub"
	markChipUnresolved = "unresolved"
)

// coverageConcernPrefix is the coverage family, success/coverage/<ac>
// (SI-338 (4)): an acceptance criterion no non-spike stub lists.
const coverageConcernPrefix = "success/coverage/"

// stubConcernPrefix is the stub-unreconciled family as the readiness
// derivation spells it: a review blocker whose journey id is
// stub-unreconciled/<journey.SanitizeStubSlug(slug)>.
const stubConcernPrefix = "review/blocker/stub-unreconciled/"

// marksUnwired is the unavailable notice's reason when no readiness was
// derived for the wall: the server wires no readiness loader.
const marksUnwired = "no readiness loader is wired in this server, so no readiness is derived for this wall"

// marksUnreadable is the reason unavailableMarks falls back to when given
// none: the notice always says why.
const marksUnreadable = "the readiness marks' input could not be read"

// wallMarksInput is what one wall's marks derive from: the wall's own
// identity (its spec ref, branch and worktree HEAD) and cards, and the
// readiness its composed refresh loaded, or the load's failure.
type wallMarksInput struct {
	Ref, Branch, Head string
	ObjectIDs         []string
	StubSlugs         []string
	Readiness         *readinesspilot.Snapshot
	LoadErr           error
}

// wallMarksInputFor is the marks' input for one loaded wall of spec name:
// its object cards and stub cards, its branch and worktree HEAD, and the
// readiness its refresh loaded, or the load's failure.
func wallMarksInputFor(name string, proj *BoardProjection, asd *asdView, readiness *readinesspilot.Snapshot, loadErr error) wallMarksInput {
	in := wallMarksInput{
		Ref: "spec/" + name, Branch: asd.Branch, Head: asd.WorktreeHead,
		Readiness: readiness, LoadErr: loadErr,
	}
	for _, c := range proj.Cards {
		in.ObjectIDs = append(in.ObjectIDs, c.ID)
	}
	for _, sv := range proj.StubViews {
		in.StubSlugs = append(in.StubSlugs, sv.Slug)
	}
	return in
}

// unavailableMarks is the marks of a wall whose marks' input cannot be
// read: the one reason, and no mark.
func unavailableMarks(reason string) wallMarks {
	if reason == "" {
		reason = marksUnreadable
	}
	return wallMarks{Unavailable: reason}
}

// instanceMarks is the readiness marks fixed for this server instance
// (SI-360 (3); SI-362 (1); SI-364 (3)): on a /b/ wall whose branch is not
// the serving root's, and on a server with no readiness loader wired, the
// marks' input can never be read, so every response that renders the
// region — the page, the poll, the fragment and a mutation's fresh
// projection — carries the one unavailable reason, computed without
// loading readiness, and their regions and tokens agree. nil on a
// serving-root wall with a loader, whose marks only a composed refresh
// derives.
func (s *boardSpecServer) instanceMarks() *wallMarks {
	var reason string
	switch {
	case s.fixedBranch != "":
		reason = marksBranchWall(s.fixedBranch)
	case s.readinessLoader == nil:
		reason = marksUnwired
	default:
		return nil
	}
	marks := unavailableMarks(reason)
	return &marks
}

// marksBranchWall is the reason on a /b/ wall whose branch is not the
// serving root's (SI-360 (3)): no readiness is loaded there, because the
// readiness loader reads the serving checkout, whose branch differs from
// the wall's (SI-338).
func marksBranchWall(branch string) string {
	return fmt.Sprintf("this wall serves branch %s from its own working tree, and readiness derives only for the serving checkout's branch, so the two cannot describe one commit", branch)
}

// marksSealed is the reason on a remote-only branch's sealed render
// (SI-352 (1)).
func marksSealed(ref string) string {
	return fmt.Sprintf("this wall is a read-only render of remote-tracking ref %s's committed content, for which no readiness is derived", ref)
}

// deriveWallMarks derives one wall's marks from its composed readiness.
// The input is unreadable — one reason, no mark — when the load failed,
// when nothing was loaded, when the snapshot's spec, branch or HEAD is not
// the wall's (SI-338), or when two stub cards map to one stub concern, so
// its mark could not name one card. Otherwise every Focus next concern
// names the object card its Object is, and the stub card whose slug
// journey's own rule maps to its id (SI-360 (1)); each mark carries its
// chip word.
func deriveWallMarks(in wallMarksInput) wallMarks {
	if in.LoadErr != nil {
		return unavailableMarks("the readiness load failed: " + in.LoadErr.Error())
	}
	snap := in.Readiness
	if snap == nil {
		return unavailableMarks(marksUnwired)
	}
	if snap.TargetRef != in.Ref || snap.Branch != in.Branch || snap.Head != in.Head {
		return unavailableMarks(fmt.Sprintf("the readiness snapshot describes %s on branch %s at HEAD %s, and this wall shows %s on branch %s at HEAD %s",
			orUnresolved(snap.TargetRef), orUnresolved(snap.Branch), orUnresolved(snap.Head),
			orUnresolved(in.Ref), orUnresolved(in.Branch), orUnresolved(in.Head)))
	}

	objects := make(map[string]bool, len(in.ObjectIDs))
	for _, id := range in.ObjectIDs {
		objects[id] = true
	}
	stubsByConcern := make(map[string][]string, len(in.StubSlugs))
	for _, slug := range in.StubSlugs {
		id := stubConcernPrefix + journey.SanitizeStubSlug(slug)
		stubsByConcern[id] = append(stubsByConcern[id], slug)
	}

	var out wallMarks
	for _, c := range snap.Attention {
		mark := wallMark{Concern: c.ID, Chip: markChip(c.ID)}
		if c.Object != "" && objects[c.Object] {
			if out.Objects == nil {
				out.Objects = map[string][]wallMark{}
			}
			out.Objects[c.Object] = append(out.Objects[c.Object], mark)
		}
		switch slugs := stubsByConcern[c.ID]; len(slugs) {
		case 0:
		case 1:
			if out.Stubs == nil {
				out.Stubs = map[string]wallMark{}
			}
			out.Stubs[slugs[0]] = mark
		default:
			sorted := append([]string(nil), slugs...)
			sort.Strings(sorted)
			return unavailableMarks(fmt.Sprintf("stubs %s share the readiness concern %s, so its mark cannot name one card",
				strings.Join(sorted, " and "), c.ID))
		}
	}
	return out
}

// markChip is a concern's chip word.
func markChip(concernID string) string {
	if strings.HasPrefix(concernID, coverageConcernPrefix) {
		return markChipNoStub
	}
	return markChipUnresolved
}

// orUnresolved names an empty identity fact as unresolved.
func orUnresolved(s string) string {
	if s == "" {
		return "(unresolved)"
	}
	return s
}
