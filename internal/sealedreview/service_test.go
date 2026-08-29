package sealedreview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/sealedexec"
)

func TestSealedReviewFreshnessContract_Behavioral(t *testing.T) {
	if got, want := RequestSchemaID, "verdi.sealed-review-request/v1"; got != want {
		t.Fatalf("RequestSchemaID = %q, want %q", got, want)
	}
	if got, want := ResultSchemaID, "verdi.sealed-review-result/v1"; got != want {
		t.Fatalf("ResultSchemaID = %q, want %q", got, want)
	}
	if got, want := sealedexec.ReviewLaunchFactsSchemaID, "verdi.sealed-review-launch-facts/v1"; got != want {
		t.Fatalf("ReviewLaunchFactsSchemaID = %q, want %q", got, want)
	}

	repository, candidate, _, _ := reviewRepositoryFixture(t)
	builderReceipt, builderReceiptBytes := reviewBuilderReceiptFixture(t, candidate)
	builderEvidence := evidenceResultBytes(t, builderReceipt.Evidence[0], []byte("builder evidence\n"))
	currentEvidence := evidenceResultBytes(t, builderReceipt.Evidence[0], []byte("builder evidence\n"))
	compilerPort := &packetCompilerFake{rebuiltEvidence: [][]byte{currentEvidence}, manifest: reviewManifestFixture(t)}
	compiler, err := NewPacketCompiler(PacketCompilerPorts{Repository: repository, Compiler: compilerPort})
	if err != nil {
		t.Fatal(err)
	}
	reviewer := Reviewer{
		Lane: "review", Adapter: contextevent.AdapterCodex, AdapterVersion: "1.2.3", Model: "gpt-5.6",
		ProfileID: "reviewer-profile", ProfileDigest: testDigestA,
	}
	packetRequest := PacketRequest{
		Round: RoundR0, Candidate: candidate, Reviewer: reviewer,
		AcceptedSpecPath: "spec.md", ReviewPolicyPath: "policy.md",
		BuilderReceiptBytes: builderReceiptBytes, BuilderEvidenceResultBytes: [][]byte{builderEvidence},
	}
	r0Packet, err := compiler.Compile(context.Background(), packetRequest)
	if err != nil {
		t.Fatalf("Compile(R0): %v", err)
	}
	r0Execution := reviewExecutionRequest(t, r0Packet, "session-review-r0", "epoch-review-r0")

	executor := &reviewExecutorFake{providerSessions: []string{"provider-review-r0", "provider-review-r2"}}
	completion := &reviewCompletionFake{}
	builderRuntime := BuilderRuntime{
		ReceiptDigest: builderReceipt.Digest, VerdiSession: "session-builder", ProviderSession: "provider-builder",
		WorkspaceID: builderReceipt.ExecutionWorkspaceID,
	}
	runtimeResolver := &builderRuntimeResolverFake{runtime: builderRuntime}
	packetVerifier := &packetEvidenceVerifierFake{}
	service, err := NewService(ServicePorts{Executor: executor, Completion: completion, BuilderRuntime: runtimeResolver, PacketEvidence: packetVerifier})
	if err != nil {
		t.Fatalf("NewService(valid): %v", err)
	}
	if _, err := NewService(ServicePorts{}); err == nil {
		t.Fatal("NewService(zero ports) error = nil")
	}

	r0Request := Request{
		Schema: RequestSchemaID, Packet: r0Packet.Packet, ExecutionRequest: r0Execution,
		BuilderReceipt: builderReceipt, PriorReview: nil,
	}
	r0RequestBytes, err := EncodeRequest(r0Request)
	if err != nil {
		t.Fatalf("EncodeRequest(R0): %v", err)
	}
	if !bytes.Contains(r0RequestBytes, []byte(`"prior_review":null`)) {
		t.Fatalf("R0 request does not carry an explicit null prior_review: %s", r0RequestBytes)
	}
	decodedR0, err := DecodeRequest(bytes.NewReader(r0RequestBytes))
	if err != nil {
		t.Fatalf("DecodeRequest(R0): %v", err)
	}
	if !reflect.DeepEqual(decodedR0, r0Request) {
		t.Fatalf("request round trip changed value\ngot:  %#v\nwant: %#v", decodedR0, r0Request)
	}

	r0, err := service.Review(context.Background(), r0Request)
	if err != nil {
		t.Fatalf("Review(R0): %v", err)
	}
	assertReviewResult(t, r0, r0Request)
	if got := completion.calls[0].ReceiptRole; got != contextreceipt.RoleReviewer {
		t.Fatalf("R0 completion role = %q, want reviewer", got)
	}
	if got, want := completion.calls[0].ReviewOf, []string{builderReceipt.Digest}; !reflect.DeepEqual(got, want) {
		t.Fatalf("R0 review_of = %#v, want %#v", got, want)
	}
	if got, want := completion.calls[0].ReviewInputs, packetProjection(r0Packet.Packet); !reflect.DeepEqual(got, want) {
		t.Fatalf("R0 review_inputs = %#v, want exact packet projection %#v", got, want)
	}
	assertLaunchInput(t, executor.calls[0], r0Packet, r0Execution, nil)
	assertLaunchFactsLiteral(t, executor.runs[0], r0Packet, r0Execution, nil)

	r0ReceiptBytes, err := contextreceipt.EncodeReceipt(r0.ReviewReceipt)
	if err != nil {
		t.Fatal(err)
	}
	adjudicationBytes := reviewAdjudicationFixture(t, candidate, r0.ReviewReceipt.Digest)
	packetRequest.Round = RoundR2
	packetRequest.PriorReviewReceiptBytes = r0ReceiptBytes
	packetRequest.AdjudicationBytes = adjudicationBytes
	r2Packet, err := compiler.Compile(context.Background(), packetRequest)
	if err != nil {
		t.Fatalf("Compile(R2): %v", err)
	}
	r2Execution := reviewExecutionRequest(t, r2Packet, "session-review-r2", "epoch-review-r2")
	prior := &PriorReview{ReceiptDigest: r0.ReviewReceipt.Digest, AdjudicationDigest: r2Packet.Packet.Items[5].ContentDigest}
	r2Request := Request{
		Schema: RequestSchemaID, Packet: r2Packet.Packet, ExecutionRequest: r2Execution,
		BuilderReceipt: builderReceipt, PriorReview: prior,
	}
	r2, err := service.Review(context.Background(), r2Request)
	if err != nil {
		t.Fatalf("Review(R2): %v", err)
	}
	assertReviewResult(t, r2, r2Request)
	assertLaunchInput(t, executor.calls[1], r2Packet, r2Execution, prior)
	assertLaunchFactsLiteral(t, executor.runs[1], r2Packet, r2Execution, prior)
	if got, want := completion.calls[1].ReviewInputs, packetProjection(r2Packet.Packet); !reflect.DeepEqual(got, want) {
		t.Fatalf("R2 review_inputs = %#v, want exact packet projection %#v", got, want)
	}
	if got, want := completion.calls[1].ReviewOf, []string{builderReceipt.Digest}; !reflect.DeepEqual(got, want) {
		t.Fatalf("R2 review_of = %#v, want %#v", got, want)
	}
	if got := packetVerifier.callCount(); got != 2 {
		t.Fatalf("packet evidence verification calls = %d, want exact R0/R2 prelaunch checks", got)
	}

	if got, want := []any{
		r0.Round == r2.Round,
		r0.PacketDigest == r2.PacketDigest,
		r0.ExecutionResult.Session == r2.ExecutionResult.Session,
		r0.ExecutionResult.ExecutionWorkspaceID == r2.ExecutionResult.ExecutionWorkspaceID,
		r0.ExecutionResult.TerminalManifestDigest == r2.ExecutionResult.TerminalManifestDigest,
		r0.ReviewReceipt.DispatchDigest == r2.ReviewReceipt.DispatchDigest,
		r0.ReviewReceipt.EventChainRoot == r2.ReviewReceipt.EventChainRoot,
		r0.ReviewReceipt.Digest == r2.ReviewReceipt.Digest,
		executor.runs[0].AdapterSessionRef == executor.runs[1].AdapterSessionRef,
	}, []any{false, false, false, false, false, false, false, false, false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("R0/R2 reused a required-distinct identity: got %#v", got)
	}
	if builderReceipt.ExecutionWorkspaceID == r0.ExecutionResult.ExecutionWorkspaceID ||
		builderReceipt.ManifestDigest == r0.ReviewReceipt.ManifestDigest ||
		builderReceipt.DispatchDigest == r0.ReviewReceipt.DispatchDigest ||
		builderReceipt.EventChainRoot == r0.ReviewReceipt.EventChainRoot ||
		builderReceipt.Digest == r0.ReviewReceipt.Digest {
		t.Fatal("R0 reused a builder execution identity")
	}
	if !reflect.DeepEqual(r0Packet.Packet.Reviewer, r2Packet.Packet.Reviewer) {
		t.Fatal("configured reviewer identity drifted between R0 and R2")
	}
	for i, call := range executor.calls {
		if got, want := call.ModelArgv, []string{"--model", "gpt-5.6"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("review launch[%d] explicit model argv = %#v, want %#v", i, got, want)
		}
		if len(call.Data) != 1 || call.Data[0].Content != string(call.PacketBytes) {
			t.Fatalf("review launch[%d] inherited data outside its one canonical packet", i)
		}
		if strings.Contains(call.Data[0].Content, `"kind":"builder-conversation"`) || strings.Contains(call.Data[0].Content, `"kind":"prior-reviewer-conversation"`) {
			t.Fatalf("review launch[%d] inherited a forbidden conversation", i)
		}
	}

	r2ResultBytes, err := EncodeResult(r2)
	if err != nil {
		t.Fatalf("EncodeResult(R2): %v", err)
	}
	decodedResult, err := DecodeResult(bytes.NewReader(r2ResultBytes))
	if err != nil {
		t.Fatalf("DecodeResult(R2): %v", err)
	}
	if !reflect.DeepEqual(decodedResult, r2) {
		t.Fatal("result strict round trip changed R2")
	}

	t.Run("strict request and result codecs reject structural mutations", func(t *testing.T) {
		for name, raw := range map[string][]byte{
			"request unknown":   bytes.Replace(r0RequestBytes, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1),
			"request duplicate": bytes.Replace(r0RequestBytes, []byte(`"schema":`), []byte(`"schema":"verdi.sealed-review-request/v1","schema":`), 1),
			"request trailing":  append(append([]byte(nil), r0RequestBytes...), []byte("{}")...),
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := DecodeRequest(bytes.NewReader(raw)); err == nil {
					t.Fatal("DecodeRequest(mutated) error = nil")
				}
			})
		}
		for name, raw := range map[string][]byte{
			"result unknown":   bytes.Replace(r2ResultBytes, []byte(`"schema":`), []byte(`"extra":true,"schema":`), 1),
			"result duplicate": bytes.Replace(r2ResultBytes, []byte(`"schema":`), []byte(`"schema":"verdi.sealed-review-result/v1","schema":`), 1),
			"result trailing":  append(append([]byte(nil), r2ResultBytes...), []byte("{}")...),
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := DecodeResult(bytes.NewReader(raw)); err == nil {
					t.Fatal("DecodeResult(mutated) error = nil")
				}
			})
		}
		wrongReceipt := r2
		wrongReceipt.ReviewReceipt = r0.ReviewReceipt
		if _, err := EncodeResult(wrongReceipt); err == nil {
			t.Fatal("EncodeResult(mismatched reviewer receipt) error = nil")
		}
		wrongAck := r2
		wrongAck.ReceiptEventAck = r0.ReceiptEventAck
		if _, err := EncodeResult(wrongAck); err == nil {
			t.Fatal("EncodeResult(mismatched receipt ack) error = nil")
		}
	})

	t.Run("prelaunch lineage identity and start-only refusals", func(t *testing.T) {
		cases := []struct {
			name   string
			mutate func(*Request)
		}{
			{"resume", func(r *Request) { r.ExecutionRequest.Action = sealedexec.ActionResume; r.ExecutionRequest.Start = nil }},
			{"R0 prior", func(r *Request) {
				r.PriorReview = &PriorReview{ReceiptDigest: testDigestA, AdjudicationDigest: testDigestB}
			}},
			{"lane drift", func(r *Request) { r.ExecutionRequest.Lane = "builder" }},
			{"profile drift", func(r *Request) { r.ExecutionRequest.Profile.ID = "ambient-profile" }},
			{"adapter version drift", func(r *Request) { r.ExecutionRequest.AdapterVersion = "9.9.9" }},
			{"candidate drift", func(r *Request) { r.ExecutionRequest.InputTree = candidate.BaseTree }},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				bad := cloneReviewRequest(t, r0Request)
				tc.mutate(&bad)
				before := len(executor.calls)
				if _, err := service.Review(context.Background(), bad); err == nil {
					t.Fatal("Review(mutated prelaunch request) error = nil")
				}
				if len(executor.calls) != before {
					t.Fatal("prelaunch refusal reached provider")
				}
			})
		}

		for _, tc := range []struct {
			name   string
			mutate func(*Request)
		}{
			{"missing prior", func(r *Request) { r.PriorReview = nil }},
			{"wrong R0 receipt", func(r *Request) { r.PriorReview.ReceiptDigest = testDigestB }},
			{"wrong adjudication", func(r *Request) { r.PriorReview.AdjudicationDigest = testDigestA }},
			{"session reuse", func(r *Request) {
				r.ExecutionRequest.Session = r0Execution.Session
				r.ExecutionRequest.ExecutionWorkspaceRequest = r0Execution.ExecutionWorkspaceRequest
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				bad := cloneReviewRequest(t, r2Request)
				tc.mutate(&bad)
				before := len(executor.calls)
				if _, err := service.Review(context.Background(), bad); err == nil {
					t.Fatal("Review(mutated R2 lineage) error = nil")
				}
				if len(executor.calls) != before {
					t.Fatal("R2 lineage refusal reached provider")
				}
			})
		}
		fresh, err := NewService(ServicePorts{Executor: executor, Completion: completion, BuilderRuntime: runtimeResolver, PacketEvidence: packetVerifier})
		if err != nil {
			t.Fatal(err)
		}
		before := len(executor.calls)
		if _, err := fresh.Review(context.Background(), r2Request); err == nil {
			t.Fatal("Review(R2 without actual R0 service lineage) error = nil")
		}
		if len(executor.calls) != before {
			t.Fatal("R2 without actual R0 reached provider")
		}
	})

	t.Run("acknowledged launch fact failures never reach reviewer completion", func(t *testing.T) {
		mutations := []struct {
			name   string
			mutate func(*sealedexec.ExecutionRun)
		}{
			{"missing facts", func(run *sealedexec.ExecutionRun) {
				run.ReviewLaunchFacts = nil
				run.ReviewLaunchEvent = nil
				run.ReviewLaunchAck = nil
			}},
			{"facts mismatch", func(run *sealedexec.ExecutionRun) { run.ReviewLaunchFacts.Model = "ambient-model" }},
			{"event envelope mismatch", func(run *sealedexec.ExecutionRun) { run.ReviewLaunchEvent.Lane = "builder" }},
			{"ack mispaired", func(run *sealedexec.ExecutionRun) { run.ReviewLaunchAck.EventDigest = testDigestA }},
		}
		for _, tc := range mutations {
			t.Run(tc.name, func(t *testing.T) {
				localExecutor := &reviewExecutorFake{providerSessions: []string{"provider-negative-" + strings.ReplaceAll(tc.name, " ", "-")}, mutateRun: tc.mutate}
				localCompletion := &reviewCompletionFake{}
				local, err := NewService(ServicePorts{Executor: localExecutor, Completion: localCompletion, BuilderRuntime: runtimeResolver, PacketEvidence: packetVerifier})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := local.Review(context.Background(), r0Request); err == nil {
					t.Fatal("Review(mutated acknowledged launch proof) error = nil")
				}
				if len(localCompletion.calls) != 0 {
					t.Fatal("invalid launch proof reached reviewer receipt completion")
				}
			})
		}
	})

	t.Run("R2 provider-session reuse never reaches reviewer completion", func(t *testing.T) {
		localExecutor := &reviewExecutorFake{providerSessions: []string{"provider-reused", "provider-reused"}}
		localCompletion := &reviewCompletionFake{}
		local, err := NewService(ServicePorts{Executor: localExecutor, Completion: localCompletion, BuilderRuntime: runtimeResolver, PacketEvidence: packetVerifier})
		if err != nil {
			t.Fatal(err)
		}
		localR0, err := local.Review(context.Background(), r0Request)
		if err != nil {
			t.Fatalf("local Review(R0): %v", err)
		}
		badR2 := cloneReviewRequest(t, r2Request)
		badR2.PriorReview.ReceiptDigest = localR0.ReviewReceipt.Digest
		if _, err := local.Review(context.Background(), badR2); err == nil {
			t.Fatal("Review(R2 provider-session reuse) error = nil")
		}
		if len(localCompletion.calls) != 1 {
			t.Fatalf("completion calls = %d, want only R0 completion", len(localCompletion.calls))
		}
	})

	t.Run("builder runtime authority prevents conversation identity reuse", func(t *testing.T) {
		if len(runtimeResolver.receiptDigests) < 2 {
			t.Fatalf("builder runtime resolver calls = %d, want at least R0 and R2", len(runtimeResolver.receiptDigests))
		}
		for _, digest := range runtimeResolver.receiptDigests {
			if digest != builderReceipt.Digest {
				t.Fatalf("builder runtime resolver key = %q, want only exact builder digest %q", digest, builderReceipt.Digest)
			}
		}

		reusedSession := cloneReviewRequest(t, r0Request)
		reusedSession.ExecutionRequest = reviewExecutionRequest(t, r0Packet, builderRuntime.VerdiSession, "epoch-builder-reuse")
		beforeLaunch := len(executor.calls)
		if _, err := service.Review(context.Background(), reusedSession); err == nil {
			t.Fatal("Review(builder Verdi-session reuse) error = nil")
		}
		if len(executor.calls) != beforeLaunch {
			t.Fatal("builder Verdi-session reuse reached provider launch")
		}

		providerExecutor := &reviewExecutorFake{providerSessions: []string{builderRuntime.ProviderSession}}
		providerCompletion := &reviewCompletionFake{}
		providerService, err := NewService(ServicePorts{Executor: providerExecutor, Completion: providerCompletion, BuilderRuntime: &builderRuntimeResolverFake{runtime: builderRuntime}, PacketEvidence: packetVerifier})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := providerService.Review(context.Background(), r0Request); err == nil {
			t.Fatal("Review(builder provider-session reuse) error = nil")
		}
		if len(providerCompletion.calls) != 0 {
			t.Fatal("builder provider-session reuse reached reviewer completion")
		}

		for _, mutation := range []struct {
			name    string
			runtime BuilderRuntime
		}{
			{name: "empty receipt", runtime: BuilderRuntime{VerdiSession: "session-builder", ProviderSession: "provider-builder", WorkspaceID: builderReceipt.ExecutionWorkspaceID}},
			{name: "empty Verdi session", runtime: BuilderRuntime{ReceiptDigest: builderReceipt.Digest, ProviderSession: "provider-builder", WorkspaceID: builderReceipt.ExecutionWorkspaceID}},
			{name: "empty provider session", runtime: BuilderRuntime{ReceiptDigest: builderReceipt.Digest, VerdiSession: "session-builder", WorkspaceID: builderReceipt.ExecutionWorkspaceID}},
			{name: "empty workspace", runtime: BuilderRuntime{ReceiptDigest: builderReceipt.Digest, VerdiSession: "session-builder", ProviderSession: "provider-builder"}},
			{name: "receipt mismatch", runtime: BuilderRuntime{ReceiptDigest: testDigestB, VerdiSession: "session-builder", ProviderSession: "provider-builder", WorkspaceID: builderReceipt.ExecutionWorkspaceID}},
			{name: "workspace mismatch", runtime: BuilderRuntime{ReceiptDigest: builderReceipt.Digest, VerdiSession: "session-builder", ProviderSession: "provider-builder", WorkspaceID: "workspace-other"}},
		} {
			t.Run(mutation.name, func(t *testing.T) {
				localExecutor := &reviewExecutorFake{providerSessions: []string{"provider-unused"}}
				localCompletion := &reviewCompletionFake{}
				local, err := NewService(ServicePorts{
					Executor: localExecutor, Completion: localCompletion,
					BuilderRuntime: &builderRuntimeResolverFake{runtime: mutation.runtime},
					PacketEvidence: packetVerifier,
				})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := local.Review(context.Background(), r0Request); err == nil {
					t.Fatal("Review(invalid builder runtime) error = nil")
				}
				if len(localExecutor.calls) != 0 || len(localCompletion.calls) != 0 {
					t.Fatal("invalid builder runtime crossed a launch or completion boundary")
				}
			})
		}
	})

	t.Run("packet evidence failure refuses before provider launch", func(t *testing.T) {
		localExecutor := &reviewExecutorFake{providerSessions: []string{"provider-unused"}}
		localCompletion := &reviewCompletionFake{}
		localVerifier := &packetEvidenceVerifierFake{err: errors.New("authenticated packet mismatch")}
		local, err := NewService(ServicePorts{
			Executor: localExecutor, Completion: localCompletion, BuilderRuntime: runtimeResolver, PacketEvidence: localVerifier,
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := local.Review(context.Background(), r0Request); err == nil {
			t.Fatal("Review(packet evidence mismatch) error = nil")
		}
		if localVerifier.callCount() != 1 || len(localExecutor.calls) != 0 || len(localCompletion.calls) != 0 {
			t.Fatal("packet evidence mismatch crossed a launch or receipt boundary")
		}
	})

	t.Run("R0 reservation is atomic and owner rollback permits retry", func(t *testing.T) {
		blockingExecutor := newBlockingReviewExecutor()
		blockingCompletion := &reviewCompletionFake{}
		local, err := NewService(ServicePorts{
			Executor: blockingExecutor, Completion: blockingCompletion, BuilderRuntime: runtimeResolver,
			PacketEvidence: &packetEvidenceVerifierFake{},
		})
		if err != nil {
			t.Fatal(err)
		}
		type reviewOutcome struct {
			result Result
			err    error
		}
		firstDone := make(chan reviewOutcome, 1)
		go func() {
			result, reviewErr := local.Review(context.Background(), r0Request)
			firstDone <- reviewOutcome{result: result, err: reviewErr}
		}()
		<-blockingExecutor.started
		duplicateResult, duplicateErr := local.Review(context.Background(), r0Request)
		close(blockingExecutor.release)
		first := <-firstDone
		if first.err != nil {
			t.Fatalf("reserved owner Review(R0) error = %v", first.err)
		}
		if duplicateErr == nil || !reflect.DeepEqual(duplicateResult, Result{}) {
			t.Fatalf("concurrent duplicate result/error = %#v/%v, want refusal", duplicateResult, duplicateErr)
		}
		if got := blockingExecutor.callCount(); got != 1 {
			t.Fatalf("concurrent duplicate provider launches = %d, want 1", got)
		}
		if got := len(blockingCompletion.calls); got != 1 {
			t.Fatalf("concurrent duplicate receipt mints = %d, want 1", got)
		}
		if result, err := local.Review(context.Background(), r0Request); err == nil || !reflect.DeepEqual(result, Result{}) {
			t.Fatalf("completed duplicate result/error = %#v/%v, want prelaunch refusal", result, err)
		}
		if got := blockingExecutor.callCount(); got != 1 {
			t.Fatalf("completed duplicate provider launches = %d, want 1", got)
		}
		if got := len(blockingCompletion.calls); got != 1 {
			t.Fatalf("completed duplicate receipt mints = %d, want 1", got)
		}

		retryExecutor := &retryReviewExecutor{}
		retryCompletion := &reviewCompletionFake{}
		retryService, err := NewService(ServicePorts{
			Executor: retryExecutor, Completion: retryCompletion, BuilderRuntime: runtimeResolver,
			PacketEvidence: &packetEvidenceVerifierFake{},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result, err := retryService.Review(context.Background(), r0Request); err == nil || !reflect.DeepEqual(result, Result{}) {
			t.Fatalf("first failed owner result/error = %#v/%v", result, err)
		}
		if result, err := retryService.Review(context.Background(), r0Request); err != nil || result.ReviewReceipt.Digest == "" {
			t.Fatalf("retry after owner rollback result/error = %#v/%v", result, err)
		}
		if retryExecutor.callCount() != 2 || len(retryCompletion.calls) != 1 {
			t.Fatalf("retry launch/completion counts = %d/%d, want 2/1", retryExecutor.callCount(), len(retryCompletion.calls))
		}
	})

	t.Run("execution or completion failures return no review result", func(t *testing.T) {
		failedExecutor := &reviewExecutorFake{err: errors.New("provider failed")}
		failedCompletion := &reviewCompletionFake{}
		failed, _ := NewService(ServicePorts{Executor: failedExecutor, Completion: failedCompletion, BuilderRuntime: runtimeResolver, PacketEvidence: packetVerifier})
		if result, err := failed.Review(context.Background(), r0Request); err == nil || !reflect.DeepEqual(result, Result{}) {
			t.Fatalf("execution failure result/error = %#v/%v", result, err)
		}
		if len(failedCompletion.calls) != 0 {
			t.Fatal("execution failure reached receipt completion")
		}

		okExecutor := &reviewExecutorFake{providerSessions: []string{"provider-completion-failure"}}
		brokenCompletion := &reviewCompletionFake{err: errors.New("receipt ack failed")}
		broken, _ := NewService(ServicePorts{Executor: okExecutor, Completion: brokenCompletion, BuilderRuntime: runtimeResolver, PacketEvidence: packetVerifier})
		if result, err := broken.Review(context.Background(), r0Request); err == nil || !reflect.DeepEqual(result, Result{}) {
			t.Fatalf("completion failure result/error = %#v/%v", result, err)
		}
	})
}

type blockingReviewExecutor struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
}

func newBlockingReviewExecutor() *blockingReviewExecutor {
	return &blockingReviewExecutor{started: make(chan struct{}), release: make(chan struct{})}
}

func (f *blockingReviewExecutor) ExecuteReview(ctx context.Context, request sealedexec.ExecutionRequest, _ []contextcompile.DataItem, launch sealedexec.ReviewLaunch) (sealedexec.ExecutionRun, error) {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()
	if call == 1 {
		close(f.started)
		select {
		case <-f.release:
		case <-ctx.Done():
			return sealedexec.ExecutionRun{}, ctx.Err()
		}
	}
	return reviewExecutionRun(tContext{}, request, launch, fmt.Sprintf("provider-blocking-%d", call)), nil
}

func (f *blockingReviewExecutor) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type retryReviewExecutor struct {
	mu    sync.Mutex
	calls int
}

func (f *retryReviewExecutor) ExecuteReview(_ context.Context, request sealedexec.ExecutionRequest, _ []contextcompile.DataItem, launch sealedexec.ReviewLaunch) (sealedexec.ExecutionRun, error) {
	f.mu.Lock()
	f.calls++
	call := f.calls
	f.mu.Unlock()
	if call == 1 {
		return sealedexec.ExecutionRun{}, errors.New("provider failed once")
	}
	return reviewExecutionRun(tContext{}, request, launch, "provider-retry"), nil
}

func (f *retryReviewExecutor) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type packetEvidenceVerifierFake struct {
	mu      sync.Mutex
	err     error
	packets []Packet
}

func (f *packetEvidenceVerifierFake) VerifyPacketEvidence(_ context.Context, packet Packet) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.packets = append(f.packets, clonePacket(packet))
	return f.err
}

func (f *packetEvidenceVerifierFake) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.packets)
}

