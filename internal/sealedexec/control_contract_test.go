package sealedexec

import (
	"bytes"
	"testing"

	"github.com/jyang234/verdi/internal/contextevent"
)

func TestExecutionControlRecordContract_Static(t *testing.T) {
	t.Run("handback blank-digest canonical round trip", func(t *testing.T) {
		record := validHandbackRecord(t)
		encoded, err := EncodeHandbackRecord(record)
		if err != nil {
			t.Fatalf("EncodeHandbackRecord: %v", err)
		}
		assertCanonicalControlBytes(t, encoded)
		decoded, err := DecodeHandbackRecord(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("DecodeHandbackRecord: %v", err)
		}
		if decoded.Digest == "" || decoded.Digest == record.Digest {
			t.Fatalf("handback digest was not populated from blank preimage: %q", decoded.Digest)
		}
		ack := mustCanonicalControlAck(t, validControlAckForHandback(decoded))
		if err := ValidateHandbackAck(decoded, ack); err != nil {
			t.Fatalf("ValidateHandbackAck: %v", err)
		}
	})

	t.Run("all quarantine reasons and observation unions", func(t *testing.T) {
		reasons := []QuarantineReason{
			QuarantineRunwayDirty, QuarantineRunwayMoved, QuarantineChildDirty,
			QuarantineNonDescendant, QuarantineProtectedSpecChange,
			QuarantineFastForwardFailed, QuarantinePostVerificationMismatch,
			QuarantineNonAuthoritative, QuarantineExecutionIncomplete,
			QuarantineTerminalDurabilityFailed, QuarantineOutputWriteFailed,
		}
		if got, want := len(reasons), 11; got != want {
			t.Fatalf("quarantine reason count = %d, want %d", got, want)
		}
		seenReceipt := map[QuarantineReceiptState]bool{}
		seenOutput := map[QuarantineOutputState]bool{}
		seenObservation := map[RepositoryObservationState]bool{}
		seenProof := map[ProofState]bool{}
		seenPreserved := map[PreservedState]bool{}
		for _, reason := range reasons {
			reason := reason
			t.Run(string(reason), func(t *testing.T) {
				record := validQuarantineRecord(t, reason)
				encoded, err := EncodeQuarantineRecord(record)
				if err != nil {
					t.Fatalf("EncodeQuarantineRecord: %v", err)
				}
				assertCanonicalControlBytes(t, encoded)
				decoded, err := DecodeQuarantineRecord(bytes.NewReader(encoded))
				if err != nil {
					t.Fatalf("DecodeQuarantineRecord: %v", err)
				}
				seenReceipt[decoded.Receipt.State] = true
				seenOutput[decoded.Repository.Output.State] = true
				seenObservation[decoded.Observed.Runway.State] = true
				seenObservation[decoded.Observed.Child.State] = true
				seenObservation[decoded.Observed.PostRunway.State] = true
				seenProof[decoded.Observed.Descendant.State] = true
				seenPreserved[decoded.Preserved.State] = true
				ack := mustCanonicalControlAck(t, validControlAckForQuarantine(decoded))
				if err := ValidateQuarantineAck(decoded, ack); err != nil {
					t.Fatalf("ValidateQuarantineAck: %v", err)
				}
			})
		}
		for _, state := range []QuarantineReceiptState{QuarantineReceiptAbsent, QuarantineReceiptDurable} {
			if !seenReceipt[state] {
				t.Errorf("receipt state %q not exercised", state)
			}
		}
		for _, state := range []QuarantineOutputState{QuarantineOutputAbsent, QuarantineOutputObserved} {
			if !seenOutput[state] {
				t.Errorf("output state %q not exercised", state)
			}
		}
		for _, state := range []RepositoryObservationState{RepositoryObserved, RepositoryUnproven} {
			if !seenObservation[state] {
				t.Errorf("repository observation state %q not exercised", state)
			}
		}
		for _, state := range []ProofState{ProofProven, ProofViolatedWithWitness, ProofUnproven} {
			if !seenProof[state] {
				t.Errorf("proof state %q not exercised", state)
			}
		}
		for _, state := range []PreservedState{PreservedNone, PreservedPartial, PreservedFinalized} {
			if !seenPreserved[state] {
				t.Errorf("preserved state %q not exercised", state)
			}
		}
	})

	t.Run("abort and all matching ack dispositions", func(t *testing.T) {
		quarantine := mustCanonicalQuarantine(t, validQuarantineRecord(t, QuarantineTerminalDurabilityFailed))
		abort := mustCanonicalAbort(t, validAbortRecord(t, quarantine))
		if err := ValidateAbortAgainstQuarantine(abort, quarantine); err != nil {
			t.Fatalf("ValidateAbortAgainstQuarantine: %v", err)
		}
		ack := mustCanonicalControlAck(t, validControlAckForAbort(abort))
		if err := ValidateAbortAck(abort, ack); err != nil {
			t.Fatalf("ValidateAbortAck: %v", err)
		}

		for _, fixture := range []struct {
			name string
			ack  ControlAck
		}{
			{name: "fast-forwarded", ack: validControlAckForHandback(mustCanonicalHandback(t, validHandbackRecord(t)))},
			{name: "quarantined", ack: validControlAckForQuarantine(quarantine)},
			{name: "abort-preserve", ack: validControlAckForAbort(abort)},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				encoded, err := EncodeControlAck(fixture.ack)
				if err != nil {
					t.Fatalf("EncodeControlAck: %v", err)
				}
				if _, err := DecodeControlAck(bytes.NewReader(encoded)); err != nil {
					t.Fatalf("DecodeControlAck: %v", err)
				}
			})
		}
	})

	t.Run("pre-session partial preservation remains representable", func(t *testing.T) {
		record := validQuarantineRecord(t, QuarantineExecutionIncomplete)
		partial := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/pre-session-partial", Digest: testDigest("pre-session-partial")}
		record.Preserved = PreservedExecution{State: PreservedPartial, Ref: &partial}
		if _, err := EncodeQuarantineRecord(record); err != nil {
			t.Fatalf("EncodeQuarantineRecord(pre-session partial): %v", err)
		}
	})

	t.Run("shallow protected spec path round trips canonically", func(t *testing.T) {
		record := validQuarantineRecord(t, QuarantineProtectedSpecChange)
		record.Observed.ProtectedPaths = []string{".verdi/specs/example/spec.md"}
		encoded, err := EncodeQuarantineRecord(record)
		if err != nil {
			t.Fatalf("EncodeQuarantineRecord(shallow protected path): %v", err)
		}
		decoded, err := DecodeQuarantineRecord(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("DecodeQuarantineRecord(shallow protected path): %v", err)
		}
		if got, want := decoded.Observed.ProtectedPaths, []string{".verdi/specs/example/spec.md"}; len(got) != 1 || got[0] != want[0] {
			t.Fatalf("protected paths = %q, want %q", got, want)
		}
	})

	t.Run("wire mutations fail closed", func(t *testing.T) {
		handback := mustEncodeHandback(t, validHandbackRecord(t))
		quarantine := mustEncodeQuarantine(t, validQuarantineRecord(t, QuarantineProtectedSpecChange))
		canonicalQuarantine := mustCanonicalQuarantine(t, validQuarantineRecord(t, QuarantineTerminalDurabilityFailed))
		abort := mustEncodeAbort(t, validAbortRecord(t, canonicalQuarantine))
		ack := mustEncodeControlAck(t, validControlAckForQuarantine(canonicalQuarantine))

		for name, mutation := range map[string][]byte{
			"unknown":                     bytes.Replace(handback, []byte(`"schema":`), []byte(`"future":true,"schema":`), 1),
			"duplicate":                   bytes.Replace(handback, []byte(`"flight":"flight-1"`), []byte(`"flight":"flight-1","flight":"flight-1"`), 1),
			"null":                        bytes.Replace(handback, []byte(`"input":{`), []byte(`"input":null,"discard":{`), 1),
			"trailing":                    append(append([]byte(nil), handback...), []byte("{}\n")...),
			"noncanonical":                append([]byte(" "), handback...),
			"wrong blank digest preimage": bytes.Replace(handback, []byte(`"digest":"sha256:`), []byte(`"digest":"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff","discard":"sha256:`), 1),
		} {
			t.Run("handback/"+name, func(t *testing.T) {
				if _, err := DecodeHandbackRecord(bytes.NewReader(mutation)); err == nil {
					t.Fatalf("accepted %s mutation: %q", name, mutation)
				}
			})
		}
		for name, mutation := range map[string][]byte{
			"unknown":      bytes.Replace(quarantine, []byte(`"reason":`), []byte(`"future":true,"reason":`), 1),
			"duplicate":    bytes.Replace(quarantine, []byte(`"reason":"protected-spec-change"`), []byte(`"reason":"protected-spec-change","reason":"protected-spec-change"`), 1),
			"null":         bytes.Replace(quarantine, []byte(`"preserved":{`), []byte(`"preserved":null,"discard":{`), 1),
			"noncanonical": append([]byte(" "), quarantine...),
		} {
			t.Run("quarantine/"+name, func(t *testing.T) {
				if _, err := DecodeQuarantineRecord(bytes.NewReader(mutation)); err == nil {
					t.Fatalf("accepted %s mutation: %q", name, mutation)
				}
			})
		}
		for name, raw := range map[string][]byte{"abort": abort, "ack": ack} {
			mutation := append([]byte(" "), raw...)
			t.Run(name+"/noncanonical", func(t *testing.T) {
				var err error
				if name == "abort" {
					_, err = DecodeAbortRecord(bytes.NewReader(mutation))
				} else {
					_, err = DecodeControlAck(bytes.NewReader(mutation))
				}
				if err == nil {
					t.Fatalf("accepted noncanonical %s", name)
				}
			})
		}
	})

	t.Run("semantic contradictions fail closed", func(t *testing.T) {
		handback := validHandbackRecord(t)
		for name, mutate := range map[string]func(*HandbackRecord){
			"bad git":           func(record *HandbackRecord) { record.Output.Commit = "ABC" },
			"wrong disposition": func(record *HandbackRecord) { record.Disposition = ControlDispositionQuarantined },
			"pre mismatch":      func(record *HandbackRecord) { record.PreRunway.Head = testSHA2 },
			"post mismatch":     func(record *HandbackRecord) { record.PostRunway.Head = testSHA1 },
			"stale digest":      func(record *HandbackRecord) { record.Digest = testDigest("wrong") },
		} {
			t.Run("handback/"+name, func(t *testing.T) {
				bad := handback
				mutate(&bad)
				if _, err := EncodeHandbackRecord(bad); err == nil {
					t.Fatalf("accepted %s", name)
				}
			})
		}

		quarantine := validQuarantineRecord(t, QuarantineProtectedSpecChange)
		for name, mutate := range map[string]func(*QuarantineRecord){
			"outside protected boundary": func(record *QuarantineRecord) {
				record.Observed.ProtectedPaths = []string{".verdi/specs/example/notes.md"}
			},
			"unclean protected path": func(record *QuarantineRecord) {
				record.Observed.ProtectedPaths = []string{".verdi/specs/example/../spec.md"}
			},
			"unsorted protected paths": func(record *QuarantineRecord) {
				record.Observed.ProtectedPaths = []string{".verdi/specs/z/spec.md", ".verdi/specs/a/spec.md"}
			},
			"duplicate protected paths": func(record *QuarantineRecord) {
				record.Observed.ProtectedPaths = []string{".verdi/specs/x/spec.md", ".verdi/specs/x/spec.md"}
			},
			"wrong reason facts": func(record *QuarantineRecord) { record.Reason = QuarantineNonDescendant },
			"durable without finalized": func(record *QuarantineRecord) {
				record.Preserved = PreservedExecution{State: PreservedPartial, Ref: record.Preserved.Ref}
			},
			"receipt mismatch":  func(record *QuarantineRecord) { record.Receipt.EventAck.ReceiptDigest = testDigest("other") },
			"bad preserved ref": func(record *QuarantineRecord) { record.Preserved.Ref.Schema = "future" },
			"unsorted proof witnesses": func(record *QuarantineRecord) {
				record.Observed.Descendant = Proof{State: ProofViolatedWithWitness, Witnesses: []string{"z", "a"}}
			},
		} {
			t.Run("quarantine/"+name, func(t *testing.T) {
				bad := quarantine
				bad.Observed.ProtectedPaths = append([]string(nil), quarantine.Observed.ProtectedPaths...)
				mutate(&bad)
				if _, err := EncodeQuarantineRecord(bad); err == nil {
					t.Fatalf("accepted %s", name)
				}
			})
		}

		precheckWithoutDurableResult := validQuarantineRecord(t, QuarantineRunwayDirty)
		precheckWithoutDurableResult.Receipt = QuarantineReceipt{State: QuarantineReceiptAbsent}
		partial := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/invalid-precheck-partial", Digest: testDigest("invalid-precheck-partial")}
		precheckWithoutDurableResult.Preserved = PreservedExecution{State: PreservedPartial, Ref: &partial}
		if _, err := EncodeQuarantineRecord(precheckWithoutDurableResult); err == nil {
			t.Fatal("accepted factual handback quarantine without durable finalized result")
		}

		canonical := mustCanonicalQuarantine(t, validQuarantineRecord(t, QuarantineTerminalDurabilityFailed))
		abortRecord := mustCanonicalAbort(t, validAbortRecord(t, canonical))
		badAbort := abortRecord
		badAbort.Preserved.Digest = testDigest("different-preserved")
		badAbort.Digest = ""
		badAbort = mustCanonicalAbort(t, badAbort)
		if err := ValidateAbortAgainstQuarantine(badAbort, canonical); err == nil {
			t.Fatal("abort accepted a preserved reference that conflicts with quarantine")
		}

		ackRecord := mustCanonicalControlAck(t, validControlAckForQuarantine(canonical))
		ackRecord.WorkspaceID = "other-workspace"
		ackRecord.Digest = ""
		ackRecord = mustCanonicalControlAck(t, ackRecord)
		if err := ValidateQuarantineAck(canonical, ackRecord); err == nil {
			t.Fatal("ack accepted conflicting record identity")
		}

		canonicalHandback := mustCanonicalHandback(t, validHandbackRecord(t))
		handbackAck := mustCanonicalControlAck(t, validControlAckForHandback(canonicalHandback))
		canonicalHandback.Output.Tree = "ABC"
		if err := ValidateHandbackAck(canonicalHandback, handbackAck); err == nil {
			t.Fatal("ack validation accepted an invalid/stale persisted record")
		}
	})
}

