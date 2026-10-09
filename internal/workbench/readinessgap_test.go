package workbench

// TestReadinessTab_LoaderParityNoGaps is spec/wall-strip-and-drawer-v2
// ac-7's witness (obligation ac-7--static; dc-4; SI-368 (4)). It closes
// readiness-recovery ac-5's committed gap witness, which listed the wall
// shell's families against the continuous loader's as "a GAP LIST, not
// parity": the wall shell's own derivation is retired, and the record
// drawer's Readiness tab renders the per-request loader's snapshot from
// one load, so the tab's facts are the loader's facts for the same spec
// and head and no fixture can show a gap. The proof runs over the
// claim-wall fixture through the production loader and over the
// readiness page's fixtures through the tab's route; the committed
// golden's gap lists are empty; and a static check over this package's
// production sources proves the shell's derivation, its renderers and the
// capabilities consultation that fed it (F3CR-7) are gone, with no
// declaration and no caller left. ac-7's "for the e2e harness fixture" is
// proven structurally (SI-368 (4)): the harness provisioner is
// cmd/e2eharness's package main, which a Go test cannot import (SI-209).

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	stdhtml "html"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// addClaimWallManifest commits a minimal .verdi/verdi.yaml onto the
// claim-wall fixture's already-checked-out design branch:
// newClaimWallFixture (boardspecasd_test.go) never writes one, and the
// readiness loader opens the store to resolve the operating model.
func addClaimWallManifest(t *testing.T, root string) {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(root, ".verdi", "verdi.yaml")
	if err := os.WriteFile(path, []byte("schema: verdi.layout/v1\n"), 0o644); err != nil {
		t.Fatalf("writing verdi.yaml: %v", err)
	}
	if err := gitx.AddAll(ctx, root); err != nil {
		t.Fatalf("git add verdi.yaml: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, root, "fixture: add verdi.yaml for the readiness loader"); err != nil {
		t.Fatalf("committing verdi.yaml: %v", err)
	}
}

// family reduces one concern id to its family: the first two path
// segments, with any remaining tail collapsed to the literal "<id>"
// (e.g. "shape/question/oq-1" -> "shape/question/<id>",
// "success/blocker/x/y" -> "success/blocker/<id>"). shape/board/<kind>/<id>
// rows keep three segments instead of two — the item kind (question,
// agent-task) stays a distinguishing part of the family; only the
// trailing item id collapses ("shape/board/question/oq-2" ->
// "shape/board/question/<id>"). An id with no tail beyond the kept
// segments (e.g. "context/verdict", "shape/board" itself) is returned
// unchanged.
func family(id string) string {
	parts := strings.Split(id, "/")
	keep := 2
	if len(parts) >= 4 && parts[0] == "shape" && parts[1] == "board" {
		keep = 3
	}
	if len(parts) <= keep {
		return id
	}
	return strings.Join(parts[:keep], "/") + "/<id>"
}

// readinessGapCapsBridge is the fixture design bridge (R-RR1-20): the wall
// is served with the design bridge wired, as `verdi serve` always wires
// it, never a bare server, with the Mutable:true capabilities posture
// asdcorrection_test.go scripts.
func readinessGapCapsBridge() *scriptedCapsBridge {
	return &scriptedCapsBridge{script: func(int) (DesignReadOutcome, *DesignCapabilitiesView) {
		return DesignReadOutcome{JSON: []byte(`{}`)}, &DesignCapabilitiesView{Mutable: true, PolicyMode: "draft-write", PolicyDigest: "sha256:caps-gap"}
	}}
}

// paritySection is one fixture's family gap lists: the families the tab
// renders that the loader did not produce (wall-only), the loader's
// families the tab does not render (loader-only), and the families whose
// blocking flag differs between them.
type paritySection struct {
	Fixture                        string
	WallOnly, LoaderOnly, Disagree []string
}

// renderReadinessParity renders the witness's exact committed text: the
// header, then each fixture's three sorted, indented family lists.
func renderReadinessParity(sections []paritySection) string {
	var b strings.Builder
	b.WriteString("# readiness parity — the record drawer's Readiness tab versus the per-request loader (spec/wall-strip-and-drawer-v2 ac-7, dc-4), closing readiness-recovery ac-5's gap list\n")
	b.WriteString("# This is PARITY: every list below is empty. The tab renders the loader's snapshot from one load (SI-368 (4)), so a family listed here is a gap, and a change here must be deliberate.\n")
	b.WriteString("# Fixtures: the claim-wall Go fixture through the production loader, and the readiness page's fixtures through the tab's route. ac-7's \"for the e2e harness fixture\" is proven structurally (SI-368 (4)): the harness provisioner is cmd/e2eharness's package main and unimportable from a Go test (SI-209).\n")
	b.WriteString("# The wall shell's own derivation is retired: TestReadinessTab_LoaderParityNoGaps proves no production source declares or calls it. Each family only it produced has a home (SI-368 (3), (32)); success/evidence/<ac> cannot arise on a decoded spec, so its disclosed loss is vacuous.\n")
	for _, s := range sections {
		fmt.Fprintf(&b, "%s:\n", s.Fixture)
		for _, list := range []struct {
			name     string
			families []string
		}{{"wall-only", s.WallOnly}, {"loader-only", s.LoaderOnly}, {"blocking-disagreement", s.Disagree}} {
			fmt.Fprintf(&b, "  %s:\n", list.name)
			for _, f := range list.families {
				fmt.Fprintf(&b, "    %s\n", f)
			}
		}
	}
	return b.String()
}

// tabParity compares one tab body with the loader's snapshot it rendered:
// the same concerns, each exactly once and no other, each row's facts the
// loader's own — state, step, primary line, fact, blocking flag, timing
// and witnesses — and the same steps, states and focus; then it reduces
// both to families for the gap lists.
func tabParity(t *testing.T, fixture, tab string, snap readinesspilot.Snapshot) paritySection {
	t.Helper()
	loaderIDs := map[string]bool{}
	for _, c := range snap.AllConcerns {
		loaderIDs[c.ID] = true
	}
	tabIDs := tabConcernIDs(tab)
	for _, id := range tabIDs {
		if !loaderIDs[id] {
			t.Errorf("%s: the tab renders concern %s, which the loader did not produce", fixture, id)
		}
	}
	if len(tabIDs) != len(snap.AllConcerns) {
		t.Errorf("%s: the tab renders %d concern rows, the loader's snapshot has %d", fixture, len(tabIDs), len(snap.AllConcerns))
	}

	wallFamilies, wallBlocking := map[string]bool{}, map[string]bool{}
	for _, c := range snap.AllConcerns {
		row := readTabRow(t, tab, c.ID)
		primary := c.Guidance
		if primary == "" {
			primary = c.Summary
		}
		witnesses := append([]string(nil), c.Witnesses...)
		want := tabRow{
			State: string(c.State), Area: string(c.Area), Primary: primary, Fact: c.Summary,
			Blocking: strconv.FormatBool(c.Blocking), Timing: string(c.Timing),
			TargetKind: row.TargetKind, Target: row.Target, Witnesses: witnesses,
		}
		if len(want.Witnesses) == 0 {
			want.Witnesses = nil
		}
		if !reflect.DeepEqual(row, want) {
			t.Errorf("%s: concern %s's facts in the tab differ from the loader's:\n tab: %+v\nwant: %+v", fixture, c.ID, row, want)
		}
		f := family(c.ID)
		wallFamilies[f] = true
		if row.Blocking == "true" {
			wallBlocking[f] = true
		}
	}

	stations := regexp.MustCompile(`<li class="readiness-station[^"]*" data-area-id="([^"]+)" data-state="([^"]+)"`).FindAllStringSubmatch(tab, -1)
	if len(stations) != len(snap.Areas) {
		t.Errorf("%s: the tab's stepper has %d steps, the loader's %d", fixture, len(stations), len(snap.Areas))
	}
	for i := 0; i < len(stations) && i < len(snap.Areas); i++ {
		if stations[i][1] != string(snap.Areas[i].ID) || stations[i][2] != string(snap.Areas[i].State) {
			t.Errorf("%s: step %d is %s %s, the loader's %s %s", fixture, i+1, stations[i][1], stations[i][2], snap.Areas[i].ID, snap.Areas[i].State)
		}
	}
	if got := tabFocus(t, tab); got != string(snap.CurrentFocus) {
		t.Errorf("%s: the tab's focus is %q, the loader's %q", fixture, got, snap.CurrentFocus)
	}
	if !strings.Contains(tab, `data-readiness-stale="1"`) || !strings.Contains(tab, stdhtml.EscapeString(snap.StaleNotice)) {
		t.Errorf("%s: the tab lacks the loader's per-request stamp %q", fixture, snap.StaleNotice)
	}

	loaderFamilies, loaderBlocking := map[string]bool{}, map[string]bool{}
	for _, c := range snap.AllConcerns {
		f := family(c.ID)
		loaderFamilies[f] = true
		if c.Blocking {
			loaderBlocking[f] = true
		}
	}
	// The tab's families come from the rows it renders, not the loader's.
	tabFamilies := map[string]bool{}
	for _, id := range tabIDs {
		tabFamilies[family(id)] = true
	}
	section := paritySection{Fixture: fixture}
	for f := range tabFamilies {
		if !loaderFamilies[f] {
			section.WallOnly = append(section.WallOnly, f)
		}
	}
	for f := range loaderFamilies {
		if !tabFamilies[f] {
			section.LoaderOnly = append(section.LoaderOnly, f)
		}
		if wallFamilies[f] && wallBlocking[f] != loaderBlocking[f] {
			section.Disagree = append(section.Disagree, f)
		}
	}
	sort.Strings(section.WallOnly)
	sort.Strings(section.LoaderOnly)
	sort.Strings(section.Disagree)
	return section
}

// retiredShellSymbols is every identifier of the wall shell's own
// derivation, its renderers and the capabilities consultation that fed it
// (deriveASDShell, assembleASDShell and the shell model; writeASDConcern
// and its chip; the capabilities memo — F3CR-7). None may be declared or
// referenced by this package's production sources.
func retiredShellSymbols() []string {
	return []string{
		"deriveASDShell", "assembleASDShell", "asdShell", "asdShellInput", "asdConcern", "asdArea", "asdAreaID",
		"asdAreaOrder", "asdAreaLabels", "asdAreaShape", "asdAreaSuccess", "asdAreaContext", "asdAreaReview",
		"asdStateProven", "asdStateViolated", "asdStateUnproven", "asdObjectFact", "asdACFact",
		"writeASDConcern", "writeASDState", "asdPlainState", "asdAreaAfter", "policyEditingClause",
		"cachedCapabilities", "policyStamp", "capsCacheEntry",
	}
}

// retiredShellFields is every struct field that carried the shell or its
// capabilities consultation, by the struct that held it.
func retiredShellFields() map[string][]string {
	return map[string][]string{
		"asdView":         {"Shell", "Caps", "CapsFailure", "DesignWired"},
		"boardSpecServer": {"capsMu", "capsCache"},
	}
}

// shellDerivationLeftovers parses every production .go file in dir and
// returns each retired symbol it declares or references and each retired
// field its structs still declare, by position, and how many files it
// read; live names the declarations the scan must find, so a scan that
// reads nothing cannot pass.
func shellDerivationLeftovers(t *testing.T, dir string, live []string) ([]string, int) {
	t.Helper()
	retired := map[string]bool{}
	for _, s := range retiredShellSymbols() {
		retired[s] = true
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	declared := map[string]bool{}
	var found []string
	parsed := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		parsed++
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil {
					declared[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						declared[ts.Name.Name] = true
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				if retired[n.Name] {
					found = append(found, fset.Position(n.Pos()).String()+": "+n.Name)
				}
			case *ast.TypeSpec:
				st, ok := n.Type.(*ast.StructType)
				if !ok {
					return true
				}
				for _, field := range retiredShellFields()[n.Name.Name] {
					for _, fl := range st.Fields.List {
						for _, fn := range fl.Names {
							if fn.Name == field {
								found = append(found, fset.Position(fn.Pos()).String()+": "+n.Name.Name+"."+field)
							}
						}
					}
				}
			}
			return true
		})
	}
	for _, name := range live {
		if !declared[name] {
			t.Fatalf("the scan found no declaration of %s; it would vacuously pass", name)
		}
	}
	return found, parsed
}

