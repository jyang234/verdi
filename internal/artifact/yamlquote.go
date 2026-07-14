package artifact

import "encoding/json"

// YAMLDoubleQuote renders s as a YAML double-quoted scalar. This is the
// EMISSION side of the seam whose decode side this package already owns
// (doc.go: internal/artifact is the sole importer of gopkg.in/yaml.v3,
// decoding exclusively through DecodeStrict's restricted dialect):
// hand-rendered frontmatter producers (align, decisionsweep, workbench) need
// to quote arbitrary text — finding notes, judge output, obligation
// titles — that may itself contain quotes, colons, or newlines, into that
// dialect without a second, hand-rolled escaper.
//
// encoding/json's string escaping (\", \\, \n, \t, \r, control chars via
// \u00XX) is a valid subset of YAML double-quoted scalar escaping, so
// json.Marshal on a plain string is a safe, well-tested way to produce a
// YAML double-quoted scalar: the output is always valid YAML.
func YAMLDoubleQuote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		// json.Marshal on a string cannot fail for well-formed UTF-8/any Go
		// string (invalid UTF-8 is replaced, not rejected); this exists only
		// to satisfy err-checking discipline, never to be reached.
		return `""`
	}
	return string(b)
}