func assertCanonicalControlBytes(t *testing.T, data []byte) {
	t.Helper()
	if !bytes.HasSuffix(data, []byte("\n")) || bytes.HasSuffix(data, []byte("\n\n")) {
		t.Fatalf("not exactly one trailing LF: %q", data)
	}
	if bytes.Contains(data, []byte(":null")) {
		t.Fatalf("canonical control bytes contain null: %s", data)
	}
}

func validReceiptEventAck() contextevent.ReceiptEventAck {
	return contextevent.ReceiptEventAck{Schema: contextevent.ReceiptAckSchemaID, Flight: "flight-1", Lane: "lane-1", Epoch: "epoch-1", Session: "session-1", ManifestRevision: 1, Kind: contextevent.KindReceipt, SourceSequence: 8, EventDigest: testDigest("receipt-event"), GlobalSequence: 21, ReceiptDigest: testDigest("receipt")}
}

func validHandbackRecord(t *testing.T) HandbackRecord {
	t.Helper()
	ack := validReceiptEventAck()
	return HandbackRecord{Schema: ExecutionHandbackSchemaID, Flight: ack.Flight, Lane: ack.Lane, Epoch: ack.Epoch, Session: ack.Session, ATCRunway: ".vatc", WorkspaceID: "workspace-1", Receipt: DurableReceipt{Digest: ack.ReceiptDigest, EventAck: ack}, Input: GitIdentity{Commit: testSHA1, Tree: testTree1}, Output: GitIdentity{Commit: testSHA2, Tree: testTree2}, PreRunway: RunwayState{Head: testSHA1, Tree: testTree1, Clean: true}, PostRunway: RunwayState{Head: testSHA2, Tree: testTree2, Clean: true}, Disposition: ControlDispositionFastForwarded}
}

