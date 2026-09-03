// The frozen producer is an external test package so that it may build its
// nested fixtures with the private owning codec (internal/sealedexec) that
// will import this publication in Task 2. An external test package may depend
// on a package that depends on the package under test; the publication itself
// still never imports sealedexec, so the one-way dependency holds. The
// publication's own API is dot-imported to keep every assertion written in the
// published names a reviewer compares against the contract.
package contextowner_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	. "github.com/jyang234/verdi/internal/contextowner"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/sealedexec"
)

// Fixture identities. Every digest is a canonical lowercase sha256; every Git
// object id is a full lowercase hexadecimal SHA-1.
const (
	fixtureFlight         = "flight-1"
	fixtureLane           = "lane-1"
	fixtureEpoch          = "epoch-1"
	fixtureSession        = "session-1"
	fixtureRunway         = ".vatc"
	fixtureAdapterVersion = "1.0.0"

	fixtureRequestDigest    = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	fixtureManifestDigest   = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	fixtureProjectionDigest = "sha256:3333333333333333333333333333333333333333333333333333333333333333"
	fixtureAuthorityDigest  = "sha256:4444444444444444444444444444444444444444444444444444444444444444"
	fixtureProfileDigest    = "sha256:5555555555555555555555555555555555555555555555555555555555555555"
	fixtureRecorderDigest   = "sha256:6666666666666666666666666666666666666666666666666666666666666666"
	fixtureEventDigest      = "sha256:7777777777777777777777777777777777777777777777777777777777777777"
	fixtureChildDigest      = "sha256:8888888888888888888888888888888888888888888888888888888888888888"
	fixtureExpansionDigest  = "sha256:9999999999999999999999999999999999999999999999999999999999999999"
	fixtureExpansionRoot    = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	fixtureCheckpointDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	fixtureDispatchDigest   = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	fixtureResultFacts      = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"
	fixtureReceiptDigest    = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	fixtureVerifyDigest     = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	fixturePrincipalDigest  = "sha256:fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	fixtureCommit       = "1234567890123456789012345678901234567890"
	fixtureTree         = "abcdef0123456789abcdef0123456789abcdef01"
	fixtureOutputCommit = "2222222222222222222222222222222222222222"
	fixtureOutputTree   = "bcdef0123456789abcdef0123456789abcdef012"

	fixtureResultEventDigest = "sha256:3333333333333333333333333333333333333333333333333333333333333333"

	fixtureSegmentDigest    = "sha256:ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed"
	fixtureSegmentReference = "controller-segment/sha256/ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed"
)

// nestedDoc returns one exact standalone canonical JSON document with the
// trailing LF trimmed, which is how every published arm carries a nested
// canonical Verdi document.
func nestedDoc(t *testing.T, members map[string]any) json.RawMessage {
	t.Helper()
	encoded, err := canonjson.Marshal(members)
	if err != nil {
		t.Fatalf("canonjson.Marshal nested document: %v", err)
	}
	return json.RawMessage(bytes.TrimSuffix(encoded, []byte("\n")))
}

// standaloneDoc trims the trailing LF from one canonical document its owning
// codec produced, which is how a published arm carries a nested document.
func standaloneDoc(encoded []byte) json.RawMessage {
	return json.RawMessage(bytes.TrimSuffix(encoded, []byte("\n")))
}

// Every nested fixture below is produced by the codec that owns the document,
// so the frozen producer proves the published arms accept exactly what the
// accepted contract emits — never a document that merely declares the right
// schema literal.

func fixtureWorkspaceIdentity(t *testing.T) execworkspace.Identity {
	t.Helper()
	identity, err := sealedexec.NewExecutionWorkspaceRequest(
		fixtureFlight, fixtureLane, fixtureEpoch, fixtureSession, fixtureCommit)
	if err != nil {
		t.Fatalf("NewExecutionWorkspaceRequest: %v", err)
	}
	return identity
}

func fixtureWorkspaceID(t *testing.T) string {
	t.Helper()
	id, err := fixtureWorkspaceIdentity(t).WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID: %v", err)
	}
	return id
}

func fixtureInstructionProjection(t *testing.T) sealedexec.InstructionProjection {
	t.Helper()
	content := "sealed instructions\n"
	projection := sealedexec.InstructionProjection{
		Schema: sealedexec.InstructionProjectionSchemaID,
		Files: []sealedexec.InstructionFile{{
			Path: "AGENTS.md", ContentDigest: fixtureDigestOf([]byte(content)), Content: content,
		}},
	}
	encoded, err := sealedexec.EncodeInstructionProjection(projection)
	if err != nil {
		t.Fatalf("EncodeInstructionProjection: %v", err)
	}
	decoded, err := sealedexec.DecodeInstructionProjection(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeInstructionProjection: %v", err)
	}
	return decoded
}

func fixtureManifest(t *testing.T, projection sealedexec.InstructionProjection) contextcompile.Manifest {
	t.Helper()
	files := make([]contextcompile.ProjectionFileRef, len(projection.Files))
	for i, file := range projection.Files {
		files[i] = contextcompile.ProjectionFileRef{Path: file.Path, Digest: file.ContentDigest}
	}
	var scope policyartifact.Scope
	scopeJSON := []byte(`{"phases":["build"],"environments":["local"],"paths":[".verdi/**"],"refs":["spec/test"]}`)
	if err := json.Unmarshal(scopeJSON, &scope); err != nil {
		t.Fatalf("decode scope fixture: %v", err)
	}
	manifest := contextcompile.Manifest{
		Schema:    contextcompile.ManifestSchema,
		Phase:     contextcompile.PhaseBuild,
		Adapter:   contextcompile.AdapterRef{ID: "codex", Version: "1.0.0"},
		Revisions: contextcompile.Revisions{Authority: fixtureDigestOf([]byte("revision-authority")), Context: 1},
		AcceptedSpec: contextcompile.AcceptedSpec{
			Ref: "spec/test", Path: ".verdi/specs/active/test/spec.md", Blob: fixtureCommit,
			Commit: fixtureCommit, ContentDigest: fixtureDigestOf([]byte("accepted-spec")),
		},
		ParentFeatures: []contextcompile.ParentFeature{},
		Decisions:      []contextcompile.DecisionRef{},
		Obligations:    []contextcompile.Obligation{},
		Repository: contextcompile.RepositoryFacts{
			RemoteOrigin: contextcompile.StringFact{Known: true, Value: "origin"},
			Branch:       contextcompile.StringFact{Known: true, Value: "feature/test"},
			Head:         contextcompile.StringFact{Known: true, Value: fixtureCommit},
			DefaultBranch: contextcompile.DefaultBranchFact{
				Known: true, Name: "main", Ref: "refs/heads/main", Head: fixtureCommit,
			},
			Relationship: contextcompile.RelationshipEqual,
			Dirty:        contextcompile.BoolFact{Known: true, Value: false},
			Staged:       contextcompile.BoolFact{Known: true, Value: false},
			Worktree:     contextcompile.WorktreeFact{Managed: true, Name: "test-worktree"},
			Source:       contextcompile.RepoSourceHead,
			Disclosures:  []contextcompile.DisclosureCode{},
		},
		Policy: contextcompile.PolicySection{
			EffectiveDigest:    fixtureDigestOf([]byte("effective-policy")),
			ConstitutionDigest: fixtureDigestOf([]byte("constitution")),
			ProfileID:          "profile",
			ProfileDigest:      fixtureDigestOf([]byte("policy-profile")),
			Entries:            []contextcompile.PolicyEntry{},
		},
		Owners: []string{"platform-team"},
		Scope:  scope,
		GovernanceProfile: contextcompile.GovernanceProfileRef{
			ID: "profile", Class: gp.ClassSolo, Digest: fixtureDigestOf([]byte("governance-profile")),
		},
		Actors: contextcompile.ActorsSection{
			Posture: contextcompile.ResolutionUnproven, Resolutions: []gp.PrincipalResolution{},
			Disclosures: []contextcompile.DisclosureCode{contextcompile.DisclosureActorResolutionUnproven},
		},
		Included:        []contextcompile.IncludedEntry{},
		Excluded:        []contextcompile.ExcludedEntry{},
		Opaque:          []contextcompile.OpaqueEntry{},
		Capabilities:    execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
		ProjectionFiles: files,
		RequiredInputs:  []contextcompile.RequiredInput{},
		Evidence: contextcompile.EvidenceSection{
			Authority:       contextcompile.EvidenceAuthorityAdvisory,
			Freshness:       contextcompile.EvidenceFreshnessUnknown,
			ConsumedReports: []string{}, Disclosures: []contextcompile.DisclosureCode{},
		},
		Disclosures: []contextcompile.DisclosureCode{contextcompile.DisclosureActorResolutionUnproven},
	}
	encoded, err := contextcompile.EncodeManifest(manifest)
	if err != nil {
		t.Fatalf("EncodeManifest: %v", err)
	}
	decoded, err := contextcompile.DecodeManifest(encoded)
	if err != nil {
		t.Fatalf("DecodeManifest: %v", err)
	}
	return decoded
}

func fixtureAuthorityReport(t *testing.T, manifestDigest string) policyconflict.Report {
	t.Helper()
	report := policyconflict.Report{
		Schema: policyconflict.ReportSchema,
		Input: policyconflict.InputIdentity{
			Target: policyconflict.TargetIdentity{
				Kind:     policyconflict.TargetAcceptedContext,
				Accepted: &policyconflict.AcceptedIdentity{ManifestDigest: manifestDigest},
			},
			ConstitutionDigest:    fixtureDigestOf([]byte("constitution")),
			EffectivePolicyDigest: fixtureDigestOf([]byte("effective-policy")),
			PolicyEntries:         []policyconflict.PolicyEntryIdentity{},
			Profile: policyconflict.ProfileIdentity{
				ID: "profile", Class: string(gp.ClassSolo),
				Digest: fixtureDigestOf([]byte("governance-profile")),
			},
			EvaluatedOn: "2026-08-27",
		},
		Mechanical:  []policyconflict.MechanicalEvaluation{},
		Semantic:    []policyconflict.SemanticEvaluation{},
		Disclosures: []policyconflict.Disclosure{},
		Verdict:     policyconflict.VerdictPass,
	}
	repository := []byte(`{"remote_origin":{"known":true,"value":"origin"},` +
		`"branch":{"known":true,"value":"feature/test"},` +
		`"head":{"known":true,"value":"` + fixtureCommit + `"},` +
		`"default_branch":{"known":true,"name":"main","ref":"refs/heads/main","head":"` + fixtureCommit + `"},` +
		`"relationship":"equal","dirty":{"known":true,"value":false},"staged":{"known":true,"value":false},` +
		`"worktree":{"managed":true,"name":"test-worktree"},"source":"head"}`)
	if err := json.Unmarshal(repository, &report.Input.Repository); err != nil {
		t.Fatalf("decode policy report repository fixture: %v", err)
	}
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}
	decoded, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatalf("DecodeReport: %v", err)
	}
	return decoded
}

// fixtureSealedRequest is the one genuine sealed execution request the
// published arms carry.
func fixtureSealedRequest(t *testing.T) sealedexec.ExecutionRequest {
	t.Helper()
	projection := fixtureInstructionProjection(t)
	manifest := fixtureManifest(t, projection)
	return sealedexec.ExecutionRequest{
		Schema:                    sealedexec.ExecutionRequestSchemaID,
		Action:                    sealedexec.ActionStart,
		Flight:                    fixtureFlight,
		Lane:                      fixtureLane,
		Epoch:                     fixtureEpoch,
		ManifestRevision:          0,
		Session:                   fixtureSession,
		ATCRunway:                 fixtureRunway,
		InputCommit:               fixtureCommit,
		InputTree:                 fixtureTree,
		Manifest:                  manifest,
		ManifestDigest:            manifest.Digest,
		InstructionProjection:     projection,
		ProjectionDigest:          projection.Digest,
		ExecutionWorkspaceRequest: fixtureWorkspaceIdentity(t),
		Adapter:                   contextevent.AdapterCodex,
		AdapterVersion:            fixtureAdapterVersion,
		Profile: sealedexec.LogicalRef{
			Schema: ProfileRefSchemaID, ID: "project", Digest: fixtureProfileDigest,
		},
		Grants:           execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
		AuthorityVerdict: fixtureAuthorityReport(t, manifest.Digest),
		RecorderEndpoint: sealedexec.LogicalRef{
			Schema: RecorderEndpointRefSchemaID, ID: "recorder", Digest: fixtureRecorderDigest,
		},
		Start: &sealedexec.StartArm{ExpectedSourceSequence: 1},
	}
}

func fixtureExecutionRequestDoc(t *testing.T) json.RawMessage {
	t.Helper()
	encoded, err := sealedexec.EncodeExecutionRequest(fixtureSealedRequest(t))
	if err != nil {
		t.Fatalf("EncodeExecutionRequest: %v", err)
	}
	return standaloneDoc(encoded)
}

