// verdi accept <spec-ref> (05 §CLI, R4-I-12): the design branch's final
// action — mechanically flips a draft spec's
// `status: draft -> accepted-pending-build` and writes the frozen stamp
// (`commit` = the content-final sha it supersedes, `at` = that commit's
// own committer date — never wall clock), then commits the flip. Merging
// the resulting spec MR to main *is* acceptance (03 §Lifecycle: two MRs).
// Round four widens accept from feature-only to both spec classes
// (feature and story share one lifecycle, 02 §Kind registry): a story
// spec's acceptance additionally computes R4-I-12's stub-match (below) and
// stamps `stub_matched: true` into the same frozen block when it holds.
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go
// convention.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/disclosure"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
	"github.com/OWNER/verdi/internal/storyresolve"
)

// draftStatusLineRe matches the scaffold's own `status: draft` frontmatter
// line (design.go's scaffold functions always write exactly this form),
// tolerating an optional surrounding quote so a human's re-quoting edit
// during the design branch does not break the flip.
var draftStatusLineRe = regexp.MustCompile(`(?m)^status:\s*"?draft"?\s*$`)

// acceptedStatusLineRe matches an `accepted-pending-build` status
// frontmatter line, tolerating an optional surrounding quote (mirroring
// draftStatusLineRe). It is the only status a predecessor spec can legally
// be flipped FROM when its successor is accepted (VL-004's sole
// accepted-pending-build→superseded transition, D-12).
var acceptedStatusLineRe = regexp.MustCompile(`(?m)^status:\s*"?accepted-pending-build"?\s*$`)

// cmdAccept is `verdi accept`'s entry point, invoked by dispatch.go.
func cmdAccept(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "accept: usage: verdi accept <spec-ref> (e.g. spec/stale-decline)")
		return 2
	}
	specArg := args[0]

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	return runAccept(ctx, root, specArg, stdout, stderr)
}