type builderRuntimeResolverFake struct {
	mu             sync.Mutex
	runtime        BuilderRuntime
	err            error
	receiptDigests []string
}

func (f *builderRuntimeResolverFake) ResolveBuilderRuntime(_ context.Context, receiptDigest string) (BuilderRuntime, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.receiptDigests = append(f.receiptDigests, receiptDigest)
	return f.runtime, f.err
}

type reviewExecutionCall struct {
	Request     sealedexec.ExecutionRequest
	Data        []contextcompile.DataItem
	Launch      sealedexec.ReviewLaunch
	PacketBytes []byte
	ModelArgv   []string
}

type reviewExecutorFake struct {
	calls            []reviewExecutionCall
	runs             []sealedexec.ExecutionRun
	providerSessions []string
	mutateRun        func(*sealedexec.ExecutionRun)
	err              error
}

func (f *reviewExecutorFake) ExecuteReview(_ context.Context, request sealedexec.ExecutionRequest, data []contextcompile.DataItem, launch sealedexec.ReviewLaunch) (sealedexec.ExecutionRun, error) {
	if f.err != nil {
		return sealedexec.ExecutionRun{}, f.err
	}
	packetBytes := []byte(nil)
	if len(data) == 1 {
		packetBytes = []byte(data[0].Content)
	}
	f.calls = append(f.calls, reviewExecutionCall{
		Request: request, Data: append([]contextcompile.DataItem(nil), data...), Launch: launch,
		PacketBytes: packetBytes, ModelArgv: []string{"--model", launch.Model},
	})
	providerSession := fmt.Sprintf("provider-review-%d", len(f.calls))
	if len(f.providerSessions) >= len(f.calls) {
		providerSession = f.providerSessions[len(f.calls)-1]
	}
	run := reviewExecutionRun(tContext{}, request, launch, providerSession)
	if f.mutateRun != nil {
		f.mutateRun(&run)
	}
	f.runs = append(f.runs, run)
	return run, nil
}