// fixtureSealedResumeRequest exercises the request's other closed arm: the
// resume checkpoint the published contract must accept unchanged.
func fixtureSealedResumeRequest(t *testing.T) sealedexec.ExecutionRequest {
	t.Helper()
	request := fixtureSealedRequest(t)
	request.Action = sealedexec.ActionResume
	request.Start = nil
	identity := fixtureWorkspaceIdentity(t)
	workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(identity)
	if err != nil {
		t.Fatalf("ExecutionWorkspaceRequestDigest: %v", err)
	}
	grants, err := execworkspace.EncodeGrantSet(request.Grants)
	if err != nil {
		t.Fatalf("EncodeGrantSet: %v", err)
	}
	revision := contextevent.Revision{
		Schema: contextevent.RevisionSchemaID, ManifestRevision: request.ManifestRevision,
		ManifestDigest: request.ManifestDigest, FirstGlobalSequence: 1,
		TerminalGlobalSequence: 3, TerminalSourceSequence: 3,
		TerminalKind: contextevent.KindExecutionResult, EventRoot: fixtureResultEventDigest,
	}
	root, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		t.Fatalf("EventChainRoot: %v", err)
	}
	continuity := sealedexec.ExecutionContinuity{
		Schema: sealedexec.ExecutionContinuitySchemaID, Flight: fixtureFlight, Lane: fixtureLane,
		Epoch: fixtureEpoch, Session: fixtureSession, Adapter: contextevent.AdapterCodex,
		AdapterVersion: fixtureAdapterVersion, ATCRunway: fixtureRunway,
		InputCommit: fixtureCommit, InputTree: fixtureTree,
		CurrentCommit: fixtureOutputCommit, CurrentTree: fixtureOutputTree,
		ExecutionWorkspaceID: fixtureWorkspaceID(t), ExecutionWorkspaceRequestDigest: workspaceDigest,
		ProfileDigest: request.Profile.Digest, GrantDigest: fixtureDigestOf(grants),
		AuthorityVerdictDigest:  request.AuthorityVerdict.Digest,
		CurrentManifestRevision: request.ManifestRevision, CurrentManifestDigest: request.ManifestDigest,
		ProjectionDigest: request.ProjectionDigest, RevisionSegments: []contextevent.Revision{revision},
		EventChainRoot: root, ExpansionLedgerRoot: fixtureExpansionRoot,
		TerminalSourceSequence:   revision.TerminalSourceSequence,
		TerminalGlobalSequence:   revision.TerminalGlobalSequence,
		RecorderCheckpointDigest: fixtureCheckpointDigest, AdapterSessionRef: "codex-session-1",
	}
	encoded, err := sealedexec.EncodeExecutionContinuity(continuity)
	if err != nil {
		t.Fatalf("EncodeExecutionContinuity: %v", err)
	}
	decoded, err := sealedexec.DecodeExecutionContinuity(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeExecutionContinuity: %v", err)
	}
	request.Resume = &sealedexec.ResumeArm{Continuity: decoded, ContinuityDigest: decoded.Digest}
	return request
}

func fixtureResumeRequestDoc(t *testing.T) json.RawMessage {
	t.Helper()
	encoded, err := sealedexec.EncodeExecutionRequest(fixtureSealedResumeRequest(t))
	if err != nil {
		t.Fatalf("EncodeExecutionRequest resume: %v", err)
	}
	return standaloneDoc(encoded)
}

func fixtureGrantsDoc(t *testing.T) json.RawMessage {
	t.Helper()
	encoded, err := execworkspace.EncodeGrantSet(execworkspace.GrantSet{Grants: []execworkspace.Grant{}})
	if err != nil {
		t.Fatalf("EncodeGrantSet: %v", err)
	}
	return standaloneDoc(encoded)
}

func fixtureReportDoc(t *testing.T) json.RawMessage {
	t.Helper()
	report := fixtureAuthorityReport(t, fixtureSealedRequest(t).ManifestDigest)
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport: %v", err)
	}
	return standaloneDoc(encoded)
}

// fixtureAdapterStopEvent is one genuine canonical execution event.
func fixtureAdapterStopEvent(t *testing.T) contextevent.Event {
	t.Helper()
	request := fixtureSealedRequest(t)
	payloadSchema, err := contextevent.PayloadSchema(contextevent.KindAdapterStop)
	if err != nil {
		t.Fatalf("PayloadSchema: %v", err)
	}
	event := contextevent.Event{
		Schema: EventSchemaID, SourceSequence: 1, Flight: fixtureFlight, Lane: fixtureLane,
		Epoch: fixtureEpoch, ManifestRevision: request.ManifestRevision,
		ManifestDigest: request.ManifestDigest, Session: fixtureSession, ATCRunway: fixtureRunway,
		ExecutionWorkspaceID: fixtureWorkspaceID(t), CandidateCommit: fixtureCommit,
		CandidateTree: fixtureTree, Adapter: contextevent.AdapterCodex,
		AdapterVersion: fixtureAdapterVersion, OccurredAt: "2026-08-28T12:34:56Z",
		Kind: contextevent.KindAdapterStop, PayloadSchema: payloadSchema,
		Payload: &contextevent.AdapterStopPayload{
			Schema: payloadSchema, Adapter: contextevent.AdapterCodex,
			AdapterVersion: fixtureAdapterVersion, Session: fixtureSession,
			ExitCode: 0, ReasonCode: "completed",
		},
	}
	encoded, err := contextevent.EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent: %v", err)
	}
	decoded, err := contextevent.DecodeEvent(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEvent: %v", err)
	}
	return decoded
}

func fixtureEventDoc(t *testing.T) json.RawMessage {
	t.Helper()
	encoded, err := contextevent.EncodeEvent(fixtureAdapterStopEvent(t))
	if err != nil {
		t.Fatalf("EncodeEvent: %v", err)
	}
	return standaloneDoc(encoded)
}

func fixtureDataItemDoc(t *testing.T) json.RawMessage {
	t.Helper()
	_, encoded, err := contextcompile.BuildDataItem(contextcompile.Candidate{
		ID: "path:README.md", Source: contextcompile.SourceHeadTree, Path: "README.md",
	}, contextcompile.IncludedRepositoryFile, []byte("repository data\n"))
	if err != nil {
		t.Fatalf("BuildDataItem: %v", err)
	}
	return standaloneDoc(encoded)
}

// fixtureReceiptTriple is one genuine receipt, the receipt event that carries
// its exact canonical bytes, and the acknowledgment that made it durable.
func fixtureReceiptTriple(t *testing.T) (contextreceipt.Receipt, contextevent.Event, contextevent.ReceiptEventAck) {
	t.Helper()
	request := fixtureSealedRequest(t)
	workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		t.Fatalf("ExecutionWorkspaceRequestDigest: %v", err)
	}
	revision := contextevent.Revision{
		Schema: contextevent.RevisionSchemaID, ManifestRevision: request.ManifestRevision,
		ManifestDigest: request.ManifestDigest, FirstGlobalSequence: 1,
		TerminalGlobalSequence: 1, TerminalSourceSequence: 1,
		TerminalKind: contextevent.KindExecutionResult, EventRoot: fixtureResultEventDigest,
	}
	root, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		t.Fatalf("EventChainRoot: %v", err)
	}
	receipt := contextreceipt.Receipt{
		Schema: contextreceipt.SchemaID, Role: contextreceipt.RoleBuilder,
		Authority: contextreceipt.AuthorityAuthoritative, ManifestDigest: request.ManifestDigest,
		DispatchDigest: fixtureDispatchDigest, ATCRunway: fixtureRunway,
		ExecutionWorkspaceRequestDigest: workspaceDigest, ExecutionWorkspaceID: fixtureWorkspaceID(t),
		InputCommit: fixtureCommit, InputTree: fixtureTree,
		OutputCommit: fixtureOutputCommit, OutputTree: fixtureOutputTree, Clean: true,
		RevisionSegments: []contextevent.Revision{revision}, EventChainRoot: root,
		TerminalManifestRevision: request.ManifestRevision, TerminalSourceSequence: 1,
		TerminalGlobalSequence: 1, Expansions: []contextreceipt.Expansion{},
		Obligations: []contextreceipt.Obligation{}, Evidence: []contextreceipt.Evidence{},
		RunnerPrincipalResolution: fixturePrincipal(t), Adapter: contextevent.AdapterCodex,
		AdapterVersion: fixtureAdapterVersion, ReviewInputs: []contextreceipt.ReviewInput{},
	}
	receiptBytes, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		t.Fatalf("EncodeReceipt: %v", err)
	}
	receipt, err = contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		t.Fatalf("DecodeReceipt: %v", err)
	}
	payloadSchema, err := contextevent.PayloadSchema(contextevent.KindReceipt)
	if err != nil {
		t.Fatalf("PayloadSchema: %v", err)
	}
	receiptJSON := bytes.TrimSuffix(receiptBytes, []byte("\n"))
	event := contextevent.Event{
		Schema: EventSchemaID, SourceSequence: 2, Flight: fixtureFlight, Lane: fixtureLane,
		Epoch: fixtureEpoch, ManifestRevision: request.ManifestRevision,
		ManifestDigest: request.ManifestDigest, Session: fixtureSession, ATCRunway: fixtureRunway,
		ExecutionWorkspaceID: fixtureWorkspaceID(t), CandidateCommit: fixtureOutputCommit,
		CandidateTree: fixtureOutputTree, Adapter: contextevent.AdapterCodex,
		AdapterVersion: fixtureAdapterVersion, OccurredAt: "2026-08-28T12:34:57Z",
		Kind: contextevent.KindReceipt, PayloadSchema: payloadSchema,
		Payload: &contextevent.ReceiptPayload{
			Schema: payloadSchema, Role: contextreceipt.RoleBuilder, ReceiptDigest: receipt.Digest,
			Authority: contextreceipt.AuthorityAuthoritative, ExecutionEventChainRoot: receipt.EventChainRoot,
			Detail: contextevent.Detail{
				Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON,
				Digest: fixtureDigestOf(receiptJSON), RedactionProfile: contextevent.RedactionProfileStandard,
				RedactedJSON: receiptJSON,
			},
		},
		PriorEventDigest: fixtureResultEventDigest,
	}
	encoded, err := contextevent.EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent receipt: %v", err)
	}
	event, err = contextevent.DecodeEvent(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEvent receipt: %v", err)
	}
	ack := contextevent.ReceiptEventAck{
		Schema: contextevent.ReceiptAckSchemaID, Flight: event.Flight, Lane: event.Lane,
		Epoch: event.Epoch, Session: event.Session, ManifestRevision: event.ManifestRevision,
		Kind: event.Kind, SourceSequence: event.SourceSequence, EventDigest: event.EventDigest,
		GlobalSequence: 2, ReceiptDigest: receipt.Digest,
	}
	if _, err := contextevent.EncodeReceiptEventAck(ack); err != nil {
		t.Fatalf("EncodeReceiptEventAck: %v", err)
	}
	return receipt, event, ack
}

func fixtureReceiptAppend(t *testing.T) ReceiptAppend {
	t.Helper()
	receipt, event, _ := fixtureReceiptTriple(t)
	receiptBytes, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		t.Fatalf("EncodeReceipt: %v", err)
	}
	eventBytes, err := contextevent.EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent: %v", err)
	}
	return ReceiptAppend{Receipt: standaloneDoc(receiptBytes), Event: standaloneDoc(eventBytes)}
}

func fixtureHandbackRecord(t *testing.T) sealedexec.HandbackRecord {
	t.Helper()
	receipt, _, ack := fixtureReceiptTriple(t)
	return sealedexec.HandbackRecord{
		Schema: sealedexec.ExecutionHandbackSchemaID, Flight: fixtureFlight, Lane: fixtureLane,
		Epoch: fixtureEpoch, Session: fixtureSession, ATCRunway: fixtureRunway,
		WorkspaceID: fixtureWorkspaceID(t),
		Receipt:     sealedexec.DurableReceipt{Digest: receipt.Digest, EventAck: ack},
		Input:       sealedexec.GitIdentity{Commit: fixtureCommit, Tree: fixtureTree},
		Output:      sealedexec.GitIdentity{Commit: fixtureOutputCommit, Tree: fixtureOutputTree},
		PreRunway:   sealedexec.RunwayState{Head: fixtureCommit, Tree: fixtureTree, Clean: true},
		PostRunway:  sealedexec.RunwayState{Head: fixtureOutputCommit, Tree: fixtureOutputTree, Clean: true},
		Disposition: sealedexec.ControlDispositionFastForwarded,
	}
}

// fixtureQuarantineRecord is the incomplete-execution arm: no receipt, no
// output, and nothing preserved, so its carried bytes are non-null and empty.
func fixtureQuarantineRecord(t *testing.T) sealedexec.QuarantineRecord {
	t.Helper()
	return sealedexec.QuarantineRecord{
		Schema: sealedexec.ExecutionQuarantineSchemaID, Flight: fixtureFlight, Lane: fixtureLane,
		Epoch: fixtureEpoch, Session: fixtureSession, ATCRunway: fixtureRunway,
		WorkspaceID: fixtureWorkspaceID(t),
		Receipt:     sealedexec.QuarantineReceipt{State: sealedexec.QuarantineReceiptAbsent},
		Repository: sealedexec.QuarantineRepository{
			Input:  sealedexec.GitIdentity{Commit: fixtureCommit, Tree: fixtureTree},
			Output: sealedexec.QuarantineOutput{State: sealedexec.QuarantineOutputAbsent},
		},
		Observed: sealedexec.QuarantineObservations{
			Runway:         sealedexec.RepoObservation{State: sealedexec.RepositoryUnproven},
			Child:          sealedexec.RepoObservation{State: sealedexec.RepositoryUnproven},
			Descendant:     sealedexec.Proof{State: sealedexec.ProofUnproven, Witnesses: []string{"execution-incomplete"}},
			ProtectedPaths: []string{}, FastForward: sealedexec.FastForwardNotAttempted,
			PostRunway: sealedexec.RepoObservation{State: sealedexec.RepositoryUnproven},
		},
		Reason:    sealedexec.QuarantineExecutionIncomplete,
		Preserved: sealedexec.PreservedExecution{State: sealedexec.PreservedNone},
	}
}

func fixtureAbortRecord(t *testing.T) sealedexec.AbortRecord {
	t.Helper()
	preserved, err := sealedexec.PreservedExecutionForBytes(
		sealedexec.PreservedPartial, []byte("preserved partial execution\n"))
	if err != nil {
		t.Fatalf("PreservedExecutionForBytes: %v", err)
	}
	return sealedexec.AbortRecord{
		Schema: sealedexec.ExecutionAbortSchemaID, Flight: fixtureFlight, Lane: fixtureLane,
		Epoch: fixtureEpoch, Session: fixtureSession, WorkspaceID: fixtureWorkspaceID(t),
		QuarantineDigest: fixtureQuarantineRecordDigest(t),
		OwnerDecision: sealedexec.LogicalRef{
			Schema: "verdi.owner-decision-ref/v1", ID: "owner-decision",
			Digest: fixtureDigestOf([]byte("owner-decision")),
		},
		Preserved:   *preserved.Ref,
		Disposition: sealedexec.ControlDispositionAbortPreserve,
	}
}