// runAccept is the testable core: given an already-resolved root, run the
// whole accept ritual and return the exit code (CLAUDE.md: 0 clean,
// 1 verdict — the spec fails an accept precondition — 2 operational).
func runAccept(ctx context.Context, root, specArg string, stdout, stderr io.Writer) int {
	ref, err := artifact.ParseRef(specArg)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() {
		fmt.Fprintf(stderr, "accept: %q is not a spec ref (want spec/<name>, e.g. spec/stale-decline)\n", specArg)
		return 2
	}

	specPath := filepath.Join(root, ".verdi", "specs", "active", ref.Name, "spec.md")
	raw, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(stderr, "accept: reading %s: %v\n", specPath, err)
		return 2
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: %v\n", specPath, err)
		return 2
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: %v\n", specPath, err)
		return 2
	}
	_ = body

	if spec.Class != artifact.ClassFeature && spec.Class != artifact.ClassStory {
		fmt.Fprintf(stderr, "accept: %s is a %s spec (no story, no acceptance criteria); only a feature or story spec can be accepted\n", ref.String(), spec.Class)
		return 1
	}
	if spec.Status != "draft" {
		fmt.Fprintf(stderr, "accept: %s status is %q, not draft; only a draft spec can be accepted\n", ref.String(), spec.Status)
		return 1
	}

	if !draftStatusLineRe.Match(fm) {
		fmt.Fprintf(stderr, "accept: %s: internal error: decoded status is draft, but no status: draft frontmatter line was found to flip\n", specPath)
		return 2
	}
	if n := len(draftStatusLineRe.FindAllIndex(fm, -1)); n != 1 {
		fmt.Fprintf(stderr, "accept: %s: internal error: expected exactly one status: draft line, found %d\n", specPath, n)
		return 2
	}

	preFlipHead, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	commitDate, err := gitx.CommitDate(ctx, root, preFlipHead)
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	if len(commitDate) < 10 {
		fmt.Fprintf(stderr, "accept: internal error: commit date %q too short to derive a YYYY-MM-DD frozen.at\n", commitDate)
		return 2
	}
	at := commitDate[:10]

	stubMatched := false
	if spec.Class == artifact.ClassStory {
		var reason string
		stubMatched, reason = computeStubMatch(root, spec)
		if stubMatched {
			fmt.Fprintf(stdout, "accept: %s: stub-matched (R4-I-12): eligible for single-approver acceptance (forge/CODEOWNERS configuration, never verdi-enforced)\n", ref.String())
		} else {
			fmt.Fprintf(stdout, "accept: %s: not stub-matched (%s): full review applies\n", ref.String(), reason)
		}
	}

	// Rung-4 blast-radius-priced quorum disclosure (03 §The amendment
	// ladder rung 4, blastradius.go): fires only when the feature being
	// accepted itself carries a supersession: block — i.e. this accept IS
	// a rung-4 supersession's acceptance MR, never an ordinary first
	// acceptance. verdi computes and discloses the label; it never
	// enforces an approval count (03: "the mechanics of counting
	// approvals stay repo/CODEOWNERS configuration either way").
	if spec.Class == artifact.ClassFeature && spec.Supersession != nil {
		radius, berr := computeBlastRadius(root, spec)
		if berr != nil {
			fmt.Fprintln(stderr, "accept:", berr)
			return 2
		}
		if radius.PredecessorRef != "" {
			affectedRefs := make([]string, len(radius.Affected))
			for i, a := range radius.Affected {
				affectedRefs[i] = a.SpecRef
			}
			fmt.Fprintf(stdout, "accept: %s: rung-4 feature supersession of %s — %d affected in-flight/closed stor(y/ies) %v -> computed quorum: %s (disclosed fact; approval-count enforcement stays forge/CODEOWNERS configuration, never verdi behavior, 03 §The amendment ladder)\n",
				ref.String(), radius.PredecessorRef, len(radius.Affected), affectedRefs, radius.Quorum)
		}
	}

	frozenLine := fmt.Sprintf("frozen: { at: %s, commit: %s", at, preFlipHead)
	if stubMatched {
		frozenLine += ", stub_matched: true"
	}
	frozenLine += " }"

	newFm := draftStatusLineRe.ReplaceAll(fm, []byte("status: accepted-pending-build"))
	newFm = append(newFm, []byte("\n"+frozenLine)...)

	// Self-validate the flipped content before writing anything to disk
	// (CLAUDE.md: "never fake success").
	flipped, err := artifact.DecodeSpec(newFm)
	if err != nil {
		fmt.Fprintln(stderr, "accept: internal error: flipped frontmatter failed self-validation:", err)
		return 2
	}
	if flipped.Status != "accepted-pending-build" || flipped.Frozen == nil || flipped.Frozen.Commit != preFlipHead {
		fmt.Fprintln(stderr, "accept: internal error: flipped frontmatter does not carry the expected status/frozen stamp")
		return 2
	}
	if flipped.Frozen.StubMatched != stubMatched {
		fmt.Fprintln(stderr, "accept: internal error: flipped frontmatter's stub_matched does not match the computed value")
		return 2
	}

	newContent := "---\n" + string(newFm) + "\n---\n" + string(body)
	if err := os.WriteFile(specPath, []byte(newContent), 0o644); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	// Round-5 amendment (D-12): accepting a spec that carries a `supersedes`
	// edge to a predecessor STORY spec also flips that predecessor's status
	// to `superseded` in the same ritual — the sole legal writer of VL-004's
	// accepted-pending-build→superseded transition, a status-only edit VL-010
	// admits on an otherwise-frozen spec. The predecessor keeps its frozen
	// stamp and stays in specs/active/. Written to disk here so the caller's
	// own AddAll/CreateCommit lands it in the same commit as the accept flip.
	if rc := supersedePredecessors(root, spec, stdout, stderr); rc != 0 {
		return rc
	}

	if err := gitx.AddAll(ctx, root); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	if _, err := gitx.CreateCommit(ctx, root, fmt.Sprintf("accept: %s draft -> accepted-pending-build", ref.String())); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	fmt.Fprintf(stdout, "accept: %s status: draft -> accepted-pending-build\n", ref.String())
	fmt.Fprintf(stdout, "accept: frozen: { at: %s, commit: %s, stub_matched: %t }\n", at, preFlipHead, stubMatched)
	return 0
}

