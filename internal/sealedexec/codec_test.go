package sealedexec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
	"github.com/jyang234/verdi/internal/policyconflict"
)

const (
	testSHA1  = "1111111111111111111111111111111111111111"
	testSHA2  = "2222222222222222222222222222222222222222"
	testTree1 = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	testTree2 = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestExecutionRequestCodec_Static(t *testing.T) {
	start := validExecutionRequest(t, ActionStart)
	encoded := mustEncodeExecutionRequest(t, start)
	decoded, err := DecodeExecutionRequest(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeExecutionRequest(valid start): %v", err)
	}
	if decoded.Action != ActionStart || decoded.Start == nil || decoded.Resume != nil {
		t.Fatalf("decoded start union = action %q, start %#v, resume %#v", decoded.Action, decoded.Start, decoded.Resume)
	}

	resume := validExecutionRequest(t, ActionResume)
	resumeEncoded := mustEncodeExecutionRequest(t, resume)
	resumeDecoded, err := DecodeExecutionRequest(bytes.NewReader(resumeEncoded))
	if err != nil {
		t.Fatalf("DecodeExecutionRequest(valid resume): %v", err)
	}
	if resumeDecoded.Action != ActionResume || resumeDecoded.Start != nil || resumeDecoded.Resume == nil {
		t.Fatalf("decoded resume union = action %q, start %#v, resume %#v", resumeDecoded.Action, resumeDecoded.Start, resumeDecoded.Resume)
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{"duplicate key", func() error {
			_, err := DecodeExecutionRequest(bytes.NewReader(duplicateFirstKey(encoded)))
			return err
		}},
		{"unknown field", func() error { _, err := DecodeExecutionRequest(bytes.NewReader(withUnknownField(encoded))); return err }},
		{"trailing data", func() error {
			_, err := DecodeExecutionRequest(bytes.NewReader(append(append([]byte{}, encoded...), []byte("{}\n")...)))
			return err
		}},
		{"noncanonical bytes", func() error { _, err := DecodeExecutionRequest(bytes.NewReader(indentJSON(t, encoded))); return err }},
		{"null action", func() error {
			_, err := DecodeExecutionRequest(bytes.NewReader(mutateJSON(t, encoded, func(m map[string]any) { m["action"] = nil })))
			return err
		}},
		{"absent flight", func() error {
			_, err := DecodeExecutionRequest(bytes.NewReader(mutateJSON(t, encoded, func(m map[string]any) { delete(m, "flight") })))
			return err
		}},
		{"empty lane", func() error { bad := start; bad.Lane = ""; _, err := EncodeExecutionRequest(bad); return err }},
		{"unknown action", func() error {
			bad := start
			bad.Action = Action("replace")
			_, err := EncodeExecutionRequest(bad)
			return err
		}},
		{"both arms", func() error {
			bad := start
			bad.Resume = validResumeArmForRequest(t, bad)
			_, err := EncodeExecutionRequest(bad)
			return err
		}},
		{"neither arm", func() error { bad := start; bad.Start = nil; _, err := EncodeExecutionRequest(bad); return err }},
		{"wrong start sequence", func() error {
			bad := start
			bad.Start = &StartArm{ExpectedSourceSequence: 2}
			_, err := EncodeExecutionRequest(bad)
			return err
		}},
		{"projection digest mismatch", func() error {
			bad := start
			bad.ProjectionDigest = testDigest("wrong projection")
			_, err := EncodeExecutionRequest(bad)
			return err
		}},
		{"manifest digest mismatch", func() error {
			bad := start
			bad.ManifestDigest = testDigest("wrong manifest")
			_, err := EncodeExecutionRequest(bad)
			return err
		}},
		{"workspace commit mismatch", func() error {
			bad := start
			identity, err := NewExecutionWorkspaceRequest(bad.Flight, bad.Lane, bad.Epoch, bad.Session, testSHA2)
			if err != nil {
				return err
			}
			bad.ExecutionWorkspaceRequest = identity
			_, err = EncodeExecutionRequest(bad)
			return err
		}},
		{"workspace run id mismatch", func() error {
			bad := start
			identity, err := execworkspace.NewExactIdentity("vatc-wrong", bad.InputCommit)
			if err != nil {
				return err
			}
			bad.ExecutionWorkspaceRequest = identity
			_, err = EncodeExecutionRequest(bad)
			return err
		}},
		{"workspace patch arm", func() error {
			bad := start
			identity, err := execworkspace.NewPatchIdentity("vatc-patch", bad.InputCommit, []byte("patch"))
			if err != nil {
				return err
			}
			bad.ExecutionWorkspaceRequest = identity
			_, err = EncodeExecutionRequest(bad)
			return err
		}},
		{"projection inventory mismatch", func() error {
			bad := start
			bad.Manifest.ProjectionFiles[0].Digest = testDigest("different projection file")
			bad.Manifest.Digest = ""
			bad.Manifest = mustCanonicalManifest(t, bad.Manifest)
			bad.ManifestDigest = bad.Manifest.Digest
			bad.AuthorityVerdict = mustCanonicalAuthorityReport(t, validAuthorityVerdict(t, bad.ManifestDigest))
			_, err := EncodeExecutionRequest(bad)
			return err
		}},
		{"wrong resume digest", func() error {
			bad := resume
			arm := *bad.Resume
			arm.ContinuityDigest = testDigest("wrong continuity")
			bad.Resume = &arm
			_, err := EncodeExecutionRequest(bad)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("accepted invalid execution request")
			}
		})
	}
}

