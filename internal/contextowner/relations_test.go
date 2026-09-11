package contextowner

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
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

func TestValidateRelations_RejectsStructurallyValidMismatches(t *testing.T) {
	t.Run("verify-authority", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "verify-authority.owner-reply.json", func(t *testing.T, r *Reply) {
			r.VerifyAuthority.Facts.ManifestRevision++
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationVerifyAuthority,
			"sha256:77a2c3dc80d395e7b382dc4198f4d5608f5fc410761bf527ce638784226bfb82",
			"1111111111111111111111111111111111111111")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationVerifyAuthority,
			"sha256:77a2c3dc80d395e7b382dc4198f4d5608f5fc410761bf527ce638784226bfb82",
			"1111111111111111111111111111111111111111")
	})

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

	t.Run("recorder-checkpoint", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "recorder-checkpoint.owner-reply.json", func(t *testing.T, r *Reply) {
			eventDigest := digestBytes([]byte("recorder-checkpoint-active-revision-event"))
			r.RecorderCheckpoint.Checkpoint.ActiveRevision = &ActiveRevision{
				Revision:           1,
				ManifestDigest:     digestBytes([]byte("recorder-checkpoint-active-revision-manifest")),
				NextSourceSequence: 2,
				PriorEventDigest:   eventDigest,
				LastGlobalSequence: 2,
				EventAcks: []contextevent.EventAck{{
					Schema: contextevent.AckSchemaID, Flight: "flight-OTHER", Lane: "lane-1", Epoch: "epoch-1",
					Session: "session-1", ManifestRevision: 1, Kind: contextevent.KindAdapterStop,
					SourceSequence: 1, EventDigest: eventDigest, GlobalSequence: 2,
				}},
			}
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationRecorderCheckpoint,
			"flight-1", "sha256:47320987f9a49d5b00119b960f247a956773f57543982b8bfcb6da5bb3afd9ef")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationRecorderCheckpoint,
			"flight-1", "sha256:47320987f9a49d5b00119b960f247a956773f57543982b8bfcb6da5bb3afd9ef")
	})

	t.Run("recorder-append", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "recorder-append.owner-reply.json", func(t *testing.T, r *Reply) {
			r.RecorderAppend.Ack.EventDigest = digestBytes([]byte("recorder-append-mutation-probe"))
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationRecorderAppend,
			"sha256:301ed869bd08ab15d310ea77164faf5b23bda12d0d7a80280ef06021920f43f3",
			"vatc-ea6ea82dda1fdda1361db233a4cb62d0d570505601212b34deb395786c8f5916--111111111111")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationRecorderAppend,
			"sha256:301ed869bd08ab15d310ea77164faf5b23bda12d0d7a80280ef06021920f43f3",
			"vatc-ea6ea82dda1fdda1361db233a4cb62d0d570505601212b34deb395786c8f5916--111111111111")
	})

	t.Run("store-redacted-segment", func(t *testing.T) {
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
		requireRelationMismatch(t, ValidateRelations(reply), OperationStoreRedactedSegment,
			"sha256:ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed",
			"controller-segment/sha256/ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationStoreRedactedSegment,
			"sha256:ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed",
			"controller-segment/sha256/ecf59a2696ca44a417e20e2a7eabb1b26e82c779f8546bea354a2cc80e8e1eed")
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

	t.Run("verify-opaque-boundary", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "verify-opaque-boundary.owner-reply.json", func(t *testing.T, r *Reply) {
			r.VerifyOpaqueBoundary.Facts.Rows = append(r.VerifyOpaqueBoundary.Facts.Rows, OpaqueIdentity{
				ID: "opaque-mutation-probe", Kind: "harness-vendor-base", AdapterID: "codex", AdapterVersion: "1.0.0",
			})
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationVerifyOpaqueBoundary,
			"sha256:00de1d1d9277138da6fa5e753782a551dd291a9ddefdc22b57c92468551fc7c7",
			"verdi.context-owner/verify-opaque-boundary-result/v1")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationVerifyOpaqueBoundary,
			"sha256:00de1d1d9277138da6fa5e753782a551dd291a9ddefdc22b57c92468551fc7c7",
			"verdi.context-owner/verify-opaque-boundary-result/v1")
	})

	t.Run("verify-provider-session", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "verify-provider-session.owner-reply.json", func(t *testing.T, r *Reply) {
			r.VerifyProviderSession.Facts.SessionRef = "mutation-probe-session"
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationVerifyProviderSession,
			"sha256:1900eab6c028483d7126599ee6f50de0d27907b5c65fa90524580b4b0f9852b0", "workspace-1")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationVerifyProviderSession,
			"sha256:1900eab6c028483d7126599ee6f50de0d27907b5c65fa90524580b4b0f9852b0", "workspace-1")
	})

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

	t.Run("append-receipt", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "append-receipt.owner-reply.json", func(t *testing.T, r *Reply) {
			r.AppendReceipt.Ack.ReceiptDigest = digestBytes([]byte("append-receipt-mutation-probe"))
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationAppendReceipt,
			"sha256:95de75491558577517c2524ed1c14bcc3dc04686ec0e1ab9c0a74e116612837c",
			"sha256:58c0f7fd836eddcb59a05a3b5fbca9bd616a8604c31548b7163f1ac0928bf969")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationAppendReceipt,
			"sha256:95de75491558577517c2524ed1c14bcc3dc04686ec0e1ab9c0a74e116612837c",
			"sha256:58c0f7fd836eddcb59a05a3b5fbca9bd616a8604c31548b7163f1ac0928bf969")
	})

	t.Run("resolve-receipt-verification-authority", func(t *testing.T) {
		reply, resultArm := mutateReply(t, "resolve-receipt-verification-authority.owner-reply.json", func(t *testing.T, r *Reply) {
			r.ResolveReceiptVerificationAuthority.Authority.TrustFact.SourceID = "mutation-probe"
		})
		requireRelationMismatch(t, ValidateRelations(reply), OperationResolveReceiptVerificationAuthority,
			"fixture", "project-profile")
		_, err := NewReply(reply.Call, resultArm)
		requireRelationMismatch(t, err, OperationResolveReceiptVerificationAuthority,
			"fixture", "project-profile")
	})

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
}
