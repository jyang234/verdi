package constitutionapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/constitutionimpact"
)

func TestDecodeInspectRequest_StrictDecode(t *testing.T) {
	if _, err := DecodeInspectRequest([]byte(`{"schema":"` + InspectRequestSchema + `"}`)); err != nil {
		t.Fatalf("versioned request: %v", err)
	}
	if _, err := DecodeInspectRequest([]byte(`{"schema":"` + InspectRequestSchema + `","unknown":true}`)); err == nil {
		t.Fatal("expected an unknown-field refusal")
	}
	if _, err := DecodeInspectRequest([]byte(`{"schema":"` + InspectRequestSchema + `"}{}`)); err == nil {
		t.Fatal("expected a trailing-data refusal")
	}
	if _, err := DecodeInspectRequest([]byte(`not json`)); err == nil {
		t.Fatal("expected a malformed-JSON refusal")
	}
}

func TestEncodeResult_EmbedsCanonicalImpactCoverage(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	review, typed := svc.ImpactReview(context.Background(), root, ImpactReviewRequest{})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	encoded, err := EncodeResult(review)
	if err != nil {
		t.Fatalf("EncodeResult: %v", err)
	}
	var envelope struct {
		Coverage json.RawMessage `json:"coverage"`
	}
	if err := json.Unmarshal(encoded, &envelope); err != nil {
		t.Fatalf("decode result envelope: %v", err)
	}
	nested := append(append([]byte(nil), envelope.Coverage...), '\n')
	if _, err := constitutionimpact.DecodeCoverage(nested); err != nil {
		t.Fatalf("nested coverage does not satisfy its frozen wire codec: %v\nresult=%s", err, encoded)
	}
	want, err := constitutionimpact.EncodeCoverage(review.Coverage)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(nested, want) {
		t.Fatalf("nested coverage is not byte-canonical:\ngot:  %s\nwant: %s", nested, want)
	}
	if bytes.Contains(encoded, []byte(`"Coverage"`)) || bytes.Contains(encoded, []byte(`"ConsumerIdentity"`)) {
		t.Fatalf("result leaked domain-struct field names instead of the frozen coverage grammar: %s", encoded)
	}
}

// TestDecodeRequests_RequireExactEnvelopeVersion proves the request half of
// the .../request/v1 contract is real rather than a doc-comment claim: an
// absent, empty, or differing schema is refused, never defaulted into
// whichever version this build happens to implement.
func TestDecodeRequests_RequireExactEnvelopeVersion(t *testing.T) {
	decoders := map[string]struct {
		want   string
		decode func([]byte) error
	}{
		"inspect": {InspectRequestSchema, func(b []byte) error {
			_, err := DecodeInspectRequest(b)
			return err
		}},
		"validate": {ValidateRequestSchema, func(b []byte) error {
			_, err := DecodeValidateRequest(b)
			return err
		}},
		"impact-review": {ImpactReviewRequestSchema, func(b []byte) error {
			_, err := DecodeImpactReviewRequest(b)
			return err
		}},
		"submit-preparation": {SubmitPreparationRequestSchema, func(b []byte) error {
			_, err := DecodeSubmitPreparationRequest(b)
			return err
		}},
		"propose": {ProposeRequestSchema, func(b []byte) error {
			_, err := DecodeProposeRequest(b)
			return err
		}},
	}
	for name, d := range decoders {
		t.Run(name, func(t *testing.T) {
			if err := d.decode([]byte(`{}`)); err == nil {
				t.Fatal("expected a refusal for a request carrying no schema field at all")
			}
			if err := d.decode([]byte(`{"schema":""}`)); err == nil {
				t.Fatal("expected a refusal for an empty schema field")
			}
			if err := d.decode([]byte(`{"schema":"verdi.constitution-inspect-request/v99"}`)); err == nil {
				t.Fatal("expected a refusal for an unrecognized envelope version")
			}
			if err := d.decode([]byte(`{"schema":"` + d.want + `"}`)); err != nil {
				t.Fatalf("expected the exact envelope version to be accepted: %v", err)
			}
		})
	}
}

// TestDecodeRequests_RejectDuplicateKeysAndNulls proves the closed grammar
// these envelopes declare: two spellings of one document (a repeated key)
// and an explicit null (indistinguishable after decoding from both an
// omitted key and an empty value) are refused rather than silently
// reinterpreted.
func TestDecodeRequests_RejectDuplicateKeysAndNulls(t *testing.T) {
	cases := []struct {
		name string
		body string
		fn   func([]byte) error
	}{
		{"impact-review duplicate targets", `{"schema":"` + ImpactReviewRequestSchema + `","targets":[],"targets":[]}`,
			func(b []byte) error { _, err := DecodeImpactReviewRequest(b); return err }},
		{"impact-review duplicate schema", `{"schema":"` + ImpactReviewRequestSchema + `","schema":"` + ImpactReviewRequestSchema + `"}`,
			func(b []byte) error { _, err := DecodeImpactReviewRequest(b); return err }},
		{"impact-review null targets", `{"schema":"` + ImpactReviewRequestSchema + `","targets":null}`,
			func(b []byte) error { _, err := DecodeImpactReviewRequest(b); return err }},
		{"validate whole document null", `null`,
			func(b []byte) error { _, err := DecodeValidateRequest(b); return err }},
		{"inspect null schema", `{"schema":null}`,
			func(b []byte) error { _, err := DecodeInspectRequest(b); return err }},
		{"propose null content", `{"schema":"` + ProposeRequestSchema + `","content":null}`,
			func(b []byte) error { _, err := DecodeProposeRequest(b); return err }},
		{"propose duplicate branch", `{"schema":"` + ProposeRequestSchema + `","branch":"a","branch":"b"}`,
			func(b []byte) error { _, err := DecodeProposeRequest(b); return err }},
		{"submit-preparation nested null", `{"schema":"` + SubmitPreparationRequestSchema + `","targets":[{"spec":null}]}`,
			func(b []byte) error { _, err := DecodeSubmitPreparationRequest(b); return err }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := c.fn([]byte(c.body)); err == nil {
				t.Fatalf("expected a refusal for %s", c.body)
			}
		})
	}
}

