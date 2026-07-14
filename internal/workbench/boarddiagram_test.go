package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/diagrambase"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// diagramFixtureBody is the from-scratch proposal's mermaid body —
// adversarial formatting on purpose (mixed indentation, a comment, a
// blank line, trailing spaces): the write paths must carry every byte.
const diagramFixtureBody = "flowchart TD\n" +
	"  loansvc[\"Loan service\"]   \n" +
	"\tbilling\n" +
	"\n" +
	"  %% hand comment \n" +
	"  loansvc --> billing\n"

const diagramFixtureName = "target-topology"

const diagramFixture = `---
id: diagram/target-topology
kind: diagram
class: proposal
title: "Target topology"
status: proposed
owners: [platform-team]
---
` + diagramFixtureBody

// The out-of-subset proposal: renderer-legal, but not the op grammar's
// flowchart subset (ac-2's disclosed-unavailable path).
const diagramSequenceFixture = `---
id: diagram/sequence-sketch
kind: diagram
class: proposal
title: "Sequence sketch"
status: proposed
owners: [platform-team]
---
sequenceDiagram
  Alice->>Bob: hi
`

// An incumbent (class-absent) diagram: NOT served by the editor.
const diagramIncumbentFixture = `---
id: diagram/incumbent
kind: diagram
title: "Incumbent"
status: active
owners: [platform-team]
---
graph TD
  a --> b
`

const diagramBaseName = "base-topology"
const diagramBaseBody = "graph TD\n  loansvc --> notification-svc\n"
const diagramBaseFixture = `---
id: diagram/base-topology
kind: diagram
title: "Base topology"
status: active
owners: [platform-team]
---
` + diagramBaseBody

// newDiagramFixture builds a repo holding the proposals, checked out on
// a design branch (the editor's authoring branch state). The derived
// proposals are authored AFTER the seed commit so their derived_from can
// pin that commit with a genuine digest — hence baseCommit is returned
// alongside the root.
func newDiagramFixture(t *testing.T) (root string, baseCommit string) {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/diagrams/" + diagramFixtureName + ".mermaid": diagramFixture,
			".verdi/diagrams/sequence-sketch.mermaid":            diagramSequenceFixture,
			".verdi/diagrams/incumbent.mermaid":                  diagramIncumbentFixture,
			".verdi/diagrams/" + diagramBaseName + ".mermaid":    diagramBaseFixture,
			".verdi/.gitignore":                                  "data/\n",
		},
		Message: "seed diagram fixtures",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/diagrams"); err != nil {
		t.Fatalf("checkout design branch: %v", err)
	}
	return repo.Dir, repo.Head
}

// writeDerivedFixture writes a derived proposal into root's working tree
// pinning baseCommit with the given digest (a matching one via
// diagrambase.CanonicalGraphDigest, or a corrupted constant).
func writeDerivedFixture(t *testing.T, root, name, baseCommit, digest, body string) {
	t.Helper()
	content := fmt.Sprintf(`---
id: diagram/%s
kind: diagram
class: proposal
title: "Derived"
status: proposed
owners: [platform-team]
derived_from: { ref: diagram/%s@%s, digest: %s }
---
%s`, name, diagramBaseName, baseCommit, digest, body)
	path := filepath.Join(root, ".verdi", "diagrams", name+".mermaid")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing derived fixture: %v", err)
	}
}

func getDiagram(t *testing.T, h http.Handler, name, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/diagram/"+name+path, nil))
	return rec
}

func postDiagramAPI(t *testing.T, h http.Handler, name, action, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/board/diagram/"+name+"/api/"+action, strings.NewReader(body))
	h.ServeHTTP(rec, req)
	return rec
}

func readDiagramBody(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, ".verdi", "diagrams", name+".mermaid"))
	if err != nil {
		t.Fatalf("reading diagram: %v", err)
	}
	i := strings.Index(string(raw), "\n---\n")
	if i < 0 {
		t.Fatalf("no frontmatter close in %q", raw)
	}
	return string(raw[i+len("\n---\n"):])
}