func TestExecutionContinuityCodec_Static(t *testing.T) {
	continuity := validExecutionContinuity(t)
	encoded := mustEncodeExecutionContinuity(t, continuity)
	decoded, err := DecodeExecutionContinuity(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeExecutionContinuity(valid): %v", err)
	}
	if decoded.EventChainRoot != continuity.EventChainRoot || decoded.Digest == "" {
		t.Fatalf("decoded continuity root/digest = %q/%q", decoded.EventChainRoot, decoded.Digest)
	}

	two := validTwoRevisionContinuity(t)
	tests := []struct {
		name string
		run  func() error
	}{
		{"duplicate key", func() error {
			_, err := DecodeExecutionContinuity(bytes.NewReader(duplicateFirstKey(encoded)))
			return err
		}},
		{"unknown field", func() error {
			_, err := DecodeExecutionContinuity(bytes.NewReader(withUnknownField(encoded)))
			return err
		}},
		{"trailing data", func() error {
			_, err := DecodeExecutionContinuity(bytes.NewReader(append(append([]byte{}, encoded...), '0')))
			return err
		}},
		{"noncanonical bytes", func() error { _, err := DecodeExecutionContinuity(bytes.NewReader(indentJSON(t, encoded))); return err }},
		{"null revisions", func() error {
			_, err := DecodeExecutionContinuity(bytes.NewReader(mutateJSON(t, encoded, func(m map[string]any) { m["revision_segments"] = nil })))
			return err
		}},
		{"absent session", func() error {
			_, err := DecodeExecutionContinuity(bytes.NewReader(mutateJSON(t, encoded, func(m map[string]any) { delete(m, "session") })))
			return err
		}},
		{"unknown adapter", func() error {
			bad := continuity
			bad.Adapter = contextevent.Adapter("other")
			_, err := EncodeExecutionContinuity(bad)
			return err
		}},
		{"self digest mismatch", func() error {
			bad := continuity
			bad.Digest = testDigest("wrong self")
			_, err := EncodeExecutionContinuity(bad)
			return err
		}},
		{"wrong root", func() error {
			bad := continuity
			bad.EventChainRoot = testDigest("wrong root")
			_, err := EncodeExecutionContinuity(bad)
			return err
		}},
		{"terminal source mismatch", func() error {
			bad := continuity
			bad.TerminalSourceSequence++
			_, err := EncodeExecutionContinuity(bad)
			return err
		}},
		{"terminal global mismatch", func() error {
			bad := continuity
			bad.TerminalGlobalSequence++
			_, err := EncodeExecutionContinuity(bad)
			return err
		}},
		{"truncated revision", func() error {
			bad := two
			bad.RevisionSegments = bad.RevisionSegments[:1]
			_, err := EncodeExecutionContinuity(bad)
			return err
		}},
		{"empty adapter session", func() error {
			bad := continuity
			bad.AdapterSessionRef = ""
			_, err := EncodeExecutionContinuity(bad)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("accepted invalid execution continuity")
			}
		})
	}
}

