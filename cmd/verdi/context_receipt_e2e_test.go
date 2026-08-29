package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/execworkspace"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/sealedexec"
)

func TestContextReceiptVerifyCLI_BuiltBinary(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "verdi")
	build := exec.Command("go", "build", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build verdi: %v\n%s", err, output)
	}

	t.Run("sole grammar and strict input", func(t *testing.T) {
		for _, test := range []struct {
			name string
			args []string
			in   string
		}{
			{name: "missing verify", args: []string{"context", "receipt"}},
			{name: "mint is not public", args: []string{"context", "receipt", "mint"}},
			{name: "strict request", args: []string{"context", "receipt", "verify", "--request", "-"}, in: "{}\n"},
		} {
			t.Run(test.name, func(t *testing.T) {
				observation := runReceiptBinary(t, binary, t.TempDir(), []byte(test.in), nil, test.args...)
				if observation.exitCode != 2 || observation.stdout != "" || !strings.HasPrefix(observation.stderr, "context receipt:") {
					t.Fatalf("observation = %#v, want path-free operational receipt diagnostic", observation)
				}
			})
		}
	})

	fixture := newReceiptCLIFixture(t)
	t.Run("path and stdin are byte-identical absent-controller unproven verdicts", func(t *testing.T) {
		dir := t.TempDir()
		requestPath := filepath.Join(dir, "private-proof-name.json")
		if err := os.WriteFile(requestPath, fixture.requestBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		stdinResult := runReceiptBinary(t, binary, dir, fixture.requestBytes, nil, "context", "receipt", "verify", "--request", "-")
		pathResult := runReceiptBinary(t, binary, dir, nil, nil, "context", "receipt", "verify", "--request", requestPath)
		for name, result := range map[string]sealedContextObservation{"stdin": stdinResult, "path": pathResult} {
			if result.exitCode != 1 || result.stderr != "" {
				t.Fatalf("%s result = %#v, want clean verdict exit 1", name, result)
			}
			verdict, err := contextreceipt.DecodeVerdict(strings.NewReader(result.stdout))
			if err != nil || verdict.State != contextreceipt.StateUnproven {
				t.Fatalf("%s verdict = %#v/%v", name, verdict, err)
			}
		}
		if stdinResult.stdout != pathResult.stdout {
			t.Fatalf("stdin/path verdict bytes differ\nstdin: %s\npath: %s", stdinResult.stdout, pathResult.stdout)
		}
		if strings.Contains(pathResult.stderr, requestPath) {
			t.Fatalf("diagnostic leaks request path: %q", pathResult.stderr)
		}
	})

	t.Run("atomic output is the only write", func(t *testing.T) {
		dir := t.TempDir()
		marker := filepath.Join(dir, "marker")
		if err := os.WriteFile(marker, []byte("unchanged\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out := filepath.Join(dir, "verdict.json")
		result := runReceiptBinary(t, binary, dir, fixture.requestBytes, nil, "context", "receipt", "verify", "--request", "-", "--out", out)
		if result.exitCode != 1 || result.stdout != "" || result.stderr != "" {
			t.Fatalf("output result = %#v", result)
		}
		outBytes, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := contextreceipt.DecodeVerdict(bytes.NewReader(outBytes)); err != nil {
			t.Fatalf("output verdict: %v", err)
		}
		if markerBytes, err := os.ReadFile(marker); err != nil || string(markerBytes) != "unchanged\n" {
			t.Fatalf("marker mutated: %q/%v", markerBytes, err)
		}
		failure := runReceiptBinary(t, binary, dir, fixture.requestBytes, nil, "context", "receipt", "verify", "--request", "-", "--out", dir)
		if failure.exitCode != 2 || failure.stdout != "" || strings.Contains(failure.stderr, dir) {
			t.Fatalf("output failure = %#v, want redacted operational exit", failure)
		}
	})

	t.Run("wrong repository proof is a violated verdict", func(t *testing.T) {
		request := fixture.request
		proof, err := contextreceipt.DecodeRepositoryProof(bytes.NewReader(request.Proofs.RepositoryProofBytes))
		if err != nil {
			t.Fatal(err)
		}
		proof.Digest = ""
		proof.Objects[0].Content = append(proof.Objects[0].Content, 'x')
		request.Proofs.RepositoryProofBytes, err = contextreceipt.EncodeRepositoryProof(proof)
		if err != nil {
			t.Fatal(err)
		}
		request.Digest = ""
		requestBytes, err := contextreceipt.EncodeVerifyRequest(request)
		if err != nil {
			t.Fatal(err)
		}
		result := runReceiptBinary(t, binary, t.TempDir(), requestBytes, nil, "context", "receipt", "verify", "--request", "-")
		verdict, decodeErr := contextreceipt.DecodeVerdict(strings.NewReader(result.stdout))
		if result.exitCode != 1 || decodeErr != nil || verdict.State != contextreceipt.StateViolated {
			t.Fatalf("wrong repository result = %#v, verdict=%#v/%v", result, verdict, decodeErr)
		}
	})

	t.Run("exact controller proof makes every operand proven", func(t *testing.T) {
		result, call := runReceiptWithControllerReply(t, binary, fixture.requestBytes, func(call sealedexec.ControllerCall) []byte {
			return receiptAuthorityResultBytes(t, call, fixture.authority)
		})
		verdict, err := contextreceipt.DecodeVerdict(strings.NewReader(result.stdout))
		if result.exitCode != 0 || result.stderr != "" || err != nil || verdict.State != contextreceipt.StateProven {
			t.Fatalf("all-proven result = %#v, verdict=%#v/%v", result, verdict, err)
		}
		if call.Operation != sealedexec.ControllerOperationResolveReceiptVerificationAuthority || !reflect.DeepEqual(call.ResolveReceiptVerificationAuthority.Query.RunnerClaim, fixture.request.Receipt.RunnerPrincipalResolution.Claim) {
			t.Fatalf("authority call = %#v", call)
		}
	})

	t.Run("controller unavailable fact remains a verdict", func(t *testing.T) {
		facts := fixture.authority
		witness := contextreceipt.Witness{Code: "authority-unavailable", SourceID: "controller", Detail: "unavailable"}
		facts.Profile = contextreceipt.ProfileAuthority{State: contextreceipt.StateUnproven, ProfileBytes: []byte{}, Witnesses: []contextreceipt.Witness{witness}}
		facts.TrustFact = gp.TrustFact{SourceID: fixture.request.Receipt.RunnerPrincipalResolution.Claim.TrustSource, SourceKind: gp.TrustSourceIdentityProvider, Subjects: []string{}, Available: false, Valid: false, Reason: "unavailable"}
		facts.Isolation = contextreceipt.IsolationAuthority{State: contextreceipt.StateUnproven, Witnesses: []contextreceipt.Witness{witness}}
		facts.Persistence = contextreceipt.PersistenceAuthority{State: contextreceipt.StateUnproven, Witnesses: []contextreceipt.Witness{witness}}
		result, _ := runReceiptWithControllerReply(t, binary, fixture.requestBytes, func(call sealedexec.ControllerCall) []byte {
			return receiptAuthorityResultBytes(t, call, facts)
		})
		verdict, err := contextreceipt.DecodeVerdict(strings.NewReader(result.stdout))
		if result.exitCode != 1 || result.stderr != "" || err != nil || verdict.State != contextreceipt.StateUnproven {
			t.Fatalf("unavailable authority result = %#v, verdict=%#v/%v", result, verdict, err)
		}
	})

	t.Run("controller protocol faults are operational", func(t *testing.T) {
		for _, test := range []struct {
			name  string
			reply func(sealedexec.ControllerCall) []byte
		}{
			{name: "malformed", reply: func(sealedexec.ControllerCall) []byte { return []byte("{}\n") }},
			{name: "operation", reply: func(call sealedexec.ControllerCall) []byte {
				result := sealedexec.ControllerResult{Schema: sealedexec.ControllerResultSchemaID, CallSequence: call.CallSequence, Operation: sealedexec.ControllerOperationNextStamp, NextStamp: sealedexec.ControllerNextStampResult{Schema: "verdi.context-controller/next-stamp-result/v1", Stamp: "2026-08-28T12:34:56Z"}}
				encoded, err := sealedexec.EncodeControllerResult(result)
				if err != nil {
					t.Fatal(err)
				}
				return encoded
			}},
			{name: "sequence", reply: func(call sealedexec.ControllerCall) []byte {
				call.CallSequence++
				return receiptAuthorityResultBytes(t, call, fixture.authority)
			}},
			{name: "result mismatch", reply: func(call sealedexec.ControllerCall) []byte {
				facts := fixture.authority
				facts.Isolation.ProfileDigest = receiptTestDigest("wrong-profile")
				return receiptAuthorityResultBytes(t, call, facts)
			}},
		} {
			t.Run(test.name, func(t *testing.T) {
				result, _ := runReceiptWithControllerReply(t, binary, fixture.requestBytes, test.reply)
				if result.exitCode != 2 || result.stdout != "" || !strings.HasPrefix(result.stderr, "context receipt:") {
					t.Fatalf("protocol fault result = %#v", result)
				}
			})
		}
	})
}

type receiptCLIFixture struct {
	requestBytes []byte
	request      contextreceipt.VerifyRequest
	authority    contextreceipt.AuthorityFacts
}

type receiptTrustReader struct{ fact gp.TrustFact }

func (r receiptTrustReader) ReadTrustFact(context.Context, gp.TrustSource, gp.PrincipalClaim) (gp.TrustFact, error) {
	return r.fact, nil
}

func newReceiptCLIFixture(t *testing.T) receiptCLIFixture {
	t.Helper()
	const source, subject = "ci-runner", "runner@example.com"
	profileDoc := gp.Profile{
		Schema: gp.SchemaID, ID: "project-profile", Class: gp.ClassSolo,
		ApplicableTransitions: []string{"context-receipt-verify"},
		IdentityTrustSources:  []gp.TrustSource{{ID: source, Kind: gp.TrustSourceIdentityProvider}},
		RoleMappings:          []gp.RoleMapping{{Role: "managed-runner", TrustSource: source, Subjects: []string{subject}}},
		OwnershipSources:      []gp.OwnershipSource{}, SignatureRequirements: []gp.SignatureRequirement{},
		RequiredApprovers: []gp.ApproverRequirement{}, DistinctnessRules: []gp.DistinctnessRule{},
		EvidenceSourceRestrictions: []gp.EvidenceSourceRestriction{}, EscalationThresholds: []gp.EscalationThreshold{},
	}
	profileBytes, err := canonjson.Marshal(profileDoc)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := gp.DecodeProfile(profileBytes, gp.Catalog{Roles: []string{"managed-runner"}, Transitions: []string{"context-receipt-verify"}, EvidenceSources: []string{source}})
	if err != nil {
		t.Fatalf("DecodeProfile fixture: %v", err)
	}
	profileDigest, err := profile.Digest()
	if err != nil {
		t.Fatal(err)
	}
	trustFact := gp.TrustFact{SourceID: source, SourceKind: gp.TrustSourceIdentityProvider, Subjects: []string{subject}, EvidenceDigest: receiptTestDigest("trust"), Available: true, Valid: true}
	resolution, err := gp.NewResolver(receiptTrustReader{fact: trustFact}).Resolve(context.Background(), profile, gp.PrincipalClaim{TrustSource: source, Subject: subject})
	if err != nil {
		t.Fatal(err)
	}

	treeBody := []byte{}
	treeOID := receiptGitOID("tree", treeBody)
	commitBody := []byte("tree " + treeOID + "\n\nfixture\n")
	commitOID := receiptGitOID("commit", commitBody)
	projection := sealedCanonicalProjection(t)
	manifest := sealedCanonicalManifest(t, projection, commitOID)
	workspace, err := sealedexec.NewExecutionWorkspaceRequest("flight-1", "lane-1", "epoch-1", "session-1", commitOID)
	if err != nil {
		t.Fatal(err)
	}
	execution := sealedexec.ExecutionRequest{
		Schema: sealedexec.ExecutionRequestSchemaID, Action: sealedexec.ActionStart,
		Flight: "flight-1", Lane: "lane-1", Epoch: "epoch-1", Session: "session-1",
		ManifestRevision: 0, ATCRunway: "/sealed/runway", InputCommit: commitOID, InputTree: treeOID,
		Manifest: manifest, ManifestDigest: manifest.Digest, InstructionProjection: projection, ProjectionDigest: projection.Digest,
		ExecutionWorkspaceRequest: workspace, Adapter: contextevent.AdapterCodex, AdapterVersion: "1.0.0",
		Profile: sealedexec.LogicalRef{Schema: sealedexec.ProjectProfileRefSchemaID, ID: profile.ID, Digest: profileDigest},
		Grants:  execworkspace.GrantSet{Grants: []execworkspace.Grant{}}, AuthorityVerdict: sealedCanonicalAuthorityReport(t, manifest.Digest, commitOID),
		RecorderEndpoint: sealedexec.LogicalRef{Schema: sealedexec.RecorderEndpointRefSchemaID, ID: "vatc-recorder", Digest: receiptTestDigest("recorder")},
		Start:            &sealedexec.StartArm{ExpectedSourceSequence: 1},
	}
	executionBytes, err := sealedexec.EncodeExecutionRequest(execution)
	if err != nil {
		t.Fatalf("EncodeExecutionRequest fixture: %v", err)
	}
	workspaceID, err := workspace.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(workspace)
	if err != nil {
		t.Fatal(err)
	}
	resultSchema, err := contextevent.PayloadSchema(contextevent.KindExecutionResult)
	if err != nil {
		t.Fatal(err)
	}
	executionEvent := contextevent.Event{
		Schema: contextevent.EventSchemaID, SourceSequence: 1, Flight: execution.Flight, Lane: execution.Lane, Epoch: execution.Epoch,
		ManifestRevision: 0, ManifestDigest: execution.ManifestDigest, Session: execution.Session, ATCRunway: execution.ATCRunway,
		ExecutionWorkspaceID: workspaceID, CandidateCommit: commitOID, CandidateTree: treeOID, Adapter: execution.Adapter, AdapterVersion: execution.AdapterVersion,
		OccurredAt: "2026-08-28T12:34:56Z", Kind: contextevent.KindExecutionResult, PayloadSchema: resultSchema,
		Payload:          &contextevent.ExecutionResultPayload{Schema: resultSchema, Authority: contextevent.AuthorityAuthoritative, InputCommit: commitOID, OutputCommit: commitOID, OutputTree: treeOID, Clean: true, ManifestDigest: execution.ManifestDigest, ResultFactsDigest: receiptTestDigest("result-facts")},
		PriorEventDigest: "",
	}
	executionEventBytes, err := contextevent.EncodeEvent(executionEvent)
	if err != nil {
		t.Fatalf("EncodeEvent execution result: %v", err)
	}
	executionEvent, err = contextevent.DecodeEvent(bytes.NewReader(executionEventBytes))
	if err != nil {
		t.Fatal(err)
	}
	revisions := []contextevent.Revision{{Schema: contextevent.RevisionSchemaID, ManifestRevision: 0, ManifestDigest: execution.ManifestDigest, FirstGlobalSequence: 1, TerminalGlobalSequence: 1, TerminalSourceSequence: 1, TerminalKind: contextevent.KindExecutionResult, EventRoot: executionEvent.EventDigest}}
	eventChainRoot, err := contextevent.EventChainRoot(revisions)
	if err != nil {
		t.Fatal(err)
	}
	receipt := contextreceipt.Receipt{
		Schema: contextreceipt.SchemaID, Role: contextreceipt.RoleBuilder, Authority: contextreceipt.AuthorityAuthoritative,
		ManifestDigest: execution.ManifestDigest, DispatchDigest: receiptRawDigest(executionBytes), ATCRunway: execution.ATCRunway,
		ExecutionWorkspaceRequestDigest: workspaceDigest, ExecutionWorkspaceID: workspaceID,
		InputCommit: commitOID, InputTree: treeOID, OutputCommit: commitOID, OutputTree: treeOID, Clean: true,
		RevisionSegments: revisions, EventChainRoot: eventChainRoot, TerminalManifestRevision: 0, TerminalSourceSequence: 1, TerminalGlobalSequence: 1,
		Expansions: []contextreceipt.Expansion{}, Obligations: []contextreceipt.Obligation{}, Evidence: []contextreceipt.Evidence{},
		RunnerPrincipalResolution: resolution, Adapter: execution.Adapter, AdapterVersion: execution.AdapterVersion,
		ReviewInputs: []contextreceipt.ReviewInput{},
	}
	receiptBytes, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		t.Fatalf("EncodeReceipt fixture: %v", err)
	}
	receipt, err = contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		t.Fatal(err)
	}
	receiptEventBytes, receiptEvent, ack := receiptCompletionForCLI(t, receipt, receiptBytes, execution)
	candidate := contextreceipt.Candidate{BaseCommit: commitOID, BaseTree: treeOID, HeadCommit: commitOID, HeadTree: treeOID}
	objects := []contextreceipt.RepositoryObject{{OID: commitOID, Type: "commit", Content: commitBody}, {OID: treeOID, Type: "tree", Content: treeBody}}
	sort.Slice(objects, func(i, j int) bool { return objects[i].OID < objects[j].OID })
	repositoryBytes, err := contextreceipt.EncodeRepositoryProof(contextreceipt.RepositoryProof{
		Schema: contextreceipt.RepositoryProofSchemaID, ObjectFormat: "sha1", Candidate: candidate, Objects: objects,
		ExecutionObservation: contextreceipt.ExecutionObservation{WorkspaceID: workspaceID, Commit: commitOID, Tree: treeOID, Clean: true, EventDigest: executionEvent.EventDigest},
	})
	if err != nil {
		t.Fatal(err)
	}
	requestBytes, err := contextreceipt.EncodeVerifyRequest(contextreceipt.VerifyRequest{
		Schema: contextreceipt.VerifyRequestSchemaID, Receipt: receipt, ReceiptEventAck: ack, Candidate: candidate,
		Proofs: contextreceipt.ProofBundle{ExecutionRequestBytes: executionBytes, RepositoryProofBytes: repositoryBytes, ExecutionEventBytes: [][]byte{executionEventBytes}, ReceiptEventBytes: receiptEventBytes, ExpansionDataBytes: [][]byte{}, ObligationBytes: [][]byte{}, EvidenceResultBytes: [][]byte{}, ReviewPacketBytes: []byte{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := contextreceipt.DecodeVerifyRequest(bytes.NewReader(requestBytes))
	if err != nil {
		t.Fatal(err)
	}
	ackDigest, err := canonjson.Digest(ack)
	if err != nil {
		t.Fatal(err)
	}
	authority := contextreceipt.AuthorityFacts{
		Profile: contextreceipt.ProfileAuthority{State: contextreceipt.StateProven, ProfileBytes: profileBytes, Witnesses: []contextreceipt.Witness{}}, TrustFact: trustFact,
		Isolation:   contextreceipt.IsolationAuthority{State: contextreceipt.StateProven, ProfileID: profile.ID, ProfileDigest: profileDigest, Session: execution.Session, WorkspaceID: workspaceID, Witnesses: []contextreceipt.Witness{}},
		Persistence: contextreceipt.PersistenceAuthority{State: contextreceipt.StateProven, ReceiptDigest: receipt.Digest, ReceiptEventDigest: receiptEvent.EventDigest, ReceiptAckDigest: ackDigest, Witnesses: []contextreceipt.Witness{}},
	}
	return receiptCLIFixture{requestBytes: requestBytes, request: request, authority: authority}
}

func receiptCompletionForCLI(t *testing.T, receipt contextreceipt.Receipt, receiptBytes []byte, execution sealedexec.ExecutionRequest) ([]byte, contextevent.Event, contextevent.ReceiptEventAck) {
	t.Helper()
	represented := bytes.TrimSuffix(receiptBytes, []byte("\n"))
	schema, err := contextevent.PayloadSchema(contextevent.KindReceipt)
	if err != nil {
		t.Fatal(err)
	}
	event := contextevent.Event{
		Schema: contextevent.EventSchemaID, SourceSequence: receipt.TerminalSourceSequence + 1,
		Flight: execution.Flight, Lane: execution.Lane, Epoch: execution.Epoch, ManifestRevision: receipt.TerminalManifestRevision, ManifestDigest: receipt.ManifestDigest,
		Session: execution.Session, ATCRunway: receipt.ATCRunway, ExecutionWorkspaceID: receipt.ExecutionWorkspaceID,
		CandidateCommit: receipt.OutputCommit, CandidateTree: receipt.OutputTree, Adapter: receipt.Adapter, AdapterVersion: receipt.AdapterVersion,
		OccurredAt: "2026-08-28T12:34:57Z", Kind: contextevent.KindReceipt, PayloadSchema: schema,
		Payload:          &contextevent.ReceiptPayload{Schema: schema, Role: receipt.Role, ReceiptDigest: receipt.Digest, Authority: receipt.Authority, ExecutionEventChainRoot: receipt.EventChainRoot, Detail: contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: receiptRawDigest(represented), RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: represented}},
		PriorEventDigest: receipt.RevisionSegments[len(receipt.RevisionSegments)-1].EventRoot,
	}
	encoded, err := contextevent.EncodeEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	event, err = contextevent.DecodeEvent(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	ack := contextevent.ReceiptEventAck{Schema: contextevent.ReceiptAckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch, Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind, SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: receipt.TerminalGlobalSequence + 1, ReceiptDigest: receipt.Digest}
	if _, err := contextevent.EncodeReceiptEventAck(ack); err != nil {
		t.Fatal(err)
	}
	return encoded, event, ack
}

func runReceiptBinary(t *testing.T, binary, dir string, stdin []byte, extraFiles []*os.File, args ...string) sealedContextObservation {
	t.Helper()
	command := exec.Command(binary, args...)
	command.Dir, command.Stdin, command.ExtraFiles = dir, bytes.NewReader(stdin), extraFiles
	command.Env = []string{"PATH=/definitely-unavailable", "HOME=/no-ambient-home"}
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Start()
	for _, file := range extraFiles {
		_ = file.Close()
	}
	if err == nil {
		err = command.Wait()
	}
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			t.Fatalf("run receipt binary: %v", err)
		}
		code = exitErr.ExitCode()
	}
	return sealedContextObservation{exitCode: code, stdout: stdout.String(), stderr: stderr.String()}
}

func runReceiptWithControllerReply(t *testing.T, binary string, request []byte, reply func(sealedexec.ControllerCall) []byte) (sealedContextObservation, sealedexec.ControllerCall) {
	t.Helper()
	return runWithControllerReply(t, binary, t.TempDir(), request, []string{"context", "receipt", "verify", "--request", "-"}, reply)
}

func receiptAuthorityResultBytes(t *testing.T, call sealedexec.ControllerCall, authority contextreceipt.AuthorityFacts) []byte {
	t.Helper()
	result := sealedexec.ControllerResult{
		Schema: sealedexec.ControllerResultSchemaID, CallSequence: call.CallSequence, Operation: sealedexec.ControllerOperationResolveReceiptVerificationAuthority,
		ResolveReceiptVerificationAuthority: sealedexec.ControllerResolveReceiptVerificationAuthorityResult{Schema: "verdi.context-controller/resolve-receipt-verification-authority-result/v1", Authority: authority},
	}
	encoded, err := sealedexec.EncodeControllerResult(result)
	if err != nil {
		t.Fatalf("EncodeControllerResult authority: %v", err)
	}
	return encoded
}

func receiptGitOID(kind string, body []byte) string {
	preimage := append([]byte(kind+" "+strconv.Itoa(len(body))+"\x00"), body...)
	sum := sha1.Sum(preimage)
	return hex.EncodeToString(sum[:])
}

func receiptRawDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func receiptTestDigest(label string) string { return receiptRawDigest([]byte(label)) }