// tContext lets the fake's method build fixtures without storing *testing.T.
// Fixture construction panics only on an impossible bug in test-owned values.
type tContext struct{}

func reviewExecutionRun(_ tContext, request sealedexec.ExecutionRequest, launch sealedexec.ReviewLaunch, providerSession string) sealedexec.ExecutionRun {
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		panic(err)
	}
	workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		panic(err)
	}
	facts := sealedexec.ReviewLaunchFacts{
		Schema: sealedexec.ReviewLaunchFactsSchemaID, Round: launch.Round, PacketDigest: launch.PacketDigest,
		PriorReview: launch.PriorReview, Lane: request.Lane, Adapter: request.Adapter,
		AdapterVersion: request.AdapterVersion, Model: launch.Model, ProfileID: request.Profile.ID,
		ProfileDigest: request.Profile.Digest, Session: request.Session, WorkspaceID: workspaceID,
	}
	factsBytes, err := sealedexec.EncodeReviewLaunchFacts(facts)
	if err != nil {
		panic(err)
	}
	payloadSchema, err := contextevent.PayloadSchema(contextevent.KindAdapterStart)
	if err != nil {
		panic(err)
	}
	event := contextevent.Event{
		Schema: contextevent.EventSchemaID, SourceSequence: 1, Flight: request.Flight, Lane: request.Lane,
		Epoch: request.Epoch, ManifestRevision: request.ManifestRevision, ManifestDigest: request.ManifestDigest,
		Session: request.Session, ATCRunway: request.ATCRunway, ExecutionWorkspaceID: workspaceID,
		CandidateCommit: request.InputCommit, CandidateTree: request.InputTree, Adapter: request.Adapter,
		AdapterVersion: request.AdapterVersion, OccurredAt: "2026-08-29T12:00:00Z", Kind: contextevent.KindAdapterStart,
		PayloadSchema: payloadSchema, Payload: &contextevent.AdapterStartPayload{
			Schema: payloadSchema, Adapter: request.Adapter, AdapterVersion: request.AdapterVersion,
			Session: request.Session, ProfileDigest: request.Profile.Digest, WorkspaceRequestDigest: workspaceDigest,
			Detail: &contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: rawDigestForTest(factsBytes), RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: factsBytes},
		}, PriorEventDigest: "",
	}
	eventBytes, err := contextevent.EncodeEvent(event)
	if err != nil {
		panic(err)
	}
	event, err = contextevent.DecodeEvent(bytes.NewReader(eventBytes))
	if err != nil {
		panic(err)
	}
	ack := contextevent.EventAck{
		Schema: contextevent.AckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch,
		Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind,
		SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: 1,
	}
	workspace := sealedexec.WorkspaceFacts{
		Verification: sealedexec.Verification{State: contextcompile.ResolutionProven}, WorkspaceID: workspaceID,
		Path: "/data/execution/" + workspaceID, Request: request.ExecutionWorkspaceRequest,
		RequestDigest: workspaceDigest, CurrentCommit: request.InputCommit, CurrentTree: request.InputTree, Clean: true,
	}
	return sealedexec.ExecutionRun{
		Authority: contextevent.AuthorityAuthoritative, Witnesses: []string{}, Workspace: workspace,
		Profile:           sealedexec.ResolvedProfile{Verification: sealedexec.Verification{State: contextcompile.ResolutionProven}, Ref: request.Profile, Digest: request.Profile.Digest, Name: request.Profile.ID, AdapterVersion: request.AdapterVersion, WorkspacePath: workspace.Path},
		AdapterSessionRef: providerSession, ReviewLaunchFacts: &facts, ReviewLaunchEvent: &event, ReviewLaunchAck: &ack,
		Acks: []contextevent.EventAck{ack},
	}
}

