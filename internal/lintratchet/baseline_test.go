package lintratchet

import (
	"strings"
	"testing"
)

// TestEncodeBaseline proves the committed baseline's canonical form: one
// entry per key with its count, sorted by (linter, package, message,
// source), object keys sorted, no HTML escaping, and a trailing newline, so
// regenerating it from the same findings always writes the same bytes.
func TestEncodeBaseline(t *testing.T) {
	cases := []struct {
		name   string
		counts Counts
		want   string
	}{
		{name: "empty", counts: Counts{}, want: "{\"findings\":[]}\n"},
		{name: "nil", counts: nil, want: "{\"findings\":[]}\n"},
		{
			name: "sorted by linter, package, message, then source, unescaped",
			counts: Counts{
				{Linter: "noctx", Package: "a", Message: "m", Source: "s"}:                     1,
				{Linter: "errorlint", Package: "b", Message: "m", Source: "s"}:                 2,
				{Linter: "errorlint", Package: "a", Message: "n", Source: "s"}:                 1,
				{Linter: "errorlint", Package: "a", Message: "m", Source: "\tx := a < b && c"}: 3,
				{Linter: "errorlint", Package: "a", Message: "m", Source: "\tw"}:               1,
			},
			want: `{"findings":[` +
				`{"count":1,"linter":"errorlint","message":"m","package":"a","source":"\tw"},` +
				`{"count":3,"linter":"errorlint","message":"m","package":"a","source":"\tx := a < b && c"},` +
				`{"count":1,"linter":"errorlint","message":"n","package":"a","source":"s"},` +
				`{"count":2,"linter":"errorlint","message":"m","package":"b","source":"s"},` +
				`{"count":1,"linter":"noctx","message":"m","package":"a","source":"s"}]}` + "\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := EncodeBaseline(tc.counts)
			if err != nil {
				t.Fatalf("EncodeBaseline: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("EncodeBaseline =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

// TestEncodeBaselineRefusesNonPositiveCounts is EncodeBaseline's negative
// path: a count below one is no allowance at all, and writing it would
// produce a baseline ParseBaseline refuses.
func TestEncodeBaselineRefusesNonPositiveCounts(t *testing.T) {
	for _, n := range []int{0, -1} {
		if _, err := EncodeBaseline(Counts{{Linter: "l", Package: "p", Message: "m"}: n}); err == nil {
			t.Errorf("EncodeBaseline with count %d: nil error, want one", n)
		}
	}
}

// TestParseBaseline is the strict decode of the committed baseline: unknown
// fields, trailing data, a missing field, an empty linter, package, or
// message, a count below one, and a key listed twice are all malformed.
func TestParseBaseline(t *testing.T) {
	k := Key{Linter: "errorlint", Package: "a", Message: "m", Source: "\ts"}
	cases := []struct {
		name    string
		in      string
		want    Counts
		wantErr string
	}{
		{name: "empty", in: `{"findings":[]}`, want: Counts{}},
		{name: "one entry", in: `{"findings":[{"count":2,"linter":"errorlint","message":"m","package":"a","source":"\ts"}]}` + "\n", want: Counts{k: 2}},
		{name: "an empty source line is a key like any other", in: `{"findings":[{"count":1,"linter":"errorlint","message":"m","package":"a","source":""}]}`, want: Counts{{Linter: "errorlint", Package: "a", Message: "m"}: 1}},

		{name: "empty input", in: "", wantErr: "baseline"},
		{name: "truncated", in: `{"findings":[{"count":2,`, wantErr: "baseline"},
		{name: "trailing data", in: `{"findings":[]}{}`, wantErr: "trailing data"},
		{name: "no findings", in: `{}`, wantErr: "findings"},
		{name: "null findings", in: `{"findings":null}`, wantErr: "findings"},
		{name: "an unknown top-level field", in: `{"findings":[],"version":1}`, wantErr: "version"},
		{name: "an unknown entry field", in: `{"findings":[{"count":1,"linter":"l","message":"m","package":"p","source":"s","file":"p/x.go"}]}`, wantErr: "file"},
		{name: "no count", in: `{"findings":[{"linter":"l","message":"m","package":"p","source":"s"}]}`, wantErr: "count"},
		{name: "a zero count", in: `{"findings":[{"count":0,"linter":"l","message":"m","package":"p","source":"s"}]}`, wantErr: "count"},
		{name: "a negative count", in: `{"findings":[{"count":-2,"linter":"l","message":"m","package":"p","source":"s"}]}`, wantErr: "count"},
		{name: "a fractional count", in: `{"findings":[{"count":1.5,"linter":"l","message":"m","package":"p","source":"s"}]}`, wantErr: "baseline"},
		{name: "no linter", in: `{"findings":[{"count":1,"message":"m","package":"p","source":"s"}]}`, wantErr: "linter"},
		{name: "an empty linter", in: `{"findings":[{"count":1,"linter":"","message":"m","package":"p","source":"s"}]}`, wantErr: "linter"},
		{name: "no package", in: `{"findings":[{"count":1,"linter":"l","message":"m","source":"s"}]}`, wantErr: "package"},
		{name: "no message", in: `{"findings":[{"count":1,"linter":"l","package":"p","source":"s"}]}`, wantErr: "message"},
		{name: "no source", in: `{"findings":[{"count":1,"linter":"l","message":"m","package":"p"}]}`, wantErr: "source"},
		{name: "a key listed twice", in: `{"findings":[{"count":1,"linter":"l","message":"m","package":"p","source":"s"},{"count":2,"linter":"l","message":"m","package":"p","source":"s"}]}`, wantErr: "twice"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBaseline([]byte(tc.in))
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("ParseBaseline = %v, nil error; want an error containing %q", got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("ParseBaseline error %q does not contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseBaseline: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("ParseBaseline = %v, want %v", got, tc.want)
			}
			for key, n := range tc.want {
				if got[key] != n {
					t.Fatalf("ParseBaseline[%+v] = %d, want %d", key, got[key], n)
				}
			}
		})
	}
}

// TestBaselineRoundTrip proves EncodeBaseline's output is exactly what
// ParseBaseline reads back.
func TestBaselineRoundTrip(t *testing.T) {
	in := Counts{
		{Linter: "gochecknoglobals", Package: "internal/x", Message: "v is a global variable", Source: "var v = map[string]int{}"}: 1,
		{Linter: "errorlint", Package: ".", Message: "non-wrapping format verb", Source: "\treturn fmt.Errorf(\"<%v>\", err)"}:     13,
	}
	data, err := EncodeBaseline(in)
	if err != nil {
		t.Fatalf("EncodeBaseline: %v", err)
	}
	out, err := ParseBaseline(data)
	if err != nil {
		t.Fatalf("ParseBaseline: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("round trip = %v, want %v", out, in)
	}
	for k, n := range in {
		if out[k] != n {
			t.Fatalf("round trip[%+v] = %d, want %d", k, out[k], n)
		}
	}
}