func TestReadinessTab_LoaderParityNoGaps(t *testing.T) {
	var sections []paritySection

	// The claim wall, through the production loader: the tab's facts are
	// the loader's facts for the same spec and the same head.
	claim := newTabWall(t, claimWallName, claimWallSpec, nil)
	tab, snap := claim.tab(t)
	head := gitOut(t, claim.root, "rev-parse", "HEAD")
	if snap.TargetRef != "spec/"+claimWallName || snap.Head != head || !strings.Contains(snap.StaleNotice, head) {
		t.Fatalf("the loader derived %s at %s (%q), want spec/%s at HEAD %s", snap.TargetRef, snap.Head, snap.StaleNotice, claimWallName, head)
	}
	sections = append(sections, tabParity(t, "claim-wall", tab, snap))

	// The readiness page's fixtures, through the tab's route: one load each.
	for _, fixture := range []struct {
		name string
		snap func() readinesspilot.Snapshot
	}{
		{"readiness-page/mixed", readinessFixture},
		{"readiness-page/with-role", readinessWithRoleFixture},
		{"readiness-page/all-proven", readinessAllProvenFixture},
		{"readiness-page/last-step", readinessLastStepFixture},
	} {
		loader := &countingReadinessLoader{snap: fixture.snap()}
		root := t.TempDir()
		name := strings.TrimPrefix(loader.snap.TargetRef, "spec/")
		writeReadinessTabSpec(t, root, name)
		rec := tabGet(t, t.Context(), NewHandlerWith(root, Deps{ReadinessLoader: loader}), "/board/spec/"+name+"/readiness")
		if rec.Code != http.StatusOK || loader.calls != 1 {
			t.Fatalf("%s: the tab = %d after %d loads, want 200 after one", fixture.name, rec.Code, loader.calls)
		}
		sections = append(sections, tabParity(t, fixture.name, rec.Body.String(), fixture.snap()))
	}

	goldenPath := filepath.Join("testdata", "readiness-gap.golden")
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden %s: %v", goldenPath, err)
	}
	if got := renderReadinessParity(sections); got != string(want) {
		t.Fatalf("the parity witness does not match the committed golden %s.\n--- got ---\n%s--- want ---\n%s", goldenPath, got, string(want))
	}
	for _, s := range sections {
		if len(s.WallOnly)+len(s.LoaderOnly)+len(s.Disagree) != 0 {
			t.Errorf("%s lists a gap: wall-only %q, loader-only %q, blocking-disagreement %q", s.Fixture, s.WallOnly, s.LoaderOnly, s.Disagree)
		}
	}

	t.Run("the wall shell's own derivation is gone", func(t *testing.T) {
		found, parsed := shellDerivationLeftovers(t, ".", []string{"policyGuideFor", "renderReadinessTab", "reviewAcceptanceFor", "asdView"})
		if parsed == 0 {
			t.Fatal("scanned no production source; the static check would vacuously pass")
		}
		for _, f := range found {
			t.Errorf("the retired wall shell derivation survives at %s", f)
		}
	})

	t.Run("success/evidence's disclosed loss is vacuous", func(t *testing.T) {
		// The shell raised success/evidence/<ac> for a criterion declaring
		// no evidence kind. Decode refuses such a criterion
		// (artifact.AcceptanceCriterion.Validate), so the family cannot
		// arise on a stored spec (SI-368 (32) T4) ...
		fm, _, err := artifact.SplitFrontmatter([]byte(claimWallSpec))
		if err != nil {
			t.Fatal(err)
		}
		bare := strings.Replace(string(fm), "evidence: [attestation]", "evidence: []", 1)
		if bare == string(fm) {
			t.Fatal("the claim wall's criterion moved: this case never built an evidence-less criterion")
		}
		if _, err := artifact.DecodeSpec([]byte(bare)); err == nil || !strings.Contains(err.Error(), "declares no expected evidence kind") {
			t.Fatalf("DecodeSpec(an evidence-less criterion) err = %v, want the refusal", err)
		}
		// ... and every criterion that can arise shows its evidence state in
		// its card's obligation rows: one row per declared kind.
		rec := tabGet(t, t.Context(), claim.h, "/board/spec/"+claimWallName)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET the claim wall = %d", rec.Code)
		}
		page := rec.Body.String()
		if !strings.Contains(page, `data-testid="obligations-ac-1"`) || strings.Count(page, `data-obligation-kind="attestation"`) != 1 {
			t.Fatalf("the claim wall's ac-1 card does not carry one obligation row per declared evidence kind")
		}
	})
}
