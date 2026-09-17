package supersede

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// Reason is a Resolve refusal's classification — a distinguishable,
// typed value (never only a prose-matched string) so a caller (the CLI
// today; the board's Revise action, W3-C, later) can branch on WHY a
// predecessor was refused without parsing Error() text.
type Reason string

const (
	// ReasonNotFound means predName has no spec.md in the current
	// checkout's active zone at all.
	ReasonNotFound Reason = "not-found"
	// ReasonNotDecodable means predName's spec.md exists but its
	// frontmatter does not split or strict-decode.
	ReasonNotDecodable Reason = "not-decodable"
	// ReasonWrongClass means predName decodes but is not class: feature —
	// supersession is feature-only (02 §Kind registry: story and
	// component classes refuse it).
	ReasonWrongClass Reason = "wrong-class"
	// ReasonWrongStatus means predName is class: feature but its
	// Git-derived EFFECTIVE status (internal/specstate) is not
	// accepted-pending-build — "implementations build accepted specs
	// only" (internal/stubinstantiate.SealedFeatureWallGuard's identical
	// owner's rule, reused here as prose, not as a call: that guard's
	// action-name wording is stub-instantiate-specific).
	ReasonWrongStatus Reason = "wrong-status"
)

// ResolveError is Resolve's typed refusal: Reason classifies why,
// PredecessorName names what was rejected, and Error() carries the full
// human-readable detail a CLI or API caller prints or relays as-is.
type ResolveError struct {
	Reason          Reason
	PredecessorName string
	Detail          string
}

func (e *ResolveError) Error() string { return e.Detail }

// Predecessor is what a successful Resolve proved: predName's own bare
// name, its exact current bytes (as read from the current checkout's
// active zone), its decoded frontmatter, and its resolved EFFECTIVE status
// (always "accepted-pending-build" whenever Resolve returns a nil error —
// carried on the struct anyway so a caller never has to re-derive or
// hardcode the string literal).
type Predecessor struct {
	Name   string
	Raw    []byte
	Spec   *artifact.SpecFrontmatter
	Status string
}

// Resolve reads predName's spec.md out of the CURRENT checkout's active
// zone (store.ActiveSpecPath — the same convention cmd/verdi/
// designfromstub.go's runDesignStartFromStub already reads a stub's own
// feature from), decodes it, and proves its Git-derived EFFECTIVE status
// through the shared specstate projector — exactly as that same function
// does (specstate.NewProjector().Resolve against a Candidate built from
// these exact bytes, never spec.Status read bare off the decode, since a
// locally-edited-but-unmerged claim of acceptance must not pass). It
// refuses with a typed *ResolveError unless the class is feature and the
// resolved status is accepted-pending-build (02 §Kind registry:
// supersession is feature-only; the "owner's rule" every other
// scaffold-a-story action already enforces via SealedFeatureWallGuard,
// applied here to Resolve's own distinct precondition — an ACCEPTED
// FEATURE is being superseded, not built from).
//
// This is meant to be called BEFORE any checkout switch (the "preparation
// boundary" pattern runDesignStart itself already follows: resolve/validate
// everything before the first Git mutation) — root is whatever checkout
// the caller currently has, and the read is of THAT checkout's working
// tree, not any particular branch by name.
//
// mdl resolves the class/status words BOTH refusal messages below speak
// through the store's own display vocabulary (spec/vocabulary-surfaces;
// mirrors internal/stubinstantiate.SealedFeatureWallGuard's identical
// class/status refusal messages byte-for-byte in shape: model.Indefinite
// wrapping model.DisplayClass/DisplayState, never a bare class or
// lifecycle-state word) — never display prose unclassified (ledger
// L-M13a(6), internal/specalign's TestVocabProseWitness). Nil-safe
// exactly like every DisplayClass/DisplayState call (falls back to the
// bare id when no model was resolved).
func Resolve(ctx context.Context, root, predName string, mdl *model.Model) (Predecessor, error) {
	specPath := store.ActiveSpecPath(root, predName)
	raw, err := os.ReadFile(specPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Predecessor{}, &ResolveError{
				Reason:          ReasonNotFound,
				PredecessorName: predName,
				Detail:          fmt.Sprintf("supersede: predecessor spec/%s not found at %s", predName, specPath),
			}
		}
		return Predecessor{}, fmt.Errorf("supersede: reading predecessor spec/%s: %w", predName, err)
	}

	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return Predecessor{}, &ResolveError{
			Reason:          ReasonNotDecodable,
			PredecessorName: predName,
			Detail:          fmt.Sprintf("supersede: predecessor spec/%s: %v", predName, err),
		}
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return Predecessor{}, &ResolveError{
			Reason:          ReasonNotDecodable,
			PredecessorName: predName,
			Detail:          fmt.Sprintf("supersede: predecessor spec/%s frontmatter does not strict-decode: %v", predName, err),
		}
	}

	if spec.Class != artifact.ClassFeature {
		return Predecessor{}, &ResolveError{
			Reason:          ReasonWrongClass,
			PredecessorName: predName,
			Detail: fmt.Sprintf("supersede: predecessor spec/%s: supersession is only available on %s (02 §Kind registry); its class is %s",
				predName, model.Indefinite(mdl.DisplayClass("feature")), mdl.DisplayClass(string(spec.Class))),
		}
	}

	relPath := store.ActiveSpecRelPath(predName)
	result, err := specstate.NewProjector().Resolve(ctx, root, specstate.Candidate{Path: relPath, Content: raw})
	if err != nil {
		return Predecessor{}, fmt.Errorf("supersede: resolving predecessor spec/%s's effective status: %w", predName, err)
	}
	status := string(result.ArtifactStatus())
	if status != "accepted-pending-build" {
		return Predecessor{}, &ResolveError{
			Reason:          ReasonWrongStatus,
			PredecessorName: predName,
			Detail: fmt.Sprintf("supersede: predecessor spec/%s: supersession is only available on %s (implementations build accepted specs only); its effective status is %s",
				predName, model.Indefinite(mdl.DisplayState("feature", "accepted-pending-build")), mdl.DisplayState("feature", status)),
		}
	}

	return Predecessor{Name: predName, Raw: raw, Spec: spec, Status: status}, nil
}