func fixtureQuarantineRecordDigest(t *testing.T) string {
	t.Helper()
	encoded, err := sealedexec.EncodeQuarantineRecord(fixtureQuarantineRecord(t))
	if err != nil {
		t.Fatalf("EncodeQuarantineRecord: %v", err)
	}
	record, err := sealedexec.DecodeQuarantineRecord(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeQuarantineRecord: %v", err)
	}
	return record.Digest
}

func fixtureRecordDoc(t *testing.T, schema string) json.RawMessage {
	t.Helper()
	var (
		encoded []byte
		err     error
	)
	switch schema {
	case HandbackRecordSchemaID:
		encoded, err = sealedexec.EncodeHandbackRecord(fixtureHandbackRecord(t))
	case QuarantineRecordSchemaID:
		encoded, err = sealedexec.EncodeQuarantineRecord(fixtureQuarantineRecord(t))
	case AbortRecordSchemaID:
		encoded, err = sealedexec.EncodeAbortRecord(fixtureAbortRecord(t))
	default:
		t.Fatalf("unknown control record schema %q", schema)
	}
	if err != nil {
		t.Fatalf("encode %s: %v", schema, err)
	}
	return standaloneDoc(encoded)
}

func fixtureControlAckDoc(t *testing.T, recordSchema string) json.RawMessage {
	t.Helper()
	record := fixtureRecordDoc(t, recordSchema)
	var (
		digest      string
		disposition sealedexec.ControlDisposition
	)
	switch recordSchema {
	case HandbackRecordSchemaID:
		decoded, err := sealedexec.DecodeHandbackRecord(bytes.NewReader(frameDoc(record)))
		if err != nil {
			t.Fatalf("DecodeHandbackRecord: %v", err)
		}
		digest, disposition = decoded.Digest, sealedexec.ControlDispositionFastForwarded
	case QuarantineRecordSchemaID:
		decoded, err := sealedexec.DecodeQuarantineRecord(bytes.NewReader(frameDoc(record)))
		if err != nil {
			t.Fatalf("DecodeQuarantineRecord: %v", err)
		}
		digest, disposition = decoded.Digest, sealedexec.ControlDispositionQuarantined
	default:
		decoded, err := sealedexec.DecodeAbortRecord(bytes.NewReader(frameDoc(record)))
		if err != nil {
			t.Fatalf("DecodeAbortRecord: %v", err)
		}
		digest, disposition = decoded.Digest, sealedexec.ControlDispositionAbortPreserve
	}
	encoded, err := sealedexec.EncodeControlAck(sealedexec.ControlAck{
		Schema: ControlAckSchemaID, RecordSchema: recordSchema, RecordDigest: digest,
		Flight: fixtureFlight, Lane: fixtureLane, Epoch: fixtureEpoch, Session: fixtureSession,
		WorkspaceID: fixtureWorkspaceID(t), Disposition: disposition, ControllerGlobalSequence: 1,
	})
	if err != nil {
		t.Fatalf("EncodeControlAck: %v", err)
	}
	return standaloneDoc(encoded)
}

// frameDoc re-frames a nested document as one standalone document.
func frameDoc(document json.RawMessage) []byte {
	return append(append([]byte(nil), document...), '\n')
}

