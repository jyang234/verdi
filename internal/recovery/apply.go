package recovery

import (
	"context"
	"errors"
	"io"

	"github.com/jyang234/verdi/internal/store"
)

// ErrNotImplemented is Task 3's own placeholder for `verdi recover
// --apply` (R-RR3-9, docs/superpowers/plans/2026-09-21-readiness-
// recovery-wave-3.md Task 4): Task 4 (Tier 3, owner risk gate) implements
// the real apply protocol over branchcut.Unwind and reclaim.Apply; until
// it lands, every call refuses uniformly, so cmd/verdi's --apply grammar
// and exit-code mapping (R-RR3-1) can be built and tested independently
// of the executors. cmdRecover maps this error to exit 2, printing its
// text.
var ErrNotImplemented = errors.New("recovery: --apply lands in Task 4 of the wave-3 plan")

// Outcome is Task 4's own placeholder return shape (its Interfaces block:
// ChoiceID, Postconditions, After, Journey, JourneyErr). This stub never
// populates it; Apply's Task 4 implementation replaces this whole file
// ("Create: internal/recovery/apply.go (replaces Task 3's stub)").
type Outcome struct{}

// Apply carries Task 4's exact signature so cmd/verdi's recover.go can be
// written and tested against the real call shape now. Every call in this
// stub returns ErrNotImplemented; Task 4 replaces this file wholesale.
func Apply(ctx context.Context, cfg *store.Config, ref, choiceID string, stderr io.Writer) (Outcome, error) {
	return Outcome{}, ErrNotImplemented
}
