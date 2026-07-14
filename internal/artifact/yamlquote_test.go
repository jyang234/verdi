package artifact

import (
	"encoding/json"
	"testing"
)

// TestYAMLDoubleQuote_ByteEquivalence pins YAMLDoubleQuote's output against
// the old inline pattern (json.Marshal of a string, `""` on error) that
// align/render.go's yamlDQ, decisionsweep/render.go's yamlDQ, and
// workbench/obligationauthor.go's yamlDoubleQuote each hand-rolled before
// this shared home existed (spec/shared-homes ac-3, dc-4's "bit-for-bit
// identical" bar). The old pattern can never itself error on a Go string,
// so this proves equivalence over the values those three call sites
// actually see, not over the unreachable error branch.
func TestYAMLDoubleQuote_ByteEquivalence(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"plain", "hello world"},
		{"embedded double quotes", `she said "hello"`},
		{"single quotes", "it's a test"},
		{"newlines", "line one\nline two\nline three"},
		{"tabs", "col1\tcol2\tcol3"},
		{"unicode accented", "café résumé naïve"},
		{"unicode emoji", "shipped 🚀 done ✅"},
		{"empty string", ""},
		{"leading and trailing spaces", "  padded  "},
		{"looks like YAML", "foo: bar"},
		{"looks like YAML flow mapping", "{ type: link, ref: spec/x#ac-1 }"},
		{"backslashes", `C:\path\to\file`},
		{"mixed backslash and quote", `she said \"hello\" then left`},
		{"carriage return", "line one\r\nline two"},
		{"control character", "bell\x07here"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := YAMLDoubleQuote(tc.in)

			// The old inline pattern, reproduced verbatim: json.Marshal of
			// the plain string, falling back to `""` on error.
			var want string
			b, err := json.Marshal(tc.in)
			if err != nil {
				want = `""`
			} else {
				want = string(b)
			}

			if got != want {
				t.Fatalf("YAMLDoubleQuote(%q) = %q, want %q (old inline pattern)", tc.in, got, want)
			}
		})
	}
}
