package contextowner

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// The exact 16/6 partition contract §3 fixes, spelled here independently of
// relations.go's own relationOperations map so that a drift in the map is
// caught rather than trusted.
var wantRelationOperations = []Operation{
	OperationVerifyAuthority, OperationResolveProfile, OperationVerifyConflict,
	OperationResolveRecorder, OperationRecorderCheckpoint, OperationRecorderAppend,
	OperationStoreRedactedSegment, OperationResolveRedactedSegment,
	OperationVerifyOpaqueBoundary, OperationVerifyProviderSession, OperationResolveContext,
	OperationAppendReceipt, OperationResolveReceiptVerificationAuthority,
	OperationPersistHandback, OperationPersistQuarantine, OperationPersistAbort,
}

var wantNoRelationOperations = []Operation{
	OperationVerifyExpansion, OperationStoreAdapterSession, OperationNextStamp,
	OperationVerifyEpoch, OperationInstallExpansion, OperationResolveReceiptInputs,
}

func TestHasRelations_Classification(t *testing.T) {
	if got := len(wantRelationOperations) + len(wantNoRelationOperations); got != 22 {
		t.Fatalf("test fixture lists %d operations, want 22", got)
	}
	for _, op := range wantRelationOperations {
		if !HasRelations(op) {
			t.Errorf("HasRelations(%s) = false, want true", op)
		}
	}
	for _, op := range wantNoRelationOperations {
		if HasRelations(op) {
			t.Errorf("HasRelations(%s) = true, want false", op)
		}
	}

	all := append(append([]Operation{}, wantRelationOperations...), wantNoRelationOperations...)
	slices.Sort(all)
	want := Operations()
	slices.Sort(want)
	if !slices.Equal(all, want) {
		t.Errorf("relation-bearing + no-relation operations = %v, want exactly Operations() partitioned = %v", all, want)
	}
}

func TestValidateRelations_NoRelationArmsAlwaysNil(t *testing.T) {
	fixtures := map[Operation]string{
		OperationVerifyExpansion:      "verify-expansion.owner-reply.json",
		OperationStoreAdapterSession:  "store-adapter-session.owner-reply.json",
		OperationNextStamp:            "next-stamp.owner-reply.json",
		OperationVerifyEpoch:          "verify-epoch.owner-reply.json",
		OperationInstallExpansion:     "install-expansion.owner-reply.json",
		OperationResolveReceiptInputs: "resolve-receipt-inputs.owner-reply.json",
	}
	for _, op := range wantNoRelationOperations {
		op := op
		t.Run(string(op), func(t *testing.T) {
			fixture, ok := fixtures[op]
			if !ok {
				t.Fatalf("test setup: no fixture named for %s", op)
			}
			reply, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, fixture)))
			if err != nil {
				t.Fatalf("DecodeReply(%s): %v", fixture, err)
			}
			if err := ValidateRelations(reply); err != nil {
				t.Errorf("ValidateRelations(%s) = %v, want nil", op, err)
			}
		})
	}
}

// structurallyValidReply proves reply -- already mutated by the caller --
// round-trips through EncodeReply and DecodeReply, and returns the
// redecoded reply together with its exact canonical result-arm bytes. A
// mutation that fails this step is not "structurally valid" and would prove
// nothing about ValidateRelations specifically.
func structurallyValidReply(t *testing.T, reply Reply) (Reply, []byte) {
	t.Helper()
	encoded, err := EncodeReply(reply)
	if err != nil {
		t.Fatalf("EncodeReply: %v (mutation is not structurally valid)", err)
	}
	redecoded, err := DecodeReply(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeReply of the mutated reply did not round-trip: %v", err)
	}
	resultArm, err := ResultArm(redecoded)
	if err != nil {
		t.Fatalf("ResultArm: %v", err)
	}
	return redecoded, resultArm
}

// mutateReply decodes fixture, applies mutate to the typed Reply, and proves
// the result is still structurally valid (see structurallyValidReply).
func mutateReply(t *testing.T, fixture string, mutate func(t *testing.T, reply *Reply)) (Reply, []byte) {
	t.Helper()
	reply, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, fixture)))
	if err != nil {
		t.Fatalf("DecodeReply(%s): %v", fixture, err)
	}
	mutate(t, &reply)
	return structurallyValidReply(t, reply)
}

// resealControlAck decodes a canonical control-ack document, applies mutate
// to its generic member map, recomputes its self-digest exactly as
// validateNestedSelfDigest requires (the canonical encoding of the same
// members with "digest" blanked), and returns the resealed document.
func resealControlAck(t *testing.T, ack json.RawMessage, mutate func(members map[string]any)) json.RawMessage {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(ack))
	decoder.UseNumber()
	var members map[string]any
	if err := decoder.Decode(&members); err != nil {
		t.Fatalf("decode control ack to reseal: %v", err)
	}
	mutate(members)
	members["digest"] = ""
	digest, err := canonjson.Digest(members)
	if err != nil {
		t.Fatalf("canonjson.Digest reseal: %v", err)
	}
	members["digest"] = digest
	encoded, err := canonjson.Marshal(members)
	if err != nil {
		t.Fatalf("canonjson.Marshal resealed ack: %v", err)
	}
	return json.RawMessage(bytes.TrimSuffix(encoded, []byte("\n")))
}

