// Package contextcompile defines the Wave-3 context compiler's strict wire
// contracts (docs/superpowers/specs/2026-08-11-context-compiler-authority-design.md,
// ledger SI-79/SI-80): the request the caller supplies, the manifest and
// data-item payloads the compiler returns. This file declares only the
// shapes; codec.go owns byte<->value conversion and validate.go owns every
// grammar, enum-closure, digest, and ordering rule.
//
// Every nested type below carries `json` tags and decodes directly (no
// separate per-field wire mirror) — the established internal/journey
// pattern for a schema whose nested fields need no absent/null
// distinction beyond "empty is invalid". Only Request, Manifest, and
// DataItem themselves are exempt: they are never marshaled directly (their
// top-level fields need absent/null/empty presence detection a plain
// struct tag cannot give), so codec.go decodes/encodes them through a
// private pointer-backed wire document instead.
package contextcompile

import (
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// Schema identifiers for the three wire documents this package owns
// (authority design §§3, 8.1, 8.2).
const (
	RequestSchema  = "verdi.context-compile-request/v1"
	ManifestSchema = "verdi.context-manifest/v1"
	DataItemSchema = "verdi.context-data-item/v1"
)

// --- closed enums (authority design §§3, 5.1, 5.2, 6, 7, 8.2) -------------

// Phase is one of the three lifecycle phases a request or manifest may name
// (authority design §3). Unknown values fail closed.
type Phase string

const (
	PhaseDesign Phase = "design"
	PhaseBuild  Phase = "build"
	PhaseReview Phase = "review"
)

// Source is a candidate's source classification (authority design §5's
// source table). Unknown values fail closed.
type Source string

const (
	SourceHeadTree        Source = "head-tree"
	SourceWorktreeOverlay Source = "worktree-overlay"
	SourceStoreAuthority  Source = "store-authority"
	SourceDeclaredContext Source = "declared-context"
	SourceProjection      Source = "projection"
	SourceOpaque          Source = "opaque"
)

// IncludedKind is one of the seven closed included-candidate kinds
// (authority design §5.1). Unknown values fail closed.
type IncludedKind string

const (
	IncludedAcceptedSpec          IncludedKind = "accepted-spec"
	IncludedParentFeatureFragment IncludedKind = "parent-feature-fragment"
	IncludedObligation            IncludedKind = "obligation"
	IncludedPolicyArtifact        IncludedKind = "policy-artifact"
	IncludedRepositoryFile        IncludedKind = "repository-file"
	IncludedDeclaredContextRef    IncludedKind = "declared-context-ref"
	IncludedInstructionProjection IncludedKind = "instruction-projection"
)

// ExclusionReason is one of the ten closed exclusion reasons (authority
// design §5.2). Unknown values fail closed.
type ExclusionReason string

const (
	ExclusionDesignProvenanceSidecar   ExclusionReason = "design-provenance-sidecar"
	ExclusionDataZoneDisposable        ExclusionReason = "data-zone-disposable"
	ExclusionUncommittedContent        ExclusionReason = "uncommitted-content"
	ExclusionOutOfDeclaredScope        ExclusionReason = "out-of-declared-scope"
	ExclusionPhaseInapplicable         ExclusionReason = "phase-inapplicable"
	ExclusionSupersededSpec            ExclusionReason = "superseded-spec"
	ExclusionArchivedRecord            ExclusionReason = "archived-record"
	ExclusionGeneratedProjectionOutput ExclusionReason = "generated-projection-output"
	ExclusionNonTextData               ExclusionReason = "non-text-data"
	ExclusionNonRegularFile            ExclusionReason = "non-regular-file"
)

// Applicability is the closed three-valued phase/scope applicability result
// (authority design §6). Unknown values fail closed.
type Applicability string

const (
	ApplicabilityApplicable   Applicability = "applicable"
	ApplicabilityInapplicable Applicability = "inapplicable"
	ApplicabilityUnknown      Applicability = "unknown"
)

// Resolution is the closed three-valued proof state shared by
// actors.posture and required_inputs[].resolution (authority design §8.2).
// Unknown values fail closed.
type Resolution string

const (
	ResolutionProven              Resolution = "proven"
	ResolutionViolatedWithWitness Resolution = "violated-with-witness"
	ResolutionUnproven            Resolution = "unproven"
)

// PayloadChannel discriminates authority (instruction-projection) bytes
// from ordinary non-authoritative data bytes (authority design §7).
// Unknown values fail closed.
type PayloadChannel string

const (
	ChannelData      PayloadChannel = "data"
	ChannelAuthority PayloadChannel = "authority"
)

// DisclosureCode is the closed manifest disclosure vocabulary (authority
// design §8.2's fourteen codes). Unknown values fail closed.
type DisclosureCode string

const (
	DisclosureActorResolutionUnproven      DisclosureCode = "actor-resolution-unproven"
	DisclosureRepositoryRemoteUnknown      DisclosureCode = "repository-remote-unknown"
	DisclosureRepositoryBranchUnknown      DisclosureCode = "repository-branch-unknown"
	DisclosureRepositoryHeadUnknown        DisclosureCode = "repository-head-unknown"
	DisclosureDefaultBranchUnknown         DisclosureCode = "default-branch-unknown"
	DisclosureDefaultRelationshipUnknown   DisclosureCode = "default-relationship-unknown"
	DisclosureDirtyStateUnknown            DisclosureCode = "dirty-state-unknown"
	DisclosureStagedStateUnknown           DisclosureCode = "staged-state-unknown"
	DisclosureFreshnessUnknown             DisclosureCode = "freshness-unknown"
	DisclosureApplicabilityUnknown         DisclosureCode = "applicability-unknown"
	DisclosureReviewResultDiffUnproven     DisclosureCode = "review-result-diff-unproven"
	DisclosureReviewEvidenceBundleUnproven DisclosureCode = "review-evidence-bundle-unproven"
	DisclosureReviewBuilderReceiptUnproven DisclosureCode = "review-builder-receipt-unproven"
	DisclosureOpaqueHarnessVendorBase      DisclosureCode = "opaque-harness-vendor-base"
)

// --- smaller closed vocabularies (plain strings, validated in validate.go) -

// The three closed policy-entry kinds (authority design §8.2).
const (
	PolicyEntryPolicy    = "policy"
	PolicyEntryOverlay   = "overlay"
	PolicyEntryExemption = "exemption"
)

// The five closed required-input kinds (authority design §6 review capsule,
// §8.2).
const (
	RequiredInputAcceptedSpec   = "accepted-spec"
	RequiredInputResultDiff     = "result-diff"
	RequiredInputEvidenceBundle = "evidence-bundle"
	RequiredInputBuilderReceipt = "builder-receipt"
	RequiredInputReviewPolicy   = "review-policy"
)

// EvidenceAuthorityAdvisory is v1's one legal evidence.authority value
// (authority design §8.2: "advisory for every v1 local compile").
const EvidenceAuthorityAdvisory = "advisory"

// The three closed evidence-freshness values (authority design §8.2).
const (
	EvidenceFreshnessFresh   = "fresh"
	EvidenceFreshnessStale   = "stale"
	EvidenceFreshnessUnknown = "unknown"
)

// OpaqueKindHarnessVendorBase is the one legal opaque-entry kind (authority
// design §5's opaque source table row).
const OpaqueKindHarnessVendorBase = "harness-vendor-base"

// DataItemClassification is the one legal data-item classification
// (authority design §8.1).
const DataItemClassification = "non-authoritative-data"

// The five closed repository relationship values (shared repository-facts
// shape, authority design §4/§8.2; mirrors internal/journey's
// RepositoryFacts.Relationship pending the SI-85 extraction).
const (
	RelationshipEqual    = "equal"
	RelationshipAhead    = "ahead"
	RelationshipBehind   = "behind"
	RelationshipDiverged = "diverged"
	RelationshipUnknown  = "unknown"
)

// The four closed repository source values (shared repository-facts shape,
// authority design §8.2; mirrors internal/journey's RepositoryFacts.Source).
const (
	RepoSourceHead         = "head"
	RepoSourceWorkingTree  = "working-tree"
	RepoSourceRemoteRef    = "remote-ref"
	RepoSourceReceiptBound = "receipt-bound"
)

// --- request (authority design §3) -----------------------------------------

// AdapterRef identifies a harness adapter by its constitution-registered
// (id, version) pair. It is never the store's full Adapter declaration
// (policyartifact.Adapter's managed files and discovery filenames) — only
// the reference a request claims and a manifest echoes back.
type AdapterRef struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// Expected is the request's optional-as-a-whole expected-repository claim
// (authority design §3): when present, both Branch and Head are mandatory.
type Expected struct {
	Branch string `json:"branch"`
	Head   string `json:"head"`
}

// Request is the decoded, validated `verdi.context-compile-request/v1`
// document (authority design §3). Expected is nil when the request omits
// the optional expected-repository claim. Request is never marshaled
// directly; DecodeRequest/EncodeRequest go through a private wire document
// that distinguishes absent, explicitly null, and present-empty fields.
type Request struct {
	Schema   string
	Adapter  AdapterRef
	Expected *Expected
	Grants   execworkspace.GrantSet
	Phase    Phase
	Scope    policyartifact.Scope
	Spec     string
}

// --- shared repository-facts shape (authority design §4, §8.2) -------------
//
// These mirror internal/journey's RepositoryFacts fields exactly (SI-85:
// "the shared fact shape canonically encodes exactly like the current
// journey repository section"). Task 1 defines this package's own copy —
// the shared internal/repositoryfacts leaf does not exist yet (a later
// task extracts it); a future task may convert into these fields but does
// not need to change their wire shape to do so.

// StringFact is a string-valued fact whose presence is itself proven or
// unproven: Value must be "" whenever Known is false.
type StringFact struct {
	Known bool   `json:"known"`
	Value string `json:"value"`
}

// BoolFact is a boolean-valued fact with the same known/unproven
// discipline as StringFact.
type BoolFact struct {
	Known bool `json:"known"`
	Value bool `json:"value"`
}

// DefaultBranchFact is the configured default branch's identity.
type DefaultBranchFact struct {
	Known bool   `json:"known"`
	Name  string `json:"name"`
	Ref   string `json:"ref"`
	Head  string `json:"head"`
}

// WorktreeFact is the active worktree's identity.
type WorktreeFact struct {
	Managed bool   `json:"managed"`
	Name    string `json:"name"`
}

// RepositoryFacts is the manifest's `repository` section: shared repository
// facts plus this manifest's own disclosures for any unknown fact
// (authority design §8.2).
type RepositoryFacts struct {
	RemoteOrigin  StringFact        `json:"remote_origin"`
	Branch        StringFact        `json:"branch"`
	Head          StringFact        `json:"head"`
	DefaultBranch DefaultBranchFact `json:"default_branch"`
	Relationship  string            `json:"relationship"`
	Dirty         BoolFact          `json:"dirty"`
	Staged        BoolFact          `json:"staged"`
	Worktree      WorktreeFact      `json:"worktree"`
	Source        string            `json:"source"`
	Disclosures   []DisclosureCode  `json:"disclosures"`
}

// --- manifest (authority design §8.2) ---------------------------------------

// Revisions is the manifest's `revisions` section. Parent is never carried
// on the domain type: v1's root context revision is always 1 with no
// parent (authority design §9), so the wire codec rejects any encoded
// parent rather than modeling a field that can never legally hold a value.
type Revisions struct {
	Authority string `json:"authority"`
	Context   int    `json:"context"`
}

// AcceptedSpec is the manifest's `accepted_spec` section: the merge-
// signaled accepted baseline's identity and exact-byte digest.
type AcceptedSpec struct {
	Ref           string `json:"ref"`
	Path          string `json:"path"`
	Blob          string `json:"blob"`
	Commit        string `json:"commit"`
	ContentDigest string `json:"content_digest"`
}

// ParentFeature is one row of the manifest's sorted `parent_features` list.
type ParentFeature struct {
	Ref            string `json:"ref"`
	Path           string `json:"path"`
	SourceDigest   string `json:"source_digest"`
	FragmentDigest string `json:"fragment_digest"`
	PayloadDigest  string `json:"payload_digest"`
}

// DecisionRef is one row of the manifest's sorted `decisions` list.
type DecisionRef struct {
	Ref           string `json:"ref"`
	ContentDigest string `json:"content_digest"`
}

// Obligation is one row of the manifest's sorted `obligations` list.
type Obligation struct {
	Ref           string                `json:"ref"`
	Path          string                `json:"path"`
	AC            string                `json:"ac"`
	Kind          artifact.EvidenceKind `json:"kind"`
	ContentDigest string                `json:"content_digest"`
}

// PolicyEntry is one row of policy.entries: one applicable policy, overlay,
// or exemption operand.
type PolicyEntry struct {
	Kind          string        `json:"kind"`
	ID            string        `json:"id"`
	Digest        string        `json:"digest"`
	Applicability Applicability `json:"applicability"`
}

// PolicySection is the manifest's `policy` section.
type PolicySection struct {
	EffectiveDigest    string        `json:"effective_digest"`
	ConstitutionDigest string        `json:"constitution_digest"`
	ProfileID          string        `json:"profile_id"`
	ProfileDigest      string        `json:"profile_digest"`
	Entries            []PolicyEntry `json:"entries"`
}

// GovernanceProfileRef is the manifest's `governance_profile` section.
type GovernanceProfileRef struct {
	ID     string                    `json:"id"`
	Class  governanceprincipal.Class `json:"class"`
	Digest string                    `json:"digest"`
}

// ActorsSection is the manifest's `actors` section. Resolutions carries
// only governanceprincipal.PrincipalResolution's exported fields (its
// private seal never round-trips): the manifest is a canonical projection
// of an already-sealed resolution, never a reconstitutable one (authority
// design §4).
type ActorsSection struct {
	Posture     Resolution                                `json:"posture"`
	Resolutions []governanceprincipal.PrincipalResolution `json:"resolutions"`
	Disclosures []DisclosureCode                          `json:"disclosures"`
}

// IncludedEntry is one row of the manifest's sorted `included` ledger. Path
// and Ref are nil when inapplicable to this entry's kind.
type IncludedEntry struct {
	ID             string           `json:"id"`
	Source         Source           `json:"source"`
	Kind           IncludedKind     `json:"kind"`
	Applicability  Applicability    `json:"applicability"`
	PayloadChannel PayloadChannel   `json:"payload_channel"`
	Path           *string          `json:"path,omitempty"`
	Ref            *string          `json:"ref,omitempty"`
	ContentDigest  string           `json:"content_digest"`
	PayloadDigest  string           `json:"payload_digest"`
	Disclosures    []DisclosureCode `json:"disclosures"`
}

// ExcludedEntry is one row of the manifest's sorted `excluded` ledger. It
// never carries content or a content digest.
type ExcludedEntry struct {
	ID            string           `json:"id"`
	Source        Source           `json:"source"`
	Reason        ExclusionReason  `json:"reason"`
	Applicability Applicability    `json:"applicability"`
	Path          *string          `json:"path,omitempty"`
	Ref           *string          `json:"ref,omitempty"`
	Disclosures   []DisclosureCode `json:"disclosures"`
}

// OpaqueEntry is one row of the manifest's sorted `opaque` ledger: the one
// fixed adapter-owned harness-vendor-base candidate. It claims no digest.
type OpaqueEntry struct {
	ID          string           `json:"id"`
	Kind        string           `json:"kind"`
	Adapter     AdapterRef       `json:"adapter"`
	Disclosures []DisclosureCode `json:"disclosures"`
}

// ProjectionFileRef is one row of the manifest's sorted `projection_files`
// list: one selected-adapter file the pure renderer produced.
type ProjectionFileRef struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// RequiredInput is one row of the manifest's sorted `required_inputs` list
// (`[]` for design/build; exactly the review capsule's five rows for
// review). Digest is nil when the row's resolution carries no digest
// witness.
type RequiredInput struct {
	Kind       string     `json:"kind"`
	Resolution Resolution `json:"resolution"`
	Digest     *string    `json:"digest,omitempty"`
	Witnesses  []string   `json:"witnesses"`
}

// EvidenceSection is the manifest's `evidence` section. ConsumedReports is
// always `[]` in v1 (authority design §8.2).
type EvidenceSection struct {
	Authority       string           `json:"authority"`
	Freshness       string           `json:"freshness"`
	ConsumedReports []string         `json:"consumed_reports"`
	Disclosures     []DisclosureCode `json:"disclosures"`
}

// Manifest is the decoded, validated `verdi.context-manifest/v1` document
// (authority design §8.2). Dispositions and expansions are never carried on
// the domain type: v1 always serializes them as `[]` and rejects any
// nonempty encoding, so — like Revisions.Parent — no field exists for a
// value that can never legally be set. Manifest is never marshaled
// directly; DecodeManifest/EncodeManifest go through a private wire
// document.
type Manifest struct {
	Schema            string
	Phase             Phase
	Adapter           AdapterRef
	Revisions         Revisions
	AcceptedSpec      AcceptedSpec
	ParentFeatures    []ParentFeature
	Decisions         []DecisionRef
	Obligations       []Obligation
	Repository        RepositoryFacts
	Policy            PolicySection
	Owners            []string
	Scope             policyartifact.Scope
	GovernanceProfile GovernanceProfileRef
	Actors            ActorsSection
	Included          []IncludedEntry
	Excluded          []ExcludedEntry
	Opaque            []OpaqueEntry
	Capabilities      execworkspace.GrantSet
	ProjectionFiles   []ProjectionFileRef
	RequiredInputs    []RequiredInput
	Evidence          EvidenceSection
	Disclosures       []DisclosureCode
	Digest            string
}

// --- data item (authority design §8.1) --------------------------------------

// DataItem is the decoded, validated `verdi.context-data-item/v1` document:
// one provenance-wrapped non-authoritative payload. Path and Ref are nil
// when inapplicable to this item's kind. DataItem is never marshaled
// directly; DecodeDataItem/EncodeDataItem go through a private wire
// document.
type DataItem struct {
	Schema         string
	ID             string
	Source         Source
	Kind           IncludedKind
	Path           *string
	Ref            *string
	Classification string
	ContentDigest  string
	Content        string
	Digest         string
}
