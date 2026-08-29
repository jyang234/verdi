package contextreceipt

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

func TestContextReceiptVerifyContract_Static(t *testing.T) {
	t.Parallel()

	if got, want := VerifyRequestSchemaID, "verdi.context-receipt-verify-request/v1"; got != want {
		t.Fatalf("VerifyRequestSchemaID = %q, want %q", got, want)
	}
	if got, want := VerdictSchemaID, "verdi.context-receipt-verdict/v1"; got != want {
		t.Fatalf("VerdictSchemaID = %q, want %q", got, want)
	}
	if got, want := RepositoryProofSchemaID, "verdi.context-repository-proof/v1"; got != want {
		t.Fatalf("RepositoryProofSchemaID = %q, want %q", got, want)
	}

	wantKinds := []OperandKind{
		"receipt", "candidate", "execution-request", "repository", "manifest",
		"dispatch", "events", "event-chain", "expansions", "obligations",
		"evidence", "receipt-event", "receipt-ack", "governance-profile",
		"runner", "isolation", "review-packet", "review-link", "freshness",
	}
	if !reflect.DeepEqual(OperandKinds(), wantKinds) {
		t.Fatalf("OperandKinds() = %#v, want independent literal %#v", OperandKinds(), wantKinds)
	}

	request := verifyRequestCodecFixture(t)
	encoded, err := EncodeVerifyRequest(request)
	if err != nil {
		t.Fatalf("EncodeVerifyRequest() error = %v", err)
	}
	decoded, err := DecodeVerifyRequest(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeVerifyRequest() error = %v", err)
	}
	reencoded, err := EncodeVerifyRequest(decoded)
	if err != nil {
		t.Fatalf("EncodeVerifyRequest(decoded) error = %v", err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Fatalf("verify request round trip changed bytes\nfirst: %s\nagain: %s", encoded, reencoded)
	}

	for name, mutated := range map[string][]byte{
		"unknown field":        bytes.Replace(encoded, []byte(`"schema":`), []byte(`"unknown":true,"schema":`), 1),
		"duplicate field":      bytes.Replace(encoded, []byte(`"schema":`), []byte(`"schema":"verdi.context-receipt-verify-request/v1","schema":`), 1),
		"null event documents": bytes.Replace(encoded, []byte(`"execution_event_bytes":["e30K"]`), []byte(`"execution_event_bytes":null`), 1),
		"trailing data":        append(append([]byte(nil), encoded...), []byte("{}")...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeVerifyRequest(bytes.NewReader(mutated)); err == nil {
				t.Fatal("DecodeVerifyRequest(mutated) error = nil")
			}
		})
	}

	verdict := Verdict{
		Schema: VerdictSchemaID, RequestDigest: decoded.Digest, ReceiptDigest: decoded.Receipt.Digest,
		ReceiptRole: decoded.Receipt.Role, ReceiptAuthority: decoded.Receipt.Authority,
		State: StateProven, Operands: operandFixture(StateProven),
		Findings: []Finding{}, Witnesses: []Witness{},
	}
	verdictBytes, err := EncodeVerdict(verdict)
	if err != nil {
		t.Fatalf("EncodeVerdict() error = %v", err)
	}
	if _, err := DecodeVerdict(bytes.NewReader(verdictBytes)); err != nil {
		t.Fatalf("DecodeVerdict() error = %v", err)
	}
}

