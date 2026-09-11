// Mapping producer for the public owner bridge (VATC F12 controller-owner
// bridge correction §3, implementation plan Task 2 Steps 1 and 5).
//
// Every expectation in this file is written independently of the mapping code
// it prosecutes: the public arm is derived from the private payload by
// replacing the one exact quoted schema literal, and the public call and reply
// envelopes are assembled as literal canonical JSON in sorted-key order. If
// DecodeOwnerCall or EncodeOwnerReply ever chose a projection instead of
// applying the ratified publication rule, these bytes would disagree.
package sealedexec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/contextowner"
)

// ownerRequestDigest is the cross-process identity binding defined by §3.1:
// SHA-256 over the exact standalone canonical private request-payload bytes,
// including their trailing LF. It is spelled out here rather than reused from
// the bridge so the producer cannot agree with a mistake in it.
func ownerRequestDigest(privateRequest []byte) string {
	sum := sha256.Sum256(privateRequest)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// ownerPrivateRequestBytes renders one fixture call's private request payload
// as the standalone canonical document that crosses the FD-3 boundary.
func ownerPrivateRequestBytes(t *testing.T, call ControllerCall) []byte {
	t.Helper()
	payload, err := encodeControllerCallPayload(call)
	if err != nil {
		t.Fatalf("encode private request payload for %s: %v", call.Operation, err)
	}
	return frameNested(payload)
}

// ownerPrivateResultBytes renders one fixture result's private result payload
// as the standalone canonical document the existing FD-3 codec expects back.
func ownerPrivateResultBytes(t *testing.T, result ControllerResult) []byte {
	t.Helper()
	payload, err := encodeControllerSuccessPayload(result)
	if err != nil {
		t.Fatalf("encode private result payload for %s: %v", result.Operation, err)
	}
	return frameNested(payload)
}

// ownerPublishedArm applies §3.3's publication rule by hand: replace only the
// top-level schema literal and retain every other byte. The single-occurrence
// assertion is part of the proof — a private literal that also appeared inside
// a nested document would make a blind replacement silently wrong.
func ownerPublishedArm(t *testing.T, private []byte, privateSchema, publicSchema string) []byte {
	t.Helper()
	from := []byte(`"schema":"` + privateSchema + `"`)
	to := []byte(`"schema":"` + publicSchema + `"`)
	if got := bytes.Count(private, from); got != 1 {
		t.Fatalf("private payload carries %d occurrences of %s, want exactly 1", got, from)
	}
	published := bytes.Replace(private, from, to, 1)
	return bytes.TrimSuffix(published, []byte("\n"))
}

// ownerPublicCallDocument assembles the exact canonical public call. Canonical
// JSON sorts object keys, so controller_request_digest, operation, request and
// schema is the one legal member order.
func ownerPublicCallDocument(operation ControllerOperation, digest string, arm []byte) []byte {
	return []byte(`{"controller_request_digest":"` + digest + `","operation":"` + string(operation) +
		`","request":` + string(arm) + `,"schema":"` + contextowner.CallSchemaID + `"}` + "\n")
}

// ownerPublicReplyDocument assembles the exact canonical public reply around a
// byte-for-byte copy of the call the owner answered.
func ownerPublicReplyDocument(call, resultArm []byte) []byte {
	nested := bytes.TrimSuffix(call, []byte("\n"))
	return []byte(`{"call":` + string(nested) + `,"result":` + string(resultArm) +
		`,"schema":"` + contextowner.ReplySchemaID + `"}` + "\n")
}

// ownerResultFixture is the private result the paired call fixture actually
// admits. controllerResultFixture answers the shape question; the one patch
// below answers the identity question, because verify-authority's accepted
// spec commit is bound to the request manifest rather than to its input
// commit.
func ownerResultFixture(t *testing.T, operation ControllerOperation) ControllerResult {
	t.Helper()
	result := controllerResultFixture(t, 1, operation)
	if operation == ControllerOperationVerifyAuthority {
		request := validExecutionRequest(t, ActionStart)
		result.VerifyAuthority.Facts.AcceptedSpecCommit = request.Manifest.AcceptedSpec.Commit
	}
	return result
}

// ownerReplyFor builds the public reply an owner would return for one call.
func ownerReplyFor(t *testing.T, operation ControllerOperation, callDocument, resultArm []byte) contextowner.Reply {
	t.Helper()
	document := ownerPublicReplyDocument(callDocument, resultArm)
	reply, err := contextowner.DecodeReply(bytes.NewReader(document))
	if err != nil {
		t.Fatalf("decode public reply for %s: %v\n%s", operation, err, document)
	}
	return reply
}

// TestContextOwnerBridgeSet pins the Global Constraint: the bridge covers
// exactly Verdi operations 1–22, and ATC-owned resolve-claim-mcp remains
// operation 23 in the controller registry without ever entering either bridge
// verb (§3.4, §3.1/§3.2).
//
// The bridged set is proven to be the controller registry minus that one name —
// an exclusion, not a second hand-written list. The difference is what makes a
// future registry change loud: a Verdi operation added to the controller enters
// the bridged set and fails the mapping producer, whereas a set derived by
// agreeing with the public registry would have quietly dropped it.
func TestContextOwnerBridgeSet(t *testing.T) {
	registry := ControllerOperations()
	if len(registry) != 23 {
		t.Fatalf("controller registry has %d operations, want 23", len(registry))
	}
	if got := registry[22]; got != ControllerOperationResolveClaimMCP {
		t.Fatalf("operation 23 = %q, want %q", got, ControllerOperationResolveClaimMCP)
	}

	want := make([]ControllerOperation, 0, len(registry)-1)
	for _, operation := range registry {
		if operation != ControllerOperationResolveClaimMCP {
			want = append(want, operation)
		}
	}

	bridged := BridgedOperations()
	if len(bridged) != 22 {
		t.Fatalf("bridge covers %d operations, want the closed 22", len(bridged))
	}
	if !reflect.DeepEqual(bridged, want) {
		t.Fatalf("bridged set\n got %v\nwant %v", bridged, want)
	}
	for _, operation := range bridged {
		if operation == ControllerOperationResolveClaimMCP {
			t.Fatal("ATC-owned resolve-claim-mcp entered the bridged set")
		}
	}

	// The one resolver both bridge verbs consult must reach the same closed
	// answer, in both directions.
	for _, operation := range bridged {
		if _, ok := OwnerBridgeOperation(string(operation)); !ok {
			t.Fatalf("OwnerBridgeOperation refused bridged operation %s", operation)
		}
	}
	if _, ok := OwnerBridgeOperation(string(ControllerOperationResolveClaimMCP)); ok {
		t.Fatal("OwnerBridgeOperation resolved the ATC-owned operation")
	}

	// The public wire is the other half of the constraint: operation 23 is
	// never published, so no owner can be asked to answer it.
	published := contextowner.Operations()
	if len(published) != 22 {
		t.Fatalf("public owner registry publishes %d operations, want 22", len(published))
	}
	for i, operation := range published {
		if string(operation) == string(ControllerOperationResolveClaimMCP) {
			t.Fatal("the public owner registry publishes ATC-owned resolve-claim-mcp")
		}
		if string(operation) != string(want[i]) {
			t.Fatalf("published[%d] = %q, want the bridged operation %q", i, operation, want[i])
		}
	}

	// The bridged set is a copy: a caller cannot rewrite the closed union.
	bridged[0] = "mutated"
	if BridgedOperations()[0] != want[0] {
		t.Fatal("BridgedOperations must return a copy of the closed set")
	}
}

// TestContextOwnerBridgeMapping is the 22-row producer §6.5 requires: for every
// Verdi-owned operation it starts from a valid private request payload, proves
// the exact public call and its request digest, constructs the matching public
// reply, and proves the exact private result bytes come back.
//
// Coverage is asserted against the bridged set rather than against the number
// of rows written here. Because that set is the controller registry minus only
// the ATC-owned exclusion, an operation added to the controller without a
// bridge row fails this test instead of being silently unmapped.
func TestContextOwnerBridgeMapping(t *testing.T) {
	operations := BridgedOperations()
	if len(operations) != 22 {
		t.Fatalf("bridge covers %d operations, want the closed 22", len(operations))
	}

	for _, operation := range operations {
		t.Run(string(operation), func(t *testing.T) {
			call := controllerCallFixture(t, 1, operation)
			privateRequest := ownerPrivateRequestBytes(t, call)
			digest := ownerRequestDigest(privateRequest)

			publicCall, err := DecodeOwnerCall(operation, privateRequest)
			if err != nil {
				t.Fatalf("DecodeOwnerCall(%s): %v", operation, err)
			}
			if publicCall.Schema != contextowner.CallSchemaID {
				t.Fatalf("call schema = %q, want %q", publicCall.Schema, contextowner.CallSchemaID)
			}
			if string(publicCall.Operation) != string(operation) {
				t.Fatalf("call operation = %q, want %q", publicCall.Operation, operation)
			}
			if publicCall.ControllerRequestDigest != digest {
				t.Fatalf("controller_request_digest = %q, want %q", publicCall.ControllerRequestDigest, digest)
			}

			requestArm := ownerPublishedArm(t, privateRequest,
				controllerRequestSchema(operation), contextowner.RequestSchema(contextowner.Operation(operation)))
			wantCall := ownerPublicCallDocument(operation, digest, requestArm)
			gotCall, err := contextowner.EncodeCall(publicCall)
			if err != nil {
				t.Fatalf("EncodeCall(%s): %v", operation, err)
			}
			if !bytes.Equal(gotCall, wantCall) {
				t.Fatalf("public call bytes\n got %s\nwant %s", gotCall, wantCall)
			}

			result := ownerResultFixture(t, operation)
			privateResult := ownerPrivateResultBytes(t, result)
			resultArm := ownerPublishedArm(t, privateResult,
				controllerResultSchema(operation), contextowner.ResultSchema(contextowner.Operation(operation)))

			reply := ownerReplyFor(t, operation, wantCall, resultArm)
			gotResult, err := EncodeOwnerReply(operation, reply)
			if err != nil {
				t.Fatalf("EncodeOwnerReply(%s): %v", operation, err)
			}
			if !bytes.Equal(gotResult, privateResult) {
				t.Fatalf("private result bytes\n got %s\nwant %s", gotResult, privateResult)
			}
			if !bytes.HasSuffix(gotResult, []byte("\n")) || bytes.HasSuffix(gotResult, []byte("\n\n")) {
				t.Fatalf("private result does not carry exactly one canonical LF: %q", gotResult)
			}
		})
	}
}

// TestContextOwnerBridgeResolveContextNonProven extends the mapping producer
// above with SI-194's specific arm: an honest owner's non-proven ref-absent
// answer — no data item — round-trips through the full public-document
// bridge into the exact private controller result, the same way
// TestContextOwnerBridgeMapping proves the proven arm. Before SI-194's
// contextowner fix, an owner could not construct this public reply at all
// without fabricating a data item (contextowner's own EncodeReply refused
// it); this proves the corrected public wire, routed through
// EncodeOwnerReply, produces private bytes the sealed controller client
// decodes back to the identical non-proven, data-free resolution.
func TestContextOwnerBridgeResolveContextNonProven(t *testing.T) {
	operation := ControllerOperationResolveContext
	call := controllerCallFixture(t, 1, operation)
	privateRequest := ownerPrivateRequestBytes(t, call)
	digest := ownerRequestDigest(privateRequest)

	publicCall, err := DecodeOwnerCall(operation, privateRequest)
	if err != nil {
		t.Fatalf("DecodeOwnerCall(%s): %v", operation, err)
	}
	requestArm := ownerPublishedArm(t, privateRequest,
		controllerRequestSchema(operation), contextowner.RequestSchema(contextowner.Operation(operation)))
	wantCall := ownerPublicCallDocument(operation, digest, requestArm)
	gotCall, err := contextowner.EncodeCall(publicCall)
	if err != nil {
		t.Fatalf("EncodeCall(%s): %v", operation, err)
	}
	if !bytes.Equal(gotCall, wantCall) {
		t.Fatalf("public call bytes\n got %s\nwant %s", gotCall, wantCall)
	}

	// The honest owner's answer: non-proven, ref-absent, no data item.
	result := ControllerResult{
		Schema: ControllerResultSchemaID, CallSequence: call.CallSequence, Operation: operation,
		ResolveContext: ControllerResolveContextResult{
			Schema: controllerResultSchema(operation),
			Resolution: ContextResolution{
				Verification: Verification{State: contextcompile.ResolutionUnproven, Failure: FailureUnavailable, Witnesses: []string{"ref-absent"}},
				Ref:          call.ResolveContext.Query.Ref,
			},
		},
	}
	privateResult := ownerPrivateResultBytes(t, result)
	if bytes.Contains(privateResult, []byte(`"data"`)) {
		t.Fatalf("fixture private result unexpectedly names a data member: %s", privateResult)
	}
	resultArm := ownerPublishedArm(t, privateResult,
		controllerResultSchema(operation), contextowner.ResultSchema(contextowner.Operation(operation)))

	reply := ownerReplyFor(t, operation, wantCall, resultArm)
	gotResult, err := EncodeOwnerReply(operation, reply)
	if err != nil {
		t.Fatalf("EncodeOwnerReply(%s): %v", operation, err)
	}
	if !bytes.Equal(gotResult, privateResult) {
		t.Fatalf("private result bytes\n got %s\nwant %s", gotResult, privateResult)
	}
	if bytes.Contains(gotResult, []byte(`"data"`)) {
		t.Fatalf("bridged non-proven private result names a data member: %s", gotResult)
	}

	decoded := ControllerResult{Operation: operation}
	if err := decodeControllerSuccessPayload(gotResult, &decoded); err != nil {
		t.Fatalf("decodeControllerSuccessPayload(bridged result): %v", err)
	}
	resolution := decoded.ResolveContext.Resolution
	if resolution.State != contextcompile.ResolutionUnproven || resolution.Data != (contextcompile.DataItem{}) ||
		!reflect.DeepEqual(resolution.Witnesses, []string{"ref-absent"}) || resolution.Ref != call.ResolveContext.Query.Ref {
		t.Fatalf("decoded controller result resolution = %#v", resolution)
	}
}

// TestContextOwnerBridgeDecodeRefusals proves decode refuses every malformed,
// noncanonical, misframed, or out-of-registry private request. Nothing here
// may be repaired into a public call: a bridge that quietly re-canonicalized
// its input would publish a digest for bytes the controller never sent.
func TestContextOwnerBridgeDecodeRefusals(t *testing.T) {
	operation := ControllerOperationResolveRecorder
	valid := ownerPrivateRequestBytes(t, controllerCallFixture(t, 1, operation))
	trimmed := bytes.TrimSuffix(valid, []byte("\n"))

	cases := []struct {
		name      string
		operation ControllerOperation
		request   []byte
	}{
		{"unknown operation", ControllerOperation("frobnicate"), valid},
		{"empty operation", ControllerOperation(""), valid},
		{"atc-owned operation 23", ControllerOperation("resolve-claim-mcp"), valid},
		// The strongest exclusion row: a private claim query that is exactly
		// what the ATC controller answers over FD-3, so nothing about the
		// payload is wrong and only the bridged set refuses it.
		{"atc-owned operation 23 with its own well-formed payload", ControllerOperationResolveClaimMCP,
			ownerPrivateRequestBytes(t, controllerCallFixture(t, 1, ControllerOperationResolveClaimMCP))},
		{"operation mismatched to payload", ControllerOperationVerifyExpansion, valid},
		{"nil request", operation, nil},
		{"empty request", operation, []byte{}},
		{"lone newline", operation, []byte("\n")},
		{"json null", operation, []byte("null\n")},
		{"missing trailing newline", operation, trimmed},
		{"two trailing newlines", operation, append(append([]byte(nil), valid...), '\n')},
		{"leading whitespace", operation, append([]byte(" "), valid...)},
		{"trailing data", operation, append(append([]byte(nil), trimmed...), []byte("{}\n")...)},
		{"unknown member", operation, []byte(`{"extra":1,` + strings.TrimPrefix(string(trimmed), "{") + "\n")},
		{"noncanonical member order", operation, []byte(`{"schema":"` + controllerRequestSchema(operation) + `","ref":{}}` + "\n")},
		{"public schema literal", operation, ownerPublishedArm(t, valid,
			controllerRequestSchema(operation), contextowner.RequestSchema(contextowner.Operation(operation)))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			call, err := DecodeOwnerCall(tc.operation, tc.request)
			if err == nil {
				t.Fatalf("DecodeOwnerCall accepted %s: %+v", tc.name, call)
			}
			if !errors.Is(err, ErrOwnerRequestRefused) {
				t.Fatalf("refusal for %s is not a request refusal: %v", tc.name, err)
			}
			if !reflect.DeepEqual(call, contextowner.Call{}) {
				t.Fatalf("DecodeOwnerCall returned a non-zero call with an error: %+v", call)
			}
			if strings.Contains(err.Error(), string(tc.request)) && len(tc.request) > 0 {
				t.Fatalf("diagnostic echoes the rejected payload: %v", err)
			}
		})
	}
}

