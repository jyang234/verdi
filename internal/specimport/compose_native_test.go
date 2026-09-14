package specimport

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// TestCompose_Native_Happy_ByteIdentical proves a valid native primary
// (normalize_test.go's own nativeRequest/nativeSpecFixture — reused
// rather than a second near-duplicate fixture) passes through Compose
// byte-for-byte, with a stable id, and zero blocking findings.
func TestCompose_Native_Happy_ByteIdentical(t *testing.T) {
	root := minimalStoreRoot(t)
	req := nativeRequest()

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	requireNoBlocking(t, findings, candidate)
	if !bytes.Equal(candidate, []byte(nativeSpecFixture)) {
		t.Fatalf("native candidate is not byte-identical to the source primary:\ngot:  %q\nwant: %q", candidate, nativeSpecFixture)
	}
}

// TestCompose_Native_RefusesNonDraftStatus proves Compose itself refuses a
// native primary whose status is not absent/draft — even when the caller
// never ran it through Normalize at all (a hand-built Plan/Request pair,
// exactly the "no service trusts prior decoding" defense-in-depth
// composeNative's own doc comment describes) — as a blocking Finding, not
// a crash or a silently accepted candidate.
func TestCompose_Native_RefusesNonDraftStatus(t *testing.T) {
	root := minimalStoreRoot(t)
	closedSpec := strings.Replace(nativeSpecFixture, "status: draft\n", "status: closed\n", 1)
	req := nativeRequest()
	req.Sources[0].Data = []byte(closedSpec)
	plan := Plan{Native: []byte(closedSpec)}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose returned bytes for a refused native primary: %s", candidate)
	}
	if !hasBlocking(findings) {
		t.Fatalf("want a blocking finding refusing the closed native primary, got: %+v", findings)
	}
}

// TestCompose_Native_RefusesFrozenMetadata proves a native primary
// carrying a frozen: stamp is refused, not silently imported as though it
// were a fresh draft. A draft status paired with a frozen stamp is
// already a decode-time contradiction (artifact.requireFrozen: "status
// %q must not carry a frozen stamp") surfaced through normalizeNative's
// own artifact.DecodeSpec call — the observable outcome (a blocking
// Finding, nil bytes) is identical to normalizeNative's own explicit
// spec.Frozen != nil check either way.
func TestCompose_Native_RefusesFrozenMetadata(t *testing.T) {
	root := minimalStoreRoot(t)
	frozenSpec := strings.Replace(nativeSpecFixture, "status: draft\n",
		"status: draft\nfrozen: { at: \"2026-01-01\", commit: \"1234567\" }\n", 1)
	req := nativeRequest()
	req.Sources[0].Data = []byte(frozenSpec)
	plan := Plan{Native: []byte(frozenSpec)}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose returned bytes for a frozen native primary: %s", candidate)
	}
	if !hasBlocking(findings) {
		t.Fatalf("want a blocking finding refusing frozen native metadata, got: %+v", findings)
	}
}

// TestCompose_Native_RefusesTargetMismatch proves a native primary whose
// id disagrees with Target.Slug is refused.
func TestCompose_Native_RefusesTargetMismatch(t *testing.T) {
	root := minimalStoreRoot(t)
	req := nativeRequest()
	req.Target.Slug = "a-different-slug"
	plan := Plan{Native: []byte(nativeSpecFixture)}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want a blocking finding, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose returned bytes for a mismatched native primary: %s", candidate)
	}
	if !hasBlocking(findings) {
		t.Fatalf("want a blocking finding refusing the id/target mismatch, got: %+v", findings)
	}
}