func fixtureDigestOf(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func fixtureExecutionKey() ExecutionKey {
	return ExecutionKey{Flight: fixtureFlight, Lane: fixtureLane, Epoch: fixtureEpoch}
}

func fixtureProfileRef() LogicalRef {
	return LogicalRef{Schema: ProfileRefSchemaID, ID: "project", Digest: fixtureProfileDigest}
}

func fixtureRecorderRef() LogicalRef {
	return LogicalRef{Schema: RecorderEndpointRefSchemaID, ID: "recorder", Digest: fixtureRecorderDigest}
}

func fixtureEventAck() contextevent.EventAck {
	return contextevent.EventAck{
		Schema: contextevent.AckSchemaID, Flight: fixtureFlight, Lane: fixtureLane,
		Epoch: fixtureEpoch, Session: fixtureSession, ManifestRevision: 0,
		Kind: contextevent.KindAdapterStart, SourceSequence: 1,
		EventDigest: fixtureEventDigest, GlobalSequence: 1,
	}
}

func fixtureReceiptAck(t *testing.T) contextevent.ReceiptEventAck {
	t.Helper()
	_, _, ack := fixtureReceiptTriple(t)
	return ack
}

func fixtureRevision() contextevent.Revision {
	return contextevent.Revision{
		Schema: contextevent.RevisionSchemaID, ManifestRevision: 0,
		ManifestDigest: fixtureManifestDigest, FirstGlobalSequence: 1,
		TerminalGlobalSequence: 1, TerminalSourceSequence: 1,
		TerminalKind: contextevent.KindExecutionResult, EventRoot: fixtureEventDigest,
	}
}

func fixtureSegment() RedactedSegment {
	return RedactedSegment{
		Schema: RedactedSegmentSchemaID, MediaType: "application/json",
		RedactionProfile: "verdi.redaction/standard-v1", ByteCount: 13,
		Digest: fixtureSegmentDigest, Bytes: []byte(`{"answer":42}`),
	}
}

func fixtureStoredSegment() StoredSegment {
	return StoredSegment{
		Schema: StoredSegmentSchemaID, Reference: fixtureSegmentReference,
		MediaType: "application/json", RedactionProfile: "verdi.redaction/standard-v1",
		ByteCount: 13, Digest: fixtureSegmentDigest,
	}
}

func fixturePrincipal(t *testing.T) gp.PrincipalResolution {
	t.Helper()
	id, err := gp.CanonicalPrincipalID("fixture", "runner-1")
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	return gp.PrincipalResolution{
		Claim:       gp.PrincipalClaim{TrustSource: "fixture", Subject: "runner-1"},
		PrincipalID: id,
		State:       gp.ResolutionAuthenticated,
		Witnesses:   []gp.Witness{{Code: "authenticated", SourceID: "fixture", EvidenceDigest: fixturePrincipalDigest}},
	}
}

func fixtureProvenVerification() Verification {
	return Verification{State: contextcompile.ResolutionProven, Failure: FailureNone, Witnesses: []string{}}
}

func fixtureEventChainRoot(t *testing.T) string {
	t.Helper()
	root, err := contextevent.EventChainRoot([]contextevent.Revision{fixtureRevision()})
	if err != nil {
		t.Fatalf("EventChainRoot: %v", err)
	}
	return root
}

// fixtureCall builds the one valid public call for operation.
func fixtureCall(t *testing.T, operation Operation) Call {
	t.Helper()
	call := Call{Schema: CallSchemaID, Operation: operation, ControllerRequestDigest: fixtureRequestDigest}
	schema := RequestSchema(operation)
	switch operation {
	case OperationVerifyAuthority:
		call.VerifyAuthority = VerifyAuthorityRequest{Schema: schema, Request: fixtureExecutionRequestDoc(t)}
	case OperationResolveProfile:
		call.ResolveProfile = ResolveProfileRequest{Schema: schema, Query: ProfileQuery{
			Ref: fixtureProfileRef(), WorkspacePath: "/tmp/verdi-controller-workspace", Grants: fixtureGrantsDoc(t),
		}}
	case OperationVerifyConflict:
		call.VerifyConflict = VerifyConflictRequest{Schema: schema, Report: fixtureReportDoc(t)}
	case OperationResolveRecorder:
		call.ResolveRecorder = ResolveRecorderRequest{Schema: schema, Ref: fixtureRecorderRef()}
	case OperationRecorderCheckpoint:
		call.RecorderCheckpoint = RecorderCheckpointRequest{Schema: schema, Key: fixtureExecutionKey()}
	case OperationRecorderAppend:
		call.RecorderAppend = RecorderAppendRequest{Schema: schema, Event: fixtureEventDoc(t)}
	case OperationStoreRedactedSegment:
		call.StoreRedactedSegment = StoreRedactedSegmentRequest{Schema: schema, Segment: fixtureSegment()}
	case OperationResolveRedactedSegment:
		call.ResolveRedactedSegment = ResolveRedactedSegmentRequest{Schema: schema, Reference: fixtureSegmentReference}
	case OperationVerifyOpaqueBoundary:
		call.VerifyOpaqueBoundary = VerifyOpaqueBoundaryRequest{Schema: schema, Rows: []contextcompile.OpaqueEntry{}}
	case OperationVerifyProviderSession:
		call.VerifyProviderSession = VerifyProviderSessionRequest{Schema: schema, Check: ProviderSessionCheck{
			SessionRef: "provider-session", AdapterVersion: "1.0.0",
			ProfileDigest: fixtureProfileDigest, WorkspaceID: "workspace-1",
		}}
	case OperationVerifyExpansion:
		call.VerifyExpansion = VerifyExpansionRequest{Schema: schema, Key: fixtureExecutionKey()}
	case OperationStoreAdapterSession:
		call.StoreAdapterSession = StoreAdapterSessionRequest{Schema: schema, Record: SessionRecord{
			Key: fixtureExecutionKey(), SessionRef: "provider-session", AdapterVersion: "1.0.0",
			ProfileDigest: fixtureProfileDigest, WorkspaceID: "workspace-1", LifecycleAck: fixtureEventAck(),
		}}
	case OperationNextStamp:
		call.NextStamp = NextStampRequest{Schema: schema}
	case OperationResolveContext:
		call.ResolveContext = ResolveContextRequest{Schema: schema, Query: ContextQuery{
			Key: fixtureExecutionKey(), Ref: "spec/test#ac-1",
		}}
	case OperationVerifyEpoch:
		call.VerifyEpoch = VerifyEpochRequest{Schema: schema, Check: fixtureEpochCheck(t)}
	case OperationInstallExpansion:
		call.InstallExpansion = InstallExpansionRequest{Schema: schema, Install: ExpansionInstall{
			Key: fixtureExecutionKey(), RequestID: "request-1", ParentRevision: 0,
			ParentManifestDigest: fixtureManifestDigest, ChildRevision: 1,
			ChildManifestDigest: fixtureChildDigest, ExpansionDigest: fixtureExpansionDigest,
			ExpansionRoot: fixtureExpansionRoot, TerminalAck: fixtureEventAck(),
		}}
	case OperationResolveReceiptInputs:
		call.ResolveReceiptInputs = ResolveReceiptInputsRequest{Schema: schema, Query: ReceiptInputsQuery{
			Request: fixtureExecutionRequestDoc(t), WorkspaceID: "workspace-1",
			DispatchDigest: fixtureDispatchDigest, TerminalRevision: 0,
			TerminalSourceSequence: 1, TerminalGlobalSequence: 1,
			EventChainRoot: fixtureEventChainRoot(t), ResultFactsDigest: fixtureResultFacts,
		}}
	case OperationAppendReceipt:
		call.AppendReceipt = AppendReceiptRequest{Schema: schema, Append: fixtureReceiptAppend(t)}
	case OperationResolveReceiptVerificationAuthority:
		call.ResolveReceiptVerificationAuthority = ResolveReceiptVerificationAuthorityRequest{
			Schema: schema, Query: contextreceipt.AuthorityQuery{
				RequestDigest: fixtureVerifyDigest, ReceiptDigest: fixtureReceiptDigest,
				CandidateCommit: fixtureCommit, CandidateTree: fixtureTree,
				ProfileRef:  contextreceipt.ProfileRef{Schema: ProfileRefSchemaID, ID: "project", Digest: fixtureProfileDigest},
				RunnerClaim: gp.PrincipalClaim{TrustSource: "fixture", Subject: "runner-1"},
			},
		}
	case OperationPersistHandback:
		call.PersistHandback = PersistHandbackRequest{Schema: schema, Record: fixtureRecordDoc(t, HandbackRecordSchemaID)}
	case OperationPersistQuarantine:
		call.PersistQuarantine = PersistQuarantineRequest{
			Schema: schema, Record: fixtureRecordDoc(t, QuarantineRecordSchemaID), PreservedBytes: []byte{},
		}
	case OperationPersistAbort:
		call.PersistAbort = PersistAbortRequest{Schema: schema, Record: fixtureRecordDoc(t, AbortRecordSchemaID)}
	default:
		t.Fatalf("unknown fixture operation %q", operation)
	}
	return call
}

func fixtureEpochCheck(t *testing.T) EpochCheck {
	t.Helper()
	return EpochCheck{
		Snapshot: FlightStateSnapshot{
			Request: fixtureExecutionRequestDoc(t), Key: fixtureExecutionKey(),
			WorkspaceID: "workspace-1", CandidateCommit: fixtureCommit, CandidateTree: fixtureTree,
			Revision: 0, ManifestDigest: fixtureManifestDigest, ProjectionDigest: fixtureProjectionDigest,
			ExpansionRoot: fixtureExpansionRoot, NextSourceSequence: 1, PriorEventDigest: "",
			LastGlobalSequence: 0, Invalidated: false,
		},
		Resolution: fixtureContextResolution(t),
	}
}

func fixtureContextResolution(t *testing.T) ContextResolution {
	t.Helper()
	return ContextResolution{
		State: contextcompile.ResolutionProven, Failure: FailureNone, Witnesses: []string{},
		Ref: "spec/test#ac-1", Data: fixtureDataItemDoc(t),
	}
}

// fixtureReply builds the one valid public reply for operation.
func fixtureReply(t *testing.T, operation Operation) Reply {
	t.Helper()
	reply := Reply{Schema: ReplySchemaID, Call: fixtureCall(t, operation)}
	schema := ResultSchema(operation)
	verification := fixtureProvenVerification()
	switch operation {
	case OperationVerifyAuthority:
		reply.VerifyAuthority = VerifyAuthorityResult{Schema: schema, Facts: AuthorityFacts{
			State: verification.State, Failure: verification.Failure, Witnesses: verification.Witnesses,
			ManifestRevision: 0, ManifestDigest: fixtureManifestDigest,
			ProjectionDigest: fixtureProjectionDigest, AuthorityDigest: fixtureAuthorityDigest,
			AcceptedSpecCommit: fixtureCommit,
		}}
	case OperationResolveProfile:
		reply.ResolveProfile = ResolveProfileResult{Schema: schema, Material: ProfileMaterial{
			Ref: fixtureProfileRef(), Name: "project", AbsoluteExecutable: "/usr/local/bin/codex",
			AbsoluteEnvRoot: "/tmp/verdi-env", AbsoluteCodexHome: "/tmp/verdi-env/codex",
			AdapterVersion: "1.0.0", DecoderProfile: "codex-jsonl-v1",
		}}
	case OperationVerifyConflict:
		reply.VerifyConflict = VerifyConflictResult{Schema: schema, Facts: ConflictFacts{
			State: verification.State, Failure: verification.Failure, Witnesses: verification.Witnesses,
			Report: fixtureReportDoc(t),
		}}
	case OperationResolveRecorder:
		reply.ResolveRecorder = ResolveRecorderResult{Schema: schema, Facts: RecorderFacts{
			State: verification.State, Failure: verification.Failure, Witnesses: verification.Witnesses,
			Ref: fixtureRecorderRef(),
		}}
	case OperationRecorderCheckpoint:
		reply.RecorderCheckpoint = RecorderCheckpointResult{Schema: schema, Checkpoint: RecorderCheckpoint{
			State: verification.State, Failure: verification.Failure, Witnesses: verification.Witnesses,
			Digest: fixtureCheckpointDigest, Revisions: []contextevent.Revision{fixtureRevision()},
			EventChainRoot: fixtureEventChainRoot(t), TerminalSourceSequence: 1, TerminalGlobalSequence: 1,
			ActiveRevision: nil,
		}}
	case OperationRecorderAppend:
		reply.RecorderAppend = RecorderAppendResult{Schema: schema, Ack: fixtureEventAck()}
	case OperationStoreRedactedSegment:
		reply.StoreRedactedSegment = StoreRedactedSegmentResult{Schema: schema, Stored: fixtureStoredSegment()}
	case OperationResolveRedactedSegment:
		reply.ResolveRedactedSegment = ResolveRedactedSegmentResult{Schema: schema, Segment: fixtureSegment()}
	case OperationVerifyOpaqueBoundary:
		reply.VerifyOpaqueBoundary = VerifyOpaqueBoundaryResult{Schema: schema, Facts: OpaqueBoundaryFacts{
			State: verification.State, Failure: verification.Failure, Witnesses: verification.Witnesses,
			Rows: []OpaqueIdentity{},
		}}
	case OperationVerifyProviderSession:
		reply.VerifyProviderSession = VerifyProviderSessionResult{Schema: schema, Facts: ProviderSessionFacts{
			State: verification.State, Failure: verification.Failure, Witnesses: verification.Witnesses,
			SessionRef: "provider-session", AdapterVersion: "1.0.0",
			ProfileDigest: fixtureProfileDigest, WorkspaceID: "workspace-1",
		}}
	case OperationVerifyExpansion:
		reply.VerifyExpansion = VerifyExpansionResult{Schema: schema, Facts: ExpansionFacts{
			State: verification.State, Failure: verification.Failure, Witnesses: verification.Witnesses,
			Root: fixtureExpansionRoot,
		}}
	case OperationStoreAdapterSession:
		reply.StoreAdapterSession = StoreAdapterSessionResult{Schema: schema}
	case OperationNextStamp:
		reply.NextStamp = NextStampResult{Schema: schema, Stamp: "2026-08-28T12:34:56.123456789Z"}
	case OperationResolveContext:
		reply.ResolveContext = ResolveContextResult{Schema: schema, Resolution: fixtureContextResolution(t)}
	case OperationVerifyEpoch:
		reply.VerifyEpoch = VerifyEpochResult{Schema: schema, Verification: verification}
	case OperationInstallExpansion:
		reply.InstallExpansion = InstallExpansionResult{Schema: schema}
	case OperationResolveReceiptInputs:
		reply.ResolveReceiptInputs = ResolveReceiptInputsResult{Schema: schema, Inputs: ReceiptInputs{
			Expansions: []contextreceipt.Expansion{}, Obligations: []contextreceipt.Obligation{},
			Evidence: []contextreceipt.Evidence{}, ReviewInputs: []contextreceipt.ReviewInput{},
			RunnerPrincipal: fixturePrincipal(t),
		}}
	case OperationAppendReceipt:
		reply.AppendReceipt = AppendReceiptResult{Schema: schema, Ack: fixtureReceiptAck(t)}
	case OperationResolveReceiptVerificationAuthority:
		reply.ResolveReceiptVerificationAuthority = ResolveReceiptVerificationAuthorityResult{
			Schema: schema, Authority: fixtureReceiptVerificationAuthority(),
		}
	case OperationPersistHandback:
		reply.PersistHandback = PersistHandbackResult{Schema: schema, Ack: fixtureControlAckDoc(t, HandbackRecordSchemaID)}
	case OperationPersistQuarantine:
		reply.PersistQuarantine = PersistQuarantineResult{Schema: schema, Ack: fixtureControlAckDoc(t, QuarantineRecordSchemaID)}
	case OperationPersistAbort:
		reply.PersistAbort = PersistAbortResult{Schema: schema, Ack: fixtureControlAckDoc(t, AbortRecordSchemaID)}
	default:
		t.Fatalf("unknown fixture operation %q", operation)
	}
	return reply
}

func fixtureReceiptVerificationAuthority() ReceiptVerificationAuthority {
	witness := gp.Witness{Code: "authority-unavailable", SourceID: "controller", Detail: "unavailable"}
	return ReceiptVerificationAuthority{
		Profile: contextreceipt.ProfileAuthority{
			State: contextreceipt.StateUnproven, ProfileBytes: []byte{}, Witnesses: []gp.Witness{witness},
		},
		TrustFact: ReceiptVerificationTrustFact{
			SourceID: "fixture", SourceKind: gp.TrustSourceIdentityProvider,
			Subjects: []string{}, Available: false, Valid: false, Reason: "unavailable",
		},
		Isolation: contextreceipt.IsolationAuthority{
			State: contextreceipt.StateUnproven, Witnesses: []gp.Witness{witness},
		},
		Persistence: contextreceipt.PersistenceAuthority{
			State: contextreceipt.StateUnproven, Witnesses: []gp.Witness{witness},
		},
	}
}

// TestContextOwnerContract_Behavioral is the single frozen producer for the
// public owner wire: the closed 22-operation registry, its mechanically
// derived arm schemas, exact canonical call/reply bytes, the reconstruction
// of private request identity that lets the owning codec recompute
// controller_request_digest, and the adverse documents the wire must refuse.
func TestContextOwnerContract_Behavioral(t *testing.T) {
	t.Run("closed registry and derived schemas", func(t *testing.T) {
		want := []Operation{
			"verify-authority", "resolve-profile", "verify-conflict", "resolve-recorder",
			"recorder-checkpoint", "recorder-append", "store-redacted-segment",
			"resolve-redacted-segment", "verify-opaque-boundary", "verify-provider-session",
			"verify-expansion", "store-adapter-session", "next-stamp", "resolve-context",
			"verify-epoch", "install-expansion", "resolve-receipt-inputs", "append-receipt",
			"resolve-receipt-verification-authority", "persist-handback", "persist-quarantine",
			"persist-abort",
		}
		got := Operations()
		if len(got) != 22 {
			t.Fatalf("registry must publish exactly 22 operations, got %d", len(got))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("registry[%d] = %q, want %q", i, got[i], want[i])
			}
			if request := RequestSchema(want[i]); request != "verdi.context-owner/"+string(want[i])+"-request/v1" {
				t.Fatalf("RequestSchema(%q) = %q", want[i], request)
			}
			if result := ResultSchema(want[i]); result != "verdi.context-owner/"+string(want[i])+"-result/v1" {
				t.Fatalf("ResultSchema(%q) = %q", want[i], result)
			}
		}
		// Operation 23 is ATC-owned and never enters the bridge.
		for _, operation := range got {
			if operation == "resolve-claim-mcp" {
				t.Fatal("ATC-owned resolve-claim-mcp must never enter the public owner registry")
			}
		}
		// The registry is a copy: a caller cannot rewrite the closed union.
		got[0] = "mutated"
		if Operations()[0] != want[0] {
			t.Fatal("Operations must return a copy of the closed registry")
		}
	})

	t.Run("every operation round trips", func(t *testing.T) {
		for _, operation := range Operations() {
			t.Run(string(operation), func(t *testing.T) {
				call := fixtureCall(t, operation)
				encoded, err := EncodeCall(call)
				if err != nil {
					t.Fatalf("EncodeCall: %v", err)
				}
				if !bytes.HasSuffix(encoded, []byte("\n")) || bytes.HasSuffix(encoded, []byte("\n\n")) {
					t.Fatalf("call must carry exactly one trailing LF: %q", encoded)
				}
				decoded, err := DecodeCall(bytes.NewReader(encoded))
				if err != nil {
					t.Fatalf("DecodeCall: %v", err)
				}
				reencoded, err := EncodeCall(decoded)
				if err != nil {
					t.Fatalf("re-EncodeCall: %v", err)
				}
				if !bytes.Equal(encoded, reencoded) {
					t.Fatalf("call round trip is not byte-stable:\n got %s\nwant %s", reencoded, encoded)
				}

				reply := fixtureReply(t, operation)
				encodedReply, err := EncodeReply(reply)
				if err != nil {
					t.Fatalf("EncodeReply: %v", err)
				}
				if !bytes.HasSuffix(encodedReply, []byte("\n")) || bytes.HasSuffix(encodedReply, []byte("\n\n")) {
					t.Fatalf("reply must carry exactly one trailing LF: %q", encodedReply)
				}
				decodedReply, err := DecodeReply(bytes.NewReader(encodedReply))
				if err != nil {
					t.Fatalf("DecodeReply: %v", err)
				}
				reencodedReply, err := EncodeReply(decodedReply)
				if err != nil {
					t.Fatalf("re-EncodeReply: %v", err)
				}
				if !bytes.Equal(encodedReply, reencodedReply) {
					t.Fatalf("reply round trip is not byte-stable:\n got %s\nwant %s", reencodedReply, encodedReply)
				}

				// The nested call is byte-for-byte the canonical public call.
				nested, err := EncodeCall(decodedReply.Call)
				if err != nil {
					t.Fatalf("EncodeCall(reply.Call): %v", err)
				}
				if !bytes.Equal(nested, encoded) {
					t.Fatalf("reply.call is not the canonical call:\n got %s\nwant %s", nested, encoded)
				}
			})
		}
	})

	t.Run("frozen owner-group bytes", func(t *testing.T) {
		for _, row := range frozenOwnerGroupCases(t) {
			t.Run(row.group, func(t *testing.T) {
				call, err := EncodeCall(fixtureCall(t, row.operation))
				if err != nil {
					t.Fatalf("EncodeCall: %v", err)
				}
				if string(call) != row.call {
					t.Fatalf("frozen call bytes differ:\n got %s\nwant %s", call, row.call)
				}
				reply, err := EncodeReply(fixtureReply(t, row.operation))
				if err != nil {
					t.Fatalf("EncodeReply: %v", err)
				}
				if string(reply) != row.reply {
					t.Fatalf("frozen reply bytes differ:\n got %s\nwant %s", reply, row.reply)
				}
			})
		}
	})

	t.Run("published request arm reconstructs private request identity", func(t *testing.T) {
		// The publication rule replaces only the top-level schema literal.
		// Substituting the private literal back into the published arm and
		// re-canonicalizing must yield the exact private request payload, which
		// is what lets the owning codec recompute controller_request_digest.
		for _, row := range frozenOwnerGroupCases(t) {
			t.Run(row.group, func(t *testing.T) {
				call := fixtureCall(t, row.operation)
				arm, err := EncodeCall(call)
				if err != nil {
					t.Fatalf("EncodeCall: %v", err)
				}
				var wire struct {
					Request json.RawMessage `json:"request"`
				}
				if err := json.Unmarshal(arm, &wire); err != nil {
					t.Fatalf("read published request arm: %v", err)
				}
				restored := bytes.Replace(wire.Request,
					[]byte(`"schema":"`+RequestSchema(row.operation)+`"`),
					[]byte(`"schema":"verdi.context-controller/`+string(row.operation)+`-request/v1"`), 1)
				canonical, err := canonjson.Marshal(json.RawMessage(restored))
				if err != nil {
					t.Fatalf("canonicalize restored private payload: %v", err)
				}
				if string(canonical) != row.privateRequest {
					t.Fatalf("private request payload differs:\n got %s\nwant %s", canonical, row.privateRequest)
				}
			})
		}
	})

	t.Run("every published request arm is schema-substitutable", func(t *testing.T) {
		// Task 2 rebuilds the private request payload by substituting the
		// private schema literal back into the published arm and recomputing
		// its digest. That is only sound if every arm carries its public
		// literal exactly once, in the canonical "schema":"..." form, and
		// nowhere else in the document.
		for _, operation := range Operations() {
			t.Run(string(operation), func(t *testing.T) {
				encoded, err := EncodeCall(fixtureCall(t, operation))
				if err != nil {
					t.Fatalf("EncodeCall: %v", err)
				}
				var wire struct {
					Request json.RawMessage `json:"request"`
				}
				if err := json.Unmarshal(encoded, &wire); err != nil {
					t.Fatalf("read published request arm: %v", err)
				}
				public := `"schema":"` + RequestSchema(operation) + `"`
				if occurrences := bytes.Count(wire.Request, []byte(public)); occurrences != 1 {
					t.Fatalf("published arm carries its schema literal %d times, want exactly 1: %s",
						occurrences, wire.Request)
				}
				private := `"schema":"verdi.context-controller/` + string(operation) + `-request/v1"`
				restored := bytes.Replace(wire.Request, []byte(public), []byte(private), 1)
				canonical, err := canonjson.Marshal(json.RawMessage(restored))
				if err != nil {
					t.Fatalf("canonicalize restored private payload: %v", err)
				}
				// The reconstruction is canonical, so its digest is stable,
				// and it differs from the published arm only in that literal.
				if !bytes.Equal(bytes.TrimSuffix(canonical, []byte("\n")), restored) {
					t.Fatalf("restored private payload is not canonical:\n got %s\nwant %s", canonical, restored)
				}
				back := bytes.Replace(restored, []byte(private), []byte(public), 1)
				if !bytes.Equal(back, wire.Request) {
					t.Fatalf("schema substitution changed more than the literal:\n got %s\nwant %s",
						back, wire.Request)
				}
			})
		}
	})

	t.Run("adverse documents are refused", func(t *testing.T) {
		for _, row := range adverseCallCases(t) {
			t.Run(row.name, func(t *testing.T) {
				if _, err := DecodeCall(strings.NewReader(row.document)); err == nil {
					t.Fatalf("DecodeCall accepted %s", row.name)
				}
			})
		}
		for _, row := range adverseReplyCases(t) {
			t.Run(row.name, func(t *testing.T) {
				if _, err := DecodeReply(strings.NewReader(row.document)); err == nil {
					t.Fatalf("DecodeReply accepted %s", row.name)
				}
			})
		}
		for _, row := range adverseEncodeCases(t) {
			t.Run(row.name, func(t *testing.T) {
				if err := row.encode(); err == nil {
					t.Fatalf("encode accepted %s", row.name)
				}
			})
		}
	})
}