// TestContextOwnerBridgeEncodeRefusals proves encode refuses a reply whose CLI
// operation, nested call, request digest, or result arm does not belong to the
// call being answered. The digest row is the load-bearing one: without the
// recomputation, a well-formed reply carrying another call's request would be
// encoded as if the owner had answered this one.
func TestContextOwnerBridgeEncodeRefusals(t *testing.T) {
	operation := ControllerOperationResolveRecorder
	call := controllerCallFixture(t, 1, operation)
	privateRequest := ownerPrivateRequestBytes(t, call)
	digest := ownerRequestDigest(privateRequest)
	requestArm := ownerPublishedArm(t, privateRequest,
		controllerRequestSchema(operation), contextowner.RequestSchema(contextowner.Operation(operation)))
	callDocument := ownerPublicCallDocument(operation, digest, requestArm)
	resultArm := ownerPublishedArm(t, ownerPrivateResultBytes(t, ownerResultFixture(t, operation)),
		controllerResultSchema(operation), contextowner.ResultSchema(contextowner.Operation(operation)))

	// A second, genuinely different call for the same operation: same shape,
	// different recorder endpoint, therefore a different request digest.
	otherCall := controllerCallFixture(t, 1, operation)
	otherCall.ResolveRecorder.Ref.ID = otherCall.ResolveRecorder.Ref.ID + "-other"
	otherRequest := ownerPrivateRequestBytes(t, otherCall)
	otherDigest := ownerRequestDigest(otherRequest)
	otherArm := ownerPublishedArm(t, otherRequest,
		controllerRequestSchema(operation), contextowner.RequestSchema(contextowner.Operation(operation)))

	t.Run("cli operation contradicts nested call", func(t *testing.T) {
		reply := ownerReplyFor(t, operation, callDocument, resultArm)
		if _, err := EncodeOwnerReply(ControllerOperationVerifyExpansion, reply); err == nil {
			t.Fatal("EncodeOwnerReply accepted a reply for another operation")
		}
	})
	t.Run("unknown cli operation", func(t *testing.T) {
		reply := ownerReplyFor(t, operation, callDocument, resultArm)
		if _, err := EncodeOwnerReply(ControllerOperation("frobnicate"), reply); err == nil {
			t.Fatal("EncodeOwnerReply accepted an unknown operation")
		}
	})
	t.Run("digest belongs to another call", func(t *testing.T) {
		// The nested request is this call's; only the digest is the other
		// call's, so nothing but recomputation can catch it.
		document := ownerPublicCallDocument(operation, otherDigest, requestArm)
		reply := ownerReplyFor(t, operation, document, resultArm)
		if _, err := EncodeOwnerReply(operation, reply); err == nil {
			t.Fatal("EncodeOwnerReply accepted a reply whose digest names another request")
		}
	})
	t.Run("request belongs to another call", func(t *testing.T) {
		// The mirror image: this call's digest with the other call's request.
		document := ownerPublicCallDocument(operation, digest, otherArm)
		reply := ownerReplyFor(t, operation, document, resultArm)
		if _, err := EncodeOwnerReply(operation, reply); err == nil {
			t.Fatal("EncodeOwnerReply accepted a reply whose request is another call's")
		}
	})
	t.Run("zero reply", func(t *testing.T) {
		if _, err := EncodeOwnerReply(operation, contextowner.Reply{}); err == nil {
			t.Fatal("EncodeOwnerReply accepted a zero reply")
		}
	})
	t.Run("result contradicts request", func(t *testing.T) {
		// A structurally perfect reply whose resolved recorder endpoint is not
		// the one asked about: the private request/result cross-match, not the
		// public codec, is the only thing that refuses it.
		contradicting := ownerResultFixture(t, operation)
		contradicting.ResolveRecorder.Facts.Ref.ID = contradicting.ResolveRecorder.Facts.Ref.ID + "-other"
		arm := ownerPublishedArm(t, ownerPrivateResultBytes(t, contradicting),
			controllerResultSchema(operation), contextowner.ResultSchema(contextowner.Operation(operation)))
		reply := ownerReplyFor(t, operation, callDocument, arm)
		if _, err := EncodeOwnerReply(operation, reply); err == nil {
			t.Fatal("EncodeOwnerReply accepted a result that contradicts its request")
		}
	})
}

