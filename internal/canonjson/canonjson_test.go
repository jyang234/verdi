package canonjson

import (
	"bytes"
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

// --- negative / error-path tests ---

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
