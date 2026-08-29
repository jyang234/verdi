// Package sealedreview owns canonical sealed-review packets and their
// hermetic compilation boundary.
package sealedreview

import (
	"context"

	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/countersign"
)

const (
	// PacketSchemaID is the sole sealed-review packet schema.
	PacketSchemaID = "verdi.context-review-packet/v1"
	// DiffSchemaID is the sole lossless before/after blob diff schema.
	DiffSchemaID = "verdi.context-review-diff/v1"
	// EvidenceResultSchemaID is the sole redacted review evidence result schema.
	EvidenceResultSchemaID = "verdi.context-evidence-result/v1"
	// EvidenceBundleSchemaID is the sole review evidence bundle schema.
	EvidenceBundleSchemaID = "verdi.context-review-evidence-bundle/v1"
	// AdjudicationSchemaID is the sole acknowledged adjudication wrapper schema.
	AdjudicationSchemaID = "verdi.context-review-adjudication/v1"
	// ReviewBindingSchemaID is the sole packet-to-instruction binding schema.
	ReviewBindingSchemaID = "verdi.context-review-binding/v1"
)

// Round is the closed sealed-review round vocabulary.
type Round string

const (
	RoundR0 Round = "r0"
	RoundR2 Round = "r2"
)

// ItemKind is the closed packet inventory vocabulary.
type ItemKind string

const (
	ItemAcceptedSpec             ItemKind = "accepted-spec"
	ItemCurrentDiff              ItemKind = "current-diff"
	ItemEvidenceBundle           ItemKind = "evidence-bundle"
	ItemBuilderReceipt           ItemKind = "builder-receipt"
	ItemReviewPolicy             ItemKind = "review-policy"
	ItemAdjudication             ItemKind = "adjudication"
	ItemCurrentCandidateEvidence ItemKind = "current-candidate-evidence"
)

// Reviewer is the complete configured reviewer identity carried by a packet.
type Reviewer struct {
	Lane           string               `json:"lane"`
	Adapter        contextevent.Adapter `json:"adapter"`
	AdapterVersion string               `json:"adapter_version"`
	Model          string               `json:"model"`
	ProfileID      string               `json:"profile_id"`
	ProfileDigest  string               `json:"profile_digest"`
}

// Item is one exact provenance-wrapped packet input.
type Item struct {
	Kind          ItemKind `json:"kind"`
	ID            string   `json:"id"`
	MediaType     string   `json:"media_type"`
	ContentDigest string   `json:"content_digest"`
	Content       []byte   `json:"content"`
}

// Packet is the canonical minimum input to one fresh review execution.
type Packet struct {
	Schema               string                   `json:"schema"`
	Round                Round                    `json:"round"`
	Candidate            contextreceipt.Candidate `json:"candidate"`
	Reviewer             Reviewer                 `json:"reviewer"`
	BuilderReceiptDigest string                   `json:"builder_receipt_digest"`
	Items                []Item                   `json:"items"`
	Exclusions           []string                 `json:"exclusions"`
	Digest               string                   `json:"digest"`
}

// DiffState is the closed exact-path change vocabulary.
type DiffState string

const (
	DiffAdded    DiffState = "added"
	DiffModified DiffState = "modified"
	DiffDeleted  DiffState = "deleted"
)

// DiffEntry carries both exact blob sides for one changed path.
type DiffEntry struct {
	Path        []byte    `json:"path"`
	State       DiffState `json:"state"`
	BeforeMode  string    `json:"before_mode"`
	BeforeBlob  string    `json:"before_blob"`
	BeforeBytes []byte    `json:"before_bytes"`
	AfterMode   string    `json:"after_mode"`
	AfterBlob   string    `json:"after_blob"`
	AfterBytes  []byte    `json:"after_bytes"`
}

// Diff is a deterministic before/after blob inventory.
type Diff struct {
	Schema     string      `json:"schema"`
	BaseCommit string      `json:"base_commit"`
	BaseTree   string      `json:"base_tree"`
	HeadCommit string      `json:"head_commit"`
	HeadTree   string      `json:"head_tree"`
	Entries    []DiffEntry `json:"entries"`
	Digest     string      `json:"digest"`
}

// EvidenceResult is one self-digested redacted command result.
type EvidenceResult struct {
	Schema       string              `json:"schema"`
	CommandID    string              `json:"command_id"`
	Argv         []string            `json:"argv"`
	ExitCode     int                 `json:"exit_code"`
	Verdict      countersign.Verdict `json:"verdict"`
	Output       []byte              `json:"output"`
	OutputDigest string              `json:"output_digest"`
	Digest       string              `json:"digest"`
}

// EvidenceScope distinguishes builder evidence from a fresh R2 rebuild.
type EvidenceScope string