// assertClosedATCRefusal proves one exclusion diagnostic names a class rather
// than a value: it carries the declared refusal class, is framed by the package
// seam, and echoes nothing the refused document carried.
func assertClosedATCRefusal(t *testing.T, err error, class error, echoed ...string) {
	t.Helper()
	if !errors.Is(err, class) {
		t.Fatalf("refusal is not %v: %v", class, err)
	}
	if !strings.HasPrefix(err.Error(), "sealedexec: ") {
		t.Fatalf("diagnostic is not framed by the package seam: %v", err)
	}
	for _, forbidden := range echoed {
		if forbidden != "" && strings.Contains(err.Error(), forbidden) {
			t.Fatalf("diagnostic echoes the refused document: %v", err)
		}
	}
}

// TestContextOwnerBridgeRefusesATCOperation prosecutes the Global Constraint
// from the outside: never a public call for ATC-owned resolve-claim-mcp, and
// never a private result from a public reply for it.
//
// The load-bearing rows are the ones no other guard can account for. A
// well-formed private claim query is exactly what the ATC controller answers
// over FD-3, so only the bridged set stops it becoming a public call; and a
// reply whose nested call also names operation 23 satisfies the
// operation-mismatch guard, so only the bridged set stops it becoming a private
// result.
func TestContextOwnerBridgeRefusesATCOperation(t *testing.T) {
	t.Run("no public call for a well-formed private claim query", func(t *testing.T) {
		privateRequest := ownerPrivateRequestBytes(t, controllerCallFixture(t, 1, ControllerOperationResolveClaimMCP))
		call, err := DecodeOwnerCall(ControllerOperationResolveClaimMCP, privateRequest)
		if err == nil {
			t.Fatalf("DecodeOwnerCall published the ATC-owned operation: %+v", call)
		}
		if !reflect.DeepEqual(call, contextowner.Call{}) {
			t.Fatalf("DecodeOwnerCall returned a non-zero call with an error: %+v", call)
		}
		assertClosedATCRefusal(t, err, ErrOwnerRequestRefused,
			string(privateRequest), string(ControllerOperationResolveClaimMCP))
	})

	// One entirely valid resolve-recorder call/reply pair, reused as the
	// carrier for the encode rows: everything about it is legal except the
	// operation the reply is offered under.
	carrier := ControllerOperationResolveRecorder
	carrierRequest := ownerPrivateRequestBytes(t, controllerCallFixture(t, 1, carrier))
	carrierArm := ownerPublishedArm(t, carrierRequest,
		controllerRequestSchema(carrier), contextowner.RequestSchema(contextowner.Operation(carrier)))
	carrierDocument := ownerPublicCallDocument(carrier, ownerRequestDigest(carrierRequest), carrierArm)
	carrierResultArm := ownerPublishedArm(t, ownerPrivateResultBytes(t, ownerResultFixture(t, carrier)),
		controllerResultSchema(carrier), contextowner.ResultSchema(contextowner.Operation(carrier)))

	encodeRows := []struct {
		name    string
		invoked ControllerOperation
		nested  contextowner.Operation
	}{
		{"reply invoked as the ATC operation", ControllerOperationResolveClaimMCP,
			contextowner.Operation(carrier)},
		{"reply naming the ATC operation throughout", ControllerOperationResolveClaimMCP,
			contextowner.Operation(ControllerOperationResolveClaimMCP)},
		{"bridged reply carrying an ATC nested call", carrier,
			contextowner.Operation(ControllerOperationResolveClaimMCP)},
	}
	for _, row := range encodeRows {
		t.Run(row.name, func(t *testing.T) {
			reply := ownerReplyFor(t, carrier, carrierDocument, carrierResultArm)
			reply.Call.Operation = row.nested
			encoded, err := EncodeOwnerReply(row.invoked, reply)
			if err == nil {
				t.Fatalf("EncodeOwnerReply encoded a private result for %s: %s", row.name, encoded)
			}
			if encoded != nil {
				t.Fatalf("EncodeOwnerReply returned bytes with an error: %s", encoded)
			}
			assertClosedATCRefusal(t, err, ErrOwnerReplyRefused,
				string(ControllerOperationResolveClaimMCP), string(carrierRequest))
		})
	}
}

