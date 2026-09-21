// Command seamA is spike/verification-rules oq-2 evidence for "Seam A":
// bindings entries extended to clause fragments. It exercises the REAL
// artifact.ParseRef / artifact.ResolveBindingAC / artifact.DecodeBindings
// functions (never a reimplementation) against a scratch
// verdi.bindings.yaml carrying two-level clause fragments
// (spec/readiness-recovery#co-2/c1..c9), and against the real, committed
// .verdi/specs/active/readiness-recovery/spec.md (read-only) to show
// whether a CONSTRAINT id resolves through the AC-only binding-resolution
// path at all, even at one level.
//
// It also inlines a copy of internal/lint/vl003.go's unexported
// targetACSet (cited by source line in the comment on it below) because
// that function is unexported and this is a different package; the inline
// copy is marked as a replica, not a reimplementation the spike is
// proposing.
//
// Usage: go run ./docs/spikes/verification-rules/_scratch/seamA [root]
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
)

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	fmt.Println("=== step 1: ParseRef on a two-level clause fragment ===")
	twoLevel := "spec/readiness-recovery#co-2/c1"
	ref, err := artifact.ParseRef(twoLevel)
	fmt.Printf("ParseRef(%q) = %+v, err=%v\n", twoLevel, ref, err)

	fmt.Println("\n=== step 2: ParseRef on a single-level constraint fragment ===")
	oneLevel := "spec/readiness-recovery#co-2"
	ref2, err2 := artifact.ParseRef(oneLevel)
	fmt.Printf("ParseRef(%q) = %+v, err=%v\n", oneLevel, ref2, err2)

	fmt.Println("\n=== step 3: ResolveBindingAC on the single-level constraint fragment ===")
	specRef, acID, err3 := artifact.ResolveBindingAC("spec/readiness-recovery", oneLevel)
	fmt.Printf("ResolveBindingAC(%q, %q) = specRef=%q acID=%q err=%v\n", "spec/readiness-recovery", oneLevel, specRef, acID, err3)

	fmt.Println("\n=== step 4: ResolveBindingAC on the two-level clause fragment ===")
	specRef4, acID4, err4 := artifact.ResolveBindingAC("spec/readiness-recovery", twoLevel)
	fmt.Printf("ResolveBindingAC(%q, %q) = specRef=%q acID=%q err=%v\n", "spec/readiness-recovery", twoLevel, specRef4, acID4, err4)

	fmt.Println("\n=== step 5: decode the real readiness-recovery spec.md (read-only) ===")
	specPath := filepath.Join(root, ".verdi", "specs", "active", "readiness-recovery", "spec.md")
	raw, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Printf("READ ERROR: %v\n", err)
		os.Exit(2)
	}
	fm, _, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		fmt.Printf("SPLIT ERROR: %v\n", err)
		os.Exit(2)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		fmt.Printf("DECODE ERROR: %v\n", err)
		os.Exit(2)
	}
	acIDs := map[string]bool{}
	for _, ac := range spec.AcceptanceCriteria {
		acIDs[ac.ID] = true
	}
	fmt.Printf("declared acceptance-criteria ids (AC-only set targetACSet builds, internal/lint/vl003.go:317-327): %v\n", sortedKeys(acIDs))
	declaredAll := artifact.DeclaredObjectIDs(spec)
	fmt.Printf("declared object ids overall (artifact.DeclaredObjectIDs — ac+co+dc+oq): %v\n", sortedKeys(declaredAll))
	fmt.Printf("is %q in the AC-only set VL-003's targetACSet checks against? %v\n", "co-2", acIDs["co-2"])
	fmt.Printf("is %q in DeclaredObjectIDs (what VL-003's checkLink fragment path for links[].ref DOES use)? %v\n", "co-2", declaredAll["co-2"])

	fmt.Println("\n=== step 6: what the VL-003 message would read (replicating checkOneBindingsFile's exact format string) ===")
	if !acIDs["co-2"] {
		fmt.Printf("evidence-for binding %q: ac entry %q names ac %q, which %q does not declare\n", "scratch-seam-a-witness", oneLevel, "co-2", "spec/readiness-recovery")
	}

	fmt.Println("\n=== step 7: DecodeBindings a scratch verdi.bindings.yaml carrying all nine co-2/c1..c9 two-level fragments ===")
	scratch := buildScratchBindingsYAML()
	fmt.Println("--- scratch verdi.bindings.yaml ---")
	fmt.Println(scratch)
	fmt.Println("--- end scratch file ---")
	bs, err := artifact.DecodeBindings([]byte(scratch))
	if err != nil {
		fmt.Printf("DecodeBindings REJECTED: %v\n", err)
	} else {
		fmt.Printf("DecodeBindings ACCEPTED: %+v\n", bs)
	}

	fmt.Println("\n=== step 8: DecodeBindings a scratch file using ONE single-level co-2 fragment (no clause suffix) ===")
	scratchOne := `schema: verdi.bindings/v1
spec: spec/readiness-recovery
bindings:
  - producer: scratch-seam-a-witness-single
    kind: behavioral
    acs:
      - "spec/readiness-recovery#co-2"
`
	fmt.Println(scratchOne)
	bsOne, errOne := artifact.DecodeBindings([]byte(scratchOne))
	if errOne != nil {
		fmt.Printf("DecodeBindings REJECTED: %v\n", errOne)
	} else {
		fmt.Printf("DecodeBindings ACCEPTED (shape-valid): %+v\n", bsOne)
		fmt.Println("(shape-valid at decode; VL-003's checkOneBindingsFile would still separately reject it")
		fmt.Println(" as 'does not declare' via the AC-only targetACSet, per step 6 above — the semantic")
		fmt.Println(" reject happens one layer later than decode, and its message reads as if co-2 were")
		fmt.Println(" entirely undeclared rather than declared-as-a-constraint.)")
	}

	fmt.Println("\n=== step 9: ParseRef on a HYPHEN-joined clause id (alternate encoding, no '/' at all) ===")
	hyphen := "spec/readiness-recovery#co-2-c1"
	refH, errH := artifact.ParseRef(hyphen)
	fmt.Printf("ParseRef(%q) = %+v, err=%v\n", hyphen, refH, errH)
	fmt.Println("(objectIDRe allows repeated -[a-z0-9]+ groups; a hyphen-joined clause id parses today with")
	fmt.Println(" NO ref.go change — it would still need to become a DECLARED object id somewhere, e.g. by")
	fmt.Println(" also registering it in DeclaredObjectIDs, which is a separate design decision this spike")
	fmt.Println(" does not make; see README.md oq-2.)")

	fmt.Println("\n=== step 10: is DeclaredObjectIDs aware of any clause id today (co-2-c1 or co-2/c1)? ===")
	fmt.Printf("is %q in DeclaredObjectIDs? %v (clauses are not top-level objects at all in the current 02 schema)\n", "co-2-c1", declaredAll["co-2-c1"])
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// simple insertion sort, avoids importing sort for a slice this small
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}

func buildScratchBindingsYAML() string {
	s := "schema: verdi.bindings/v1\n"
	s += "spec: spec/readiness-recovery\n"
	s += "bindings:\n"
	s += "  - producer: scratch-seam-a-witness\n"
	s += "    kind: behavioral\n"
	s += "    acs:\n"
	for i := 1; i <= 9; i++ {
		s += fmt.Sprintf("      - \"spec/readiness-recovery#co-2/c%d\"\n", i)
	}
	return s
}
