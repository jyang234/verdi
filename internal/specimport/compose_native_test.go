package specimport

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// newSpecNativeFixture is a native primary that already meets the CURRENT
// new-spec requirements: problem/outcome present with resolving anchors, and
// an acceptance criterion carrying both an anchor and the feature outcome
// floor's attestation kind. normalize_test.go's own nativeSpecFixture is
// deliberately NOT reused here — it is an old v0-shaped feature (no
// problem/outcome, no anchor, no attestation), which
// TestCompose_Native_RefusesOldIncompleteFeature pins as refused.
const newSpecNativeFixture = `---
id: spec/native-widget
kind: spec
class: feature
title: "Native Widget"
status: draft
owners: [team-a]
problem: { text: "Users cannot do X.", anchor: problem }
outcome: { text: "Users can do X.", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "The widget works.", evidence: [static, attestation], anchor: ac-1 }
---
# Native Widget

## Problem

Users cannot do X.

## Outcome

Users can do X.

## ac-1

The widget works.
`

// newSpecNativeRequest is nativeRequest() over newSpecNativeFixture.
func newSpecNativeRequest() Request {
	req := nativeRequest()
	req.Sources[0].Data = []byte(newSpecNativeFixture)
	return req
}

// TestCompose_Native_Happy_ByteIdentical proves a valid native primary
// passes through Compose byte-for-byte, with a stable id, and zero blocking
// findings.
func TestCompose_Native_Happy_ByteIdentical(t *testing.T) {
	root := minimalStoreRoot(t)
	req := newSpecNativeRequest()

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	requireNoBlocking(t, findings, candidate)
	if !bytes.Equal(candidate, []byte(newSpecNativeFixture)) {
		t.Fatalf("native candidate is not byte-identical to the source primary:\ngot:  %q\nwant: %q", candidate, newSpecNativeFixture)
	}
}

// TestCompose_Native_RefusesOldIncompleteFeature proves the contract's
// "Strict decode, new-spec requiredness, anchors and project checks apply
// even to old native inputs; no archive grandfathering" is actually
// enforced. nativeSpecFixture is a v0-shaped feature: no problem, no
// outcome, its one acceptance criterion carries neither an anchor nor the
// feature outcome floor's attestation kind. vl006.isNewClassSpec treats such
// a feature as grandfathered for ORDINARY corpus lint, which is unchanged —
// but a candidate is a NEW spec, so the shared candidate seam must apply the
// current floor. The refusal names each exact reason, and the input bytes
// are never rewritten to manufacture eligibility.
func TestCompose_Native_RefusesOldIncompleteFeature(t *testing.T) {
	root := minimalStoreRoot(t)
	req := nativeRequest()

	plan, err := Normalize(req)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	candidate, findings, err := Compose(context.Background(), root, req, plan)
	if err != nil {
		t.Fatalf("Compose: want blocking findings, not an error: %v", err)
	}
	if candidate != nil {
		t.Fatalf("Compose imported an old v0-shaped native feature as a new spec:\n%s", candidate)
	}
	for _, want := range []string{
		"new-class spec has no problem attribute",
		"new-class spec has no outcome attribute",
		"acceptance criterion ac-1 has no anchor",
		"does not declare attestation among its expected evidence kinds",
	} {
		if !hasBlockingMessage(findings, want) {
			t.Fatalf("no blocking finding names %q; got: %+v", want, findings)
		}
	}
	if !bytes.Equal(req.Sources[0].Data, []byte(nativeSpecFixture)) || !bytes.Equal(plan.Native, []byte(nativeSpecFixture)) {
		t.Fatal("Compose mutated the native input bytes")
	}
}

// hasBlockingMessage reports whether some blocking finding's message
// contains want.
func hasBlockingMessage(findings []Finding, want string) bool {
	for _, f := range findings {
		if f.Blocking && strings.Contains(f.Message, want) {
			return true
		}
	}
	return false
}

// TestCompose_Native_RefusesNonDraftStatus proves Compose itself refuses a
// native primary whose status is not absent/draft — even when the caller
// never ran it through Normalize at all (a hand-built Plan/Request pair,
// exactly the "no service trusts prior decoding" defense-in-depth
// composeNative's own doc comment describes) — as a blocking Finding, not
// a crash or a silently accepted candidate. A closed status with no frozen
// stamp is already a decode-time contradiction, so the pinned reason is
// artifact's own, reached through normalizeNative's DecodeSpec call rather
// than its later explicit status check; either way the refusal is the same
// blocking invalid-candidate finding on target "native".
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
	requireNativeRefusal(t, findings, `feature spec status "closed" requires a frozen stamp`)
}

// requireNativeRefusal pins WHICH gate refused a native primary: the
// invalid-candidate code, the "native" target and the exact reason. A bare
// hasBlocking assertion would survive a regression that moved the refusal to
// an unrelated gate.
func requireNativeRefusal(t *testing.T, findings []Finding, wantMessage string) {
	t.Helper()
	for _, f := range findings {
		if f.Blocking && f.Code == FindingInvalidCandidate && f.Target == "native" && strings.Contains(f.Message, wantMessage) {
			return
		}
	}
	t.Fatalf("want a blocking %s finding on target \"native\" naming %q, got: %+v", FindingInvalidCandidate, wantMessage, findings)
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
	requireNativeRefusal(t, findings, "must not carry a frozen stamp")
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
	requireNativeRefusal(t, findings, `native primary id "spec/native-widget" does not match target slug "spec/a-different-slug"`)
}
