// The stub-match subsystem (spec/file-topics ac-2; 03 §Stub reconciliation,
// R4-I-12): computeStubMatch and its helpers, implementing R4-I-12's
// four-condition stub-match test that gives a story spec's acceptance its
// single-approver fast path — moved verbatim out of accept.go, which had
// grown three subsystems into one 587-line file. This is the production
// twin stubmatch_test.go always named. This file owns exactly this topic:
// nothing else.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// computeStubMatch implements R4-I-12's four-condition stub-match test for
// a story spec being accepted (03 §Lifecycle: the feature-first cascade,
// "Stub-matched fast path"): the story's implements fragment set equals a
// stub's declared AC set, RefSlug(title) equals that same stub's slug, the
// story introduces no disqualifying supersedes/exempts edges (the rung-3
// chain edge to a predecessor story spec is exempt — see
// disqualifyingSupersedesOrExempts), and it carries no undispositioned
// judged findings. It never fails runAccept outright —
// a non-match just means "full review applies" (03: "Stories that deviate
// from the plan ... get full review"), so every miss degrades to
// (false, reason) rather than an error.
//
// Condition (d), "no undispositioned judged findings": at accept time (a
// design-branch action, before any build) the only judged-findings surface
// this system defines is the design-branch decision-conflict report (03
// §Decision-conflict gate), the exact artifact `verdi align`'s design-branch
// mode writes (align_design.go). This function reads that report with its
// own contract — the verdi.decisionconflict/v1 schema and
// ConflictFinding.Dispositioned() rule the reader in align_design.go already
// uses (03: "structured into the same computed/judged split", but with the
// decision-conflict report's own four-value disposition vocabulary) — at the
// conventional path
// .verdi/specs/active/<name>/decision-conflict-report.md. Its absence is
// read as "no judged findings exist yet to disposition" (vacuously
// satisfied), not as a failure: unlike the merge gate's condition 3 (which
// requires a FRESH report to exist because build-branch alignment is
// mandatory machinery), the design-branch sweep is optional exploratory
// tooling this phase does not build, so a store without one yet must not
// have every story spec permanently stub-match-ineligible.
func computeStubMatch(root string, story *artifact.SpecFrontmatter) (matched bool, reason string) {
	featureName, acIDs, err := storyImplementsTarget(story)
	if err != nil {
		return false, err.Error()
	}
	if featureName == "" {
		// Split the two materially different zero-implements cases (D-4): a
		// spike carries no implements edges BY DESIGN (02 §Kind registry:
		// stub-matching is not applicable to it), whereas a non-spike story
		// with zero implements is a malformed story. Conflating them into one
		// "spike or malformed" message is exactly the kind of ambiguous
		// disclosure this feature exists to end.
		if story.Spike {
			return false, "spike: stub-matching is not applicable (a spike carries no implements edges, 02 §Kind registry)"
		}
		return false, "no implements edges (malformed story)"
	}

	feature, err := storyresolve.LoadActiveSpec(root, featureName)
	if err != nil {
		return false, fmt.Sprintf("implements-target feature %q could not be loaded: %v", featureName, err)
	}
	if feature.Class != artifact.ClassFeature {
		return false, fmt.Sprintf("implements-target %q is a %s spec, not a feature spec", featureName, feature.Class)
	}

	implSet := sortedSet(acIDs)
	var matchedStub *artifact.Stub
	for i := range feature.Stubs {
		if equalSortedSets(implSet, sortedSet(feature.Stubs[i].AcceptanceCriteria)) {
			matchedStub = &feature.Stubs[i]
			break
		}
	}
	if matchedStub == nil {
		return false, "implements-set does not equal any of the feature's declared stub AC sets"
	}

	if store.RefSlug(story.Title) != matchedStub.Slug {
		return false, fmt.Sprintf("RefSlug(title) %q does not equal the matched stub's slug %q", store.RefSlug(story.Title), matchedStub.Slug)
	}

	if disq, why := disqualifyingSupersedesOrExempts(root, story); disq {
		return false, why
	}

	dispositioned, why := judgedFindingsClear(root, story)
	if !dispositioned {
		return false, why
	}

	return true, ""
}

// storyImplementsTarget gathers the single feature spec name every
// implements edge on story targets, plus the union of AC ids those edges
// name. An error is returned only when implements edges name more than one
// distinct feature — everything else (no edges at all) is reported via the
// zero-value featureName, left for the caller to read as "not matched".
func storyImplementsTarget(story *artifact.SpecFrontmatter) (featureName string, acIDs []string, err error) {
	for _, l := range story.Links {
		if l.Type != artifact.LinkImplements {
			continue
		}
		ref, perr := artifact.ParseRef(l.Ref)
		if perr != nil || !ref.Fragment() {
			continue // already lint-checked elsewhere; ignore malformed edges here
		}
		if featureName == "" {
			featureName = ref.Name
		} else if featureName != ref.Name {
			return "", nil, fmt.Errorf("implements edges span more than one feature (%s, %s)", featureName, ref.Name)
		}
		acIDs = append(acIDs, ref.Object)
	}
	return featureName, acIDs, nil
}

