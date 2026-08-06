package journey

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
)

// Canonical validates r, then returns its canonical JSON encoding (CO-4:
// registered identifiers, stable sorting, strict schema) with Digest
// recomputed from the rest of the record — any digest r already carries
// is discarded, never trusted. Two calls on equal records (ignoring
// Digest) always yield byte-identical output.
func Canonical(r Record) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("journey: canonical: %w", err)
	}

	digestless := r
	digestless.Digest = ""
	digest, err := canonjson.Digest(digestless)
	if err != nil {
		return nil, fmt.Errorf("journey: canonical: computing digest: %w", err)
	}
	r.Digest = digest

	out, err := canonjson.Marshal(r)
	if err != nil {
		return nil, fmt.Errorf("journey: canonical: marshaling: %w", err)
	}
	return out, nil
}

// Decode strictly decodes data as a Record: unknown fields and trailing
// data are rejected, the result must validate, and its carried Digest
// must match a fresh recomputation — a record whose bytes were altered
// after Canonical produced them is rejected rather than silently trusted.
func Decode(data []byte) (Record, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var r Record
	if err := dec.Decode(&r); err != nil {
		return Record{}, fmt.Errorf("journey: decode: %w", err)
	}
	if dec.More() {
		return Record{}, fmt.Errorf("journey: decode: trailing data after top-level value")
	}

	if err := r.Validate(); err != nil {
		return Record{}, fmt.Errorf("journey: decode: %w", err)
	}

	digestless := r
	digestless.Digest = ""
	want, err := canonjson.Digest(digestless)
	if err != nil {
		return Record{}, fmt.Errorf("journey: decode: computing digest: %w", err)
	}
	if r.Digest != want {
		return Record{}, fmt.Errorf("journey: decode: digest mismatch: record carries %q, recomputed %q", r.Digest, want)
	}

	return r, nil
}
