package featurecoverage

import "github.com/jyang234/verdi/internal/artifact"

// StubDecls builds Compute's stub input from a decoded feature's declared
// stubs: every non-spike stub, in declared order, with a copy of the
// criteria it lists. A spike stub resolves open questions, never
// acceptance criteria, so it never covers one. This is the wall's own
// stub-to-declaration step, shared so the readiness loader's coverage
// family (SI-338 (4)) reads "no stub" through the same rule rather than a
// second copy of it. Never nil.
func StubDecls(stubs []artifact.Stub) []StubDecl {
	out := make([]StubDecl, 0, len(stubs))
	for _, st := range stubs {
		if st.Spike {
			continue
		}
		out = append(out, StubDecl{Slug: st.Slug, AcceptanceCriteria: append([]string(nil), st.AcceptanceCriteria...)})
	}
	return out
}