func TestBoardDiagramPage_Authoring(t *testing.T) {
	root, _ := newDiagramFixture(t)
	h := NewHandler(root)
	rec := getDiagram(t, h, diagramFixtureName, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET page = %d: %s", rec.Code, rec.Body.String())
	}
	html := rec.Body.String()
	for _, want := range []string{
		`data-testid="diagram-source"`,
		`data-testid="diagram-preview"`,
		`data-testid="diagram-render-error"`,
		`data-testid="verification-rail"`,
		`data-editor-mode="authoring"`,
		`/assets/mermaid.min.js`, // the ONE vendored pin, dc-3 — no CDN URL anywhere
		`Loan service`,           // the pane holds the artifact's source
	} {
		if !strings.Contains(html, want) {
			t.Errorf("page missing %q", want)
		}
	}
	if strings.Contains(html, "cdn") || strings.Contains(html, "https://") {
		t.Errorf("page references an external origin; the editor is hermetic (dc-3/co-4)")
	}
	// No extractor wired: the rail renders the DISCLOSED unavailable
	// state (ac-5), never an empty region.
	if !strings.Contains(html, `data-testid="verification-unavailable"`) {
		t.Errorf("rail does not disclose verification-unavailable without a wired extractor")
	}
	// From-scratch proposal: the peek/reset affordances are NOT offered.
	for _, absent := range []string{`data-testid="peek-btn"`, `data-testid="reset-btn"`} {
		if strings.Contains(html, absent) {
			t.Errorf("from-scratch proposal offers %q; ac-4 offers the affordances to derived proposals only", absent)
		}
	}
}

func TestBoardDiagramPage_NotFoundAndNotProposal(t *testing.T) {
	root, _ := newDiagramFixture(t)
	h := NewHandler(root)
	if rec := getDiagram(t, h, "no-such-diagram", ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing diagram = %d, want 404", rec.Code)
	}
	// An incumbent diagram is not a proposal: no editor surface.
	if rec := getDiagram(t, h, "incumbent", ""); rec.Code != http.StatusNotFound {
		t.Errorf("incumbent diagram = %d, want 404 (the editor serves class: proposal only)", rec.Code)
	}
	// A non-kebab name never reaches the filesystem (specNameRe); dotted
	// path traversal is already cleaned away by ServeMux itself.
	if rec := getDiagram(t, h, "Bad_Name", ""); rec.Code != http.StatusNotFound {
		t.Errorf("non-kebab name = %d, want 404", rec.Code)
	}
}

func TestBoardDiagramFragment(t *testing.T) {
	root, _ := newDiagramFixture(t)
	h := NewHandler(root)
	rec := getDiagram(t, h, diagramFixtureName, "/fragment")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET fragment = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `data-testid="diagram-editor"`) {
		t.Errorf("fragment does not carry the editor region")
	}
}

// TestBoardDiagramSave_ByteVerbatim is the save half of obligation
// ac-3--static: the body written to disk is the request's pane bytes
// VERBATIM — trailing whitespace, mixed indentation, comments, blank
// lines, no trailing newline — and the frontmatter bytes are untouched.
func TestBoardDiagramSave_ByteVerbatim(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"adversarial formatting", "flowchart TD\n   weird[\"x\"]\t \n\n\n%% c\n\tweird --> weird  \n"},
		{"no trailing newline", "flowchart TD\n  a --> b"},
		{"outside the op subset (a save is never gated on the grammar)", "sequenceDiagram\n  A->>B: hi\n"},
		{"renderer-invalid text (a save is never gated on the renderer)", "not mermaid at all\x09 trailing\t"},
		{"empty body", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := newDiagramFixture(t)
			h := NewHandler(root)
			body, err := json.Marshal(map[string]string{"source": tc.src})
			if err != nil {
				t.Fatal(err)
			}
			rec := postDiagramAPI(t, h, diagramFixtureName, "save", string(body))
			if rec.Code != http.StatusOK {
				t.Fatalf("save = %d: %s", rec.Code, rec.Body.String())
			}
			if got := readDiagramBody(t, root, diagramFixtureName); got != tc.src {
				t.Fatalf("stored body = %q, want the pane bytes verbatim %q", got, tc.src)
			}
			// The frontmatter prefix survived bit-identically.
			raw, err := os.ReadFile(filepath.Join(root, ".verdi", "diagrams", diagramFixtureName+".mermaid"))
			if err != nil {
				t.Fatal(err)
			}
			wantPrefix := strings.TrimSuffix(diagramFixture, diagramFixtureBody)
			if !strings.HasPrefix(string(raw), wantPrefix) {
				t.Fatalf("frontmatter prefix changed:\n%q", raw)
			}
		})
	}
}

