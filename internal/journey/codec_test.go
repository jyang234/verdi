package journey

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalDeterministic(t *testing.T) {
	r := validRecord(t)
	out1, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	out2, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if !bytes.Equal(out1, out2) {
		t.Errorf("Canonical is not deterministic:\n%s\nvs\n%s", out1, out2)
	}
}

func TestCanonicalSetsDigest(t *testing.T) {
	r := validRecord(t)
	out, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	digest, _ := decoded["digest"].(string)
	if !digestRe.MatchString(digest) {
		t.Errorf("canonical output digest %q does not match sha256:<hex> form", digest)
	}
}

func TestCanonicalRejectsInvalidRecord(t *testing.T) {
	r := validRecord(t)
	r.Schema = "bad-schema"
	if _, err := Canonical(r); err == nil {
		t.Fatalf("Canonical: expected error for invalid record, got nil")
	}
}

func TestCanonicalIgnoresCarriedDigest(t *testing.T) {
	r := validRecord(t)
	r.Digest = "sha256:" + strings.Repeat("0", 64)
	out, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	r2 := validRecord(t) // same content, Digest == ""
	out2, err := Canonical(r2)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	if !bytes.Equal(out, out2) {
		t.Errorf("Canonical output depends on the carried input digest; it must always recompute")
	}
}

func TestDecodeRoundTrip(t *testing.T) {
	r := validRecord(t)
	out, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}

	decoded, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	want := r
	want.Digest = decoded.Digest
	if !reflect.DeepEqual(decoded, want) {
		t.Errorf("Decode(Canonical(r)) = %+v, want %+v", decoded, want)
	}

	out2, err := Canonical(decoded)
	if err != nil {
		t.Fatalf("Canonical(decoded): %v", err)
	}
	if !bytes.Equal(out, out2) {
		t.Errorf("Canonical -> Decode -> Canonical not byte-identical:\n%s\nvs\n%s", out, out2)
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	r := validRecord(t)
	out, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	trailing := append(append([]byte{}, out...), []byte("{}")...)
	if _, err := Decode(trailing); err == nil {
		t.Fatalf("Decode: expected error for trailing data, got nil")
	} else if !strings.Contains(err.Error(), "trailing") {
		t.Errorf("Decode error = %q, want mention of trailing data", err.Error())
	}
}

func TestDecodeRejectsUnknownField(t *testing.T) {
	r := validRecord(t)
	out, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	var generic map[string]interface{}
	if err := json.Unmarshal(out, &generic); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	generic["unexpected_field"] = "surprise"
	mutated, err := json.Marshal(generic)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if _, err := Decode(mutated); err == nil {
		t.Fatalf("Decode: expected error for unknown field, got nil")
	}
}

func TestDecodeRejectsDigestMismatch(t *testing.T) {
	r := validRecord(t)
	out, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	decoded, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	mutated := decoded
	mutated.Target.Ref = "spec/other-target"
	// mutated.Digest is left as decoded's original digest: now stale.
	data, err := json.Marshal(mutated)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	_, err = Decode(data)
	if err == nil {
		t.Fatalf("Decode: expected digest mismatch error, got nil")
	}
	if !strings.Contains(err.Error(), "digest") {
		t.Errorf("Decode error = %q, want mention of digest", err.Error())
	}
}

func TestDecodeRejectsInvalidRecordBeforeDigestCheck(t *testing.T) {
	r := validRecord(t)
	out, err := Canonical(r)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	decoded, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	mutated := decoded
	mutated.Lifecycle.State = "in-review"
	data, err := json.Marshal(mutated)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	_, err = Decode(data)
	if err == nil {
		t.Fatalf("Decode: expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "state") {
		t.Errorf("Decode error = %q, want mention of the invalid lifecycle state", err.Error())
	}
}

func TestDecodeRejectsMalformedJSON(t *testing.T) {
	if _, err := Decode([]byte("{not json")); err == nil {
		t.Fatalf("Decode: expected error for malformed JSON, got nil")
	}
}