type frozenCase struct {
	group          string
	operation      Operation
	call           string
	reply          string
	privateRequest string
}

// frozenOwnerGroupCases freezes one complete request/result pair for each of
// the five ratified ATC owner groups.
//
// The envelope of every frozen pair is a literal: the published member names,
// their canonical order, and the derived arm schemas are exactly what §3.3
// publishes, and a reviewer compares them by eye. A nested canonical Verdi
// document is instead pinned to the byte-exact output of the codec that owns
// it, because that codec — not this test — is the authority for those bytes,
// and inlining a multi-kilobyte manifest, receipt, or control record as a
// literal would freeze a copy no reviewer could check against its owner.
func frozenOwnerGroupCases(t *testing.T) []frozenCase {
	t.Helper()
	executionRequest := string(fixtureExecutionRequestDoc(t))
	receiptAppend := fixtureReceiptAppend(t)
	abortRecord := string(fixtureRecordDoc(t, AbortRecordSchemaID))
	abortAck := string(fixtureControlAckDoc(t, AbortRecordSchemaID))
	receiptAck, err := contextevent.EncodeReceiptEventAck(fixtureReceiptAck(t))
	if err != nil {
		t.Fatalf("EncodeReceiptEventAck: %v", err)
	}
	authorityCall := `{"controller_request_digest":"` + fixtureRequestDigest + `",` +
		`"operation":"verify-authority",` +
		`"request":{"request":` + executionRequest + `,` +
		`"schema":"verdi.context-owner/verify-authority-request/v1"},` +
		`"schema":"verdi.context-owner-call/v1"}`
	receiptCall := `{"controller_request_digest":"` + fixtureRequestDigest + `",` +
		`"operation":"append-receipt",` +
		`"request":{"append":{"event":` + string(receiptAppend.Event) + `,` +
		`"receipt":` + string(receiptAppend.Receipt) + `},` +
		`"schema":"verdi.context-owner/append-receipt-request/v1"},` +
		`"schema":"verdi.context-owner-call/v1"}`
	abortCall := `{"controller_request_digest":"` + fixtureRequestDigest + `",` +
		`"operation":"persist-abort",` +
		`"request":{"record":` + abortRecord + `,` +
		`"schema":"verdi.context-owner/persist-abort-request/v1"},` +
		`"schema":"verdi.context-owner-call/v1"}`
	return []frozenCase{
		{
			group:     "authority",
			operation: OperationVerifyAuthority,
			call:      authorityCall + "\n",
			reply: `{"call":` + authorityCall + `,` +
				`"result":{"facts":{"accepted_spec_commit":"` + fixtureCommit + `",` +
				`"authority_digest":"` + fixtureAuthorityDigest + `",` +
				`"failure":"","manifest_digest":"` + fixtureManifestDigest + `","manifest_revision":0,` +
				`"projection_digest":"` + fixtureProjectionDigest + `","state":"proven","witnesses":[]},` +
				`"schema":"verdi.context-owner/verify-authority-result/v1"},` +
				`"schema":"verdi.context-owner-reply/v1"}` + "\n",
			privateRequest: `{"request":` + executionRequest + `,` +
				`"schema":"verdi.context-controller/verify-authority-request/v1"}` + "\n",
		},
		{
			group:     "recorder",
			operation: OperationNextStamp,
			call: `{"controller_request_digest":"` + fixtureRequestDigest + `",` +
				`"operation":"next-stamp",` +
				`"request":{"schema":"verdi.context-owner/next-stamp-request/v1"},` +
				`"schema":"verdi.context-owner-call/v1"}` + "\n",
			reply: `{"call":{"controller_request_digest":"` + fixtureRequestDigest + `",` +
				`"operation":"next-stamp",` +
				`"request":{"schema":"verdi.context-owner/next-stamp-request/v1"},` +
				`"schema":"verdi.context-owner-call/v1"},` +
				`"result":{"schema":"verdi.context-owner/next-stamp-result/v1","stamp":"2026-08-28T12:34:56.123456789Z"},` +
				`"schema":"verdi.context-owner-reply/v1"}` + "\n",
			privateRequest: `{"schema":"verdi.context-controller/next-stamp-request/v1"}` + "\n",
		},
		{
			group:     "segments",
			operation: OperationResolveRedactedSegment,
			call: `{"controller_request_digest":"` + fixtureRequestDigest + `",` +
				`"operation":"resolve-redacted-segment",` +
				`"request":{"reference":"` + fixtureSegmentReference + `",` +
				`"schema":"verdi.context-owner/resolve-redacted-segment-request/v1"},` +
				`"schema":"verdi.context-owner-call/v1"}` + "\n",
			reply: `{"call":{"controller_request_digest":"` + fixtureRequestDigest + `",` +
				`"operation":"resolve-redacted-segment",` +
				`"request":{"reference":"` + fixtureSegmentReference + `",` +
				`"schema":"verdi.context-owner/resolve-redacted-segment-request/v1"},` +
				`"schema":"verdi.context-owner-call/v1"},` +
				`"result":{"schema":"verdi.context-owner/resolve-redacted-segment-result/v1",` +
				`"segment":{"byte_count":13,"bytes":"eyJhbnN3ZXIiOjQyfQ==","digest":"` + fixtureSegmentDigest + `",` +
				`"media_type":"application/json","redaction_profile":"verdi.redaction/standard-v1",` +
				`"schema":"verdi.context-redacted-segment/v1"}},` +
				`"schema":"verdi.context-owner-reply/v1"}` + "\n",
			privateRequest: `{"reference":"` + fixtureSegmentReference + `",` +
				`"schema":"verdi.context-controller/resolve-redacted-segment-request/v1"}` + "\n",
		},
		{
			group:     "receipts",
			operation: OperationAppendReceipt,
			call:      receiptCall + "\n",
			reply: `{"call":` + receiptCall + `,` +
				`"result":{"ack":` + string(bytes.TrimSuffix(receiptAck, []byte("\n"))) + `,` +
				`"schema":"verdi.context-owner/append-receipt-result/v1"},` +
				`"schema":"verdi.context-owner-reply/v1"}` + "\n",
			privateRequest: `{"append":{"event":` + string(receiptAppend.Event) + `,` +
				`"receipt":` + string(receiptAppend.Receipt) + `},` +
				`"schema":"verdi.context-controller/append-receipt-request/v1"}` + "\n",
		},
		{
			group:     "control",
			operation: OperationPersistAbort,
			call:      abortCall + "\n",
			reply: `{"call":` + abortCall + `,` +
				`"result":{"ack":` + abortAck + `,` +
				`"schema":"verdi.context-owner/persist-abort-result/v1"},` +
				`"schema":"verdi.context-owner-reply/v1"}` + "\n",
			privateRequest: `{"record":` + abortRecord + `,` +
				`"schema":"verdi.context-controller/persist-abort-request/v1"}` + "\n",
		},
	}
}

type adverseCase struct {
	name     string
	document string
}

// validCallDocument returns the canonical call for operation as a string, the
// starting point every adverse mutation departs from.
func validCallDocument(t *testing.T, operation Operation) string {
	t.Helper()
	encoded, err := EncodeCall(fixtureCall(t, operation))
	if err != nil {
		t.Fatalf("EncodeCall: %v", err)
	}
	return string(encoded)
}

func validReplyDocument(t *testing.T, operation Operation) string {
	t.Helper()
	encoded, err := EncodeReply(fixtureReply(t, operation))
	if err != nil {
		t.Fatalf("EncodeReply: %v", err)
	}
	return string(encoded)
}

func adverseCallCases(t *testing.T) []adverseCase {
	t.Helper()
	valid := validCallDocument(t, OperationNextStamp)
	rows := []adverseCase{
		{"empty document", ""},
		{"null document", "null\n"},
		{"top-level array", "[]\n"},
		{"unknown top-level field", strings.Replace(valid, `"schema":"verdi.context-owner-call/v1"}`,
			`"schema":"verdi.context-owner-call/v1","extra":1}`, 1)},
		{"duplicate top-level field", strings.Replace(valid, `"operation":"next-stamp"`,
			`"operation":"next-stamp","operation":"next-stamp"`, 1)},
		{"trailing data", strings.TrimSuffix(valid, "\n") + "{}\n"},
		{"missing trailing LF", strings.TrimSuffix(valid, "\n")},
		{"doubled trailing LF", valid + "\n"},
		{"wrong top-level schema", strings.Replace(valid, `"schema":"verdi.context-owner-call/v1"`,
			`"schema":"verdi.context-owner-call/v2"`, 1)},
		{"reply schema on a call", strings.Replace(valid, `"schema":"verdi.context-owner-call/v1"`,
			`"schema":"verdi.context-owner-reply/v1"`, 1)},
		{"null request arm", strings.Replace(valid,
			`"request":{"schema":"verdi.context-owner/next-stamp-request/v1"}`, `"request":null`, 1)},
		{"absent request arm", strings.Replace(valid,
			`"request":{"schema":"verdi.context-owner/next-stamp-request/v1"},`, "", 1)},
		{"absent controller_request_digest", strings.Replace(valid,
			`"controller_request_digest":"`+fixtureRequestDigest+`",`, "", 1)},
		{"null controller_request_digest", strings.Replace(valid,
			`"controller_request_digest":"`+fixtureRequestDigest+`"`, `"controller_request_digest":null`, 1)},
		{"uppercase digest", strings.Replace(valid, fixtureRequestDigest,
			"sha256:1111111111111111111111111111111111111111111111111111111111111AAA", 1)},
		{"short digest", strings.Replace(valid, fixtureRequestDigest, "sha256:1111", 1)},
		{"unprefixed digest", strings.Replace(valid, fixtureRequestDigest,
			"1111111111111111111111111111111111111111111111111111111111111111", 1)},
		{"unknown operation", strings.Replace(valid, `"operation":"next-stamp"`,
			`"operation":"resolve-claim-mcp"`, 1)},
		{"empty operation", strings.Replace(valid, `"operation":"next-stamp"`, `"operation":""`, 1)},
		{"private arm schema", strings.Replace(valid, `"schema":"verdi.context-owner/next-stamp-request/v1"`,
			`"schema":"verdi.context-controller/next-stamp-request/v1"`, 1)},
		{"arm schema for another operation", strings.Replace(valid,
			`"schema":"verdi.context-owner/next-stamp-request/v1"`,
			`"schema":"verdi.context-owner/verify-expansion-request/v1"`, 1)},
		{"noncanonical member order", `{"schema":"verdi.context-owner-call/v1",` +
			`"operation":"next-stamp","controller_request_digest":"` + fixtureRequestDigest + `",` +
			`"request":{"schema":"verdi.context-owner/next-stamp-request/v1"}}` + "\n"},
		{"noncanonical whitespace", strings.Replace(valid, `{"controller_request_digest"`,
			`{ "controller_request_digest"`, 1)},
	}

	// Operation/arm mismatch: a valid verify-expansion arm under another
	// operation name.
	expansion := validCallDocument(t, OperationVerifyExpansion)
	rows = append(rows, adverseCase{"operation renamed over a foreign arm",
		strings.Replace(expansion, `"operation":"verify-expansion"`, `"operation":"recorder-checkpoint"`, 1)})

	// Nested identity mutations inside a published arm.
	segment := validCallDocument(t, OperationStoreRedactedSegment)
	rows = append(rows,
		adverseCase{"segment digest does not authenticate bytes",
			strings.Replace(segment, fixtureSegmentDigest, fixtureManifestDigest, 1)},
		adverseCase{"segment byte_count contradicts bytes",
			strings.Replace(segment, `"byte_count":13`, `"byte_count":12`, 1)},
		adverseCase{"unknown segment member",
			strings.Replace(segment, `"byte_count":13`, `"byte_count":13,"aad":"x"`, 1)},
	)

	// The reference is derived from its own digest, so only a reference that
	// is not a canonical digest address can contradict the rule.
	reference := validCallDocument(t, OperationResolveRedactedSegment)
	rows = append(rows,
		adverseCase{"segment reference is not a canonical digest address",
			strings.Replace(reference, "controller-segment/sha256/ecf59a", "controller-segment/sha256/ECF59A", 1)},
		adverseCase{"segment reference carries a foreign prefix",
			strings.Replace(reference, "controller-segment/sha256/", "controller-segment/sha512/", 1)},
	)

	install := validCallDocument(t, OperationInstallExpansion)
	rows = append(rows,
		adverseCase{"child revision does not follow parent",
			strings.Replace(install, `"child_revision":1`, `"child_revision":3`, 1)},
		adverseCase{"expansion root is not a canonical digest",
			strings.Replace(install, `"expansion_root":"`+fixtureExpansionRoot+`"`, `"expansion_root":"root"`, 1)},
	)

	profile := validCallDocument(t, OperationResolveProfile)
	rows = append(rows,
		adverseCase{"relative profile workspace path",
			strings.Replace(profile, `"workspace_path":"/tmp/verdi-controller-workspace"`,
				`"workspace_path":"verdi-controller-workspace"`, 1)},
		adverseCase{"profile ref carries the recorder schema",
			strings.Replace(profile, `"schema":"`+ProfileRefSchemaID+`"`,
				`"schema":"`+RecorderEndpointRefSchemaID+`"`, 1)},
	)

	authority := validCallDocument(t, OperationResolveReceiptVerificationAuthority)
	rows = append(rows, adverseCase{"receipt verification candidate commit is not a Git object",
		strings.Replace(authority, `"candidate_commit":"`+fixtureCommit+`"`, `"candidate_commit":"HEAD"`, 1)})

	return rows
}