// TestBoardDiagramOps_ThroughAPI: a structural op lands as dc-2's exact
// deterministic edit, changing only its grammar-named lines (ac-2/ac-3).
func TestBoardDiagramOps_ThroughAPI(t *testing.T) {
	cases := []struct {
		name, action, req, wantBody string
	}{
		{"add-node appends the lowest unused n<k>", "add-node", `{"label":"Notification"}`,
			diagramFixtureBody + "  n1[\"Notification\"]\n"},
		{"connect appends one edge line", "connect", `{"from":"billing","to":"loansvc"}`,
			diagramFixtureBody + "  billing --> loansvc\n"},
		{"rename rewrites only the label", "rename", `{"id":"loansvc","label":"Loan orchestrator"}`,
			strings.Replace(diagramFixtureBody, `loansvc["Loan service"]`, `loansvc["Loan orchestrator"]`, 1)},
		{"delete-node removes defining + edge lines", "delete-node", `{"id":"billing"}`,
			"flowchart TD\n  loansvc[\"Loan service\"]   \n\n  %% hand comment \n"},
		{"delete-edge removes that one line", "delete-edge", `{"from":"loansvc","to":"billing"}`,
			strings.Replace(diagramFixtureBody, "  loansvc --> billing\n", "", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := newDiagramFixture(t)
			h := NewHandler(root)
			rec := postDiagramAPI(t, h, diagramFixtureName, tc.action, tc.req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s = %d: %s", tc.action, rec.Code, rec.Body.String())
			}
			if got := readDiagramBody(t, root, diagramFixtureName); got != tc.wantBody {
				t.Fatalf("body after %s:\n%q\nwant:\n%q", tc.action, got, tc.wantBody)
			}
			// The response carries the post-op source (the pane's new
			// truth) and the recomputed op model.
			var resp diagramAPIResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("response: %v", err)
			}
			if resp.Source != tc.wantBody {
				t.Errorf("response source = %q, want %q", resp.Source, tc.wantBody)
			}
			if !resp.OpsAvailable {
				t.Errorf("opsAvailable = false after an in-subset op")
			}
		})
	}
}

