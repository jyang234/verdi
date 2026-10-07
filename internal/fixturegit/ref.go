package fixturegit

import "testing"

// zeroOID is git's "no object" id, passed as update-ref's expected old
// value so that CreateRef only ever creates a ref, never moves one.
const zeroOID = "0000000000000000000000000000000000000000"

// CreateRef creates ref, a full refname in any namespace, pointing at
// commit, without touching HEAD, the index or the working tree: `git
// update-ref <ref> <commit> <zero-oid>`, which fails if ref already exists.
// It fails the calling test on any git error.
//
// It is how a test seeds a ref the product never creates itself, such as
// the refs/remotes/origin/* a fetch would leave, with no remote and no
// network. gitx.UpdateRef creates only branches, through `git branch`, so
// fixtures seed every other namespace here (ledger SI-359 (5a)). This
// package is test tooling outside the verdi binary, so update-ref never
// enters production code.
func CreateRef(t testing.TB, dir, ref, commit string) {
	t.Helper()
	runGit(t, dir, nil, "update-ref", ref, commit, zeroOID)
}
