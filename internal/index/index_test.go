package index

import (
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

func TestBuild_Happy(t *testing.T) {
	root := buildSyntheticStore(t)

	ix, err := Build(root)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	wantRefs := []string{
		"adr/0001-a",
		"adr/0002-b",
		"spec/my-spec",
		"svc/svcfix/boundary-contract",
		"svc/svcfix/obligations/audit-before-publish",
		"svc/svcfix/api",
	}
	if ix.Len() != len(wantRefs) {
		t.Fatalf("Len() = %d, want %d (entries: %+v)", ix.Len(), len(wantRefs), ix.All())
	}
	for _, ref := range wantRefs {
		if _, ok := ix.Get(ref); !ok {
			t.Errorf("Build: ref %q not indexed", ref)
		}
	}

	// board.json must never be indexed as its own artifact.
	if _, ok := ix.Get("board.json"); ok {
		t.Fatal("board.json was indexed as an artifact")
	}
}

func TestBuild_Negative(t *testing.T) {
	t.Run("nonexistent root", func(t *testing.T) {
		if _, err := Build("/does/not/exist/at/all"); err == nil {
			t.Fatal("Build(nonexistent root): want error, got nil")
		}
	})

	t.Run("malformed frontmatter fails loudly", func(t *testing.T) {
		root := t.TempDir()
		writeIndexFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
		writeIndexFile(t, root, ".verdi/adr/0001-a.md", "---\nid: adr/0001-a\nkind: adr\ntitle: x\nstatus: bogus-status\nowners: [x]\n---\nbody\n")
		if _, err := Build(root); err == nil {
			t.Fatal("Build(malformed adr status): want error, got nil")
		}
	})

	t.Run("duplicate ref", func(t *testing.T) {
		root := t.TempDir()
		writeIndexFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
		writeIndexFile(t, root, ".verdi/adr/0001-a.md", syntheticADR0001)
		writeIndexFile(t, root, ".verdi/adr/0001-a-again.md", syntheticADR0001) // same id: adr/0001-a
		if _, err := Build(root); err == nil {
			t.Fatal("Build(duplicate ref): want error, got nil")
		}
	})

	t.Run("malformed .flowmap.yaml", func(t *testing.T) {
		root := t.TempDir()
		writeIndexFile(t, root, ".verdi/verdi.yaml", "schema: verdi.layout/v1\n")
		writeIndexFile(t, root, "svcfix/.flowmap.yaml", "service: &s x\nother: *s\n")
		if _, err := Build(root); err == nil {
			t.Fatal("Build(malformed .flowmap.yaml): want error, got nil")
		}
	})
}

func TestIndex_Get(t *testing.T) {
	ix, err := Build(buildSyntheticStore(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	e, ok := ix.Get("adr/0001-a")
	if !ok {
		t.Fatal("Get(adr/0001-a): not found")
	}
	if e.Title != "ADR one" {
		t.Fatalf("Get(adr/0001-a).Title = %q, want %q", e.Title, "ADR one")
	}

	if _, ok := ix.Get("adr/does-not-exist"); ok {
		t.Fatal("Get(adr/does-not-exist): want not-found, got a hit")
	}
}

func TestIndex_Backlinks(t *testing.T) {
	ix, err := Build(buildSyntheticStore(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	bl := ix.Backlinks("adr/0001-a")
	if len(bl) != 1 || bl[0].From != "adr/0002-b" || bl[0].Type != "superseded-by" {
		t.Fatalf("Backlinks(adr/0001-a) = %+v, want one superseded-by from adr/0002-b", bl)
	}

	// impacted-by backlink recorded even though svc/svcfix/boundary-contract
	// IS a real indexed entry here (proving the resolved case works, not
	// just the dangling case the golden test covers).
	bl = ix.Backlinks("svc/svcfix/boundary-contract")
	if len(bl) != 1 || bl[0].From != "spec/my-spec" || bl[0].Type != "impacted-by" {
		t.Fatalf("Backlinks(svc/svcfix/boundary-contract) = %+v, want one impacted-by from spec/my-spec", bl)
	}

	// A ref with no incoming links returns an empty (nil) slice, not an error.
	if bl := ix.Backlinks("adr/0002-b"); len(bl) != 0 {
		t.Fatalf("Backlinks(adr/0002-b) = %+v, want none", bl)
	}
}

// TestBuildBacklinks_ResolvesAndExempts is D-7's regression guard: 02 §Link
// taxonomy's inverse-of column lists resolved-by and exempted-by, which were
// missing from inverseOf. A spike's `resolves` edge and a decision's
// `exempts` edge must now invert into computed backlinks on their targets.
func TestBuildBacklinks_ResolvesAndExempts(t *testing.T) {
	entries := []*Entry{
		{Ref: "spec/spike", Kind: "spec", Links: []artifact.Link{
			{Type: artifact.LinkResolves, Ref: "spec/feature#oq-1"},
		}},
		{Ref: "spec/scoped", Kind: "spec", Links: []artifact.Link{
			{Type: artifact.LinkExempts, Ref: "adr/0007-policy"},
		}},
	}
	bl := buildBacklinks(entries)

	if got := bl["spec/feature#oq-1"]; len(got) != 1 || got[0].From != "spec/spike" || got[0].Type != "resolved-by" {
		t.Fatalf("backlinks[spec/feature#oq-1] = %+v, want one resolved-by from spec/spike", got)
	}
	if got := bl["adr/0007-policy"]; len(got) != 1 || got[0].From != "spec/scoped" || got[0].Type != "exempted-by" {
		t.Fatalf("backlinks[adr/0007-policy] = %+v, want one exempted-by from spec/scoped", got)
	}
}

func TestIndex_All(t *testing.T) {
	ix, err := Build(buildSyntheticStore(t))
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	all := ix.All()
	if len(all) != ix.Len() {
		t.Fatalf("All() returned %d entries, Len() = %d", len(all), ix.Len())
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Ref >= all[i].Ref {
			t.Fatalf("All() not sorted by Ref: %q then %q", all[i-1].Ref, all[i].Ref)
		}
	}
}