const (
	EvidenceScopeBuilder          EvidenceScope = "builder"
	EvidenceScopeCurrentCandidate EvidenceScope = "current-candidate"
)

// EvidenceRow binds one command identity to exact result bytes.
type EvidenceRow struct {
	CommandID    string `json:"command_id"`
	ResultBytes  []byte `json:"result_bytes"`
	ResultDigest string `json:"result_digest"`
}

// EvidenceBundle wraps a sorted set of exact evidence results.
type EvidenceBundle struct {
	Schema    string                   `json:"schema"`
	Scope     EvidenceScope            `json:"scope"`
	Candidate contextreceipt.Candidate `json:"candidate"`
	Rows      []EvidenceRow            `json:"rows"`
	Digest    string                   `json:"digest"`
}

// AdjudicationRow binds one exact adjudication event to its durable general
// event acknowledgment.
type AdjudicationRow struct {
	FindingOrDeviationID string                `json:"finding_or_deviation_id"`
	EventBytes           []byte                `json:"event_bytes"`
	Ack                  contextevent.EventAck `json:"ack"`
}

// Adjudication is the canonical accepted R0 adjudication supplied to R2.
type Adjudication struct {
	Schema          string            `json:"schema"`
	R0ReceiptDigest string            `json:"r0_receipt_digest"`
	Rows            []AdjudicationRow `json:"rows"`
	Digest          string            `json:"digest"`
}

// RepositoryObject is the exact body and declared Git type returned for one
// object identifier requested by PacketCompiler.
type RepositoryObject struct {
	Type    string
	Content []byte
}

// RepositoryReader reads exact named Git objects without an ambient checkout.
type RepositoryReader interface {
	ReadObject(context.Context, string) (RepositoryObject, error)
}

// EvidenceRebuildRequest asks the trusted context compiler to run the
// builder-declared evidence commands at the exact R2 candidate.
type EvidenceRebuildRequest struct {
	Candidate contextreceipt.Candidate
	Commands  []contextreceipt.Evidence
}

// ContextBinding is the exact packet-derived data the compiled manifest and
// instruction projection must echo.
type ContextBinding struct {
	Schema               string                       `json:"schema"`
	PacketDigest         string                       `json:"packet_digest"`
	AcceptedSpecDigest   string                       `json:"accepted_spec_digest"`
	ReviewPolicyDigest   string                       `json:"review_policy_digest"`
	BuilderReceiptDigest string                       `json:"builder_receipt_digest"`
	HeadCommit           string                       `json:"head_commit"`
	HeadTree             string                       `json:"head_tree"`
	ItemProjection       []contextreceipt.ReviewInput `json:"item_projection"`
	Digest               string                       `json:"digest"`
}

// ContextCompileRequest carries the exact finalized packet and its binding to
// the consumer-defined context compiler.
type ContextCompileRequest struct {
	Round       Round
	Candidate   contextreceipt.Candidate
	Reviewer    Reviewer
	PacketBytes []byte
	Binding     ContextBinding
}

// ContextCompileResult is the fresh packet-bound manifest/projection returned
// through the trusted compiler port.
type ContextCompileResult struct {
	ManifestBytes               []byte
	ManifestDigest              string
	InstructionProjectionBytes  []byte
	InstructionProjectionDigest string
	Binding                     ContextBinding
}

// ContextCompiler owns fresh evidence execution plus packet-bound manifest
// and instruction-projection compilation.
type ContextCompiler interface {
	RebuildEvidence(context.Context, EvidenceRebuildRequest) ([][]byte, error)
	Compile(context.Context, ContextCompileRequest) (ContextCompileResult, error)
}

// PacketCompilerPorts are the complete immutable packet compiler dependencies.
type PacketCompilerPorts struct {
	Repository RepositoryReader
	Compiler   ContextCompiler
}

// PacketRequest selects only the fixed I-92 packet sources.
type PacketRequest struct {
	Round                      Round
	Candidate                  contextreceipt.Candidate
	Reviewer                   Reviewer
	AcceptedSpecPath           string
	ReviewPolicyPath           string
	BuilderReceiptBytes        []byte
	BuilderEvidenceResultBytes [][]byte
	PriorReviewReceiptBytes    []byte
	AdjudicationBytes          []byte
}

// PacketResult returns the exact packet, subordinate wrappers, and the fresh
// context compilation that binds them.
type PacketResult struct {
	Packet                   Packet
	PacketBytes              []byte
	Diff                     Diff
	DiffBytes                []byte
	BuilderEvidence          EvidenceBundle
	CurrentCandidateEvidence *EvidenceBundle
	Compilation              ContextCompileResult
}