// TestContextOwnerBridgeCrossMatch prosecutes every private request/result
// identity relation the controller contract owns, one adverse row per relation.
//
// The six operations named in unrelated below carry no request/result relation
// at all in the accepted contract; they are listed so that adding a relation
// upstream without a bridge row here fails the completeness assertion rather
// than passing unnoticed. Completeness is asserted over the bridged set:
// ATC-owned operation 23 belongs to neither classification, and the relation
// table must refuse it outright instead of reading as unrelated.
func TestContextOwnerBridgeCrossMatch(t *testing.T) {
	unrelated := map[ControllerOperation]bool{
		ControllerOperationVerifyExpansion:      true,
		ControllerOperationStoreAdapterSession:  true,
		ControllerOperationNextStamp:            true,
		ControllerOperationVerifyEpoch:          true,
		ControllerOperationInstallExpansion:     true,
		ControllerOperationResolveReceiptInputs: true,
	}

	// Each corruption breaks exactly one relation while leaving the result a
	// valid document in its own right.
	corrupt := map[ControllerOperation]func(*ControllerResult){
		ControllerOperationVerifyAuthority: func(r *ControllerResult) {
			r.VerifyAuthority.Facts.ManifestDigest = testDigest("other-manifest")
		},
		ControllerOperationResolveProfile: func(r *ControllerResult) {
			r.ResolveProfile.Material.Ref.ID += "-other"
		},
		ControllerOperationVerifyConflict: func(r *ControllerResult) {
			r.VerifyConflict.Facts.Report.Input.ConstitutionDigest = testDigest("other-constitution")
		},
		ControllerOperationResolveRecorder: func(r *ControllerResult) {
			r.ResolveRecorder.Facts.Ref.ID += "-other"
		},
		ControllerOperationRecorderCheckpoint: func(r *ControllerResult) {
			// A structurally valid in-progress revision whose one durable
			// acknowledgment names another flight: only the request/result
			// relation, not the checkpoint's own rules, refuses it.
			ack := controllerActiveAck(1, 1, 2, testDigest("active-event"))
			ack.Flight = "other-flight"
			r.RecorderCheckpoint.Checkpoint.ActiveRevision = &ActiveRevision{
				Revision: 1, ManifestDigest: testDigest("active-manifest"),
				NextSourceSequence: 2, PriorEventDigest: testDigest("active-event"),
				LastGlobalSequence: 2, EventAcks: []contextevent.EventAck{ack},
			}
		},
		ControllerOperationRecorderAppend: func(r *ControllerResult) {
			r.RecorderAppend.Ack.EventDigest = testDigest("other-event")
		},
		ControllerOperationStoreRedactedSegment: func(r *ControllerResult) {
			r.StoreRedactedSegment.Stored.ByteCount++
		},
		ControllerOperationResolveRedactedSegment: func(r *ControllerResult) {
			segment, _ := controllerSegmentFixtures()
			segment.Bytes = []byte(`{"answer":43}`)
			segment.Digest = rawDigest(segment.Bytes)
			r.ResolveRedactedSegment.Segment = segment
		},
		ControllerOperationVerifyOpaqueBoundary: func(r *ControllerResult) {
			r.VerifyOpaqueBoundary.Facts.Rows = []OpaqueIdentity{{
				ID: "opaque-1", Kind: "session", AdapterID: "claude", AdapterVersion: "1",
			}}
		},
		ControllerOperationVerifyProviderSession: func(r *ControllerResult) {
			r.VerifyProviderSession.Facts.WorkspaceID = "workspace-other"
		},
		ControllerOperationResolveContext: func(r *ControllerResult) {
			r.ResolveContext.Resolution.Ref = "spec/test#ac-2"
		},
		ControllerOperationAppendReceipt: func(r *ControllerResult) {
			r.AppendReceipt.Ack.ReceiptDigest = testDigest("other-receipt")
		},
		ControllerOperationResolveReceiptVerificationAuthority: func(r *ControllerResult) {
			r.ResolveReceiptVerificationAuthority.Authority.TrustFact.SourceID = "other-source"
		},
		// Each control row swaps in an acknowledgment that is perfectly valid
		// in its own right but binds a different durable record. An ack whose
		// self-digest was simply corrupted would be refused by the control
		// codec long before the relation was consulted, and would witness
		// nothing about the relation.
		ControllerOperationPersistHandback: func(r *ControllerResult) {
			other := validHandbackRecord(t)
			other.WorkspaceID = "workspace-2"
			r.PersistHandback.Ack = mustCanonicalControlAck(t,
				validControlAckForHandback(mustCanonicalHandback(t, other)))
		},
		ControllerOperationPersistQuarantine: func(r *ControllerResult) {
			other := mustCanonicalQuarantine(t, validQuarantineRecord(t, QuarantineNonAuthoritative))
			r.PersistQuarantine.Ack = mustCanonicalControlAck(t, validControlAckForQuarantine(other))
		},
		ControllerOperationPersistAbort: func(r *ControllerResult) {
			otherQuarantine := mustCanonicalQuarantine(t, validQuarantineRecord(t, QuarantineNonAuthoritative))
			other := mustCanonicalAbort(t, validAbortRecord(t, otherQuarantine))
			r.PersistAbort.Ack = mustCanonicalControlAck(t, validControlAckForAbort(other))
		},
	}

	for _, operation := range BridgedOperations() {
		_, related := corrupt[operation]
		if related == unrelated[operation] {
			t.Fatalf("operation %s is both related and unrelated, or neither", operation)
		}
	}

	// Operation 23 is ATC-owned, so it is outside both classifications. The
	// relation table must refuse it before classification: falling through the
	// exhaustive switch would silently read it as an operation that carries no
	// request/result relation, which is a bridge answer for a call the bridge
	// never covers.
	if _, related := corrupt[ControllerOperationResolveClaimMCP]; related {
		t.Fatalf("ATC-owned %s is classified as a related bridge operation", ControllerOperationResolveClaimMCP)
	}
	if unrelated[ControllerOperationResolveClaimMCP] {
		t.Fatalf("ATC-owned %s is classified as an unrelated bridge operation", ControllerOperationResolveClaimMCP)
	}
	t.Run("atc-owned operation 23 is refused before classification", func(t *testing.T) {
		operation := ControllerOperationResolveClaimMCP
		if _, err := EncodeOwnerReply(operation, contextowner.Reply{}); !errors.Is(err, ErrOwnerReplyRefused) || !strings.Contains(err.Error(), "operation is not published") {
			t.Fatalf("EncodeOwnerReply must refuse the ATC-owned operation: %v", err)
		}
	})

	for _, operation := range BridgedOperations() {
		mutate, ok := corrupt[operation]
		if !ok {
			continue
		}
		t.Run(string(operation), func(t *testing.T) {
			call := controllerCallFixture(t, 1, operation)
			privateRequest := ownerPrivateRequestBytes(t, call)
			digest := ownerRequestDigest(privateRequest)
			requestArm := ownerPublishedArm(t, privateRequest,
				controllerRequestSchema(operation), contextowner.RequestSchema(contextowner.Operation(operation)))
			callDocument := ownerPublicCallDocument(operation, digest, requestArm)

			result := ownerResultFixture(t, operation)
			mutate(&result)
			arm := ownerPublishedArm(t, ownerPrivateResultBytes(t, result),
				controllerResultSchema(operation), contextowner.ResultSchema(contextowner.Operation(operation)))

			reply := ownerReplyFor(t, operation, callDocument, arm)
			if _, err := EncodeOwnerReply(operation, reply); err == nil {
				t.Fatalf("EncodeOwnerReply(%s) accepted a result that contradicts its request", operation)
			}
		})
	}
}

