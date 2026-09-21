package recovery

import (
	"bytes"
	"testing"
)

func TestCanonicalDeterministic(t *testing.T) {
	p := validProjection(t)
	a, err := Canonical(p)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	b, err := Canonical(p)
	if err != nil {
		t.Fatalf("Canonical (second call): %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("Canonical is not deterministic:\n%s\nvs\n%s", a, b)
	}
}

func TestDecodeRoundTrip(t *testing.T) {
	p := validProjection(t)
	data, err := Canonical(p)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	data2, err := Canonical(got)
	if err != nil {
		t.Fatalf("Canonical (round-tripped): %v", err)
	}
	if !bytes.Equal(data, data2) {
		t.Fatalf("round trip is not byte-identical:\n%s\nvs\n%s", data, data2)
	}
}

func TestDecodeRejectsTamperedDigest(t *testing.T) {
	p := validProjection(t)
	data, err := Canonical(p)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	tampered := bytes.Replace(data, []byte("close/checkout"), []byte("close/checkoutX"), 1)
	if bytes.Equal(tampered, data) {
		t.Fatal("tamper did not change the bytes")
	}
	if _, err := Decode(tampered); err == nil {
		t.Fatal("Decode(tampered) = nil error, want digest mismatch")
	}
}

func TestDecodeRejectsUnknownField(t *testing.T) {
	p := validProjection(t)
	data, err := Canonical(p)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	withField := bytes.Replace(data, []byte(`"schema":`), []byte(`"unknown_field":true,"schema":`), 1)
	if _, err := Decode(withField); err == nil {
		t.Fatal("Decode(unknown field) = nil error, want a strict-decode rejection")
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	p := validProjection(t)
	data, err := Canonical(p)
	if err != nil {
		t.Fatalf("Canonical: %v", err)
	}
	trailing := append(append([]byte{}, data...), []byte("{}")...)
	if _, err := Decode(trailing); err == nil {
		t.Fatal("Decode(trailing data) = nil error, want rejection")
	}
}
