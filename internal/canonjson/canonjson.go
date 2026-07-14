// Package canonjson implements the canonical JSON form used for every
// generated, digest- or integrity-hashed artifact in the store (PLAN.md
// I-18, mirroring upstream verdi-go's own canonjson): object keys sorted,
// no HTML escaping, and a trailing newline. Two calls to Marshal on
// semantically equal values always produce byte-identical output,
// regardless of Go map iteration order or struct field declaration order —
// the property digests and integrity hashes depend on.
package canonjson

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// Marshal returns the canonical JSON encoding of v.
//
// Implementation note: v is first marshaled with the standard encoder (so
// struct tags, MarshalJSON methods, etc. all apply normally), then
// round-tripped through a generic decode (numbers preserved verbatim via
// json.Number, so re-encoding never perturbs numeric formatting) and a
// custom recursive encoder that sorts object keys and disables HTML
// escaping at every level, including inside nested values a plain
// json.Marshal call would otherwise escape.
func Marshal(v interface{}) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("canonjson: marshal: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var generic interface{}
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("canonjson: decode intermediate form: %w", err)
	}
	// A single JSON document must contain exactly one value; reject any
	// trailing data the same way the rest of the system's strict JSON
	// decode does (CLAUDE.md: "trailing-data rejection").
	if dec.More() {
		return nil, fmt.Errorf("canonjson: trailing data after top-level value")
	}

	var buf bytes.Buffer
	if err := encode(&buf, generic); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// Digest returns v's content-address: "sha256:" followed by the lowercase
// hex encoding of the SHA-256 sum of v's canonical JSON encoding (Marshal).
// This is the one home for the canonjson-then-sha256-then-"sha256:"+hex
// tail that spec/shared-homes ac-2/dc-2 collapse from ten hand-rolled
// copies across seven packages: internal/bundle's recordDigest, internal/
// runtime probe's recordDigest, internal/decisionsweep's exemptionDigest,
// internal/align's ComputeDigest / ComputeDecisionDigest / adrCorpusDigest
// tails, internal/artifact.ObjectContentHash, internal/commitdesign's
// freezeBoard, and cmd/verdi's rollupDigest and selfHostedDigest. Digest
// lives here rather than in internal/artifact because the digest IS a
// property of the canonical encoding (dc-2): field-projecting callers keep
// their own projection structs/types and call Digest on them — this helper
// owns only the hash tail, never the shape being hashed.
func Digest(v interface{}) (string, error) {
	data, err := Marshal(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// encode writes v's canonical form to buf: map keys sorted, arrays in
// their original order (order is significant there and already
// deterministic), scalars written without HTML escaping.
func encode(buf *bytes.Buffer, v interface{}) error {
	switch val := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encodeScalar(buf, k); err != nil {
				return err
			}
			buf.WriteByte(':')
			if err := encode(buf, val[k]); err != nil {
				return err
			}
		}
		buf.WriteByte('}')

	case []interface{}:
		buf.WriteByte('[')
		for i, e := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := encode(buf, e); err != nil {
				return err
			}
		}
		buf.WriteByte(']')

	default:
		return encodeScalar(buf, val)
	}
	return nil
}

// encodeScalar writes v (a string, json.Number, bool, or nil — anything
// that isn't a JSON object or array) to buf with HTML escaping disabled.
func encodeScalar(buf *bytes.Buffer, v interface{}) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("canonjson: encode scalar %v: %w", v, err)
	}
	// json.Encoder.Encode appends a trailing newline per call; strip it
	// since Marshal adds the single, document-level trailing newline itself.
	buf.Write(bytes.TrimRight(b.Bytes(), "\n"))
	return nil
}