// supersedePredecessors flips every active `accepted-pending-build`
// STORY spec that `spec` supersedes to `status: superseded` (D-12). Scope
// is deliberately the rung-3 story-to-story chain edge (03 §The amendment
// ladder rung 3) — the same target class disqualifyingSupersedesOrExempts
// already special-cases via supersedesTargetsStory — never the rung-4
// feature supersession, whose predecessor lifecycle is governed by the
// blast-radius/cascade machinery (blastradius.go, cascadecheck.go) this
// patch does not disturb (invention ledger: smallest reversible option).
//
// Each flip is a raw, status-line-only ReplaceAll so the written file
// differs from its frozen base by exactly that one line — VL-010's
// status-only-to-superseded exception is then cleanly satisfiable and the
// frozen stamp is preserved untouched. A predecessor not in specs/active/
// (archived/closed), already superseded (idempotent), or in any status
// other than accepted-pending-build is left alone (the last case disclosed,
// never forced). Returns 0 on success, 2 on an operational failure.
func supersedePredecessors(root string, spec *artifact.SpecFrontmatter, stdout, stderr io.Writer) int {
	for _, l := range spec.Links {
		if l.Type != artifact.LinkSupersedes || !supersedesTargetsStory(root, l.Ref) {
			continue
		}
		ref, err := artifact.ParseRef(l.Ref)
		if err != nil {
			continue // malformed edges are lint's concern, not accept's
		}
		predPath := filepath.Join(root, ".verdi", "specs", "active", ref.Name, "spec.md")
		raw, err := os.ReadFile(predPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue // not in active/ (archived/closed) — nothing to flip here
			}
			fmt.Fprintf(stderr, "accept: reading predecessor %s: %v\n", predPath, err)
			return 2
		}
		predFm, _, err := artifact.SplitFrontmatter(raw)
		if err != nil {
			fmt.Fprintf(stderr, "accept: %s: %v\n", predPath, err)
			return 2
		}
		predSpec, err := artifact.DecodeSpec(predFm)
		if err != nil {
			fmt.Fprintf(stderr, "accept: %s: %v\n", predPath, err)
			return 2
		}
		if predSpec.Status == "superseded" {
			continue // already superseded — idempotent
		}
		if predSpec.Status != "accepted-pending-build" {
			fmt.Fprintln(stdout, disclosure.Render(disclosure.New("accept:supersede-predecessor", l.Ref,
				fmt.Sprintf("predecessor status is %q, not accepted-pending-build; left unflipped (only accepted-pending-build->superseded is a legal ritual transition, VL-004)", predSpec.Status))))
			continue
		}
		if n := len(acceptedStatusLineRe.FindAll(raw, -1)); n != 1 {
			fmt.Fprintf(stderr, "accept: %s: expected exactly one status: accepted-pending-build line to flip, found %d\n", predPath, n)
			return 2
		}
		newRaw := acceptedStatusLineRe.ReplaceAll(raw, []byte("status: superseded"))
		// Self-validate the flipped predecessor before writing (CLAUDE.md:
		// "never fake success"): it must still decode and keep its frozen
		// stamp — a superseded story is a post-acceptance, frozen artifact.
		flippedFm, _, err := artifact.SplitFrontmatter(newRaw)
		if err != nil {
			fmt.Fprintf(stderr, "accept: internal error: flipped predecessor %s failed self-validation: %v\n", ref.String(), err)
			return 2
		}
		flipped, err := artifact.DecodeSpec(flippedFm)
		if err != nil {
			fmt.Fprintf(stderr, "accept: internal error: flipped predecessor %s failed self-validation: %v\n", ref.String(), err)
			return 2
		}
		if flipped.Status != "superseded" || flipped.Frozen == nil {
			fmt.Fprintf(stderr, "accept: internal error: flipped predecessor %s does not carry status: superseded with its frozen stamp\n", ref.String())
			return 2
		}
		if err := os.WriteFile(predPath, newRaw, 0o644); err != nil {
			fmt.Fprintln(stderr, "accept:", err)
			return 2
		}
		fmt.Fprintf(stdout, "accept: %s: superseded by %s (status: accepted-pending-build -> superseded; status-only edit, frozen stamp preserved, stays in specs/active/)\n", ref.String(), spec.ID)
	}
	return 0
}

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
		return false, "no implements edges (spike or malformed story)"
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
