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
	"fmt"
	"reflect"
	"strings"
	"testing"

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

// TestContextOwnerBridgeMapping is the 22-row producer §6.5 requires: for every
// Verdi-owned operation it starts from a valid private request payload, proves
// the exact public call and its request digest, constructs the matching public
// reply, and proves the exact private result bytes come back.
//
// Coverage is asserted against the closed registry rather than against the
// number of rows written here, so an operation added to the controller without
// a bridge row fails this test instead of being silently unmapped.
func TestContextOwnerBridgeMapping(t *testing.T) {
	operations := ControllerOperations()
	if len(operations) != 22 {
		t.Fatalf("controller registry has %d operations, want the closed 22", len(operations))
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

// TestContextOwnerBridgeCrossMatch prosecutes every private request/result
// identity relation the controller contract owns, one adverse row per relation.
//
// The six operations named in unrelated below carry no request/result relation
// at all in the accepted contract; they are listed so that adding a relation
// upstream without a bridge row here fails the completeness assertion rather
// than passing unnoticed.
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

	for _, operation := range ControllerOperations() {
		_, related := corrupt[operation]
		if related == unrelated[operation] {
			t.Fatalf("operation %s is both related and unrelated, or neither", operation)
		}
	}

	for _, operation := range ControllerOperations() {
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
// for Task 2A's one authority-added exception to §3.3 (SI-177): the
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
		for _, other := range ControllerOperations() {
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
