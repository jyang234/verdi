package recovery

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
)

// Canonical validates p, then returns its canonical JSON encoding (sorted
// keys, no HTML escaping, trailing newline) with Digest recomputed from
// the rest of the projection — any digest p already carries is
// discarded, never trusted. Two calls on equal projections (ignoring
// Digest) always yield byte-identical output.
func Canonical(p Projection) ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("recovery: canonical: %w", err)
	}

	digestless := p
	digestless.Digest = ""
	digest, err := canonjson.Digest(digestless)
	if err != nil {
		return nil, fmt.Errorf("recovery: canonical: computing digest: %w", err)
	}
	p.Digest = digest

	out, err := canonjson.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("recovery: canonical: marshaling: %w", err)
	}
	return out, nil
}

// Decode strictly decodes data as a Projection: unknown fields and
// trailing data are rejected, the result must validate, and its carried
// Digest must match a fresh recomputation — a projection whose bytes were
// altered after Canonical produced them is rejected rather than silently
// trusted.
func Decode(data []byte) (Projection, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var p Projection
	if err := dec.Decode(&p); err != nil {
		return Projection{}, fmt.Errorf("recovery: decode: %w", err)
	}
	if dec.More() {
		return Projection{}, fmt.Errorf("recovery: decode: trailing data after top-level value")
	}

	if err := p.Validate(); err != nil {
		return Projection{}, fmt.Errorf("recovery: decode: %w", err)
	}

	digestless := p
	digestless.Digest = ""
	want, err := canonjson.Digest(digestless)
	if err != nil {
		return Projection{}, fmt.Errorf("recovery: decode: computing digest: %w", err)
	}
	if p.Digest != want {
		return Projection{}, fmt.Errorf("recovery: decode: digest mismatch: projection carries %q, recomputed %q", p.Digest, want)
	}

	return p, nil
}