// requireRelationMismatch asserts err wraps ErrRelationMismatch, names
// operation, and echoes none of excluded -- the closed-diagnostics
// requirement that a relation error never carries an operand value.
func requireRelationMismatch(t *testing.T, err error, operation Operation, excluded ...string) {
	t.Helper()
	if !errors.Is(err, ErrRelationMismatch) {
		t.Fatalf("error = %v, want one wrapping ErrRelationMismatch", err)
	}
	message := err.Error()
	if !strings.Contains(message, string(operation)) {
		t.Errorf("error %q does not name operation %s", message, operation)
	}
	for _, value := range excluded {
		if strings.Contains(message, value) {
			t.Errorf("error %q leaks operand value %q", message, value)
		}
	}
}

// The frozen fixtures' own execution identity and the operand values the
// relation errors below must never echo. Named once so every mutation can
// exclude the exact value it contradicted, and so no exclusion is accidentally
// a substring of an operation name (which would make the leak check vacuous).
const (
	fixtureKeyFlight      = "flight-1"
	fixtureKeyLane        = "lane-1"
	fixtureKeyEpoch       = "epoch-1"
	fixtureKeySession     = "session-1"
	fixtureKeyWorkspaceID = "workspace-1"

	fixtureProfileID      = "project-profile"
	fixtureProfileDigest  = "sha256:1900eab6c028483d7126599ee6f50de0d27907b5c65fa90524580b4b0f9852b0"
	fixtureManifestDigest = "sha256:77a2c3dc80d395e7b382dc4198f4d5608f5fc410761bf527ce638784226bfb82"
)

// executionIdentityOperands are the five execution-key values every control
// acknowledgment and event acknowledgment carries. None of them is a substring
// of any operation name, so requiring their absence from an error message is a
// real assertion about the message rather than a tautology.
var executionIdentityOperands = []string{
	fixtureKeyFlight, fixtureKeyLane, fixtureKeyEpoch, fixtureKeySession, fixtureKeyWorkspaceID,
}

// checkpointActiveRevision builds the one structurally valid non-null active
// revision the frozen recorder-checkpoint fixture admits. The fixture's
// complete checkpoint holds exactly one revision (manifest revision 0) with
// terminal global sequence 1, so validActiveRevision
// (codec.go:1467-1527) and validateActiveRevisionBridge (codec.go:1529-1575)
// accept revision 1, a single acknowledgment at source sequence 1 whose global
// sequence 2 advances beyond the complete checkpoint, and a prior event digest
// equal to that acknowledgment's event digest.
//
// It exists so the positive case and the per-member negatives differ in
// exactly the execution-key member under test and in nothing else.
func checkpointActiveRevision(flight, lane, epoch string) *ActiveRevision {
	eventDigest := digestBytes([]byte("recorder-checkpoint-active-revision-event"))
	return &ActiveRevision{
		Revision:           1,
		ManifestDigest:     digestBytes([]byte("recorder-checkpoint-active-revision-manifest")),
		NextSourceSequence: 2,
		PriorEventDigest:   eventDigest,
		LastGlobalSequence: 2,
		EventAcks: []contextevent.EventAck{{
			Schema: contextevent.AckSchemaID, Flight: flight, Lane: lane, Epoch: epoch,
			Session: fixtureKeySession, ManifestRevision: 1, Kind: contextevent.KindAdapterStop,
			SourceSequence: 1, EventDigest: eventDigest, GlobalSequence: 2,
		}},
	}
}

// provenIsolationAuthority builds the proven isolation authority
// validReceiptVerificationAuthority requires (codec.go:2162-2182): a
// well-formed profile id, a canonical profile digest, nonempty session and
// workspace, and the empty -- never null -- witness list a proven state
// demands. The frozen resolve-receipt-verification-authority fixture carries
// an unproven isolation, which short-circuits relations.go's second clause, so
// every test of that clause has to construct this arm in memory.
func provenIsolationAuthority(profileID, profileDigest string) contextreceipt.IsolationAuthority {
	return contextreceipt.IsolationAuthority{
		State:         contextreceipt.StateProven,
		ProfileID:     profileID,
		ProfileDigest: profileDigest,
		Session:       fixtureKeySession,
		WorkspaceID:   fixtureKeyWorkspaceID,
		Witnesses:     []contextreceipt.Witness{},
	}
}

// provenPersistenceAuthority builds the proven persistence authority
// validateReceiptPersistence requires (codec.go:2196-2224): all three digests
// nonempty and canonical, with empty -- never null -- witnesses. The frozen
// fixture's persistence carries an empty receipt digest, which short-circuits
// relations.go's third clause.
func provenPersistenceAuthority(receiptDigest string) contextreceipt.PersistenceAuthority {
	return contextreceipt.PersistenceAuthority{
		State:              contextreceipt.StateProven,
		ReceiptDigest:      receiptDigest,
		ReceiptEventDigest: digestBytes([]byte("receipt-verification-authority-event")),
		ReceiptAckDigest:   digestBytes([]byte("receipt-verification-authority-ack")),
		Witnesses:          []contextreceipt.Witness{},
	}
}

// The one declared opaque row opaqueBoundaryPair builds its pair around.
const (
	opaqueRowID          = "opaque-row"
	opaqueAdapterID      = "codex"
	opaqueAdapterVersion = "1.0.0"
)

