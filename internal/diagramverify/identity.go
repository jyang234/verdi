package diagramverify

import "strings"

// ShortName implements spec/verification-extractor dc-2's local identity-
// normalization rule: this package's own small, tested function — never
// verdi-go's frontier.ShortName (CLAUDE.md: never import verdi-go
// packages). It strips a leading "(*pkg.Type)." or "pkg.Type." receiver/
// package prefix by keeping only the substring after fqn's LAST '.', which
// works uniformly for a pointer-receiver method
// ("(*pkg.Type).Method" -> "Method"), a value-receiver method
// ("pkg.Type.Method" -> "Method"), and a bare function
// ("pkg.Function" -> "Function"): in every one of those forms the
// receiver/package prefix ends immediately before the final '.'. A fqn
// with no '.' at all (or the empty string) is returned unchanged.
//
// This is a display-name convenience, not a reimplementation of flowmap's
// own call-graph construction (the parent feature's co-2 forbidden
// reimplementation) — deliberately narrower than flowmap's internal
// collision-disambiguation scheme, disclosed rather than hidden: two truth
// FQNs whose ShortName collides are surfaced as an ambiguity by
// shortNameIndex/Parse, never silently resolved to one.
func ShortName(fqn string) string {
	if i := strings.LastIndex(fqn, "."); i >= 0 {
		return fqn[i+1:]
	}
	return fqn
}

// shortNameIndex groups truth FQNs by their ShortName. A key with more
// than one FQN is AMBIGUOUS (dc-2): a proposal node using that raw id
// cannot be classified with confidence, which Parse (grammar.go) turns
// into a whole-artifact Coverage downgrade rather than a guess.
func shortNameIndex(truthFQNs []string) map[string][]string {
	idx := make(map[string][]string, len(truthFQNs))
	for _, fqn := range truthFQNs {
		s := ShortName(fqn)
		idx[s] = append(idx[s], fqn)
	}
	return idx
}