// TestBoardDiagramOps_DisclosedUnavailable: ops against out-of-subset
// source refuse with the disclosure and write NOTHING; the save path
// stays live for the same artifact (ac-2: the code pane stays live).
func TestBoardDiagramOps_DisclosedUnavailable(t *testing.T) {
	root, _ := newDiagramFixture(t)
	h := NewHandler(root)
	before := readDiagramBody(t, root, "sequence-sketch")

	rec := postDiagramAPI(t, h, "sequence-sketch", "add-node", `{"label":"x"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("op on out-of-subset source = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "outside the op grammar") {
		t.Errorf("refusal does not disclose the subset boundary: %s", rec.Body.String())
	}
	if got := readDiagramBody(t, root, "sequence-sketch"); got != before {
		t.Fatalf("out-of-subset op rewrote the source: %q", got)
	}

	// The page itself discloses the unavailable state.
	page := getDiagram(t, h, "sequence-sketch", "")
	if !strings.Contains(page.Body.String(), `data-testid="ops-unavailable"`) {
		t.Errorf("page does not disclose ops-unavailable")
	}

	// The code pane stays fully live: an ordinary save still lands.
	newSrc := "sequenceDiagram\n  Alice->>Bob: edited\n"
	body, _ := json.Marshal(map[string]string{"source": newSrc})
	if rec := postDiagramAPI(t, h, "sequence-sketch", "save", string(body)); rec.Code != http.StatusOK {
		t.Fatalf("save on out-of-subset source = %d: %s", rec.Code, rec.Body.String())
	}
	if got := readDiagramBody(t, root, "sequence-sketch"); got != newSrc {
		t.Fatalf("save did not land: %q", got)
	}
}

// TestBoardDiagramAPI_PositionKeyFailsClosed is co-2's schema refusal
// (obligation ac-2--static): a request carrying any position key is
// refused at strict decode, whatever the action.
func TestBoardDiagramAPI_PositionKeyFailsClosed(t *testing.T) {
	root, _ := newDiagramFixture(t)
	h := NewHandler(root)
	before := readDiagramBody(t, root, diagramFixtureName)
	for _, body := range []string{
		`{"label":"x","x":10,"y":20}`,
		`{"from":"loansvc","to":"billing","position":{"x":1,"y":2}}`,
		`{"source":"graph TD\n","x":3}`,
	} {
		for _, action := range []string{"add-node", "connect", "save"} {
			rec := postDiagramAPI(t, h, diagramFixtureName, action, body)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s with position key = %d, want 400 (fail closed)", action, rec.Code)
			}
		}
	}
	if got := readDiagramBody(t, root, diagramFixtureName); got != before {
		t.Fatalf("a refused request changed the artifact")
	}
}

// TestBoardDiagram_WritesRefusedOutsideAuthoring: dc-1's gate — the same
// posture as spec-board writes. On the default branch the editor is
// read-only and every write action refuses.
func TestBoardDiagram_WritesRefusedOutsideAuthoring(t *testing.T) {
	root, _ := newDiagramFixture(t)
	// Move back to the default branch: read-only checkout.
	if err := gitx.Checkout(context.Background(), root, "main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	h := NewHandler(root)
	page := getDiagram(t, h, diagramFixtureName, "")
	if !strings.Contains(page.Body.String(), `data-editor-mode="readonly"`) {
		t.Fatalf("editor on the default branch is not read-only")
	}
	before := readDiagramBody(t, root, diagramFixtureName)
	for action, body := range map[string]string{
		"save":     `{"source":"graph TD\n"}`,
		"add-node": `{"label":"x"}`,
		"reset":    `{}`,
	} {
		rec := postDiagramAPI(t, h, diagramFixtureName, action, body)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s outside authoring = %d, want 403", action, rec.Code)
		}
	}
	if got := readDiagramBody(t, root, diagramFixtureName); got != before {
		t.Fatalf("a refused write changed the artifact")
	}
}

// TestBoardDiagram_RailStates: ac-5 — the rail renders a canned report
// verbatim through the dc-4 port, and the disclosed unavailable state on
// a verifier error; neither blocks a save.
func TestBoardDiagram_RailStates(t *testing.T) {
	root, _ := newDiagramFixture(t)
	report := `{
  "target-topology": {
    "tier": "partial",
    "findings": [
      {"identity": "loansvc", "kind": "exists"},
      {"identity": "billing", "kind": "proposed-new"},
      {"identity": "audit", "kind": "contradicted", "witness": "abc1234"},
      {"identity": "base", "kind": "stale-base"}
    ]
  }
}`
	path := filepath.Join(t.TempDir(), "verification.json")
	if err := os.WriteFile(path, []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	verifier, err := LoadCannedDiagramVerifier(path)
	if err != nil {
		t.Fatalf("LoadCannedDiagramVerifier: %v", err)
	}
	h := NewHandlerWith(root, Deps{DiagramVerifier: verifier})

	page := getDiagram(t, h, diagramFixtureName, "").Body.String()
	for _, want := range []string{
		`data-tier="partial"`,
		`data-finding-kind="exists"`,
		`data-finding-kind="proposed-new"`,
		`data-finding-kind="contradicted"`,
		`data-finding-kind="stale-base"`,
		`abc1234`,
		`candidate witness`, // dc-4's corrected candor: never a verified cause
	} {
		if !strings.Contains(page, want) {
			t.Errorf("rail missing %q", want)
		}
	}

	// A diagram absent from the canned file: the verifier errors, and the
	// rail renders the DISCLOSED unavailable state — while a save still
	// succeeds (the rail never blocks).
	seqPage := getDiagram(t, h, "sequence-sketch", "").Body.String()
	if !strings.Contains(seqPage, `data-testid="verification-unavailable"`) {
		t.Errorf("rail does not disclose unavailability on a verifier error")
	}
	body, _ := json.Marshal(map[string]string{"source": "graph TD\n  x --> y\n"})
	if rec := postDiagramAPI(t, h, "sequence-sketch", "save", string(body)); rec.Code != http.StatusOK {
		t.Fatalf("save blocked by rail state: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLoadCannedDiagramVerifier_Negative(t *testing.T) {
	dir := t.TempDir()
	cases := []struct{ name, content string }{
		{"unknown tier fails closed", `{"d": {"tier": "certain", "findings": []}}`},
		{"unknown finding kind fails closed", `{"d": {"tier": "full", "findings": [{"identity": "x", "kind": "renamed"}]}}`},
		{"unknown field fails closed", `{"d": {"tier": "full", "findings": [], "score": 3}}`},
		{"malformed json", `{`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(dir, "v.json")
			if err := os.WriteFile(p, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadCannedDiagramVerifier(p); err == nil {
				t.Fatalf("LoadCannedDiagramVerifier accepted %q", tc.content)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		if _, err := LoadCannedDiagramVerifier(filepath.Join(dir, "absent.json")); err == nil {
			t.Fatal("missing file accepted")
		}
	})
}

// TestBoardDiagram_PeekAndReset: ac-4's server half against a fixturegit
// history — peek returns the digest-verified base without writing; reset
// writes it through the ordinary save path; the corrupted digest fails
// visible on both with the artifact untouched.
func TestBoardDiagram_PeekAndReset(t *testing.T) {
	root, baseCommit := newDiagramFixture(t)
	digest, err := diagrambase.CanonicalGraphDigest([]byte(diagramBaseBody))
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	workingBody := diagramBaseBody + "  loansvc --> audit\n"
	writeDerivedFixture(t, root, "derived-good", baseCommit, digest, workingBody)
	writeDerivedFixture(t, root, "derived-corrupt", baseCommit,
		"sha256:0000000000000000000000000000000000000000000000000000000000000000", workingBody)
	h := NewHandler(root)

	t.Run("page offers the affordances for a derived proposal", func(t *testing.T) {
		page := getDiagram(t, h, "derived-good", "").Body.String()
		for _, want := range []string{`data-testid="peek-btn"`, `data-testid="reset-btn"`, `data-testid="peek-panel"`, `data-testid="rail-provenance"`} {
			if !strings.Contains(page, want) {
				t.Errorf("derived proposal page missing %q", want)
			}
		}
	})

	t.Run("peek returns the base bytes and writes nothing", func(t *testing.T) {
		rec := postDiagramAPI(t, h, "derived-good", "peek", `{}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("peek = %d: %s", rec.Code, rec.Body.String())
		}
		var resp diagramPeekResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatal(err)
		}
		if resp.Base != diagramBaseBody {
			t.Errorf("peek base = %q, want %q", resp.Base, diagramBaseBody)
		}
		if got := readDiagramBody(t, root, "derived-good"); got != workingBody {
			t.Fatalf("peek modified the artifact: %q", got)
		}
	})

	t.Run("reset writes the base byte-for-byte through the save path", func(t *testing.T) {
		rec := postDiagramAPI(t, h, "derived-good", "reset", `{}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("reset = %d: %s", rec.Code, rec.Body.String())
		}
		if got := readDiagramBody(t, root, "derived-good"); got != diagramBaseBody {
			t.Fatalf("reset body = %q, want the base byte-for-byte %q", got, diagramBaseBody)
		}
	})

	t.Run("corrupted digest: both affordances fail visible, nothing written", func(t *testing.T) {
		beforeRaw, err := os.ReadFile(filepath.Join(root, ".verdi", "diagrams", "derived-corrupt.mermaid"))
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"peek", "reset"} {
			rec := postDiagramAPI(t, h, "derived-corrupt", action, `{}`)
			if rec.Code != http.StatusConflict {
				t.Errorf("%s with corrupted digest = %d, want 409: %s", action, rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "digest mismatch") {
				t.Errorf("%s refusal does not disclose the mismatch: %s", action, rec.Body.String())
			}
		}
		afterRaw, err := os.ReadFile(filepath.Join(root, ".verdi", "diagrams", "derived-corrupt.mermaid"))
		if err != nil {
			t.Fatal(err)
		}
		if string(beforeRaw) != string(afterRaw) {
			t.Fatalf("a refused peek/reset changed the artifact on disk")
		}
	})

	t.Run("from-scratch proposal refuses the affordances", func(t *testing.T) {
		for _, action := range []string{"peek", "reset"} {
			rec := postDiagramAPI(t, h, diagramFixtureName, action, `{}`)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s on a from-scratch proposal = %d, want 400", action, rec.Code)
			}
		}
	})
}

// TestBoardDiagram_RefCardEditorLink: dc-1's reachability from a spec
// board's diagram reference card — a proposal target gains the editor
// link; an incumbent diagram does not.
func TestBoardDiagram_RefCardEditorLink(t *testing.T) {
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/refi-test/spec.md": strings.Replace(boardFixtureSpec,
				`links: [ { type: exempts, ref: adr/0001-outbox-events, note: "async by design" } ]`,
				`links: [ { type: depends-on, ref: diagram/target-topology }, { type: depends-on, ref: diagram/incumbent } ]`, 1),
			".verdi/diagrams/target-topology.mermaid": diagramFixture,
			".verdi/diagrams/incumbent.mermaid":       diagramIncumbentFixture,
			".verdi/.gitignore":                       "data/\n",
		},
		Message: "seed spec with diagram refs",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/refi-test"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	h := NewHandler(repo.Dir)
	rec := getBoard(t, h, "refi-test")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET board = %d: %s", rec.Code, rec.Body.String())
	}
	html := rec.Body.String()
	if !strings.Contains(html, `data-testid="refcard-editor-link" href="/board/diagram/target-topology"`) {
		t.Errorf("proposal diagram ref card carries no editor link")
	}
	if strings.Contains(html, `href="/board/diagram/incumbent"`) {
		t.Errorf("incumbent diagram ref card carries an editor link; the editor serves proposals only")
	}
}

// TestCorpusPage_ProposalEditorLink: dc-1's reachability from the corpus
// page.
func TestCorpusPage_ProposalEditorLink(t *testing.T) {
	root, _ := newDiagramFixture(t)
	h := NewHandler(root)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/a/diagram/"+diagramFixtureName, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET corpus page = %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `data-testid="open-editor-link"`) {
		t.Errorf("proposal corpus page carries no editor link")
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/a/diagram/incumbent", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET incumbent corpus page = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), `data-testid="open-editor-link"`) {
		t.Errorf("incumbent corpus page carries an editor link; the editor serves proposals only")
	}
}

// TestBoardDiagram_MethodDiscipline: wrong methods 405 on every route.
func TestBoardDiagram_MethodDiscipline(t *testing.T) {
	root, _ := newDiagramFixture(t)
	h := NewHandler(root)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/board/diagram/"+diagramFixtureName, nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST page = %d, want 405", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/diagram/"+diagramFixtureName+"/api/save", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET api = %d, want 405", rec.Code)
	}
}