// opaqueBoundaryPair rebuilds the frozen verify-opaque-boundary call around a
// single declared row and pairs it with facts carrying exactly one identity,
// mutated by mutate. The frozen fixture declares no rows at all, so its
// positive path never enters relations.go's per-row loop and only the row-count
// clause is reachable from it.
//
// The call is rebuilt with NewCall rather than patched in place: the request
// arm changed, so its controller request digest must be recomputed. A
// hand-patched call would fail NewReply's digest recomputation structurally
// (local.go:94-96) and would therefore prove nothing about the relation.
func opaqueBoundaryPair(t *testing.T, mutate func(identity *OpaqueIdentity)) (Reply, []byte) {
	t.Helper()
	reply, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, "verify-opaque-boundary.owner-reply.json")))
	if err != nil {
		t.Fatalf("DecodeReply(verify-opaque-boundary.owner-reply.json): %v", err)
	}
	reply.Call.VerifyOpaqueBoundary.Rows = []contextcompile.OpaqueEntry{{
		ID:          opaqueRowID,
		Kind:        contextcompile.OpaqueKindHarnessVendorBase,
		Adapter:     contextcompile.AdapterRef{ID: opaqueAdapterID, Version: opaqueAdapterVersion},
		Disclosures: []contextcompile.DisclosureCode{contextcompile.DisclosureOpaqueHarnessVendorBase},
	}}
	requestArm, err := RequestArm(reply.Call)
	if err != nil {
		t.Fatalf("RequestArm(verify-opaque-boundary): %v", err)
	}
	call, err := NewCall(OperationVerifyOpaqueBoundary, requestArm)
	if err != nil {
		t.Fatalf("NewCall(verify-opaque-boundary): %v", err)
	}
	reply.Call = call
	identity := OpaqueIdentity{
		ID: opaqueRowID, Kind: contextcompile.OpaqueKindHarnessVendorBase,
		AdapterID: opaqueAdapterID, AdapterVersion: opaqueAdapterVersion,
	}
	mutate(&identity)
	reply.VerifyOpaqueBoundary.Facts.Rows = []OpaqueIdentity{identity}
	return structurallyValidReply(t, reply)
}

