// Package contextreceipt defines the closed canonical receipt emitted by
// terminal sealed execution and review paths.
package contextreceipt

import (
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// SchemaID is the sole accepted context-receipt schema.
const SchemaID = "verdi.context-receipt/v1"

// Role reuses the closed receipt-role vocabulary carried by receipt events.
type Role = contextevent.Role

const (
	RoleBuilder  = contextevent.RoleBuilder
	RoleReviewer = contextevent.RoleReviewer
)

// Authority reuses the closed event and receipt authority vocabulary.
type Authority = contextevent.Authority

const (
	AuthorityAuthoritative = contextevent.AuthorityAuthoritative
	AuthorityAdvisory      = contextevent.AuthorityAdvisory
)

// Expansion is one closed manifest expansion identity row.
type Expansion struct {
	RequestID            string `json:"request_id"`
	ParentRevision       uint64 `json:"parent_revision"`
	ParentManifestDigest string `json:"parent_manifest_digest"`
	ChildRevision        uint64 `json:"child_revision"`
	ChildManifestDigest  string `json:"child_manifest_digest"`
	ExpansionDigest      string `json:"expansion_digest"`
}

// Obligation is one closed accepted-obligation identity row.
type Obligation struct {
	Ref           string                `json:"ref"`
	Path          string                `json:"path"`
	AC            string                `json:"ac"`
	Kind          artifact.EvidenceKind `json:"kind"`
	ContentDigest string                `json:"content_digest"`
	Producer      string                `json:"producer"`
}

// Evidence is one closed evidence command/result row. Argv preserves command
// order while the rows themselves form a canonical set.
type Evidence struct {
	CommandID    string              `json:"command_id"`
	Argv         []string            `json:"argv"`
	ExitCode     int                 `json:"exit_code"`
	Verdict      countersign.Verdict `json:"verdict"`
	OutputDigest string              `json:"output_digest"`
}

// ReviewInput is one closed review packet identity row.
type ReviewInput struct {
	Kind          string `json:"kind"`
	ContentDigest string `json:"content_digest"`
}

// Receipt is the complete acyclic context receipt. The subsequently emitted
// receipt event and its acknowledgment are deliberately outside this record.
type Receipt struct {
	Schema                          string                                  `json:"schema"`
	Role                            Role                                    `json:"role"`
	Authority                       Authority                               `json:"authority"`
	ManifestDigest                  string                                  `json:"manifest_digest"`
	DispatchDigest                  string                                  `json:"dispatch_digest"`
	ATCRunway                       string                                  `json:"atc_runway"`
	ExecutionWorkspaceRequestDigest string                                  `json:"execution_workspace_request_digest"`
	ExecutionWorkspaceID            string                                  `json:"execution_workspace_id"`
	InputCommit                     string                                  `json:"input_commit"`
	InputTree                       string                                  `json:"input_tree"`
	OutputCommit                    string                                  `json:"output_commit"`
	OutputTree                      string                                  `json:"output_tree"`
	Clean                           bool                                    `json:"clean"`
	RevisionSegments                []contextevent.Revision                 `json:"revision_segments"`
	EventChainRoot                  string                                  `json:"event_chain_root"`
	TerminalManifestRevision        uint64                                  `json:"terminal_manifest_revision"`
	TerminalSourceSequence          uint64                                  `json:"terminal_source_sequence"`
	TerminalGlobalSequence          uint64                                  `json:"terminal_global_sequence"`
	Expansions                      []Expansion                             `json:"expansions"`
	Obligations                     []Obligation                            `json:"obligations"`
	Evidence                        []Evidence                              `json:"evidence"`
	RunnerPrincipalResolution       governanceprincipal.PrincipalResolution `json:"runner_principal_resolution"`
	Adapter                         contextevent.Adapter                    `json:"adapter"`
	AdapterVersion                  string                                  `json:"adapter_version"`
	ReviewInputs                    []ReviewInput                           `json:"review_inputs"`
	ReviewOf                        []string                                `json:"review_of,omitempty"`
	Digest                          string                                  `json:"digest"`
}
