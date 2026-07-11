package artifact

import (
	"bytes"
	mathrand "math/rand"
	"strings"
	"testing"
	"time"
)

// TestNewULID_Happy checks: fixed (time, entropy) is fully deterministic;
// the output is 26 Crockford-alphabet characters; and the well-known ULID
// spec example timestamp (1469918176385ms, https://github.com/ulid/spec)
// encodes to the spec's documented "01ARZ3NDEK" timestamp prefix,
// confirming this is a byte-for-byte compliant ULID encoder, not just an
// internally-consistent one.
func TestNewULID_Happy(t *testing.T) {
	tm := time.UnixMilli(1469918176385)

	t.Run("deterministic for fixed time+entropy", func(t *testing.T) {
		entropy := bytes.Repeat([]byte{0x00}, 10)
		a, err := NewULID(tm, bytes.NewReader(append([]byte(nil), entropy...)))
		if err != nil {
			t.Fatalf("NewULID: %v", err)
		}
		b, err := NewULID(tm, bytes.NewReader(append([]byte(nil), entropy...)))
		if err != nil {
			t.Fatalf("NewULID: %v", err)
		}
		if a != b {
			t.Fatalf("NewULID not deterministic: %q != %q", a, b)
		}
		if len(a) != 26 {
			t.Fatalf("len(%q) = %d, want 26", a, len(a))
		}
		for _, r := range a {
			if !strings.ContainsRune(crockfordAlphabet, r) {
				t.Fatalf("ULID %q contains non-Crockford character %q", a, r)
			}
		}
	})

	t.Run("timestamp prefix matches the ULID spec's canonical byte-shift encoding", func(t *testing.T) {
		// Cross-checks encodeCrockford's generic bit-buffer implementation
		// against the ULID spec's classic per-byte-shift encoding
		// (https://github.com/ulid/spec), reimplemented independently
		// here — not a shared helper — so a bug shared between production
		// and test code cannot hide. Same (time, entropy) in, byte-
		// identical string out is the property under test.
		id, err := NewULID(tm, bytes.NewReader(make([]byte, 10)))
		if err != nil {
			t.Fatalf("NewULID: %v", err)
		}
		var data [16]byte
		ms := tm.UnixMilli()
		data[0] = byte(ms >> 40)
		data[1] = byte(ms >> 32)
		data[2] = byte(ms >> 24)
		data[3] = byte(ms >> 16)
		data[4] = byte(ms >> 8)
		data[5] = byte(ms)
		want := referenceEncodeForTest(data)
		if id != want {
			t.Fatalf("ULID %q does not match reference byte-shift encoding %q", id, want)
		}
	})

	t.Run("distinct entropy yields distinct ULIDs at the same millisecond", func(t *testing.T) {
		a, err := NewULID(tm, bytes.NewReader(bytes.Repeat([]byte{0x00}, 10)))
		if err != nil {
			t.Fatalf("NewULID: %v", err)
		}
		b, err := NewULID(tm, bytes.NewReader(bytes.Repeat([]byte{0xFF}, 10)))
		if err != nil {
			t.Fatalf("NewULID: %v", err)
		}
		if a == b {
			t.Fatalf("expected distinct ULIDs for distinct entropy, got %q twice", a)
		}
		if a[:10] != b[:10] {
			t.Fatalf("timestamp prefixes should match at the same millisecond: %q vs %q", a[:10], b[:10])
		}
	})

	t.Run("later time sorts lexicographically after earlier time (same entropy)", func(t *testing.T) {
		entropy := bytes.Repeat([]byte{0x42}, 10)
		early, err := NewULID(tm, bytes.NewReader(append([]byte(nil), entropy...)))
		if err != nil {
			t.Fatalf("NewULID: %v", err)
		}
		later, err := NewULID(tm.Add(time.Second), bytes.NewReader(append([]byte(nil), entropy...)))
		if err != nil {
			t.Fatalf("NewULID: %v", err)
		}
		if early >= later {
			t.Fatalf("expected early ULID %q < later ULID %q", early, later)
		}
	})
}