func TestContextReceiptVerifyContract_Behavioral(t *testing.T) {
	t.Parallel()

	malformed := verifyRequestCodecFixture(t)
	verdict, err := NewVerifier(nil).Verify(context.Background(), malformed)
	if err == nil {
		t.Fatalf("Verify() with malformed declared proof bytes returned verdict %#v, want operational error", verdict)
	}
	var nilContext context.Context
	if _, err := NewVerifier(nil).Verify(nilContext, malformed); err == nil {
		t.Fatal("Verify(nil context) error = nil")
	}

	request, projection, authority := validVerifierRequestFixture(t)
	verifier := NewVerifierWithExecutionProof(nil, staticExecutionProof{projection: projection})
	verdict, err = verifier.Verify(context.Background(), request)
	if err != nil {
		t.Fatalf("Verify(valid declared proofs) error = %v", err)
	}
	if verdict.State != StateUnproven || len(verdict.Operands) != 19 {
		t.Fatalf("absent-authority verdict = %#v, want nineteen-operand unproven", verdict)
	}
	authoritative := NewVerifierWithExecutionProof(staticAuthorityResolver{facts: authority}, staticExecutionProof{projection: projection})
	verdict, err = authoritative.Verify(context.Background(), request)
	if err != nil {
		t.Fatalf("Verify(all-proven authority) error = %v", err)
	}
	if verdict.State != StateProven || len(verdict.Findings) != 0 || len(verdict.Witnesses) != 0 {
		t.Fatalf("all-proven verdict = %#v", verdict)
	}
	for _, kind := range []OperandKind{"review-packet", "review-link", "freshness"} {
		operand := verdict.Operands[operandKindIndex(kind)]
		if operand.State != StateProven || operand.ExpectedDigest != operand.ObservedDigest || len(operand.Witnesses) != 0 {
			t.Fatalf("builder %s operand = %#v, want exact empty-arm proof", kind, operand)
		}
	}

	reviewerRequest := reviewerVerifierRequestFixture(t, request)
	reviewerVerdict, err := authoritative.Verify(context.Background(), reviewerRequest)
	if err != nil {
		t.Fatalf("Verify(reviewer without review proof verifier) error = %v", err)
	}
	for _, kind := range []OperandKind{"review-packet", "review-link", "freshness"} {
		operand := reviewerVerdict.Operands[operandKindIndex(kind)]
		if operand.State != StateUnproven || len(operand.Witnesses) != 1 {
			t.Fatalf("reviewer %s operand = %#v, want disclosed unproven", kind, operand)
		}
	}
	reviewDigest := digestRaw([]byte("review projection"))
	reviewPort := staticReviewProof{projection: ReviewProofProjection{
		Packet:    ReviewOperandProjection{State: StateProven, ExpectedDigest: reviewDigest, ObservedDigest: reviewDigest},
		Link:      ReviewOperandProjection{State: StateProven, ExpectedDigest: reviewDigest, ObservedDigest: reviewDigest},
		Freshness: ReviewOperandProjection{State: StateProven, ExpectedDigest: reviewDigest, ObservedDigest: reviewDigest},
	}}
	if _, err := (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest); err != nil {
		t.Fatalf("matching review proof projection error = %v", err)
	}
	reviewPort.projection.Packet.ObservedDigest = receiptDigestA
	if _, err := (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest); err == nil {
		t.Fatal("contradictory proven review proof projection error = nil")
	}
	reviewPort.err = errors.New("broken review port")
	if _, err := (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest); err == nil {
		t.Fatal("broken review proof port error = nil")
	}

	staleCandidate := request
	staleCandidate.Candidate.HeadCommit = verifierGitOID("commit", []byte("other candidate"))
	staleCandidate.Digest = ""
	verdict, err = authoritative.Verify(context.Background(), staleCandidate)
	if err != nil {
		t.Fatalf("Verify(stale candidate) operational error = %v", err)
	}
	candidateOperand := verdict.Operands[operandKindIndex("candidate")]
	if candidateOperand.State != StateViolated || candidateOperand.ExpectedDigest == candidateOperand.ObservedDigest {
		t.Fatalf("stale candidate operand = %#v, want distinct expected and observed witnesses", candidateOperand)
	}

	wrongAck := request
	wrongAck.ReceiptEventAck.ReceiptDigest = receiptDigestA
	wrongAck.Digest = ""
	verdict, err = verifier.Verify(context.Background(), wrongAck)
	if err != nil {
		t.Fatalf("Verify(contradictory receipt acknowledgment) operational error = %v", err)
	}
	if operand := verdict.Operands[operandKindIndex("receipt-ack")]; operand.State != StateViolated {
		t.Fatalf("receipt-ack operand = %#v, want local contradiction preserved despite unavailable persistence", operand)
	}

	proof, err := DecodeRepositoryProof(bytes.NewReader(request.Proofs.RepositoryProofBytes))
	if err != nil {
		t.Fatal(err)
	}
	proof.Digest = ""
	proof.Objects[0].Content = append(proof.Objects[0].Content, 'x')
	request.Proofs.RepositoryProofBytes, err = EncodeRepositoryProof(proof)
	if err != nil {
		t.Fatal(err)
	}
	request.Digest = ""
	verdict, err = verifier.Verify(context.Background(), request)
	if err != nil {
		t.Fatalf("Verify(contradictory repository proof) operational error = %v", err)
	}
	if verdict.State != StateViolated || verdict.Operands[operandKindIndex("repository")].State != StateViolated {
		t.Fatalf("repository contradiction verdict = %#v", verdict)
	}

	expansion := Expansion{RequestID: "request", ParentRevision: 0, ParentManifestDigest: receiptDigestA, ChildRevision: 1, ChildManifestDigest: receiptDigestB, ExpansionDigest: receiptDigestC}
	expansionPort := staticExpansionProof{projection: ExpansionProofProjection{DataItemDigest: receiptDigestA, DataDigest: receiptDigestB, ExpansionDigest: receiptDigestC}}
	if state, err := (&Verifier{expansion: expansionPort}).verifyExpansionDocuments([][]byte{[]byte("proof")}, []Expansion{expansion}); err != nil || state != StateProven {
		t.Fatalf("matching expansion proof = %q/%v", state, err)
	}
	expansionPort.projection.ExpansionDigest = receiptDigestA
	if state, err := (&Verifier{expansion: expansionPort}).verifyExpansionDocuments([][]byte{[]byte("proof")}, []Expansion{expansion}); err != nil || state != StateViolated {
		t.Fatalf("contradictory expansion proof = %q/%v", state, err)
	}
	if _, err := (&Verifier{}).verifyExpansionDocuments([][]byte{[]byte("proof")}, []Expansion{expansion}); err == nil {
		t.Fatal("missing expansion proof port error = nil")
	}
	expansionPort.err = errors.New("broken expansion port")
	if _, err := (&Verifier{expansion: expansionPort}).verifyExpansionDocuments([][]byte{[]byte("proof")}, []Expansion{expansion}); err == nil {
		t.Fatal("broken expansion proof port error = nil")
	}
	if _, err := (&Verifier{expansion: expansionPort}).verifyExpansionDocuments([][]byte{[]byte("malformed extra proof")}, []Expansion{}); err == nil {
		t.Fatal("malformed extra expansion proof error = nil")
	}
	if _, err := verifyObligationDocuments([][]byte{[]byte("{}\n")}, []Obligation{}); err == nil {
		t.Fatal("malformed extra obligation proof error = nil")
	}
	if _, err := verifyEvidenceDocuments([][]byte{[]byte("not json")}, []Evidence{}); err == nil {
		t.Fatal("malformed extra evidence proof error = nil")
	}
}

