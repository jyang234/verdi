package constitutionapp

import (
	"strings"
	"testing"
)

func TestDecodeInspectRequest_StrictDecode(t *testing.T) {
	if _, err := DecodeInspectRequest([]byte(`{}`)); err != nil {
		t.Fatalf("empty request: %v", err)
	}
	if _, err := DecodeInspectRequest([]byte(`{"unknown":true}`)); err == nil {
		t.Fatal("expected an unknown-field refusal")
	}
	if _, err := DecodeInspectRequest([]byte(`{}{}`)); err == nil {
		t.Fatal("expected a trailing-data refusal")
	}
	if _, err := DecodeInspectRequest([]byte(`not json`)); err == nil {
		t.Fatal("expected a malformed-JSON refusal")
	}
}

func TestDecodeProposeRequest_RoundTrip(t *testing.T) {
	req := ProposeRequest{
		Branch:  "policy/x",
		Kind:    KindPolicy,
		Name:    "go-toolchain",
		Content: []byte("---\nid: policy/go-toolchain\n---\n"),
		Expected: Expected{
			Branch: "policy/x",
		},
	}
	encoded, err := EncodeResult(req)
	if err != nil {
		t.Fatalf("EncodeResult: %v", err)
	}
	decoded, err := DecodeProposeRequest(encoded)
	if err != nil {
		t.Fatalf("DecodeProposeRequest: %v", err)
	}
	if decoded.Branch != req.Branch || decoded.Kind != req.Kind || decoded.Name != req.Name || string(decoded.Content) != string(req.Content) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", decoded, req)
	}
	if _, err := DecodeProposeRequest([]byte(`{"branch":"x","bogus":1}`)); err == nil {
		t.Fatal("expected an unknown-field refusal")
	}
}

func TestEncodeResult_IsCanonicalAndDeterministic(t *testing.T) {
	result := InspectResult{Schema: InspectResultSchema}
	first, err := EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult: %v", err)
	}
	second, err := EncodeResult(result)
	if err != nil {
		t.Fatalf("EncodeResult: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("expected two encodes of the same value to be byte-identical")
	}
	if !strings.HasSuffix(string(first), "\n") {
		t.Fatal("expected a trailing newline")
	}
}

func TestDecodeValidateAndImpactReviewAndSubmitPreparationRequests(t *testing.T) {
	if _, err := DecodeValidateRequest([]byte(`{}`)); err != nil {
		t.Fatalf("ValidateRequest: %v", err)
	}
	if _, err := DecodeValidateRequest([]byte(`{"root":"x"}`)); err == nil {
		t.Fatal("expected an unknown-field refusal for a client-supplied root")
	}
	if _, err := DecodeImpactReviewRequest([]byte(`{"targets":[]}`)); err != nil {
		t.Fatalf("ImpactReviewRequest: %v", err)
	}
	if _, err := DecodeSubmitPreparationRequest([]byte(`{"targets":[]}`)); err != nil {
		t.Fatalf("SubmitPreparationRequest: %v", err)
	}
}
