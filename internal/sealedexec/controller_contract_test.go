package sealedexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextreceipt"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyconflict"
)

func TestContextControllerWireContract_Static(t *testing.T) {
	t.Run("closed registry and typed canonical envelopes", func(t *testing.T) {
		literals := []struct {
			operation     ControllerOperation
			requestSchema string
			resultSchema  string
		}{
			{operation: "verify-authority", requestSchema: "verdi.context-controller/verify-authority-request/v1", resultSchema: "verdi.context-controller/verify-authority-result/v1"},
			{operation: "resolve-profile", requestSchema: "verdi.context-controller/resolve-profile-request/v1", resultSchema: "verdi.context-controller/resolve-profile-result/v1"},
			{operation: "verify-conflict", requestSchema: "verdi.context-controller/verify-conflict-request/v1", resultSchema: "verdi.context-controller/verify-conflict-result/v1"},
			{operation: "resolve-recorder", requestSchema: "verdi.context-controller/resolve-recorder-request/v1", resultSchema: "verdi.context-controller/resolve-recorder-result/v1"},
			{operation: "recorder-checkpoint", requestSchema: "verdi.context-controller/recorder-checkpoint-request/v1", resultSchema: "verdi.context-controller/recorder-checkpoint-result/v1"},
			{operation: "recorder-append", requestSchema: "verdi.context-controller/recorder-append-request/v1", resultSchema: "verdi.context-controller/recorder-append-result/v1"},
			{operation: "verify-opaque-boundary", requestSchema: "verdi.context-controller/verify-opaque-boundary-request/v1", resultSchema: "verdi.context-controller/verify-opaque-boundary-result/v1"},
			{operation: "verify-provider-session", requestSchema: "verdi.context-controller/verify-provider-session-request/v1", resultSchema: "verdi.context-controller/verify-provider-session-result/v1"},
			{operation: "verify-expansion", requestSchema: "verdi.context-controller/verify-expansion-request/v1", resultSchema: "verdi.context-controller/verify-expansion-result/v1"},
			{operation: "store-adapter-session", requestSchema: "verdi.context-controller/store-adapter-session-request/v1", resultSchema: "verdi.context-controller/store-adapter-session-result/v1"},
			{operation: "next-stamp", requestSchema: "verdi.context-controller/next-stamp-request/v1", resultSchema: "verdi.context-controller/next-stamp-result/v1"},
			{operation: "resolve-context", requestSchema: "verdi.context-controller/resolve-context-request/v1", resultSchema: "verdi.context-controller/resolve-context-result/v1"},
			{operation: "verify-epoch", requestSchema: "verdi.context-controller/verify-epoch-request/v1", resultSchema: "verdi.context-controller/verify-epoch-result/v1"},
			{operation: "install-expansion", requestSchema: "verdi.context-controller/install-expansion-request/v1", resultSchema: "verdi.context-controller/install-expansion-result/v1"},
			{operation: "resolve-receipt-inputs", requestSchema: "verdi.context-controller/resolve-receipt-inputs-request/v1", resultSchema: "verdi.context-controller/resolve-receipt-inputs-result/v1"},
			{operation: "append-receipt", requestSchema: "verdi.context-controller/append-receipt-request/v1", resultSchema: "verdi.context-controller/append-receipt-result/v1"},
			{operation: "persist-handback", requestSchema: "verdi.context-controller/persist-handback-request/v1", resultSchema: "verdi.context-controller/persist-handback-result/v1"},
			{operation: "persist-quarantine", requestSchema: "verdi.context-controller/persist-quarantine-request/v1", resultSchema: "verdi.context-controller/persist-quarantine-result/v1"},
			{operation: "persist-abort", requestSchema: "verdi.context-controller/persist-abort-request/v1", resultSchema: "verdi.context-controller/persist-abort-result/v1"},
		}
		if got, want := len(controllerOperations), len(literals); got != want {
			t.Fatalf("controller operation count = %d, want %d", got, want)
		}
		for i, literal := range literals {
			literal := literal
			operation := controllerOperations[i]
			t.Run(string(literal.operation), func(t *testing.T) {
				call := controllerCallFixture(t, uint64(i+1), operation)
				encodedCall, err := EncodeControllerCall(call)
				if err != nil {
					t.Fatalf("EncodeControllerCall: %v", err)
				}
				if !bytes.HasSuffix(encodedCall, []byte("\n")) || bytes.HasSuffix(encodedCall, []byte("\n\n")) {
					t.Fatalf("call does not carry exactly one canonical LF: %q", encodedCall)
				}
				if !bytes.Contains(encodedCall, []byte(`"operation":"`+string(literal.operation)+`"`)) {
					t.Fatalf("call operation bytes do not contain literal %q: %s", literal.operation, encodedCall)
				}
				if !bytes.Contains(encodedCall, []byte(`"schema":"`+literal.requestSchema+`"`)) {
					t.Fatalf("call lacks literal request schema %q: %s", literal.requestSchema, encodedCall)
				}
				decodedCall, err := DecodeControllerCall(bytes.NewReader(encodedCall))
				if err != nil {
					t.Fatalf("DecodeControllerCall: %v", err)
				}
				if decodedCall.Operation != literal.operation || decodedCall.CallSequence != uint64(i+1) {
					t.Fatalf("decoded call identity = (%q,%d)", decodedCall.Operation, decodedCall.CallSequence)
				}

				result := controllerResultFixture(t, uint64(i+1), operation)
				encodedResult, err := EncodeControllerResult(result)
				if err != nil {
					t.Fatalf("EncodeControllerResult: %v", err)
				}
				if !bytes.Contains(encodedResult, []byte(`"operation":"`+string(literal.operation)+`"`)) {
					t.Fatalf("result operation bytes do not contain literal %q: %s", literal.operation, encodedResult)
				}
				if !bytes.Contains(encodedResult, []byte(`"schema":"`+literal.resultSchema+`"`)) {
					t.Fatalf("result lacks literal result schema %q: %s", literal.resultSchema, encodedResult)
				}
				decodedResult, err := DecodeControllerResult(bytes.NewReader(encodedResult))
				if err != nil {
					t.Fatalf("DecodeControllerResult: %v", err)
				}
				if decodedResult.Operation != literal.operation || decodedResult.CallSequence != uint64(i+1) || decodedResult.Error != nil {
					t.Fatalf("decoded result identity/arm = (%q,%d,%v)", decodedResult.Operation, decodedResult.CallSequence, decodedResult.Error)
				}
			})
		}
	})

	t.Run("all typed wrappers carry literal requests and return typed results", func(t *testing.T) {
		request := validExecutionRequest(t, ActionStart)
		event, eventAck := controllerEventFixture(t, request)
		receipt, receiptEvent, _ := controllerReceiptFixture(t, request)
		key := executionKey(request)
		profileQuery := ProfileQuery{Ref: request.Profile, WorkspacePath: filepath.Join("/tmp", "verdi-controller-workspace"), Grants: request.Grants}
		providerCheck := ProviderSessionCheck{SessionRef: "provider-session", AdapterVersion: request.AdapterVersion, ProfileDigest: request.Profile.Digest, WorkspaceID: "workspace-1"}
		sessionRecord := SessionRecord{Key: key, SessionRef: "provider-session", AdapterVersion: request.AdapterVersion, ProfileDigest: request.Profile.Digest, WorkspaceID: "workspace-1", LifecycleAck: eventAck}
		contextQuery := ContextQuery{Key: key, Ref: "spec/test#ac-1"}
		epochCheck := controllerEpochCheckFixture(t)
		expansionInstall := ExpansionInstall{Key: key, RequestID: "request-1", ParentRevision: 0, ParentManifestDigest: request.ManifestDigest, ChildRevision: 1, ChildManifestDigest: testDigest("child-manifest"), ExpansionDigest: testDigest("expansion"), ExpansionRoot: testDigest("expansion-root"), TerminalAck: eventAck}
		receiptQuery := ReceiptInputsQuery{Request: request, WorkspaceID: "workspace-1", DispatchDigest: testDigest("dispatch"), TerminalRevision: 0, TerminalSourceSequence: 1, TerminalGlobalSequence: 1, EventChainRoot: receipt.EventChainRoot, ResultFactsDigest: testDigest("result-facts")}
		receiptAppend := ReceiptAppend{Receipt: receipt, Event: receiptEvent}
		handbackInput := validHandbackRecord(t)
		handbackFrame := mustCanonicalHandback(t, handbackInput)
		quarantineInput := validQuarantineRecord(t, QuarantineExecutionIncomplete)
		quarantineFrame := mustCanonicalQuarantine(t, quarantineInput)
		abortQuarantine := mustCanonicalQuarantine(t, validQuarantineRecord(t, QuarantineTerminalDurabilityFailed))
		abortInput := validAbortRecord(t, abortQuarantine)
		abortFrame := mustCanonicalAbort(t, abortInput)
		opaqueRows := []contextcompile.OpaqueEntry{}

		verifyAuthorityResult := controllerResultFixture(t, 1, "verify-authority")
		resolveProfileResult := controllerResultFixture(t, 1, "resolve-profile")
		verifyConflictResult := controllerResultFixture(t, 1, "verify-conflict")
		resolveRecorderResult := controllerResultFixture(t, 1, "resolve-recorder")
		recorderCheckpointResult := controllerResultFixture(t, 1, "recorder-checkpoint")
		recorderAppendResult := controllerResultFixture(t, 1, "recorder-append")
		verifyOpaqueResult := controllerResultFixture(t, 1, "verify-opaque-boundary")
		verifyProviderResult := controllerResultFixture(t, 1, "verify-provider-session")
		verifyExpansionResult := controllerResultFixture(t, 1, "verify-expansion")
		storeSessionResult := controllerResultFixture(t, 1, "store-adapter-session")
		nextStampResult := controllerResultFixture(t, 1, "next-stamp")
		resolveContextResult := controllerResultFixture(t, 1, "resolve-context")
		verifyEpochResult := controllerResultFixture(t, 1, "verify-epoch")
		installExpansionResult := controllerResultFixture(t, 1, "install-expansion")
		resolveReceiptInputsResult := controllerResultFixture(t, 1, "resolve-receipt-inputs")
		appendReceiptResult := controllerResultFixture(t, 1, "append-receipt")
		persistHandbackResult := controllerResultFixture(t, 1, "persist-handback")
		persistQuarantineResult := controllerResultFixture(t, 1, "persist-quarantine")
		persistAbortResult := controllerResultFixture(t, 1, "persist-abort")

		rows := []controllerWrapperCase{
			{name: "VerifyAuthority", operation: "verify-authority", requestSchema: "verdi.context-controller/verify-authority-request/v1", requestField: "request", requestValue: request, reply: verifyAuthorityResult, want: verifyAuthorityResult.VerifyAuthority.Facts, invoke: func(client *ControllerClient) (any, error) {
				return client.VerifyAuthority(context.Background(), request)
			}},
			{name: "ResolveProfile", operation: "resolve-profile", requestSchema: "verdi.context-controller/resolve-profile-request/v1", requestField: "query", requestValue: profileQuery, reply: resolveProfileResult, want: resolveProfileResult.ResolveProfile.Material, invoke: func(client *ControllerClient) (any, error) {
				return client.ResolveProfile(context.Background(), profileQuery)
			}},
			{name: "VerifyConflict", operation: "verify-conflict", requestSchema: "verdi.context-controller/verify-conflict-request/v1", requestField: "report", requestValue: request.AuthorityVerdict, reply: verifyConflictResult, want: verifyConflictResult.VerifyConflict.Facts, invoke: func(client *ControllerClient) (any, error) {
				return client.VerifyConflict(context.Background(), request.AuthorityVerdict)
			}},
			{name: "ResolveRecorder", operation: "resolve-recorder", requestSchema: "verdi.context-controller/resolve-recorder-request/v1", requestField: "ref", requestValue: request.RecorderEndpoint, reply: resolveRecorderResult, want: resolveRecorderResult.ResolveRecorder.Facts, invoke: func(client *ControllerClient) (any, error) {
				return client.ResolveRecorder(context.Background(), request.RecorderEndpoint)
			}},
			{name: "RecorderCheckpoint", operation: "recorder-checkpoint", requestSchema: "verdi.context-controller/recorder-checkpoint-request/v1", requestField: "key", requestValue: key, reply: recorderCheckpointResult, want: recorderCheckpointResult.RecorderCheckpoint.Checkpoint, invoke: func(client *ControllerClient) (any, error) {
				return client.RecorderCheckpoint(context.Background(), key)
			}},
			{name: "RecorderAppend", operation: "recorder-append", requestSchema: "verdi.context-controller/recorder-append-request/v1", requestField: "event", requestValue: event, reply: recorderAppendResult, want: recorderAppendResult.RecorderAppend.Ack, invoke: func(client *ControllerClient) (any, error) { return client.RecorderAppend(context.Background(), event) }},
			{name: "VerifyOpaqueBoundary", operation: "verify-opaque-boundary", requestSchema: "verdi.context-controller/verify-opaque-boundary-request/v1", requestField: "rows", requestValue: opaqueRows, reply: verifyOpaqueResult, want: verifyOpaqueResult.VerifyOpaqueBoundary.Facts, invoke: func(client *ControllerClient) (any, error) {
				return client.VerifyOpaqueBoundary(context.Background(), opaqueRows)
			}},
			{name: "VerifyProviderSession", operation: "verify-provider-session", requestSchema: "verdi.context-controller/verify-provider-session-request/v1", requestField: "check", requestValue: providerCheck, reply: verifyProviderResult, want: verifyProviderResult.VerifyProviderSession.Facts, invoke: func(client *ControllerClient) (any, error) {
				return client.VerifyProviderSession(context.Background(), providerCheck)
			}},
			{name: "VerifyExpansion", operation: "verify-expansion", requestSchema: "verdi.context-controller/verify-expansion-request/v1", requestField: "key", requestValue: key, reply: verifyExpansionResult, want: verifyExpansionResult.VerifyExpansion.Facts, invoke: func(client *ControllerClient) (any, error) { return client.VerifyExpansion(context.Background(), key) }},
			{name: "StoreAdapterSession", operation: "store-adapter-session", requestSchema: "verdi.context-controller/store-adapter-session-request/v1", requestField: "record", requestValue: sessionRecord, reply: storeSessionResult, invoke: func(client *ControllerClient) (any, error) {
				return nil, client.StoreAdapterSession(context.Background(), sessionRecord)
			}},
			{name: "NextStamp", operation: "next-stamp", requestSchema: "verdi.context-controller/next-stamp-request/v1", reply: nextStampResult, want: nextStampResult.NextStamp.Stamp, invoke: func(client *ControllerClient) (any, error) { return client.NextStamp(context.Background()) }},
			{name: "ResolveContext", operation: "resolve-context", requestSchema: "verdi.context-controller/resolve-context-request/v1", requestField: "query", requestValue: contextQuery, reply: resolveContextResult, want: resolveContextResult.ResolveContext.Resolution, invoke: func(client *ControllerClient) (any, error) {
				return client.ResolveContext(context.Background(), contextQuery)
			}},
			{name: "VerifyEpoch", operation: "verify-epoch", requestSchema: "verdi.context-controller/verify-epoch-request/v1", requestField: "check", requestValue: epochCheck, reply: verifyEpochResult, want: verifyEpochResult.VerifyEpoch.Verification, invoke: func(client *ControllerClient) (any, error) {
				return client.VerifyEpoch(context.Background(), epochCheck)
			}},
			{name: "InstallExpansion", operation: "install-expansion", requestSchema: "verdi.context-controller/install-expansion-request/v1", requestField: "install", requestValue: expansionInstall, reply: installExpansionResult, invoke: func(client *ControllerClient) (any, error) {
				return nil, client.InstallExpansion(context.Background(), expansionInstall)
			}},
			{name: "ResolveReceiptInputs", operation: "resolve-receipt-inputs", requestSchema: "verdi.context-controller/resolve-receipt-inputs-request/v1", requestField: "query", requestValue: receiptQuery, reply: resolveReceiptInputsResult, want: resolveReceiptInputsResult.ResolveReceiptInputs.Inputs, invoke: func(client *ControllerClient) (any, error) {
				return client.ResolveReceiptInputs(context.Background(), receiptQuery)
			}},
			{name: "AppendReceipt", operation: "append-receipt", requestSchema: "verdi.context-controller/append-receipt-request/v1", requestField: "append", requestValue: receiptAppend, reply: appendReceiptResult, want: appendReceiptResult.AppendReceipt.Ack, invoke: func(client *ControllerClient) (any, error) {
				return client.AppendReceipt(context.Background(), receiptAppend)
			}},
			{name: "PersistHandback", operation: "persist-handback", requestSchema: "verdi.context-controller/persist-handback-request/v1", requestField: "record", requestValue: handbackFrame, reply: persistHandbackResult, want: persistHandbackResult.PersistHandback.Ack, invoke: func(client *ControllerClient) (any, error) {
				return client.PersistHandback(context.Background(), handbackInput)
			}},
			{name: "PersistQuarantine", operation: "persist-quarantine", requestSchema: "verdi.context-controller/persist-quarantine-request/v1", requestField: "record", requestValue: quarantineFrame, reply: persistQuarantineResult, want: persistQuarantineResult.PersistQuarantine.Ack, invoke: func(client *ControllerClient) (any, error) {
				return client.PersistQuarantine(context.Background(), quarantineInput)
			}},
			{name: "PersistAbort", operation: "persist-abort", requestSchema: "verdi.context-controller/persist-abort-request/v1", requestField: "record", requestValue: abortFrame, reply: persistAbortResult, want: persistAbortResult.PersistAbort.Ack, invoke: func(client *ControllerClient) (any, error) {
				return client.PersistAbort(context.Background(), abortInput)
			}},
		}

		if got, want := len(rows), 19; got != want {
			t.Fatalf("typed wrapper row count = %d, want %d", got, want)
		}
		for _, row := range rows {
			row := row
			t.Run(row.name, func(t *testing.T) {
				transport := &controllerMemoryTransport{read: bytes.NewReader(mustEncodeControllerResult(t, row.reply))}
				client, err := NewControllerClient(transport)
				if err != nil {
					t.Fatalf("NewControllerClient: %v", err)
				}
				got, err := row.invoke(client)
				if err != nil {
					t.Fatalf("%s: %v", row.name, err)
				}
				assertLiteralControllerRequestFrame(t, transport.written.Bytes(), row)
				if !reflect.DeepEqual(got, row.want) {
					t.Fatalf("%s result = %#v, want %#v", row.name, got, row.want)
				}
			})
		}

		t.Run("shared typed errors reach wrappers without bespoke identity checks", func(t *testing.T) {
			needsTypedError := map[string]bool{
				"RecorderCheckpoint":   true,
				"VerifyExpansion":      true,
				"StoreAdapterSession":  true,
				"NextStamp":            true,
				"VerifyEpoch":          true,
				"InstallExpansion":     true,
				"ResolveReceiptInputs": true,
			}
			for _, row := range rows {
				if !needsTypedError[row.name] {
					continue
				}
				row := row
				t.Run(row.name, func(t *testing.T) {
					reply := ControllerResult{
						Schema:       ControllerResultSchemaID,
						CallSequence: 1,
						Operation:    ControllerOperation(row.operation),
						Error: &ControllerError{
							Schema: ControllerErrorSchemaID, Class: ControllerErrorClassOperational,
							Code: ControllerErrorUnavailable, Witnesses: []string{"controller unavailable"},
						},
					}
					transport := &controllerMemoryTransport{read: bytes.NewReader(mustEncodeControllerResult(t, reply))}
					client, err := NewControllerClient(transport)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := row.invoke(client); !errors.Is(err, ErrOperational) {
						t.Fatalf("%s typed error = %v", row.name, err)
					}
				})
			}
		})

		t.Run("wrapper-owned result identities fail closed", func(t *testing.T) {
			differentReport := request.AuthorityVerdict
			differentReport.Digest = ""
			differentReport.Input.EvaluatedOn = "2026-08-28"
			reportBytes, err := policyconflict.EncodeReport(differentReport)
			if err != nil {
				t.Fatalf("encode distinct conflict report fixture: %v", err)
			}
			differentReport, err = policyconflict.DecodeReport(reportBytes)
			if err != nil {
				t.Fatalf("decode distinct conflict report fixture: %v", err)
			}

			badAuthority := controllerResultFixture(t, 1, "verify-authority")
			badAuthority.VerifyAuthority.Facts.AcceptedSpecCommit = testSHA2
			badProfile := controllerResultFixture(t, 1, "resolve-profile")
			badProfile.ResolveProfile.Material.Ref = LogicalRef{Schema: request.Profile.Schema, ID: "different-profile", Digest: testDigest("different-profile")}
			badConflict := controllerResultFixture(t, 1, "verify-conflict")
			badRecorder := controllerResultFixture(t, 1, "resolve-recorder")
			badRecorder.ResolveRecorder.Facts.Ref = LogicalRef{Schema: request.RecorderEndpoint.Schema, ID: "different-recorder", Digest: testDigest("different-recorder")}
			badEventAck := controllerResultFixture(t, 1, "recorder-append")
			badEventAck.RecorderAppend.Ack.EventDigest = testDigest("different-event")
			badOpaque := controllerResultFixture(t, 1, "verify-opaque-boundary")
			badOpaque.VerifyOpaqueBoundary.Facts.Rows = []OpaqueIdentity{{ID: "opaque-other", Kind: "other", AdapterID: "adapter", AdapterVersion: "1.0.0"}}
			badProvider := controllerResultFixture(t, 1, "verify-provider-session")
			badProvider.VerifyProviderSession.Facts.WorkspaceID = "different-workspace"
			badContext := controllerResultFixture(t, 1, "resolve-context")
			badContext.ResolveContext.Resolution.Ref = "spec/other#ac-1"
			badReceiptAck := controllerResultFixture(t, 1, "append-receipt")
			badReceiptAck.AppendReceipt.Ack.ReceiptDigest = testDigest("different-receipt")
			badHandbackAck := controllerResultFixture(t, 1, "persist-handback")
			badHandbackAck.PersistHandback.Ack.WorkspaceID = "different-workspace"
			badHandbackAck.PersistHandback.Ack.Digest = ""
			badHandbackAck.PersistHandback.Ack = mustCanonicalControlAck(t, badHandbackAck.PersistHandback.Ack)
			badQuarantineAck := controllerResultFixture(t, 1, "persist-quarantine")
			badQuarantineAck.PersistQuarantine.Ack.WorkspaceID = "different-workspace"
			badQuarantineAck.PersistQuarantine.Ack.Digest = ""
			badQuarantineAck.PersistQuarantine.Ack = mustCanonicalControlAck(t, badQuarantineAck.PersistQuarantine.Ack)
			badAbortAck := controllerResultFixture(t, 1, "persist-abort")
			badAbortAck.PersistAbort.Ack.WorkspaceID = "different-workspace"
			badAbortAck.PersistAbort.Ack.Digest = ""
			badAbortAck.PersistAbort.Ack = mustCanonicalControlAck(t, badAbortAck.PersistAbort.Ack)

			mismatches := []struct {
				name   string
				reply  ControllerResult
				invoke func(*ControllerClient) (any, error)
			}{
				{name: "authority", reply: badAuthority, invoke: func(client *ControllerClient) (any, error) {
					return client.VerifyAuthority(context.Background(), request)
				}},
				{name: "profile ref", reply: badProfile, invoke: func(client *ControllerClient) (any, error) {
					return client.ResolveProfile(context.Background(), profileQuery)
				}},
				{name: "conflict report", reply: badConflict, invoke: func(client *ControllerClient) (any, error) {
					return client.VerifyConflict(context.Background(), differentReport)
				}},
				{name: "recorder ref", reply: badRecorder, invoke: func(client *ControllerClient) (any, error) {
					return client.ResolveRecorder(context.Background(), request.RecorderEndpoint)
				}},
				{name: "recorder event ack", reply: badEventAck, invoke: func(client *ControllerClient) (any, error) { return client.RecorderAppend(context.Background(), event) }},
				{name: "opaque rows", reply: badOpaque, invoke: func(client *ControllerClient) (any, error) {
					return client.VerifyOpaqueBoundary(context.Background(), opaqueRows)
				}},
				{name: "provider-session facts", reply: badProvider, invoke: func(client *ControllerClient) (any, error) {
					return client.VerifyProviderSession(context.Background(), providerCheck)
				}},
				{name: "context ref", reply: badContext, invoke: func(client *ControllerClient) (any, error) {
					return client.ResolveContext(context.Background(), contextQuery)
				}},
				{name: "receipt ack", reply: badReceiptAck, invoke: func(client *ControllerClient) (any, error) {
					return client.AppendReceipt(context.Background(), receiptAppend)
				}},
				{name: "handback ack", reply: badHandbackAck, invoke: func(client *ControllerClient) (any, error) {
					return client.PersistHandback(context.Background(), handbackInput)
				}},
				{name: "quarantine ack", reply: badQuarantineAck, invoke: func(client *ControllerClient) (any, error) {
					return client.PersistQuarantine(context.Background(), quarantineInput)
				}},
				{name: "abort ack", reply: badAbortAck, invoke: func(client *ControllerClient) (any, error) {
					return client.PersistAbort(context.Background(), abortInput)
				}},
			}
			for _, mismatch := range mismatches {
				mismatch := mismatch
				t.Run(mismatch.name, func(t *testing.T) {
					transport := &controllerMemoryTransport{read: bytes.NewReader(mustEncodeControllerResult(t, mismatch.reply))}
					client, err := NewControllerClient(transport)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := mismatch.invoke(client); !errors.Is(err, ErrOperational) {
						t.Fatalf("%s mismatch error = %v", mismatch.name, err)
					}
				})
			}
		})
	})

	t.Run("strict envelope mutations", func(t *testing.T) {
		callBytes := mustEncodeControllerCall(t, controllerCallFixture(t, 1, ControllerOperationNextStamp))
		resultBytes := mustEncodeControllerResult(t, controllerResultFixture(t, 1, ControllerOperationNextStamp))
		errorBytes := mustEncodeControllerResult(t, ControllerResult{
			Schema: ControllerResultSchemaID, CallSequence: 1, Operation: ControllerOperationNextStamp,
			Error: &ControllerError{Schema: ControllerErrorSchemaID, Class: ControllerErrorClassOperational, Code: ControllerErrorUnavailable, Witnesses: []string{"controller unavailable"}},
		})

		callMutations := map[string][]byte{
			"unknown operation":         bytes.Replace(callBytes, []byte(`"operation":"next-stamp"`), []byte(`"operation":"future"`), 1),
			"operation schema mismatch": bytes.Replace(callBytes, []byte(`next-stamp-request`), []byte(`resolve-context-request`), 1),
			"zero sequence":             bytes.Replace(callBytes, []byte(`"call_sequence":1`), []byte(`"call_sequence":0`), 1),
			"duplicate":                 bytes.Replace(callBytes, []byte(`"schema":"verdi.context-controller-call/v1"`), []byte(`"schema":"verdi.context-controller-call/v1","schema":"verdi.context-controller-call/v1"`), 1),
			"unknown":                   bytes.Replace(callBytes, []byte(`"payload":`), []byte(`"future":true,"payload":`), 1),
			"null":                      bytes.Replace(callBytes, []byte(`"payload":{"schema"`), []byte(`"payload":null,"discard":{"schema"`), 1),
			"trailing":                  append(append([]byte(nil), callBytes...), []byte("{}\n")...),
			"noncanonical":              append([]byte(" "), callBytes...),
		}
		for name, mutation := range callMutations {
			t.Run("call/"+name, func(t *testing.T) {
				if _, err := DecodeControllerCall(bytes.NewReader(mutation)); err == nil {
					t.Fatalf("DecodeControllerCall accepted %s: %q", name, mutation)
				}
			})
		}

		resultMutations := map[string][]byte{
			"wrong result type": bytes.Replace(resultBytes, []byte(`next-stamp-result`), []byte(`resolve-context-result`), 1),
			"result plus error": bytes.Replace(resultBytes, []byte(`"result":`), []byte(`"error":{"schema":"verdi.context-controller-error/v1","class":"operational","code":"internal","witnesses":["failure"]},"result":`), 1),
			"neither arm":       []byte(`{"call_sequence":1,"operation":"next-stamp","payload":{},"schema":"verdi.context-controller-result/v1"}` + "\n"),
			"null arm":          bytes.Replace(resultBytes, []byte(`"result":{`), []byte(`"result":null,"discard":{`), 1),
			"duplicate":         bytes.Replace(resultBytes, []byte(`"call_sequence":1`), []byte(`"call_sequence":1,"call_sequence":1`), 1),
			"unknown":           bytes.Replace(resultBytes, []byte(`"operation":`), []byte(`"future":true,"operation":`), 1),
			"trailing":          append(append([]byte(nil), resultBytes...), []byte("{}\n")...),
			"noncanonical":      append([]byte(" "), resultBytes...),
		}
		for name, mutation := range resultMutations {
			t.Run("result/"+name, func(t *testing.T) {
				if _, err := DecodeControllerResult(bytes.NewReader(mutation)); err == nil {
					t.Fatalf("DecodeControllerResult accepted %s: %q", name, mutation)
				}
			})
		}

		for _, code := range []string{"", "network", "verdict", "retryable"} {
			t.Run("unknown error code/"+code, func(t *testing.T) {
				mutation := bytes.Replace(errorBytes, []byte(`"code":"unavailable"`), []byte(`"code":"`+code+`"`), 1)
				if _, err := DecodeControllerResult(bytes.NewReader(mutation)); err == nil {
					t.Fatalf("accepted unknown controller error code %q", code)
				}
			})
		}
	})

	t.Run("append receipt owns two-domain identity cross-checks", func(t *testing.T) {
		request := validExecutionRequest(t, ActionStart)
		for name, mutate := range map[string]func(*ReceiptAppend){
			"receipt self digest": func(appendValue *ReceiptAppend) {
				appendValue.Event.Payload.(*contextevent.ReceiptPayload).ReceiptDigest = testDigest("different-receipt")
			},
			"role": func(appendValue *ReceiptAppend) {
				appendValue.Event.Payload.(*contextevent.ReceiptPayload).Role = contextevent.RoleReviewer
			},
			"authority": func(appendValue *ReceiptAppend) {
				appendValue.Event.Payload.(*contextevent.ReceiptPayload).Authority = contextevent.AuthorityAdvisory
			},
			"execution root": func(appendValue *ReceiptAppend) {
				appendValue.Event.Payload.(*contextevent.ReceiptPayload).ExecutionEventChainRoot = testDigest("different-root")
			},
			"represented bytes": func(appendValue *ReceiptAppend) {
				value := any(map[string]any{"message": "not the receipt"})
				raw, err := canonjson.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				digest, err := canonjson.Digest(value)
				if err != nil {
					t.Fatal(err)
				}
				detail := &appendValue.Event.Payload.(*contextevent.ReceiptPayload).Detail
				detail.RedactedJSON = bytes.TrimSuffix(raw, []byte("\n"))
				detail.Digest = digest
			},
		} {
			t.Run(name, func(t *testing.T) {
				receipt, event, _ := controllerReceiptFixture(t, request)
				appendValue := ReceiptAppend{Receipt: receipt, Event: event}
				mutate(&appendValue)
				appendValue.Event = canonicalControllerEvent(t, appendValue.Event)
				call := ControllerCall{Schema: ControllerCallSchemaID, CallSequence: 1, Operation: ControllerOperationAppendReceipt}
				call.AppendReceipt = ControllerAppendReceiptRequest{Schema: controllerRequestSchema(call.Operation), Append: appendValue}
				if _, err := EncodeControllerCall(call); err == nil {
					t.Fatalf("EncodeControllerCall accepted conflicting %s", name)
				}
			})
		}

		t.Run("malformed detail digest", func(t *testing.T) {
			receipt, event, _ := controllerReceiptFixture(t, request)
			event.Payload.(*contextevent.ReceiptPayload).Detail.Digest = testDigest("not-the-detail")
			call := ControllerCall{Schema: ControllerCallSchemaID, CallSequence: 1, Operation: ControllerOperationAppendReceipt}
			call.AppendReceipt = ControllerAppendReceiptRequest{Schema: controllerRequestSchema(call.Operation), Append: ReceiptAppend{Receipt: receipt, Event: event}}
			if _, err := EncodeControllerCall(call); err == nil {
				t.Fatal("EncodeControllerCall accepted malformed represented-byte digest")
			}
		})
	})

	t.Run("receipt input rows retain component ordering", func(t *testing.T) {
		result := controllerResultFixture(t, 1, ControllerOperationResolveReceiptInputs)
		result.ResolveReceiptInputs.Inputs.Expansions = []contextreceipt.Expansion{
			{RequestID: "same-request", ParentRevision: 2, ParentManifestDigest: testDigest("parent-2"), ChildRevision: 3, ChildManifestDigest: testDigest("child-3"), ExpansionDigest: testDigest("expansion-2")},
			{RequestID: "same-request", ParentRevision: 10, ParentManifestDigest: testDigest("parent-10"), ChildRevision: 11, ChildManifestDigest: testDigest("child-11"), ExpansionDigest: testDigest("expansion-10")},
		}
		if _, err := EncodeControllerResult(result); err != nil {
			t.Fatalf("EncodeControllerResult(component-ordered numeric rows): %v", err)
		}
	})

	t.Run("sequential client and verdict preservation", func(t *testing.T) {
		violated := controllerResultFixture(t, 1, ControllerOperationVerifyEpoch)
		violated.VerifyEpoch.Verification = Verification{
			State: contextcompile.ResolutionViolatedWithWitness, Failure: FailureMismatch,
			Witnesses: []string{"epoch changed"},
		}
		reply := mustEncodeControllerResult(t, violated)
		transport := &controllerMemoryTransport{read: bytes.NewReader(reply)}
		client, err := NewControllerClient(transport)
		if err != nil {
			t.Fatalf("NewControllerClient: %v", err)
		}
		verification, err := client.VerifyEpoch(context.Background(), controllerEpochCheckFixture(t))
		if err != nil {
			t.Fatalf("negative proof traveled as controller error: %v", err)
		}
		verdictErr := requireProven("epoch", verification)
		if !errors.Is(verdictErr, ErrVerdict) || errors.Is(verdictErr, ErrOperational) {
			t.Fatalf("negative proof classification = %v", verdictErr)
		}
		if !bytes.Contains(transport.written.Bytes(), []byte(`"call_sequence":1`)) {
			t.Fatalf("first call did not use sequence 1: %s", transport.written.Bytes())
		}

		for name, sequence := range map[string]uint64{"gapped": 2, "replayed": 0} {
			t.Run(name, func(t *testing.T) {
				bad := controllerResultFixture(t, sequence, ControllerOperationNextStamp)
				if sequence == 0 {
					// The codec itself rejects zero, so replay a valid prior sequence
					// against a client whose next call is two.
					first := mustEncodeControllerResult(t, controllerResultFixture(t, 1, ControllerOperationNextStamp))
					second := mustEncodeControllerResult(t, controllerResultFixture(t, 1, ControllerOperationNextStamp))
					transport := &controllerMemoryTransport{read: bytes.NewReader(append(first, second...))}
					client, err := NewControllerClient(transport)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := client.NextStamp(context.Background()); err != nil {
						t.Fatalf("first NextStamp: %v", err)
					}
					if _, err := client.NextStamp(context.Background()); !errors.Is(err, ErrOperational) {
						t.Fatalf("replayed reply error = %v", err)
					}
					return
				}
				transport := &controllerMemoryTransport{read: bytes.NewReader(mustEncodeControllerResult(t, bad))}
				client, err := NewControllerClient(transport)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := client.NextStamp(context.Background()); !errors.Is(err, ErrOperational) {
					t.Fatalf("gapped reply error = %v", err)
				}
			})
		}
	})

	t.Run("partial and canceled transports poison the client", func(t *testing.T) {
		partial := &partialControllerTransport{}
		client, err := NewControllerClient(partial)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.NextStamp(context.Background()); !errors.Is(err, ErrOperational) {
			t.Fatalf("partial write error = %v", err)
		}
		if _, err := client.NextStamp(context.Background()); !errors.Is(err, ErrOperational) {
			t.Fatalf("poisoned client error = %v", err)
		}

		short := &shortNilControllerTransport{}
		client, err = NewControllerClient(short)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.NextStamp(context.Background()); !errors.Is(err, ErrOperational) {
			t.Fatalf("short nil write error = %v", err)
		}
		if short.writes != 1 {
			t.Fatalf("partially written call was retried %d times", short.writes)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		transport := &controllerMemoryTransport{read: bytes.NewReader(nil)}
		client, err = NewControllerClient(transport)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.NextStamp(ctx); !errors.Is(err, ErrOperational) {
			t.Fatalf("canceled call error = %v", err)
		}
		if _, err := client.NextStamp(context.Background()); !errors.Is(err, ErrOperational) {
			t.Fatalf("canceled client was reused: %v", err)
		}
	})

	t.Run("constructor rejects a non-closable transport", func(t *testing.T) {
		transport := &nonClosableControllerTransport{read: bytes.NewReader(nil)}
		if _, err := NewControllerClient(transport); err == nil {
			t.Fatal("NewControllerClient accepted a non-closable io.ReadWriter")
		}
	})

	t.Run("cancellation closes a blocking transport without retry", func(t *testing.T) {
		transport := newBlockingControllerTransport()
		defer transport.release()
		client, err := NewControllerClient(transport)
		if err != nil {
			t.Fatalf("NewControllerClient: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		callDone := make(chan error, 1)
		go func() {
			_, err := client.NextStamp(ctx)
			callDone <- err
		}()

		awaitControllerSignal(t, transport.readStarted, "controller reply read did not block")
		cancel()
		select {
		case err := <-callDone:
			if !errors.Is(err, ErrOperational) {
				t.Fatalf("canceled blocking call error = %v", err)
			}
		case <-time.After(2 * time.Second):
			transport.release()
			<-callDone
			t.Fatal("canceled blocking call did not return promptly")
		}
		awaitControllerSignal(t, transport.readReturned, "Close did not unblock the reply read")
		if got := transport.closes.Load(); got != 1 {
			t.Fatalf("transport Close count = %d, want 1", got)
		}
		if got := transport.writes.Load(); got != 1 {
			t.Fatalf("transport write count = %d, want 1", got)
		}
		if _, err := client.NextStamp(context.Background()); !errors.Is(err, ErrOperational) {
			t.Fatalf("canceled client was not poisoned: %v", err)
		}
		if got := transport.writes.Load(); got != 1 {
			t.Fatalf("poisoned client retried transport write; count = %d", got)
		}
	})

	t.Run("client cross-matches returned identities and the canonical sent record", func(t *testing.T) {
		t.Run("authority facts", func(t *testing.T) {
			result := controllerResultFixture(t, 1, ControllerOperationVerifyAuthority)
			result.VerifyAuthority.Facts.AcceptedSpecCommit = testSHA2
			transport := &controllerMemoryTransport{read: bytes.NewReader(mustEncodeControllerResult(t, result))}
			client, err := NewControllerClient(transport)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.VerifyAuthority(context.Background(), validExecutionRequest(t, ActionStart)); !errors.Is(err, ErrOperational) {
				t.Fatalf("VerifyAuthority(identity mismatch) error = %v", err)
			}
		})

		t.Run("receipt ack", func(t *testing.T) {
			request := validExecutionRequest(t, ActionStart)
			receipt, event, _ := controllerReceiptFixture(t, request)
			result := controllerResultFixture(t, 1, ControllerOperationAppendReceipt)
			result.AppendReceipt.Ack.ReceiptDigest = testDigest("different-receipt")
			transport := &controllerMemoryTransport{read: bytes.NewReader(mustEncodeControllerResult(t, result))}
			client, err := NewControllerClient(transport)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.AppendReceipt(context.Background(), ReceiptAppend{Receipt: receipt, Event: event}); !errors.Is(err, ErrOperational) {
				t.Fatalf("AppendReceipt(identity mismatch) error = %v", err)
			}
		})

		t.Run("blank record digest", func(t *testing.T) {
			record := validHandbackRecord(t)
			canonical := mustCanonicalHandback(t, record)
			result := ControllerResult{Schema: ControllerResultSchemaID, CallSequence: 1, Operation: ControllerOperationPersistHandback}
			result.PersistHandback = ControllerPersistHandbackResult{Schema: controllerResultSchema(result.Operation), Ack: mustCanonicalControlAck(t, validControlAckForHandback(canonical))}
			transport := &controllerMemoryTransport{read: bytes.NewReader(mustEncodeControllerResult(t, result))}
			client, err := NewControllerClient(transport)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := client.PersistHandback(context.Background(), record); err != nil {
				t.Fatalf("PersistHandback(blank input digest) error = %v", err)
			}
		})
	})
}

func canonicalControllerEvent(t *testing.T, event contextevent.Event) contextevent.Event {
	t.Helper()
	event.EventDigest = ""
	encoded, err := contextevent.EncodeEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := contextevent.DecodeEvent(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

type controllerWrapperCase struct {
	name          string
	operation     string
	requestSchema string
	requestField  string
	requestValue  any
	reply         ControllerResult
	want          any
	invoke        func(*ControllerClient) (any, error)
}

func assertLiteralControllerRequestFrame(t *testing.T, frame []byte, want controllerWrapperCase) {
	t.Helper()
	if !bytes.HasSuffix(frame, []byte("\n")) || bytes.HasSuffix(frame, []byte("\n\n")) {
		t.Fatalf("%s request frame lacks exactly one LF: %q", want.name, frame)
	}
	var envelope struct {
		Schema       string          `json:"schema"`
		CallSequence uint64          `json:"call_sequence"`
		Operation    string          `json:"operation"`
		Payload      json.RawMessage `json:"payload"`
	}
	decoder := json.NewDecoder(bytes.NewReader(frame))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("%s decode captured request envelope: %v", want.name, err)
	}
	if envelope.Schema != "verdi.context-controller-call/v1" || envelope.CallSequence != 1 || envelope.Operation != want.operation {
		t.Fatalf("%s envelope identity = (%q,%d,%q), want literal operation %q", want.name, envelope.Schema, envelope.CallSequence, envelope.Operation, want.operation)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		t.Fatalf("%s decode captured request payload: %v", want.name, err)
	}
	fieldCount := 1
	if want.requestField != "" {
		fieldCount = 2
	}
	if len(payload) != fieldCount {
		t.Fatalf("%s payload fields = %v, want schema plus %q", want.name, payload, want.requestField)
	}
	var schema string
	if err := json.Unmarshal(payload["schema"], &schema); err != nil || schema != want.requestSchema {
		t.Fatalf("%s request schema = %q (%v), want %q", want.name, schema, err, want.requestSchema)
	}
	if want.requestField == "" {
		return
	}
	_, ok := payload[want.requestField]
	if !ok {
		t.Fatalf("%s payload lacks literal field %q", want.name, want.requestField)
	}
	decoded, err := DecodeControllerCall(bytes.NewReader(frame))
	if err != nil {
		t.Fatalf("%s strict-decode captured request: %v", want.name, err)
	}
	gotValue := typedControllerRequestValue(t, decoded, want.operation)
	if !reflect.DeepEqual(gotValue, want.requestValue) {
		t.Fatalf("%s typed request field %q = %#v, want %#v", want.name, want.requestField, gotValue, want.requestValue)
	}
}

func typedControllerRequestValue(t *testing.T, call ControllerCall, operation string) any {
	t.Helper()
	switch operation {
	case "verify-authority":
		return call.VerifyAuthority.Request
	case "resolve-profile":
		return call.ResolveProfile.Query
	case "verify-conflict":
		return call.VerifyConflict.Report
	case "resolve-recorder":
		return call.ResolveRecorder.Ref
	case "recorder-checkpoint":
		return call.RecorderCheckpoint.Key
	case "recorder-append":
		return call.RecorderAppend.Event
	case "verify-opaque-boundary":
		return call.VerifyOpaqueBoundary.Rows
	case "verify-provider-session":
		return call.VerifyProviderSession.Check
	case "verify-expansion":
		return call.VerifyExpansion.Key
	case "store-adapter-session":
		return call.StoreAdapterSession.Record
	case "resolve-context":
		return call.ResolveContext.Query
	case "verify-epoch":
		return call.VerifyEpoch.Check
	case "install-expansion":
		return call.InstallExpansion.Install
	case "resolve-receipt-inputs":
		return call.ResolveReceiptInputs.Query
	case "append-receipt":
		return call.AppendReceipt.Append
	case "persist-handback":
		return call.PersistHandback.Record
	case "persist-quarantine":
		return call.PersistQuarantine.Record
	case "persist-abort":
		return call.PersistAbort.Record
	default:
		t.Fatalf("missing typed request projection for literal operation %q", operation)
		return nil
	}
}

type controllerMemoryTransport struct {
	read    io.Reader
	written bytes.Buffer
}

func (t *controllerMemoryTransport) Read(p []byte) (int, error)  { return t.read.Read(p) }
func (t *controllerMemoryTransport) Write(p []byte) (int, error) { return t.written.Write(p) }
func (t *controllerMemoryTransport) Close() error                { return nil }

type nonClosableControllerTransport struct {
	read    io.Reader
	written bytes.Buffer
}

func (t *nonClosableControllerTransport) Read(p []byte) (int, error)  { return t.read.Read(p) }
func (t *nonClosableControllerTransport) Write(p []byte) (int, error) { return t.written.Write(p) }

type blockingControllerTransport struct {
	readStarted  chan struct{}
	readReturned chan struct{}
	unblock      chan struct{}
	startOnce    sync.Once
	returnOnce   sync.Once
	releaseOnce  sync.Once
	writes       atomic.Uint32
	closes       atomic.Uint32
}

func newBlockingControllerTransport() *blockingControllerTransport {
	return &blockingControllerTransport{
		readStarted:  make(chan struct{}),
		readReturned: make(chan struct{}),
		unblock:      make(chan struct{}),
	}
}

func (t *blockingControllerTransport) Read([]byte) (int, error) {
	t.startOnce.Do(func() { close(t.readStarted) })
	<-t.unblock
	t.returnOnce.Do(func() { close(t.readReturned) })
	return 0, io.ErrClosedPipe
}

func (t *blockingControllerTransport) Write(p []byte) (int, error) {
	t.writes.Add(1)
	return len(p), nil
}

func (t *blockingControllerTransport) Close() error {
	t.closes.Add(1)
	t.release()
	return nil
}

func (t *blockingControllerTransport) release() {
	t.releaseOnce.Do(func() { close(t.unblock) })
}

func awaitControllerSignal(t *testing.T, signal <-chan struct{}, timeoutMessage string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(timeoutMessage)
	}
}

type partialControllerTransport struct{ wrote bool }

func (t *partialControllerTransport) Read([]byte) (int, error) { return 0, io.EOF }
func (t *partialControllerTransport) Write(p []byte) (int, error) {
	if !t.wrote {
		t.wrote = true
		return len(p) / 2, io.ErrUnexpectedEOF
	}
	return 0, io.ErrClosedPipe
}
func (t *partialControllerTransport) Close() error { return nil }

type shortNilControllerTransport struct{ writes int }

func (t *shortNilControllerTransport) Read([]byte) (int, error) { return 0, io.EOF }
func (t *shortNilControllerTransport) Write(p []byte) (int, error) {
	t.writes++
	return len(p) / 2, nil
}
func (t *shortNilControllerTransport) Close() error { return nil }

func mustEncodeControllerCall(t *testing.T, call ControllerCall) []byte {
	t.Helper()
	encoded, err := EncodeControllerCall(call)
	if err != nil {
		t.Fatalf("EncodeControllerCall: %v", err)
	}
	return encoded
}

func mustEncodeControllerResult(t *testing.T, result ControllerResult) []byte {
	t.Helper()
	encoded, err := EncodeControllerResult(result)
	if err != nil {
		t.Fatalf("EncodeControllerResult: %v", err)
	}
	return encoded
}

func controllerCallFixture(t *testing.T, sequence uint64, operation ControllerOperation) ControllerCall {
	t.Helper()
	request := validExecutionRequest(t, ActionStart)
	event, ack := controllerEventFixture(t, request)
	receipt, receiptEvent, _ := controllerReceiptFixture(t, request)
	resolution := controllerContextResolutionFixture(t)
	call := ControllerCall{Schema: ControllerCallSchemaID, CallSequence: sequence, Operation: operation}
	switch operation {
	case ControllerOperationVerifyAuthority:
		call.VerifyAuthority = ControllerVerifyAuthorityRequest{Schema: controllerRequestSchema(operation), Request: request}
	case ControllerOperationResolveProfile:
		call.ResolveProfile = ControllerResolveProfileRequest{Schema: controllerRequestSchema(operation), Query: ProfileQuery{Ref: request.Profile, WorkspacePath: filepath.Join("/tmp", "verdi-controller-workspace"), Grants: request.Grants}}
	case ControllerOperationVerifyConflict:
		call.VerifyConflict = ControllerVerifyConflictRequest{Schema: controllerRequestSchema(operation), Report: request.AuthorityVerdict}
	case ControllerOperationResolveRecorder:
		call.ResolveRecorder = ControllerResolveRecorderRequest{Schema: controllerRequestSchema(operation), Ref: request.RecorderEndpoint}
	case ControllerOperationRecorderCheckpoint:
		call.RecorderCheckpoint = ControllerRecorderCheckpointRequest{Schema: controllerRequestSchema(operation), Key: executionKey(request)}
	case ControllerOperationRecorderAppend:
		call.RecorderAppend = ControllerRecorderAppendRequest{Schema: controllerRequestSchema(operation), Event: event}
	case ControllerOperationVerifyOpaqueBoundary:
		call.VerifyOpaqueBoundary = ControllerVerifyOpaqueBoundaryRequest{Schema: controllerRequestSchema(operation), Rows: []contextcompile.OpaqueEntry{}}
	case ControllerOperationVerifyProviderSession:
		call.VerifyProviderSession = ControllerVerifyProviderSessionRequest{Schema: controllerRequestSchema(operation), Check: ProviderSessionCheck{SessionRef: "provider-session", AdapterVersion: request.AdapterVersion, ProfileDigest: request.Profile.Digest, WorkspaceID: "workspace-1"}}
	case ControllerOperationVerifyExpansion:
		call.VerifyExpansion = ControllerVerifyExpansionRequest{Schema: controllerRequestSchema(operation), Key: executionKey(request)}
	case ControllerOperationStoreAdapterSession:
		call.StoreAdapterSession = ControllerStoreAdapterSessionRequest{Schema: controllerRequestSchema(operation), Record: SessionRecord{Key: executionKey(request), SessionRef: "provider-session", AdapterVersion: request.AdapterVersion, ProfileDigest: request.Profile.Digest, WorkspaceID: "workspace-1", LifecycleAck: ack}}
	case ControllerOperationNextStamp:
		call.NextStamp = ControllerNextStampRequest{Schema: controllerRequestSchema(operation)}
	case ControllerOperationResolveContext:
		call.ResolveContext = ControllerResolveContextRequest{Schema: controllerRequestSchema(operation), Query: ContextQuery{Key: executionKey(request), Ref: "spec/test#ac-1"}}
	case ControllerOperationVerifyEpoch:
		call.VerifyEpoch = ControllerVerifyEpochRequest{Schema: controllerRequestSchema(operation), Check: controllerEpochCheckFixture(t)}
	case ControllerOperationInstallExpansion:
		call.InstallExpansion = ControllerInstallExpansionRequest{Schema: controllerRequestSchema(operation), Install: ExpansionInstall{Key: executionKey(request), RequestID: "request-1", ParentRevision: 0, ParentManifestDigest: request.ManifestDigest, ChildRevision: 1, ChildManifestDigest: testDigest("child-manifest"), ExpansionDigest: testDigest("expansion"), ExpansionRoot: testDigest("expansion-root"), TerminalAck: ack}}
	case ControllerOperationResolveReceiptInputs:
		call.ResolveReceiptInputs = ControllerResolveReceiptInputsRequest{Schema: controllerRequestSchema(operation), Query: ReceiptInputsQuery{Request: request, WorkspaceID: "workspace-1", DispatchDigest: testDigest("dispatch"), TerminalRevision: 0, TerminalSourceSequence: 1, TerminalGlobalSequence: 1, EventChainRoot: receipt.EventChainRoot, ResultFactsDigest: testDigest("result-facts")}}
	case ControllerOperationAppendReceipt:
		call.AppendReceipt = ControllerAppendReceiptRequest{Schema: controllerRequestSchema(operation), Append: ReceiptAppend{Receipt: receipt, Event: receiptEvent}}
	case ControllerOperationPersistHandback:
		call.PersistHandback = ControllerPersistHandbackRequest{Schema: controllerRequestSchema(operation), Record: validHandbackRecord(t)}
	case ControllerOperationPersistQuarantine:
		call.PersistQuarantine = ControllerPersistQuarantineRequest{Schema: controllerRequestSchema(operation), Record: validQuarantineRecord(t, QuarantineExecutionIncomplete)}
	case ControllerOperationPersistAbort:
		quarantine := validQuarantineRecord(t, QuarantineTerminalDurabilityFailed)
		quarantine = mustCanonicalQuarantine(t, quarantine)
		call.PersistAbort = ControllerPersistAbortRequest{Schema: controllerRequestSchema(operation), Record: validAbortRecord(t, quarantine)}
	default:
		t.Fatalf("unknown fixture operation %q", operation)
	}
	_ = resolution
	return call
}

func controllerResultFixture(t *testing.T, sequence uint64, operation ControllerOperation) ControllerResult {
	t.Helper()
	request := validExecutionRequest(t, ActionStart)
	event, ack := controllerEventFixture(t, request)
	receipt, _, receiptAck := controllerReceiptFixture(t, request)
	verification := Verification{State: contextcompile.ResolutionProven, Failure: FailureNone, Witnesses: []string{}}
	revision := contextevent.Revision{Schema: contextevent.RevisionSchemaID, ManifestRevision: 0, ManifestDigest: request.ManifestDigest, FirstGlobalSequence: 1, TerminalGlobalSequence: 1, TerminalSourceSequence: 1, TerminalKind: contextevent.KindExecutionResult, EventRoot: testDigest("terminal-event")}
	root, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		t.Fatal(err)
	}
	result := ControllerResult{Schema: ControllerResultSchemaID, CallSequence: sequence, Operation: operation}
	switch operation {
	case ControllerOperationVerifyAuthority:
		result.VerifyAuthority = ControllerVerifyAuthorityResult{Schema: controllerResultSchema(operation), Facts: AuthorityFacts{Verification: verification, ManifestRevision: request.ManifestRevision, ManifestDigest: request.ManifestDigest, ProjectionDigest: request.ProjectionDigest, AuthorityDigest: request.AuthorityVerdict.Digest, AcceptedSpecCommit: request.InputCommit}}
	case ControllerOperationResolveProfile:
		result.ResolveProfile = ControllerResolveProfileResult{Schema: controllerResultSchema(operation), Material: ProfileMaterial{Ref: request.Profile, Name: "project", AbsoluteExecutable: "/usr/local/bin/codex", AbsoluteEnvRoot: "/tmp/verdi-env", AbsoluteCodexHome: "/tmp/verdi-env/codex", AdapterVersion: request.AdapterVersion, DecoderProfile: "codex-jsonl-v1"}}
	case ControllerOperationVerifyConflict:
		result.VerifyConflict = ControllerVerifyConflictResult{Schema: controllerResultSchema(operation), Facts: ConflictFacts{Verification: verification, Report: request.AuthorityVerdict}}
	case ControllerOperationResolveRecorder:
		result.ResolveRecorder = ControllerResolveRecorderResult{Schema: controllerResultSchema(operation), Facts: RecorderFacts{Verification: verification, Ref: request.RecorderEndpoint}}
	case ControllerOperationRecorderCheckpoint:
		result.RecorderCheckpoint = ControllerRecorderCheckpointResult{Schema: controllerResultSchema(operation), Checkpoint: RecorderCheckpoint{Verification: verification, Digest: testDigest("checkpoint"), Revisions: []contextevent.Revision{revision}, EventChainRoot: root, TerminalSourceSequence: 1, TerminalGlobalSequence: 1}}
	case ControllerOperationRecorderAppend:
		result.RecorderAppend = ControllerRecorderAppendResult{Schema: controllerResultSchema(operation), Ack: ack}
	case ControllerOperationVerifyOpaqueBoundary:
		result.VerifyOpaqueBoundary = ControllerVerifyOpaqueBoundaryResult{Schema: controllerResultSchema(operation), Facts: OpaqueBoundaryFacts{Verification: verification, Rows: []OpaqueIdentity{}}}
	case ControllerOperationVerifyProviderSession:
		result.VerifyProviderSession = ControllerVerifyProviderSessionResult{Schema: controllerResultSchema(operation), Facts: ProviderSessionFacts{Verification: verification, SessionRef: "provider-session", AdapterVersion: request.AdapterVersion, ProfileDigest: request.Profile.Digest, WorkspaceID: "workspace-1"}}
	case ControllerOperationVerifyExpansion:
		result.VerifyExpansion = ControllerVerifyExpansionResult{Schema: controllerResultSchema(operation), Facts: ExpansionFacts{Verification: verification, Root: testDigest("expansion-root")}}
	case ControllerOperationStoreAdapterSession:
		result.StoreAdapterSession = ControllerStoreAdapterSessionResult{Schema: controllerResultSchema(operation)}
	case ControllerOperationNextStamp:
		result.NextStamp = ControllerNextStampResult{Schema: controllerResultSchema(operation), Stamp: "2026-08-28T12:34:56.123456789Z"}
	case ControllerOperationResolveContext:
		result.ResolveContext = ControllerResolveContextResult{Schema: controllerResultSchema(operation), Resolution: controllerContextResolutionFixture(t)}
	case ControllerOperationVerifyEpoch:
		result.VerifyEpoch = ControllerVerifyEpochResult{Schema: controllerResultSchema(operation), Verification: verification}
	case ControllerOperationInstallExpansion:
		result.InstallExpansion = ControllerInstallExpansionResult{Schema: controllerResultSchema(operation)}
	case ControllerOperationResolveReceiptInputs:
		result.ResolveReceiptInputs = ControllerResolveReceiptInputsResult{Schema: controllerResultSchema(operation), Inputs: ReceiptInputs{Expansions: []contextreceipt.Expansion{}, Obligations: []contextreceipt.Obligation{}, Evidence: []contextreceipt.Evidence{}, ReviewInputs: []contextreceipt.ReviewInput{}, RunnerPrincipal: receipt.RunnerPrincipalResolution}}
	case ControllerOperationAppendReceipt:
		result.AppendReceipt = ControllerAppendReceiptResult{Schema: controllerResultSchema(operation), Ack: receiptAck}
	case ControllerOperationPersistHandback:
		record := mustCanonicalHandback(t, validHandbackRecord(t))
		result.PersistHandback = ControllerPersistHandbackResult{Schema: controllerResultSchema(operation), Ack: mustCanonicalControlAck(t, validControlAckForHandback(record))}
	case ControllerOperationPersistQuarantine:
		record := mustCanonicalQuarantine(t, validQuarantineRecord(t, QuarantineExecutionIncomplete))
		result.PersistQuarantine = ControllerPersistQuarantineResult{Schema: controllerResultSchema(operation), Ack: mustCanonicalControlAck(t, validControlAckForQuarantine(record))}
	case ControllerOperationPersistAbort:
		quarantine := mustCanonicalQuarantine(t, validQuarantineRecord(t, QuarantineTerminalDurabilityFailed))
		record := mustCanonicalAbort(t, validAbortRecord(t, quarantine))
		result.PersistAbort = ControllerPersistAbortResult{Schema: controllerResultSchema(operation), Ack: mustCanonicalControlAck(t, validControlAckForAbort(record))}
	default:
		t.Fatalf("unknown fixture operation %q", operation)
	}
	_ = event
	return result
}

func controllerEventFixture(t *testing.T, request ExecutionRequest) (contextevent.Event, contextevent.EventAck) {
	t.Helper()
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	workspace := WorkspaceFacts{WorkspaceID: workspaceID, CurrentCommit: request.InputCommit, CurrentTree: request.InputTree}
	schema, err := contextevent.PayloadSchema(contextevent.KindAdapterStop)
	if err != nil {
		t.Fatal(err)
	}
	event, err := buildEvent(request, workspace, 1, "", nil, "2026-08-28T12:34:56Z", contextevent.KindAdapterStop, &contextevent.AdapterStopPayload{Schema: schema, Adapter: request.Adapter, AdapterVersion: request.AdapterVersion, Session: request.Session, ExitCode: 0, ReasonCode: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	ack := contextevent.EventAck{Schema: contextevent.AckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch, Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind, SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: 1}
	if _, err := contextevent.EncodeEventAck(ack); err != nil {
		t.Fatal(err)
	}
	return event, ack
}

func controllerContextResolutionFixture(t *testing.T) ContextResolution {
	t.Helper()
	item, encoded, err := contextcompile.BuildDataItem(contextcompile.Candidate{ID: "path:README.md", Source: contextcompile.SourceHeadTree, Path: "README.md"}, contextcompile.IncludedRepositoryFile, []byte("repository data\n"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := contextcompile.DecodeDataItem(encoded)
	if err != nil || item.Digest != decoded.Digest {
		t.Fatalf("canonical data item: %v", err)
	}
	return ContextResolution{Verification: Verification{State: contextcompile.ResolutionProven, Witnesses: []string{}}, Ref: "spec/test#ac-1", Data: decoded}
}

func controllerEpochCheckFixture(t *testing.T) EpochCheck {
	t.Helper()
	request := validExecutionRequest(t, ActionStart)
	state := NewFlightState(FlightStateSnapshot{Request: request, ExpansionRoot: testDigest("expansion-root")})
	return EpochCheck{Snapshot: state.Snapshot(), Resolution: controllerContextResolutionFixture(t)}
}

func controllerReceiptFixture(t *testing.T, request ExecutionRequest) (contextreceipt.Receipt, contextevent.Event, contextevent.ReceiptEventAck) {
	t.Helper()
	workspaceID, err := request.ExecutionWorkspaceRequest.WorkspaceID()
	if err != nil {
		t.Fatal(err)
	}
	workspaceDigest, err := ExecutionWorkspaceRequestDigest(request.ExecutionWorkspaceRequest)
	if err != nil {
		t.Fatal(err)
	}
	principal := gp.PrincipalResolution{}
	principal.State = gp.ResolutionAuthenticated
	principal.Claim.TrustSource = "fixture"
	principal.Claim.Subject = "runner-1"
	principal.PrincipalID, err = gp.CanonicalPrincipalID(principal.Claim.TrustSource, principal.Claim.Subject)
	if err != nil {
		t.Fatal(err)
	}
	principal.Witnesses = []gp.Witness{{Code: "authenticated", SourceID: "fixture", EvidenceDigest: testDigest("principal")}}
	revision := contextevent.Revision{Schema: contextevent.RevisionSchemaID, ManifestRevision: request.ManifestRevision, ManifestDigest: request.ManifestDigest, FirstGlobalSequence: 1, TerminalGlobalSequence: 1, TerminalSourceSequence: 1, TerminalKind: contextevent.KindExecutionResult, EventRoot: testDigest("execution-result-event")}
	root, err := contextevent.EventChainRoot([]contextevent.Revision{revision})
	if err != nil {
		t.Fatal(err)
	}
	receipt := contextreceipt.Receipt{Schema: contextreceipt.SchemaID, Role: contextreceipt.RoleBuilder, Authority: contextreceipt.AuthorityAuthoritative, ManifestDigest: request.ManifestDigest, DispatchDigest: testDigest("dispatch"), ATCRunway: request.ATCRunway, ExecutionWorkspaceRequestDigest: workspaceDigest, ExecutionWorkspaceID: workspaceID, InputCommit: request.InputCommit, InputTree: request.InputTree, OutputCommit: testSHA2, OutputTree: testTree2, Clean: true, RevisionSegments: []contextevent.Revision{revision}, EventChainRoot: root, TerminalManifestRevision: request.ManifestRevision, TerminalSourceSequence: 1, TerminalGlobalSequence: 1, Expansions: []contextreceipt.Expansion{}, Obligations: []contextreceipt.Obligation{}, Evidence: []contextreceipt.Evidence{}, RunnerPrincipalResolution: principal, Adapter: request.Adapter, AdapterVersion: request.AdapterVersion, ReviewInputs: []contextreceipt.ReviewInput{}}
	receiptBytes, err := contextreceipt.EncodeReceipt(receipt)
	if err != nil {
		t.Fatalf("EncodeReceipt fixture: %v", err)
	}
	receipt, err = contextreceipt.DecodeReceipt(bytes.NewReader(receiptBytes))
	if err != nil {
		t.Fatal(err)
	}
	payloadSchema, err := contextevent.PayloadSchema(contextevent.KindReceipt)
	if err != nil {
		t.Fatal(err)
	}
	receiptJSON := bytes.TrimSuffix(receiptBytes, []byte("\n"))
	var receiptValue any
	if err := json.Unmarshal(receiptJSON, &receiptValue); err != nil {
		t.Fatal(err)
	}
	detailDigest, err := canonjson.Digest(receiptValue)
	if err != nil {
		t.Fatal(err)
	}
	detail := contextevent.Detail{Mode: contextevent.DetailInline, MediaType: contextevent.MediaTypeJSON, Digest: detailDigest, RedactionProfile: contextevent.RedactionProfileStandard, RedactedJSON: receiptJSON}
	workspace := WorkspaceFacts{WorkspaceID: workspaceID, CurrentCommit: receipt.OutputCommit, CurrentTree: receipt.OutputTree}
	event, err := buildEvent(request, workspace, 2, testDigest("execution-result-event"), nil, "2026-08-28T12:34:57Z", contextevent.KindReceipt, &contextevent.ReceiptPayload{Schema: payloadSchema, Role: contextreceipt.RoleBuilder, ReceiptDigest: receipt.Digest, Authority: contextreceipt.AuthorityAuthoritative, ExecutionEventChainRoot: receipt.EventChainRoot, Detail: detail})
	if err != nil {
		t.Fatalf("receipt event fixture: %v", err)
	}
	ack := contextevent.ReceiptEventAck{Schema: contextevent.ReceiptAckSchemaID, Flight: event.Flight, Lane: event.Lane, Epoch: event.Epoch, Session: event.Session, ManifestRevision: event.ManifestRevision, Kind: event.Kind, SourceSequence: event.SourceSequence, EventDigest: event.EventDigest, GlobalSequence: 2, ReceiptDigest: receipt.Digest}
	if _, err := contextevent.EncodeReceiptEventAck(ack); err != nil {
		t.Fatal(err)
	}
	return receipt, event, ack
}
