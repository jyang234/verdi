package workbench

// The wall's guidance for the families the retired wall shell shared with
// the readiness derivation is the derivation's own sentence (SI-338 (1):
// "the wall's guidance sentences move into the derivation and are not
// copied"), now read on the record drawer's Readiness tab.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/store"
)

// TestASDShellGuidanceIsTheDerivationsSentence raises every shared
// family on real stored walls and proves each row's guidance on the
// wall's readiness is readinesspilot.Guidance's sentence for that family,
// byte for byte, with the store's own display word for the spike
// pseudo-class routed in, never a bare vocabulary word. The wall shell
// that derived these rows is retired; the wall's readiness is the record
// drawer's Readiness tab, rendering the loader's facts (SI-368 (32) T1).
func TestASDShellGuidanceIsTheDerivationsSentence(t *testing.T) {
	modelYAML, err := os.ReadFile(filepath.Join("..", "model", "testdata", "vocab-rename.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	renamed := map[string]string{".verdi/model.yaml": string(modelYAML)}
	claim := newTabWall(t, claimWallName, claimWallSpec, renamed)
	// The loader enumerates open question and agent-task stickies.
	claim.postSticky(t, "question", "an open scratch question")
	unshaped := newTabWall(t, unshapedWallName, unshapedWallSpec, renamed)
	criterialess := newCriterialessWall(t, renamed)
	cfg, err := store.Open(claim.root)
	if err != nil {
		t.Fatal(err)
	}
	spike := cfg.Model.DisplayClass("spike")
	if spike == "spike" {
		t.Fatalf("the renamed store speaks the bare spike word; the prose witness would vacuously pass")
	}

	guidance := func(family readinesspilot.GuidanceFamily, facts readinesspilot.GuidanceFacts) string {
		sentence := readinesspilot.Guidance(family, facts)
		if sentence == "" {
			t.Fatalf("readinesspilot.Guidance returned no sentence for family %d", family)
		}
		return sentence
	}
	for _, wall := range []struct {
		w    *tabWall
		want map[string]string
	}{
		{claim, map[string]string{
			"shape/question/oq-1": guidance(readinesspilot.GuidanceQuestion, readinesspilot.GuidanceFacts{Object: "oq-1"}),
			"shape/question/oq-2": guidance(readinesspilot.GuidanceQuestion, readinesspilot.GuidanceFacts{Object: "oq-2", SpikeWord: spike, ClaimingStubs: 2}),
			"shape/board/*":       guidance(readinesspilot.GuidanceScratch, readinesspilot.GuidanceFacts{}),
		}},
		{unshaped, map[string]string{
			"shape/problem":         guidance(readinesspilot.GuidanceProblem, readinesspilot.GuidanceFacts{}),
			"shape/outcome":         guidance(readinesspilot.GuidanceOutcome, readinesspilot.GuidanceFacts{}),
			"shape/question/oq-1":   guidance(readinesspilot.GuidanceQuestion, readinesspilot.GuidanceFacts{Object: "oq-1", SpikeWord: spike, ClaimingStubs: 1}),
			"success/coverage/ac-1": guidance(readinesspilot.GuidanceCoverage, readinesspilot.GuidanceFacts{Object: "ac-1"}),
		}},
		{criterialess, map[string]string{
			"success/criteria": guidance(readinesspilot.GuidanceCriteria, readinesspilot.GuidanceFacts{}),
		}},
	} {
		tab, _ := wall.w.tab(t)
		for id, sentence := range wall.want {
			if id == "shape/board/*" {
				// The scratch board's one open item: shape/board/<kind>/<id>.
				id = ""
				for _, got := range tabConcernIDs(tab) {
					if strings.HasPrefix(got, "shape/board/") {
						id = got
					}
				}
				if id == "" {
					t.Fatalf("%s: the open sticky raised no scratch board row", wall.w.name)
				}
			}
			if got := readTabRow(t, tab, id).Primary; got != sentence {
				t.Errorf("%s: %q guidance = %q, want the derivation's %q", wall.w.name, id, got, sentence)
			}
		}
	}
}

// TestASDShellKeepsNoCopyOfTheSharedSentences scans this package's
// production sources for any string literal carrying a shared sentence's
// fixed text: the wall reads those sentences from readinesspilot.Guidance
// and keeps no copy.
func TestASDShellKeepsNoCopyOfTheSharedSentences(t *testing.T) {
	fragments := []string{
		"typed operation set-problem",
		"typed operation set-outcome",
		"or graduate a decision that answers it",
		"it after acceptance.",
		"scratch never enters the record by itself",
		"typed operation add-ac",
		"graduate a story sticky into a stub claiming",
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	scanned := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		f, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		scanned++
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			for _, fragment := range fragments {
				if strings.Contains(val, fragment) {
					t.Errorf("%s: string literal %q copies the shared guidance fragment %q; call readinesspilot.Guidance instead", fset.Position(lit.Pos()), val, fragment)
				}
			}
			return true
		})
	}
	if scanned == 0 {
		t.Fatal("scanned no production source; the copy check would vacuously pass")
	}
}
