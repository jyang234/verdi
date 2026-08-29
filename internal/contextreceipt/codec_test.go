package contextreceipt

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/countersign"
	"github.com/jyang234/verdi/internal/governanceprincipal"
)

const (
	receiptDigestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	receiptDigestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	receiptDigestC = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
	receiptSHAA    = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	receiptSHAB    = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

func TestContextReceiptContract_Static(t *testing.T) {
	t.Parallel()

	t.Run("typed composite ordering", func(t *testing.T) {
		t.Run("numeric revision", func(t *testing.T) {
			parentTwo := Expansion{RequestID: "request", ParentRevision: 2, ParentManifestDigest: receiptDigestA, ChildRevision: 3, ChildManifestDigest: receiptDigestB, ExpansionDigest: receiptDigestC}
			parentTen := Expansion{RequestID: "request", ParentRevision: 10, ParentManifestDigest: receiptDigestA, ChildRevision: 11, ChildManifestDigest: receiptDigestB, ExpansionDigest: receiptDigestC}
			if !expansionLess(parentTwo, parentTen) {
				t.Fatal("typed expansion identity must order numeric revision 2 before 10")
			}
		})
		t.Run("argv boundary", func(t *testing.T) {
			argvElements := Evidence{CommandID: "command", Argv: []string{"a", "b"}, ExitCode: 0, Verdict: countersign.VerdictProven, OutputDigest: receiptDigestA}
			argvEmbeddedBoundary := Evidence{CommandID: "command", Argv: []string{"a\x00b"}, ExitCode: 0, Verdict: countersign.VerdictProven, OutputDigest: receiptDigestA}
			if !evidenceLess(argvElements, argvEmbeddedBoundary) {
				t.Fatal("typed evidence identity must preserve argv element boundaries")
			}
		})
	})

	builder := receiptFixture(t, RoleBuilder)
	encoded, err := EncodeReceipt(builder)
	if err != nil {
		t.Fatalf("EncodeReceipt(builder) error = %v", err)
	}
	if bytes.Contains(encoded, []byte("receipt_event_ack")) {
		t.Fatalf("receipt bytes include later acknowledgment: %s", encoded)
	}
	decoded, err := DecodeReceipt(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeReceipt(builder) error = %v", err)
	}
	if decoded.Digest == "" || decoded.Role != RoleBuilder || decoded.ReviewOf != nil {
		t.Fatalf("decoded builder mismatch: %#v", decoded)
	}
	reencoded, err := EncodeReceipt(decoded)
	if err != nil {
		t.Fatalf("EncodeReceipt(decoded) error = %v", err)
	}
	if !bytes.Equal(encoded, reencoded) {
		t.Fatalf("receipt round trip changed bytes\nfirst: %s\nagain: %s", encoded, reencoded)
	}

	reviewer := receiptFixture(t, RoleReviewer)
	reviewer.ReviewOf = []string{decoded.Digest}
	reviewerBytes, err := EncodeReceipt(reviewer)
	if err != nil {
		t.Fatalf("EncodeReceipt(reviewer) error = %v", err)
	}
	reviewerDecoded, err := DecodeReceipt(bytes.NewReader(reviewerBytes))
	if err != nil {
		t.Fatalf("DecodeReceipt(reviewer) error = %v", err)
	}
	if len(reviewerDecoded.ReviewOf) != 1 || reviewerDecoded.ReviewOf[0] != decoded.Digest {
		t.Fatalf("reviewer review_of = %#v, want builder digest", reviewerDecoded.ReviewOf)
	}

	tests := []struct {
		name   string
		mutate func(*Receipt)
	}{
		{"unknown role", func(r *Receipt) { r.Role = Role("operator") }},
		{"unknown authority", func(r *Receipt) { r.Authority = Authority("trusted") }},
		{"builder review_of", func(r *Receipt) { r.ReviewOf = []string{receiptDigestA} }},
		{"reviewer missing review_of", func(r *Receipt) { r.Role = RoleReviewer }},
		{"reviewer duplicate review_of", func(r *Receipt) { r.Role = RoleReviewer; r.ReviewOf = []string{receiptDigestA, receiptDigestA} }},
		{"nil expansions", func(r *Receipt) { r.Expansions = nil }},
		{"nil obligations", func(r *Receipt) { r.Obligations = nil }},
		{"nil evidence", func(r *Receipt) { r.Evidence = nil }},
		{"nil review inputs", func(r *Receipt) { r.ReviewInputs = nil }},
		{"nil revisions", func(r *Receipt) { r.RevisionSegments = nil }},
		{"unsorted expansions", func(r *Receipt) { r.Expansions[0], r.Expansions[1] = r.Expansions[1], r.Expansions[0] }},
		{"duplicate obligation identity", func(r *Receipt) { r.Obligations = append(r.Obligations, r.Obligations[0]) }},
		{"unsorted evidence", func(r *Receipt) { r.Evidence[0], r.Evidence[1] = r.Evidence[1], r.Evidence[0] }},
		{"duplicate review input", func(r *Receipt) { r.ReviewInputs = append(r.ReviewInputs, r.ReviewInputs[0]) }},
		{"unknown obligation kind", func(r *Receipt) { r.Obligations[0].Kind = artifact.EvidenceKind("manual") }},
		{"unknown evidence verdict", func(r *Receipt) { r.Evidence[0].Verdict = countersign.Verdict("pass") }},
		{"event chain mismatch", func(r *Receipt) { r.EventChainRoot = receiptDigestA }},
		{"terminal revision mismatch", func(r *Receipt) { r.TerminalManifestRevision++ }},
		{"terminal source mismatch", func(r *Receipt) { r.TerminalSourceSequence++ }},
		{"terminal global mismatch", func(r *Receipt) { r.TerminalGlobalSequence++ }},
		{"incomplete terminal", func(r *Receipt) {
			r.RevisionSegments[len(r.RevisionSegments)-1].TerminalKind = contextevent.KindChildManifest
		}},
		{"mismatched self digest", func(r *Receipt) { r.Digest = receiptDigestA }},
		{"authoritative unresolved runner", func(r *Receipt) {
			r.RunnerPrincipalResolution.State = governanceprincipal.ResolutionUnproven
			r.RunnerPrincipalResolution.PrincipalID = ""
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receipt := receiptFixture(t, RoleBuilder)
			tt.mutate(&receipt)
			if _, err := EncodeReceipt(receipt); err == nil {
				t.Fatal("EncodeReceipt() error = nil")
			}
		})
	}

	badDocuments := [][]byte{
		append(append([]byte(nil), encoded...), []byte("{}")...),
		bytes.Replace(encoded, []byte(`"adapter":"codex"`), []byte(`"adapter":"codex","unknown":true`), 1),
		bytes.Replace(encoded, []byte(`"schema":"verdi.context-receipt/v1",`), nil, 1),
		bytes.Replace(encoded, []byte(`"expansions":[`), []byte(`"expansions":null,"discarded":[`), 1),
		bytes.Replace(encoded, []byte(`"adapter":"codex"`), []byte(`"adapter":"codex","adapter":"codex"`), 1),
		bytes.Replace(encoded, []byte(`{"adapter"`), []byte("{ \"adapter\""), 1),
	}
	for i, raw := range badDocuments {
		if _, err := DecodeReceipt(bytes.NewReader(raw)); err == nil {
			t.Errorf("DecodeReceipt(bad document %d) error = nil", i)
		}
	}
	if _, err := DecodeReceipt(receiptErrorReader{}); err == nil {
		t.Fatal("DecodeReceipt(read error) error = nil")
	}
}

type receiptErrorReader struct{}

func (receiptErrorReader) Read([]byte) (int, error) { return 0, bytes.ErrTooLarge }

func receiptFixture(t *testing.T, role Role) Receipt {
	t.Helper()
	revisions := []contextevent.Revision{
		{Schema: contextevent.RevisionSchemaID, ManifestRevision: 0, ManifestDigest: receiptDigestA, FirstGlobalSequence: 1, TerminalGlobalSequence: 4, TerminalSourceSequence: 4, TerminalKind: contextevent.KindChildManifest, EventRoot: receiptDigestA},
		{Schema: contextevent.RevisionSchemaID, ManifestRevision: 1, ManifestDigest: receiptDigestB, FirstGlobalSequence: 8, TerminalGlobalSequence: 12, TerminalSourceSequence: 3, TerminalKind: contextevent.KindChildManifest, EventRoot: receiptDigestB},
		{Schema: contextevent.RevisionSchemaID, ManifestRevision: 2, ManifestDigest: receiptDigestC, FirstGlobalSequence: 15, TerminalGlobalSequence: 21, TerminalSourceSequence: 5, TerminalKind: contextevent.KindExecutionResult, EventRoot: receiptDigestC},
	}
	root, err := contextevent.EventChainRoot(revisions)
	if err != nil {
		t.Fatal(err)
	}
	claim := governanceprincipal.PrincipalClaim{TrustSource: "ci-runner", Subject: "runner@example.com"}
	principalID, err := governanceprincipal.CanonicalPrincipalID(claim.TrustSource, claim.Subject)
	if err != nil {
		t.Fatal(err)
	}
	return Receipt{
		Schema:                          SchemaID,
		Role:                            role,
		Authority:                       AuthorityAuthoritative,
		ManifestDigest:                  receiptDigestC,
		DispatchDigest:                  receiptDigestC,
		ATCRunway:                       "/runway/flight-1",
		ExecutionWorkspaceRequestDigest: receiptDigestA,
		ExecutionWorkspaceID:            "workspace-1",
		InputCommit:                     receiptSHAA,
		InputTree:                       receiptSHAB,
		OutputCommit:                    receiptSHAB,
		OutputTree:                      receiptSHAA,
		Clean:                           true,
		RevisionSegments:                revisions,
		EventChainRoot:                  root,
		TerminalManifestRevision:        2,
		TerminalSourceSequence:          5,
		TerminalGlobalSequence:          21,
		Expansions: []Expansion{
			{RequestID: "request-1", ParentRevision: 0, ParentManifestDigest: receiptDigestA, ChildRevision: 1, ChildManifestDigest: receiptDigestB, ExpansionDigest: receiptDigestA},
			{RequestID: "request-2", ParentRevision: 1, ParentManifestDigest: receiptDigestB, ChildRevision: 2, ChildManifestDigest: receiptDigestC, ExpansionDigest: receiptDigestB},
		},
		Obligations: []Obligation{
			{Ref: "obligation/example--ac-1--behavioral", Path: ".verdi/obligations/example/ac-1--behavioral.md", AC: "ac-1", Kind: artifact.EvidenceBehavioral, ContentDigest: receiptDigestA, Producer: "go-test:internal/example:TestBehavioral"},
			{Ref: "obligation/example--ac-1--static", Path: ".verdi/obligations/example/ac-1--static.md", AC: "ac-1", Kind: artifact.EvidenceStatic, ContentDigest: receiptDigestB, Producer: "go-test:internal/example:TestStatic"},
		},
		Evidence: []Evidence{
			{CommandID: "command-1", Argv: []string{"go", "test", "./internal/example", "-run", "TestStatic"}, ExitCode: 0, Verdict: countersign.VerdictProven, OutputDigest: receiptDigestA},
			{CommandID: "command-2", Argv: []string{"go", "test", "./internal/example", "-run", "TestBehavioral"}, ExitCode: 0, Verdict: countersign.VerdictProven, OutputDigest: receiptDigestB},
		},
		RunnerPrincipalResolution: governanceprincipal.PrincipalResolution{
			Claim: claim, PrincipalID: principalID, State: governanceprincipal.ResolutionAuthenticated,
			Witnesses: []governanceprincipal.Witness{{Code: "trust-subject-verified", SourceID: "ci-runner", EvidenceDigest: receiptDigestA}},
		},
		Adapter:        contextevent.AdapterCodex,
		AdapterVersion: "1.2.3",
		ReviewInputs: []ReviewInput{
			{Kind: "accepted-spec", ContentDigest: receiptDigestA},
			{Kind: "evidence-bundle", ContentDigest: receiptDigestB},
		},
	}
}

func TestContextReceiptFixtureIsDeterministicallyOrdered(t *testing.T) {
	t.Parallel()
	receipt := receiptFixture(t, RoleBuilder)
	encoded, err := EncodeReceipt(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(encoded), "\n") {
		t.Fatalf("canonical receipt lacks trailing newline: %q", encoded)
	}
}

func TestContextReceiptContract_Behavioral(t *testing.T) {
	t.Parallel()

	builder := receiptFixture(t, RoleBuilder)
	encodedBuilder, err := EncodeReceipt(builder)
	if err != nil {
		t.Fatalf("EncodeReceipt(builder) error = %v", err)
	}
	builder, err = DecodeReceipt(bytes.NewReader(encodedBuilder))
	if err != nil {
		t.Fatalf("DecodeReceipt(builder) error = %v", err)
	}
	eventBytes, ack := receiptCompletionFixture(t, builder, encodedBuilder)
	if err := validateReceiptCompletion(builder, eventBytes, ack); err != nil {
		t.Fatalf("validateReceiptCompletion(builder) error = %v", err)
	}

	reviewer := receiptFixture(t, RoleReviewer)
	reviewer.ReviewOf = []string{builder.Digest}
	encodedReviewer, err := EncodeReceipt(reviewer)
	if err != nil {
		t.Fatalf("EncodeReceipt(reviewer) error = %v", err)
	}
	reviewer, err = DecodeReceipt(bytes.NewReader(encodedReviewer))
	if err != nil {
		t.Fatalf("DecodeReceipt(reviewer) error = %v", err)
	}
	reviewerEvent, reviewerAck := receiptCompletionFixture(t, reviewer, encodedReviewer)
	if err := validateReceiptCompletion(reviewer, reviewerEvent, reviewerAck); err != nil {
		t.Fatalf("validateReceiptCompletion(reviewer) error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Receipt, *[]byte, *contextevent.ReceiptEventAck)
	}{
		{"missing expansion", func(r *Receipt, _ *[]byte, _ *contextevent.ReceiptEventAck) { r.Expansions = nil }},
		{"missing obligation", func(r *Receipt, _ *[]byte, _ *contextevent.ReceiptEventAck) { r.Obligations = nil }},
		{"missing evidence", func(r *Receipt, _ *[]byte, _ *contextevent.ReceiptEventAck) { r.Evidence = nil }},
		{"missing reviewer link", func(r *Receipt, _ *[]byte, _ *contextevent.ReceiptEventAck) { r.Role = RoleReviewer; r.ReviewOf = nil }},
		{"missing event", func(_ *Receipt, event *[]byte, _ *contextevent.ReceiptEventAck) { *event = nil }},
		{"mismatched acknowledgment", func(_ *Receipt, _ *[]byte, ack *contextevent.ReceiptEventAck) { ack.ReceiptDigest = receiptDigestA }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receipt := builder
			event := append([]byte(nil), eventBytes...)
			candidateAck := ack
			tt.mutate(&receipt, &event, &candidateAck)
			if err := validateReceiptCompletion(receipt, event, candidateAck); err == nil {
				t.Fatal("validateReceiptCompletion() error = nil")
			}
		})
	}
}

func receiptCompletionFixture(t *testing.T, receipt Receipt, receiptBytes []byte) ([]byte, contextevent.ReceiptEventAck) {
	t.Helper()
	represented := bytes.TrimSuffix(receiptBytes, []byte("\n"))
	representedDigest := fmt.Sprintf("sha256:%x", sha256.Sum256(represented))
	event := contextevent.Event{
		Schema:               contextevent.EventSchemaID,
		SourceSequence:       receipt.TerminalSourceSequence + 1,
		Flight:               "flight-1",
		Lane:                 "builder",
		Epoch:                "epoch-1",
		ManifestRevision:     receipt.TerminalManifestRevision,
		ManifestDigest:       receipt.ManifestDigest,
		Session:              "session-1",
		ATCRunway:            receipt.ATCRunway,
		ExecutionWorkspaceID: receipt.ExecutionWorkspaceID,
		CandidateCommit:      receipt.OutputCommit,
		CandidateTree:        receipt.OutputTree,
		Adapter:              receipt.Adapter,
		AdapterVersion:       receipt.AdapterVersion,
		OccurredAt:           "2026-08-28T12:34:56Z",
		Kind:                 contextevent.KindReceipt,
		PayloadSchema:        "verdi.context-event-payload/receipt/v1",
		Payload: &contextevent.ReceiptPayload{
			Schema:                  "verdi.context-event-payload/receipt/v1",
			Role:                    receipt.Role,
			ReceiptDigest:           receipt.Digest,
			Authority:               receipt.Authority,
			ExecutionEventChainRoot: receipt.EventChainRoot,
			Detail:                  contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: representedDigest, RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: represented},
		},
		PriorEventDigest: receipt.RevisionSegments[len(receipt.RevisionSegments)-1].EventRoot,
	}
	encoded, err := contextevent.EncodeEvent(event)
	if err != nil {
		t.Fatalf("EncodeEvent(receipt) error = %v", err)
	}
	event, err = contextevent.DecodeEvent(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeEvent(receipt) error = %v", err)
	}
	ack := contextevent.ReceiptEventAck{
		Schema: contextevent.ReceiptAckSchemaID, Flight: event.Flight, Lane: event.Lane,
		Epoch: event.Epoch, Session: event.Session, ManifestRevision: event.ManifestRevision,
		Kind: event.Kind, SourceSequence: event.SourceSequence, EventDigest: event.EventDigest,
		GlobalSequence: receipt.TerminalGlobalSequence + 1, ReceiptDigest: receipt.Digest,
	}
	return encoded, ack
}
