// spec/spec-documents ac-6, wave 2 task 6: the four-way document parity
// proof. The CLI (spec doc), the MCP tool (get_document), the docs site
// (dex), and the board's Document tab all render one spec ref's Markdown
// through the SAME shared loader (internal/specdocload), so their output
// must be byte-identical — not merely similar — whichever consumer a
// reader happens to be looking at. This file is that witness, plus its
// pinned-commit arm.
package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/workbench"
)

// TestDocumentParity_FourConsumers is spec/spec-documents ac-6: the CLI,
// the board Document tab, the docs site, and the MCP tool render one ref
// at one commit to byte-identical Markdown. All four run over the same
// fixturegit store on its main checkout, where the working tree, HEAD,
// and the default branch coincide, so every consumer's reading is the
// accepted one.
func TestDocumentParity_FourConsumers(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSpecDocRepo(t)
	ctx := context.Background()
	env := []string{"CI_DEFAULT_BRANCH=main"}

	// 1. CLI
	bin := buildVerdiBinary(t)
	cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("cli exit %d: %s", code, stderr)
	}

	// 2. MCP
	backend := &mcpserve.Backend{Root: repo.Dir}
	res := backend.GetDocument(ctx, json.RawMessage(`{"ref":"spec/lockbox"}`))
	var payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	// The refusal diagnostic below prints raw, so a marshal failure
	// must not be swallowed into an empty one (final-review F8).
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshalling the mcp tool result: %v", err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.IsError || len(payload.Content) == 0 {
		t.Fatalf("mcp result: %s", raw)
	}
	var mcpDoc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(payload.Content[0].Text), &mcpDoc); err != nil {
		t.Fatal(err)
	}

	// 3. Docs site
	out := t.TempDir()
	if err := dex.Build(ctx, dex.Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatal(err)
	}
	dexBytes, err := os.ReadFile(filepath.Join(out, "a", "spec", "lockbox", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}

	// 4. Board (root mount on the main checkout)
	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{}) // internal/workbench/handler.go:53
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/lockbox/document?format=md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
	}

	board := rec.Body.String()
	if cliOut != mcpDoc.Markdown {
		t.Errorf("CLI and MCP differ:\n--- cli ---\n%s\n--- mcp ---\n%s", cliOut, mcpDoc.Markdown)
	}
	if cliOut != string(dexBytes) {
		t.Errorf("CLI and docs site differ:\n--- cli ---\n%s\n--- dex ---\n%s", cliOut, dexBytes)
	}
	if cliOut != board {
		t.Errorf("CLI and board differ:\n--- cli ---\n%s\n--- board ---\n%s", cliOut, board)
	}
	if !strings.HasSuffix(cliOut, "\n") || strings.HasSuffix(cliOut, "\n\n") {
		t.Errorf("exactly one trailing newline")
	}
	// Determinism across a second CLI run.
	again, _, _ := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox")
	if again != cliOut {
		t.Errorf("two CLI renders differ")
	}
}