func adverseReplyCases(t *testing.T) []adverseCase {
	t.Helper()
	valid := validReplyDocument(t, OperationNextStamp)
	rows := []adverseCase{
		{"empty reply", ""},
		{"null reply", "null\n"},
		{"unknown top-level field", strings.Replace(valid, `"schema":"verdi.context-owner-reply/v1"}`,
			`"schema":"verdi.context-owner-reply/v1","extra":1}`, 1)},
		{"duplicate top-level field", strings.Replace(valid, `"schema":"verdi.context-owner-reply/v1"}`,
			`"schema":"verdi.context-owner-reply/v1","schema":"verdi.context-owner-reply/v1"}`, 1)},
		{"trailing data", strings.TrimSuffix(valid, "\n") + "[]\n"},
		{"call schema on a reply", strings.Replace(valid, `"schema":"verdi.context-owner-reply/v1"}`,
			`"schema":"verdi.context-owner-call/v1"}`, 1)},
		{"absent call", strings.Replace(valid, `"call":{`, `"absent":{`, 1)},
		{"null result arm", strings.Replace(valid,
			`"result":{"schema":"verdi.context-owner/next-stamp-result/v1","stamp":"2026-08-28T12:34:56.123456789Z"}`,
			`"result":null`, 1)},
		{"private result arm schema", strings.Replace(valid,
			`"schema":"verdi.context-owner/next-stamp-result/v1"`,
			`"schema":"verdi.context-controller/next-stamp-result/v1"`, 1)},
		{"request arm schema in the result", strings.Replace(valid,
			`"schema":"verdi.context-owner/next-stamp-result/v1"`,
			`"schema":"verdi.context-owner/next-stamp-request/v1"`, 1)},
		{"stamp is not normalized UTC", strings.Replace(valid, `"stamp":"2026-08-28T12:34:56.123456789Z"`,
			`"stamp":"2026-08-28T12:34:56.123456789+00:00"`, 1)},
		{"noncanonical reply order", `{"schema":"verdi.context-owner-reply/v1",` +
			strings.TrimPrefix(strings.TrimSuffix(valid, `,"schema":"verdi.context-owner-reply/v1"}`+"\n"), "{") + "}\n"},
	}

	// A result whose operation disagrees with its own call is refused even
	// when both name a real operation.
	mismatched := strings.Replace(validReplyDocument(t, OperationVerifyExpansion),
		`"schema":"verdi.context-owner/verify-expansion-result/v1"`,
		`"schema":"verdi.context-owner/verify-authority-result/v1"`, 1)
	rows = append(rows, adverseCase{"result arm belongs to another operation", mismatched})

	// The nested call must itself be a canonical public call.
	rows = append(rows, adverseCase{"nested call carries a malformed digest",
		strings.Replace(validReplyDocument(t, OperationNextStamp), fixtureRequestDigest,
			"sha256:not-a-digest", 1)})

	authority := validReplyDocument(t, OperationVerifyAuthority)
	rows = append(rows,
		adverseCase{"proven authority facts carry a failure code",
			strings.Replace(authority, `"failure":""`, `"failure":"mismatch"`, 1)},
		adverseCase{"authority facts omit the accepted spec commit",
			strings.Replace(authority, `"accepted_spec_commit":"`+fixtureCommit+`",`, "", 1)},
		adverseCase{"authority facts rename a published member",
			strings.Replace(authority, `"manifest_digest"`, `"manifestDigest"`, 1)},
	)

	expansion := validReplyDocument(t, OperationVerifyExpansion)
	rows = append(rows, adverseCase{"non-proven expansion facts carry an installed root",
		strings.Replace(strings.Replace(expansion, `"state":"proven"`, `"state":"unproven"`, 1),
			`"failure":"","witnesses":[]`, `"failure":"unproven","witnesses":["absent"]`, 1)})

	checkpoint := validReplyDocument(t, OperationRecorderCheckpoint)
	rows = append(rows,
		adverseCase{"recorder checkpoint omits active_revision",
			strings.Replace(checkpoint, `"active_revision":null,`, "", 1)},
		adverseCase{"recorder checkpoint terminal facts contradict its revisions",
			strings.Replace(checkpoint, `"terminal_global_sequence":1,"terminal_source_sequence":1`,
				`"terminal_global_sequence":2,"terminal_source_sequence":1`, 1)},
	)

	material := validReplyDocument(t, OperationResolveProfile)
	rows = append(rows,
		adverseCase{"profile material selects both provider arms",
			strings.Replace(material, `"adapter_version":"1.0.0"`,
				`"adapter_version":"1.0.0","model":"claude-opus-5"`, 1)},
		adverseCase{"profile material executable is relative",
			strings.Replace(material, `"absolute_executable":"/usr/local/bin/codex"`,
				`"absolute_executable":"codex"`, 1)},
	)

	inputs := validReplyDocument(t, OperationResolveReceiptInputs)
	rows = append(rows,
		adverseCase{"receipt inputs omit a published array",
			strings.Replace(inputs, `"review_inputs":[],`, "", 1)},
		adverseCase{"receipt inputs carry a null array",
			strings.Replace(inputs, `"obligations":[]`, `"obligations":null`, 1)},
		adverseCase{"authenticated runner principal has no principal id",
			strings.Replace(inputs, `"principal_id":"`, `"unused":"`, 1)},
	)

	verification := validReplyDocument(t, OperationVerifyEpoch)
	rows = append(rows, adverseCase{"non-proven verification has no witnesses",
		strings.Replace(strings.Replace(verification, `"state":"proven"`, `"state":"unproven"`, 1),
			`"failure":""`, `"failure":"unproven"`, 1)})

	return rows
}

type adverseEncodeCase struct {
	name   string
	encode func() error
}

func adverseEncodeCases(t *testing.T) []adverseEncodeCase {
	t.Helper()
	rows := []adverseEncodeCase{
		{"zero arms", func() error {
			call := Call{Schema: CallSchemaID, Operation: OperationNextStamp, ControllerRequestDigest: fixtureRequestDigest}
			_, err := EncodeCall(call)
			return err
		}},
		{"two arms", func() error {
			call := fixtureCall(t, OperationNextStamp)
			call.VerifyExpansion = VerifyExpansionRequest{
				Schema: RequestSchema(OperationVerifyExpansion), Key: fixtureExecutionKey(),
			}
			_, err := EncodeCall(call)
			return err
		}},
		{"arm belongs to another operation", func() error {
			call := fixtureCall(t, OperationVerifyExpansion)
			call.Operation = OperationRecorderCheckpoint
			_, err := EncodeCall(call)
			return err
		}},
		{"unknown operation", func() error {
			call := fixtureCall(t, OperationNextStamp)
			call.Operation = "resolve-claim-mcp"
			_, err := EncodeCall(call)
			return err
		}},
		{"malformed controller_request_digest", func() error {
			call := fixtureCall(t, OperationNextStamp)
			call.ControllerRequestDigest = "sha256:NOTHEX"
			_, err := EncodeCall(call)
			return err
		}},
		{"absent controller_request_digest", func() error {
			call := fixtureCall(t, OperationNextStamp)
			call.ControllerRequestDigest = ""
			_, err := EncodeCall(call)
			return err
		}},
		{"wrong call schema", func() error {
			call := fixtureCall(t, OperationNextStamp)
			call.Schema = ReplySchemaID
			_, err := EncodeCall(call)
			return err
		}},
		{"private arm schema", func() error {
			call := fixtureCall(t, OperationNextStamp)
			call.NextStamp.Schema = "verdi.context-controller/next-stamp-request/v1"
			_, err := EncodeCall(call)
			return err
		}},
		{"nested document is not canonical", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = json.RawMessage(
				`{"schema":"verdi.context-execution-request/v1", "flight":"flight-1"}`)
			_, err := EncodeCall(call)
			return err
		}},
		{"nested document carries the wrong schema", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = nestedDoc(t, map[string]any{"schema": EventSchemaID})
			_, err := EncodeCall(call)
			return err
		}},
		{"nested document is null", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = json.RawMessage("null")
			_, err := EncodeCall(call)
			return err
		}},
		{"nested document is absent", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = nil
			_, err := EncodeCall(call)
			return err
		}},
		{"reply schema is a call schema", func() error {
			reply := fixtureReply(t, OperationNextStamp)
			reply.Schema = CallSchemaID
			_, err := EncodeReply(reply)
			return err
		}},
		{"reply result arm is empty", func() error {
			reply := fixtureReply(t, OperationNextStamp)
			reply.NextStamp = NextStampResult{}
			_, err := EncodeReply(reply)
			return err
		}},
		{"reply carries two result arms", func() error {
			reply := fixtureReply(t, OperationNextStamp)
			reply.InstallExpansion = InstallExpansionResult{Schema: ResultSchema(OperationInstallExpansion)}
			_, err := EncodeReply(reply)
			return err
		}},
		{"reply result belongs to another call", func() error {
			reply := fixtureReply(t, OperationNextStamp)
			reply.Call = fixtureCall(t, OperationInstallExpansion)
			_, err := EncodeReply(reply)
			return err
		}},
		{"reply carries an invalid nested call", func() error {
			reply := fixtureReply(t, OperationNextStamp)
			reply.Call.ControllerRequestDigest = "sha256:" + strings.Repeat("z", 64)
			_, err := EncodeReply(reply)
			return err
		}},
		{"unsorted verification witnesses", func() error {
			reply := fixtureReply(t, OperationVerifyEpoch)
			reply.VerifyEpoch.Verification = Verification{
				State: contextcompile.ResolutionUnproven, Failure: FailureUnproven,
				Witnesses: []string{"b", "a"},
			}
			_, err := EncodeReply(reply)
			return err
		}},
		{"null verification witnesses", func() error {
			reply := fixtureReply(t, OperationVerifyEpoch)
			reply.VerifyEpoch.Verification = Verification{
				State: contextcompile.ResolutionProven, Failure: FailureNone, Witnesses: nil,
			}
			_, err := EncodeReply(reply)
			return err
		}},
		{"unknown verification state", func() error {
			reply := fixtureReply(t, OperationVerifyEpoch)
			reply.VerifyEpoch.Verification = Verification{
				State: "maybe", Failure: FailureNone, Witnesses: []string{},
			}
			_, err := EncodeReply(reply)
			return err
		}},
		{"opaque rows are unsorted", func() error {
			call := fixtureCall(t, OperationVerifyOpaqueBoundary)
			call.VerifyOpaqueBoundary.Rows = []contextcompile.OpaqueEntry{
				{ID: "b", Kind: contextcompile.OpaqueKindHarnessVendorBase,
					Adapter:     contextcompile.AdapterRef{ID: "codex", Version: "1.0.0"},
					Disclosures: []contextcompile.DisclosureCode{contextcompile.DisclosureOpaqueHarnessVendorBase}},
				{ID: "a", Kind: contextcompile.OpaqueKindHarnessVendorBase,
					Adapter:     contextcompile.AdapterRef{ID: "codex", Version: "1.0.0"},
					Disclosures: []contextcompile.DisclosureCode{contextcompile.DisclosureOpaqueHarnessVendorBase}},
			}
			_, err := EncodeCall(call)
			return err
		}},
		{"opaque rows are null", func() error {
			call := fixtureCall(t, OperationVerifyOpaqueBoundary)
			call.VerifyOpaqueBoundary.Rows = nil
			_, err := EncodeCall(call)
			return err
		}},
		{"quarantine preserved bytes are null", func() error {
			call := fixtureCall(t, OperationPersistQuarantine)
			call.PersistQuarantine.PreservedBytes = nil
			_, err := EncodeCall(call)
			return err
		}},
	}
	return rows
}

