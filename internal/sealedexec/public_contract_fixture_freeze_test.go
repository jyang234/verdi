// TestFreezePublicContractFixtures is Task 1 Lane A's one-shot, env-gated
// generator (VATC F12 controller-owner bridge correction §3, sibling
// invention-ledger row SI-181). It runs exactly once, at Verdi commit
// 4502917a8ec5cb4f34f125d9ba71ed493065b8aa (runtime-identical to the pinned
// baseline 8ab423fefa14f6cee3070ca96754928daf28c062), before any v2 codec
// exists in this repository, and freezes
// internal/contextowner/testdata/public-contract/ as a literal,
// non-self-referential byte oracle for later FD-3 v2 transport work.
//
// Every arm, wrapper, legacy payload, and v1 frame it writes is the exact
// output of the baseline encoders (contextowner.EncodeCall/EncodeReply,
// sealedexec.EncodeControllerCall/EncodeControllerResult, and the sealedexec
// owner bridge) -- never hand-assembled. Only the v2 outer envelopes, which
// no encoder in this build can produce yet, are literal string compositions,
// built by hand around those same baseline bytes.
//
// This file is meant to be removed from the tree immediately after its one
// commit: once a v2 codec exists, nothing may regenerate the oracle it is
// judged against. It refuses to run a second time once its target directory
// already carries a manifest.
package sealedexec

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextowner"
)

// publicContractFixtureDir is internal/contextowner/testdata/public-contract,
// spelled relative to this package's directory because `go test` runs with
// the package source directory as its working directory.
const publicContractFixtureDir = "../contextowner/testdata/public-contract"

const publicContractProvenance = `Generated at Verdi commit 4502917a8ec5cb4f34f125d9ba71ed493065b8aa
(runtime-identical to pinned baseline 8ab423fefa14f6cee3070ca96754928daf28c062)
by the one-shot env-gated generator test TestFreezePublicContractFixtures, run
BEFORE any v2 codec change existed in this repository.

Arms, wrappers, legacy payloads, and v1 frames in this directory are the exact
output of the baseline encoders: contextowner.EncodeCall, contextowner.EncodeReply,
sealedexec.EncodeControllerCall, sealedexec.EncodeControllerResult, and the
sealedexec owner bridge (DecodeOwnerCall/EncodeOwnerReply). The v2 call and
result frames, and the error v2 frame, are literal string compositions
assembled by hand from those same baseline bytes -- never produced by any
encoder.

These bytes are the oracle for the v2 transport. They must never be
regenerated with a later encoder, and this generator refuses to run again
once this directory already carries a manifest. Task 2 copies these exact
bytes into ATC testdata.
`

