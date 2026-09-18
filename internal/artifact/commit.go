package artifact

// ValidCommit reports whether s is a well-formed git commit sha in the
// ONE form a pinned ref accepts: 7-40 lowercase hex characters (commitRe,
// enforced on Ref.Commit by ref.go's validate). Exported so a surface
// that takes a commit OUTSIDE a ref — an MCP tool argument, a CLI flag —
// holds it to exactly the rule `kind/name@commit` is held to, instead of
// copying the pattern into its own package or handing an unvalidated
// string to git as a revision expression.
//
// It lives in its own file rather than beside commitRe in ref.go because
// ref.go's exact bytes are bound by the sealed public-consolidation
// witness (internal/sealedexec/testdata/public-consolidation/checks.json,
// whose whole-document digest is pinned in
// public_consolidation_witness_test.go): editing that file at all reds
// the witness as "stale witness source", and re-binding it is a sealed
// artifact's own change, not this feature's. The witness checks only the
// paths its map names, so a new file in this package is outside it; the
// regex itself is NOT copied here — commitRe stays package-level in
// ref.go and this is the one reference to it.
func ValidCommit(s string) bool {
	return commitRe.MatchString(s)
}
