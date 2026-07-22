// Accept's freeze-moment obligation backstop (spec/obligation-seam ac-1/
// ac-2/ac-3, spec/creation-surfaces#ac-4, ledger L-N8 as adjudicated at
// Task 8 — design doc §12 rules O-1/O-1b/O-2/O-3/O-3b/O-4/O-6): before
// accept's in-ritual lint gate ever runs, scaffold exactly the (ac, kind)
// pairs a story spec declares and has no DECODABLE obligation for yet,
// stamped identically to the spec's own upcoming flip stamp, so a story is
// born with its declared evidence kinds' obligations already in hand. The
// backstop skips — never overwrites — any pair a decodable obligation
// already covers (O-3/O-3b), and accept.go unlinks exactly what this
// invocation newly created on any later refusal or error (O-1b, via
// unlinkScaffoldedObligations below). Feature specs never carry
// obligations (03 §The feature fold; dc-3) — scaffoldMissingObligations is
// a no-op for anything but a story-class spec, so accept.go can call it
// unconditionally for either class.
//
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go/attest.go
// convention, so accept.go's own diff for wiring this in stays small.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/store"
)

// fallbackOperatorOwner is the disclosed sentinel operatorOwner falls back
// to when $USER is unset — honest, greppable, and never mistaken for a
// real username (O-6: "owners = the accepting operator ($USER, fallback
// sentinel)"). Mirrors internal/workbench/boardspecapi.go's own
// annotationAuthor()'s "board" fallback, scoped to accept's own domain
// instead of the board's.
const fallbackOperatorOwner = "unassigned-operator"

// operatorOwner names the accepting operator for a backstop-scaffolded
// obligation's owners: field (O-6). The OS user is honest attribution for
// who ran `verdi accept`; fallbackOperatorOwner covers a bare/CI
// environment with no USER set.
func operatorOwner() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return fallbackOperatorOwner
}

// obligationBackstopDisclosureLine is O-6's required disclosure, verbatim:
// every backstop-scaffolded obligation's body carries this line so it is
// frozen honestly as disclosed-as-unproven, never disguised as elaborated
// intent.
const obligationBackstopDisclosureLine = "This obligation was scaffolded at accept; not elaborated."

// backstopObligationBody renders a scaffolded obligation's body: the
// required disclosure line (O-6), plain-language pointers at what to do
// next, and the acceptance criterion's own already-declared text — never a
// fabricated claim about what the evidence specifically shows, since the
// whole point of the disclosure is that nobody has said that yet.
func backstopObligationBody(specRef, acID string, kind artifact.EvidenceKind, acText string) string {
	return fmt.Sprintf(
		"%s It is a placeholder for %s's %s evidence, written by accept's\n"+
			"freeze-moment backstop because no obligation existed for this pair\n"+
			"when %s was accepted (spec/creation-surfaces#ac-4). Replace this body\n"+
			"with a first-person statement of what that evidence must specifically\n"+
			"show before relying on it — by hand, or via `verdi obligation author\n"+
			"%s %s %s` on a design branch before the replacement itself freezes.\n"+
			"The acceptance criterion's own declared text, for reference:\n\n%s\n",
		obligationBackstopDisclosureLine, acID, kind, specRef, specRef, acID, kind, acText)
}

