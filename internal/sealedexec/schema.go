// Package sealedexec defines the strict canonical wire boundary for sealed
// provider execution requests, continuity checkpoints, results, and provider
// inputs.
package sealedexec

import (
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/policyconflict"
)

const (
	// ExecutionRequestSchemaID is the sealed start-or-resume request schema.
	ExecutionRequestSchemaID = "verdi.context-execution-request/v1"
	// ExecutionContinuitySchemaID is the resumable execution checkpoint schema.
	ExecutionContinuitySchemaID = "verdi.execution-continuity/v1"
	// ExecutionResultSchemaID is the finalized public execution result schema.
	ExecutionResultSchemaID = "verdi.context-execution-result/v1"
	// InstructionProjectionSchemaID is the sole provider instruction channel.
	InstructionProjectionSchemaID = "verdi.instruction-projection/v1"
	// ProjectProfileRefSchemaID identifies a logical sealed project profile.
	ProjectProfileRefSchemaID = "verdi.sealed-project-profile-ref/v1"
	// RecorderEndpointRefSchemaID identifies a logical durable recorder sink.
	RecorderEndpointRefSchemaID = "verdi.context-recorder-endpoint-ref/v1"
)

// Action selects exactly one execution request arm.
type Action string

const (
	// ActionStart selects a fresh execution.
	ActionStart Action = "start"
	// ActionResume selects a canonical continuity checkpoint.
	ActionResume Action = "resume"
)

// LogicalRef is a credential-free identity for a service-resolved operand.
type LogicalRef struct {
	Schema string `json:"schema"`
	ID     string `json:"id"`
	Digest string `json:"digest"`
}

// InstructionFile is one exact UTF-8 instruction-projection file.
type InstructionFile struct {
	Path          string `json:"path"`
	ContentDigest string `json:"content_digest"`
	Content       string `json:"content"`
}

// InstructionProjection is the immutable, self-digested project-authority
// channel supplied to a provider.
type InstructionProjection struct {
	Schema string            `json:"schema"`
	Files  []InstructionFile `json:"files"`
	Digest string            `json:"digest"`
}

// StartArm fixes the first source sequence for a fresh execution.
type StartArm struct {
	ExpectedSourceSequence uint64 `json:"expected_source_sequence"`
}

// ResumeArm embeds the complete canonical continuity checkpoint and repeats
// its digest as the request-arm binding.
type ResumeArm struct {
	Continuity       ExecutionContinuity `json:"continuity"`
	ContinuityDigest string              `json:"continuity_digest"`
}

// ExecutionRequest is the complete sealed start-or-resume request envelope.
type ExecutionRequest struct {
	Schema                    string
	Action                    Action
	Flight                    string
	Lane                      string
	Epoch                     string
	ManifestRevision          uint64
	Session                   string
	ATCRunway                 string
	InputCommit               string
	InputTree                 string
	Manifest                  contextcompile.Manifest
	ManifestDigest            string
	InstructionProjection     InstructionProjection
	ProjectionDigest          string
	ExecutionWorkspaceRequest execworkspace.Identity
	Adapter                   contextevent.Adapter
	AdapterVersion            string
	Profile                   LogicalRef
	Grants                    execworkspace.GrantSet
	AuthorityVerdict          policyconflict.Report
	RecorderEndpoint          LogicalRef
	Start                     *StartArm
	Resume                    *ResumeArm
}

// ExecutionContinuity is the complete self-digested resume checkpoint.
type ExecutionContinuity struct {
	Schema                          string                  `json:"schema"`
	Flight                          string                  `json:"flight"`
	Lane                            string                  `json:"lane"`
	Epoch                           string                  `json:"epoch"`
	Session                         string                  `json:"session"`
	Adapter                         contextevent.Adapter    `json:"adapter"`
	AdapterVersion                  string                  `json:"adapter_version"`
	ATCRunway                       string                  `json:"atc_runway"`
	InputCommit                     string                  `json:"input_commit"`
	InputTree                       string                  `json:"input_tree"`
	CurrentCommit                   string                  `json:"current_commit"`
	CurrentTree                     string                  `json:"current_tree"`
	ExecutionWorkspaceID            string                  `json:"execution_workspace_id"`
	ExecutionWorkspaceRequestDigest string                  `json:"execution_workspace_request_digest"`
	ProfileDigest                   string                  `json:"profile_digest"`
	GrantDigest                     string                  `json:"grant_digest"`
	AuthorityVerdictDigest          string                  `json:"authority_verdict_digest"`
	CurrentManifestRevision         uint64                  `json:"current_manifest_revision"`
	CurrentManifestDigest           string                  `json:"current_manifest_digest"`
	ProjectionDigest                string                  `json:"projection_digest"`
	RevisionSegments                []contextevent.Revision `json:"revision_segments"`
	EventChainRoot                  string                  `json:"event_chain_root"`
	ExpansionLedgerRoot             string                  `json:"expansion_ledger_root"`
	TerminalSourceSequence          uint64                  `json:"terminal_source_sequence"`
	TerminalGlobalSequence          uint64                  `json:"terminal_global_sequence"`
	RecorderCheckpointDigest        string                  `json:"recorder_checkpoint_digest"`
	AdapterSessionRef               string                  `json:"adapter_session_ref"`
	Digest                          string                  `json:"digest"`
}

// ExecutionResult is the complete acyclic public result. Receipt and
// ReceiptEventAck are its ratified integrity boundary; no result self-digest
// exists.
type ExecutionResult struct {
	Schema                   string
	Verdict                  contextcompile.Resolution
	Authority                contextevent.Authority
	Witnesses                []string
	Flight                   string
	Lane                     string
	Epoch                    string
	Session                  string
	ATCRunway                string
	ExecutionWorkspaceID     string
	Adapter                  contextevent.Adapter
	AdapterVersion           string
	InputCommit              string
	InputTree                string
	OutputCommit             string
	OutputTree               string
	Clean                    bool
	TerminalManifestDigest   string
	TerminalManifestRevision uint64
	TerminalSourceSequence   uint64
	TerminalGlobalSequence   uint64
	EventChainRoot           string
	Receipt                  contextreceipt.Receipt
	ReceiptEventAck          contextevent.ReceiptEventAck
}

// InstructionAuthority is the one explicit provider instruction channel.
type InstructionAuthority struct {
	Projection InstructionProjection
}

// ProviderInput separates immutable project authority from provenance-
// wrapped repository and corpus data.
type ProviderInput struct {
	Instructions InstructionAuthority
	Data         []contextcompile.DataItem
}
