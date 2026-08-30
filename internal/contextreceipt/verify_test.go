package contextreceipt

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/countersign"
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
	if decoded.Proofs.ExecutionEventAckBytes == nil || len(decoded.Proofs.ExecutionEventAckBytes) != len(decoded.Proofs.ExecutionEventBytes) {
		t.Fatalf("execution acknowledgment proofs = %#v, want non-null parallel array", decoded.Proofs.ExecutionEventAckBytes)
	}
	for i, raw := range decoded.Proofs.ExecutionEventAckBytes {
		if _, err := contextevent.DecodeEventAck(bytes.NewReader(raw)); err != nil {
			t.Fatalf("execution acknowledgment proof[%d] is not an exact canonical EventAck: %v", i, err)
		}
	}

	invalidShapes := []struct {
		name   string
		mutate func(*VerifyRequest)
	}{
		{name: "nil event acknowledgments", mutate: func(r *VerifyRequest) { r.Proofs.ExecutionEventAckBytes = nil }},
		{name: "short event acknowledgments", mutate: func(r *VerifyRequest) { r.Proofs.ExecutionEventAckBytes = [][]byte{} }},
		{name: "empty event acknowledgment", mutate: func(r *VerifyRequest) { r.Proofs.ExecutionEventAckBytes[0] = []byte{} }},
	}
	for _, tt := range invalidShapes {
		t.Run(tt.name, func(t *testing.T) {
			invalid := verifyRequestCodecFixture(t)
			tt.mutate(&invalid)
			if _, err := EncodeVerifyRequest(invalid); err == nil {
				t.Fatal("EncodeVerifyRequest(invalid acknowledgment shape) error = nil")
			}
		})
	}
	ackArray, err := json.Marshal(decoded.Proofs.ExecutionEventAckBytes)
	if err != nil {
		t.Fatal(err)
	}
	ackField := append([]byte(`"execution_event_ack_bytes":`), ackArray...)
	nullAcknowledgments := bytes.Replace(encoded, ackField, []byte(`"execution_event_ack_bytes":null`), 1)
	missingAcknowledgments := bytes.Replace(encoded, append([]byte(","), ackField...), nil, 1)
	if bytes.Equal(nullAcknowledgments, encoded) || bytes.Equal(missingAcknowledgments, encoded) {
		t.Fatal("failed to construct exact acknowledgment-field mutations")
	}

	for name, mutated := range map[string][]byte{
		"unknown field":              bytes.Replace(encoded, []byte(`"schema":`), []byte(`"unknown":true,"schema":`), 1),
		"duplicate field":            bytes.Replace(encoded, []byte(`"schema":`), []byte(`"schema":"verdi.context-receipt-verify-request/v1","schema":`), 1),
		"null event documents":       bytes.Replace(encoded, []byte(`"execution_event_bytes":["e30K"]`), []byte(`"execution_event_bytes":null`), 1),
		"null event acknowledgments": nullAcknowledgments,
		"missing acknowledgments":    missingAcknowledgments,
		"trailing data":              append(append([]byte(nil), encoded...), []byte("{}")...),
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

	for _, tt := range []struct {
		name   string
		mutate func(*contextevent.EventAck)
	}{
		{name: "first revision endpoint", mutate: func(ack *contextevent.EventAck) { ack.GlobalSequence++ }},
		{name: "terminal revision endpoint", mutate: func(ack *contextevent.EventAck) { ack.GlobalSequence-- }},
		{name: "global order", mutate: func(ack *contextevent.EventAck) { ack.GlobalSequence = 11 }},
	} {
		t.Run(tt.name+" contradiction is a verdict", func(t *testing.T) {
			contradictory := request
			ackIndex := 0
			if tt.name != "first revision endpoint" {
				ackIndex = len(contradictory.Proofs.ExecutionEventAckBytes) - 1
			}
			ack, err := contextevent.DecodeEventAck(bytes.NewReader(contradictory.Proofs.ExecutionEventAckBytes[ackIndex]))
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(&ack)
			contradictory.Proofs.ExecutionEventAckBytes = cloneByteDocuments(contradictory.Proofs.ExecutionEventAckBytes)
			contradictory.Proofs.ExecutionEventAckBytes[ackIndex], err = contextevent.EncodeEventAck(ack)
			if err != nil {
				t.Fatal(err)
			}
			contradictory.Digest = ""
			verdict, err := authoritative.Verify(context.Background(), contradictory)
			if err != nil {
				t.Fatalf("Verify(valid contradictory acknowledgments) operational error = %v", err)
			}
			for _, kind := range []OperandKind{"events", "event-chain"} {
				operand := verdict.Operands[operandKindIndex(kind)]
				if operand.State != StateViolated {
					t.Fatalf("%s operand = %#v, want acknowledgment contradiction witness", kind, operand)
				}
				if kind == "event-chain" && operand.ExpectedDigest == operand.ObservedDigest {
					t.Fatalf("event-chain operand = %#v, want independently observed acknowledgment endpoints", operand)
				}
			}
		})
	}

	t.Run("receipt acknowledgment follows observed execution terminal", func(t *testing.T) {
		contradictory := request
		contradictory.Proofs.ExecutionEventAckBytes = cloneByteDocuments(contradictory.Proofs.ExecutionEventAckBytes)
		last := len(contradictory.Proofs.ExecutionEventAckBytes) - 1
		ack, err := contextevent.DecodeEventAck(bytes.NewReader(contradictory.Proofs.ExecutionEventAckBytes[last]))
		if err != nil {
			t.Fatal(err)
		}
		ack.GlobalSequence = contradictory.ReceiptEventAck.GlobalSequence
		contradictory.Proofs.ExecutionEventAckBytes[last], err = contextevent.EncodeEventAck(ack)
		if err != nil {
			t.Fatal(err)
		}
		contradictory.Digest = ""
		verdict, err := authoritative.Verify(context.Background(), contradictory)
		if err != nil {
			t.Fatalf("Verify(receipt acknowledgment at observed execution terminal) error = %v", err)
		}
		if operand := verdict.Operands[operandKindIndex("receipt-ack")]; operand.State != StateViolated {
			t.Fatalf("receipt-ack operand = %#v, want strict post-execution order violation", operand)
		}
	})

	t.Run("malformed event acknowledgment is operational", func(t *testing.T) {
		invalid := request
		invalid.Proofs.ExecutionEventAckBytes = cloneByteDocuments(invalid.Proofs.ExecutionEventAckBytes)
		invalid.Proofs.ExecutionEventAckBytes[0] = []byte("{}\n")
		invalid.Digest = ""
		if verdict, err := authoritative.Verify(context.Background(), invalid); err == nil {
			t.Fatalf("Verify(malformed acknowledgment) returned verdict %#v, want operational error", verdict)
		}
	})

	for _, tt := range []struct {
		name   string
		mutate func(*contextevent.EventAck)
	}{
		{name: "flight", mutate: func(ack *contextevent.EventAck) { ack.Flight += "-other" }},
		{name: "lane", mutate: func(ack *contextevent.EventAck) { ack.Lane += "-other" }},
		{name: "epoch", mutate: func(ack *contextevent.EventAck) { ack.Epoch += "-other" }},
		{name: "session", mutate: func(ack *contextevent.EventAck) { ack.Session += "-other" }},
		{name: "manifest revision", mutate: func(ack *contextevent.EventAck) { ack.ManifestRevision++ }},
		{name: "kind", mutate: func(ack *contextevent.EventAck) { ack.Kind = contextevent.KindWrite }},
		{name: "source sequence", mutate: func(ack *contextevent.EventAck) { ack.SourceSequence++ }},
		{name: "event digest", mutate: func(ack *contextevent.EventAck) { ack.EventDigest = receiptDigestA }},
	} {
		t.Run("mispaired "+tt.name+" is operational", func(t *testing.T) {
			invalid := request
			invalid.Proofs.ExecutionEventAckBytes = cloneByteDocuments(invalid.Proofs.ExecutionEventAckBytes)
			ack, err := contextevent.DecodeEventAck(bytes.NewReader(invalid.Proofs.ExecutionEventAckBytes[0]))
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(&ack)
			invalid.Proofs.ExecutionEventAckBytes[0], err = contextevent.EncodeEventAck(ack)
			if err != nil {
				t.Fatal(err)
			}
			invalid.Digest = ""
			if verdict, err := authoritative.Verify(context.Background(), invalid); err == nil {
				t.Fatalf("Verify(mispaired %s acknowledgment) returned verdict %#v, want operational error", tt.name, verdict)
			}
		})
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
	}, wantRaw: append([]byte(nil), reviewerRequest.Proofs.ReviewPacketBytes...), wantReceiptDigest: reviewerRequest.Receipt.Digest, wantCandidate: reviewerRequest.Candidate}
	reviewerRepository, err := DecodeRepositoryProof(bytes.NewReader(reviewerRequest.Proofs.RepositoryProofBytes))
	if err != nil {
		t.Fatal(err)
	}
	reviewPort.wantRepository = &reviewerRepository
	projectionResult, err := (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest, ExecutionProjection{}, nil, nil, reviewerRepository)
	if err != nil {
		t.Fatalf("matching review proof projection error = %v", err)
	}
	for name, projection := range map[string]ReviewOperandProjection{"packet": projectionResult.Packet, "link": projectionResult.Link, "freshness": projectionResult.Freshness} {
		if projection.State != StateProven || projection.ExpectedDigest != projection.ObservedDigest {
			t.Fatalf("matching reviewer %s projection = %#v, want proven", name, projection)
		}
	}
	launchEvent := contextevent.Event{Kind: contextevent.KindAdapterStart, EventDigest: receiptDigestA}
	launchAck := contextevent.EventAck{Kind: contextevent.KindAdapterStart, EventDigest: receiptDigestA}
	launchExecution := ExecutionProjection{Flight: "flight-review"}
	wantLaunch := ReviewLaunchProof{Present: true, Execution: launchExecution, Event: launchEvent, Ack: launchAck}
	reviewPort.wantLaunch = &wantLaunch
	if _, err := (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest, launchExecution, []contextevent.Event{launchEvent}, []contextevent.EventAck{launchAck}, reviewerRepository); err != nil {
		t.Fatalf("acknowledged launch proof selection error = %v", err)
	}
	reviewPort.wantLaunch = nil
	reviewPort.projection.Freshness = ReviewOperandProjection{State: StateViolated, ExpectedDigest: reviewDigest, ObservedDigest: receiptDigestA}
	projectionResult, err = (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest, ExecutionProjection{}, nil, nil, reviewerRepository)
	if err != nil {
		t.Fatalf("stale review proof projection error = %v", err)
	}
	if projectionResult.Freshness.State != StateViolated || projectionResult.Freshness.ExpectedDigest == projectionResult.Freshness.ObservedDigest {
		t.Fatalf("stale reviewer freshness projection = %#v, want violated", projectionResult.Freshness)
	}
	reviewPort.projection.Freshness = ReviewOperandProjection{State: StateProven, ExpectedDigest: reviewDigest, ObservedDigest: reviewDigest}
	reviewPort.projection.Link = ReviewOperandProjection{State: StateViolated, ExpectedDigest: reviewDigest, ObservedDigest: receiptDigestA}
	projectionResult, err = (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest, ExecutionProjection{}, nil, nil, reviewerRepository)
	if err != nil {
		t.Fatalf("wrong-link review proof projection error = %v", err)
	}
	if projectionResult.Link.State != StateViolated || projectionResult.Link.ExpectedDigest == projectionResult.Link.ObservedDigest {
		t.Fatalf("wrong-link reviewer projection = %#v, want violated", projectionResult.Link)
	}
	reviewPort.projection.Link = ReviewOperandProjection{State: StateProven, ExpectedDigest: reviewDigest, ObservedDigest: reviewDigest}
	reviewPort.projection.Packet.ObservedDigest = receiptDigestA
	if _, err := (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest, ExecutionProjection{}, nil, nil, reviewerRepository); err == nil {
		t.Fatal("contradictory proven review proof projection error = nil")
	}
	reviewPort.err = errors.New("broken review port")
	if _, err := (&Verifier{review: reviewPort}).reviewProjection(reviewerRequest, ExecutionProjection{}, nil, nil, reviewerRepository); err == nil {
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
	if _, _, err := verifyObligationDocuments([][]byte{[]byte("{}\n")}, []Obligation{}, RepositoryProof{}); err == nil {
		t.Fatal("malformed extra obligation proof error = nil")
	}
	if _, _, err := (&Verifier{evidence: staticEvidenceProof{err: errors.New("malformed evidence result")}}).verifyEvidenceDocuments([][]byte{[]byte("not json")}, []Evidence{}); err == nil {
		t.Fatal("malformed extra evidence proof error = nil")
	}

	t.Run("complete nonempty obligation and evidence rows are derived from owner proofs", func(t *testing.T) {
		obligationBytes := []byte(`---
id: obligation/example--ac-1--behavioral
kind: obligation
title: "Example behavioral obligation"
owners: [platform-team]
for_kind: behavioral
quality:
  state: elaborated
  claim: "The behavioral producer proves the complete receipt row."
  falsifier: "Any copied receipt field can make a stale proof pass."
  scope: "The exact obligation document and authenticated candidate tree."
  producer: {kind: test, ref: "go-test:internal/example:TestBehavioral"}
  authoritative_source: {kind: ci-job, ref: "verify"}
  freshness:
    invalidated_by: [spec, code]
    rule: "Rerun at the exact candidate commit."
links:
  - {type: verifies, ref: "spec/example"}
frozen: {at: 2026-08-29, commit: abcdef0}
---
# Example behavioral obligation
`)
		wantObligation := Obligation{
			Ref: "obligation/example--ac-1--behavioral", Path: ".verdi/obligations/example/ac-1--behavioral.md",
			AC: "ac-1", Kind: artifact.EvidenceBehavioral, ContentDigest: digestRaw(obligationBytes),
			Producer: "go-test:internal/example:TestBehavioral",
		}
		repository := obligationRepositoryProofFixture(t, wantObligation.Path, obligationBytes)
		state, observed, err := verifyObligationDocuments([][]byte{obligationBytes}, []Obligation{wantObligation}, repository)
		if err != nil || state != StateProven || !reflect.DeepEqual(observed, []Obligation{wantObligation}) {
			t.Fatalf("complete obligation proof = %q/%#v/%v, want exact derived row", state, observed, err)
		}
		obligationMutations := []struct {
			name   string
			mutate func(*Obligation)
		}{
			{name: "ref", mutate: func(row *Obligation) { row.Ref = "obligation/other--ac-1--behavioral" }},
			{name: "path", mutate: func(row *Obligation) { row.Path = ".verdi/obligations/other/ac-1--behavioral.md" }},
			{name: "ac", mutate: func(row *Obligation) { row.AC = "ac-2" }},
			{name: "kind", mutate: func(row *Obligation) { row.Kind = artifact.EvidenceStatic }},
			{name: "content digest", mutate: func(row *Obligation) { row.ContentDigest = receiptDigestA }},
			{name: "producer", mutate: func(row *Obligation) { row.Producer = "go-test:internal/example:TestOther" }},
		}
		for _, mutation := range obligationMutations {
			t.Run("obligation "+mutation.name, func(t *testing.T) {
				declared := wantObligation
				mutation.mutate(&declared)
				state, observed, err := verifyObligationDocuments([][]byte{obligationBytes}, []Obligation{declared}, repository)
				if err != nil || state != StateViolated {
					t.Fatalf("mutated obligation proof = %q/%v, want violated", state, err)
				}
				if !reflect.DeepEqual(observed, []Obligation{wantObligation}) {
					t.Fatalf("observed obligation = %#v, want document-derived %#v", observed, []Obligation{wantObligation})
				}
			})
		}
		wrongTree := obligationRepositoryProofFixture(t, wantObligation.Path, []byte("different blob bytes\n"))
		if state, _, err := verifyObligationDocuments([][]byte{obligationBytes}, []Obligation{wantObligation}, wrongTree); err != nil || state != StateViolated {
			t.Fatalf("wrong head-tree blob proof = %q/%v, want violated", state, err)
		}
		unelaboratedBytes := []byte(`---
id: obligation/example--ac-1--behavioral
kind: obligation
title: "Example unresolved obligation"
owners: [platform-team]
for_kind: behavioral
quality:
  state: unresolved-design-debt
links:
  - {type: verifies, ref: "spec/example"}
frozen: {at: 2026-08-29, commit: abcdef0}
---
# Example unresolved obligation
`)
		unelaboratedDeclared := wantObligation
		unelaboratedDeclared.ContentDigest = digestRaw(unelaboratedBytes)
		unelaboratedRepository := obligationRepositoryProofFixture(t, wantObligation.Path, unelaboratedBytes)
		if state, _, err := verifyObligationDocuments([][]byte{unelaboratedBytes}, []Obligation{unelaboratedDeclared}, unelaboratedRepository); err != nil || state != StateViolated {
			t.Fatalf("unelaborated obligation proof = %q/%v, want violated", state, err)
		}

		wantEvidence := Evidence{
			CommandID: "verify", Argv: []string{"make", "verify"}, ExitCode: 0,
			Verdict: countersign.VerdictProven, OutputDigest: digestRaw([]byte("verified\n")),
		}
		evidencePort := staticEvidenceProof{projection: EvidenceProofProjection{
			CommandID: wantEvidence.CommandID, Argv: append([]string(nil), wantEvidence.Argv...),
			ExitCode: wantEvidence.ExitCode, Verdict: wantEvidence.Verdict, OutputDigest: wantEvidence.OutputDigest,
		}}
		state, observedEvidence, err := (&Verifier{evidence: evidencePort}).verifyEvidenceDocuments([][]byte{[]byte("owner-typed-result")}, []Evidence{wantEvidence})
		if err != nil || state != StateProven || !reflect.DeepEqual(observedEvidence, []Evidence{wantEvidence}) {
			t.Fatalf("complete evidence proof = %q/%#v/%v, want exact derived row", state, observedEvidence, err)
		}
		evidenceMutations := []struct {
			name   string
			mutate func(*Evidence)
		}{
			{name: "command id", mutate: func(row *Evidence) { row.CommandID = "other" }},
			{name: "argv", mutate: func(row *Evidence) { row.Argv = []string{"make", "test"} }},
			{name: "exit code", mutate: func(row *Evidence) { row.ExitCode = 1 }},
			{name: "verdict", mutate: func(row *Evidence) { row.Verdict = countersign.VerdictViolated }},
			{name: "output digest", mutate: func(row *Evidence) { row.OutputDigest = receiptDigestA }},
		}
		for _, mutation := range evidenceMutations {
			t.Run("evidence "+mutation.name, func(t *testing.T) {
				declared := wantEvidence
				declared.Argv = append([]string(nil), wantEvidence.Argv...)
				mutation.mutate(&declared)
				state, observed, err := (&Verifier{evidence: evidencePort}).verifyEvidenceDocuments([][]byte{[]byte("owner-typed-result")}, []Evidence{declared})
				if err != nil || state != StateViolated {
					t.Fatalf("mutated evidence proof = %q/%v, want violated", state, err)
				}
				if !reflect.DeepEqual(observed, []Evidence{wantEvidence}) {
					t.Fatalf("observed evidence = %#v, want owner-derived %#v", observed, []Evidence{wantEvidence})
				}
			})
		}
		if _, _, err := (&Verifier{}).verifyEvidenceDocuments([][]byte{[]byte("owner-typed-result")}, []Evidence{wantEvidence}); err == nil {
			t.Fatal("missing evidence proof verifier error = nil")
		}
	})
}

func TestVerifierBuilderFreshnessProjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		candidate Candidate
		want      string
	}{
		{
			name: "candidate one",
			candidate: Candidate{
				BaseCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				BaseTree:   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				HeadCommit: "cccccccccccccccccccccccccccccccccccccccc",
				HeadTree:   "dddddddddddddddddddddddddddddddddddddddd",
			},
			want: "sha256:7d1cfa95a2bda603281d2f03c382c2ae1c9c131075d5127c679a15335a2a9aa2",
		},
		{
			name: "candidate two",
			candidate: Candidate{
				BaseCommit: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				BaseTree:   "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				HeadCommit: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
				HeadTree:   "dddddddddddddddddddddddddddddddddddddddd",
			},
			want: "sha256:562854b899c548cfc28e272177735ecf3e71fd338a61540a228aa72a0e77c836",
		},
	}

	digests := make([]string, 0, len(tests))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projection, err := (&Verifier{}).reviewProjection(VerifyRequest{
				Receipt:   Receipt{Role: RoleBuilder},
				Candidate: tt.candidate,
			}, ExecutionProjection{}, nil, nil, RepositoryProof{})
			if err != nil {
				t.Fatalf("reviewProjection() error = %v", err)
			}
			digests = append(digests, projection.Freshness.ExpectedDigest)
			if got := projection.Freshness.ExpectedDigest; got != tt.want {
				t.Fatalf("builder freshness expected digest = %q, want independent literal %q", got, tt.want)
			}
			if got := projection.Freshness.ObservedDigest; got != tt.want {
				t.Fatalf("builder freshness observed digest = %q, want independent literal %q", got, tt.want)
			}
		})
	}
	if digests[0] == digests[1] {
		t.Fatalf("builder freshness digests are candidate-insensitive: %q", digests[0])
	}
}