type reviewCompletionFake struct {
	calls []sealedexec.CompletionRequest
	err   error
}

func (f *reviewCompletionFake) Complete(_ context.Context, request sealedexec.CompletionRequest) (sealedexec.Completion, error) {
	f.calls = append(f.calls, request)
	if f.err != nil {
		return sealedexec.Completion{}, f.err
	}
	return reviewCompletionFixture(request), nil
}

func reviewCompletionFixture(input sealedexec.CompletionRequest) sealedexec.Completion {
	request := input.Request
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		panic(err)
	}
	workspaceDigest, err := sealedexec.ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		panic(err)
	}
	requestBytes, err := sealedexec.EncodeExecutionRequest(request)
	if err != nil {
		panic(err)
	}
	eventRoot := rawDigestForTest([]byte("event-root:" + request.Session))
	revision := contextevent.Revision{
		Schema: contextevent.RevisionSchemaID, ManifestRevision: request.ManifestRevision,
		ManifestDigest: request.ManifestDigest, FirstGlobalSequence: 1, TerminalGlobalSequence: 2,
		TerminalSourceSequence: 2, TerminalKind: contextevent.KindExecutionResult, EventRoot: eventRoot,
	}
	chainRoot, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		panic(err)
	}
	receipt := reviewReceiptBaseForExecution(request, workspaceID, workspaceDigest, rawDigestForTest(requestBytes), revision, chainRoot)
	receipt.Role = input.ReceiptRole
	receipt.ReviewInputs = append([]contextreceipt.ReviewInput(nil), input.ReviewInputs...)
	receipt.ReviewOf = append([]string(nil), input.ReviewOf...)
	receiptBytes, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		panic(err)
	}
	receipt, err = contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		panic(err)
	}
	ack := contextevent.ReceiptEventAck{
		Schema: contextevent.ReceiptAckSchemaID, Flight: request.Flight, Lane: request.Lane, Epoch: request.Epoch,
		Session: request.Session, ManifestRevision: request.ManifestRevision, Kind: contextevent.KindReceipt,
		SourceSequence: 3, EventDigest: rawDigestForTest([]byte("receipt-event:" + request.Session)),
		GlobalSequence: 3, ReceiptDigest: receipt.Digest,
	}
	result := sealedexec.ExecutionResult{
		Schema: sealedexec.ExecutionResultSchemaID, Verdict: contextcompile.ResolutionProven,
		Authority: contextevent.AuthorityAuthoritative, Witnesses: []string{}, Flight: request.Flight,
		Lane: request.Lane, Epoch: request.Epoch, Session: request.Session, ATCRunway: request.ATCRunway,
		ExecutionWorkspaceID: workspaceID, Adapter: request.Adapter, AdapterVersion: request.AdapterVersion,
		InputCommit: request.InputCommit, InputTree: request.InputTree, OutputCommit: request.InputCommit,
		OutputTree: request.InputTree, Clean: true, TerminalManifestDigest: request.ManifestDigest,
		TerminalManifestRevision: request.ManifestRevision, TerminalSourceSequence: 2, TerminalGlobalSequence: 2,
		EventChainRoot: chainRoot, Receipt: receipt, ReceiptEventAck: ack,
	}
	resultBytes, err := sealedexec.EncodeExecutionResult(result)
	if err != nil {
		panic(err)
	}
	result, err = sealedexec.DecodeExecutionResult(bytes.NewReader(resultBytes))
	if err != nil {
		panic(err)
	}
	return sealedexec.Completion{
		Result: result, ResultBytes: resultBytes, Receipt: receipt, ReceiptEventAck: ack,
		Revisions: []contextevent.Revision{revision}, EventChainRoot: chainRoot,
		Verdict: contextcompile.ResolutionProven, Authority: contextevent.AuthorityAuthoritative,
		Output: input.Run.Workspace,
	}
}