type staticExecutionProof struct{ projection ExecutionProjection }

func (f staticExecutionProof) DecodeExecutionProof([]byte) (ExecutionProjection, error) {
	return f.projection, nil
}

type staticAuthorityResolver struct {
	facts AuthorityFacts
	err   error
}

func (r staticAuthorityResolver) ResolveReceiptVerificationAuthority(context.Context, AuthorityQuery) (AuthorityFacts, error) {
	return r.facts, r.err
}

type staticExpansionProof struct {
	projection ExpansionProofProjection
	err        error
}

type staticReviewProof struct {
	projection ReviewProofProjection
	err        error
}

func (f staticReviewProof) VerifyReviewProof([]byte, Receipt, Candidate) (ReviewProofProjection, error) {
	return f.projection, f.err
}

func (f staticExpansionProof) VerifyExpansionProof([]byte, Expansion) (ExpansionProofProjection, error) {
	return f.projection, f.err
}

func validVerifierRequestFixture(t *testing.T) (VerifyRequest, ExecutionProjection, AuthorityFacts) {
	t.Helper()
	treeBody := []byte{}
	treeOID := verifierGitOID("tree", treeBody)
	commitBody := []byte("tree " + treeOID + "\n\nfixture\n")
	commitOID := verifierGitOID("commit", commitBody)
	executionBytes := []byte("{\"fixture\":true}\n")

	receipt := receiptFixture(t, RoleBuilder)
	receipt.ManifestDigest = receiptDigestA
	receipt.DispatchDigest = digestRaw(executionBytes)
	receipt.InputCommit, receipt.InputTree = commitOID, treeOID
	receipt.OutputCommit, receipt.OutputTree = commitOID, treeOID
	receipt.Expansions = []Expansion{}
	receipt.Obligations = []Obligation{}
	receipt.Evidence = []Evidence{}
	receipt.ReviewInputs = []ReviewInput{}
	claim := receipt.RunnerPrincipalResolution.Claim
	profileDoc := gp.Profile{
		Schema: gp.SchemaID, ID: "project-profile", Class: gp.ClassSolo,
		ApplicableTransitions: []string{receiptVerificationTransition},
		IdentityTrustSources:  []gp.TrustSource{{ID: claim.TrustSource, Kind: gp.TrustSourceIdentityProvider}},
		RoleMappings:          []gp.RoleMapping{{Role: receiptVerificationRole, TrustSource: claim.TrustSource, Subjects: []string{claim.Subject}}},
		OwnershipSources:      []gp.OwnershipSource{}, SignatureRequirements: []gp.SignatureRequirement{},
		RequiredApprovers: []gp.ApproverRequirement{}, DistinctnessRules: []gp.DistinctnessRule{},
		EvidenceSourceRestrictions: []gp.EvidenceSourceRestriction{}, EscalationThresholds: []gp.EscalationThreshold{},
	}
	profileBytes, err := canonjson.Marshal(profileDoc)
	if err != nil {
		t.Fatal(err)
	}
	profile, err := gp.DecodeProfile(profileBytes, gp.Catalog{Roles: []string{receiptVerificationRole}, Transitions: []string{receiptVerificationTransition}, EvidenceSources: []string{claim.TrustSource}})
	if err != nil {
		t.Fatal(err)
	}
	profileDigest, err := profile.Digest()
	if err != nil {
		t.Fatal(err)
	}
	trustFact := gp.TrustFact{SourceID: claim.TrustSource, SourceKind: gp.TrustSourceIdentityProvider, Subjects: []string{claim.Subject}, EvidenceDigest: receiptDigestA, Available: true, Valid: true}
	receipt.RunnerPrincipalResolution, err = gp.NewResolver(oneTrustFactReader{fact: trustFact}).Resolve(context.Background(), profile, claim)
	if err != nil {
		t.Fatal(err)
	}

	schema, err := contextevent.PayloadSchema(contextevent.KindExecutionResult)
	if err != nil {
		t.Fatal(err)
	}
	executionEvent := contextevent.Event{
		Schema: contextevent.EventSchemaID, SourceSequence: 1, Flight: "flight-1", Lane: "builder", Epoch: "epoch-1",
		ManifestRevision: 0, ManifestDigest: receipt.ManifestDigest, Session: "session-1", ATCRunway: receipt.ATCRunway,
		ExecutionWorkspaceID: receipt.ExecutionWorkspaceID, CandidateCommit: commitOID, CandidateTree: treeOID,
		Adapter: receipt.Adapter, AdapterVersion: receipt.AdapterVersion, OccurredAt: "2026-08-28T12:34:56Z",
		Kind: contextevent.KindExecutionResult, PayloadSchema: schema,
		Payload:          &contextevent.ExecutionResultPayload{Schema: schema, Authority: contextevent.AuthorityAuthoritative, InputCommit: commitOID, OutputCommit: commitOID, OutputTree: treeOID, Clean: true, ManifestDigest: receipt.ManifestDigest, ResultFactsDigest: receiptDigestB},
		PriorEventDigest: "",
	}
	executionEventBytes, err := contextevent.EncodeEvent(executionEvent)
	if err != nil {
		t.Fatal(err)
	}
	executionEvent, err = contextevent.DecodeEvent(bytes.NewReader(executionEventBytes))
	if err != nil {
		t.Fatal(err)
	}
	receipt.RevisionSegments = []contextevent.Revision{{Schema: contextevent.RevisionSchemaID, ManifestRevision: 0, ManifestDigest: receipt.ManifestDigest, FirstGlobalSequence: 1, TerminalGlobalSequence: 1, TerminalSourceSequence: 1, TerminalKind: contextevent.KindExecutionResult, EventRoot: executionEvent.EventDigest}}
	receipt.EventChainRoot, err = contextevent.EventChainRoot(receipt.RevisionSegments)
	if err != nil {
		t.Fatal(err)
	}
	receipt.TerminalManifestRevision, receipt.TerminalSourceSequence, receipt.TerminalGlobalSequence = 0, 1, 1
	receiptBytes, err := EncodeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		t.Fatal(err)
	}
	receiptEventBytes, ack := receiptCompletionFixture(t, receipt, receiptBytes)
	receiptEvent, err := contextevent.DecodeEvent(bytes.NewReader(receiptEventBytes))
	if err != nil {
		t.Fatal(err)
	}
	candidate := Candidate{BaseCommit: commitOID, BaseTree: treeOID, HeadCommit: commitOID, HeadTree: treeOID}
	objects := []RepositoryObject{{OID: commitOID, Type: "commit", Content: commitBody}, {OID: treeOID, Type: "tree", Content: treeBody}}
	sort.Slice(objects, func(i, j int) bool { return objects[i].OID < objects[j].OID })
	repositoryBytes, err := EncodeRepositoryProof(RepositoryProof{
		Schema: RepositoryProofSchemaID, ObjectFormat: "sha1", Candidate: candidate, Objects: objects,
		ExecutionObservation: ExecutionObservation{WorkspaceID: receipt.ExecutionWorkspaceID, Commit: commitOID, Tree: treeOID, Clean: true, EventDigest: executionEvent.EventDigest},
	})
	if err != nil {
		t.Fatal(err)
	}
	requestBytes, err := EncodeVerifyRequest(VerifyRequest{
		Schema: VerifyRequestSchemaID, Receipt: receipt, ReceiptEventAck: ack, Candidate: candidate,
		Proofs: ProofBundle{ExecutionRequestBytes: executionBytes, RepositoryProofBytes: repositoryBytes, ExecutionEventBytes: [][]byte{executionEventBytes}, ReceiptEventBytes: receiptEventBytes, ExpansionDataBytes: [][]byte{}, ObligationBytes: [][]byte{}, EvidenceResultBytes: [][]byte{}, ReviewPacketBytes: []byte{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := DecodeVerifyRequest(bytes.NewReader(requestBytes))
	if err != nil {
		t.Fatal(err)
	}
	projection := ExecutionProjection{
		Flight: "flight-1", Lane: "builder", Epoch: "epoch-1", Session: "session-1", ATCRunway: receipt.ATCRunway,
		ManifestRevision: 0, ManifestDigest: receipt.ManifestDigest, InputCommit: commitOID, InputTree: treeOID,
		ExecutionWorkspaceRequestDigest: receipt.ExecutionWorkspaceRequestDigest, Adapter: receipt.Adapter, AdapterVersion: receipt.AdapterVersion,
		ProfileRef: ProfileRef{Schema: "verdi.project-profile-ref/v1", ID: profile.ID, Digest: profileDigest},
	}
	ackDigest, err := canonjson.Digest(ack)
	if err != nil {
		t.Fatal(err)
	}
	authority := AuthorityFacts{
		Profile: ProfileAuthority{State: StateProven, ProfileBytes: profileBytes, Witnesses: []Witness{}}, TrustFact: trustFact,
		Isolation:   IsolationAuthority{State: StateProven, ProfileID: profile.ID, ProfileDigest: profileDigest, Session: projection.Session, WorkspaceID: receipt.ExecutionWorkspaceID, Witnesses: []Witness{}},
		Persistence: PersistenceAuthority{State: StateProven, ReceiptDigest: receipt.Digest, ReceiptEventDigest: receiptEvent.EventDigest, ReceiptAckDigest: ackDigest, Witnesses: []Witness{}},
	}
	return request, projection, authority
}

func verifierGitOID(kind string, content []byte) string {
	preimage := append([]byte(kind+" "+strconv.Itoa(len(content))+"\x00"), content...)
	sum := sha1.Sum(preimage)
	return hex.EncodeToString(sum[:])
}

func reviewerVerifierRequestFixture(t *testing.T, request VerifyRequest) VerifyRequest {
	t.Helper()
	receipt := request.Receipt
	receipt.Role = RoleReviewer
	receipt.ReviewOf = []string{receiptDigestA}
	receipt.Digest = ""
	receiptBytes, err := EncodeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		t.Fatal(err)
	}
	receiptEventBytes, receiptAck := receiptCompletionFixture(t, receipt, receiptBytes)
	request.Receipt = receipt
	request.ReceiptEventAck = receiptAck
	request.Proofs.ReceiptEventBytes = receiptEventBytes
	request.Proofs.ReviewPacketBytes = []byte("{}\n")
	request.Digest = ""
	requestBytes, err := EncodeVerifyRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	request, err = DecodeVerifyRequest(bytes.NewReader(requestBytes))
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func verifyRequestCodecFixture(t *testing.T) VerifyRequest {
	t.Helper()
	receipt := receiptFixture(t, RoleBuilder)
	receiptBytes, err := EncodeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		t.Fatal(err)
	}
	_, ack := receiptCompletionFixture(t, receipt, receiptBytes)
	return VerifyRequest{
		Schema:          VerifyRequestSchemaID,
		Receipt:         receipt,
		ReceiptEventAck: ack,
		Candidate:       Candidate{BaseCommit: receipt.InputCommit, BaseTree: receipt.InputTree, HeadCommit: receipt.OutputCommit, HeadTree: receipt.OutputTree},
		Proofs: ProofBundle{
			ExecutionRequestBytes: []byte("{}\n"), RepositoryProofBytes: []byte("{}\n"),
			ExecutionEventBytes: [][]byte{[]byte("{}\n")}, ReceiptEventBytes: []byte("{}\n"),
			ExpansionDataBytes: [][]byte{}, ObligationBytes: [][]byte{},
			EvidenceResultBytes: [][]byte{}, ReviewPacketBytes: []byte{},
		},
	}
}

func operandFixture(state State) []Operand {
	rows := make([]Operand, 0, 19)
	for _, kind := range OperandKinds() {
		rows = append(rows, Operand{Kind: kind, ID: string(kind), State: state, ExpectedDigest: receiptDigestA, ObservedDigest: receiptDigestA, Witnesses: []Witness{}})
	}
	return rows
}