func TestExecutionResultCodec_Static(t *testing.T) {
	result := validExecutionResult(t, contextcompile.ResolutionProven, contextevent.AuthorityAuthoritative)
	encoded := mustEncodeExecutionResult(t, result)
	decoded, err := DecodeExecutionResult(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeExecutionResult(valid authoritative): %v", err)
	}
	if decoded.Authority != contextevent.AuthorityAuthoritative || decoded.Receipt.Digest == "" {
		t.Fatalf("decoded result authority/digest = %q/%q", decoded.Authority, decoded.Receipt.Digest)
	}

	advisory := validExecutionResult(t, contextcompile.ResolutionUnproven, contextevent.AuthorityAdvisory)
	if _, err := EncodeExecutionResult(advisory); err != nil {
		t.Fatalf("EncodeExecutionResult(valid advisory): %v", err)
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{"duplicate key", func() error { _, err := DecodeExecutionResult(bytes.NewReader(duplicateFirstKey(encoded))); return err }},
		{"unknown field", func() error { _, err := DecodeExecutionResult(bytes.NewReader(withUnknownField(encoded))); return err }},
		{"trailing data", func() error {
			_, err := DecodeExecutionResult(bytes.NewReader(append(append([]byte{}, encoded...), []byte("null\n")...)))
			return err
		}},
		{"noncanonical bytes", func() error { _, err := DecodeExecutionResult(bytes.NewReader(indentJSON(t, encoded))); return err }},
		{"null receipt", func() error {
			_, err := DecodeExecutionResult(bytes.NewReader(mutateJSON(t, encoded, func(m map[string]any) { m["receipt"] = nil })))
			return err
		}},
		{"absent ack", func() error {
			_, err := DecodeExecutionResult(bytes.NewReader(mutateJSON(t, encoded, func(m map[string]any) { delete(m, "receipt_event_ack") })))
			return err
		}},
		{"authority upgrade", func() error {
			bad := advisory
			bad.Authority = contextevent.AuthorityAuthoritative
			bad.Verdict = contextcompile.ResolutionProven
			bad.Witnesses = []string{}
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"proven advisory", func() error {
			bad := result
			bad.Authority = contextevent.AuthorityAdvisory
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"unknown verdict", func() error {
			bad := result
			bad.Verdict = contextcompile.Resolution("maybe")
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"unknown authority", func() error {
			bad := result
			bad.Authority = contextevent.Authority("elevated")
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"authoritative witnesses", func() error {
			bad := result
			bad.Witnesses = []string{"adverse"}
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"advisory without witnesses", func() error {
			bad := advisory
			bad.Witnesses = []string{}
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"dirty authoritative output", func() error { bad := result; bad.Clean = false; _, err := EncodeExecutionResult(bad); return err }},
		{"wrong root", func() error {
			bad := result
			bad.EventChainRoot = testDigest("wrong result root")
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"terminal mismatch", func() error {
			bad := result
			bad.TerminalSourceSequence++
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"receipt repository mismatch", func() error {
			bad := result
			bad.OutputCommit = testSHA1
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"receipt digest mismatch", func() error {
			bad := result
			bad.ReceiptEventAck.ReceiptDigest = testDigest("wrong receipt")
			_, err := EncodeExecutionResult(bad)
			return err
		}},
		{"ack identity mismatch", func() error {
			bad := result
			bad.ReceiptEventAck.Session = "other"
			_, err := EncodeExecutionResult(bad)
			return err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("accepted invalid execution result")
			}
		})
	}
}

func TestProviderInputAuthorityBoundary_Static(t *testing.T) {
	projection := validInstructionProjection()
	encoded := mustEncodeInstructionProjection(t, projection)
	decoded, err := DecodeInstructionProjection(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeInstructionProjection(valid): %v", err)
	}
	input := ProviderInput{
		Instructions: InstructionAuthority{Projection: decoded},
		Data:         []contextcompile.DataItem{validDataItem(t)},
	}
	if err := input.Validate(); err != nil {
		t.Fatalf("ProviderInput.Validate(valid): %v", err)
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{"duplicate projection key", func() error {
			_, err := DecodeInstructionProjection(bytes.NewReader(duplicateFirstKey(encoded)))
			return err
		}},
		{"unknown projection field", func() error {
			_, err := DecodeInstructionProjection(bytes.NewReader(withUnknownField(encoded)))
			return err
		}},
		{"projection trailing data", func() error {
			_, err := DecodeInstructionProjection(bytes.NewReader(append(append([]byte{}, encoded...), '0')))
			return err
		}},
		{"projection noncanonical", func() error {
			_, err := DecodeInstructionProjection(bytes.NewReader(indentJSON(t, encoded)))
			return err
		}},
		{"null projection rows", func() error {
			_, err := DecodeInstructionProjection(bytes.NewReader(mutateJSON(t, encoded, func(m map[string]any) { m["files"] = nil })))
			return err
		}},
		{"empty projection", func() error {
			bad := projection
			bad.Files = []InstructionFile{}
			_, err := EncodeInstructionProjection(bad)
			return err
		}},
		{"unsorted projection", func() error {
			bad := twoFileProjection()
			bad.Files[0], bad.Files[1] = bad.Files[1], bad.Files[0]
			_, err := EncodeInstructionProjection(bad)
			return err
		}},
		{"duplicate projection", func() error {
			bad := twoFileProjection()
			bad.Files[1] = bad.Files[0]
			_, err := EncodeInstructionProjection(bad)
			return err
		}},
		{"invalid UTF-8 projection", func() error {
			bad := projection
			bad.Files[0].Content = string([]byte{0xff})
			_, err := EncodeInstructionProjection(bad)
			return err
		}},
		{"content digest mismatch", func() error {
			bad := projection
			bad.Files[0].ContentDigest = testDigest("wrong content")
			_, err := EncodeInstructionProjection(bad)
			return err
		}},
		{"self digest mismatch", func() error {
			bad := projection
			bad.Digest = testDigest("wrong projection self")
			_, err := EncodeInstructionProjection(bad)
			return err
		}},
		{"instruction disguised as data", func() error {
			bad := input
			bad.Data = []contextcompile.DataItem{{Schema: contextcompile.DataItemSchema, Kind: contextcompile.IncludedInstructionProjection}}
			return bad.Validate()
		}},
		{"missing instruction authority", func() error { bad := input; bad.Instructions = InstructionAuthority{}; return bad.Validate() }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil {
				t.Fatal("accepted invalid provider authority boundary")
			}
		})
	}
	if got := input.Data; got == nil {
		t.Fatal("provider data channel must remain an explicit typed DataItem array")
	}
}

func validExecutionRequest(t *testing.T, action Action) ExecutionRequest {
	t.Helper()
	projection := mustCanonicalProjection(t, validInstructionProjection())
	manifest := mustCanonicalManifest(t, validManifestForProjection(t, projection))
	workspace, err := NewExecutionWorkspaceRequest("flight-1", "lane-1", "epoch-1", "session-1", testSHA1)
	if err != nil {
		t.Fatalf("NewExecutionWorkspaceRequest: %v", err)
	}
	request := ExecutionRequest{
		Schema:                    ExecutionRequestSchemaID,
		Action:                    action,
		Flight:                    "flight-1",
		Lane:                      "lane-1",
		Epoch:                     "epoch-1",
		ManifestRevision:          0,
		Session:                   "session-1",
		ATCRunway:                 ".vatc",
		InputCommit:               testSHA1,
		InputTree:                 testTree1,
		Manifest:                  manifest,
		ManifestDigest:            manifest.Digest,
		InstructionProjection:     projection,
		ProjectionDigest:          projection.Digest,
		ExecutionWorkspaceRequest: workspace,
		Adapter:                   contextevent.AdapterCodex,
		AdapterVersion:            "1.0.0",
		Profile: LogicalRef{
			Schema: ProjectProfileRefSchemaID,
			ID:     "project-profile",
			Digest: testDigest("profile"),
		},
		Grants:           execworkspace.GrantSet{Grants: []execworkspace.Grant{}},
		AuthorityVerdict: mustCanonicalAuthorityReport(t, validAuthorityVerdict(t, manifest.Digest)),
		RecorderEndpoint: LogicalRef{
			Schema: RecorderEndpointRefSchemaID,
			ID:     "vatc-recorder",
			Digest: testDigest("recorder"),
		},
	}
	if action == ActionStart {
		request.Start = &StartArm{ExpectedSourceSequence: 1}
	} else {
		request.Resume = validResumeArmForRequest(t, request)
	}
	return request
}

func validResumeArmForRequest(t *testing.T, request ExecutionRequest) *ResumeArm {
	t.Helper()
	continuity := validExecutionContinuity(t)
	continuity.CurrentManifestRevision = request.ManifestRevision
	continuity.CurrentManifestDigest = request.ManifestDigest
	continuity.RevisionSegments[len(continuity.RevisionSegments)-1].ManifestRevision = request.ManifestRevision
	continuity.RevisionSegments[len(continuity.RevisionSegments)-1].ManifestDigest = request.ManifestDigest
	root, err := contextevent.EventChainRoot(continuity.RevisionSegments)
	if err != nil {
		t.Fatalf("EventChainRoot fixture: %v", err)
	}
	continuity.EventChainRoot = root
	continuity.ProjectionDigest = request.ProjectionDigest
	continuity.ProfileDigest = request.Profile.Digest
	grants, err := execworkspace.EncodeGrantSet(request.Grants)
	if err != nil {
		t.Fatalf("EncodeGrantSet fixture: %v", err)
	}
	continuity.GrantDigest = rawDigest(grants)
	continuity.AuthorityVerdictDigest = request.AuthorityVerdict.Digest
	continuity.ExecutionWorkspaceRequestDigest, err = ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		t.Fatalf("ExecutionWorkspaceRequestDigest fixture: %v", err)
	}
	continuity.ExecutionWorkspaceID, err = request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatalf("WorkspaceID fixture: %v", err)
	}
	continuity.Digest = ""
	continuity = mustCanonicalContinuity(t, continuity)
	return &ResumeArm{Continuity: continuity, ContinuityDigest: continuity.Digest}
}

func validExecutionContinuity(t *testing.T) ExecutionContinuity {
	t.Helper()
	revision := contextevent.Revision{
		Schema:                 contextevent.RevisionSchemaID,
		ManifestRevision:       0,
		ManifestDigest:         testDigest("manifest-0"),
		FirstGlobalSequence:    1,
		TerminalGlobalSequence: 3,
		TerminalSourceSequence: 3,
		TerminalKind:           contextevent.KindExecutionResult,
		EventRoot:              testDigest("event-0"),
	}
	root, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		t.Fatalf("EventChainRoot: %v", err)
	}
	workspace, err := NewExecutionWorkspaceRequest("flight-1", "lane-1", "epoch-1", "session-1", testSHA1)
	if err != nil {
		t.Fatalf("NewExecutionWorkspaceRequest: %v", err)
	}
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(workspace)
	if err != nil {
		t.Fatalf("ExecutionWorkspaceRequestDigest: %v", err)
	}
	return ExecutionContinuity{
		Schema:                          ExecutionContinuitySchemaID,
		Flight:                          "flight-1",
		Lane:                            "lane-1",
		Epoch:                           "epoch-1",
		Session:                         "session-1",
		Adapter:                         contextevent.AdapterCodex,
		AdapterVersion:                  "1.0.0",
		ATCRunway:                       ".vatc",
		InputCommit:                     testSHA1,
		InputTree:                       testTree1,
		CurrentCommit:                   testSHA2,
		CurrentTree:                     testTree2,
		ExecutionWorkspaceID:            "flight-1--111111111111",
		ExecutionWorkspaceRequestDigest: workspaceDigest,
		ProfileDigest:                   testDigest("profile"),
		GrantDigest:                     testDigest("grants"),
		AuthorityVerdictDigest:          testDigest("authority"),
		CurrentManifestRevision:         0,
		CurrentManifestDigest:           revision.ManifestDigest,
		ProjectionDigest:                testDigest("projection"),
		RevisionSegments:                []contextevent.Revision{revision},
		EventChainRoot:                  root,
		ExpansionLedgerRoot:             testDigest("expansions"),
		TerminalSourceSequence:          revision.TerminalSourceSequence,
		TerminalGlobalSequence:          revision.TerminalGlobalSequence,
		RecorderCheckpointDigest:        testDigest("checkpoint"),
		AdapterSessionRef:               "codex-session-1",
	}
}

func validTwoRevisionContinuity(t *testing.T) ExecutionContinuity {
	t.Helper()
	continuity := validExecutionContinuity(t)
	first := contextevent.Revision{
		Schema:                 contextevent.RevisionSchemaID,
		ManifestRevision:       0,
		ManifestDigest:         testDigest("manifest-0"),
		FirstGlobalSequence:    1,
		TerminalGlobalSequence: 2,
		TerminalSourceSequence: 2,
		TerminalKind:           contextevent.KindChildManifest,
		EventRoot:              testDigest("event-0"),
	}
	second := contextevent.Revision{
		Schema:                 contextevent.RevisionSchemaID,
		ManifestRevision:       1,
		ManifestDigest:         testDigest("manifest-1"),
		FirstGlobalSequence:    3,
		TerminalGlobalSequence: 5,
		TerminalSourceSequence: 3,
		TerminalKind:           contextevent.KindExecutionResult,
		EventRoot:              testDigest("event-1"),
	}
	continuity.RevisionSegments = []contextevent.Revision{first, second}
	root, err := contextevent.EventChainRoot(continuity.RevisionSegments)
	if err != nil {
		t.Fatalf("EventChainRoot fixture: %v", err)
	}
	continuity.EventChainRoot = root
	continuity.CurrentManifestRevision = second.ManifestRevision
	continuity.CurrentManifestDigest = second.ManifestDigest
	continuity.TerminalSourceSequence = second.TerminalSourceSequence
	continuity.TerminalGlobalSequence = second.TerminalGlobalSequence
	return continuity
}

func validExecutionResult(t *testing.T, verdict contextcompile.Resolution, authority contextevent.Authority) ExecutionResult {
	t.Helper()
	revision := contextevent.Revision{
		Schema:                 contextevent.RevisionSchemaID,
		ManifestRevision:       0,
		ManifestDigest:         testDigest("manifest-0"),
		FirstGlobalSequence:    1,
		TerminalGlobalSequence: 3,
		TerminalSourceSequence: 3,
		TerminalKind:           contextevent.KindExecutionResult,
		EventRoot:              testDigest("execution-result-event"),
	}
	root, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		t.Fatalf("EventChainRoot fixture: %v", err)
	}
	receipt := validReceipt(t, authority, revision, root)
	receiptBytes, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		t.Fatalf("EncodeReceipt fixture: %v", err)
	}
	receipt, err = contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		t.Fatalf("DecodeReceipt fixture: %v", err)
	}
	witnesses := []string{}
	if authority == contextevent.AuthorityAdvisory {
		witnesses = []string{"runner authentication unproven"}
	}
	return ExecutionResult{
		Schema:                   ExecutionResultSchemaID,
		Verdict:                  verdict,
		Authority:                authority,
		Witnesses:                witnesses,
		Flight:                   "flight-1",
		Lane:                     "lane-1",
		Epoch:                    "epoch-1",
		Session:                  "session-1",
		ATCRunway:                ".vatc",
		ExecutionWorkspaceID:     receipt.ExecutionWorkspaceID,
		Adapter:                  receipt.Adapter,
		AdapterVersion:           receipt.AdapterVersion,
		InputCommit:              receipt.InputCommit,
		InputTree:                receipt.InputTree,
		OutputCommit:             receipt.OutputCommit,
		OutputTree:               receipt.OutputTree,
		Clean:                    true,
		TerminalManifestDigest:   receipt.ManifestDigest,
		TerminalManifestRevision: receipt.TerminalManifestRevision,
		TerminalSourceSequence:   receipt.TerminalSourceSequence,
		TerminalGlobalSequence:   receipt.TerminalGlobalSequence,
		EventChainRoot:           receipt.EventChainRoot,
		Receipt:                  receipt,
		ReceiptEventAck: contextevent.ReceiptEventAck{
			Schema:           contextevent.ReceiptAckSchemaID,
			Flight:           "flight-1",
			Lane:             "lane-1",
			Epoch:            "epoch-1",
			Session:          "session-1",
			ManifestRevision: receipt.TerminalManifestRevision,
			Kind:             contextevent.KindReceipt,
			SourceSequence:   receipt.TerminalSourceSequence + 1,
			EventDigest:      testDigest("receipt-event"),
			GlobalSequence:   receipt.TerminalGlobalSequence + 1,
			ReceiptDigest:    receipt.Digest,
		},
	}
}

func validReceipt(t *testing.T, authority contextevent.Authority, revision contextevent.Revision, root string) contextreceipt.Receipt {
	t.Helper()
	state := gp.ResolutionAuthenticated
	if authority == contextevent.AuthorityAdvisory {
		state = gp.ResolutionUnproven
	}
	resolution := gp.PrincipalResolution{
		State: state,
		Claim: gp.PrincipalClaim{
			TrustSource: "test-trust-source",
			Subject:     "test-subject",
		},
		Witnesses: []gp.Witness{{Code: "test-principal", SourceID: "test-source"}},
	}
	if state == gp.ResolutionAuthenticated {
		principalID, err := gp.CanonicalPrincipalID(resolution.Claim.TrustSource, resolution.Claim.Subject)
		if err != nil {
			t.Fatalf("CanonicalPrincipalID fixture: %v", err)
		}
		resolution.PrincipalID = principalID
	}
	return contextreceipt.Receipt{
		Schema:                          contextreceipt.SchemaID,
		Role:                            contextreceipt.RoleBuilder,
		Authority:                       authority,
		ManifestDigest:                  revision.ManifestDigest,
		DispatchDigest:                  testDigest("dispatch"),
		ATCRunway:                       ".vatc",
		ExecutionWorkspaceRequestDigest: testDigest("workspace-request"),
		ExecutionWorkspaceID:            "flight-1--111111111111",
		InputCommit:                     testSHA1,
		InputTree:                       testTree1,
		OutputCommit:                    testSHA2,
		OutputTree:                      testTree2,
		Clean:                           true,
		RevisionSegments:                []contextevent.Revision{revision},
		EventChainRoot:                  root,
		TerminalManifestRevision:        revision.ManifestRevision,
		TerminalSourceSequence:          revision.TerminalSourceSequence,
		TerminalGlobalSequence:          revision.TerminalGlobalSequence,
		Expansions:                      []contextreceipt.Expansion{},
		Obligations:                     []contextreceipt.Obligation{},
		Evidence:                        []contextreceipt.Evidence{},
		RunnerPrincipalResolution:       resolution,
		Adapter:                         contextevent.AdapterCodex,
		AdapterVersion:                  "1.0.0",
		ReviewInputs:                    []contextreceipt.ReviewInput{},
	}
}

func validInstructionProjection() InstructionProjection {
	content := "sealed instructions\n"
	return InstructionProjection{
		Schema: InstructionProjectionSchemaID,
		Files: []InstructionFile{{
			Path:          "AGENTS.md",
			ContentDigest: rawDigest([]byte(content)),
			Content:       content,
		}},
	}
}

func validDataItem(t *testing.T) contextcompile.DataItem {
	t.Helper()
	content := []byte("repository data\n")
	item, encoded, err := contextcompile.BuildDataItem(contextcompile.Candidate{
		ID: "path:README.md", Source: contextcompile.SourceHeadTree, Path: "README.md",
	}, contextcompile.IncludedRepositoryFile, content)
	if err != nil {
		t.Fatalf("BuildDataItem fixture: %v", err)
	}
	decoded, err := contextcompile.DecodeDataItem(encoded)
	if err != nil {
		t.Fatalf("DecodeDataItem fixture: %v", err)
	}
	if item.Digest != decoded.Digest {
		t.Fatalf("BuildDataItem digest %q != decoded digest %q", item.Digest, decoded.Digest)
	}
	return decoded
}

func twoFileProjection() InstructionProjection {
	projection := validInstructionProjection()
	content := "second authority file\n"
	projection.Files = append(projection.Files, InstructionFile{
		Path:          "CONTRIBUTING.md",
		ContentDigest: rawDigest([]byte(content)),
		Content:       content,
	})
	return projection
}

func validManifestForProjection(t *testing.T, projection InstructionProjection) contextcompile.Manifest {
	t.Helper()
	files := make([]contextcompile.ProjectionFileRef, len(projection.Files))
	for i, file := range projection.Files {
		files[i] = contextcompile.ProjectionFileRef{Path: file.Path, Digest: file.ContentDigest}
	}
	return contextcompile.Manifest{
		Schema:    contextcompile.ManifestSchema,
		Phase:     contextcompile.PhaseBuild,
		Adapter:   contextcompile.AdapterRef{ID: "codex", Version: "1.0.0"},
		Revisions: contextcompile.Revisions{Authority: testDigest("revision-authority"), Context: 1},
		AcceptedSpec: contextcompile.AcceptedSpec{
			Ref: "spec/test", Path: ".verdi/specs/active/test/spec.md", Blob: testSHA1,
			Commit: testSHA1, ContentDigest: testDigest("accepted-spec"),
		},
		ParentFeatures: []contextcompile.ParentFeature{},
		Decisions:      []contextcompile.DecisionRef{},
		Obligations:    []contextcompile.Obligation{},
		Repository: contextcompile.RepositoryFacts{
			RemoteOrigin: contextcompile.StringFact{Known: true, Value: "origin"},
			Branch:       contextcompile.StringFact{Known: true, Value: "feature/test"},
			Head:         contextcompile.StringFact{Known: true, Value: testSHA1},
			DefaultBranch: contextcompile.DefaultBranchFact{
				Known: true, Name: "main", Ref: "refs/heads/main", Head: testSHA1,
			},
			Relationship: contextcompile.RelationshipEqual,
			Dirty:        contextcompile.BoolFact{Known: true, Value: false},
			Staged:       contextcompile.BoolFact{Known: true, Value: false},
			Worktree:     contextcompile.WorktreeFact{Managed: true, Name: "test-worktree"},
			Source:       contextcompile.RepoSourceHead,
			Disclosures:  []contextcompile.DisclosureCode{},
		},
		Policy: contextcompile.PolicySection{
			EffectiveDigest: testDigest("effective-policy"), ConstitutionDigest: testDigest("constitution"),
			ProfileID: "profile", ProfileDigest: testDigest("policy-profile"), Entries: []contextcompile.PolicyEntry{},
		},
		Owners:            []string{"platform-team"},
		Scope:             validScope(t),
		GovernanceProfile: contextcompile.GovernanceProfileRef{ID: "profile", Class: gp.ClassSolo, Digest: testDigest("governance-profile")},
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
		Digest:      testDigest("manifest"),
	}
}

func validScope(t *testing.T) policyartifact.Scope {
	t.Helper()
	var scope policyartifact.Scope
	data := []byte(`{"phases":["build"],"environments":["local"],"paths":[".verdi/**"],"refs":["spec/test"]}`)
	if err := json.Unmarshal(data, &scope); err != nil {
		t.Fatalf("decode scope fixture: %v", err)
	}
	return scope
}

func validAuthorityVerdict(t *testing.T, manifestDigest string) policyconflict.Report {
	t.Helper()
	report := policyconflict.Report{
		Schema: policyconflict.ReportSchema,
		Input: policyconflict.InputIdentity{
			Target: policyconflict.TargetIdentity{
				Kind: policyconflict.TargetAcceptedContext,
				Accepted: &policyconflict.AcceptedIdentity{
					ManifestDigest: manifestDigest,
				},
			},
			ConstitutionDigest:    testDigest("constitution"),
			EffectivePolicyDigest: testDigest("effective-policy"),
			PolicyEntries:         []policyconflict.PolicyEntryIdentity{},
			Profile: policyconflict.ProfileIdentity{
				ID: "profile", Class: string(gp.ClassSolo), Digest: testDigest("governance-profile"),
			},
			EvaluatedOn: "2026-08-27",
		},
		Mechanical:  []policyconflict.MechanicalEvaluation{},
		Semantic:    []policyconflict.SemanticEvaluation{},
		Disclosures: []policyconflict.Disclosure{},
		Verdict:     policyconflict.VerdictPass,
		Digest:      testDigest("authority"),
	}
	repository := []byte(`{"remote_origin":{"known":true,"value":"origin"},"branch":{"known":true,"value":"feature/test"},"head":{"known":true,"value":"1111111111111111111111111111111111111111"},"default_branch":{"known":true,"name":"main","ref":"refs/heads/main","head":"1111111111111111111111111111111111111111"},"relationship":"equal","dirty":{"known":true,"value":false},"staged":{"known":true,"value":false},"worktree":{"managed":true,"name":"test-worktree"},"source":"head"}`)
	if err := json.Unmarshal(repository, &report.Input.Repository); err != nil {
		t.Fatalf("decode policy report repository fixture: %v", err)
	}
	return report
}

func mustCanonicalProjection(t *testing.T, projection InstructionProjection) InstructionProjection {
	t.Helper()
	encoded := mustEncodeInstructionProjection(t, projection)
	decoded, err := DecodeInstructionProjection(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeInstructionProjection fixture: %v", err)
	}
	return decoded
}

func mustCanonicalManifest(t *testing.T, manifest contextcompile.Manifest) contextcompile.Manifest {
	t.Helper()
	encoded, err := contextcompile.EncodeManifest(manifest)
	if err != nil {
		t.Fatalf("EncodeManifest fixture: %v", err)
	}
	decoded, err := contextcompile.DecodeManifest(encoded)
	if err != nil {
		t.Fatalf("DecodeManifest fixture: %v", err)
	}
	return decoded
}

func mustCanonicalAuthorityReport(t *testing.T, report policyconflict.Report) policyconflict.Report {
	t.Helper()
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport fixture: %v", err)
	}
	decoded, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatalf("DecodeReport fixture: %v", err)
	}
	return decoded
}