func validQuarantineRecord(t *testing.T, reason QuarantineReason) QuarantineRecord {
	t.Helper()
	ack := validReceiptEventAck()
	unprovenObservation := RepoObservation{State: RepositoryUnproven}
	unprovenProof := Proof{State: ProofUnproven, Witnesses: []string{"not observed"}}
	finalizedRef := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/execution-1", Digest: testDigest("finalized-result")}
	record := QuarantineRecord{Schema: ExecutionQuarantineSchemaID, Flight: ack.Flight, Lane: ack.Lane, Epoch: ack.Epoch, Session: ack.Session, ATCRunway: ".vatc", WorkspaceID: "workspace-1", Receipt: QuarantineReceipt{State: QuarantineReceiptDurable, Digest: ack.ReceiptDigest, EventAck: &ack}, Repository: QuarantineRepository{Input: GitIdentity{Commit: testSHA1, Tree: testTree1}, Output: QuarantineOutput{State: QuarantineOutputObserved, Commit: testSHA2, Tree: testTree2}}, Observed: QuarantineObservations{Runway: unprovenObservation, Child: unprovenObservation, Descendant: unprovenProof, ProtectedPaths: []string{}, FastForward: FastForwardNotAttempted, PostRunway: unprovenObservation}, Reason: reason, Preserved: PreservedExecution{State: PreservedFinalized, Ref: &finalizedRef}}

	observedInput := RepoObservation{State: RepositoryObserved, Commit: testSHA1, Tree: testTree1, Clean: true}
	observedChild := RepoObservation{State: RepositoryObserved, Commit: testSHA2, Tree: testTree2, Clean: true}
	proven := Proof{State: ProofProven, Witnesses: []string{}}
	switch reason {
	case QuarantineRunwayDirty:
		record.Observed.Runway = RepoObservation{State: RepositoryObserved, Commit: testSHA1, Tree: testTree1, Clean: false}
	case QuarantineRunwayMoved:
		record.Observed.Runway = RepoObservation{State: RepositoryObserved, Commit: testSHA2, Tree: testTree2, Clean: true}
	case QuarantineChildDirty:
		record.Observed.Runway = observedInput
		record.Observed.Child = RepoObservation{State: RepositoryObserved, Commit: testSHA2, Tree: testTree2, Clean: false}
	case QuarantineNonDescendant:
		record.Observed.Runway, record.Observed.Child = observedInput, observedChild
		record.Observed.Descendant = Proof{State: ProofViolatedWithWitness, Witnesses: []string{"output is not a descendant"}}
	case QuarantineProtectedSpecChange:
		record.Observed.Runway, record.Observed.Child, record.Observed.Descendant = observedInput, observedChild, proven
		record.Observed.ProtectedPaths = []string{".verdi/specs/active/example/spec.md"}
	case QuarantineFastForwardFailed:
		record.Observed.Runway, record.Observed.Child, record.Observed.Descendant = observedInput, observedChild, proven
		record.Observed.FastForward = FastForwardFailed
	case QuarantinePostVerificationMismatch:
		record.Observed.Runway, record.Observed.Child, record.Observed.Descendant = observedInput, observedChild, proven
		record.Observed.FastForward = FastForwardSucceeded
		record.Observed.PostRunway = RepoObservation{State: RepositoryObserved, Commit: testSHA2, Tree: testTree1, Clean: true}
	case QuarantineNonAuthoritative:
		// No handback observations are made for an advisory finalized result.
	case QuarantineExecutionIncomplete:
		record.Receipt = QuarantineReceipt{State: QuarantineReceiptAbsent}
		record.Repository.Output = QuarantineOutput{State: QuarantineOutputAbsent}
		record.Preserved = PreservedExecution{State: PreservedNone}
		record.Session = ""
	case QuarantineTerminalDurabilityFailed:
		record.Receipt = QuarantineReceipt{State: QuarantineReceiptAbsent}
		partialRef := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/execution-partial", Digest: testDigest("partial-result")}
		record.Preserved = PreservedExecution{State: PreservedPartial, Ref: &partialRef}
	case QuarantineOutputWriteFailed:
		// Durable finalized result, but handback observations remain unproven.
	default:
		t.Fatalf("unknown quarantine reason %q", reason)
	}
	return record
}