// TestDocumentParity_PinnedCommit is ac-6's pinned-commit arm: the CLI's
// --at, MCP's commit argument, and the docs site (which always renders at
// its own build commit — dex/document.go's writeSpecDocuments uses
// ModeAt at stamp.SHA, never the working tree) all name the SAME
// historical commit and must still agree byte-for-byte. The board has no
// pinned-commit mode at all — boarddocument.go's loadDocument always
// reads ModeWorkingTree (the serving checkout's own working tree, never a
// caller-supplied commit) — so it is deliberately not part of this arm.
func TestDocumentParity_PinnedCommit(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSpecDocRepo(t)
	ctx := context.Background()
	env := []string{"CI_DEFAULT_BRANCH=main"}

	// 1. CLI, pinned via --at.
	bin := buildVerdiBinary(t)
	cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox", "--at", repo.Head)
	if code != 0 {
		t.Fatalf("cli --at exit %d: %s", code, stderr)
	}
	if !strings.Contains(cliOut, "commit `"+repo.Head+"`") {
		t.Errorf("pinned CLI render must name commit %s, got:\n%s", repo.Head, cliOut)
	}

	// 2. MCP, pinned via the commit argument.
	backend := &mcpserve.Backend{Root: repo.Dir}
	args, err := json.Marshal(map[string]string{"ref": "spec/lockbox", "commit": repo.Head})
	if err != nil {
		t.Fatal(err)
	}
	res := backend.GetDocument(ctx, json.RawMessage(args))
	var payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	// The refusal diagnostic below prints raw, so a marshal failure
	// must not be swallowed into an empty one (final-review F8).
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshalling the mcp tool result: %v", err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.IsError || len(payload.Content) == 0 {
		t.Fatalf("mcp result: %s", raw)
	}
	var mcpDoc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(payload.Content[0].Text), &mcpDoc); err != nil {
		t.Fatal(err)
	}

	// 3. Docs site: writeSpecDocuments always renders ModeAt at the
	// build's own stamp commit, so a plain Build (Commit unset, defaulting
	// to HEAD == repo.Head on this single-layer fixture) is already the
	// pinned reading.
	out := t.TempDir()
	if err := dex.Build(ctx, dex.Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatal(err)
	}
	dexBytes, err := os.ReadFile(filepath.Join(out, "a", "spec", "lockbox", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}

	if cliOut != mcpDoc.Markdown {
		t.Errorf("CLI --at and MCP commit= differ:\n--- cli ---\n%s\n--- mcp ---\n%s", cliOut, mcpDoc.Markdown)
	}
	if cliOut != string(dexBytes) {
		t.Errorf("CLI --at and docs site differ:\n--- cli ---\n%s\n--- dex ---\n%s", cliOut, dexBytes)
	}
	// The board is skipped here: it has no pinned-commit mode to compare
	// against (see the function doc comment).
}