func mustCanonicalContinuity(t *testing.T, continuity ExecutionContinuity) ExecutionContinuity {
	t.Helper()
	encoded := mustEncodeExecutionContinuity(t, continuity)
	decoded, err := DecodeExecutionContinuity(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeExecutionContinuity fixture: %v", err)
	}
	return decoded
}

func mustEncodeInstructionProjection(t *testing.T, projection InstructionProjection) []byte {
	t.Helper()
	encoded, err := EncodeInstructionProjection(projection)
	if err != nil {
		t.Fatalf("EncodeInstructionProjection: %v", err)
	}
	return encoded
}

func mustEncodeExecutionRequest(t *testing.T, request ExecutionRequest) []byte {
	t.Helper()
	encoded, err := EncodeExecutionRequest(request)
	if err != nil {
		t.Fatalf("EncodeExecutionRequest: %v", err)
	}
	return encoded
}

func mustEncodeExecutionContinuity(t *testing.T, continuity ExecutionContinuity) []byte {
	t.Helper()
	encoded, err := EncodeExecutionContinuity(continuity)
	if err != nil {
		t.Fatalf("EncodeExecutionContinuity: %v", err)
	}
	return encoded
}

func mustEncodeExecutionResult(t *testing.T, result ExecutionResult) []byte {
	t.Helper()
	encoded, err := EncodeExecutionResult(result)
	if err != nil {
		t.Fatalf("EncodeExecutionResult: %v", err)
	}
	return encoded
}

func testDigest(s string) string { return rawDigest([]byte(s)) }

func rawDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func duplicateFirstKey(data []byte) []byte {
	trimmed := bytes.TrimSuffix(data, []byte("\n"))
	return append(append([]byte(`{"schema":"duplicate",`), trimmed[1:]...), '\n')
}

func withUnknownField(data []byte) []byte {
	trimmed := bytes.TrimSuffix(data, []byte("\n"))
	return append(append([]byte(`{"unknown":true,`), trimmed[1:]...), '\n')
}

func indentJSON(t *testing.T, data []byte) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := json.Indent(&out, bytes.TrimSpace(data), "", "  "); err != nil {
		t.Fatalf("json.Indent: %v", err)
	}
	out.WriteByte('\n')
	return out.Bytes()
}

func mutateJSON(t *testing.T, data []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var doc map[string]any
	if err := decoder.Decode(&doc); err != nil {
		t.Fatalf("decode mutation source: %v", err)
	}
	mutate(doc)
	encoded, err := canonjson.Marshal(doc)
	if err != nil {
		t.Fatalf("encode mutation: %v", err)
	}
	return encoded
}