func TestFreezePublicContractFixtures(t *testing.T) {
	if os.Getenv("VERDI_FREEZE_PUBLIC_CONTRACT_FIXTURES") != "1" {
		t.Skip("set VERDI_FREEZE_PUBLIC_CONTRACT_FIXTURES=1 to regenerate the frozen public contract fixtures")
	}

	manifestPath := filepath.Join(publicContractFixtureDir, "SHA256SUMS")
	if _, err := os.Stat(manifestPath); err == nil {
		t.Fatalf("refusing to regenerate: %s already exists", manifestPath)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat %s: %v", manifestPath, err)
	}
	if err := os.MkdirAll(publicContractFixtureDir, 0o755); err != nil {
		t.Fatalf("create %s: %v", publicContractFixtureDir, err)
	}

	files := map[string][]byte{}
	add := func(name string, data []byte) {
		if _, exists := files[name]; exists {
			t.Fatalf("duplicate fixture file %s", name)
		}
		files[name] = data
	}

	type artifacts struct {
		call          ControllerCall
		ownerCallDoc  []byte
		resultArmNoLF []byte
	}
	perOp := make(map[ControllerOperation]artifacts)
	digests := map[string]string{}

	bridged := BridgedOperations()
	if len(bridged) != 22 {
		t.Fatalf("BridgedOperations() returned %d operations, want 22", len(bridged))
	}

	for _, operation := range bridged {
		opName := string(operation)
		publicOp := contextowner.Operation(operation)

		call := controllerCallFixture(t, 1, operation)
		privateRequest := ownerPrivateRequestBytes(t, call)
		add(opName+".legacy-request.json", privateRequest)

		digest := ownerRequestDigest(privateRequest)
		digests[opName] = digest

		requestArmNoLF := ownerPublishedArm(t, privateRequest, controllerRequestSchema(operation), contextowner.RequestSchema(publicOp))
		add(opName+".request.json", withTrailingLF(requestArmNoLF))

		publicCall, err := DecodeOwnerCall(operation, privateRequest)
		if err != nil {
			t.Fatalf("DecodeOwnerCall(%s): %v", operation, err)
		}
		gotCall, err := contextowner.EncodeCall(publicCall)
		if err != nil {
			t.Fatalf("EncodeCall(%s): %v", operation, err)
		}
		wantCallDoc := ownerPublicCallDocument(operation, digest, requestArmNoLF)
		if !bytes.Equal(gotCall, wantCallDoc) {
			t.Fatalf("public call bytes for %s\n got  %s\n want %s", operation, gotCall, wantCallDoc)
		}
		add(opName+".owner-call.json", gotCall)

		result := ownerResultFixture(t, operation)
		privateResult := ownerPrivateResultBytes(t, result)
		add(opName+".legacy-result.json", privateResult)

		resultArmNoLF := ownerPublishedArm(t, privateResult, controllerResultSchema(operation), contextowner.ResultSchema(publicOp))
		add(opName+".result.json", withTrailingLF(resultArmNoLF))

		replyDoc := ownerPublicReplyDocument(gotCall, resultArmNoLF)
		reply, err := contextowner.DecodeReply(bytes.NewReader(replyDoc))
		if err != nil {
			t.Fatalf("decode public reply for %s: %v\n%s", operation, err, replyDoc)
		}
		if reencoded, err := contextowner.EncodeReply(reply); err != nil || !bytes.Equal(reencoded, replyDoc) {
			t.Fatalf("owner reply for %s does not re-encode byte-identically (err=%v)", operation, err)
		}
		gotPrivateResult, err := EncodeOwnerReply(operation, reply)
		if err != nil {
			t.Fatalf("EncodeOwnerReply(%s): %v", operation, err)
		}
		if !bytes.Equal(gotPrivateResult, privateResult) {
			t.Fatalf("EncodeOwnerReply(%s) mismatch\n got  %s\n want %s", operation, gotPrivateResult, privateResult)
		}
		add(opName+".owner-reply.json", replyDoc)

		callV1, err := EncodeControllerCall(call)
		if err != nil {
			t.Fatalf("EncodeControllerCall(%s): %v", operation, err)
		}
		requireControllerCallRoundTrip(t, opName, callV1)
		add(opName+".call.v1.json", callV1)

		resultV1, err := EncodeControllerResult(result)
		if err != nil {
			t.Fatalf("EncodeControllerResult(%s): %v", operation, err)
		}
		requireControllerResultRoundTrip(t, opName, resultV1)
		add(opName+".result.v1.json", resultV1)

		add(opName+".call.v2.json", literalControllerCallV2(opName, requestArmNoLF))
		add(opName+".result.v2.json", literalControllerResultV2(opName, resultArmNoLF))

		perOp[operation] = artifacts{call: call, ownerCallDoc: gotCall, resultArmNoLF: resultArmNoLF}
	}

	// Operation 23: ATC-owned resolve-claim-mcp. No public arm exists for it;
	// its controller-prefixed wrappers stay exactly as payloads, and only the
	// outer envelope advances to v2.
	claimOp := ControllerOperationResolveClaimMCP
	claimCall := controllerCallFixture(t, 1, claimOp)
	claimPrivateRequest := ownerPrivateRequestBytes(t, claimCall)
	add("resolve-claim-mcp.legacy-request.json", claimPrivateRequest)

	claimResult := controllerResultFixture(t, 1, claimOp)
	claimPrivateResult := ownerPrivateResultBytes(t, claimResult)
	add("resolve-claim-mcp.legacy-result.json", claimPrivateResult)

	claimCallV1, err := EncodeControllerCall(claimCall)
	if err != nil {
		t.Fatalf("EncodeControllerCall(resolve-claim-mcp): %v", err)
	}
	requireControllerCallRoundTrip(t, "resolve-claim-mcp", claimCallV1)
	add("resolve-claim-mcp.call.v1.json", claimCallV1)

	claimResultV1, err := EncodeControllerResult(claimResult)
	if err != nil {
		t.Fatalf("EncodeControllerResult(resolve-claim-mcp): %v", err)
	}
	requireControllerResultRoundTrip(t, "resolve-claim-mcp", claimResultV1)
	add("resolve-claim-mcp.result.v1.json", claimResultV1)

	claimRequestPayloadNoLF := bytes.TrimSuffix(claimPrivateRequest, []byte("\n"))
	add("resolve-claim-mcp.call.v2.json", literalControllerCallV2("resolve-claim-mcp", claimRequestPayloadNoLF))
	claimResultPayloadNoLF := bytes.TrimSuffix(claimPrivateResult, []byte("\n"))
	add("resolve-claim-mcp.result.v2.json", literalControllerResultV2("resolve-claim-mcp", claimResultPayloadNoLF))

	// Variant: resolve-context's honest non-proven, ref-absent, data-free
	// resolution (SI-194), the same fixture TestContextOwnerBridgeResolveContextNonProven
	// already proves round-trips through the full bridge.
	resolveContextArtifacts := perOp[ControllerOperationResolveContext]
	nonProvenRef := resolveContextArtifacts.call.ResolveContext.Query.Ref
	nonProvenResolution := ContextResolution{
		Verification: Verification{State: contextcompile.ResolutionUnproven, Failure: FailureUnavailable, Witnesses: []string{"ref-absent"}},
		Ref:          nonProvenRef,
	}
	nonProvenResult := ControllerResult{
		Schema: ControllerResultSchemaID, CallSequence: 1, Operation: ControllerOperationResolveContext,
		ResolveContext: ControllerResolveContextResult{
			Schema:     controllerResultSchema(ControllerOperationResolveContext),
			Resolution: nonProvenResolution,
		},
	}
	nonProvenPrivateResult := ownerPrivateResultBytes(t, nonProvenResult)
	if bytes.Contains(nonProvenPrivateResult, []byte(`"data"`)) {
		t.Fatalf("resolve-context nonproven legacy result unexpectedly names a data member: %s", nonProvenPrivateResult)
	}
	add("resolve-context.nonproven.legacy-result.json", nonProvenPrivateResult)

	nonProvenResultArmNoLF := ownerPublishedArm(t, nonProvenPrivateResult,
		controllerResultSchema(ControllerOperationResolveContext), contextowner.ResultSchema(contextowner.OperationResolveContext))
	add("resolve-context.nonproven.result.json", withTrailingLF(nonProvenResultArmNoLF))

	nonProvenReplyDoc := ownerPublicReplyDocument(resolveContextArtifacts.ownerCallDoc, nonProvenResultArmNoLF)
	nonProvenReply, err := contextowner.DecodeReply(bytes.NewReader(nonProvenReplyDoc))
	if err != nil {
		t.Fatalf("decode resolve-context nonproven reply: %v", err)
	}
	if reencoded, err := contextowner.EncodeReply(nonProvenReply); err != nil || !bytes.Equal(reencoded, nonProvenReplyDoc) {
		t.Fatalf("resolve-context nonproven reply does not re-encode byte-identically (err=%v)", err)
	}
	gotNonProvenPrivateResult, err := EncodeOwnerReply(ControllerOperationResolveContext, nonProvenReply)
	if err != nil {
		t.Fatalf("EncodeOwnerReply(resolve-context nonproven): %v", err)
	}
	if !bytes.Equal(gotNonProvenPrivateResult, nonProvenPrivateResult) {
		t.Fatalf("EncodeOwnerReply(resolve-context nonproven) mismatch\n got  %s\n want %s", gotNonProvenPrivateResult, nonProvenPrivateResult)
	}
	if bytes.Contains(gotNonProvenPrivateResult, []byte(`"data"`)) {
		t.Fatalf("bridged resolve-context nonproven result unexpectedly names a data member: %s", gotNonProvenPrivateResult)
	}
	add("resolve-context.nonproven.owner-reply.json", nonProvenReplyDoc)

	nonProvenResultV1, err := EncodeControllerResult(nonProvenResult)
	if err != nil {
		t.Fatalf("EncodeControllerResult(resolve-context nonproven): %v", err)
	}
	requireControllerResultRoundTrip(t, "resolve-context.nonproven", nonProvenResultV1)
	add("resolve-context.nonproven.result.v1.json", nonProvenResultV1)

	add("resolve-context.nonproven.result.v2.json", literalControllerResultV2("resolve-context", nonProvenResultArmNoLF))

	// Variant: a verify-epoch request whose check.resolution is that same
	// non-proven, data-free resolution, snapshot otherwise unchanged from
	// controllerEpochCheckFixture. Attempted defensively: if the baseline
	// private codec or contextowner refuses it, the variant is omitted and
	// the exact refusal is logged as a finding rather than forced.
	verifyEpochArtifacts := perOp[ControllerOperationVerifyEpoch]
	variantCall := verifyEpochArtifacts.call
	variantCall.VerifyEpoch = ControllerVerifyEpochRequest{
		Schema: controllerRequestSchema(ControllerOperationVerifyEpoch),
		Check: EpochCheck{
			Snapshot:   verifyEpochArtifacts.call.VerifyEpoch.Check.Snapshot,
			Resolution: nonProvenResolution,
		},
	}
	privateRequest, publicVariantCall, ok, refusal := attemptVerifyEpochNonProvenResolutionVariant(t, variantCall)
	if !ok {
		t.Logf("FINDING: verify-epoch.nonproven-resolution variant omitted; the baseline refused it: %s", refusal)
	} else {
		add("verify-epoch.nonproven-resolution.legacy-request.json", privateRequest)
		variantDigest := ownerRequestDigest(privateRequest)
		digests["verify-epoch.nonproven-resolution"] = variantDigest

		variantRequestArmNoLF := ownerPublishedArm(t, privateRequest,
			controllerRequestSchema(ControllerOperationVerifyEpoch), contextowner.RequestSchema(contextowner.OperationVerifyEpoch))
		add("verify-epoch.nonproven-resolution.request.json", withTrailingLF(variantRequestArmNoLF))

		gotVariantCall, err := contextowner.EncodeCall(publicVariantCall)
		if err != nil {
			t.Fatalf("EncodeCall(verify-epoch nonproven-resolution variant): %v", err)
		}
		wantVariantCallDoc := ownerPublicCallDocument(ControllerOperationVerifyEpoch, variantDigest, variantRequestArmNoLF)
		if !bytes.Equal(gotVariantCall, wantVariantCallDoc) {
			t.Fatalf("verify-epoch nonproven-resolution public call bytes\n got  %s\n want %s", gotVariantCall, wantVariantCallDoc)
		}
		add("verify-epoch.nonproven-resolution.owner-call.json", gotVariantCall)

		variantReplyDoc := ownerPublicReplyDocument(gotVariantCall, verifyEpochArtifacts.resultArmNoLF)
		variantReply, err := contextowner.DecodeReply(bytes.NewReader(variantReplyDoc))
		if err != nil {
			t.Fatalf("decode verify-epoch nonproven-resolution reply: %v", err)
		}
		if reencoded, err := contextowner.EncodeReply(variantReply); err != nil || !bytes.Equal(reencoded, variantReplyDoc) {
			t.Fatalf("verify-epoch nonproven-resolution reply does not re-encode byte-identically (err=%v)", err)
		}
		add("verify-epoch.nonproven-resolution.owner-reply.json", variantReplyDoc)

		variantCallV1, err := EncodeControllerCall(variantCall)
		if err != nil {
			t.Fatalf("EncodeControllerCall(verify-epoch nonproven-resolution): %v", err)
		}
		requireControllerCallRoundTrip(t, "verify-epoch.nonproven-resolution", variantCallV1)
		add("verify-epoch.nonproven-resolution.call.v1.json", variantCallV1)

		add("verify-epoch.nonproven-resolution.call.v2.json", literalControllerCallV2("verify-epoch", variantRequestArmNoLF))
	}

	// Error frame: one fixed operational refusal.
	errorResult := ControllerResult{
		Schema: ControllerResultSchemaID, CallSequence: 1, Operation: ControllerOperationNextStamp,
		Error: &ControllerError{
			Schema: ControllerErrorSchemaID, Class: ControllerErrorClassOperational, Code: ControllerErrorUnavailable,
			Witnesses: []string{"fixture controller refusal"},
		},
	}
	errorResultV1, err := EncodeControllerResult(errorResult)
	if err != nil {
		t.Fatalf("EncodeControllerResult(error fixture): %v", err)
	}
	requireControllerResultRoundTrip(t, "error", errorResultV1)
	add("error.result.v1.json", errorResultV1)
	add("error.result.v2.json", []byte(`{"call_sequence":1,"operation":"next-stamp","payload":{"error":{"class":"operational","code":"unavailable","schema":"verdi.context-controller-error/v1","witnesses":["fixture controller refusal"]}},"schema":"verdi.context-controller-result/v2"}`+"\n"))

	// Inventory and provenance.
	digestsJSON, err := canonjson.Marshal(digests)
	if err != nil {
		t.Fatalf("marshal digests.json: %v", err)
	}
	add("digests.json", digestsJSON)
	add("provenance.txt", []byte(publicContractProvenance))

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)

	var sums bytes.Buffer
	for _, name := range names {
		sum := sha256.Sum256(files[name])
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}

	for _, name := range names {
		if err := os.WriteFile(filepath.Join(publicContractFixtureDir, name), files[name], 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := os.WriteFile(manifestPath, sums.Bytes(), 0o644); err != nil {
		t.Fatalf("write SHA256SUMS: %v", err)
	}

	t.Logf("froze %d fixture files plus SHA256SUMS under %s", len(names), publicContractFixtureDir)
}

// attemptVerifyEpochNonProvenResolutionVariant tries to build the
// verify-epoch request variant whose check.resolution is the same
// non-proven, data-free resolution SI-194 established for resolve-context.
// It reports ok=false with the exact refusal text only when the baseline
// private codec, DecodeOwnerCall, or contextowner.DecodeCall itself refuses
// the candidate; any other failure is a genuine test-infrastructure fault
// and is fatal immediately.
func attemptVerifyEpochNonProvenResolutionVariant(t *testing.T, variantCall ControllerCall) (privateRequest []byte, publicCall contextowner.Call, ok bool, refusal string) {
	t.Helper()
	payload, err := encodeControllerCallPayload(variantCall)
	if err != nil {
		return nil, contextowner.Call{}, false, fmt.Sprintf("encodeControllerCallPayload: %v", err)
	}
	privateRequest = frameNested(payload)
	publicCall, err = DecodeOwnerCall(ControllerOperationVerifyEpoch, privateRequest)
	if err != nil {
		return nil, contextowner.Call{}, false, fmt.Sprintf("DecodeOwnerCall: %v", err)
	}
	encoded, err := contextowner.EncodeCall(publicCall)
	if err != nil {
		t.Fatalf("EncodeCall(verify-epoch nonproven-resolution variant): %v", err)
	}
	if _, err := contextowner.DecodeCall(bytes.NewReader(encoded)); err != nil {
		return nil, contextowner.Call{}, false, fmt.Sprintf("contextowner.DecodeCall: %v", err)
	}
	return privateRequest, publicCall, true, ""
}

func withTrailingLF(data []byte) []byte {
	return append(append([]byte{}, data...), '\n')
}

// literalControllerCallV2 and literalControllerResultV2 are the exact v2
// outer-envelope composition rule: plain string assembly around one
// already-canonical inner payload, never a second encoder.
func literalControllerCallV2(operation string, payloadNoLF []byte) []byte {
	return []byte(`{"call_sequence":1,"operation":"` + operation + `","payload":` + string(payloadNoLF) +
		`,"schema":"verdi.context-controller-call/v2"}` + "\n")
}

func literalControllerResultV2(operation string, resultMemberNoLF []byte) []byte {
	return []byte(`{"call_sequence":1,"operation":"` + operation + `","payload":{"result":` + string(resultMemberNoLF) +
		`},"schema":"verdi.context-controller-result/v2"}` + "\n")
}

func requireControllerCallRoundTrip(t *testing.T, label string, encoded []byte) {
	t.Helper()
	decoded, err := DecodeControllerCall(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("%s: DecodeControllerCall: %v", label, err)
	}
	reencoded, err := EncodeControllerCall(decoded)
	if err != nil || !bytes.Equal(reencoded, encoded) {
		t.Fatalf("%s: call.v1 does not round-trip (err=%v)", label, err)
	}
}

func requireControllerResultRoundTrip(t *testing.T, label string, encoded []byte) {
	t.Helper()
	decoded, err := DecodeControllerResult(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("%s: DecodeControllerResult: %v", label, err)
	}
	reencoded, err := EncodeControllerResult(decoded)
	if err != nil || !bytes.Equal(reencoded, encoded) {
		t.Fatalf("%s: result.v1 does not round-trip (err=%v)", label, err)
	}
}