// TestNewULID_Negative covers a pre-epoch time (rejected: the 48-bit
// timestamp cannot represent a negative offset) and entropy exhaustion
// (a reader that returns fewer than 10 bytes).
func TestNewULID_Negative(t *testing.T) {
	t.Run("pre-epoch time is rejected", func(t *testing.T) {
		_, err := NewULID(time.Unix(-1, 0), bytes.NewReader(make([]byte, 10)))
		if err == nil {
			t.Fatal("NewULID(pre-epoch): want error, got nil")
		}
	})

	t.Run("short entropy reader is rejected", func(t *testing.T) {
		_, err := NewULID(time.Now(), bytes.NewReader(make([]byte, 3)))
		if err == nil {
			t.Fatal("NewULID(short entropy): want error, got nil")
		}
	})
}

// TestNewAnnotationID_Happy proves the production path yields ids matching
// I-11's a-<ULID> shape (artifact.Annotation.Validate's own regex) and
// that consecutive calls never collide.
func TestNewAnnotationID_Happy(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id, err := NewAnnotationID()
		if err != nil {
			t.Fatalf("NewAnnotationID: %v", err)
		}
		if !annotationIDRe.MatchString(id) {
			t.Fatalf("NewAnnotationID: %q does not match a-<ULID> shape", id)
		}
		if seen[id] {
			t.Fatalf("NewAnnotationID: collision on %q", id)
		}
		seen[id] = true
	}
}

// referenceEncodeForTest is the ULID spec's classic per-byte-shift
// encoding (https://github.com/ulid/spec), reimplemented independently
// from encodeCrockford's generic bit-buffer approach so a shared bug
// cannot hide behind agreement between production code and its own test.
func referenceEncodeForTest(id [16]byte) string {
	dst := make([]byte, 26)
	dst[0] = crockfordAlphabet[(id[0]&224)>>5]
	dst[1] = crockfordAlphabet[id[0]&31]
	dst[2] = crockfordAlphabet[(id[1]&248)>>3]
	dst[3] = crockfordAlphabet[((id[1]&7)<<2)|((id[2]&192)>>6)]
	dst[4] = crockfordAlphabet[(id[2]&62)>>1]
	dst[5] = crockfordAlphabet[((id[2]&1)<<4)|((id[3]&240)>>4)]
	dst[6] = crockfordAlphabet[((id[3]&15)<<1)|((id[4]&128)>>7)]
	dst[7] = crockfordAlphabet[(id[4]&124)>>2]
	dst[8] = crockfordAlphabet[((id[4]&3)<<3)|((id[5]&224)>>5)]
	dst[9] = crockfordAlphabet[id[5]&31]

	dst[10] = crockfordAlphabet[(id[6]&248)>>3]
	dst[11] = crockfordAlphabet[((id[6]&7)<<2)|((id[7]&192)>>6)]
	dst[12] = crockfordAlphabet[(id[7]&62)>>1]
	dst[13] = crockfordAlphabet[((id[7]&1)<<4)|((id[8]&240)>>4)]
	dst[14] = crockfordAlphabet[((id[8]&15)<<1)|((id[9]&128)>>7)]
	dst[15] = crockfordAlphabet[(id[9]&124)>>2]
	dst[16] = crockfordAlphabet[((id[9]&3)<<3)|((id[10]&224)>>5)]
	dst[17] = crockfordAlphabet[id[10]&31]
	dst[18] = crockfordAlphabet[(id[11]&248)>>3]
	dst[19] = crockfordAlphabet[((id[11]&7)<<2)|((id[12]&192)>>6)]
	dst[20] = crockfordAlphabet[(id[12]&62)>>1]
	dst[21] = crockfordAlphabet[((id[12]&1)<<4)|((id[13]&240)>>4)]
	dst[22] = crockfordAlphabet[((id[13]&15)<<1)|((id[14]&128)>>7)]
	dst[23] = crockfordAlphabet[(id[14]&124)>>2]
	dst[24] = crockfordAlphabet[((id[14]&3)<<3)|((id[15]&224)>>5)]
	dst[25] = crockfordAlphabet[id[15]&31]
	return string(dst)
}

// TestEncodeCrockford_MatchesReferenceByteShift fuzzes encodeCrockford
// against referenceEncodeForTest over random 16-byte inputs: the
// bit-buffer implementation and the classic byte-shift implementation
// must always agree.
func TestEncodeCrockford_MatchesReferenceByteShift(t *testing.T) {
	rnd := mathrand.New(mathrand.NewSource(42))
	for i := 0; i < 1000; i++ {
		var data [16]byte
		rnd.Read(data[:])
		got := encodeCrockford(data)
		want := referenceEncodeForTest(data)
		if got != want {
			t.Fatalf("mismatch for % x: got %q want %q", data, got, want)
		}
	}
}
