package execworkspace

import (
	"encoding/json"
	"errors"
	"fmt"
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

// TestSidecar_Encode_IsCanonicalJSON pins the EXACT bytes of both shapes:
// sorted keys, no interior whitespace, one trailing newline (spec §Workspace
// naming: "canonical JSON — sorted keys, trailing newline"). A golden-byte
// assertion, not a shape assertion — the sidecar is a durable on-disk
// witness, so its serialization is part of the contract.
func TestSidecar_Encode_IsCanonicalJSON(t *testing.T) {
	exact, err := NewExactIdentity("run", validSHA)
	if err != nil {
		t.Fatalf("NewExactIdentity: %v", err)
	}
	gotExact, err := EncodeSidecar(exact)
	if err != nil {
		t.Fatalf("EncodeSidecar: %v", err)
	}
	wantExact := `{"commit_sha":"` + validSHA + `","run_id":"run","schema":"verdi.execution-request/v1"}` + "\n"
	if string(gotExact) != wantExact {
		t.Fatalf("EncodeSidecar(exact) =\n%q\nwant\n%q", gotExact, wantExact)
	}

	patch, err := NewPatchIdentity("run", validSHA, []byte("patch bytes"))
	if err != nil {
		t.Fatalf("NewPatchIdentity: %v", err)
	}
	gotPatch, err := EncodeSidecar(patch)
	if err != nil {
		t.Fatalf("EncodeSidecar: %v", err)
	}
	wantPatch := `{"commit_sha":"` + validSHA + `","patch_sha256":"` + patch.PatchSHA256 + `","run_id":"run","schema":"verdi.execution-request/v1"}` + "\n"
	if string(gotPatch) != wantPatch {
		t.Fatalf("EncodeSidecar(patch) =\n%q\nwant\n%q", gotPatch, wantPatch)
	}

	// The pinned bytes really are what a JSON reader sees, and the schema
	// tag in them is this package's constant rather than a literal that
	// could drift from it.
	var generic map[string]interface{}
	if err := json.Unmarshal(gotExact, &generic); err != nil {
		t.Fatalf("EncodeSidecar output is not valid JSON: %v", err)
	}
	if generic["schema"] != sidecarSchema {
		t.Fatalf("schema field = %v, want %q", generic["schema"], sidecarSchema)
	}
	if _, ok := generic["patch_sha256"]; ok {
		t.Fatalf("exact-shape sidecar carries a patch_sha256 key: %q", gotExact)
	}
}

// TestSidecar_Encode_RejectsInvalidIdentity proves a malformed identity is
// never serialized: the durable witness cannot record a request this package
// would refuse to hand back.
func TestSidecar_Encode_RejectsInvalidIdentity(t *testing.T) {
	cases := map[string]Identity{
		"short commit sha": {Shape: ExactSHA, RunID: "run", CommitSHA: "abc"},
		"empty run id":     {Shape: ExactSHA, CommitSHA: validSHA},
		"unknown shape":    {Shape: Shape(42), RunID: "run", CommitSHA: validSHA},
		"exact with patch": {Shape: ExactSHA, RunID: "run", CommitSHA: validSHA, PatchSHA256: validPatchSHA},
	}
	for name, id := range cases {
		t.Run(name, func(t *testing.T) {
			data, err := EncodeSidecar(id)
			if err == nil {
				t.Fatalf("EncodeSidecar(%+v) = %q, want error", id, data)
			}
			if data != nil {
				t.Fatalf("EncodeSidecar returned %q alongside an error, want nil", data)
			}
		})
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

// TestSidecar_Decode_RejectsWrongSchema feeds CANONICAL bytes (sorted keys,
// trailing newline) so the schema check itself is what rejects them, not the
// canonical-bytes gate.
func TestSidecar_Decode_RejectsWrongSchema(t *testing.T) {
	data := []byte(`{"commit_sha":"` + validSHA + `","run_id":"run","schema":"verdi.execution-request/v0"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for wrong schema, got nil")
	} else if !strings.Contains(err.Error(), "schema") {
		t.Fatalf("DecodeSidecar error = %v, want the schema check to reject it", err)
	}
}

func TestSidecar_Decode_RejectsTrailingData(t *testing.T) {
	data := []byte(`{"schema":"verdi.execution-request/v1","run_id":"run","commit_sha":"` + validSHA + `"}` + "\n{}")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for trailing data, got nil")
	}
}

// TestSidecar_Decode_RejectsMissingCommitSHA: an absent commit_sha key is
// caught by the canonical-bytes gate (the canonical encoding of the decoded
// document always carries the key, with its zero value) before the sha
// validation would fire — either way it is a hard decode error.
func TestSidecar_Decode_RejectsMissingCommitSHA(t *testing.T) {
	data := []byte(`{"run_id":"run","schema":"verdi.execution-request/v1"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for missing commit sha, got nil")
	}
}

func TestSidecar_Decode_RejectsInvalidCommitSHA(t *testing.T) {
	data := []byte(`{"commit_sha":"not-a-sha","run_id":"run","schema":"verdi.execution-request/v1"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar: want error for invalid commit sha, got nil")
	}
}

func TestSidecar_Decode_RejectsInvalidPatchSHA256(t *testing.T) {
	data := []byte(`{"commit_sha":"` + validSHA + `","patch_sha256":"deadbeef","run_id":"run","schema":"verdi.execution-request/v1"}` + "\n")
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
	absent := []byte(`{"commit_sha":"` + validSHA + `","run_id":"run","schema":"verdi.execution-request/v1"}` + "\n")
	got, err := DecodeSidecar(absent)
	if err != nil {
		t.Fatalf("DecodeSidecar(absent patch digest): unexpected error: %v", err)
	}
	if got.Shape != ExactSHA {
		t.Fatalf("Shape = %v, want ExactSHA when patch_sha256 key is absent", got.Shape)
	}

	presentEmpty := []byte(`{"commit_sha":"` + validSHA + `","patch_sha256":"","run_id":"run","schema":"verdi.execution-request/v1"}` + "\n")
	if _, err := DecodeSidecar(presentEmpty); err == nil {
		t.Fatalf("DecodeSidecar(patch_sha256 present but empty): want error, got nil")
	}
}

// TestSidecar_Decode_RejectsExplicitNullPatchDigest proves an explicit JSON
// null for patch_sha256 is an UNDECODABLE sidecar, not a silent synonym for
// the key being absent (the exact-SHA shape): the canonical-bytes gate
// rejects it because re-encoding the decoded document omits the key
// entirely, so the bytes on disk are not the canonical encoding of what was
// read.
func TestSidecar_Decode_RejectsExplicitNullPatchDigest(t *testing.T) {
	data := []byte(`{"commit_sha":"` + validSHA + `","patch_sha256":null,"run_id":"run","schema":"verdi.execution-request/v1"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar(patch_sha256 null): want error, got nil")
	}
}

// TestSidecar_Decode_RejectsDuplicateKeys proves a duplicate key is an
// undecodable sidecar rather than encoding/json's silent last-wins read.
func TestSidecar_Decode_RejectsDuplicateKeys(t *testing.T) {
	data := []byte(`{"commit_sha":"` + validSHA + `","commit_sha":"` + validSHA + `","run_id":"run","schema":"verdi.execution-request/v1"}` + "\n")
	if _, err := DecodeSidecar(data); err == nil {
		t.Fatalf("DecodeSidecar(duplicate commit_sha): want error, got nil")
	}
}

// TestSidecar_Decode_RejectsNonCanonicalBytes proves every non-canonical
// serialization of an otherwise well-formed sidecar is undecodable: only the
// exact canonical bytes this package would itself write are accepted.
func TestSidecar_Decode_RejectsNonCanonicalBytes(t *testing.T) {
	cases := map[string]string{
		"unsorted keys":          `{"schema":"verdi.execution-request/v1","run_id":"run","commit_sha":"` + validSHA + `"}` + "\n",
		"interior whitespace":    `{"commit_sha": "` + validSHA + `", "run_id": "run", "schema": "verdi.execution-request/v1"}` + "\n",
		"indented":               "{\n  \"commit_sha\": \"" + validSHA + "\",\n  \"run_id\": \"run\",\n  \"schema\": \"verdi.execution-request/v1\"\n}\n",
		"no trailing newline":    `{"commit_sha":"` + validSHA + `","run_id":"run","schema":"verdi.execution-request/v1"}`,
		"extra trailing newline": `{"commit_sha":"` + validSHA + `","run_id":"run","schema":"verdi.execution-request/v1"}` + "\n\n",
		"escaped unicode run id": `{"commit_sha":"` + validSHA + `","run_id":"\u0072un","schema":"verdi.execution-request/v1"}` + "\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeSidecar([]byte(body)); err == nil {
				t.Fatalf("DecodeSidecar(%s): want error for non-canonical bytes, got nil", name)
			}
		})
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

	// A caller wraps this error on its way up; errors.As must still reach
	// the typed mismatch through the wrap, so the mismatch stays
	// programmatically recognizable rather than only human-readable.
	wrapped := fmt.Errorf("materializing workspace: %w", err)
	var mismatch *ErrIdentityMismatch
	if !errors.As(wrapped, &mismatch) {
		t.Fatalf("errors.As did not reach *ErrIdentityMismatch through a wrap: %v (%T)", wrapped, wrapped)
	}
	if mismatch.WorkspaceID != "workspace-id" {
		t.Fatalf("mismatch.WorkspaceID = %q, want %q", mismatch.WorkspaceID, "workspace-id")
	}
	if !mismatch.Proposed.Equal(a) || !mismatch.Recorded.Equal(b) {
		t.Fatalf("mismatch carries the wrong identities: proposed %+v recorded %+v", mismatch.Proposed, mismatch.Recorded)
	}

	msg := wrapped.Error()
	if !strings.Contains(msg, "run-a") || !strings.Contains(msg, "run-b") {
		t.Fatalf("ErrIdentityMismatch message %q does not name both requests", msg)
	}
	if !strings.Contains(msg, "workspace-id") {
		t.Fatalf("ErrIdentityMismatch message %q does not name the workspace id", msg)
	}
}
