package workbench

// The wall shell's guidance for the families it shares with the readiness
// derivation is the derivation's own sentence (SI-338 (1): "the wall's
// guidance sentences move into the derivation and are not copied").

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
)

// TestASDShellGuidanceIsTheDerivationsSentence derives the wall shell over
// inputs that raise every shared family and proves each row's guidance is
// readinesspilot.Guidance's sentence for that family, byte for byte.
func TestASDShellGuidanceIsTheDerivationsSentence(t *testing.T) {
	in := asdShellInput{
		ProblemPresent:  false,
		OutcomePresent:  false,
		OpenQuestions:   []asdObjectFact{{ID: "oq-1", Text: "t1"}, {ID: "oq-2", Text: "t2", ClaimedBySlugs: []string{"a-spike", "b-spike"}}, {ID: "oq-3", Text: "t3", ClaimedBySlugs: []string{"c-spike"}}},
		OpenStickyCount: 2,
		UncoveredACs:    []string{"ac-4"},
		Mode:            "authoring",
		Branch:          "design/x",
		StateFormal:     "proposed",
		SpikeWord:       "probe",
	}
	want := map[string]string{
		"shape/problem":         readinesspilot.Guidance(readinesspilot.GuidanceProblem, readinesspilot.GuidanceFacts{}),
		"shape/outcome":         readinesspilot.Guidance(readinesspilot.GuidanceOutcome, readinesspilot.GuidanceFacts{}),
		"shape/question/oq-1":   readinesspilot.Guidance(readinesspilot.GuidanceQuestion, readinesspilot.GuidanceFacts{Object: "oq-1"}),
		"shape/question/oq-2":   readinesspilot.Guidance(readinesspilot.GuidanceQuestion, readinesspilot.GuidanceFacts{Object: "oq-2", SpikeWord: "probe", ClaimingStubs: 2}),
		"shape/question/oq-3":   readinesspilot.Guidance(readinesspilot.GuidanceQuestion, readinesspilot.GuidanceFacts{Object: "oq-3", SpikeWord: "probe", ClaimingStubs: 1}),
		"shape/board":           readinesspilot.Guidance(readinesspilot.GuidanceScratch, readinesspilot.GuidanceFacts{}),
		"success/criteria":      readinesspilot.Guidance(readinesspilot.GuidanceCriteria, readinesspilot.GuidanceFacts{}),
		"success/coverage/ac-4": readinesspilot.Guidance(readinesspilot.GuidanceCoverage, readinesspilot.GuidanceFacts{Object: "ac-4"}),
	}
	got := map[string]string{}
	for _, c := range deriveASDShell(in).All {
		got[c.ID] = c.Guidance
	}
	for id, sentence := range want {
		if sentence == "" {
			t.Fatalf("readinesspilot.Guidance returned no sentence for %q", id)
		}
		if got[id] != sentence {
			t.Errorf("wall shell %q guidance = %q, want the derivation's %q", id, got[id], sentence)
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