func validAbortRecord(t *testing.T, quarantine QuarantineRecord) AbortRecord {
	t.Helper()
	if quarantine.Digest == "" {
		t.Fatal("validAbortRecord requires canonical quarantine")
	}
	ref := PreservedExecutionRef{Schema: PreservedExecutionRefSchemaID, ID: "controller/execution-abort", Digest: testDigest("abort-preserved")}
	if quarantine.Preserved.Ref != nil {
		ref = *quarantine.Preserved.Ref
	}
	return AbortRecord{Schema: ExecutionAbortSchemaID, Flight: quarantine.Flight, Lane: quarantine.Lane, Epoch: quarantine.Epoch, Session: quarantine.Session, WorkspaceID: quarantine.WorkspaceID, QuarantineDigest: quarantine.Digest, OwnerDecision: LogicalRef{Schema: "verdi.owner-decision-ref/v1", ID: "decision-1", Digest: testDigest("decision")}, Preserved: ref, Disposition: ControlDispositionAbortPreserve}
}

func validControlAckForHandback(record HandbackRecord) ControlAck {
	return ControlAck{Schema: ExecutionControlAckSchemaID, RecordSchema: record.Schema, RecordDigest: record.Digest, Flight: record.Flight, Lane: record.Lane, Epoch: record.Epoch, Session: record.Session, WorkspaceID: record.WorkspaceID, Disposition: ControlDispositionFastForwarded, ControllerGlobalSequence: 1}
}

