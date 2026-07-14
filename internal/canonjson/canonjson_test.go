package canonjson

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestMarshal_SortsObjectKeys(t *testing.T) {
	got, err := Marshal(map[string]interface{}{"b": 1, "a": 2, "c": 3})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := "{\"a\":2,\"b\":1,\"c\":3}\n"
	if string(got) != want {
		t.Fatalf("Marshal = %q, want %q", got, want)
	}
}

func TestMarshal_TrailingNewline(t *testing.T) {
	got, err := Marshal(map[string]interface{}{"a": 1})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !bytes.HasSuffix(got, []byte("\n")) {
		t.Fatalf("Marshal = %q, want trailing newline", got)
	}
	if bytes.HasSuffix(got, []byte("\n\n")) {
		t.Fatalf("Marshal = %q, want exactly one trailing newline", got)
	}
}

func TestMarshal_NoHTMLEscaping(t *testing.T) {
	got, err := Marshal(map[string]interface{}{"a": "<b>&\"c\"</b>"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	// The standard library's default HTML-escaping would turn "<" and "&"
	// into the < / & escape sequences below; canonical JSON must
	// leave them as literal bytes instead.
	forbidden := []string{"\\u003c", "\\u0026"}
	for _, seq := range forbidden {
		if strings.Contains(string(got), seq) {
			t.Fatalf("Marshal HTML-escaped output (found %s): %q", seq, got)
		}
	}
	want := "{\"a\":\"<b>&\\\"c\\\"</b>\"}\n"
	if string(got) != want {
		t.Fatalf("Marshal = %q, want %q", got, want)
	}
}

func TestMarshal_NestedStructuresDeterministic(t *testing.T) {
	type inner struct {
		Zeta  int `json:"zeta"`
		Alpha int `json:"alpha"`
	}
	type outer struct {
		Items []inner           `json:"items"`
		Tags  map[string]string `json:"tags"`
	}

	v := outer{
		Items: []inner{{Zeta: 1, Alpha: 2}, {Zeta: 3, Alpha: 4}},
		Tags:  map[string]string{"z": "last", "a": "first"},
	}

	got1, err := Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got2, err := Marshal(v)
	if err != nil {
		t.Fatalf("Marshal (second call): %v", err)
	}
	if !bytes.Equal(got1, got2) {
		t.Fatalf("Marshal not deterministic across calls: %q vs %q", got1, got2)
	}

	want := "{\"items\":[{\"alpha\":2,\"zeta\":1},{\"alpha\":4,\"zeta\":3}],\"tags\":{\"a\":\"first\",\"z\":\"last\"}}\n"
	if string(got1) != want {
		t.Fatalf("Marshal = %q, want %q", got1, want)
	}
}

func TestMarshal_MapIterationOrderDoesNotLeak(t *testing.T) {
	// Go deliberately randomizes map iteration order; build the same
	// logical map many times and confirm the encoding never varies.
	first, err := Marshal(map[string]interface{}{"k1": 1, "k2": 2, "k3": 3, "k4": 4, "k5": 5})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for i := 0; i < 50; i++ {
		got, err := Marshal(map[string]interface{}{"k1": 1, "k2": 2, "k3": 3, "k4": 4, "k5": 5})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if !bytes.Equal(first, got) {
			t.Fatalf("iteration %d: Marshal output varied: %q vs %q", i, got, first)
		}
	}
}

func TestMarshal_PreservesNumberFormatting(t *testing.T) {
	got, err := Marshal(map[string]interface{}{"n": 10000000000})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := "{\"n\":10000000000}\n"
	if string(got) != want {
		t.Fatalf("Marshal = %q, want %q (large integers must not become float scientific notation)", got, want)
	}
}

// --- Digest: dc-2's byte-equivalence witness ---

// digestFixture stands in for the shape every former hand-rolled digest
// tail hashed (an id/kind/text-style projection struct) — spec/shared-
// homes ac-2's callers keep exactly this kind of projection type and pass
// it to Digest; Digest itself owns only the hash tail (dc-2).
type digestFixture struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
	Text string `json:"text"`
}

// TestDigest_ByteEquivalentToOldHandRolledTail is the ac-2/dc-2 witness:
// it computes the digest of a committed fixture value two ways — the OLD
// hand-copied pattern (inline here: canonjson.Marshal, sha256.Sum256,
// "sha256:"+hex, exactly as bundle.recordDigest / artifact.ObjectContentHash
// / etc. used to each spell it out independently) and the NEW canonjson.
// Digest — and asserts they are byte-identical, additionally pinning the
// exact golden digest string as a literal so any future drift in either
// Marshal's canonical form or Digest's hash tail fails loudly here first,
// before any of the ten collapsed call sites.
func TestDigest_ByteEquivalentToOldHandRolledTail(t *testing.T) {
	v := digestFixture{Kind: "acceptance_criteria", ID: "ac-2", Text: "shared homes digest collapse"}

	// OLD pattern: hand-rolled inline, exactly as every pre-collapse call
	// site spelled it out.
	data, err := Marshal(v)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	sum := sha256.Sum256(data)
	oldDigest := "sha256:" + hex.EncodeToString(sum[:])

	// NEW pattern: the collapsed helper.
	newDigest, err := Digest(v)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}

	if newDigest != oldDigest {
		t.Fatalf("Digest = %q, old hand-rolled pattern = %q: not byte-equivalent", newDigest, oldDigest)
	}

	const golden = "sha256:c565185ee3a4278abe9b3d829d296799d1ff08d5ef04be1c7bbabad6b1fd77d7"
	if newDigest != golden {
		t.Fatalf("Digest = %q, want pinned golden %q", newDigest, golden)
	}
}

func TestDigest_DeterministicAcrossCalls(t *testing.T) {
	v := digestFixture{Kind: "constraints", ID: "co-1", Text: "no network in any test"}
	first, err := Digest(v)
	if err != nil {
		t.Fatalf("Digest: %v", err)
	}
	for i := 0; i < 10; i++ {
		got, err := Digest(v)
		if err != nil {
			t.Fatalf("Digest (call %d): %v", i, err)
		}
		if got != first {
			t.Fatalf("Digest call %d = %q, want %q (not deterministic)", i, got, first)
		}
	}
}

// --- negative / error-path tests ---

func TestDigest_UnsupportedType(t *testing.T) {
	_, err := Digest(map[string]interface{}{"bad": make(chan int)})
	if err == nil {
		t.Fatal("Digest: want error for unmarshalable value (chan), got nil")
	}
}

func TestMarshal_UnsupportedType(t *testing.T) {
	_, err := Marshal(map[string]interface{}{"bad": make(chan int)})
	if err == nil {
		t.Fatal("Marshal: want error for unmarshalable value (chan), got nil")
	}
}

func TestMarshal_FunctionValue(t *testing.T) {
	_, err := Marshal(func() {})
	if err == nil {
		t.Fatal("Marshal: want error for function value, got nil")
	}
}

func TestMarshal_NaNRejected(t *testing.T) {
	_, err := Marshal(map[string]interface{}{"n": nanFloat()})
	if err == nil {
		t.Fatal("Marshal: want error for NaN float, got nil")
	}
}

func nanFloat() float64 {
	var zero float64
	return zero / zero
}
