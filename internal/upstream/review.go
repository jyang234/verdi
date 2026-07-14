package upstream

import (
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
)

// ReviewVerdict is a review artifact's three-valued `verdict` field
// (PLAN.md §3: "BLOCK / STRUCTURALLY-CLEAR / NO-STRUCTURAL-SIGNAL"). All
// three were observed in spike S1's captures.
type ReviewVerdict string

const (
	ReviewBlock              ReviewVerdict = "BLOCK"
	ReviewStructurallyClear  ReviewVerdict = "STRUCTURALLY-CLEAR"
	ReviewNoStructuralSignal ReviewVerdict = "NO-STRUCTURAL-SIGNAL"
)

var validReviewVerdicts = map[ReviewVerdict]bool{
	ReviewBlock:              true,
	ReviewStructurallyClear:  true,
	ReviewNoStructuralSignal: true,
}

// Touch is one `review.touches[]` entry: a package the reviewed change
// touched, and how many nodes it added to the graph.
type Touch struct {
	Package    string `json:"package"`
	NodesAdded int    `json:"nodes_added,omitempty"`
}

// Violation is one `review.new_violations[]` entry (BLOCK verdicts only).
type Violation struct {
	Rule    string `json:"rule"`
	Summary string `json:"summary"`
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
}

// ContractChange is one `review.contract_changes[]` entry: a boundary-level
// change the reviewed branch introduced (e.g. a new route). Op is upstream's
// own single-character form ("+"/"-"), distinct from this package's own
// DiffOp used by ComputeBoundaryDiff.
type ContractChange struct {
	Op      string `json:"op"`
	Surface string `json:"surface"`
	Name    string `json:"name"`
}

// Caution is one `review.standing_cautions[]` entry: a pre-existing policy
// caveat restated for the reviewed change (e.g. an unproven io_budget
// bound), distinct from NewViolations, which are new.
type Caution struct {
	Rule    string `json:"rule"`
	Summary string `json:"summary"`
}

// Review is `groundwork review --json`'s output, persisted verbatim as
// derived/.../review.json (PLAN.md §3). Every field beyond Service and
// Verdict is optional/omitempty and verdict-dependent, confirmed against
// real captures for all three verdicts (testdata/svcfix-canned, generated
// by re-running spike S1's binaries against testdata/svcfix's own compiled
// fixture rather than guessed): STRUCTURALLY-CLEAR carries Touches and
// ContractChanges; BLOCK carries NewViolations and ReachableFrom;
// NO-STRUCTURAL-SIGNAL carries none of those four. StandingCautions,
// Algo, Caveats, and Digest appear on every verdict observed.
type Review struct {
	Service          string           `json:"service"`
	Verdict          ReviewVerdict    `json:"verdict"`
	Shape            string           `json:"shape,omitempty"`
	Touches          []Touch          `json:"touches,omitempty"`
	ContractChanges  []ContractChange `json:"contract_changes,omitempty"`
	NewViolations    []Violation      `json:"new_violations,omitempty"`
	ReachableFrom    []string         `json:"reachable_from,omitempty"`
	StandingCautions []Caution        `json:"standing_cautions,omitempty"`
	Algo             string           `json:"algo,omitempty"`
	Caveats          []string         `json:"caveats,omitempty"`
	Digest           string           `json:"digest,omitempty"`
}

// DecodeReview strict-decodes and validates a review artifact.
func DecodeReview(data []byte) (*Review, error) {
	var r Review
	if err := artifact.DecodeStrictJSON(data, &r); err != nil {
		return nil, fmt.Errorf("upstream: decoding review artifact: %w", err)
	}
	if err := r.Validate(); err != nil {
		return nil, err
	}
	return &r, nil
}

// Validate checks Service is non-empty and Verdict is a known enum, failing
// closed on any value spike S1 did not observe.
func (r Review) Validate() error {
	if r.Service == "" {
		return fmt.Errorf("upstream: review artifact has an empty service")
	}
	if !validReviewVerdicts[r.Verdict] {
		return fmt.Errorf("upstream: review artifact: unknown verdict %q", r.Verdict)
	}
	return nil
}

// Blocking reports whether r's verdict is BLOCK — the exit-1 case
// (`groundwork review`'s own contract: "BLOCK exits non-zero").
func (r Review) Blocking() bool { return r.Verdict == ReviewBlock }