// disqualifyingSupersedesOrExempts reports whether story carries a
// supersedes/exempts edge that disqualifies it from the stub-matched fast
// path, at the top level or on any of its decisions (03: "the story
// introduces no supersedes/exempts edges").
//
// W3 adjudication of a spec contradiction (03's rung-3 story-supersession
// chain vs R4-I-12's fourth conjunct; the spec text is being amended in
// parallel to match this rule): a `supersedes` edge whose target resolves to
// a spec of class STORY is the rung-3 chain edge to the story's OWN
// predecessor — story-spec v2 supersedes v1 (03 §The amendment ladder rung
// 3). That edge does NOT disqualify: it IS the fast path ("the stub-matched
// fast path applies when the feature mapping is unchanged"). Every `exempts`
// edge, and every `supersedes` edge targeting anything else — an ADR, a
// feature spec, a decision object — still disqualifies.
func disqualifyingSupersedesOrExempts(root string, story *artifact.SpecFrontmatter) (bool, string) {
	links := append([]artifact.Link(nil), story.Links...)
	for _, d := range story.Decisions {
		links = append(links, d.Links...)
	}
	for _, l := range links {
		switch l.Type {
		case artifact.LinkExempts:
			return true, fmt.Sprintf("story carries an exempts edge (%s), disqualifying the stub-matched fast path", l.Ref)
		case artifact.LinkSupersedes:
			if supersedesTargetsStory(root, l.Ref) {
				continue // rung-3 chain edge to the predecessor story — the fast path itself
			}
			return true, fmt.Sprintf("story carries a supersedes edge to a non-story target (%s); only the rung-3 chain edge to a predecessor story spec is exempt", l.Ref)
		}
	}
	return false, ""
}

// supersedesTargetsStory reports whether ref resolves to a spec of class
// story in either specs/active/ or specs/archive/ — the only supersedes
// target R4-I-12's chain-edge exception admits. A predecessor story is
// commonly archived (closed) by the time its successor is accepted, so both
// zones must be consulted, matching internal/align's decision-edge
// resolution (readSpecByName). Anything unresolvable (a malformed ref, a
// non-spec kind such as an ADR, a target not loadable in either zone, or a
// fragment ref into a feature spec) is NOT a story, so the edge disqualifies:
// fail closed toward full review, never toward the fast path.
func supersedesTargetsStory(root, ref string) bool {
	r, err := artifact.ParseRef(ref)
	if err != nil || r.Kind != artifact.KindSpec {
		return false
	}
	target, err := storyresolve.LoadSpec(root, r.Name)
	if err != nil || target == nil {
		return false
	}
	return target.Class == artifact.ClassStory
}

// judgedFindingsClear checks the design-branch decision-conflict report
// (see computeStubMatch's doc comment for the disclosed judgment call on
// where/whether this artifact exists at accept time).
func judgedFindingsClear(root string, story *artifact.SpecFrontmatter) (bool, string) {
	specRef, err := artifact.ParseRef(story.ID)
	if err != nil {
		return true, "" // unreachable: story.ID already decoded successfully
	}
	path := filepath.Join(root, ".verdi", "specs", "active", specRef.Name, "decision-conflict-report.md")
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, ""
		}
		return false, fmt.Sprintf("reading decision-conflict-report.md: %v", err)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return false, fmt.Sprintf("decision-conflict-report.md: %v", err)
	}
	decoded, err := artifact.DecodeDecisionConflict(fm)
	if err != nil {
		return false, fmt.Sprintf("decision-conflict-report.md failed to decode: %v", err)
	}
	var undispositioned []string
	for _, f := range decoded.Findings {
		if !f.Dispositioned() {
			undispositioned = append(undispositioned, f.ID)
		}
	}
	if len(undispositioned) > 0 {
		sort.Strings(undispositioned)
		return false, fmt.Sprintf("undispositioned judged finding(s): %v", undispositioned)
	}
	return true, ""
}

func sortedSet(ids []string) []string {
	out := append([]string(nil), ids...)
	sort.Strings(out)
	// dedup
	uniq := out[:0]
	var last string
	for i, id := range out {
		if i == 0 || id != last {
			uniq = append(uniq, id)
		}
		last = id
	}
	return uniq
}

func equalSortedSets(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
