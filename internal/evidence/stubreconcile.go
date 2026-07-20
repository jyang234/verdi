package evidence

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
)

// StubStory is one candidate implementing story for stub reconciliation:
// its closure status and the feature AC ids (within the feature under
// reconciliation) its `implements` edges name. It is a narrower view of
// the same discovery ImplementingStory carries for the feature fold —
// kept as its own type because reconciliation needs the story regardless
// of AC-by-AC grouping (a story can partially contribute to more than one
// stub) where the feature fold groups by AC.
type StubStory struct {
	SpecRef string
	ACIDs   []string
	Closed  bool
}

// StubWithdrawal is one caller-declared "this stub is not being built"
// record: `{withdrawn: note}` in 03 §Stub reconciliation's closure-MR
// reconciliation-block vocabulary. 03 describes the withdrawn state only
// as a fact the closure MR *carries*, never a committed artifact schema a
// human authors ahead of time — so this is a judgment call, disclosed in
// the phase report: the caller (eventually `verdi close <feature>`,
// V1-P4+, out of this phase's scope) is responsible for sourcing
// withdrawal declarations from wherever they end up living (a future
// annotation type, a close-time flag, or a closure-MR-authoring UI) and
// handing them to ReconcileStubs as plain data — this package only folds
// the already-declared set, the same "caller resolves, fold reduces"
// idiom the rest of this package uses throughout.
type StubWithdrawal struct {
	Slug string
	Note string
}

// StubReconcileInput is ReconcileStubs's input for one feature spec.
type StubReconcileInput struct {
	// Spec is the feature spec whose Stubs the check reconciles. Required.
	Spec *artifact.SpecFrontmatter
	// Stories are every candidate implementing story discovered for this
	// feature (via the same backlink-inversion mechanism FeatureInput.Stories
	// is built from) — not pre-grouped by AC, since one story can
	// contribute to more than one stub.
	Stories []StubStory
	// Withdrawals are the caller-declared withdrawn-with-note stubs (see
	// StubWithdrawal's doc comment on why this is caller-supplied data,
	// not a decoded artifact).
	Withdrawals []StubWithdrawal
	// Model is the store's resolved operating model, used ONLY to route
	// class display words in this check's own refusal prose — exactly
	// FeatureInput.Model's contract (see that field's doc comment): nil
	// renders bare ids, no reconciliation DECISION ever reads it.
	Model *model.Model
}

// StubBucket is one stub's reconciliation state (03 §Stub reconciliation).
type StubBucket string

const (
	// StubRealized: the stub's declared AC set is fully covered by the
	// union of one or more closed implementing stories.
	StubRealized StubBucket = "realized-by"
	// StubWithdrawnBucket: an explicit withdrawal was declared for this
	// stub, and it is not (yet, or ever) fully realized by closed stories
	// — see ReconcileStubs's doc comment for the precedence rule when
	// both a withdrawal and full coverage exist.
	StubWithdrawnBucket StubBucket = "withdrawn-with-note"
	// StubUnreconciled: neither realized nor withdrawn — blocks closure
	// (03 §Stub reconciliation: "A stub in neither state blocks closure").
	StubUnreconciled StubBucket = "unreconciled"
)

// StubResult is one stub's reconciliation outcome.
type StubResult struct {
	Slug       string
	Bucket     StubBucket
	RealizedBy []string // closed implementing stories' spec refs that (jointly) cover this stub's AC set; empty unless Bucket == StubRealized
	Note       string   // the withdrawal note; empty unless Bucket == StubWithdrawnBucket
}

// UnplannedAddition is a closed implementing story whose ACIDs overlap no
// stub's declared AC set at all (03 §Stub reconciliation: "a closed
// implementing story that traces to no stub ... is recorded in the
// reconciliation block as an unplanned addition").
type UnplannedAddition struct {
	SpecRef string
	ACIDs   []string
}

