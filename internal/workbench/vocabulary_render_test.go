// spec/vocabulary-surfaces ac-2, the boardspecrender surface: the board's
// class tag and terminal status badge render the resolved model's display
// names — through the identical model.DisplayClass/DisplayState lookups
// the CLI half uses — with the bare id kept in every CSS class, testid,
// and data attribute (a rename is display-only, never addressing), and
// byte-identical fallback when no rename (or no model) is present.
package workbench

import (
	"bytes"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// vocabTestModel mirrors internal/model/testdata/vocab-rename.yaml's
// rename set (feature -> Initiative, accepted-pending-build -> Ready to
// build) plus a superseded rename so the terminal badge's one rendering
// site is provable too.
func vocabTestModel() *model.Model {
	return &model.Model{
		Schema: "verdi.model/v1",
		Classes: map[string]model.Class{
			"feature": {Template: "feature.md"},
			"story":   {Parent: "feature", Template: "story.md"},
		},
		Vocabulary: model.Vocabulary{
			Verbs:   map[string]string{"accept": "Sign off"},
			States:  map[string]string{"accepted-pending-build": "Ready to build", "superseded": "Shelved"},
			Classes: map[string]string{"feature": "Initiative"},
		},
	}
}

// TestBoardRender_ClassTagModelVocabulary proves the case-file class tag's
// visible word resolves through DisplayClass while its CSS class and
// testid keep the bare id.
func TestBoardRender_ClassTagModelVocabulary(t *testing.T) {
	proj := &BoardProjection{
		Spec:    "vocab-probe",
		Title:   "Vocab probe",
		Mode:    modeReadOnly,
		Status:  "accepted-pending-build",
		Class:   "feature",
		Problem: "p",
		Outcome: "o",
	}
	proj.applyModelVocabulary(vocabTestModel())

	html := renderBoardRegion(proj, &boardGitState{})
	if !strings.Contains(html, `<span class="case-class-tag case-class-tag--feature" data-testid="case-class-tag">Initiative</span>`) {
		t.Fatalf("board region = %q, want the class tag to read Initiative with the id kept in its CSS class", html)
	}
	if strings.Contains(html, `data-testid="case-class-tag">feature<`) {
		t.Fatal("board region still renders the bare class id as the tag's visible text")
	}
}

// TestBoardRender_ClassTagFallbackUnchanged is the parity half: with no
// model applied (or a model with no renames) the markup is byte-identical
// to today's.
func TestBoardRender_ClassTagFallbackUnchanged(t *testing.T) {
	proj := &BoardProjection{
		Spec:    "vocab-probe",
		Title:   "Vocab probe",
		Mode:    modeReadOnly,
		Status:  "accepted-pending-build",
		Class:   "feature",
		Problem: "p",
		Outcome: "o",
	}
	plain := renderBoardRegion(proj, &boardGitState{})

	enriched := &BoardProjection{
		Spec:    "vocab-probe",
		Title:   "Vocab probe",
		Mode:    modeReadOnly,
		Status:  "accepted-pending-build",
		Class:   "feature",
		Problem: "p",
		Outcome: "o",
	}
	enriched.applyModelVocabulary(model.Canonical())
	if got := renderBoardRegion(enriched, &boardGitState{}); got != plain {
		t.Fatal("canonical model changed the rendered board region; the no-rename path must be byte-identical")
	}
	if !strings.Contains(plain, `data-testid="case-class-tag">feature`) {
		t.Fatalf("board region = %q, want the bare class id with no model applied", plain)
	}
}

// TestBoardRender_TerminalStatusBadgeModelVocabulary proves the board
// head's superseded badge text resolves through DisplayState while
// badge-<id> and the testid keep the bare id.
func TestBoardRender_TerminalStatusBadgeModelVocabulary(t *testing.T) {
	proj := &BoardProjection{
		Spec:   "old-probe",
		Title:  "Old probe",
		Mode:   modeReadOnly,
		Status: "superseded",
		Class:  "feature",
	}
	proj.applyModelVocabulary(vocabTestModel())

	page, err := renderBoardSpecPage(proj, &boardGitState{})
	if err != nil {
		t.Fatalf("renderBoardSpecPage: %v", err)
	}
	if !strings.Contains(string(page), `<span class="badge badge-superseded board-status-badge" data-testid="board-status-badge">Shelved</span>`) {
		t.Fatalf("board page = %q, want the status badge to read Shelved with badge-superseded kept as its CSS class", string(page))
	}
}

// TestWriteStatusChip_ModelVocabulary proves the shared home/directory
// status chip renders the display label while badge-<id> keeps the id —
// and the empty-label fallback is byte-identical to today's chip.
func TestWriteStatusChip_ModelVocabulary(t *testing.T) {
	var renamed bytes.Buffer
	writeStatusChip(&renamed, "accepted-pending-build", "Ready to build")
	if got := renamed.String(); got != `<span class="badge badge-accepted-pending-build">Ready to build</span>` {
		t.Fatalf("renamed chip = %q", got)
	}

	var plain bytes.Buffer
	writeStatusChip(&plain, "accepted-pending-build", "")
	if got := plain.String(); got != `<span class="badge badge-accepted-pending-build">accepted-pending-build</span>` {
		t.Fatalf("fallback chip = %q", got)
	}
}
