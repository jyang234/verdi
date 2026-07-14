package dex

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/forge/fake"
)

// seededSupersessionForge builds the hermetic forge double the
// pending-supersession fold reads open MRs through: one MR open against
// "main" whose source branch carries testdata/dexoverlay's candidate v2
// spec for spec/accepted-pending-build (its manifest amends ac-2 only —
// see the overlay README).
func seededSupersessionForge(t *testing.T) *fake.Forge {
	t.Helper()
	candidate, err := os.ReadFile(filepath.Join(dexOverlayDir, "mr", "accepted-pending-build-v2.spec.md"))
	if err != nil {
		t.Fatalf("reading MR candidate fixture: %v", err)
	}
	f := fake.New()
	f.SeedOpenMR("main", forge.OpenMR{ID: "mr-7", SourceBranch: "design/accepted-pending-build-v2"})
	f.SeedFile("design/accepted-pending-build-v2", ".verdi/specs/active/accepted-pending-build-v2/spec.md", candidate)
	return f
}

// buildV2Site builds the full site over the v2 fixture corpus with the
// seeded forge, once per test.
func buildV2Site(t *testing.T) string {
	t.Helper()
	repo := buildDexFixtureRepo(t)
	outDir := t.TempDir()
	err := Build(context.Background(), Options{
		Root: repo.Dir, OutDir: outDir,
		Forge: seededSupersessionForge(t), DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return outDir
}

// TestBuildV2_FeatureLens is the V1-P8 exit criterion "the feature page
// renders the stub list paired with the computed live mapping under the
// 'acceptance-time plan; current mapping computed below' banner, never the
// frozen stubs alone" (05 §Lenses, feature lens).
func TestBuildV2_FeatureLens(t *testing.T) {
	outDir := buildV2Site(t)
	page := readFile(t, outDir, "a/spec/accepted-pending-build/index.html")

	t.Run("banner carries the exact honesty text", func(t *testing.T) {
		if !strings.Contains(page, `data-testid="acceptance-plan-banner"`) {
			t.Fatalf("feature page missing the acceptance-plan banner; got:\n%s", page)
		}
		if !strings.Contains(page, "acceptance-time plan; current mapping computed below") {
			t.Fatal("banner missing the 05 §Lenses banner text")
		}
	})

	t.Run("stub plan lists every declared stub", func(t *testing.T) {
		if !strings.Contains(page, `data-testid="stub-plan"`) {
			t.Fatal("feature page missing the stub plan")
		}
		for _, slug := range []string{"borrower-update-api", "borrower-update-ui", "borrower-update-audit-log"} {
			if !strings.Contains(page, `data-testid="stub-`+slug+`"`) {
				t.Fatalf("stub plan missing stub-%s", slug)
			}
		}
	})

	t.Run("live mapping names the implementing stories", func(t *testing.T) {
		if !strings.Contains(page, `data-testid="live-mapping"`) {
			t.Fatal("feature page missing the live mapping")
		}
		liveIdx := strings.Index(page, `data-testid="live-mapping"`)
		live := page[liveIdx:]
		for _, ref := range []string{"spec/borrower-update-api", "spec/borrower-update-mobile"} {
			if !strings.Contains(live, ref) {
				t.Fatalf("live mapping missing implementing story %s; got:\n%s", ref, live)
			}
		}
	})

	t.Run("stubs never render without the live mapping", func(t *testing.T) {
		// The pairing law: any page carrying a stub plan carries the live
		// mapping too, checked across the whole built site.
		err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".html") {
				return err
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			s := string(data)
			if strings.Contains(s, `data-testid="stub-plan"`) && !strings.Contains(s, `data-testid="live-mapping"`) {
				t.Errorf("%s renders frozen stubs without the live mapping", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking site: %v", err)
		}
	})

	t.Run("feature with no stubs still pairs an empty plan with the live mapping", func(t *testing.T) {
		// loan-workflow is a round-four feature with no stubs: block; its
		// live mapping still names borrower-update-mobile (implements
		// spec/loan-workflow#ac-1).
		lw := readFile(t, outDir, "a/spec/loan-workflow/index.html")
		if !strings.Contains(lw, `data-testid="acceptance-plan-banner"`) || !strings.Contains(lw, `data-testid="live-mapping"`) {
			t.Fatal("round-four feature without stubs must still render the paired lens section")
		}
		if !strings.Contains(lw, "spec/borrower-update-mobile") {
			t.Fatal("loan-workflow live mapping missing spec/borrower-update-mobile")
		}
	})

	t.Run("grandfathered v0 feature and story pages carry no lens section", func(t *testing.T) {
		for _, rel := range []string{"a/spec/stale-decline/index.html", "a/spec/borrower-update-api/index.html"} {
			p := readFile(t, outDir, rel)
			if strings.Contains(p, `data-testid="stub-plan"`) || strings.Contains(p, `data-testid="acceptance-plan-banner"`) {
				t.Fatalf("%s must not render the feature lens section", rel)
			}
		}
	})
}

var (
	exemptionHeadingRe = regexp.MustCompile(`(\d+) active exemption`)
	exemptionItemRe    = regexp.MustCompile(`data-testid="exemption-\d+"`)
)

// TestBuildV2_ExemptionPages is the V1-P8 exit criterion "the exemption
// page lists the fixture ADR's active exemptions with the exempting specs
// named" (05 §Lenses: computed and countable — "ADR-7: 9 active
// exemptions"; 03 §Exemption audit).
func TestBuildV2_ExemptionPages(t *testing.T) {
	outDir := buildV2Site(t)

	t.Run("exempted ADR page counts and names the exempting spec", func(t *testing.T) {
		page := readFile(t, outDir, "a/adr/0001-outbox-events/exemptions/index.html")
		m := exemptionHeadingRe.FindStringSubmatch(page)
		if m == nil {
			t.Fatalf("exemptions page carries no countable heading; got:\n%s", page)
		}
		stated, _ := strconv.Atoi(m[1])
		items := len(exemptionItemRe.FindAllString(page, -1))
		if stated != items {
			t.Fatalf("stated count %d != listed item count %d (countable law)", stated, items)
		}
		if stated == 0 {
			t.Fatal("fixture ADR must have at least one active exemption")
		}
		if !strings.Contains(page, "spec/accepted-pending-build") {
			t.Fatal("exemption item must name the exempting spec")
		}
		if !strings.Contains(page, "dc-1") {
			t.Fatal("exemption item must name the exempting decision")
		}
	})

	t.Run("unexempted ADR page states zero, never omits", func(t *testing.T) {
		page := readFile(t, outDir, "a/adr/0003-retry-policy/exemptions/index.html")
		m := exemptionHeadingRe.FindStringSubmatch(page)
		if m == nil || m[1] != "0" {
			t.Fatalf("unexempted ADR's page must state '0 active exemptions'; got match %v", m)
		}
		if strings.Contains(page, `data-testid="exemption-1"`) {
			t.Fatal("unexempted ADR's page must list no exemption items")
		}
	})

	t.Run("the ADR permalink page links to its exemption page", func(t *testing.T) {
		page := readFile(t, outDir, "a/adr/0001-outbox-events/index.html")
		if !strings.Contains(page, "/a/adr/0001-outbox-events/exemptions/") {
			t.Fatal("ADR page missing the link to its per-ADR exemption page")
		}
	})
}

// TestBuildV2_LadderBadges is the V1-P8 exit criterion "a story page
// carries a spec-stale badge and a pending-supersession badge on the
// fixture stories that carry those flags" (05 §Lenses story lens;
// §Verdi-dex: "read-only, computed identically to the workbench story
// lens").
func TestBuildV2_LadderBadges(t *testing.T) {
	outDir := buildV2Site(t)

	t.Run("flagged story carries both badges", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/borrower-update-mobile/index.html")
		if !strings.Contains(page, `data-testid="badge-spec-stale"`) {
			t.Fatalf("borrower-update-mobile missing the spec-stale badge; got:\n%s", page)
		}
		if !strings.Contains(page, `data-testid="badge-pending-supersession"`) {
			t.Fatalf("borrower-update-mobile missing the pending-supersession badge; got:\n%s", page)
		}
		// The detail rows disclose WHY (mirroring the closure gate's reasons).
		if !strings.Contains(page, "mr-7") {
			t.Fatal("pending-supersession detail must name the open MR")
		}
	})

	t.Run("unflagged story carries neither badge", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/borrower-update-api/index.html")
		if strings.Contains(page, `data-testid="badge-spec-stale"`) || strings.Contains(page, `data-testid="badge-pending-supersession"`) {
			t.Fatal("borrower-update-api must carry no ladder badge (its edges touch only carried objects; no deviation report)")
		}
	})

	t.Run("nil forge discloses pending-supersession unproven, never renders the badge", func(t *testing.T) {
		repo := buildDexFixtureRepo(t)
		outDir2 := t.TempDir()
		if err := Build(context.Background(), Options{Root: repo.Dir, OutDir: outDir2}); err != nil {
			t.Fatalf("Build: %v", err)
		}
		page := readFile(t, outDir2, "a/spec/borrower-update-mobile/index.html")
		if strings.Contains(page, `data-testid="badge-pending-supersession"`) {
			t.Fatal("without a forge the pending-supersession badge must not render (unproven is not flagged)")
		}
		if !strings.Contains(page, "unproven") {
			t.Fatal("without a forge the story page must disclose pending-supersession as unproven (three-valued honesty, never a silent pass)")
		}
		// spec-stale is tree-computed and must still flag.
		if !strings.Contains(page, `data-testid="badge-spec-stale"`) {
			t.Fatal("spec-stale badge must render regardless of forge availability")
		}
	})
}

// TestBuildV2_ByStoryAxis covers 05 §Verdi-dex's by-story axis: the
// archived quartet (spec, board, rollup, deviation report), with
// layout.json rendered in the board slot for round-four archives and
// board.json for grandfathered v0 archives (00 §Glossary "the quartet";
// 03 §Alignment report, round-four note).
func TestBuildV2_ByStoryAxis(t *testing.T) {
	outDir := buildV2Site(t)

	t.Run("hub lists every archived story", func(t *testing.T) {
		hub := readFile(t, outDir, "by-story/index.html")
		for _, name := range []string{"loan-refi-2023", "refi-rate-check-2024"} {
			if !strings.Contains(hub, "/by-story/"+name+"/") {
				t.Fatalf("by-story hub missing %s", name)
			}
		}
	})

	t.Run("round-four quartet renders layout.json in the board slot", func(t *testing.T) {
		page := readFile(t, outDir, "by-story/refi-rate-check-2024/index.html")
		if !strings.Contains(page, "layout.json") {
			t.Fatalf("round-four quartet page must render the layout.json board slot; got:\n%s", page)
		}
		if !strings.Contains(page, "verdi.boardlayout/v1") {
			t.Fatal("board slot must render the coordinate sidecar's content")
		}
		if strings.Contains(page, "board.json") {
			t.Fatal("a round-four archive has no frozen board.json to render")
		}
		for _, section := range []string{"rollup.json", "Deviation report"} {
			if !strings.Contains(page, section) {
				t.Fatalf("quartet page missing the %s section", section)
			}
		}
	})

	t.Run("grandfathered quartet renders board.json in the board slot", func(t *testing.T) {
		page := readFile(t, outDir, "by-story/loan-refi-2023/index.html")
		if !strings.Contains(page, "board.json") {
			t.Fatal("grandfathered quartet page must render its frozen board.json")
		}
		if !strings.Contains(page, "verdi.board/v1") {
			t.Fatal("board slot must render the frozen board's content")
		}
		if !strings.Contains(page, "grandfathered") {
			t.Fatal("the grandfathered form must be labeled as such (temporal honesty)")
		}
	})

	t.Run("quartet page links the spec permalink", func(t *testing.T) {
		page := readFile(t, outDir, "by-story/refi-rate-check-2024/index.html")
		if !strings.Contains(page, "/a/spec/refi-rate-check-2024/") {
			t.Fatal("quartet page must link the archived spec's permalink")
		}
	})
}

// TestBuildV2_ByteIdenticalRebuild re-proves constitution 1 over the v2
// options surface: same tree + same forge state, twice, byte-identical
// (the V1-P8 exit criterion mirrors v0 Phase 12's determinism exit).
func TestBuildV2_ByteIdenticalRebuild(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	f := seededSupersessionForge(t)

	out1, out2 := t.TempDir(), t.TempDir()
	ctx := context.Background()
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: out1, Forge: f, DefaultBranch: "main"}); err != nil {
		t.Fatalf("first Build: %v", err)
	}
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: out2, Forge: f, DefaultBranch: "main"}); err != nil {
		t.Fatalf("second Build: %v", err)
	}

	h1, n1, err := hashTree(out1)
	if err != nil {
		t.Fatalf("hashing out1: %v", err)
	}
	h2, n2, err := hashTree(out2)
	if err != nil {
		t.Fatalf("hashing out2: %v", err)
	}
	if n1 == 0 || n1 != n2 {
		t.Fatalf("file counts: %d vs %d (want equal, nonzero)", n1, n2)
	}
	if h1 != h2 {
		t.Fatalf("v2 rebuild is not byte-identical: %s vs %s", h1, h2)
	}
}
