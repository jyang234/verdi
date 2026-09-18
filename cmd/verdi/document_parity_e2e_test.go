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
