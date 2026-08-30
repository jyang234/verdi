package sealedexec

import (
	"bytes"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextevent"
)

func TestExecutionControlRecordContract_Static(t *testing.T) {
	t.Run("execution partial is strict canonical actual-run state", func(t *testing.T) {
		fixture := newHandbackFixture(t, contextevent.AuthorityAuthoritative)
		encoded, err := EncodeExecutionPartial(fixture.request, fixture.run)
		if err != nil {
			t.Fatalf("EncodeExecutionPartial: %v", err)
		}
		assertCanonicalControlBytes(t, encoded)
		decoded, err := DecodeExecutionPartial(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("DecodeExecutionPartial: %v", err)
		}
		if decoded.Schema != ExecutionPartialSchemaID || decoded.Flight != fixture.request.Flight ||
			decoded.Lane != fixture.request.Lane || decoded.Epoch != fixture.request.Epoch ||
			decoded.Session != fixture.request.Session || decoded.Action != fixture.request.Action ||
			decoded.ManifestRevision != fixture.request.ManifestRevision || decoded.ManifestDigest != fixture.request.ManifestDigest ||
			decoded.Adapter != fixture.request.Adapter || decoded.AdapterVersion != fixture.request.AdapterVersion ||
			decoded.WorkspaceID != fixture.run.Workspace.WorkspaceID || decoded.AdapterSessionRef != fixture.run.AdapterSessionRef ||
			decoded.Authority != fixture.run.Authority || len(decoded.EventAcks) != len(fixture.run.Acks) {
			t.Fatalf("decoded execution partial lost request/run identity: %#v", decoded)
		}
		for name, mutation := range map[string][]byte{
			"unknown":      bytes.Replace(encoded, []byte(`"schema":`), []byte(`"future":true,"schema":`), 1),
			"null acks":    bytes.Replace(encoded, []byte(`"event_acks":[`), []byte(`"event_acks":null,"discard":[`), 1),
			"noncanonical": append([]byte(" "), encoded...),
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := DecodeExecutionPartial(bytes.NewReader(mutation)); err == nil {
					t.Fatalf("DecodeExecutionPartial accepted %s mutation", name)
				}
			})
		}

		badRun := fixture.run
		badRun.Acks = append([]contextevent.EventAck(nil), fixture.run.Acks...)
		badRun.Acks[0].Session = "other-session"
		if _, err := EncodeExecutionPartial(fixture.request, badRun); err == nil {
			t.Fatal("EncodeExecutionPartial accepted an acknowledgment from another run")
		}
	})

	// I-117/SI-164: after an embedded scoped-MCP expansion the shared flight
	// state's terminal manifest — not the dispatched request manifest — is what
	// the preserved partial represents, and its complete acknowledgment stream is
	// validated with exactly the authority successful completion applies.
	t.Run("execution partial represents the shared post-expansion terminal", func(t *testing.T) {
		fixture := newExpandedCompletionFixture(t)
		request, run := fixture.request, fixture.run
		parent, child := request.ManifestRevision, run.Terminal.Revision
		if child != parent+1 || run.Terminal.ManifestDigest == request.ManifestDigest {
			t.Fatalf("expanded fixture terminal = revision %d digest %q, want the installed child past request revision %d",
				child, run.Terminal.ManifestDigest, parent)
		}
		encoded, err := EncodeExecutionPartial(request, run)
		if err != nil {
			t.Fatalf("EncodeExecutionPartial: %v", err)
		}
		assertCanonicalControlBytes(t, encoded)
		decoded, err := DecodeExecutionPartial(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("DecodeExecutionPartial: %v", err)
		}
		// The partial binds the actual post-expansion manifest, and no
		// acknowledged MCP-owned event was dropped to fit the original request.
		if decoded.ManifestRevision != child || decoded.ManifestDigest != run.Terminal.ManifestDigest {
			t.Fatalf("partial manifest = revision %d digest %q, want the terminal revision %d digest %q",
				decoded.ManifestRevision, decoded.ManifestDigest, child, run.Terminal.ManifestDigest)
		}
		if len(decoded.EventAcks) != len(run.Acks) {
			t.Fatalf("partial carries %d acknowledgments, want the complete stream of %d", len(decoded.EventAcks), len(run.Acks))
		}
		// The child revision is exactly the successor and restarts source order.
		crossed := 0
		for i, ack := range decoded.EventAcks {
			if ack != run.Acks[i] {
				t.Fatalf("partial acknowledgment %d = %#v, want %#v", i, ack, run.Acks[i])
			}
			if i > 0 && ack.ManifestRevision == decoded.EventAcks[i-1].ManifestRevision+1 {
				crossed++
				if ack.SourceSequence != 1 {
					t.Fatalf("child revision %d opened at source %d, want 1", ack.ManifestRevision, ack.SourceSequence)
				}
			}
		}
		if crossed != 1 || decoded.EventAcks[len(decoded.EventAcks)-1].ManifestRevision != child {
			t.Fatalf("partial stream crossed %d revisions ending at %d, want exactly one crossing into %d",
				crossed, decoded.EventAcks[len(decoded.EventAcks)-1].ManifestRevision, child)
		}

		// A decoded partial whose represented revision is stale against its own
		// acknowledged stream is refused: the last acknowledgment fixes it.
		stale := decoded
		stale.ManifestRevision = parent
		staleBytes, err := canonjson.Marshal(stale)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeExecutionPartial(bytes.NewReader(staleBytes)); err == nil {
			t.Fatal("DecodeExecutionPartial accepted a partial that predates its own acknowledged terminal revision")
		}

		for name, mutate := range map[string]func(*ExecutionRun){
			"same-revision source gap":                          func(r *ExecutionRun) { r.Acks[2].SourceSequence++ },
			"child revision that does not restart source order": func(r *ExecutionRun) { r.Acks[4].SourceSequence = 5 },
			"skipped child revision": func(r *ExecutionRun) {
				for i := 4; i < len(r.Acks); i++ {
					r.Acks[i].ManifestRevision = parent + 2
				}
				r.Terminal.Revision = parent + 2
			},
			"backward child revision":     func(r *ExecutionRun) { r.Acks[len(r.Acks)-1].ManifestRevision = parent },
			"non-increasing global order": func(r *ExecutionRun) { r.Acks[5].GlobalSequence = r.Acks[4].GlobalSequence },
			"stale terminal snapshot revision": func(r *ExecutionRun) {
				r.Terminal.Revision = parent
				r.Terminal.ManifestDigest = r.Terminal.Request.ManifestDigest
			},
			// These two are caught only by the terminal cross-match: the
			// represented revision still agrees with the stream, but the snapshot
			// the partial is built from did not reach it.
			"terminal snapshot behind the final acknowledgment": func(r *ExecutionRun) {
				r.Terminal.NextSourceSequence--
				r.Terminal.LastGlobalSequence--
			},
			"terminal snapshot from another flight": func(r *ExecutionRun) { r.Terminal.Key.Flight = "other-flight" },
		} {
			t.Run("refuses "+name, func(t *testing.T) {
				bad := run
				bad.Acks = append([]contextevent.EventAck(nil), run.Acks...)
				mutate(&bad)
				if _, err := EncodeExecutionPartial(request, bad); err == nil {
					t.Fatalf("EncodeExecutionPartial accepted %s", name)
				}
			})
		}
	})

	// I-118/SI-166: the atomic child-install/first-child-append boundary is a
	// closed partial-only exception. After `child-manifest` is acknowledged at
	// revision R and the install succeeds, the shared state opens R+1 at source
	// one with no prior-event digest and the unchanged global order, while the
	// acknowledged stream still ends at R. A failure there must preserve the
	// installed child manifest plus the complete parent prefix; successful
	// completion stays strict because it appends its terminal event on the
	// child.
	t.Run("execution partial preserves an installed child with no child acknowledgment", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
		request := fixture.request
		parent := request.ManifestRevision
		child := parent + 1
		childDigest := testDigest("installed-child-manifest")
		acks := []contextevent.EventAck{
			completionAck(request, parent, 1, 1, contextevent.KindAdapterStart, testDigest("adapter-start")),
			completionAck(request, parent, 2, 2, contextevent.KindContextRequest, testDigest("context-request")),
			completionAck(request, parent, 3, 3, contextevent.KindContextDecision, testDigest("context-decision")),
			completionAck(request, parent, 4, 4, contextevent.KindChildManifest, testDigest("child-manifest")),
		}
		last := acks[len(acks)-1]
		run := fixture.run
		run.Acks = acks
		run.Terminal = installedChildTerminal(request, childDigest, last)

		// Successful completion remains strict: its terminal cross-match still
		// refuses a snapshot the acknowledged stream has not reached.
		if err := validateRunTerminal(request, run.Terminal, last); err == nil {
			t.Fatal("completion's terminal validator accepted an installed child with no child acknowledgment")
		}

		encoded, err := EncodeExecutionPartial(request, run)
		if err != nil {
			t.Fatalf("EncodeExecutionPartial: %v", err)
		}
		assertCanonicalControlBytes(t, encoded)
		decoded, err := DecodeExecutionPartial(bytes.NewReader(encoded))
		if err != nil {
			t.Fatalf("DecodeExecutionPartial: %v", err)
		}
		// The installed child manifest is represented with the complete parent
		// acknowledgment prefix; nothing is dropped and nothing is invented.
		if decoded.ManifestRevision != child || decoded.ManifestDigest != childDigest {
			t.Fatalf("partial manifest = revision %d digest %q, want the installed child %d/%q",
				decoded.ManifestRevision, decoded.ManifestDigest, child, childDigest)
		}
		if len(decoded.EventAcks) != len(acks) {
			t.Fatalf("partial carries %d acknowledgments, want the complete parent prefix of %d", len(decoded.EventAcks), len(acks))
		}
		for i, ack := range decoded.EventAcks {
			if ack != acks[i] {
				t.Fatalf("partial acknowledgment %d = %#v, want %#v", i, ack, acks[i])
			}
		}
		if final := decoded.EventAcks[len(decoded.EventAcks)-1]; final.ManifestRevision != parent {
			t.Fatalf("partial stream ends at revision %d, want the parent %d", final.ManifestRevision, parent)
		}
		// Encoding is idempotent over its own decoded value.
		reencoded, err := canonjson.Marshal(decoded)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(reencoded, encoded) {
			t.Fatalf("execution partial round trip is not byte-stable:\n%s\n%s", encoded, reencoded)
		}

		// Every deviation from the exact opening position stays closed: the
		// arm admits one shape, not a class of stale terminals.
		for name, mutate := range map[string]func(*ExecutionRun){
			"absent bridge":              func(r *ExecutionRun) { r.Terminal.PriorRevision = nil },
			"bridge to another revision": func(r *ExecutionRun) { r.Terminal.PriorRevision.ManifestRevision = parent - 1 },
			"bridge to another event":    func(r *ExecutionRun) { r.Terminal.PriorRevision.EventRoot = testDigest("other-event") },
			"bridge to another source sequence": func(r *ExecutionRun) {
				r.Terminal.PriorRevision.TerminalSourceSequence = last.SourceSequence - 1
			},
			"bridge to another global sequence": func(r *ExecutionRun) {
				r.Terminal.PriorRevision.TerminalGlobalSequence = last.GlobalSequence - 1
			},
			"child source order already advanced": func(r *ExecutionRun) { r.Terminal.NextSourceSequence = 2 },
			"child carries a prior event digest":  func(r *ExecutionRun) { r.Terminal.PriorEventDigest = last.EventDigest },
			"global order already advanced":       func(r *ExecutionRun) { r.Terminal.LastGlobalSequence = last.GlobalSequence + 1 },
			"skipped child revision":              func(r *ExecutionRun) { r.Terminal.Revision = parent + 2 },
			"backward child revision":             func(r *ExecutionRun) { r.Terminal.Revision = parent },
			"empty installed manifest digest":     func(r *ExecutionRun) { r.Terminal.ManifestDigest = "" },
			"noncanonical installed digest":       func(r *ExecutionRun) { r.Terminal.ManifestDigest = "sha256:not-a-digest" },
			"snapshot from another flight":        func(r *ExecutionRun) { r.Terminal.Key.Flight = "other-flight" },
			"snapshot of another request revision": func(r *ExecutionRun) {
				r.Terminal.Request.ManifestRevision = parent + 1
			},
			"snapshot of another request manifest": func(r *ExecutionRun) {
				r.Terminal.Request.ManifestDigest = testDigest("other-manifest")
			},
		} {
			t.Run("refuses "+name, func(t *testing.T) {
				bad := run
				bad.Acks = append([]contextevent.EventAck(nil), acks...)
				bridge := *run.Terminal.PriorRevision
				bad.Terminal.PriorRevision = &bridge
				mutate(&bad)
				if _, err := EncodeExecutionPartial(request, bad); err == nil {
					t.Fatalf("EncodeExecutionPartial accepted %s", name)
				}
			})
		}

		// The private wire admits a last acknowledgment only on the represented
		// revision or its exact immediate predecessor. A skipped revision is
		// still refused on decode.
		skipped := decoded
		skipped.ManifestRevision = parent + 2
		skippedBytes, err := canonjson.Marshal(skipped)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeExecutionPartial(bytes.NewReader(skippedBytes)); err == nil {
			t.Fatal("DecodeExecutionPartial accepted a partial that skips a manifest revision past its stream")
		}
		// A backward representation is refused for the same reason.
		backward := decoded
		backward.ManifestRevision = parent - 1
		backwardBytes, err := canonjson.Marshal(backward)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeExecutionPartial(bytes.NewReader(backwardBytes)); err == nil {
			t.Fatal("DecodeExecutionPartial accepted a partial that predates its own acknowledged stream")
		}
	})

	// Pre-shared-state compatibility: a run that carries no terminal snapshot —
	// and a run that has not yet acknowledged anything — still preserves the
	// dispatched request manifest exactly as before.
	t.Run("execution partial without shared terminal state keeps the request manifest", func(t *testing.T) {
		fixture := newCompletionFixture(t, contextevent.AuthorityAuthoritative)
		for name, run := range map[string]ExecutionRun{
			"acknowledged stream without a snapshot": func() ExecutionRun {
				run := fixture.run
				run.Terminal = FlightStateSnapshot{}
				return run
			}(),
			"empty stream": func() ExecutionRun {
				run := fixture.run
				run.Acks = nil
				run.Terminal = FlightStateSnapshot{}
				return run
			}(),
			"empty stream with an opening snapshot": func() ExecutionRun {
				run := fixture.run
				run.Acks = nil
				return run
			}(),
		} {
			t.Run(name, func(t *testing.T) {
				encoded, err := EncodeExecutionPartial(fixture.request, run)
				if err != nil {
					t.Fatalf("EncodeExecutionPartial: %v", err)
				}
				decoded, err := DecodeExecutionPartial(bytes.NewReader(encoded))
				if err != nil {
					t.Fatalf("DecodeExecutionPartial: %v", err)
				}
				if decoded.ManifestRevision != fixture.request.ManifestRevision || decoded.ManifestDigest != fixture.request.ManifestDigest {
					t.Fatalf("partial manifest = revision %d digest %q, want the request manifest",
						decoded.ManifestRevision, decoded.ManifestDigest)
				}
				if decoded.EventAcks == nil || len(decoded.EventAcks) != len(run.Acks) {
					t.Fatalf("partial carries %#v, want the exact %d acknowledgments", decoded.EventAcks, len(run.Acks))
				}
			})
		}
	})

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
			QuarantineRepositoryVerificationFailed, QuarantineChildOutputMismatch,
			QuarantineHandbackDurabilityFailed,
		}
		if got, want := len(reasons), 14; got != want {
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

	t.Run("I-83 operational prefixes and attempt-aware factual alternatives", func(t *testing.T) {
		observedInput := RepoObservation{State: RepositoryObserved, Commit: testSHA1, Tree: testTree1, Clean: true}
		observedChild := RepoObservation{State: RepositoryObserved, Commit: testSHA2, Tree: testTree2, Clean: true}
		proven := Proof{State: ProofProven, Witnesses: []string{}}
		unproven := RepoObservation{State: RepositoryUnproven}

		prefixes := []QuarantineObservations{
			validQuarantineRecord(t, QuarantineRepositoryVerificationFailed).Observed,
			{Runway: observedInput, Child: unproven, Descendant: Proof{State: ProofUnproven, Witnesses: []string{"not observed"}}, ProtectedPaths: []string{}, FastForward: FastForwardNotAttempted, PostRunway: unproven},
			{Runway: observedInput, Child: observedChild, Descendant: Proof{State: ProofUnproven, Witnesses: []string{"not observed"}}, ProtectedPaths: []string{}, FastForward: FastForwardNotAttempted, PostRunway: unproven},
			{Runway: observedInput, Child: observedChild, Descendant: proven, ProtectedPaths: []string{}, FastForward: FastForwardNotAttempted, PostRunway: unproven},
			{Runway: observedInput, Child: observedChild, Descendant: proven, ProtectedPaths: []string{}, FastForward: FastForwardFailed, PostRunway: unproven},
			{Runway: observedInput, Child: observedChild, Descendant: proven, ProtectedPaths: []string{}, FastForward: FastForwardSucceeded, PostRunway: unproven},
		}
		for i, observed := range prefixes {
			record := validQuarantineRecord(t, QuarantineRepositoryVerificationFailed)
			record.Observed = observed
			if _, err := EncodeQuarantineRecord(record); err != nil {
				t.Fatalf("repository-verification prefix %d: %v", i, err)
			}
		}

		missingRunwayPrefix := validQuarantineRecord(t, QuarantineRepositoryVerificationFailed)
		missingRunwayPrefix.Observed.Child = observedChild
		if _, err := EncodeQuarantineRecord(missingRunwayPrefix); err == nil {
			t.Fatal("repository-verification accepted a child observation without the earlier runway prefix")
		}

		for _, attempt := range []FastForwardState{FastForwardNotAttempted, FastForwardFailed} {
			for _, test := range []struct {
				reason QuarantineReason
				post   RepoObservation
			}{
				{reason: QuarantineRunwayDirty, post: RepoObservation{State: RepositoryObserved, Commit: testSHA1, Tree: testTree1, Clean: false}},
				{reason: QuarantineRunwayMoved, post: RepoObservation{State: RepositoryObserved, Commit: testSHA2, Tree: testTree1, Clean: true}},
			} {
				record := validQuarantineRecord(t, QuarantineRepositoryVerificationFailed)
				record.Reason = test.reason
				record.Observed = QuarantineObservations{Runway: observedInput, Child: observedChild, Descendant: proven, ProtectedPaths: []string{}, FastForward: attempt, PostRunway: test.post}
				if _, err := EncodeQuarantineRecord(record); err != nil {
					t.Fatalf("late %s with %s: %v", test.reason, attempt, err)
				}
			}
		}

		for name, post := range map[string]RepoObservation{
			"clean input":  observedInput,
			"exact output": observedChild,
		} {
			record := validQuarantineRecord(t, QuarantineFastForwardFailed)
			record.Observed.PostRunway = post
			if _, err := EncodeQuarantineRecord(record); err != nil {
				t.Fatalf("attempted merge failure at %s: %v", name, err)
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
		record.Observed.PostRunway = observedInput
	case QuarantinePostVerificationMismatch:
		record.Observed.Runway, record.Observed.Child, record.Observed.Descendant = observedInput, observedChild, proven
		record.Observed.FastForward = FastForwardSucceeded
		record.Observed.PostRunway = RepoObservation{State: RepositoryObserved, Commit: testSHA2, Tree: testTree1, Clean: true}
	case QuarantineHandbackDurabilityFailed:
		record.Observed.Runway, record.Observed.Child, record.Observed.Descendant = observedInput, observedChild, proven
		record.Observed.FastForward = FastForwardSucceeded
		record.Observed.PostRunway = observedChild
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
	case QuarantineRepositoryVerificationFailed:
		// Durable finalized result with no valid repository observation prefix.
	case QuarantineChildOutputMismatch:
		record.Observed.Runway = observedInput
		record.Observed.Child = RepoObservation{State: RepositoryObserved, Commit: testSHA1, Tree: testTree1, Clean: true}
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

// installedChildTerminal is the exact opening position the shared flight state
// holds after installExpansionLocked has advanced it: the child revision at
// source one with no prior-event digest, the unchanged never-resetting global
// order, and the exact non-null bridge to the acknowledged parent terminal.
func installedChildTerminal(request ExecutionRequest, childDigest string, last contextevent.EventAck) FlightStateSnapshot {
	return FlightStateSnapshot{
		Request: request, Key: executionKey(request), Revision: last.ManifestRevision + 1,
		ManifestDigest: childDigest, ProjectionDigest: request.ProjectionDigest,
		NextSourceSequence: 1, PriorEventDigest: "",
		LastGlobalSequence: last.GlobalSequence,
		PriorRevision: &contextevent.PriorRevision{
			ManifestRevision: last.ManifestRevision, ManifestDigest: request.ManifestDigest,
			EventRoot: last.EventDigest, TerminalSourceSequence: last.SourceSequence,
			TerminalGlobalSequence: last.GlobalSequence,
		},
	}
}