// TestContextOwnerBridgeDiagnosticsAreClosed proves no bridge diagnostic
// carries input bytes, a secret-looking value, or an absolute path. The bridge
// runs where a rejected controller payload may contain anything the caller
// sent; its refusal must name a class, never a value.
func TestContextOwnerBridgeDiagnosticsAreClosed(t *testing.T) {
	const marker = "s3cr3t-payload-marker"
	poisoned := []byte(`{"ref":{"digest":"` + marker + `","id":"/private/var/secret","schema":"x"},"schema":"` +
		controllerRequestSchema(ControllerOperationResolveRecorder) + `"}` + "\n")

	_, err := DecodeOwnerCall(ControllerOperationResolveRecorder, poisoned)
	if err == nil {
		t.Fatal("DecodeOwnerCall accepted a payload with an invalid digest")
	}
	for _, forbidden := range []string{marker, "/private/var/secret"} {
		if strings.Contains(err.Error(), forbidden) {
			t.Fatalf("diagnostic leaked %q: %v", forbidden, err)
		}
	}
	if !strings.HasPrefix(err.Error(), "sealedexec: ") {
		t.Fatalf("diagnostic is not framed by the package seam: %v", err)
	}
}

// TestContextOwnerBridgeIsPure proves the bridge is a pure function of its
// operands: the same private request maps to the same public call every time,
// and DecodeOwnerCall never retains or mutates the caller's slice.
func TestContextOwnerBridgeIsPure(t *testing.T) {
	operation := ControllerOperationNextStamp
	privateRequest := ownerPrivateRequestBytes(t, controllerCallFixture(t, 1, operation))
	original := append([]byte(nil), privateRequest...)

	first, err := DecodeOwnerCall(operation, privateRequest)
	if err != nil {
		t.Fatalf("DecodeOwnerCall: %v", err)
	}
	second, err := DecodeOwnerCall(operation, privateRequest)
	if err != nil {
		t.Fatalf("DecodeOwnerCall (repeat): %v", err)
	}
	if fmt.Sprintf("%+v", first) != fmt.Sprintf("%+v", second) {
		t.Fatal("DecodeOwnerCall is not deterministic")
	}
	if !bytes.Equal(privateRequest, original) {
		t.Fatal("DecodeOwnerCall mutated the caller's request bytes")
	}
}

