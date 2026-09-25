package contextcompile

import (
	"fmt"

	"github.com/jyang234/verdi/internal/repositoryfacts"
)

// ResolveExpectedRepositorySnapshot is ResolveExpectedRepository's
// successor for the compiler's stage 3 (SI-257, plan R-PB-3): identical,
// except that the branch it compares against the expectation is
// snapshot.BranchBeingClosed() — the checked-out branch, else a detached
// checkout's validated CI ref, else unknown — rather than the physical
// snapshot.Facts.Branch, which keeps recording the detached checkout.
// Expectations remain assertions only; they never fill an unknown
// computed value. A mismatch or an unknown branch or HEAD is the same
// *ExpectedRepositoryMismatchRefusal, its computed branch taken from that
// resolution. ResolveExpectedRepository itself is unchanged and still
// serves the design-phase candidate arm (conflict.go).
func ResolveExpectedRepositorySnapshot(expected *Expected, snapshot repositoryfacts.Snapshot) error {
	if expected == nil {
		return nil
	}
	facts := snapshot.Facts
	if err := facts.Validate(); err != nil {
		return fmt.Errorf("contextcompile: computed repository facts: %w", err)
	}
	if err := snapshot.CIRef.Validate(); err != nil {
		return fmt.Errorf("contextcompile: computed repository CI ref: %w", err)
	}
	if err := validateNonEmpty("expected.branch", expected.Branch); err != nil {
		return err
	}
	if err := validateGitHash("expected.head", expected.Head); err != nil {
		return err
	}
	branch := snapshot.BranchBeingClosed()
	if !branch.Known || !facts.Head.Known || branch.Value != expected.Branch || facts.Head.Value != expected.Head {
		return &ExpectedRepositoryMismatchRefusal{
			Expected:       *expected,
			ComputedBranch: branch.Value,
			ComputedHead:   facts.Head.Value,
			BranchKnown:    branch.Known,
			HeadKnown:      facts.Head.Known,
		}
	}
	return nil
}