// TestContextOwnerNestedInterior_Adverse proves the mechanically published
// arms retain the nested value rules and cross-field validation of the
// accepted private payload (correction §3.3 step 3). A nested document that
// only looks like its schema — canonical object bytes declaring the right
// schema literal — is not an instance of that schema, and the published arm
// must refuse it exactly as the owning private codec does. Each row is an
// operand the private controller codec rejects; accepting any of them would
// let DecodeCall/DecodeReply admit content no owner could act on.
func TestContextOwnerNestedInterior_Adverse(t *testing.T) {
	rows := []struct {
		name   string
		encode func() error
	}{
		{"verify-authority request is not an execution request", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = skeletalDoc(t, ExecutionRequestSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"verify-conflict report is not a policy-conflict report", func() error {
			call := fixtureCall(t, OperationVerifyConflict)
			call.VerifyConflict.Report = skeletalDoc(t, PolicyConflictReportSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"profile query grants are not a grant set", func() error {
			call := fixtureCall(t, OperationResolveProfile)
			call.ResolveProfile.Query.Grants = nestedDoc(t, map[string]any{"not_a_grant_set": true})
			_, err := EncodeCall(call)
			return err
		}},
		{"recorder-append event is not an execution event", func() error {
			call := fixtureCall(t, OperationRecorderAppend)
			call.RecorderAppend.Event = skeletalDoc(t, EventSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"epoch snapshot request is not an execution request", func() error {
			call := fixtureCall(t, OperationVerifyEpoch)
			call.VerifyEpoch.Check.Snapshot.Request = skeletalDoc(t, ExecutionRequestSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"epoch resolution data is not a data item", func() error {
			call := fixtureCall(t, OperationVerifyEpoch)
			call.VerifyEpoch.Check.Resolution.Data = skeletalDoc(t, DataItemSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"receipt inputs request is not an execution request", func() error {
			call := fixtureCall(t, OperationResolveReceiptInputs)
			call.ResolveReceiptInputs.Query.Request = skeletalDoc(t, ExecutionRequestSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"append-receipt receipt is not a receipt", func() error {
			call := fixtureCall(t, OperationAppendReceipt)
			call.AppendReceipt.Append.Receipt = skeletalDoc(t, ReceiptSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"append-receipt event is not an execution event", func() error {
			call := fixtureCall(t, OperationAppendReceipt)
			call.AppendReceipt.Append.Event = skeletalDoc(t, EventSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"persist-handback record is not a handback record", func() error {
			call := fixtureCall(t, OperationPersistHandback)
			call.PersistHandback.Record = skeletalDoc(t, HandbackRecordSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"persist-quarantine record is not a quarantine record", func() error {
			call := fixtureCall(t, OperationPersistQuarantine)
			call.PersistQuarantine.Record = skeletalDoc(t, QuarantineRecordSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"persist-abort record is not an abort record", func() error {
			call := fixtureCall(t, OperationPersistAbort)
			call.PersistAbort.Record = skeletalDoc(t, AbortRecordSchemaID)
			_, err := EncodeCall(call)
			return err
		}},
		{"conflict facts report is not a policy-conflict report", func() error {
			reply := fixtureReply(t, OperationVerifyConflict)
			reply.VerifyConflict.Facts.Report = skeletalDoc(t, PolicyConflictReportSchemaID)
			_, err := EncodeReply(reply)
			return err
		}},
		{"context resolution data is not a data item", func() error {
			reply := fixtureReply(t, OperationResolveContext)
			reply.ResolveContext.Resolution.Data = skeletalDoc(t, DataItemSchemaID)
			_, err := EncodeReply(reply)
			return err
		}},
		{"persist-handback ack is not a control ack", func() error {
			reply := fixtureReply(t, OperationPersistHandback)
			reply.PersistHandback.Ack = skeletalDoc(t, ControlAckSchemaID)
			_, err := EncodeReply(reply)
			return err
		}},
		{"persist-quarantine ack is not a control ack", func() error {
			reply := fixtureReply(t, OperationPersistQuarantine)
			reply.PersistQuarantine.Ack = skeletalDoc(t, ControlAckSchemaID)
			_, err := EncodeReply(reply)
			return err
		}},
		{"persist-abort ack is not a control ack", func() error {
			reply := fixtureReply(t, OperationPersistAbort)
			reply.PersistAbort.Ack = skeletalDoc(t, ControlAckSchemaID)
			_, err := EncodeReply(reply)
			return err
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			if err := row.encode(); err == nil {
				t.Fatalf("published arm accepted %s", row.name)
			}
		})
	}
}

// skeletalDoc is a canonical JSON object that declares schema and nothing
// else: schema-shaped, never a valid instance of that schema.
func skeletalDoc(t *testing.T, schema string) json.RawMessage {
	t.Helper()
	return nestedDoc(t, map[string]any{"schema": schema})
}

// TestContextOwnerNestedCrossField_Adverse proves the published arms retain
// the accepted cross-field validation, not only the member shapes. Every row
// starts from a document the owning codec produced and changes exactly one
// fact so that the document contradicts itself; for a self-digested record the
// digest is recomputed, so the row witnesses the semantic rule rather than the
// digest that would otherwise mask it.
func TestContextOwnerNestedCrossField_Adverse(t *testing.T) {
	// Positive control: resealing without a semantic change must still be
	// accepted, so every adverse row below fails on the rule it names rather
	// than on a digest the helper happened to break.
	t.Run("reseal without a semantic change is accepted", func(t *testing.T) {
		for _, operation := range []Operation{
			OperationPersistHandback, OperationPersistQuarantine, OperationPersistAbort,
		} {
			call := fixtureCall(t, operation)
			record := recordArm(t, &call, operation)
			*record = resealedDoc(t, *record, func(map[string]any) {})
			if _, err := EncodeCall(call); err != nil {
				t.Fatalf("resealed %s record was refused: %v", operation, err)
			}
		}
	})

	rows := []struct {
		name   string
		encode func() error
	}{
		{"execution request manifest_digest contradicts its manifest", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = revisedDoc(t, call.VerifyAuthority.Request, func(members map[string]any) {
				members["manifest_digest"] = fixtureAuthorityDigest
			})
			_, err := EncodeCall(call)
			return err
		}},
		{"execution request adapter_version contradicts its manifest", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = revisedDoc(t, call.VerifyAuthority.Request, func(members map[string]any) {
				members["adapter_version"] = "9.9.9"
			})
			_, err := EncodeCall(call)
			return err
		}},
		{"execution request resume action carries the start arm", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = revisedDoc(t, call.VerifyAuthority.Request, func(members map[string]any) {
				members["action"] = "resume"
			})
			_, err := EncodeCall(call)
			return err
		}},
		{"execution request workspace request contradicts the dispatch tuple", func() error {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = revisedDoc(t, call.VerifyAuthority.Request, func(members map[string]any) {
				members["session"] = "session-2"
			})
			_, err := EncodeCall(call)
			return err
		}},
		{"append-receipt event is not the receipt's own event", func() error {
			call := fixtureCall(t, OperationAppendReceipt)
			call.AppendReceipt.Append.Event = fixtureEventDoc(t)
			_, err := EncodeCall(call)
			return err
		}},
		{"quarantine preserves nothing but carries preserved bytes", func() error {
			call := fixtureCall(t, OperationPersistQuarantine)
			call.PersistQuarantine.PreservedBytes = []byte("preserved execution\n")
			_, err := EncodeCall(call)
			return err
		}},
		{"quarantine reason contradicts its observations", func() error {
			call := fixtureCall(t, OperationPersistQuarantine)
			call.PersistQuarantine.Record = resealedDoc(t, call.PersistQuarantine.Record, func(members map[string]any) {
				members["reason"] = "fast-forward-failed"
			})
			_, err := EncodeCall(call)
			return err
		}},
		{"handback pre_runway does not equal its input", func() error {
			call := fixtureCall(t, OperationPersistHandback)
			call.PersistHandback.Record = resealedDoc(t, call.PersistHandback.Record, func(members map[string]any) {
				members["pre_runway"].(map[string]any)["head"] = fixtureOutputCommit
			})
			_, err := EncodeCall(call)
			return err
		}},
		{"abort disposition is not abort-preserve", func() error {
			call := fixtureCall(t, OperationPersistAbort)
			call.PersistAbort.Record = resealedDoc(t, call.PersistAbort.Record, func(members map[string]any) {
				members["disposition"] = "fast-forwarded"
			})
			_, err := EncodeCall(call)
			return err
		}},
		{"control ack disposition contradicts its record_schema", func() error {
			reply := fixtureReply(t, OperationPersistAbort)
			reply.PersistAbort.Ack = resealedDoc(t, reply.PersistAbort.Ack, func(members map[string]any) {
				members["disposition"] = "quarantined"
			})
			_, err := EncodeReply(reply)
			return err
		}},
		{"control ack controller_global_sequence is not positive", func() error {
			reply := fixtureReply(t, OperationPersistAbort)
			reply.PersistAbort.Ack = resealedDoc(t, reply.PersistAbort.Ack, func(members map[string]any) {
				members["controller_global_sequence"] = json.Number("0")
			})
			_, err := EncodeReply(reply)
			return err
		}},
		{"handback record renames a published member", func() error {
			call := fixtureCall(t, OperationPersistHandback)
			call.PersistHandback.Record = revisedDoc(t, call.PersistHandback.Record, func(members map[string]any) {
				members["postRunway"] = members["post_runway"]
				delete(members, "post_runway")
			})
			_, err := EncodeCall(call)
			return err
		}},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			if err := row.encode(); err == nil {
				t.Fatalf("published arm accepted %s", row.name)
			}
		})
	}
}

// TestContextOwnerPublishedResumeArm proves the published execution-request
// contract accepts the other closed action arm — a resume request carrying its
// complete continuity checkpoint — unchanged, and refuses a continuity that
// contradicts the request it resumes even when the checkpoint is internally
// consistent.
func TestContextOwnerPublishedResumeArm(t *testing.T) {
	resume := fixtureResumeRequestDoc(t)

	t.Run("resume request is accepted unchanged", func(t *testing.T) {
		call := fixtureCall(t, OperationVerifyAuthority)
		call.VerifyAuthority.Request = resume
		encoded, err := EncodeCall(call)
		if err != nil {
			t.Fatalf("EncodeCall(resume): %v", err)
		}
		decoded, err := DecodeCall(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("DecodeCall(resume): %v", err)
		}
		if !bytes.Equal(decoded.VerifyAuthority.Request, resume) {
			t.Fatal("resume request did not survive the published arm byte-for-byte")
		}
	})

	adverse := []struct {
		name   string
		mutate func(members map[string]any)
	}{
		{"continuity profile_digest contradicts the request", func(members map[string]any) {
			continuity := members["resume"].(map[string]any)["continuity"].(map[string]any)
			continuity["profile_digest"] = fixtureAuthorityDigest
		}},
		{"continuity manifest revision contradicts the request", func(members map[string]any) {
			continuity := members["resume"].(map[string]any)["continuity"].(map[string]any)
			continuity["current_manifest_revision"] = json.Number("7")
		}},
		{"start arm accompanies the resume arm", func(members map[string]any) {
			members["start"] = map[string]any{"expected_source_sequence": json.Number("1")}
		}},
	}
	for _, row := range adverse {
		t.Run(row.name, func(t *testing.T) {
			call := fixtureCall(t, OperationVerifyAuthority)
			call.VerifyAuthority.Request = revisedDoc(t, resume, func(members map[string]any) {
				row.mutate(members)
				resealContinuity(t, members)
			})
			if _, err := EncodeCall(call); err == nil {
				t.Fatalf("published arm accepted %s", row.name)
			}
		})
	}
}

// resealContinuity recomputes the nested checkpoint's self-digest and the
// resume arm's repeat of it, so a mutated continuity stays internally
// consistent and only contradicts the request that resumes it.
func resealContinuity(t *testing.T, members map[string]any) {
	t.Helper()
	resume, ok := members["resume"].(map[string]any)
	if !ok {
		return
	}
	continuity, ok := resume["continuity"].(map[string]any)
	if !ok {
		return
	}
	continuity["digest"] = ""
	digest, err := canonjson.Digest(continuity)
	if err != nil {
		t.Fatalf("canonjson.Digest continuity: %v", err)
	}
	continuity["digest"] = digest
	resume["continuity_digest"] = digest
}

// fixtureExecutionPartialBytes is one genuine canonical execution partial:
// the document is certified by the private decoder that owns it, so the
// fixture is an instance of the accepted contract rather than of this test's
// idea of it. The bytes carry their trailing LF, which is how the controller
// carries preserved execution bytes.
func fixtureExecutionPartialBytes(t *testing.T) []byte {
	t.Helper()
	request := fixtureSealedRequest(t)
	encoded, err := canonjson.Marshal(map[string]any{
		"schema": "verdi.context-execution-partial/v1", "flight": fixtureFlight,
		"lane": fixtureLane, "epoch": fixtureEpoch, "session": fixtureSession,
		"action": "start", "manifest_revision": json.Number("0"),
		"manifest_digest": request.ManifestDigest, "adapter": "codex",
		"adapter_version": fixtureAdapterVersion, "workspace_id": fixtureWorkspaceID(t),
		"adapter_session_ref": "codex-session-1", "authority": "authoritative",
		"witnesses": []any{}, "event_acks": []any{},
	})
	if err != nil {
		t.Fatalf("canonjson.Marshal partial: %v", err)
	}
	if _, err := sealedexec.DecodeExecutionPartial(bytes.NewReader(encoded)); err != nil {
		t.Fatalf("execution partial fixture is not a genuine private instance: %v", err)
	}
	return encoded
}

// fixtureExecutionResultBytes is one genuine canonical finalized result.
func fixtureExecutionResultBytes(t *testing.T) []byte {
	t.Helper()
	receipt, _, ack := fixtureReceiptTriple(t)
	encoded, err := sealedexec.EncodeExecutionResult(sealedexec.ExecutionResult{
		Schema: "verdi.context-execution-result/v1", Verdict: contextcompile.ResolutionProven,
		Authority: contextevent.AuthorityAuthoritative, Witnesses: []string{},
		Flight: fixtureFlight, Lane: fixtureLane, Epoch: fixtureEpoch, Session: fixtureSession,
		ATCRunway: fixtureRunway, ExecutionWorkspaceID: fixtureWorkspaceID(t),
		Adapter: contextevent.AdapterCodex, AdapterVersion: fixtureAdapterVersion,
		InputCommit: fixtureCommit, InputTree: fixtureTree,
		OutputCommit: fixtureOutputCommit, OutputTree: fixtureOutputTree, Clean: true,
		TerminalManifestDigest: receipt.ManifestDigest, TerminalManifestRevision: receipt.TerminalManifestRevision,
		TerminalSourceSequence: receipt.TerminalSourceSequence, TerminalGlobalSequence: receipt.TerminalGlobalSequence,
		EventChainRoot: receipt.EventChainRoot, Receipt: receipt, ReceiptEventAck: ack,
	})
	if err != nil {
		t.Fatalf("EncodeExecutionResult: %v", err)
	}
	return encoded
}

// quarantinePreservedRecord builds the genuine quarantine record that carries
// exactly data under state, so the locator never rejects a preserved-bytes
// mutation for the wrong reason.
func quarantinePreservedRecord(t *testing.T, state sealedexec.PreservedState, data []byte) sealedexec.QuarantineRecord {
	t.Helper()
	preserved, err := sealedexec.PreservedExecutionForBytes(state, data)
	if err != nil {
		t.Fatalf("PreservedExecutionForBytes: %v", err)
	}
	record := fixtureQuarantineRecord(t)
	record.Preserved = preserved
	if state != sealedexec.PreservedFinalized {
		return record
	}
	receipt, _, ack := fixtureReceiptTriple(t)
	record.Receipt = sealedexec.QuarantineReceipt{
		State: sealedexec.QuarantineReceiptDurable, Digest: receipt.Digest, EventAck: &ack,
	}
	record.Repository.Output = sealedexec.QuarantineOutput{
		State: sealedexec.QuarantineOutputObserved, Commit: fixtureOutputCommit, Tree: fixtureOutputTree,
	}
	record.Reason = sealedexec.QuarantineNonAuthoritative
	return record
}

// quarantinePreservedCall is the published call carrying that record and those
// exact preserved bytes.
func quarantinePreservedCall(t *testing.T, state sealedexec.PreservedState, data []byte) Call {
	t.Helper()
	encoded, err := sealedexec.EncodeQuarantineRecord(quarantinePreservedRecord(t, state, data))
	if err != nil {
		t.Fatalf("EncodeQuarantineRecord: %v", err)
	}
	call := fixtureCall(t, OperationPersistQuarantine)
	call.PersistQuarantine.Record = standaloneDoc(encoded)
	call.PersistQuarantine.PreservedBytes = data
	return call
}

// TestContextOwnerPreservedExecutionDifferential is a differential oracle over
// the quarantine preservation cross-field: every row starts from a document
// the private codec certifies, applies one semantic mutation, and asserts that
// the private validator and the published arm agree. A public arm that accepts
// what sealedexec.ValidateQuarantinePreservation rejects is the false green
// this correction exists to close.
func TestContextOwnerPreservedExecutionDifferential(t *testing.T) {
	partial := fixtureExecutionPartialBytes(t)
	finalized := fixtureExecutionResultBytes(t)

	t.Run("genuine preserved arms are accepted", func(t *testing.T) {
		for _, row := range []struct {
			name  string
			state sealedexec.PreservedState
			data  []byte
		}{
			{"none", sealedexec.PreservedNone, []byte{}},
			{"partial", sealedexec.PreservedPartial, partial},
			{"finalized", sealedexec.PreservedFinalized, finalized},
		} {
			t.Run(row.name, func(t *testing.T) {
				record := quarantinePreservedRecord(t, row.state, row.data)
				if err := sealedexec.ValidateQuarantinePreservation(record, row.data); err != nil {
					t.Fatalf("private validator rejected the genuine %s fixture: %v", row.name, err)
				}
				if _, err := EncodeCall(quarantinePreservedCall(t, row.state, row.data)); err != nil {
					t.Fatalf("published arm rejected the genuine %s fixture: %v", row.name, err)
				}
			})
		}
	})

	type preservedRow struct {
		name   string
		state  sealedexec.PreservedState
		base   []byte
		mutate func(members map[string]any)
	}
	rows := []preservedRow{
		{"partial authority contradicts its witnesses", sealedexec.PreservedPartial, partial,
			func(m map[string]any) { m["witnesses"] = []any{"adverse"} }},
		{"partial event_acks are null", sealedexec.PreservedPartial, partial,
			func(m map[string]any) { m["event_acks"] = nil }},
		{"partial action is outside the closed union", sealedexec.PreservedPartial, partial,
			func(m map[string]any) { m["action"] = "restart" }},
		{"partial adapter is unknown", sealedexec.PreservedPartial, partial,
			func(m map[string]any) { m["adapter"] = "gemini" }},
		{"partial manifest_digest is not canonical", sealedexec.PreservedPartial, partial,
			func(m map[string]any) { m["manifest_digest"] = "digest" }},
		{"finalized verdict contradicts authoritative authority", sealedexec.PreservedFinalized, finalized,
			func(m map[string]any) { m["verdict"] = "unproven" }},
		{"finalized output_tree contradicts its receipt", sealedexec.PreservedFinalized, finalized,
			func(m map[string]any) { m["output_tree"] = fixtureTree }},
		{"finalized terminal_source_sequence is zero", sealedexec.PreservedFinalized, finalized,
			func(m map[string]any) { m["terminal_source_sequence"] = json.Number("0") }},
		{"finalized witnesses are null", sealedexec.PreservedFinalized, finalized,
			func(m map[string]any) { m["witnesses"] = nil }},
		{"finalized receipt_event_ack does not follow the terminal position", sealedexec.PreservedFinalized, finalized,
			func(m map[string]any) {
				m["receipt_event_ack"].(map[string]any)["global_sequence"] = json.Number("1")
			}},
	}
	// Accept-direction rows: mutations the private validator still accepts.
	// The published arm must accept them too, or it is over-strict and would
	// refuse operands the accepted contract allows.
	accepted := []preservedRow{
		{"partial resumes rather than starts", sealedexec.PreservedPartial, partial,
			func(m map[string]any) { m["action"] = "resume" }},
		{"partial carries no adapter session yet", sealedexec.PreservedPartial, partial,
			func(m map[string]any) { m["adapter_session_ref"] = "" }},
		{"partial is advisory with explicit witnesses", sealedexec.PreservedPartial, partial,
			func(m map[string]any) {
				m["authority"] = "advisory"
				m["witnesses"] = []any{"adapter-stopped"}
			}},
	}
	for _, group := range []struct {
		rows       []preservedRow
		wantReject bool
	}{{rows, true}, {accepted, false}} {
		for _, row := range group.rows {
			t.Run(row.name, func(t *testing.T) {
				data := remarshalledBytes(t, row.base, row.mutate)
				record := quarantinePreservedRecord(t, row.state, data)
				_, publicErr := EncodeCall(quarantinePreservedCall(t, row.state, data))
				requireAgreement(t, row.name, group.wantReject,
					sealedexec.ValidateQuarantinePreservation(record, data), publicErr)
			})
		}
	}
}

// requireAgreement is the differential assertion: the published arm and the
// private codec must reach the same verdict, and the row must reach the
// verdict it was written to prove. Agreement alone would pass vacuously if a
// mutation stopped being adverse, so wantReject is asserted on both sides.
func requireAgreement(t *testing.T, name string, wantReject bool, privateErr, publicErr error) {
	t.Helper()
	switch {
	case privateErr == nil && publicErr != nil:
		t.Fatalf("published arm rejected %s that the private codec accepts: %v", name, publicErr)
	case privateErr != nil && publicErr == nil:
		t.Fatalf("published arm accepted %s that the private codec rejects: %v", name, privateErr)
	case wantReject && privateErr == nil:
		t.Fatalf("%s is no longer adverse to either codec; the row proves nothing", name)
	case !wantReject && privateErr != nil:
		t.Fatalf("%s was written as an accepted operand but the private codec refused it: %v", name, privateErr)
	}
}

// TestContextOwnerNestedContinuityDifferential is the same differential oracle
// over the resume checkpoint: each mutation recomputes the checkpoint's own
// self-digest, so a row that both codecs must refuse is refused for the
// invariant it names and not for a digest the mutation happened to break.
func TestContextOwnerNestedContinuityDifferential(t *testing.T) {
	resume := fixtureResumeRequestDoc(t)

	type continuityRow struct {
		name   string
		mutate func(continuity map[string]any)
	}
	accepted := []continuityRow{
		{"unmutated checkpoint", func(map[string]any) {}},
		{"current_commit and current_tree advance", func(c map[string]any) {
			c["current_commit"] = fixtureCommit
			c["current_tree"] = fixtureTree
		}},
		{"expansion_ledger_root names another canonical digest", func(c map[string]any) {
			c["expansion_ledger_root"] = fixtureChildDigest
		}},
		{"recorder_checkpoint_digest names another canonical digest", func(c map[string]any) {
			c["recorder_checkpoint_digest"] = fixtureExpansionDigest
		}},
		{"adapter_session_ref names another session", func(c map[string]any) {
			c["adapter_session_ref"] = "codex-session-2"
		}},
	}
	rows := []continuityRow{
		{"event_chain_root does not match revision_segments", func(c map[string]any) {
			c["event_chain_root"] = fixtureAuthorityDigest
		}},
		{"terminal_source_sequence contradicts the final revision", func(c map[string]any) {
			c["terminal_source_sequence"] = json.Number("9")
		}},
		{"terminal_global_sequence contradicts the final revision", func(c map[string]any) {
			c["terminal_global_sequence"] = json.Number("9")
		}},
		{"revision_segments no longer chain to the root", func(c map[string]any) {
			segments := c["revision_segments"].([]any)
			segments[len(segments)-1].(map[string]any)["event_root"] = fixtureAuthorityDigest
		}},
		{"adapter_session_ref is empty", func(c map[string]any) {
			c["adapter_session_ref"] = ""
		}},
		{"current_tree is not a Git object", func(c map[string]any) {
			c["current_tree"] = "HEAD"
		}},
		{"expansion_ledger_root is not a canonical digest", func(c map[string]any) {
			c["expansion_ledger_root"] = "root"
		}},
		{"recorder_checkpoint_digest is not a canonical digest", func(c map[string]any) {
			c["recorder_checkpoint_digest"] = "checkpoint"
		}},
		{"adapter is outside the closed union", func(c map[string]any) {
			c["adapter"] = "gemini"
		}},
	}
	for _, group := range []struct {
		rows       []continuityRow
		wantReject bool
	}{{rows, true}, {accepted, false}} {
		for _, row := range group.rows {
			t.Run(row.name, func(t *testing.T) {
				mutated := revisedDoc(t, resume, func(members map[string]any) {
					row.mutate(members["resume"].(map[string]any)["continuity"].(map[string]any))
					resealContinuity(t, members)
				})
				// sealedexec.DecodeExecutionRequest is the exact private
				// counterpart of the published execution-request arm, so
				// agreement here is agreement on the whole accepted
				// contract, checkpoint interior included.
				_, privateErr := sealedexec.DecodeExecutionRequest(bytes.NewReader(frameDoc(mutated)))
				call := fixtureCall(t, OperationVerifyAuthority)
				call.VerifyAuthority.Request = mutated
				_, publicErr := EncodeCall(call)
				requireAgreement(t, row.name, group.wantReject, privateErr, publicErr)
			})
		}
	}
}

// remarshalledBytes re-canonicalizes a standalone document, trailing LF and
// all, after a single change.
func remarshalledBytes(t *testing.T, document []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	members := revisedMembers(t, document, mutate)
	encoded, err := canonjson.Marshal(members)
	if err != nil {
		t.Fatalf("canonjson.Marshal document: %v", err)
	}
	return encoded
}

// recordArm addresses the control record the operation carries.
func recordArm(t *testing.T, call *Call, operation Operation) *json.RawMessage {
	t.Helper()
	switch operation {
	case OperationPersistHandback:
		return &call.PersistHandback.Record
	case OperationPersistQuarantine:
		return &call.PersistQuarantine.Record
	case OperationPersistAbort:
		return &call.PersistAbort.Record
	default:
		t.Fatalf("operation %q carries no control record", operation)
		return nil
	}
}

// revisedDoc re-canonicalizes one nested document after a single change.
func revisedDoc(t *testing.T, document json.RawMessage, mutate func(map[string]any)) json.RawMessage {
	t.Helper()
	return nestedDoc(t, revisedMembers(t, document, mutate))
}

// resealedDoc re-canonicalizes a self-digested nested document after a single
// change and recomputes its digest exactly as its owning codec does, so the
// document stays internally consistent and only its semantics contradict.
func resealedDoc(t *testing.T, document json.RawMessage, mutate func(map[string]any)) json.RawMessage {
	t.Helper()
	members := revisedMembers(t, document, mutate)
	members["digest"] = ""
	digest, err := canonjson.Digest(members)
	if err != nil {
		t.Fatalf("canonjson.Digest reseal: %v", err)
	}
	members["digest"] = digest
	return nestedDoc(t, members)
}

func revisedMembers(t *testing.T, document json.RawMessage, mutate func(map[string]any)) map[string]any {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var members map[string]any
	if err := decoder.Decode(&members); err != nil {
		t.Fatalf("decode nested document: %v", err)
	}
	mutate(members)
	return members
}

// TestContextOwnerCopiesDecodedValues proves the codec never hands a caller a
// slice that aliases its input, so a later caller cannot rewrite a decoded
// call or reply through the buffer it was decoded from.
func TestContextOwnerCopiesDecodedValues(t *testing.T) {
	encoded, err := EncodeCall(fixtureCall(t, OperationPersistQuarantine))
	if err != nil {
		t.Fatalf("EncodeCall: %v", err)
	}
	buffer := append([]byte(nil), encoded...)
	decoded, err := DecodeCall(bytes.NewReader(buffer))
	if err != nil {
		t.Fatalf("DecodeCall: %v", err)
	}
	for i := range buffer {
		buffer[i] = 'x'
	}
	reencoded, err := EncodeCall(decoded)
	if err != nil {
		t.Fatalf("re-EncodeCall: %v", err)
	}
	if !bytes.Equal(reencoded, encoded) {
		t.Fatalf("decoded call aliases its input:\n got %s\nwant %s", reencoded, encoded)
	}

	// Mutating one decoded value must not reach a second decode of the same
	// bytes: every returned slice is the codec's own copy.
	segment, err := EncodeCall(fixtureCall(t, OperationStoreRedactedSegment))
	if err != nil {
		t.Fatalf("EncodeCall segment: %v", err)
	}
	first, err := DecodeCall(bytes.NewReader(segment))
	if err != nil {
		t.Fatalf("DecodeCall segment: %v", err)
	}
	first.StoreRedactedSegment.Segment.Bytes[0] = 'X'
	second, err := DecodeCall(bytes.NewReader(segment))
	if err != nil {
		t.Fatalf("re-DecodeCall segment: %v", err)
	}
	if second.StoreRedactedSegment.Segment.Bytes[0] != '{' {
		t.Fatal("decoded segment bytes are shared between decodes")
	}
}
