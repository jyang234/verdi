package specimport

import (
	"fmt"
	"net/url"

	"github.com/jyang234/verdi/internal/canonjson"
)

// Result schema constants (spec-import-contract.md, "Preview, identity and
// atomic publication" / "Errors and browser behavior").
const (
	PreviewResultSchema = "verdi.spec-import-preview/v1"
	ResultSchema        = "verdi.spec-import-result/v1"
)

// Apply Result.Status values (spec-import-contract.md: "Result schema ...
// has status created or already-created").
const (
	StatusCreated        = "created"
	StatusAlreadyCreated = "already-created"
)

// PreviewResult is Preview's exact deterministic, read-only output
// (spec-import-contract.md, "Preview, identity and atomic publication").
// Every field is part of the replay domain Digest binds, except Digest
// itself. Candidate is nil exactly when the candidate could not be
// constructed (a blocking finding); Ready is true only when Findings
// carries no blocking entry.
type PreviewResult struct {
	Schema        string     `json:"schema"`
	Digest        string     `json:"digest"`
	BaseCommit    string     `json:"base_commit"`
	ModelDigest   string     `json:"model_digest"`
	ConfigDigest  string     `json:"config_digest"`
	EngineDigest  string     `json:"engine_digest"`
	RequestDigest string     `json:"request_digest"`
	SpecRef       string     `json:"spec_ref"`
	Candidate     []byte     `json:"candidate,omitempty"`
	Fields        []Field    `json:"fields"`
	Sources       []Snapshot `json:"sources"`
	Coverage      []Coverage `json:"coverage"`
	Findings      []Finding  `json:"findings"`
	Ready         bool       `json:"ready"`
}

// computePreviewDigest returns pr's SHA-256 lowercase hex digest over its
// own canonical JSON encoding, with the Digest field itself zeroed first
// (spec-import-contract.md: "digests are SHA-256 lowercase hex over
// canonical JSON excluding the digest itself"). Request order is preserved
// end to end — Fields/Sources/Coverage/Findings are never re-sorted here —
// so the same bytes/options/project context always reproduce the same
// digest.
func computePreviewDigest(pr PreviewResult) (string, error) {
	pr.Digest = ""
	data, err := canonjson.Marshal(pr)
	if err != nil {
		return "", fmt.Errorf("specimport: digesting preview result: %w", err)
	}
	return sha256Hex(data), nil
}

// Result is Apply's exact typed outcome (spec-import-contract.md: "Result
// schema verdi.spec-import-result/v1 has status created or already-created,
// branch, commit, spec_ref, preview_digest, statements_deferred boolean,
// disclosures (Finding values), and board_path").
type Result struct {
	Schema             string    `json:"schema"`
	Status             string    `json:"status"`
	Branch             string    `json:"branch"`
	Commit             string    `json:"commit"`
	SpecRef            string    `json:"spec_ref"`
	PreviewDigest      string    `json:"preview_digest"`
	StatementsDeferred bool      `json:"statements_deferred"`
	Disclosures        []Finding `json:"disclosures"`
	BoardPath          string    `json:"board_path"`
}

// specRef returns the canonical whole spec ref for slug.
func specRef(slug string) string { return "spec/" + slug }

// designBranch returns the create-only target ref's short branch name for
// slug: "design/<slug>".
func designBranch(slug string) string { return "design/" + slug }

// boardPath builds Result.BoardPath using the existing branch-board route
// convention (spec-import-contract.md: "board_path using the existing
// /b/design%2F<slug>/board/spec/<slug> branch-board route convention").
// url.PathEscape treats designBranch(slug) as one path SEGMENT, so its "/"
// is escaped to "%2F" exactly like the existing route; this package does
// not import internal/workbench (a later, UI-owned adapter) merely to reuse
// one string template.
func boardPath(slug string) string {
	return fmt.Sprintf("/b/%s/board/spec/%s", url.PathEscape(designBranch(slug)), slug)
}