func TestValidateRelations_RejectsStructurallyValidMismatches(t *testing.T) {
	// verify-authority: one mutant per clause of the five-member comparison at
	// relations.go:158-160. Each replacement value is itself structurally valid
	// (canonical digest, full lowercase Git object id) so validAuthorityFacts
	// accepts the mutated arm and the refusal is attributable to the relation.
	verifyAuthorityOperands := []string{
		fixtureManifestDigest,
		"sha256:fde2d35824934d2a819018616efafcc824a6803ca82c49e663e9212185777b46",
		"sha256:677f85e0991fdf0730b6129faef1aaca219393837b86445b26db84db42cbdffa",
		"1111111111111111111111111111111111111111",
	}
	for _, row := range []struct {
		name   string
		mutate func(facts *AuthorityFacts)
	}{
		{"manifest_revision", func(facts *AuthorityFacts) { facts.ManifestRevision++ }},
		{"manifest_digest", func(facts *AuthorityFacts) {
			facts.ManifestDigest = digestBytes([]byte("verify-authority-manifest-probe"))
		}},
		{"projection_digest", func(facts *AuthorityFacts) {
			facts.ProjectionDigest = digestBytes([]byte("verify-authority-projection-probe"))
		}},
		{"authority_digest", func(facts *AuthorityFacts) {
			facts.AuthorityDigest = digestBytes([]byte("verify-authority-verdict-probe"))
		}},
		{"accepted_spec_commit", func(facts *AuthorityFacts) {
			facts.AcceptedSpecCommit = "2222222222222222222222222222222222222222"
		}},
	} {
		row := row
		t.Run("verify-authority "+row.name, func(t *testing.T) {
			reply, resultArm := mutateReply(t, "verify-authority.owner-reply.json", func(t *testing.T, r *Reply) {
				row.mutate(&r.VerifyAuthority.Facts)
			})
			requireRelationMismatch(t, ValidateRelations(reply), OperationVerifyAuthority, verifyAuthorityOperands...)
			_, err := NewReply(reply.Call, resultArm)
			requireRelationMismatch(t, err, OperationVerifyAuthority, verifyAuthorityOperands...)
		})
	}

	t.Run("resolve-profile", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "resolve-profile.owner-reply.json", func(t *testing.T, r *Reply) {
			r.ResolveProfile.Material.Ref.ID = "mutation-probe-profile"
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationResolveProfile,
			"project-profile", "sha256:1900eab6c028483d7126599ee6f50de0d27907b5c65fa90524580b4b0f9852b0")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationResolveProfile,
			"project-profile", "sha256:1900eab6c028483d7126599ee6f50de0d27907b5c65fa90524580b4b0f9852b0")
	})

	t.Run("verify-conflict", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "verify-conflict.owner-reply.json", func(t *testing.T, r *Reply) {
			report, err := policyconflict.DecodeReport(frameExact(r.VerifyConflict.Facts.Report))
			if err != nil {
				t.Fatalf("policyconflict.DecodeReport: %v", err)
			}
			report.Input.EvaluatedOn = "2026-08-28"
			encoded, err := policyconflict.EncodeReport(report)
			if err != nil {
				t.Fatalf("policyconflict.EncodeReport: %v", err)
			}
			r.VerifyConflict.Facts.Report = bytes.TrimSuffix(encoded, []byte("\n"))
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationVerifyConflict,
			"2026-08-27", "sha256:677f85e0991fdf0730b6129faef1aaca219393837b86445b26db84db42cbdffa")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationVerifyConflict,
			"2026-08-27", "sha256:677f85e0991fdf0730b6129faef1aaca219393837b86445b26db84db42cbdffa")
	})

	t.Run("resolve-recorder", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "resolve-recorder.owner-reply.json", func(t *testing.T, r *Reply) {
			r.ResolveRecorder.Facts.Ref.ID = "mutation-probe-recorder"
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationResolveRecorder,
			"vatc-recorder", "sha256:93384247058b5e037a16c08536d5a3b3c20453cda6571c7e016942f9f93b274f")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationResolveRecorder,
			"vatc-recorder", "sha256:93384247058b5e037a16c08536d5a3b3c20453cda6571c7e016942f9f93b274f")
	})

	// recorder-checkpoint: one mutant per member of the execution-key
	// comparison at relations.go:228. The frozen fixture's active revision is
	// null, so the loop is entered only by a constructed revision; the matching
	// positive (TestValidateRelations_AcceptsStructurallyValidMatches) proves
	// this construction agrees with the request key when nothing is mutated,
	// which is what makes a dropped comparison detectable here.
	recorderCheckpointOperands := append(append([]string{}, executionIdentityOperands...),
		"sha256:47320987f9a49d5b00119b960f247a956773f57543982b8bfcb6da5bb3afd9ef")
	for _, row := range []struct {
		name                string
		flight, lane, epoch string
	}{
		{"flight", "flight-OTHER", fixtureKeyLane, fixtureKeyEpoch},
		{"lane", fixtureKeyFlight, "lane-OTHER", fixtureKeyEpoch},
		{"epoch", fixtureKeyFlight, fixtureKeyLane, "epoch-OTHER"},
	} {
		row := row
		t.Run("recorder-checkpoint "+row.name, func(t *testing.T) {
			reply, resultArm := mutateReply(t, "recorder-checkpoint.owner-reply.json", func(t *testing.T, r *Reply) {
				r.RecorderCheckpoint.Checkpoint.ActiveRevision = checkpointActiveRevision(row.flight, row.lane, row.epoch)
			})
			requireRelationMismatch(t, ValidateRelations(reply), OperationRecorderCheckpoint, recorderCheckpointOperands...)
			_, err := NewReply(reply.Call, resultArm)
			requireRelationMismatch(t, err, OperationRecorderCheckpoint, recorderCheckpointOperands...)
		})
	}

	// recorder-append: one mutant per event-identity clause of relations.go's
	// nine-clause comparison at :248-251.
	//
	// The ninth clause, ack.GlobalSequence == 0, is NOT reachable through a
	// structurally valid reply and is deliberately not mutated here: the result
	// arm canonicalizes the acknowledgment with contextevent.EncodeEventAck
	// (codec.go:578 canonicalEventAck), whose validateAckIdentity refuses a
	// zero global sequence outright (internal/contextevent/codec.go:362-364).
	// Any mutant setting it to zero would fail structurallyValidReply and prove
	// a structural refusal rather than a relation one. relations.go keeps the
	// clause for exact parity with the incumbent validateAck at priorGlobal=0.
	recorderAppendOperands := append(append([]string{}, executionIdentityOperands...),
		"sha256:301ed869bd08ab15d310ea77164faf5b23bda12d0d7a80280ef06021920f43f3",
		"vatc-ea6ea82dda1fdda1361db233a4cb62d0d570505601212b34deb395786c8f5916--111111111111")
	for _, row := range []struct {
		name   string
		mutate func(ack *contextevent.EventAck)
	}{
		{"flight", func(ack *contextevent.EventAck) { ack.Flight = "flight-OTHER" }},
		{"lane", func(ack *contextevent.EventAck) { ack.Lane = "lane-OTHER" }},
		{"epoch", func(ack *contextevent.EventAck) { ack.Epoch = "epoch-OTHER" }},
		{"session", func(ack *contextevent.EventAck) { ack.Session = "session-OTHER" }},
		{"manifest_revision", func(ack *contextevent.EventAck) { ack.ManifestRevision++ }},
		{"kind", func(ack *contextevent.EventAck) { ack.Kind = contextevent.KindPrompt }},
		{"source_sequence", func(ack *contextevent.EventAck) { ack.SourceSequence++ }},
		{"event_digest", func(ack *contextevent.EventAck) {
			ack.EventDigest = digestBytes([]byte("recorder-append-mutation-probe"))
		}},
	} {
		row := row
		t.Run("recorder-append "+row.name, func(t *testing.T) {
			reply, resultArm := mutateReply(t, "recorder-append.owner-reply.json", func(t *testing.T, r *Reply) {
				row.mutate(&r.RecorderAppend.Ack)
			})
			requireRelationMismatch(t, ValidateRelations(reply), OperationRecorderAppend, recorderAppendOperands...)
			_, err := NewReply(reply.Call, resultArm)
			requireRelationMismatch(t, err, OperationRecorderAppend, recorderAppendOperands...)
		})
	}

	// store-redacted-segment: relations.go:268-270 compares five members, of
	// which only three are separately reachable on a structurally valid reply.
	//
	//   - reference and digest are one clause, not two: validateStoredSegment
	//     requires stored.Reference == segmentReference(stored.Digest)
	//     (codec.go:1402-1408) and segmentReference is injective on canonical
	//     digests (codec.go:1412-1417), so stored.Reference contradicts the
	//     request exactly when stored.Digest does. The first subtest below
	//     mutates the pair together, which is the only structurally valid way
	//     to reach either.
	//   - media_type and redaction_profile are unreachable: both sides are
	//     pinned to the same two literals -- the request segment by
	//     validRedactedSegment (codec.go:1353-1358) and the stored segment by
	//     validateStoredSegment (codec.go:1390-1395) -- so no structurally
	//     valid pair can disagree on them.
	//   - byte_count is reachable, and is the clause the second subtest adds:
	//     the stored row is only required to be positive (codec.go:1396-1398),
	//     never bound to the submitted segment's length.
	storeRedactedOperands := []string{
		"sha256:ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed",
		"controller-segment/sha256/ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed",
	}
	t.Run("store-redacted-segment reference and digest", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "store-redacted-segment.owner-reply.json", func(t *testing.T, r *Reply) {
			probe, err := canonjson.Marshal(map[string]any{"probe": true})
			if err != nil {
				t.Fatalf("canonjson.Marshal: %v", err)
			}
			trimmed := bytes.TrimSuffix(probe, []byte("\n"))
			digest := digestBytes(trimmed)
			reference, err := segmentReference(digest)
			if err != nil {
				t.Fatalf("segmentReference: %v", err)
			}
			r.StoreRedactedSegment.Stored.Digest = digest
			r.StoreRedactedSegment.Stored.Reference = reference
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationStoreRedactedSegment, storeRedactedOperands...)
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationStoreRedactedSegment, storeRedactedOperands...)
	})

	t.Run("store-redacted-segment byte_count", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "store-redacted-segment.owner-reply.json", func(t *testing.T, r *Reply) {
			r.StoreRedactedSegment.Stored.ByteCount++
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationStoreRedactedSegment, storeRedactedOperands...)
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationStoreRedactedSegment, storeRedactedOperands...)
	})

	t.Run("resolve-redacted-segment", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "resolve-redacted-segment.owner-reply.json", func(t *testing.T, r *Reply) {
			probe, err := canonjson.Marshal(map[string]any{"probe": true})
			if err != nil {
				t.Fatalf("canonjson.Marshal: %v", err)
			}
			trimmed := bytes.TrimSuffix(probe, []byte("\n"))
			r.ResolveRedactedSegment.Segment = RedactedSegment{
				Schema: RedactedSegmentSchemaID, MediaType: contextevent.MediaTypeJSON,
				RedactionProfile: contextevent.RedactionProfileStandard,
				ByteCount:        uint64(len(trimmed)), Digest: digestBytes(trimmed), Bytes: trimmed,
			}
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationResolveRedactedSegment,
			"sha256:ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed",
			"controller-segment/sha256/ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationResolveRedactedSegment,
			"sha256:ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed",
			"controller-segment/sha256/ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed")
	})

	opaqueBoundaryOperands := []string{
		"sha256:00de1d1d9277138da6fa5e753782a551dd291a9ddefdc22b57c92468551fc7c7",
		"verdi.context-owner/verify-opaque-boundary-result/v1",
		opaqueRowID, opaqueAdapterID, opaqueAdapterVersion,
	}
	t.Run("verify-opaque-boundary row count", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "verify-opaque-boundary.owner-reply.json", func(t *testing.T, r *Reply) {
			r.VerifyOpaqueBoundary.Facts.Rows = append(r.VerifyOpaqueBoundary.Facts.Rows, OpaqueIdentity{
				ID: "opaque-mutation-probe", Kind: "harness-vendor-base", AdapterID: "codex", AdapterVersion: "1.0.0",
			})
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationVerifyOpaqueBoundary, opaqueBoundaryOperands...)
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationVerifyOpaqueBoundary, opaqueBoundaryOperands...)
	})

	// verify-opaque-boundary per-row clauses (relations.go:301-302). These need
	// a declared row, which the frozen fixture does not have, so the pair is
	// rebuilt around one (opaqueBoundaryPair) with the row count kept equal --
	// otherwise the length clause at :297 would absorb the refusal and the
	// per-field comparison would stay untested.
	for _, row := range []struct {
		name   string
		mutate func(identity *OpaqueIdentity)
	}{
		{"id", func(identity *OpaqueIdentity) { identity.ID = "opaque-OTHER" }},
		{"kind", func(identity *OpaqueIdentity) { identity.Kind = "harness-vendor-OTHER" }},
		{"adapter_id", func(identity *OpaqueIdentity) { identity.AdapterID = "adapter-OTHER" }},
		{"adapter_version", func(identity *OpaqueIdentity) { identity.AdapterVersion = "9.9.9" }},
	} {
		row := row
		t.Run("verify-opaque-boundary row "+row.name, func(t *testing.T) {
			reply, resultArm := opaqueBoundaryPair(t, row.mutate)
			if got := len(reply.Call.VerifyOpaqueBoundary.Rows); got != len(reply.VerifyOpaqueBoundary.Facts.Rows) {
				t.Fatalf("test setup: %d declared rows vs %d reported identities, want equal lengths",
					got, len(reply.VerifyOpaqueBoundary.Facts.Rows))
			}
			requireRelationMismatch(t, ValidateRelations(reply), OperationVerifyOpaqueBoundary, opaqueBoundaryOperands...)
			_, err := NewReply(reply.Call, resultArm)
			requireRelationMismatch(t, err, OperationVerifyOpaqueBoundary, opaqueBoundaryOperands...)
		})
	}

	// verify-provider-session: one mutant per member of the four-member
	// comparison at relations.go:315-316. The fixture's own session_ref
	// ("provider-session") is deliberately NOT in the excluded set: it is a
	// substring of the operation name, so requiring its absence would fail for
	// a reason that has nothing to do with operand leakage.
	verifyProviderSessionOperands := []string{fixtureProfileDigest, fixtureKeyWorkspaceID}
	for _, row := range []struct {
		name   string
		mutate func(facts *ProviderSessionFacts)
	}{
		{"session_ref", func(facts *ProviderSessionFacts) { facts.SessionRef = "mutation-probe-session" }},
		{"adapter_version", func(facts *ProviderSessionFacts) { facts.AdapterVersion = "9.9.9" }},
		{"profile_digest", func(facts *ProviderSessionFacts) {
			facts.ProfileDigest = digestBytes([]byte("verify-provider-session-profile-probe"))
		}},
		{"workspace_id", func(facts *ProviderSessionFacts) { facts.WorkspaceID = "workspace-OTHER" }},
	} {
		row := row
		t.Run("verify-provider-session "+row.name, func(t *testing.T) {
			reply, resultArm := mutateReply(t, "verify-provider-session.owner-reply.json", func(t *testing.T, r *Reply) {
				row.mutate(&r.VerifyProviderSession.Facts)
			})
			requireRelationMismatch(t, ValidateRelations(reply), OperationVerifyProviderSession, verifyProviderSessionOperands...)
			_, err := NewReply(reply.Call, resultArm)
			requireRelationMismatch(t, err, OperationVerifyProviderSession, verifyProviderSessionOperands...)
		})
	}

	t.Run("resolve-context", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "resolve-context.owner-reply.json", func(t *testing.T, r *Reply) {
			r.ResolveContext.Resolution.Ref = "mutation-probe-ref"
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationResolveContext,
			"spec/test#ac-1", "path:README.md")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationResolveContext,
			"spec/test#ac-1", "path:README.md")
	})

	// append-receipt: one mutant per clause of the nine-clause comparison at
	// relations.go:349-352.
	//
	// The kind clause is unreachable on a structurally valid pair: the request
	// event's kind is pinned to "receipt" by validateReceiptAppendBinding
	// (codec.go:1890-1891) and the result acknowledgment's kind is pinned to
	// the same literal by validateReceiptEventAck
	// (internal/contextevent/codec.go:344-346), so they cannot disagree. It is
	// retained in relations.go for parity with the incumbent
	// validateReceiptAppendAck.
	appendReceiptOperands := append(append([]string{}, executionIdentityOperands...),
		"sha256:95de75491558577517c2524ed1c14bcc3dc04686ec0e1ab9c0a74e116612837c",
		"sha256:58c0f7fd836eddcb59a05a3b5fbca9bd616a8604c31548b7163f1ac0928bf969")
	for _, row := range []struct {
		name   string
		mutate func(ack *contextevent.ReceiptEventAck)
	}{
		{"flight", func(ack *contextevent.ReceiptEventAck) { ack.Flight = "flight-OTHER" }},
		{"lane", func(ack *contextevent.ReceiptEventAck) { ack.Lane = "lane-OTHER" }},
		{"epoch", func(ack *contextevent.ReceiptEventAck) { ack.Epoch = "epoch-OTHER" }},
		{"session", func(ack *contextevent.ReceiptEventAck) { ack.Session = "session-OTHER" }},
		{"manifest_revision", func(ack *contextevent.ReceiptEventAck) { ack.ManifestRevision++ }},
		{"source_sequence", func(ack *contextevent.ReceiptEventAck) { ack.SourceSequence++ }},
		{"event_digest", func(ack *contextevent.ReceiptEventAck) {
			ack.EventDigest = digestBytes([]byte("append-receipt-event-mutation-probe"))
		}},
		{"receipt_digest", func(ack *contextevent.ReceiptEventAck) {
			ack.ReceiptDigest = digestBytes([]byte("append-receipt-mutation-probe"))
		}},
	} {
		row := row
		t.Run("append-receipt "+row.name, func(t *testing.T) {
			reply, resultArm := mutateReply(t, "append-receipt.owner-reply.json", func(t *testing.T, r *Reply) {
				row.mutate(&r.AppendReceipt.Ack)
			})
			requireRelationMismatch(t, ValidateRelations(reply), OperationAppendReceipt, appendReceiptOperands...)
			_, err := NewReply(reply.Call, resultArm)
			requireRelationMismatch(t, err, OperationAppendReceipt, appendReceiptOperands...)
		})
	}

	// resolve-receipt-verification-authority: one mutant per clause of the
	// three-clause relation at relations.go:366-375. The frozen fixture's
	// isolation state is "unproven" and its persistence receipt digest is
	// empty, so the second and third clauses are short-circuited on the frozen
	// positive path; both proven arms below are therefore constructed in
	// memory, and structurallyValidReply proves each is a document
	// validReceiptVerificationAuthority accepts before the relation is asserted.
	receiptAuthorityOperands := []string{
		"fixture", fixtureProfileID, fixtureProfileDigest,
		"sha256:95de75491558577517c2524ed1c14bcc3dc04686ec0e1ab9c0a74e116612837c",
	}
	for _, row := range []struct {
		name   string
		mutate func(query contextreceipt.AuthorityQuery, authority *ReceiptVerificationAuthority)
	}{
		{"trust_fact source_id", func(_ contextreceipt.AuthorityQuery, authority *ReceiptVerificationAuthority) {
			authority.TrustFact.SourceID = "mutation-probe"
		}},
		{"proven isolation profile_id", func(query contextreceipt.AuthorityQuery, authority *ReceiptVerificationAuthority) {
			authority.Isolation = provenIsolationAuthority("mutation-probe-profile", query.ProfileRef.Digest)
		}},
		{"proven isolation profile_digest", func(query contextreceipt.AuthorityQuery, authority *ReceiptVerificationAuthority) {
			authority.Isolation = provenIsolationAuthority(query.ProfileRef.ID,
				digestBytes([]byte("receipt-verification-isolation-probe")))
		}},
		{"persistence receipt_digest", func(_ contextreceipt.AuthorityQuery, authority *ReceiptVerificationAuthority) {
			authority.Persistence = provenPersistenceAuthority(digestBytes([]byte("receipt-verification-persistence-probe")))
		}},
	} {
		row := row
		t.Run("resolve-receipt-verification-authority "+row.name, func(t *testing.T) {
			reply, resultArm := mutateReply(t, "resolve-receipt-verification-authority.owner-reply.json", func(t *testing.T, r *Reply) {
				row.mutate(r.Call.ResolveReceiptVerificationAuthority.Query,
					&r.ResolveReceiptVerificationAuthority.Authority)
			})
			requireRelationMismatch(t, ValidateRelations(reply), OperationResolveReceiptVerificationAuthority,
				receiptAuthorityOperands...)
			_, err := NewReply(reply.Call, resultArm)
			requireRelationMismatch(t, err, OperationResolveReceiptVerificationAuthority, receiptAuthorityOperands...)
		})
	}

	t.Run("persist-handback record_digest", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "persist-handback.owner-reply.json", func(t *testing.T, r *Reply) {
			r.PersistHandback.Ack = resealControlAck(t, r.PersistHandback.Ack, func(members map[string]any) {
				members["record_digest"] = digestBytes([]byte("persist-handback-mutation-probe"))
			})
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationPersistHandback,
			"sha256:507162dfd086f7593d16f890de1e9f5bfefbcbf4564ab9281d65f7a396a2e910", "workspace-1")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationPersistHandback,
			"sha256:507162dfd086f7593d16f890de1e9f5bfefbcbf4564ab9281d65f7a396a2e910", "workspace-1")
	})

	t.Run("persist-handback-wrong-record-kind", func(t *testing.T) {
		handback, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, "persist-handback.owner-reply.json")))
		if err != nil {
			t.Fatalf("DecodeReply(persist-handback.owner-reply.json): %v", err)
		}
		quarantine, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, "persist-quarantine.owner-reply.json")))
		if err != nil {
			t.Fatalf("DecodeReply(persist-quarantine.owner-reply.json): %v", err)
		}
		handback.PersistHandback.Ack = quarantine.PersistQuarantine.Ack
		reply, resultArm := structurallyValidReply(t, handback)
		requireRelationMismatch(t, ValidateRelations(reply), OperationPersistHandback)
		_, err = NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationPersistHandback)
	})

	t.Run("persist-quarantine record_digest", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "persist-quarantine.owner-reply.json", func(t *testing.T, r *Reply) {
			r.PersistQuarantine.Ack = resealControlAck(t, r.PersistQuarantine.Ack, func(members map[string]any) {
				members["record_digest"] = digestBytes([]byte("persist-quarantine-mutation-probe"))
			})
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationPersistQuarantine,
			"sha256:1d2e1e7866728233e652ecb063b825969c7428115e1b4eaad15aa59f14e29a49", "execution-incomplete")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationPersistQuarantine,
			"sha256:1d2e1e7866728233e652ecb063b825969c7428115e1b4eaad15aa59f14e29a49", "execution-incomplete")
	})

	t.Run("persist-quarantine-wrong-record-kind", func(t *testing.T) {
		quarantine, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, "persist-quarantine.owner-reply.json")))
		if err != nil {
			t.Fatalf("DecodeReply(persist-quarantine.owner-reply.json): %v", err)
		}
		handback, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, "persist-handback.owner-reply.json")))
		if err != nil {
			t.Fatalf("DecodeReply(persist-handback.owner-reply.json): %v", err)
		}
		quarantine.PersistQuarantine.Ack = handback.PersistHandback.Ack
		reply, resultArm := structurallyValidReply(t, quarantine)
		requireRelationMismatch(t, ValidateRelations(reply), OperationPersistQuarantine)
		_, err = NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationPersistQuarantine)
	})

	t.Run("persist-abort record_digest", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "persist-abort.owner-reply.json", func(t *testing.T, r *Reply) {
			r.PersistAbort.Ack = resealControlAck(t, r.PersistAbort.Ack, func(members map[string]any) {
				members["record_digest"] = digestBytes([]byte("persist-abort-mutation-probe"))
			})
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationPersistAbort,
			"sha256:2426eceb7ec72554111ab089b29586c8f2a7759df52ed95555396e80ef2983c4", "decision-1")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationPersistAbort,
			"sha256:2426eceb7ec72554111ab089b29586c8f2a7759df52ed95555396e80ef2983c4", "decision-1")
	})

	t.Run("persist-abort-wrong-record-kind", func(t *testing.T) {
		abort, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, "persist-abort.owner-reply.json")))
		if err != nil {
			t.Fatalf("DecodeReply(persist-abort.owner-reply.json): %v", err)
		}
		handback, err := DecodeReply(bytes.NewReader(readPublicContractFixture(t, "persist-handback.owner-reply.json")))
		if err != nil {
			t.Fatalf("DecodeReply(persist-handback.owner-reply.json): %v", err)
		}
		abort.PersistAbort.Ack = handback.PersistHandback.Ack
		reply, resultArm := structurallyValidReply(t, abort)
		requireRelationMismatch(t, ValidateRelations(reply), OperationPersistAbort)
		_, err = NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationPersistAbort)
	})

	// persist-*: the remaining five members of validateControlAckRelation's
	// eight-member comparison (relations.go:438-440), run over all three
	// operations that share the function. record_digest has its own subtest per
	// operation above, and record_schema is covered by the three
	// *-wrong-record-kind swaps.
	//
	// disposition cannot be mutated on its own: validateNestedControlAck binds
	// it to the acknowledgment's record_schema through dispositionForRecordSchema
	// (codec.go:2567-2574, :2581-2592), and the relation's own expected
	// (record_schema, disposition) pair is that same 1:1 mapping, so on a
	// structurally valid acknowledgment the disposition clause is true exactly
	// when the record_schema clause is. The *-wrong-record-kind swaps reach the
	// pair jointly, which is the only reachable way to reach it at all.
	//
	// Each mutation is resealed (resealControlAck recomputes the self-digest
	// validateNestedSelfDigest requires, codec.go:2389-2405), so the mutant is a
	// genuinely valid control-ack document and the refusal is the relation's.
	for _, operation := range []struct {
		operation Operation
		fixture   string
		ack       func(reply *Reply) *json.RawMessage
	}{
		{OperationPersistHandback, "persist-handback.owner-reply.json",
			func(reply *Reply) *json.RawMessage { return &reply.PersistHandback.Ack }},
		{OperationPersistQuarantine, "persist-quarantine.owner-reply.json",
			func(reply *Reply) *json.RawMessage { return &reply.PersistQuarantine.Ack }},
		{OperationPersistAbort, "persist-abort.owner-reply.json",
			func(reply *Reply) *json.RawMessage { return &reply.PersistAbort.Ack }},
	} {
		operation := operation
		for _, member := range []struct{ name, value string }{
			{"flight", "flight-OTHER"},
			{"lane", "lane-OTHER"},
			{"epoch", "epoch-OTHER"},
			{"session", "session-OTHER"},
			{"workspace_id", "workspace-OTHER"},
		} {
			member := member
			t.Run(string(operation.operation)+" "+member.name, func(t *testing.T) {
				reply, resultArm := mutateReply(t, operation.fixture, func(t *testing.T, r *Reply) {
					ack := operation.ack(r)
					*ack = resealControlAck(t, *ack, func(members map[string]any) {
						members[member.name] = member.value
					})
				})
				requireRelationMismatch(t, ValidateRelations(reply), operation.operation, executionIdentityOperands...)
				_, err := NewReply(reply.Call, resultArm)
				requireRelationMismatch(t, err, operation.operation, executionIdentityOperands...)
			})
		}
	}
}