// scaffoldMissingObligations is the backstop's own core (O-1/O-2/O-3/O-3b/
// O-4/O-6): for a story-class spec, it scaffolds a stub obligation for
// every declared (ac, kind) pair with no decodable obligation yet at the
// convention path (internal/evidence.Obligations — the identical
// decode-based predicate VL-020 itself uses, O-3b), stamping every stub
// frozen with the given frozen value (the caller passes preFlipHead's own
// stamp, O-4) and owner (the caller passes operatorOwner(), O-6). It never
// overwrites: a pair whose obligation already decodes is skipped outright
// (O-3). created lists exactly the paths newly written this call, in
// declaration order, for the caller to stage (O-2) and, on any later
// failure, unlink (O-1b) — created is returned even when err != nil, so a
// failure partway through scaffolding still reports what was written so
// far.
//
// A pre-existing file at the convention path that FAILS to decode (a
// present-but-malformed obligation — a real, if rare, tree state
// evidence.Obligations itself refuses to silently paper over) is treated
// as an operational error, not as "missing" nor as "covered": accept
// refuses rather than guessing whether to clobber it or pretend it counts
// (spec/obligation-seam ac-2's disclosed reading — a WRITING path's
// posture is deliberately more conservative here than VL-020's own
// read-only classify-and-report one).
//
// spec is the caller's already-decoded, PRE-flip spec (still carrying
// status: draft on disk) — its own AcceptanceCriteria/Class fields are all
// this needs; it is never mutated.
func scaffoldMissingObligations(root, specName string, spec *artifact.SpecFrontmatter, frozen artifact.Frozen, owner string) (created []string, err error) {
	if spec.Class != artifact.ClassStory {
		return nil, nil // dc-3: feature (and component) ACs never carry obligations
	}
	specRef := "spec/" + specName

	for _, ac := range spec.AcceptanceCriteria {
		existing, oerr := evidence.Obligations(root, specName, ac.ID)
		if oerr != nil {
			return created, fmt.Errorf("checking existing obligations for %s: %w", ac.ID, oerr)
		}
		for _, kind := range ac.Evidence {
			if _, covered := existing[kind]; covered {
				continue // O-3/O-3b: a decodable obligation already covers this pair
			}

			path := store.ObligationPath(root, specName, ac.ID, string(kind))

			// judged-coverage-predicate-forkind-keying: the coverage predicate
			// above keys on each obligation's DECODED for_kind, but a decodable
			// obligation can sit at a DIFFERENT kind's convention filename (its
			// for_kind disagreeing with the name — internally consistent enough
			// to decode, path/id agreement being VL-011's job). Such a file
			// never errored above and never counted THIS pair as covered, yet
			// it already occupies this pair's convention path. WriteObligationFile
			// is unconditional by design (O-5: the seam stays policy-free; the
			// board's create-only check and this backstop each own their own
			// overwrite policy at the call site), so the backstop must itself
			// refuse to write over any occupied path: if anything at all already
			// sits here, this is a conflicted state (a present file whose own
			// for_kind disagrees with its filename) for the operator or VL-011 to
			// reconcile — refuse operationally rather than clobber a hand-authored
			// file. A correctly-named, decodable file was already skipped as
			// covered above and never reaches here; a present-but-undecodable
			// file already surfaced as an error out of evidence.Obligations.
			if _, statErr := os.Stat(path); statErr == nil {
				return created, fmt.Errorf("obligation already present at %s but not recognized as covering %s %s evidence — the file's own for_kind disagrees with its filename, a conflicted state to reconcile by hand or via VL-011; refusing to overwrite it", path, ac.ID, kind)
			} else if !os.IsNotExist(statErr) {
				return created, fmt.Errorf("checking obligation path %s: %w", path, statErr)
			}

			id := fmt.Sprintf("obligation/%s--%s--%s", specName, ac.ID, kind)
			title := fmt.Sprintf("scaffolded obligation: %s %s evidence", ac.ID, kind)
			content := evidence.RenderObligation(evidence.ObligationInput{
				ID:          id,
				Title:       title,
				ForKind:     kind,
				VerifiesRef: specRef,
				Body:        backstopObligationBody(specRef, ac.ID, kind, ac.Text),
				Owners:      []string{owner},
				Frozen:      frozen,
			})
			if werr := evidence.WriteObligationFile(path, content); werr != nil {
				return created, fmt.Errorf("scaffolding obligation for %s %s: %w", ac.ID, kind, werr)
			}
			created = append(created, path)
		}
	}
	return created, nil
}

// unlinkScaffoldedObligations is O-1b's cleanup: given exactly the paths
// scaffoldMissingObligations newly created this invocation (never a path
// merely skipped as already-covered — those are never in this slice),
// remove them, and then remove any directory this invocation newly created
// and left empty: first the per-spec obligations directory (when it did not
// pre-exist), then the .verdi/obligations/ PARENT (when IT did not pre-exist
// either — atomicfile's MkdirAll may have created both,
// judged-obligations-parent-dir-residue). Each os.Remove is best-effort and a
// no-op via the ignored ENOTEMPTY when other, pre-existing files still live
// there; the per-spec removal necessarily precedes the parent's, so a parent
// whose sole child was that per-spec dir becomes removable. A removal failure
// is disclosed to stderr rather than silently swallowed, but never changes the
// caller's own exit code: the caller already has the real refusal or error to
// report.
func unlinkScaffoldedObligations(created []string, obligationDir string, obligationDirPreExisted, obligationsParentPreExisted bool, stderr io.Writer) {
	for _, p := range created {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(stderr, "accept: warning: cleaning up scaffolded obligation %s after refusal: %v\n", p, err)
		}
	}
	if !obligationDirPreExisted {
		_ = os.Remove(obligationDir) // best-effort; only removes it if now empty
	}
	if !obligationsParentPreExisted {
		_ = os.Remove(filepath.Dir(obligationDir)) // .verdi/obligations, only if newly created and now empty
	}
}
