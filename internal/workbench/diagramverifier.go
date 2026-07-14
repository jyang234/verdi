// The diagram editor's verification-rail port (spec/board-editor ac-5,
// dc-4): the workbench defines the CONSUMER-side interface (04 §port
// pattern — interfaces at the consumer), and the verification-extractor
// story's deliverable implements it. The rail consumes, never computes
// (co-3): this file carries no graph analysis, no flowmap semantics, no
// tier arithmetic — only the report shape the rail renders verbatim and
// a canned-file implementation for the hermetic e2e harness (co-4),
// mirroring CommentFeed/CannedCommentFeed exactly.
package workbench

import (
	"context"
	"fmt"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
)

// DiagramVerification is one proposal's verification report as the rail
// renders it: the artifact-wide coverage tier (full / partial /
// illustrative — spec/diagram-proposals dc-3) and the per-element
// findings (spec/diagram-proposals ac-1). The rail renders these AS
// GIVEN; it never derives, corrects, or fills them in.
type DiagramVerification struct {
	Tier     string           `json:"tier"`
	Findings []DiagramFinding `json:"findings"`
}

// DiagramFinding is one element's disclosed classification. Witness is
// the CANDIDATE witness commit a contradicted finding may carry
// (verification-extractor dc-4's corrected candor) — rendered as a
// candidate, never as a verified cause.
type DiagramFinding struct {
	Identity string `json:"identity"`
	Kind     string `json:"kind"`
	Witness  string `json:"witness,omitempty"`
}

// The closed vocabularies the rail accepts. Unknown values fail closed
// (CLAUDE.md) at the seam — a fabricated tier is exactly the lie ac-5
// forbids the rail to render.
var (
	diagramTiers = map[string]bool{"full": true, "partial": true, "illustrative": true}
	findingKinds = map[string]bool{"exists": true, "proposed-new": true, "contradicted": true, "stale-base": true}
)

// Validate checks the report speaks only the closed vocabularies.
func (v DiagramVerification) Validate() error {
	if !diagramTiers[v.Tier] {
		return fmt.Errorf("workbench: verification tier %q is not a known tier (full/partial/illustrative); fail closed", v.Tier)
	}
	for _, f := range v.Findings {
		if !findingKinds[f.Kind] {
			return fmt.Errorf("workbench: finding kind %q is not a known kind (exists/proposed-new/contradicted/stale-base); fail closed", f.Kind)
		}
	}
	return nil
}

// DiagramVerifier is the rail's one seam (dc-4). VerifyDiagram returns
// the named proposal's report, or an error when no report can be given —
// the rail then renders the disclosed verification-unavailable state.
// The rail NEVER blocks on this interface's outcome: an edit and a save
// succeed identically with a report, without one, and through an error.
type DiagramVerifier interface {
	VerifyDiagram(ctx context.Context, name string) (*DiagramVerification, error)
}

// CannedDiagramVerifier is a DiagramVerifier backed by one strict-decoded
// JSON file mapping diagram name → report — the hermetic double the e2e
// harness wires through `verdi serve` (VERDI_DIAGRAM_VERIFICATION; no
// network in any test, co-4). A diagram absent from the file has no
// report: the rail renders the disclosed unavailable state for it.
type CannedDiagramVerifier struct {
	reports map[string]DiagramVerification
}

// LoadCannedDiagramVerifier strict-decodes path (unknown fields and
// unknown enum values fail closed at load, never at render).
func LoadCannedDiagramVerifier(path string) (*CannedDiagramVerifier, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("workbench: reading canned diagram verification: %w", err)
	}
	var reports map[string]DiagramVerification
	if err := artifact.DecodeStrictJSON(data, &reports); err != nil {
		return nil, fmt.Errorf("workbench: canned diagram verification %s: %w", path, err)
	}
	for name, r := range reports {
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("workbench: canned diagram verification %s (diagram %q): %w", path, name, err)
		}
	}
	return &CannedDiagramVerifier{reports: reports}, nil
}

// VerifyDiagram implements DiagramVerifier.
func (c *CannedDiagramVerifier) VerifyDiagram(_ context.Context, name string) (*DiagramVerification, error) {
	r, ok := c.reports[name]
	if !ok {
		return nil, fmt.Errorf("no verification report for diagram %q", name)
	}
	return &r, nil
}