// TestContextOwnerBridgeInstallExpansionV2 is the private/public differential
// for Task 2A's one authority-added exception to §3.3 (SI-182): the
// install-expansion request advances to v2 on BOTH sides carrying the
// requested ref, the request purpose, and the canonical installed data item,
// while its result and all 21 other request arms stay at the publication base.
//
// The differential matters because the widened facts are what a restart
// replays. If the projection dropped one of them, or if the request digest
// bound the narrow bytes rather than the widened ones, an owner could
// acknowledge an install whose lineage no later process could reconstruct.
func TestContextOwnerBridgeInstallExpansionV2(t *testing.T) {
	const (
		privateV2 = "verdi.context-controller/install-expansion-request/v2"
		privateV1 = "verdi.context-controller/install-expansion-request/v1"
		publicV2  = "verdi.context-owner/install-expansion-request/v2"
		publicV1  = "verdi.context-owner/install-expansion-request/v1"
	)
	operation := ControllerOperationInstallExpansion
	public := contextowner.Operation(operation)
	privateRequest := ownerPrivateRequestBytes(t, controllerCallFixture(t, 1, operation))

	t.Run("both sides advance the request only", func(t *testing.T) {
		if got := controllerRequestSchema(operation); got != privateV2 {
			t.Fatalf("private install request schema = %q, want %q", got, privateV2)
		}
		if got := contextowner.RequestSchema(public); got != publicV2 {
			t.Fatalf("published install request schema = %q, want %q", got, publicV2)
		}
		if got := controllerResultSchema(operation); got != "verdi.context-controller/install-expansion-result/v1" {
			t.Fatalf("private install result schema = %q, want the publication base", got)
		}
		if got := contextowner.ResultSchema(public); got != "verdi.context-owner/install-expansion-result/v1" {
			t.Fatalf("published install result schema = %q, want the publication base", got)
		}
		// The 21 other bridged arms. Operation 23 is excluded because the public
		// wire publishes no arm for it at all: deriving one here would assert a
		// published schema for an operation no owner is ever asked to answer.
		for _, other := range BridgedOperations() {
			if other == operation {
				continue
			}
			wantPrivate := "verdi.context-controller/" + string(other) + "-request/v1"
			wantPublic := "verdi.context-owner/" + string(other) + "-request/v1"
			if got := controllerRequestSchema(other); got != wantPrivate {
				t.Fatalf("private request schema for %s = %q, want %q", other, got, wantPrivate)
			}
			if got := contextowner.RequestSchema(contextowner.Operation(other)); got != wantPublic {
				t.Fatalf("published request schema for %s = %q, want %q", other, got, wantPublic)
			}
		}
	})

	t.Run("the widened facts survive the projection", func(t *testing.T) {
		for _, member := range []string{`"ref":"spec/extra"`, `"purpose":"needed for implementation"`, `"data":{`} {
			if !bytes.Contains(privateRequest, []byte(member)) {
				t.Fatalf("private install payload lacks %s: %s", member, privateRequest)
			}
		}
		call, err := DecodeOwnerCall(operation, privateRequest)
		if err != nil {
			t.Fatalf("DecodeOwnerCall: %v", err)
		}
		arm := ownerPublishedArm(t, privateRequest, privateV2, publicV2)
		want := ownerPublicCallDocument(operation, ownerRequestDigest(privateRequest), arm)
		got, err := contextowner.EncodeCall(call)
		if err != nil {
			t.Fatalf("EncodeCall: %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("published install call\n got %s\nwant %s", got, want)
		}

		// The reply round trip still returns the exact v1 private result.
		result := ownerResultFixture(t, operation)
		privateResult := ownerPrivateResultBytes(t, result)
		resultArm := ownerPublishedArm(t, privateResult,
			controllerResultSchema(operation), contextowner.ResultSchema(public))
		reply := ownerReplyFor(t, operation, want, resultArm)
		encoded, err := EncodeOwnerReply(operation, reply)
		if err != nil {
			t.Fatalf("EncodeOwnerReply: %v", err)
		}
		if !bytes.Equal(encoded, privateResult) {
			t.Fatalf("private install result\n got %s\nwant %s", encoded, privateResult)
		}
	})

	t.Run("the v1 install request cannot be served", func(t *testing.T) {
		legacy := bytes.Replace(privateRequest, []byte(`"`+privateV2+`"`), []byte(`"`+privateV1+`"`), 1)
		if bytes.Equal(legacy, privateRequest) {
			t.Fatal("the private install payload does not declare the v2 request schema")
		}
		if _, err := DecodeOwnerCall(operation, legacy); err == nil {
			t.Fatal("DecodeOwnerCall published a migration-only v1 install request")
		}
	})

	t.Run("the request digest binds the widened facts", func(t *testing.T) {
		arm := ownerPublishedArm(t, privateRequest, privateV2, publicV2)
		callDocument := ownerPublicCallDocument(operation, ownerRequestDigest(privateRequest), arm)
		result := ownerResultFixture(t, operation)
		resultArm := ownerPublishedArm(t, ownerPrivateResultBytes(t, result),
			controllerResultSchema(operation), contextowner.ResultSchema(public))
		document := ownerPublicReplyDocument(callDocument, resultArm)

		for _, mutation := range []struct{ name, from, to string }{
			{"rewritten ref", `"ref":"spec/extra"`, `"ref":"spec/other"`},
			{"rewritten purpose", `"purpose":"needed for implementation"`, `"purpose":"rewritten purpose"`},
		} {
			t.Run(mutation.name, func(t *testing.T) {
				rewritten := bytes.Replace(document, []byte(mutation.from), []byte(mutation.to), 1)
				if bytes.Equal(rewritten, document) {
					t.Fatalf("reply does not carry %s", mutation.from)
				}
				reply, err := contextowner.DecodeReply(bytes.NewReader(rewritten))
				if err != nil {
					// A rewritten fact the public codec already refuses is an
					// equally closed outcome; nothing reached the bridge.
					return
				}
				if _, err := EncodeOwnerReply(operation, reply); err == nil {
					t.Fatal("EncodeOwnerReply accepted a reply whose install facts were rewritten")
				}
			})
		}
	})
}