func TestVerifierAcceptsNonAdjacentReceiptAckGlobalSequence(t *testing.T) {
	t.Parallel()

	request, execution, authority := validVerifierRequestFixture(t)
	request.ReceiptEventAck.GlobalSequence = request.Receipt.TerminalGlobalSequence + 7
	request.Digest = ""
	ackDigest, err := canonjson.Digest(request.ReceiptEventAck)
	if err != nil {
		t.Fatalf("digest receipt acknowledgment: %v", err)
	}
	authority.Persistence.ReceiptAckDigest = ackDigest

	verdict, err := NewVerifierWithExecutionProof(staticAuthorityResolver{facts: authority}, staticExecutionProof{projection: execution}).Verify(context.Background(), request)
	if err != nil {
		t.Fatalf("Verify(receipt acknowledgment with interleaved global sequence) error = %v", err)
	}
	if operand := verdict.Operands[operandKindIndex("receipt-ack")]; operand.State != StateProven || operand.ExpectedDigest != operand.ObservedDigest {
		t.Fatalf("receipt-ack operand = %#v, want carried later global sequence proven", operand)
	}
	if verdict.State != StateProven {
		t.Fatalf("verdict state = %q, want %q", verdict.State, StateProven)
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

type staticEvidenceProof struct {
	projection EvidenceProofProjection
	err        error
}

func (f staticEvidenceProof) VerifyEvidenceProof([]byte) (EvidenceProofProjection, error) {
	f.projection.Argv = append([]string(nil), f.projection.Argv...)
	return f.projection, f.err
}

type staticReviewProof struct {
	projection        ReviewProofProjection
	err               error
	wantRaw           []byte
	wantReceiptDigest string
	wantCandidate     Candidate
	wantRepository    *RepositoryProof
	wantLaunch        *ReviewLaunchProof
}

func (f staticReviewProof) VerifyReviewProof(raw []byte, receipt Receipt, candidate Candidate, repository RepositoryProof, launch ReviewLaunchProof) (ReviewProofProjection, error) {
	if f.wantRaw != nil && !bytes.Equal(raw, f.wantRaw) {
		return ReviewProofProjection{}, errors.New("review packet bytes changed at verifier port")
	}
	if f.wantReceiptDigest != "" && receipt.Digest != f.wantReceiptDigest {
		return ReviewProofProjection{}, errors.New("review receipt changed at verifier port")
	}
	if f.wantCandidate != (Candidate{}) && candidate != f.wantCandidate {
		return ReviewProofProjection{}, errors.New("review candidate changed at verifier port")
	}
	if f.wantRepository != nil && !reflect.DeepEqual(repository, *f.wantRepository) {
		return ReviewProofProjection{}, errors.New("review repository proof changed at verifier port")
	}
	if f.wantLaunch != nil && !reflect.DeepEqual(launch, *f.wantLaunch) {
		return ReviewProofProjection{}, errors.New("review launch proof changed at verifier port")
	}
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
	promptSchema, err := contextevent.PayloadSchema(contextevent.KindPrompt)
	if err != nil {
		t.Fatal(err)
	}
	promptDetail := contextevent.Detail{
		Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON,
		Digest: digestRawNoFrame([]byte(`{"prompt":"redacted"}`)), RedactionProfile: contextevent.RedactionProfileStandard,
		RedactedJSON: []byte(`{"prompt":"redacted"}`),
	}
	promptEvent := contextevent.Event{
		Schema: contextevent.EventSchemaID, SourceSequence: 1, Flight: "flight-1", Lane: "builder", Epoch: "epoch-1",
		ManifestRevision: 0, ManifestDigest: receipt.ManifestDigest, Session: "session-1", ATCRunway: receipt.ATCRunway,
		ExecutionWorkspaceID: receipt.ExecutionWorkspaceID, CandidateCommit: commitOID, CandidateTree: treeOID,
		Adapter: receipt.Adapter, AdapterVersion: receipt.AdapterVersion, OccurredAt: "2026-08-28T12:34:55Z",
		Kind: contextevent.KindPrompt, PayloadSchema: promptSchema,
		Payload: &contextevent.PromptPayload{Schema: promptSchema, PromptDigest: receiptDigestA, Detail: promptDetail},
	}
	promptEventBytes, err := contextevent.EncodeEvent(promptEvent)
	if err != nil {
		t.Fatal(err)
	}
	promptEvent, err = contextevent.DecodeEvent(bytes.NewReader(promptEventBytes))
	if err != nil {
		t.Fatal(err)
	}

	executionEvent := contextevent.Event{
		Schema: contextevent.EventSchemaID, SourceSequence: 2, Flight: "flight-1", Lane: "builder", Epoch: "epoch-1",
		ManifestRevision: 0, ManifestDigest: receipt.ManifestDigest, Session: "session-1", ATCRunway: receipt.ATCRunway,
		ExecutionWorkspaceID: receipt.ExecutionWorkspaceID, CandidateCommit: commitOID, CandidateTree: treeOID,
		Adapter: receipt.Adapter, AdapterVersion: receipt.AdapterVersion, OccurredAt: "2026-08-28T12:34:56Z",
		Kind: contextevent.KindExecutionResult, PayloadSchema: schema,
		Payload:          &contextevent.ExecutionResultPayload{Schema: schema, Authority: contextevent.AuthorityAuthoritative, InputCommit: commitOID, OutputCommit: commitOID, OutputTree: treeOID, Clean: true, ManifestDigest: receipt.ManifestDigest, ResultFactsDigest: receiptDigestB},
		PriorEventDigest: promptEvent.EventDigest,
	}
	executionEventBytes, err := contextevent.EncodeEvent(executionEvent)
	if err != nil {
		t.Fatal(err)
	}
	executionEvent, err = contextevent.DecodeEvent(bytes.NewReader(executionEventBytes))
	if err != nil {
		t.Fatal(err)
	}
	receipt.RevisionSegments = []contextevent.Revision{{Schema: contextevent.RevisionSchemaID, ManifestRevision: 0, ManifestDigest: receipt.ManifestDigest, FirstGlobalSequence: 11, TerminalGlobalSequence: 19, TerminalSourceSequence: 2, TerminalKind: contextevent.KindExecutionResult, EventRoot: executionEvent.EventDigest}}
	receipt.EventChainRoot, err = contextevent.EventChainRoot(receipt.RevisionSegments)
	if err != nil {
		t.Fatal(err)
	}
	receipt.TerminalManifestRevision, receipt.TerminalSourceSequence, receipt.TerminalGlobalSequence = 0, 2, 19
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
	promptAckBytes := executionEventAckBytes(t, promptEvent, 11)
	resultAckBytes := executionEventAckBytes(t, executionEvent, 19)
	requestBytes, err := EncodeVerifyRequest(VerifyRequest{
		Schema: VerifyRequestSchemaID, Receipt: receipt, ReceiptEventAck: ack, Candidate: candidate,
		Proofs: ProofBundle{ExecutionRequestBytes: executionBytes, RepositoryProofBytes: repositoryBytes, ExecutionEventBytes: [][]byte{promptEventBytes, executionEventBytes}, ExecutionEventAckBytes: [][]byte{promptAckBytes, resultAckBytes}, ReceiptEventBytes: receiptEventBytes, ExpansionDataBytes: [][]byte{}, ObligationBytes: [][]byte{}, EvidenceResultBytes: [][]byte{}, ReviewPacketBytes: []byte{}},
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

func obligationRepositoryProofFixture(t *testing.T, documentPath string, document []byte) RepositoryProof {
	t.Helper()
	const prefix = ".verdi/obligations/example/"
	if !strings.HasPrefix(documentPath, prefix) {
		t.Fatalf("obligation fixture path = %q, want prefix %q", documentPath, prefix)
	}
	blobOID := verifierGitOID("blob", document)
	leaf := verifierTreeBody(t, "100644", strings.TrimPrefix(documentPath, prefix), blobOID)
	leafOID := verifierGitOID("tree", leaf)
	obligations := verifierTreeBody(t, "40000", "example", leafOID)
	obligationsOID := verifierGitOID("tree", obligations)
	verdi := verifierTreeBody(t, "40000", "obligations", obligationsOID)
	verdiOID := verifierGitOID("tree", verdi)
	root := verifierTreeBody(t, "40000", ".verdi", verdiOID)
	rootOID := verifierGitOID("tree", root)
	commitBody := []byte("tree " + rootOID + "\n\nobligation fixture\n")
	commitOID := verifierGitOID("commit", commitBody)
	objects := []RepositoryObject{
		{OID: commitOID, Type: "commit", Content: commitBody},
		{OID: leafOID, Type: "tree", Content: leaf},
		{OID: obligationsOID, Type: "tree", Content: obligations},
		{OID: verdiOID, Type: "tree", Content: verdi},
		{OID: rootOID, Type: "tree", Content: root},
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].OID < objects[j].OID })
	candidate := Candidate{BaseCommit: commitOID, BaseTree: rootOID, HeadCommit: commitOID, HeadTree: rootOID}
	return RepositoryProof{
		Schema: RepositoryProofSchemaID, ObjectFormat: "sha1", Candidate: candidate, Objects: objects,
		ExecutionObservation: ExecutionObservation{WorkspaceID: "workspace-obligation", Commit: commitOID, Tree: rootOID, Clean: true, EventDigest: receiptDigestA},
	}
}

func verifierTreeBody(t *testing.T, mode, name, oid string) []byte {
	t.Helper()
	rawOID, err := hex.DecodeString(oid)
	if err != nil {
		t.Fatal(err)
	}
	body := append([]byte(mode+" "+name+"\x00"), rawOID...)
	return body
}

func executionEventAckBytes(t *testing.T, event contextevent.Event, globalSequence uint64) []byte {
	t.Helper()
	encoded, err := contextevent.EncodeEventAck(contextevent.EventAck{
		Schema: contextevent.AckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch,
		Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind,
		SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: globalSequence,
	})
	if err != nil {
		t.Fatalf("EncodeEventAck fixture: %v", err)
	}
	return encoded
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
	eventAckBytes, err := contextevent.EncodeEventAck(contextevent.EventAck{
		Schema: contextevent.AckSchemaID, Flight: "flight-1", Lane: "builder", Epoch: "epoch-1", Session: "session-1",
		ManifestRevision: 0, Kind: contextevent.KindExecutionResult, SourceSequence: 1, EventDigest: receiptDigestA, GlobalSequence: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	return VerifyRequest{
		Schema:          VerifyRequestSchemaID,
		Receipt:         receipt,
		ReceiptEventAck: ack,
		Candidate:       Candidate{BaseCommit: receipt.InputCommit, BaseTree: receipt.InputTree, HeadCommit: receipt.OutputCommit, HeadTree: receipt.OutputTree},
		Proofs: ProofBundle{
			ExecutionRequestBytes: []byte("{}\n"), RepositoryProofBytes: []byte("{}\n"),
			ExecutionEventBytes: [][]byte{[]byte("{}\n")}, ExecutionEventAckBytes: [][]byte{eventAckBytes}, ReceiptEventBytes: []byte("{}\n"),
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
