package dex

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/disclosureview"
)

// TestBuild_DisclosuresPage: the dex ships the read-only edition of the
// disclosures view (spec/disclosures-panel ac-3). The fixture repo is a
// bare clone (fixturegit writes no mutable zone), so VL-017's
// disclosed-unproven notice is live for its new-class specs — real
// disclosures for the page to carry.
func TestBuild_DisclosuresPage(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	outDir := t.TempDir()
	ctx := context.Background()
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: outDir}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	page := readFile(t, outDir, "disclosures/index.html")

	t.Run("carries the checkout's live disclosures", func(t *testing.T) {
		for name, want := range map[string]string{
			"shared view container": `<section class="disclosures-view"`,
			"a live VL-017 item":    `<code class="disclosure-source">lint:VL-017</code>`,
			"severity badge":        `<span class="disclosure-severity">disclosed-unproven</span>`,
			"living-gated banner":   "main @",
		} {
			if !strings.Contains(page, want) {
				t.Errorf("disclosures page missing %s: want substring %q", name, want)
			}
		}
	})

	t.Run("renders the one shared markup verbatim (ac-3: no separate logic path)", func(t *testing.T) {
		items, err := disclosureview.Current(ctx, repo.Dir)
		if err != nil {
			t.Fatalf("Current: %v", err)
		}
		want := string(disclosureview.HTML(items, dexDisclosuresNote))
		if !strings.Contains(page, want) {
			t.Fatalf("dex page does not embed the shared view's own output verbatim.\nwant fragment:\n%s\npage:\n%s", want, page)
		}
	})

	t.Run("home links the page", func(t *testing.T) {
		home := readFile(t, outDir, "index.html")
		if !strings.Contains(home, `href="/disclosures/"`) {
			t.Fatal("home page does not link /disclosures/")
		}
	})

	t.Run("read-only: no forms or action buttons", func(t *testing.T) {
		for _, banned := range []string{"<form", "<button", "contenteditable"} {
			if strings.Contains(page, banned) {
				t.Errorf("read-only dex edition carries an editing affordance: %q", banned)
			}
		}
	})
}

// TestBuild_DisclosuresPage_EmptyState: with the mutable zone present and
// no other disclosure live, the page renders the explicit positive claim
// — never a blank region (the story's ac-1 empty-state rule, on the dex
// edition).
func TestBuild_DisclosuresPage_EmptyState(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".verdi", "data", "mutable"), 0o755); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: outDir}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	page := readFile(t, outDir, "disclosures/index.html")
	if strings.Contains(page, "disclosure-item") {
		t.Fatalf("want no items with the mutable zone present:\n%s", page)
	}
	if !strings.Contains(page, "No current disclosures.") {
		t.Fatalf("empty enumeration must render the positive claim:\n%s", page)
	}
}