// TestDocumentParity_BoardWithForeignReadinessSnapshot is ac-6 under the
// SHIPPED board wiring rather than an empty Deps (final-review F1).
// `verdi serve` builds the board with workbench.Deps{Readiness: snapshot}
// (cmd/verdi/serve.go), which reaches the loader as
// specdocload.Request.Readiness (internal/workbench/boarddocument.go);
// the other three consumers pass none. The loader hands that snapshot to
// specdoc.WithReadiness, which supplies it ONLY when snap.TargetRef names
// the spec being rendered (internal/specdoc/readiness.go). So a served
// process has two configurations, and this test pins the one where
// parity holds:
//
//   - TargetRef names a DIFFERENT spec (this test, and the ordinary case
//     — one startup snapshot targets one spec, every other spec's
//     document is read with no readiness): WithReadiness returns the
//     facts untouched, the board renders "Readiness was not supplied for
//     this render." exactly as the CLI, docs site and MCP do, and all
//     four legs are byte-identical. Proven here.
//   - TargetRef names the spec being rendered: the board alone renders
//     the Readiness section's facts and the four consumers diverge.
//     That is the adjudicated boundary R-W2-3 (plan; SDD ledger Task 6),
//     disclosed in the wave report — NOT covered by any parity assertion,
//     here or elsewhere. Named so a reader of a green gate does not read
//     it as "ac-6 proven for every configuration".
//
// Without the TargetRef gate this test reds: the board leg would grow a
// populated Readiness section the other three legs do not have.
func TestDocumentParity_BoardWithForeignReadinessSnapshot(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSpecDocRepo(t)
	ctx := context.Background()
	env := []string{"CI_DEFAULT_BRANCH=main"}

	// 1. CLI
	bin := buildVerdiBinary(t)
	cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("cli exit %d: %s", code, stderr)
	}

	// 2. MCP
	backend := &mcpserve.Backend{Root: repo.Dir}
	res := backend.GetDocument(ctx, json.RawMessage(`{"ref":"spec/lockbox"}`))
	var payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshalling the mcp tool result: %v", err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.IsError || len(payload.Content) == 0 {
		t.Fatalf("mcp result: %s", raw)
	}
	var mcpDoc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(payload.Content[0].Text), &mcpDoc); err != nil {
		t.Fatal(err)
	}

	// 3. Docs site
	out := t.TempDir()
	if err := dex.Build(ctx, dex.Options{Root: repo.Dir, OutDir: out}); err != nil {
		t.Fatal(err)
	}
	dexBytes, err := os.ReadFile(filepath.Join(out, "a", "spec", "lockbox", "spec.md"))
	if err != nil {
		t.Fatal(err)
	}

	// 4. Board, wired the way serve.go wires it: a real startup snapshot,
	// targeting another spec. Every field carries a marker string, so a
	// leak is visible in the diff rather than merely changing a byte.
	snap := readinesspilot.Snapshot{
		TargetRef:    "spec/other-wall",
		TargetTitle:  "Other wall",
		TargetClass:  "feature",
		Branch:       "main",
		Head:         strings.Repeat("c", 40),
		CurrentFocus: readinesspilot.AreaShape,
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "FOREIGN-AREA-LABEL", State: readinesspilot.StateProven},
		},
		Attention: []readinesspilot.Concern{{
			ID: "shape/question/oq-1", Area: readinesspilot.AreaShape, State: readinesspilot.StateUnproven,
			Blocking: true, Timing: readinesspilot.TimingCurrent,
			Summary: "FOREIGN-CONCERN-SUMMARY", Witnesses: []string{"FOREIGN-WITNESS"},
		}},
		StaleNotice: "FOREIGN-STALE-NOTICE",
	}
	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{Readiness: &snap})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/lockbox/document?format=md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
	}
	board := rec.Body.String()

	if cliOut != mcpDoc.Markdown {
		t.Errorf("CLI and MCP differ:\n--- cli ---\n%s\n--- mcp ---\n%s", cliOut, mcpDoc.Markdown)
	}
	if cliOut != string(dexBytes) {
		t.Errorf("CLI and docs site differ:\n--- cli ---\n%s\n--- dex ---\n%s", cliOut, dexBytes)
	}
	if cliOut != board {
		t.Errorf("CLI and board-with-foreign-readiness differ:\n--- cli ---\n%s\n--- board ---\n%s", cliOut, board)
	}

	// The board's own reading of that configuration, stated rather than
	// inferred from equality: the section is present and says the facts
	// were not supplied, and not one field of the other spec's snapshot
	// reached this document.
	if !strings.Contains(board, "## Readiness\n\nReadiness was not supplied for this render.") {
		t.Errorf("board must state the unsupplied readiness:\n%s", board)
	}
	for _, leaked := range []string{"FOREIGN-AREA-LABEL", "FOREIGN-CONCERN-SUMMARY", "FOREIGN-WITNESS", "FOREIGN-STALE-NOTICE", "spec/other-wall", "readiness snapshot for"} {
		if strings.Contains(board, leaked) {
			t.Errorf("another spec's snapshot leaked %q into spec/lockbox's document:\n%s", leaked, board)
		}
	}
}

// validReadinessSnapshotFor returns a Snapshot targeting ref that passes
// Snapshot.Validate(): every one of the four fixed areas proven, no
// attention items. Mirrors internal/readinesspilot/schema_test.go's own
// (unexported) validSnapshot() — reproduced here for the same reason
// internal/mcpserve/tool_get_document_test.go's validReadinessSnapshot is
// (that package's own private helper cannot be imported from here either).
// The closed concern-identity vocabulary (readinesspilot/schema.go's
// concernIdentity) fixes these four concern ids' areas and blocking flags;
// they are not arbitrary. TargetTitle/TargetClass/Branch/RequestDigest are
// Validate()-only fields specdoc.WithReadiness never reads.
func validReadinessSnapshotFor(ref, head string) readinesspilot.Snapshot {
	concern := func(id string, area readinesspilot.AreaID, blocking bool) readinesspilot.Concern {
		return readinesspilot.Concern{
			ID: id, Area: area, State: readinesspilot.StateProven, Blocking: blocking,
			Timing: readinesspilot.TimingCurrent, Summary: "source-derived readiness fact",
			Witnesses: []string{}, Destination: readinesspilot.Destination{CLI: []string{}},
		}
	}
	return readinesspilot.Snapshot{
		TargetRef:     ref,
		TargetTitle:   "Parity test target",
		TargetClass:   "feature",
		Branch:        "main",
		Head:          head,
		RequestDigest: "sha256:" + strings.Repeat("a", 64),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaContext, Label: "Check constraints", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Get approval", State: readinesspilot.StateProven},
		},
		CurrentFocus: "",
		Attention:    []readinesspilot.Concern{},
		AllConcerns: []readinesspilot.Concern{
			concern("shape/problem", readinesspilot.AreaShape, true),
			concern("success/contributor/static", readinesspilot.AreaSuccess, false),
			concern("context/verdict", readinesspilot.AreaContext, true),
			concern("review/action", readinesspilot.AreaReview, true),
		},
		StaleNotice: "Startup snapshot at " + head + "; restart verdi serve after an edit.",
	}
}

