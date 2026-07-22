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
	"regexp"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
)

// draftStatusLineRe matches the scaffold's own `status: draft` frontmatter
// line (design.go's scaffold functions always write exactly this form),
// tolerating an optional surrounding quote so a human's re-quoting edit
// during the design branch does not break the flip.
var draftStatusLineRe = regexp.MustCompile(`(?m)^status:\s*"?draft"?\s*$`)

// cmdAccept is `verdi accept`'s entry point, invoked by dispatch.go.
func cmdAccept(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "accept: usage: verdi accept <spec-ref|diagram-ref> (e.g. spec/stale-decline, diagram/loansvc-target-topology)")
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
	if err != nil || ref.Pinned() || (ref.Kind != artifact.KindSpec && ref.Kind != artifact.KindDiagram) {
		fmt.Fprintf(stderr, "accept: %q is not a spec or diagram ref (want spec/<name> or diagram/<name>, e.g. spec/stale-decline)\n", specArg)
		return 2
	}

	// spec/proposal-artifact ac-3/dc-2: a diagram/... ref dispatches to the
	// new, narrower ritual entirely — no stub-match, no CODEOWNERS routing,
	// no supersedes cascade, since a diagram carries no ACs or stubs to
	// match against. The spec ritual below is unchanged for spec/... refs.
	if ref.Kind == artifact.KindDiagram {
		return runAcceptDiagram(ctx, root, ref, stdout, stderr)
	}

	// The resolved operating model (store.Open's config bottleneck, L-M3):
	// spec/vocabulary-surfaces ac-1 — the state words accept's verdict
	// lines print resolve through Config.Model's display vocabulary. A
	// store whose verdi.yaml/model.yaml cannot be resolved is operational
	// (exit 2), matching every other manifest-loading verb's posture.
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	mdl := cfg.Model

	specPath := store.ActiveSpecPath(root, ref.Name)
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
		// Display resolution (L-M13(1)): the three class words are display
		// prose, articles agreeing via model.Indefinite
		// (judged-cli-refusal-prose-class-state-words-still-bare). The
		// "(no story, no acceptance criteria)" parenthetical names the
		// story:/acceptance_criteria: FRONTMATTER FIELDS such a spec lacks
		// — identity, kept bare.
		classWord := mdl.DisplayClass(string(spec.Class))
		featureWord := mdl.DisplayClass("feature")
		fmt.Fprintf(stderr, "accept: %s is %s spec (no story, no acceptance criteria); only %s or %s spec can be accepted\n", ref.String(),
			model.Indefinite(classWord),
			model.Indefinite(featureWord), mdl.DisplayClass("story"))
		return 1
	}
	if spec.Status != "draft" {
		// Display resolution (L-M13(1)), mirroring the class-refusal branch
		// above and buildstart.go's superseded-word refusal: the trailing
		// draft state word is display prose too, its article agreeing via
		// model.Indefinite. It was the one bare state word left hard-coded
		// ("only a draft spec") beside two already-resolved ones in this one
		// sentence (judged-ac4-draft-prose-leak).
		draftWord := mdl.DisplayState(string(spec.Class), "draft")
		fmt.Fprintf(stderr, "accept: %s status is %q, not %s; only %s spec can be accepted\n", ref.String(),
			mdl.DisplayState(string(spec.Class), string(spec.Status)),
			draftWord,
			model.Indefinite(draftWord))
		return 1
	}

	if !draftStatusLineRe.Match(fm) {
		// vocab:identity — frontmatter status-line machinery (field + enum value)
		fmt.Fprintf(stderr, "accept: %s: internal error: decoded status is draft, but no status: draft frontmatter line was found to flip\n", specPath)
		return 2
	}
	if n := len(draftStatusLineRe.FindAllIndex(fm, -1)); n != 1 {
		// vocab:identity — frontmatter status-line machinery (field + enum value)
		fmt.Fprintf(stderr, "accept: %s: internal error: expected exactly one status: draft line, found %d\n", specPath, n)
		return 2
	}

	// spec/obligation-seam ac-1 (O-1/O-4): compute preFlipHead FIRST —
	// before scaffolding and before the in-ritual lint gate — so every
	// backstop-scaffolded obligation, and the spec's own flip moments
	// later, stamp the identical commit: never the not-yet-created accept
	// commit itself. Reordered deliberately from this function's
	// pre-obligation-seam shape, which computed this after the lint gate.
	preFlipHead, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	at, err := gitx.CommitDateOnly(ctx, root, preFlipHead)
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	// spec/obligation-seam ac-1/ac-2 (O-1/O-2/O-3/O-3b/O-6,
	// acceptobligation.go): scaffold exactly the missing declared (ac,
	// kind) obligation pairs to disk before the lint gate runs — a no-op
	// for a feature-class spec (dc-3).
	obligationDir := store.ObligationDir(root, ref.Name)
	_, dirStatErr := os.Stat(obligationDir)
	obligationDirPreExisted := dirStatErr == nil

	createdObligations, err := scaffoldMissingObligations(root, ref.Name, spec, artifact.NewFrozen(at, preFlipHead), operatorOwner())
	// spec/obligation-seam ac-3 (O-1b): from here to the end of this
	// function, ANY refusal or operational error must unlink exactly the
	// obligation stubs newly created above — pristine tree on refusal,
	// pre-existing obligations and the rest of the tree untouched.
	// success flips true only immediately before the final, successful
	// return.
	success := false
	defer func() {
		if !success {
			unlinkScaffoldedObligations(createdObligations, obligationDir, obligationDirPreExisted, stderr)
		}
	}()
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	// D6-23: refuse to freeze a quartet the store's own linter rejects,
	// before any part of the ritual below runs — no stub-match/blast-radius
	// disclosure printed, no status flip, no frozen stamp (acceptlint.go).
	if rc := lintQuartetOrRefuse(ctx, root, ref, spec, stderr); rc != 0 {
		return rc
	}

	stubMatched := false
	if spec.Class == artifact.ClassStory {
		var reason string
		stubMatched, reason = computeStubMatch(root, spec, mdl)
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
			// Display resolution (L-M13(1)): the class word, the closed
			// state word, and the stor(y/ies) alternation
			// (displayAlternation) all resolve; the affected REFS and the
			// quorum enum value stay identity.
			fmt.Fprintf(stdout, "accept: %s: rung-4 %s supersession of %s — %d affected in-flight/%s %s %v -> computed quorum: %s (disclosed fact; approval-count enforcement stays forge/CODEOWNERS configuration, never verdi behavior, 03 §The amendment ladder)\n",
				ref.String(), mdl.DisplayClass("feature"), radius.PredecessorRef, len(radius.Affected),
				mdl.DisplayState("story", "closed"),
				displayAlternation(mdl.DisplayClass("story"), mdl.DisplayClassPlural("story")),
				affectedRefs, radius.Quorum)
		}
	}

	frozenLine := fmt.Sprintf("frozen: { at: %s, commit: %s", at, preFlipHead)
	if stubMatched {
		frozenLine += ", stub_matched: true"
	}
	frozenLine += " }"

	// vocab:identity — frontmatter status-line machinery (field + enum value)
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

	// Round-5 amendment (D-12), extended by round 6's ac-1
	// (feature-supersession-state): accepting a spec that carries a
	// `supersedes` edge to a predecessor STORY spec, or a WHOLE-SPEC
	// `supersedes` edge to a predecessor FEATURE spec, also flips that
	// predecessor's status to `superseded` in the same ritual — the sole
	// legal writer of VL-004's accepted-pending-build→superseded transition,
	// a status-only edit VL-010 admits on an otherwise-frozen spec. The
	// predecessor keeps its frozen stamp and stays in specs/active/. Written
	// to disk here so the caller's own scoped AddPaths/CreateCommit below
	// lands it in the same commit as the accept flip.
	predecessorPaths, rc := supersedePredecessors(root, spec, mdl, stdout, stderr)
	if rc != 0 {
		return rc
	}

	// D6-33: stage exactly the paths this ritual modified — specPath's own
	// flip plus any predecessor(s) supersedePredecessors just flipped above
	// — never the rest of the working tree. gitx.AddAll's `git add -A` twice
	// swept an unrelated untracked scratch build binary into an acceptance
	// commit (round-6 witness, both acceptance agents hit it independently
	// in the same wave); a ritual that writes a frozen stamp must not also
	// silently commit whatever else happens to be sitting in the checkout.
	// spec/obligation-seam ac-1 (O-2): scaffolded obligation paths join
	// accept's scoped addPaths set so they land inside the same accept
	// commit as the status flip — the mechanism that makes "the gap cannot
	// be replayed away" literally true rather than aspirational.
	addPaths := append([]string{specPath}, predecessorPaths...)
	addPaths = append(addPaths, createdObligations...)
	if err := gitx.AddPaths(ctx, root, addPaths...); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	// vocab:identity — git commit subject (history, never display prose)
	if _, err := gitx.CreateCommit(ctx, root, fmt.Sprintf("accept: %s draft -> accepted-pending-build", ref.String())); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	success = true

	// Display resolution only (spec/vocabulary-surfaces ac-1): the commit
	// subject above stays on bare ids (history is identity, never display).
	fmt.Fprintf(stdout, "accept: %s status: %s -> %s\n", ref.String(),
		mdl.DisplayState(string(spec.Class), "draft"),
		mdl.DisplayState(string(spec.Class), "accepted-pending-build"))
	fmt.Fprintf(stdout, "accept: frozen: { at: %s, commit: %s, stub_matched: %t }\n", at, preFlipHead, stubMatched)
	return 0
}