// TestValidateRelations_AcceptsStructurallyValidMatches proves the three
// relation clauses the frozen oracle leaves short-circuited are satisfied
// rather than merely skipped.
//
// The negatives above cannot prove this on their own: a relation function that
// answered ErrRelationMismatch unconditionally would pass every one of them.
// These cases pin the other direction -- a proven isolation that does name the
// queried profile, a persisted receipt digest that does equal the queried one,
// a non-null active revision whose acknowledgments do carry the request's
// execution key, and reported opaque identities that do match the declared row
// must all validate clean -- which is what makes the corresponding negatives
// attributable to the mutated member.
func TestValidateRelations_AcceptsStructurallyValidMatches(t *testing.T) {
	requireRelationsAccepted := func(t *testing.T, reply Reply, resultArm []byte) {
		t.Helper()
		if err := ValidateRelations(reply); err != nil {
			t.Fatalf("ValidateRelations = %v, want nil", err)
		}
		if _, err := NewReply(reply.Call, resultArm); err != nil {
			t.Fatalf("NewReply = %v, want nil", err)
		}
	}

	t.Run("recorder-checkpoint active revision carries the execution key", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "recorder-checkpoint.owner-reply.json", func(t *testing.T, r *Reply) {
			key := r.Call.RecorderCheckpoint.Key
			r.RecorderCheckpoint.Checkpoint.ActiveRevision = checkpointActiveRevision(key.Flight, key.Lane, key.Epoch)
		})
		if reply.RecorderCheckpoint.Checkpoint.ActiveRevision == nil {
			t.Fatalf("test setup: active revision is null, so the acknowledgment loop is never entered")
		}
		requireRelationsAccepted(t, reply, resultArm)
	})

	t.Run("resolve-receipt-verification-authority proven isolation names the queried profile", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "resolve-receipt-verification-authority.owner-reply.json", func(t *testing.T, r *Reply) {
			query := r.Call.ResolveReceiptVerificationAuthority.Query
			r.ResolveReceiptVerificationAuthority.Authority.Isolation =
				provenIsolationAuthority(query.ProfileRef.ID, query.ProfileRef.Digest)
		})
		if got := reply.ResolveReceiptVerificationAuthority.Authority.Isolation.State; got != contextreceipt.StateProven {
			t.Fatalf("test setup: isolation state = %q, want %q so the clause is entered", got, contextreceipt.StateProven)
		}
		requireRelationsAccepted(t, reply, resultArm)
	})

	t.Run("resolve-receipt-verification-authority persistence receipt matches the query", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "resolve-receipt-verification-authority.owner-reply.json", func(t *testing.T, r *Reply) {
			r.ResolveReceiptVerificationAuthority.Authority.Persistence =
				provenPersistenceAuthority(r.Call.ResolveReceiptVerificationAuthority.Query.ReceiptDigest)
		})
		if reply.ResolveReceiptVerificationAuthority.Authority.Persistence.ReceiptDigest == "" {
			t.Fatalf("test setup: persistence receipt_digest is empty, so the clause is never entered")
		}
		requireRelationsAccepted(t, reply, resultArm)
	})

	t.Run("verify-opaque-boundary identity matches the declared row", func(t *testing.T) {
		reply, resultArm := opaqueBoundaryPair(t, func(*OpaqueIdentity) {})
		if len(reply.Call.VerifyOpaqueBoundary.Rows) != 1 {
			t.Fatalf("test setup: %d declared rows, want exactly 1 so the per-row loop is entered",
				len(reply.Call.VerifyOpaqueBoundary.Rows))
		}
		requireRelationsAccepted(t, reply, resultArm)
	})
}