func reviewReceiptBaseForExecution(request sealedexec.ExecutionRequest, workspaceID, workspaceDigest, dispatchDigest string, revision contextevent.Revision, chainRoot string) contextreceipt.Receipt {
	claim := gp.PrincipalClaim{TrustSource: "ci-runner", Subject: "reviewer@example.com"}
	principalID, err := gp.CanonicalPrincipalID(claim.TrustSource, claim.Subject)
	if err != nil {
		panic(err)
	}
	return contextreceipt.Receipt{
		Schema: contextreceipt.SchemaID, Role: contextreceipt.RoleReviewer, Authority: contextreceipt.AuthorityAuthoritative,
		ManifestDigest: request.ManifestDigest, DispatchDigest: dispatchDigest, ATCRunway: request.ATCRunway,
		ExecutionWorkspaceRequestDigest: workspaceDigest, ExecutionWorkspaceID: workspaceID,
		InputCommit: request.InputCommit, InputTree: request.InputTree, OutputCommit: request.InputCommit,
		OutputTree: request.InputTree, Clean: true, RevisionSegments: []contextevent.Revision{revision}, EventChainRoot: chainRoot,
		TerminalManifestRevision: request.ManifestRevision, TerminalSourceSequence: 2, TerminalGlobalSequence: 2,
		Expansions: []contextreceipt.Expansion{}, Obligations: []contextreceipt.Obligation{}, Evidence: []contextreceipt.Evidence{},
		RunnerPrincipalResolution: gp.PrincipalResolution{
			Claim: claim, PrincipalID: principalID, State: gp.ResolutionAuthenticated,
			Witnesses: []gp.Witness{{Code: "trust-subject-verified", SourceID: claim.TrustSource, EvidenceDigest: testDigestA}},
		},
		Adapter: request.Adapter, AdapterVersion: request.AdapterVersion, ReviewInputs: []contextreceipt.ReviewInput{},
	}
}