// stripReadinessSection removes the "## Readiness" section (through the
// next "## " heading, or EOF when Readiness is the last section, which it
// always is for kind spec — specdoc/kind.go's Sections()) from doc. Used
// to pin that two renders differ by EXACTLY that section — never
// elsewhere in the document.
func stripReadinessSection(t *testing.T, doc string) string {
	t.Helper()
	const heading = "## Readiness"
	start := strings.Index(doc, heading)
	if start < 0 {
		t.Fatalf("document has no %q heading:\n%s", heading, doc)
	}
	rest := doc[start+len(heading):]
	if next := strings.Index(rest, "## "); next >= 0 {
		return doc[:start] + rest[next:]
	}
	return doc[:start]
}

// TestDocumentParity_BoardAndMCPShareReadiness is R-W3-3: `verdi serve`
// wires the SAME startup readiness snapshot into both the board
// (workbench.Deps{Readiness: ...}) and get_document (Backend.Readiness,
// cmd/verdi/serve.go:296-297) — so a snapshot targeting the rendered spec
// produces byte-identical Markdown from both legs, carrying a populated
// Readiness section, and the CLI's readiness-less render (spec doc has no
// snapshot input at all) differs from them by EXACTLY that section.
func TestDocumentParity_BoardAndMCPShareReadiness(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSpecDocRepo(t)
	ctx := context.Background()
	env := []string{"CI_DEFAULT_BRANCH=main"}

	// 1. CLI — no readiness input exists on this path at all.
	bin := buildVerdiBinary(t)
	cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("cli exit %d: %s", code, stderr)
	}

	snap := validReadinessSnapshotFor("spec/lockbox", repo.Head)
	if err := snap.Validate(); err != nil {
		t.Fatal(err)
	}

	// 2. MCP
	backend := &mcpserve.Backend{Root: repo.Dir, Readiness: &snap}
	res := backend.GetDocument(ctx, json.RawMessage(`{"ref":"spec/lockbox"}`))
	var payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshalling the mcp tool result: %v", err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.IsError || len(payload.Content) == 0 {
		t.Fatalf("mcp result: %s", raw)
	}
	var mcpDoc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(payload.Content[0].Text), &mcpDoc); err != nil {
		t.Fatal(err)
	}

	// 3. Board, wired the way serve.go wires it: the identical snapshot.
	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{Readiness: &snap})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/lockbox/document?format=md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
	}
	board := rec.Body.String()

	if board != mcpDoc.Markdown {
		t.Errorf("board and MCP differ under the SAME readiness snapshot:\n--- board ---\n%s\n--- mcp ---\n%s", board, mcpDoc.Markdown)
	}
	for name, doc := range map[string]string{"board": board, "mcp": mcpDoc.Markdown} {
		if !strings.Contains(doc, "## Readiness") || strings.Contains(doc, "Readiness was not supplied for this render.") {
			t.Fatalf("%s must carry a populated Readiness section:\n%s", name, doc)
		}
	}

	// The CLI leg carries no snapshot, so stripping each leg's Readiness
	// section must leave byte-identical remainders — the divergence is
	// exactly that section, nowhere else in the document.
	if boardStripped, cliStripped := stripReadinessSection(t, board), stripReadinessSection(t, cliOut); boardStripped != cliStripped {
		t.Errorf("board and CLI diverge by more than the Readiness section:\n--- board (stripped) ---\n%s\n--- cli (stripped) ---\n%s", boardStripped, cliStripped)
	}
}
