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

	"github.com/jyang234/verdi/internal/artifactview"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/model"
)

// vocabTestModel mirrors internal/model/testdata/vocab-rename.yaml's
// rename set (feature -> Initiative, accepted-pending-build -> Ready to
// build) plus a superseded rename so the terminal badge's one rendering
// site is provable too, plus story/spike renames so every class-word
// PROSE site (the vocabulary-prose closure: stub labels, the Instantiate
// button, the guide note, the yarn key, the oq-claims chip, dialog copy)
// is provable with words distinct from the bare ids — "spike" renaming
// through Vocabulary.Classes is exactly the variant-marker pseudo-class
// treatment applyModelVocabulary established.
func vocabTestModel() *model.Model {
	return &model.Model{
		Schema: "verdi.model/v1",
		Classes: map[string]model.Class{
			"feature": {Template: "feature.md"},
			"story":   {Parent: "feature", Template: "story.md"},
		},
		Vocabulary: model.Vocabulary{
			Verbs:  map[string]string{"accept": "Sign off"},
			States: map[string]string{"accepted-pending-build": "Ready to build", "superseded": "Shelved"},
			Classes: map[string]string{
				"feature": "Initiative",
				"story":   "Change Request",
				"spike":   "Deep Dive",
			},
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

// vocabProseProjection is the vocabulary-prose closure's render fixture:
// a feature wall exercising every class-word PROSE site the region
// renderer owns — stub cards (story + spike), the sealed wall's
// Instantiate affordances, the oq multi-claim chip, scoping yarn (the
// yarn key's planning meanings), a story/spike proto-sticky beside a
// plain comment sticky, and (in authoring) the four-move guide.
func vocabProseProjection(mode boardModeKind) *BoardProjection {
	return &BoardProjection{
		Spec:    "vocab-probe",
		Title:   "Vocab probe",
		Mode:    mode,
		Status:  "accepted-pending-build",
		Class:   "feature",
		Problem: "p",
		Outcome: "o",
		Cards: []cardView{
			{ID: "ac-1", Kind: "acceptance-criterion", Text: "a"},
			{ID: "oq-1", Kind: "open-question", Text: "q"},
		},
		ACCoverage: map[string]int{"ac-1": 1},
		OQClaims:   map[string]int{"oq-1": 2},
		StubViews:  []StubView{{Slug: "alpha"}, {Slug: "beta", Spike: true}},
		Edges: []edgeView{
			{Type: "covers", From: "stub:alpha", To: "ac-1", Layer: "scoping"},
			{Type: "resolves", From: "stub:beta", To: "oq-1", Layer: "scoping"},
		},
		Stickies: []scratchStickyView{
			{ID: "a-01", Type: "story", Body: "s", Author: "pm"},
			{ID: "a-02", Type: "comment", Body: "c", Author: "pm"},
		},
		RefCards: []refCardView{
			{Ref: "spec/parent-feature", FeatureHref: "/board/spec/parent-feature"},
			{Ref: "spec/old-feature", FeatureHref: "/a/spec/old-feature", Archived: true},
		},
	}
}

// TestBoardRender_ClassWordProseModelVocabulary proves every class-word
// prose site on the board region resolves through the model (the
// vocabulary-prose closure over the closure-time findings: stub-card
// labels, Instantiate buttons, the guide note, plus the sites the same
// category sweep enumerated — yarn key, oq-claims chip, proto-sticky
// type words) while every identity-layer string provably stays bare.
func TestBoardRender_ClassWordProseModelVocabulary(t *testing.T) {
	proj := vocabProseProjection(modeAuthoring)
	proj.applyModelVocabulary(vocabTestModel())
	html := renderBoardRegion(proj, &boardGitState{})

	for _, want := range []string{
		// Stub-card kind labels (finding site boardspecrender.go:358).
		`<span class="card-kind-label">Change Request stub</span>`,
		`<span class="card-kind-label">Deep Dive stub</span>`,
		// The sealed wall's Instantiate affordance (finding site ~:410).
		`>Instantiate Change Request</button>`,
		`>Instantiate Deep Dive</button>`,
		// The feature-wall guide note (finding site ~:845).
		`This is a <strong>Initiative</strong> wall: outcome ACs and Change Request stubs.`,
		`a Initiative never lists its Change Requests.`,
		`when the Initiative lands (outcomes, never Change Request-sized tasks)`,
		// The yarn key's scoping meanings.
		`a planned Change Request will deliver it`,
		`a planned Deep Dive will answer it`,
		// The oq multi-claim chip.
		`>claimed by 2 Deep Dives</span>`,
		// A proto-sticky's visible type word (the class it becomes);
		// the comment sticky's chip stays its own taxonomy word.
		`<span class="sticky-type">Change Request</span>`,
		`<span class="sticky-type">comment</span>`,
		`aria-label="Draw attribution yarn from this Change Request sticky"`,
		// The family-links affordances on implements-edge reference cards.
		`>open Initiative board</a>`,
		`>open Initiative <span class="badge badge-archived"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("board region missing renamed prose %q", want)
		}
	}

	// Bare class words must be gone from the display prose...
	for _, gone := range []string{
		`>story stub<`, `>spike stub<`, `>Instantiate story<`, `>Instantiate spike<`,
		`claimed by 2 spikes`, `a planned story will deliver it`, `a planned spike will answer it`,
		`<span class="sticky-type">story</span>`, `>open feature board<`,
	} {
		if strings.Contains(html, gone) {
			t.Errorf("board region still renders bare class-word prose %q", gone)
		}
	}

	// ...while the identity layer provably keeps them (vocabulary.go's
	// enumeration rule): testids, data attributes, CSS modifiers.
	for _, keep := range []string{
		`data-testid="stub-card-alpha"`, `data-stub="beta"`, `data-spike="true"`,
		`stubcard--spike`, `data-instantiate="alpha"`, `data-testid="instantiate-beta"`,
		`data-annotation-type="story"`, `sticky--story`, `data-testid="oq-claims-oq-1"`,
		`data-testid="refcard-board-link"`, `data-testid="refcard-feature-archived"`,
		`href="/board/spec/parent-feature"`,
	} {
		if !strings.Contains(html, keep) {
			t.Errorf("board region lost identity-layer string %q — a rename must never move addressing", keep)
		}
	}
}

// TestBoardRender_ObligationYarnTitleModelVocabulary proves the story
// wall's obligation-yarn tooltip speaks the renamed class word.
func TestBoardRender_ObligationYarnTitleModelVocabulary(t *testing.T) {
	proj := &BoardProjection{
		Spec: "story-probe", Title: "Story probe", Mode: modeAuthoring,
		Status: "draft", Class: "story",
		Stickies: []scratchStickyView{{ID: "a-03", Type: "comment", Body: "c"}},
	}
	proj.applyModelVocabulary(vocabTestModel())
	html := renderBoardRegion(proj, &boardGitState{})
	if !strings.Contains(html, `title="drag to a Change Request acceptance criterion to author its evidence obligation"`) {
		t.Fatalf("obligation yarn handle title not resolved; got:\n%s", html)
	}
}

// TestBoardRender_RegionParityNoRenames is the parity floor over the
// FULL prose fixture: the canonical (empty-vocabulary) model renders the
// region byte-identically to no model at all — today's literals, including
// every hand-written plural.
func TestBoardRender_RegionParityNoRenames(t *testing.T) {
	plain := renderBoardRegion(vocabProseProjection(modeAuthoring), &boardGitState{})

	enriched := vocabProseProjection(modeAuthoring)
	enriched.applyModelVocabulary(model.Canonical())
	if got := renderBoardRegion(enriched, &boardGitState{}); got != plain {
		t.Fatal("canonical model changed the rendered board region; the no-rename path must be byte-identical")
	}
	for _, want := range []string{
		`>story stub<`, `>spike stub<`, `>Instantiate story<`, `>Instantiate spike<`,
		`claimed by 2 spikes`, `a planned story will deliver it`,
		`This is a <strong>feature</strong> wall: outcome ACs and story stubs.`,
		`a feature never lists its stories.`, `>open feature board</a>`,
	} {
		if !strings.Contains(plain, want) {
			t.Errorf("no-rename board region missing today's literal %q", want)
		}
	}
}

// TestBoardRender_PageWordsPayloadModelVocabulary proves the page embeds
// the client's words map and the resolved consequence labels ONLY when a
// rename exists — and that the payload's class VALUE (the client's
// gating id) stays bare either way.
func TestBoardRender_PageWordsPayloadModelVocabulary(t *testing.T) {
	proj := vocabProseProjection(modeAuthoring)
	proj.applyModelVocabulary(vocabTestModel())
	page, err := renderBoardSpecPage(proj, &boardGitState{})
	if err != nil {
		t.Fatalf("renderBoardSpecPage: %v", err)
	}
	html := string(page)
	if !strings.Contains(html, `"words":{"feature":"Initiative","spike":"Deep Dive","story":"Change Request"}`) {
		t.Fatalf("page payload missing the words map; got page:\n%s", html)
	}
	if !strings.Contains(html, "records that this Change Request delivers that acceptance criterion") {
		t.Fatal("payload consequence label for implements not resolved")
	}
	if !strings.Contains(html, "records that this Deep Dive answers that open question") {
		t.Fatal("payload consequence label for resolves not resolved")
	}
	if !strings.Contains(html, `"class":"feature"`) {
		t.Fatal("payload class VALUE must stay the bare id — it gates client behavior, never display")
	}

	// Parity: with no renames the page is byte-identical to no model at
	// all — no words key, today's consequence literals.
	plainPage, err := renderBoardSpecPage(vocabProseProjection(modeAuthoring), &boardGitState{})
	if err != nil {
		t.Fatalf("renderBoardSpecPage plain: %v", err)
	}
	canonical := vocabProseProjection(modeAuthoring)
	canonical.applyModelVocabulary(model.Canonical())
	canonicalPage, err := renderBoardSpecPage(canonical, &boardGitState{})
	if err != nil {
		t.Fatalf("renderBoardSpecPage canonical: %v", err)
	}
	if string(canonicalPage) != string(plainPage) {
		t.Fatal("canonical model changed the rendered page; the no-rename path must be byte-identical")
	}
	if strings.Contains(string(plainPage), `"words":`) {
		t.Fatal("no-rename page must embed no words key at all (the ClassLabel posture)")
	}
	if !strings.Contains(string(plainPage), "records that this story delivers that acceptance criterion") {
		t.Fatal("no-rename page missing today's implements consequence literal")
	}
}

// TestConsequenceLabelsFor_Vocabulary pins the picker copy's two class-word
// rows at the unit level: a zero classWords reproduces today's literals
// exactly, a renamed model reaches both rows, and the class-word-free rows
// never vary.
func TestConsequenceLabelsFor_Vocabulary(t *testing.T) {
	plain := consequenceLabelsFor(classWords{})
	if got := plain["implements"]; got != "records that this story delivers that acceptance criterion; its owners see the claim" {
		t.Fatalf("zero-words implements label = %q", got)
	}
	if got := plain["resolves"]; got != "records that this spike answers that open question" {
		t.Fatalf("zero-words resolves label = %q", got)
	}

	renamed := consequenceLabelsFor(classWords{m: vocabTestModel()})
	if got := renamed["implements"]; got != "records that this Change Request delivers that acceptance criterion; its owners see the claim" {
		t.Fatalf("renamed implements label = %q", got)
	}
	if got := renamed["resolves"]; got != "records that this Deep Dive answers that open question" {
		t.Fatalf("renamed resolves label = %q", got)
	}
	if plain["supersedes"] != renamed["supersedes"] || plain["relates"] != renamed["relates"] {
		t.Fatal("class-word-free consequence rows must not vary with the vocabulary")
	}
}

// TestRenderBoardPageV0_ModelVocabulary covers the grandfathered v0 board
// page: the commit-to-design ritual copy and the tracker-ref field label
// resolve, a proto-sticky's chip resolves, and the identity layer
// (data-type, field names) stays bare — with byte-parity when nothing is
// renamed.
func TestRenderBoardPageV0_ModelVocabulary(t *testing.T) {
	state := boardClientState{
		Key: "probe",
		Stickies: []stickyView{
			{ID: "s1", Type: "story", Body: "b", Status: "open"},
			{ID: "s2", Type: "comment", Body: "c", Status: "open"},
		},
	}

	renamed, err := renderBoardPage(state, classWords{m: vocabTestModel()})
	if err != nil {
		t.Fatalf("renderBoardPage: %v", err)
	}
	html := string(renamed)
	for _, want := range []string{
		"Freezes this board into a draft Initiative spec:",
		`>Change Request ref <span class="optional">(optional)</span></label>`,
		`<span class="sticky-type">Change Request</span>`,
		`<span class="sticky-type">comment</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("v0 board page missing renamed prose %q", want)
		}
	}
	for _, keep := range []string{`data-type="story"`, `name="story_ref"`, `id="commit-story-ref"`} {
		if !strings.Contains(html, keep) {
			t.Errorf("v0 board page lost identity-layer string %q", keep)
		}
	}

	plain, err := renderBoardPage(state, classWords{})
	if err != nil {
		t.Fatalf("renderBoardPage plain: %v", err)
	}
	canonical, err := renderBoardPage(state, classWords{m: model.Canonical()})
	if err != nil {
		t.Fatalf("renderBoardPage canonical: %v", err)
	}
	if string(canonical) != string(plain) {
		t.Fatal("canonical model changed the v0 board page; the no-rename path must be byte-identical")
	}
	if !strings.Contains(string(plain), "draft feature spec") || !strings.Contains(string(plain), "Story ref <span") {
		t.Fatal("no-rename v0 board page missing today's literals")
	}
}

// TestCorpusMetaRows_ModelVocabulary covers the corpus page's metadata
// card: the Class VALUE and the Story row LABEL resolve (the dex twin's
// exact posture) and the Status VALUE resolves through DisplayState,
// while the story tracker-ref VALUE and the Kind row stay identity.
func TestCorpusMetaRows_ModelVocabulary(t *testing.T) {
	e := &index.Entry{Kind: "spec", Status: "accepted-pending-build"}
	m := artifactview.Meta{Class: "story", Story: "jira:LOAN-1"}

	rows := corpusMetaRows(e, m, vocabTestModel())
	byLabel := map[string]string{}
	for _, r := range rows {
		byLabel[r.Label] = r.Value
	}
	if byLabel["Kind"] != "spec" {
		t.Fatalf("Kind row = %q, want the bare kind (identity)", byLabel["Kind"])
	}
	if byLabel["Status"] != "Ready to build" {
		t.Fatalf("Status row = %q, want the renamed state word", byLabel["Status"])
	}
	if byLabel["Class"] != "Change Request" {
		t.Fatalf("Class row = %q, want the renamed class word", byLabel["Class"])
	}
	if byLabel["Change Request"] != "jira:LOAN-1" {
		t.Fatalf("story row = %v, want label 'Change Request' with the tracker ref VALUE untouched", rows)
	}

	// Nil model: today's rows byte-for-byte.
	plain := corpusMetaRows(e, m, nil)
	byLabel = map[string]string{}
	for _, r := range plain {
		byLabel[r.Label] = r.Value
	}
	if byLabel["Status"] != "accepted-pending-build" || byLabel["Class"] != "story" || byLabel["Story"] != "jira:LOAN-1" {
		t.Fatalf("nil-model corpus rows = %v, want today's bare ids", plain)
	}
}

// TestWriteSnapshotPicker_ModelVocabulary covers the verdict page's
// empty-picker copy: the class word resolves, and the no-model fallback
// is today's sentence.
func TestWriteSnapshotPicker_ModelVocabulary(t *testing.T) {
	var renamed bytes.Buffer
	writeSnapshotPicker(&renamed, "jira:LOAN-1", nil, classWords{m: vocabTestModel()})
	if !strings.Contains(renamed.String(), "Fewer than two snapshots exist yet for this Change Request.") {
		t.Fatalf("picker = %q, want the renamed class word", renamed.String())
	}

	var plain bytes.Buffer
	writeSnapshotPicker(&plain, "jira:LOAN-1", nil, classWords{})
	if !strings.Contains(plain.String(), "Fewer than two snapshots exist yet for this story.") {
		t.Fatalf("no-model picker = %q, want today's literal", plain.String())
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