func validControlAckForQuarantine(record QuarantineRecord) ControlAck {
	return ControlAck{Schema: ExecutionControlAckSchemaID, RecordSchema: record.Schema, RecordDigest: record.Digest, Flight: record.Flight, Lane: record.Lane, Epoch: record.Epoch, Session: record.Session, WorkspaceID: record.WorkspaceID, Disposition: ControlDispositionQuarantined, ControllerGlobalSequence: 2}
}

func validControlAckForAbort(record AbortRecord) ControlAck {
	return ControlAck{Schema: ExecutionControlAckSchemaID, RecordSchema: record.Schema, RecordDigest: record.Digest, Flight: record.Flight, Lane: record.Lane, Epoch: record.Epoch, Session: record.Session, WorkspaceID: record.WorkspaceID, Disposition: ControlDispositionAbortPreserve, ControllerGlobalSequence: 3}
}

func mustEncodeHandback(t *testing.T, record HandbackRecord) []byte {
	t.Helper()
	data, err := EncodeHandbackRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustCanonicalHandback(t *testing.T, record HandbackRecord) HandbackRecord {
	t.Helper()
	decoded, err := DecodeHandbackRecord(bytes.NewReader(mustEncodeHandback(t, record)))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func mustEncodeQuarantine(t *testing.T, record QuarantineRecord) []byte {
	t.Helper()
	data, err := EncodeQuarantineRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustCanonicalQuarantine(t *testing.T, record QuarantineRecord) QuarantineRecord {
	t.Helper()
	decoded, err := DecodeQuarantineRecord(bytes.NewReader(mustEncodeQuarantine(t, record)))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func mustEncodeAbort(t *testing.T, record AbortRecord) []byte {
	t.Helper()
	data, err := EncodeAbortRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustCanonicalAbort(t *testing.T, record AbortRecord) AbortRecord {
	t.Helper()
	decoded, err := DecodeAbortRecord(bytes.NewReader(mustEncodeAbort(t, record)))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func mustEncodeControlAck(t *testing.T, ack ControlAck) []byte {
	t.Helper()
	data, err := EncodeControlAck(ack)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func mustCanonicalControlAck(t *testing.T, ack ControlAck) ControlAck {
	t.Helper()
	decoded, err := DecodeControlAck(bytes.NewReader(mustEncodeControlAck(t, ack)))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}