// StubReconciliation is ReconcileStubs's whole-feature result.
type StubReconciliation struct {
	Stubs     []StubResult
	Unplanned []UnplannedAddition
	// Blocked is true iff any stub is StubUnreconciled (03: "blocks
	// closure").
	Blocked bool
}

// ReconcileStubs implements 03 §Stub reconciliation's VL-014-shaped
// bidirectional completeness check: every acceptance-time stub is either
// realized-by named closed stories or explicitly withdrawn-with-note; a
// stub in neither state blocks closure; a closed implementing story
// tracing to no stub is recorded as an unplanned addition, never an error.
//
// realized-by's coverage rule (03 gives the property, not the algorithm):
// a stub is realized-by the set of CLOSED implementing stories whose
// ACIDs, restricted to the stub's own declared AC set, jointly cover that
// set exactly — "one or more named closed stories" (03), so coverage may
// span several stories. 03 additionally says a realizing story's
// "title/outcome trace to the stub" — a human-judged property no computed
// field captures (Finding/Stub carry no such link); this function computes
// the AC-coverage half only and does not attempt to compute title/outcome
// tracing, disclosed in the phase report as a deliberately narrowed,
// reversible reading (the coverage half is the part 03 calls a
// "VL-014-shaped bidirectional completeness check" — i.e. computed).
//
// Precedence when a stub is both fully covered and separately declared
// withdrawn (a caller/authoring inconsistency ReconcileStubs does not
// reject): full coverage by real closed stories wins — StubRealized —
// since the stub demonstrably WAS built; declaring it withdrawn in that
// case would be the dishonest reading.
func ReconcileStubs(in StubReconcileInput) (StubReconciliation, error) {
	if in.Spec == nil {
		return StubReconciliation{}, fmt.Errorf("evidence: ReconcileStubs: Spec is required")
	}
	if in.Spec.Class != artifact.ClassFeature {
		// The spoken class word is display and resolves (L-M13a(6) work
		// order); the class COMPARISON and the echoed %q id stay bare.
		return StubReconciliation{}, fmt.Errorf("evidence: ReconcileStubs: spec %q is class %q, not %s spec", in.Spec.ID, in.Spec.Class, model.Indefinite(in.Model.DisplayClass("feature")))
	}

	withdrawn := make(map[string]string, len(in.Withdrawals))
	for _, w := range in.Withdrawals {
		withdrawn[w.Slug] = w.Note
	}

	accounted := make(map[string]bool, len(in.Stories)) // SpecRef -> contributes to >=1 stub

	var out StubReconciliation
	for _, stub := range in.Spec.Stubs {
		wantACs := make(map[string]bool, len(stub.AcceptanceCriteria))
		for _, id := range stub.AcceptanceCriteria {
			wantACs[id] = true
		}

		var realizers []string
		covered := make(map[string]bool, len(wantACs))
		for _, s := range in.Stories {
			if !s.Closed {
				continue
			}
			contributes := false
			for _, ac := range s.ACIDs {
				if wantACs[ac] {
					covered[ac] = true
					contributes = true
				}
			}
			if contributes {
				realizers = append(realizers, s.SpecRef)
				accounted[s.SpecRef] = true
			}
		}

		res := StubResult{Slug: stub.Slug}
		switch {
		case len(covered) == len(wantACs) && len(wantACs) > 0:
			res.Bucket = StubRealized
			res.RealizedBy = realizers
		case withdrawn[stub.Slug] != "":
			res.Bucket = StubWithdrawnBucket
			res.Note = withdrawn[stub.Slug]
		default:
			res.Bucket = StubUnreconciled
			out.Blocked = true
		}
		out.Stubs = append(out.Stubs, res)
	}

	for _, s := range in.Stories {
		if !s.Closed {
			continue
		}
		if accounted[s.SpecRef] {
			continue
		}
		out.Unplanned = append(out.Unplanned, UnplannedAddition{SpecRef: s.SpecRef, ACIDs: s.ACIDs})
	}

	return out, nil
}
