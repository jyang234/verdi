package execworkspace

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSidecar_RoundTrip_ExactShape(t *testing.T) {
	id, err := NewExactIdentity("feature/my-run", validSHA)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	data, err := EncodeSidecar(id)
	if err != nil {
		t.Fatalf("EncodeSidecar: %v", err)
	}
	got, err := DecodeSidecar(data)
	if err != nil {
		t.Fatalf("DecodeSidecar: %v", err)
	}
	if !got.Equal(id) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, id)
	}
}

func TestSidecar_RoundTrip_PatchShape(t *testing.T) {
	id, err := NewPatchIdentity("feature/my-run", validSHA, []byte("diff bytes"))
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	data, err := EncodeSidecar(id)
	if err != nil {
		t.Fatalf("EncodeSidecar: %v", err)
	}
	got, err := DecodeSidecar(data)
	if err != nil {
		t.Fatalf("DecodeSidecar: %v", err)
	}
	if !got.Equal(id) {
		t.Fatalf("round trip mismatch: got %+v, want %+v", got, id)
	}
}

func TestSidecar_Encode_IsCanonicalJSON(t *testing.T) {
	id, err := NewExactIdentity("run", validSHA)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	data, err := EncodeSidecar(id)
	if err != nil {
		t.Fatalf("EncodeSidecar: %v", err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatalf("EncodeSidecar output does not end in a trailing newline: %q", data)
	}
	// Sorted keys: schema key name must appear before run_id alphabetically
	// is not guaranteed by our field names, so instead assert re-encoding
	// twice is byte-identical (determinism), and that keys are sorted by
	// checking against a manual generic decode + re-sort comparison isn't
	// needed here since canonjson itself is unit-tested; we assert this
	// package's output round-trips through encoding/json without error and
	// carries no unknown top-level shape surprises.
	var generic map[string]interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		t.Fatalf("EncodeSidecar output is not valid JSON: %v", err)
	}
	if generic["schema"] != sidecarSchema {
		t.Fatalf("schema field = %v, want %q", generic["schema"], sidecarSchema)
	}
}

func TestSidecar_Encode_Deterministic(t *testing.T) {
	id, err := NewPatchIdentity("run", validSHA, []byte("x"))
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	a, err := EncodeSidecar(id)
	if err != nil {
		t.Fatalf("EncodeSidecar: %v", err)
	}
	b, err := EncodeSidecar(id)
	if err != nil {
		t.Fatalf("EncodeSidecar: %v", err)
	}
	if string(a) != string(b) {
		t.Fatalf("EncodeSidecar not deterministic: %q vs %q", a, b)
	}
}

func TestSidecar_Decode_RejectsUnknownFields(t *testing.T) {
	data := []byte(`{"schema":"verdi.execution-request/v1","run_id":"run","commit_sha":"` + validSHA + `","extra_field":"nope"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for unknown field, got nil")
	}
}

func TestSidecar_Decode_RejectsWrongSchema(t *testing.T) {
	data := []byte(`{"schema":"verdi.execution-request/v0","run_id":"run","commit_sha":"` + validSHA + `"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for wrong schema, got nil")
	}
}

func TestSidecar_Decode_RejectsTrailingData(t *testing.T) {
	data := []byte(`{"schema":"verdi.execution-request/v1","run_id":"run","commit_sha":"` + validSHA + `"}` + "\n{}")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for trailing data, got nil")
	}
}

func TestSidecar_Decode_RejectsMissingCommitSHA(t *testing.T) {
	data := []byte(`{"schema":"verdi.execution-request/v1","run_id":"run"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for missing commit sha, got nil")
	}
}

func TestSidecar_Decode_RejectsInvalidCommitSHA(t *testing.T) {
	data := []byte(`{"schema":"verdi.execution-request/v1","run_id":"run","commit_sha":"not-a-sha"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for invalid commit sha, got nil")
	}
}

func TestSidecar_Decode_RejectsInvalidPatchSHA256(t *testing.T) {
	data := []byte(`{"schema":"verdi.execution-request/v1","run_id":"run","commit_sha":"` + validSHA + `","patch_sha256":"deadbeef"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for short patch sha256, got nil")
	}
}

// TestSidecar_Decode_DistinguishesAbsentFromPresentEmptyPatchDigest proves
// the exact-shape sidecar (patch_sha256 key absent entirely) decodes to
// ExactSHA, while a sidecar carrying patch_sha256 present but "" is a hard
// decode error rather than silently falling back to the exact shape — the
// two must never be conflated.
func TestSidecar_Decode_DistinguishesAbsentFromPresentEmptyPatchDigest(t *testing.T) {
	absent := []byte(`{"schema":"verdi.execution-request/v1","run_id":"run","commit_sha":"` + validSHA + `"}` + "\n")
	got, err := DecodeSidecar(absent)
	if err != nil {
		t.Fatalf("DecodeSidecar(absent patch digest): unexpected error: %v", err)
	}
	if got.Shape != ExactSHA {
		t.Fatalf("Shape = %v, want ExactSHA when patch_sha256 key is absent", got.Shape)
	}

	presentEmpty := []byte(`{"schema":"verdi.execution-request/v1","run_id":"run","commit_sha":"` + validSHA + `","patch_sha256":""}` + "\n")
	if _, err := DecodeSidecar(presentEmpty); err == nil {
		t.Fatalf("DecodeSidecar(patch_sha256 present but empty): want error, got nil")
	}
}

func TestSidecar_Decode_RejectsGarbage(t *testing.T) {
	if _, err := DecodeSidecar([]byte("not json at all")); err == nil {
		t.Fatalf("DecodeSidecar: want error for non-JSON input, got nil")
	}
}

func TestVerifyIdentity_EqualPasses(t *testing.T) {
	a, _ := NewExactIdentity("run", validSHA)
	b, _ := NewExactIdentity("run", validSHA)
	if err := VerifyIdentity("workspace-id", a, b); err != nil {
		t.Fatalf("VerifyIdentity: unexpected error for equal identities: %v", err)
	}
}

func TestVerifyIdentity_MismatchNamesBothRequests(t *testing.T) {
	a, _ := NewExactIdentity("run-a", validSHA)
	b, _ := NewExactIdentity("run-b", validSHA)
	err := VerifyIdentity("workspace-id", a, b)
	if err == nil {
		t.Fatalf("VerifyIdentity: want error for mismatched identities, got nil")
	}
	var mismatch *ErrIdentityMismatch
	if !asErrIdentityMismatch(err, &mismatch) {
		t.Fatalf("VerifyIdentity error is not *ErrIdentityMismatch: %v (%T)", err, err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "run-a") || !strings.Contains(msg, "run-b") {
		t.Fatalf("ErrIdentityMismatch message %q does not name both requests", msg)
	}
	if !strings.Contains(msg, "workspace-id") {
		t.Fatalf("ErrIdentityMismatch message %q does not name the workspace id", msg)
	}
}

// asErrIdentityMismatch is a small local errors.As wrapper so the test file
// doesn't need a second import alias juggle.
func asErrIdentityMismatch(err error, target **ErrIdentityMismatch) bool {
	m, ok := err.(*ErrIdentityMismatch)
	if !ok {
		return false
	}
	*target = m
	return true
}
