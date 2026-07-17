// The predecessor-supersession flip flow (spec/file-topics ac-2; 03 §The
// amendment ladder, D-12, extended by round 6's ac-1
// feature-supersession-state): supersedePredecessors and its helpers,
// which flip an accepted predecessor spec's status to `superseded` as part
// of accepting its successor — moved verbatim out of accept.go, which had
// grown three subsystems into one 587-line file. This file owns exactly
// this topic: nothing else.
package main

import (
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// acceptedStatusLineRe matches an `accepted-pending-build` status
// frontmatter line, tolerating an optional surrounding quote (mirroring
// draftStatusLineRe). It is the only status a predecessor spec can legally
// be flipped FROM when its successor is accepted (VL-004's sole
// accepted-pending-build→superseded transition, D-12).
var acceptedStatusLineRe = regexp.MustCompile(`(?m)^status:\s*"?accepted-pending-build"?\s*$`)

// supersedePredecessors flips every active `accepted-pending-build`
// predecessor spec that `spec` supersedes to `status: superseded` (D-12,
// extended by round 6's ac-1, feature-supersession-state, to feature-class
// predecessors). Two distinct edge shapes feed it, both reduced through the
// one shared mechanism below (flipPredecessorToSuperseded):
//
//   - every `supersedes` link (spec may carry more than one) whose target
//     resolves to a STORY-class spec — the rung-3 story-to-story chain edge
//     (03 §The amendment ladder rung 3), identified by supersedesTargetsStory
//     exactly as D-12 shipped it, the same target class
//     disqualifyingSupersedesOrExempts already special-cases;
//   - spec's single WHOLE-SPEC `supersedes` link (blastradius.go's
//     wholeSpecSupersedesTarget — never an object-fragment #ac-N edge, a
//     decision-level override belonging to 03 §Decision-conflict gate's
//     rung-2 machinery), when it resolves to a FEATURE-class spec.
//
// Neither shape touches the rung-4 cascade/blast-radius machinery that
// governs a superseded FEATURE's downstream *stories* (blastradius.go,
// cascadecheck.go) — the flip is a statement about the predecessor's own
// terminal lifecycle, orthogonal to its stories' verdicts (invention
// ledger: smallest reversible option). Returns every predecessor spec.md
// path actually flipped (D6-33: accept.go's caller needs these to stage
// exactly what it modified, never the rest of the working tree) plus 0 on
// success (including every no-op case, which contributes no path), 2 on an
// operational failure.
func supersedePredecessors(root string, spec *artifact.SpecFrontmatter, mdl *model.Model, stdout, stderr io.Writer) ([]string, int) {
	var paths []string
	for _, l := range spec.Links {
		if l.Type != artifact.LinkSupersedes || !supersedesTargetsStory(root, l.Ref) {
			continue
		}
		path, rc := flipPredecessorToSuperseded(root, l.Ref, spec.ID, mdl, stdout, stderr)
		if rc != 0 {
			return paths, rc
		}
		if path != "" {
			paths = append(paths, path)
		}
	}

	// ac-1 (feature-supersession-state): the feature-rung mirror of the
	// story loop above, scoped to spec's single WHOLE-SPEC supersedes edge
	// resolving to a FEATURE-class predecessor. supersedesTargetsFeature
	// fails closed (false) on a fragment ref, so an object-fragment
	// `supersedes` edge (a decision-level override) never reaches here.
	if wholeRef := wholeSpecSupersedesTarget(spec); wholeRef != "" && supersedesTargetsFeature(root, wholeRef) {
		path, rc := flipPredecessorToSuperseded(root, wholeRef, spec.ID, mdl, stdout, stderr)
		if rc != 0 {
			return paths, rc
		}
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths, 0
}

// flipPredecessorToSuperseded performs the single-predecessor half of the
// supersession ritual for predecessorRef — the one shared mechanism behind
// both edge shapes supersedePredecessors reduces (the rung-3 story chain
// edge and ac-1's feature chain edge alike): read the predecessor at
// specs/active/<name>/spec.md and, if (and only if) its status is
// accepted-pending-build, flip that single status line to `superseded`,
// self-validate the flipped bytes still decode with the frozen stamp
// intact, and write it back.
//
// Each flip is a raw, status-line-only ReplaceAll so the written file
// differs from its frozen base by exactly that one line — VL-010's
// status-only-to-superseded exception is then cleanly satisfiable and the
// frozen stamp is preserved untouched. A predecessor not in specs/active/
// (archived/closed), already superseded (idempotent), or in any status
// other than accepted-pending-build — including `closed`: dc-2's
// deliberately deferred closed->superseded case, invention ledger — is left
// alone (the last case disclosed, never forced). Returns the predecessor's
// spec.md path when (and only when) it actually wrote a flip — D6-33: the
// caller (supersedePredecessors) collects these so accept.go's own scoped
// AddPaths call stages exactly what this ritual modified, nothing else —
// and "" for every no-op case (malformed ref, absent, idempotent, or wrong
// status). rc is 0 on success (including every no-op case), 2 on an
// operational failure.
func flipPredecessorToSuperseded(root, predecessorRef, successorID string, mdl *model.Model, stdout, stderr io.Writer) (path string, rc int) {
	ref, err := artifact.ParseRef(predecessorRef)
	if err != nil {
		return "", 0 // malformed edges are lint's concern, not accept's
	}
	predPath := store.ActiveSpecPath(root, ref.Name)
	raw, err := os.ReadFile(predPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0 // not in active/ (archived/closed) — nothing to flip here
		}
		fmt.Fprintf(stderr, "accept: reading predecessor %s: %v\n", predPath, err)
		return "", 2
	}
	predFm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: %v\n", predPath, err)
		return "", 2
	}
	predSpec, err := artifact.DecodeSpec(predFm)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: %v\n", predPath, err)
		return "", 2
	}
	if predSpec.Status == "superseded" {
		return "", 0 // already superseded — idempotent
	}
	if predSpec.Status != "accepted-pending-build" {
		// Display resolution only (spec/vocabulary-surfaces ac-1): the
		// state WORDS this disclosure prints resolve through the model;
		// the status COMPARISON above, the frontmatter regex below, and
		// the commit history all stay on bare ids — a rename is cosmetic.
		predClass := string(predSpec.Class)
		fmt.Fprintln(stdout, disclosure.Render(disclosure.New("accept:supersede-predecessor", predecessorRef,
			fmt.Sprintf("predecessor status is %q, not %s; left unflipped (only %s->%s is a legal ritual transition, VL-004)",
				mdl.DisplayState(predClass, string(predSpec.Status)),
				mdl.DisplayState(predClass, "accepted-pending-build"),
				mdl.DisplayState(predClass, "accepted-pending-build"),
				mdl.DisplayState(predClass, "superseded")))))
		return "", 0
	}
	if n := len(acceptedStatusLineRe.FindAll(raw, -1)); n != 1 {
		fmt.Fprintf(stderr, "accept: %s: expected exactly one status: accepted-pending-build line to flip, found %d\n", predPath, n)
		return "", 2
	}
	newRaw := acceptedStatusLineRe.ReplaceAll(raw, []byte("status: superseded"))
	// Self-validate the flipped predecessor before writing (CLAUDE.md:
	// "never fake success"): it must still decode and keep its frozen
	// stamp — a superseded spec is a post-acceptance, frozen artifact.
	flippedFm, _, err := artifact.SplitFrontmatter(newRaw)
	if err != nil {
		fmt.Fprintf(stderr, "accept: internal error: flipped predecessor %s failed self-validation: %v\n", ref.String(), err)
		return "", 2
	}
	flipped, err := artifact.DecodeSpec(flippedFm)
	if err != nil {
		fmt.Fprintf(stderr, "accept: internal error: flipped predecessor %s failed self-validation: %v\n", ref.String(), err)
		return "", 2
	}
	if flipped.Status != "superseded" || flipped.Frozen == nil {
		fmt.Fprintf(stderr, "accept: internal error: flipped predecessor %s does not carry status: superseded with its frozen stamp\n", ref.String())
		return "", 2
	}
	if err := os.WriteFile(predPath, newRaw, 0o644); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return "", 2
	}
	fmt.Fprintf(stdout, "accept: %s: superseded by %s (status: %s -> %s; status-only edit, frozen stamp preserved, stays in specs/active/)\n",
		ref.String(), successorID,
		mdl.DisplayState(string(predSpec.Class), "accepted-pending-build"),
		mdl.DisplayState(string(predSpec.Class), "superseded"))
	return predPath, 0
}

// supersedesTargetsFeature reports whether ref is a WHOLE-SPEC supersedes
// edge — no object fragment, the same shape wholeSpecSupersedesTarget
// (blastradius.go) already identifies for the rung-4 cascade — that
// resolves to a spec of class feature in either specs/active/ or
// specs/archive/ (mirroring supersedesTargetsStory's own active/archive
// reach). An object-fragment supersedes edge (e.g. a decision's #ac-N
// override, 03 §Decision-conflict gate's rung-2 machinery) is never a
// feature-predecessor chain edge, so — like a malformed or unresolvable
// ref — it fails closed here (false) rather than triggering a flip: fail
// closed toward no-flip, never toward one (mirroring
// supersedesTargetsStory's own fail-closed posture).
func supersedesTargetsFeature(root, ref string) bool {
	r, err := artifact.ParseRef(ref)
	if err != nil || r.Kind != artifact.KindSpec || r.Fragment() {
		return false
	}
	target, err := storyresolve.LoadSpec(root, r.Name)
	if err != nil || target == nil {
		return false
	}
	return target.Class == artifact.ClassFeature
}