func reviewExecutionRequest(t *testing.T, packet PacketResult, session, epoch string) sealedexec.ExecutionRequest {
	t.Helper()
	manifest, err := contextcompile.DecodeManifest(packet.Compilation.ManifestBytes)
	if err != nil {
		t.Fatal(err)
	}
	projection := sealedexec.InstructionProjection{
		Schema: sealedexec.InstructionProjectionSchemaID,
		Files: []sealedexec.InstructionFile{{
			Path: manifest.ProjectionFiles[0].Path, ContentDigest: rawDigestForTest(packet.Compilation.InstructionProjectionBytes),
			Content: string(packet.Compilation.InstructionProjectionBytes),
		}},
	}
	projectionBytes, err := sealedexec.EncodeInstructionProjection(projection)
	if err != nil {
		t.Fatal(err)
	}
	projection, err = sealedexec.DecodeInstructionProjection(bytes.NewReader(projectionBytes))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := sealedexec.NewExecutionWorkspaceRequest("flight-review", packet.Packet.Reviewer.Lane, epoch, session, packet.Packet.Candidate.HeadCommit)
	if err != nil {
		t.Fatal(err)
	}
	request := sealedexec.ExecutionRequest{
		Schema: sealedexec.ExecutionRequestSchemaID, Action: sealedexec.ActionStart, Flight: "flight-review",
		Lane: packet.Packet.Reviewer.Lane, Epoch: epoch, ManifestRevision: 0, Session: session,
		ATCRunway: "/runway/review", InputCommit: packet.Packet.Candidate.HeadCommit, InputTree: packet.Packet.Candidate.HeadTree,
		Manifest: manifest, ManifestDigest: manifest.Digest, InstructionProjection: projection, ProjectionDigest: projection.Digest,
		ExecutionWorkspaceRequest: workspace, Adapter: packet.Packet.Reviewer.Adapter,
		AdapterVersion: packet.Packet.Reviewer.AdapterVersion,
		Profile:        sealedexec.LogicalRef{Schema: sealedexec.ProjectProfileRefSchemaID, ID: packet.Packet.Reviewer.ProfileID, Digest: packet.Packet.Reviewer.ProfileDigest},
		Grants:         manifest.Capabilities, AuthorityVerdict: reviewAuthorityReport(t, manifest),
		RecorderEndpoint: sealedexec.LogicalRef{Schema: sealedexec.RecorderEndpointRefSchemaID, ID: "vatc-recorder", Digest: testDigestB},
		Start:            &sealedexec.StartArm{ExpectedSourceSequence: 1},
	}
	encoded, err := sealedexec.EncodeExecutionRequest(request)
	if err != nil {
		t.Fatalf("EncodeExecutionRequest(review): %v", err)
	}
	request, err = sealedexec.DecodeExecutionRequest(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	return request
}

func reviewAuthorityReport(t *testing.T, manifest contextcompile.Manifest) policyconflict.Report {
	t.Helper()
	report := policyconflict.Report{
		Schema: policyconflict.ReportSchema,
		Input: policyconflict.InputIdentity{
			Target:             policyconflict.TargetIdentity{Kind: policyconflict.TargetAcceptedContext, Accepted: &policyconflict.AcceptedIdentity{ManifestDigest: manifest.Digest}},
			ConstitutionDigest: manifest.Policy.ConstitutionDigest, EffectivePolicyDigest: manifest.Policy.EffectiveDigest,
			PolicyEntries: []policyconflict.PolicyEntryIdentity{},
			Profile:       policyconflict.ProfileIdentity{ID: manifest.GovernanceProfile.ID, Class: string(manifest.GovernanceProfile.Class), Digest: manifest.GovernanceProfile.Digest},
			EvaluatedOn:   "2026-08-29",
		},
		Mechanical: []policyconflict.MechanicalEvaluation{}, Semantic: []policyconflict.SemanticEvaluation{},
		Disclosures: []policyconflict.Disclosure{}, Verdict: policyconflict.VerdictPass,
	}
	repository := []byte(`{"remote_origin":{"known":true,"value":"origin"},"branch":{"known":true,"value":"feature/test"},"head":{"known":true,"value":"` + manifest.Repository.Head.Value + `"},"default_branch":{"known":true,"name":"main","ref":"refs/heads/main","head":"` + manifest.Repository.Head.Value + `"},"relationship":"equal","dirty":{"known":true,"value":false},"staged":{"known":true,"value":false},"worktree":{"managed":true,"name":"test-worktree"},"source":"head"}`)
	if err := json.Unmarshal(repository, &report.Input.Repository); err != nil {
		t.Fatal(err)
	}
	encoded, err := policyconflict.EncodeReport(report)
	if err != nil {
		t.Fatalf("EncodeReport(review): %v", err)
	}
	report, err = policyconflict.DecodeReport(encoded)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func assertReviewResult(t *testing.T, result Result, request Request) {
	t.Helper()
	if result.Schema != ResultSchemaID || result.Round != request.Packet.Round || result.PacketDigest != request.Packet.Digest {
		t.Fatalf("result identity = %#v, want schema/round/packet", result)
	}
	if result.ReviewReceipt.Role != contextreceipt.RoleReviewer || result.ExecutionResult.Receipt.Digest != result.ReviewReceipt.Digest {
		t.Fatalf("result reviewer receipt cross-match failed: %#v", result.ReviewReceipt)
	}
	if result.ReceiptEventAck.ReceiptDigest != result.ReviewReceipt.Digest || result.ExecutionResult.ReceiptEventAck != result.ReceiptEventAck {
		t.Fatal("result specialized receipt acknowledgment cross-match failed")
	}
}

func assertLaunchInput(t *testing.T, call reviewExecutionCall, packet PacketResult, request sealedexec.ExecutionRequest, prior *PriorReview) {
	t.Helper()
	if call.Request.Action != sealedexec.ActionStart || call.Request.Resume != nil || call.Request.Start == nil {
		t.Fatal("review executor received a non-start request")
	}
	wantLaunch := sealedexec.ReviewLaunch{Round: string(packet.Packet.Round), PacketDigest: packet.Packet.Digest, Model: packet.Packet.Reviewer.Model}
	if prior != nil {
		wantLaunch.PriorReview = &sealedexec.ReviewPrior{ReceiptDigest: prior.ReceiptDigest, AdjudicationDigest: prior.AdjudicationDigest}
	}
	if !reflect.DeepEqual(call.Launch, wantLaunch) {
		t.Fatalf("review launch = %#v, want %#v", call.Launch, wantLaunch)
	}
	if !reflect.DeepEqual(call.Request, request) || !bytes.Equal(call.PacketBytes, packet.PacketBytes) {
		t.Fatal("review executor did not receive the exact execution request and canonical packet")
	}
}

func assertLaunchFactsLiteral(t *testing.T, run sealedexec.ExecutionRun, packet PacketResult, request sealedexec.ExecutionRequest, prior *PriorReview) {
	t.Helper()
	if run.ReviewLaunchFacts == nil || run.ReviewLaunchEvent == nil || run.ReviewLaunchAck == nil {
		t.Fatal("review run lacks acknowledged launch projection")
	}
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	priorLiteral := "null"
	if prior != nil {
		priorLiteral = fmt.Sprintf(`{"adjudication_digest":%q,"receipt_digest":%q}`, prior.AdjudicationDigest, prior.ReceiptDigest)
	}
	want := []byte(fmt.Sprintf(`{"adapter":"codex","adapter_version":"1.2.3","lane":"review","model":"gpt-5.6","packet_digest":%q,"prior_review":%s,"profile_digest":%q,"profile_id":"reviewer-profile","round":%q,"schema":"verdi.sealed-review-launch-facts/v1","session":%q,"workspace_id":%q}`,
		packet.Packet.Digest, priorLiteral, testDigestA, packet.Packet.Round, request.Session, workspaceID))
	got, err := sealedexec.EncodeReviewLaunchFacts(*run.ReviewLaunchFacts)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("launch facts canonical bytes changed\ngot:  %s\nwant: %s", got, want)
	}
	payload, ok := run.ReviewLaunchEvent.Payload.(*contextevent.AdapterStartPayload)
	if !ok || payload.Detail == nil || !bytes.Equal(payload.Detail.RedactedJSON, want) || payload.Detail.Digest != rawDigestForTest(want) {
		t.Fatalf("launch facts detail did not bind exact literal bytes: %#v", run.ReviewLaunchEvent.Payload)
	}
	if run.ReviewLaunchAck.EventDigest != run.ReviewLaunchEvent.EventDigest || run.ReviewLaunchAck.SourceSequence != run.ReviewLaunchEvent.SourceSequence {
		t.Fatal("launch event acknowledgment is mispaired")
	}
}

func cloneReviewRequest(t *testing.T, request Request) Request {
	t.Helper()
	encoded, err := EncodeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	cloned, err := DecodeRequest(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	return cloned
}