func TestDecodeProposeRequest_RoundTrip(t *testing.T) {
	req := ProposeRequest{
		Schema:  ProposeRequestSchema,
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
	if decoded.Schema != req.Schema || decoded.Branch != req.Branch || decoded.Kind != req.Kind || decoded.Name != req.Name || string(decoded.Content) != string(req.Content) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", decoded, req)
	}
	if _, err := DecodeProposeRequest([]byte(`{"schema":"` + ProposeRequestSchema + `","branch":"x","bogus":1}`)); err == nil {
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

// TestFailureRepositoryEffectsCanonicalWire pins the additive failure
// disclosure as part of Failure v1. The optional field is absent for
// side-effect-free refusals and, when present, carries non-null ordered path
// arrays so an adapter cannot collapse "known empty" into "not observed."
func TestFailureRepositoryEffectsCanonicalWire(t *testing.T) {
	t.Run("side-effect-free refusal omits effects", func(t *testing.T) {
		encoded, err := EncodeResult(verdict("input-invalid", "bad input").Failure())
		if err != nil {
			t.Fatal(err)
		}
		const want = "{\"classification\":\"verdict\",\"code\":\"input-invalid\",\"detail\":\"bad input\",\"schema\":\"verdi.constitution-failure/v1\"}\n"
		if string(encoded) != want {
			t.Fatalf("failure bytes = %s, want %s", encoded, want)
		}
	})

	t.Run("post-commit failure includes exact effects", func(t *testing.T) {
		typed := operational("io-failure", "resolving repository identity", errors.New("refused"))
		typed.RepositoryEffects = &RepositoryEffects{
			Operation:        "propose",
			InitialBranch:    "main",
			InitialHead:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			TargetBranch:     "policy/x",
			TargetHeadBefore: "",
			CurrentBranch:    "policy/x",
			CurrentHead:      "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			LandedCommit:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			BranchCreated:    true,
			WorktreePaths:    []string{".verdi/policy/overlays/x.md"},
			StagedPaths:      []string{".verdi/policy/overlays/x.md"},
			Unproven:         []string{},
		}
		encoded, err := EncodeResult(typed.Failure())
		if err != nil {
			t.Fatal(err)
		}
		const want = "{\"classification\":\"operational\",\"code\":\"io-failure\",\"detail\":\"resolving repository identity\",\"repository_effects\":{\"branch_created\":true,\"current_branch\":\"policy/x\",\"current_head\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\"initial_branch\":\"main\",\"initial_head\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\",\"landed_commit\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\",\"operation\":\"propose\",\"staged_paths\":[\".verdi/policy/overlays/x.md\"],\"target_branch\":\"policy/x\",\"target_head_before\":\"\",\"unproven\":[],\"worktree_paths\":[\".verdi/policy/overlays/x.md\"]},\"schema\":\"verdi.constitution-failure/v1\"}\n"
		if string(encoded) != want {
			t.Fatalf("failure bytes = %s, want %s", encoded, want)
		}
	})
}

func TestDecodeValidateAndImpactReviewAndSubmitPreparationRequests(t *testing.T) {
	if _, err := DecodeValidateRequest([]byte(`{"schema":"` + ValidateRequestSchema + `"}`)); err != nil {
		t.Fatalf("ValidateRequest: %v", err)
	}
	if _, err := DecodeValidateRequest([]byte(`{"schema":"` + ValidateRequestSchema + `","root":"x"}`)); err == nil {
		t.Fatal("expected an unknown-field refusal for a client-supplied root")
	}
	if _, err := DecodeImpactReviewRequest([]byte(`{"schema":"` + ImpactReviewRequestSchema + `","targets":[]}`)); err != nil {
		t.Fatalf("ImpactReviewRequest: %v", err)
	}
	if _, err := DecodeSubmitPreparationRequest([]byte(`{"schema":"` + SubmitPreparationRequestSchema + `","targets":[]}`)); err != nil {
		t.Fatalf("SubmitPreparationRequest: %v", err)
	}
	// Each envelope refuses its SIBLING's version: these are five distinct
	// contracts, not one interchangeable object with five names.
	if _, err := DecodeImpactReviewRequest([]byte(`{"schema":"` + SubmitPreparationRequestSchema + `","targets":[]}`)); err == nil {
		t.Fatal("expected impact-review to refuse a submit-preparation envelope")
	}
}
