package mcpserve

import (
	"crypto/rand"
	"fmt"
	"io"
	"time"
)

// crockfordAlphabet is the 32-symbol Crockford base32 alphabet (excludes
// I, L, O, U to avoid visual confusion) — the ULID spec's encoding
// (https://github.com/ulid/spec), matching artifact.annotationIDRe's
// `[0-9A-HJKMNP-TV-Z]{26}` shape (I-11).
const crockfordAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewULID generates one ULID (26-character Crockford base32: a 48-bit
// millisecond timestamp followed by 80 bits of entropy) from an
// explicit time and entropy source — injectable so tests are
// deterministic (PLAN.md Phase 9 deliverable 2: "implement Crockford-
// base32 ULID stdlib-only with injectable time+entropy for deterministic
// tests"). t must not be before the Unix epoch (a ULID's timestamp field
// cannot represent a negative offset).
func NewULID(t time.Time, entropy io.Reader) (string, error) {
	ms := t.UnixMilli()
	if ms < 0 {
		return "", fmt.Errorf("mcpserve: NewULID: time %s is before the Unix epoch", t)
	}
	// 48-bit timestamp ceiling: 2^48-1 ms after epoch, far beyond any
	// real use, but explicit rather than silently truncating.
	if ms > 0xFFFFFFFFFFFF {
		return "", fmt.Errorf("mcpserve: NewULID: time %s overflows the ULID's 48-bit millisecond timestamp", t)
	}

	var rnd [10]byte // 80 bits of entropy
	if _, err := io.ReadFull(entropy, rnd[:]); err != nil {
		return "", fmt.Errorf("mcpserve: NewULID: reading entropy: %w", err)
	}

	var data [16]byte // 128 bits: 48-bit timestamp + 80-bit entropy
	data[0] = byte(ms >> 40)
	data[1] = byte(ms >> 32)
	data[2] = byte(ms >> 24)
	data[3] = byte(ms >> 16)
	data[4] = byte(ms >> 8)
	data[5] = byte(ms)
	copy(data[6:], rnd[:])

	return encodeCrockford(data), nil
}

// NewAnnotationID generates a fresh "a-<ULID>" annotation id (I-11) using
// the real clock and crypto/rand — the production path; tests use NewULID
// directly with injected time+entropy for determinism.
func NewAnnotationID() (string, error) {
	id, err := NewULID(time.Now(), rand.Reader)
	if err != nil {
		return "", err
	}
	return "a-" + id, nil
}

// encodeCrockford encodes 128 bits (16 bytes) as 26 Crockford base32
// characters. 26*5 = 130 bits, 2 more than the 128 input bits, so the
// bitstream is conceptually left-padded with 2 zero bits before being cut
// into 5-bit groups — the same convention the ULID spec's reference
// encodings use, and the reason a ULID's first character is always in
// 0-7.
func encodeCrockford(data [16]byte) string {
	var bits [130]byte // 2 padding bits + 128 data bits, one byte (0/1) per bit
	for i := 0; i < 128; i++ {
		byteIdx := i / 8
		bitIdx := 7 - uint(i%8)
		bits[2+i] = (data[byteIdx] >> bitIdx) & 1
	}

	var out [26]byte
	for i := 0; i < 26; i++ {
		var v byte
		for j := 0; j < 5; j++ {
			v = (v << 1) | bits[i*5+j]
		}
		out[i] = crockfordAlphabet[v]
	}
	return string(out[:])
}
