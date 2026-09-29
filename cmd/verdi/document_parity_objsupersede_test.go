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
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
	"github.com/jyang234/verdi/internal/workbench"
)

// TestDocumentParity_ClosedSpecObjectSupersession extends the
// TestDocumentParity_* pattern to the closed-spec object supersession
// lines (design §6; spec/spec-documents ac-6; the L5 docs review's I-1):
// on the accepted scenario store, the superseded closed feature's document
// and its successor's document render byte-identically from the CLI, MCP
// get_document, the docs site, and — for the active successor, whose
// board exists — the board's Document tab. The closed feature is archived,
// and the board route 404s on the archive zone (ADJ-39), so its parity is
// three-way; the successor's is four-way. Each document must carry the
// lines, so the identity is not vacuous.
func TestDocumentParity_ClosedSpecObjectSupersession(t *testing.T) {
	for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITHUB_BASE_REF", "CI_DEFAULT_BRANCH", "CI_MERGE_REQUEST_TARGET_BRANCH_NAME"} {
		t.Setenv(key, "")
	}
	repo := scenario.Build(t, "accepted")
	ctx := context.Background()
	bin := buildVerdiBinary(t)

	// The docs site, once.
	out := t.TempDir()
	if err := dex.Build(ctx, dex.Options{Root: repo.Dir, OutDir: out, Commit: "main"}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, ref, marker string
		board             bool
	}{
		{"closed-feature", "spec/closed-feature", `data-testid="objsupersede-dc-1-since" data-state="superseded">superseded since 2024-02-15 by <a href="/a/spec/successor#dc-1">spec/successor#dc-1</a></span>`, false},
		{"successor", "spec/successor", `data-testid="objsupersede-dc-1-spec-closed-feature-dc-1-edge" data-state="in-force">supersedes <a href="/a/spec/closed-feature#dc-1">spec/closed-feature#dc-1</a></span>`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 1. CLI, --no-readiness (the other legs wire no readiness loader).
			cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, nil, "spec", "doc", tc.ref, "--no-readiness")
			if code != 0 {
				t.Fatalf("cli exit %d: %s", code, stderr)
			}
			// 2. MCP get_document.
			backend := &mcpserve.Backend{Root: repo.Dir}
			res := backend.GetDocument(ctx, json.RawMessage(`{"ref":"`+tc.ref+`"}`))
			raw, err := json.Marshal(res)
			if err != nil {
				t.Fatal(err)
			}
			var payload struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
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
			// 3. The docs site's spec.md.
			dexBytes, err := os.ReadFile(filepath.Join(out, "a", tc.ref, "spec.md"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(cliOut, tc.marker) {
				t.Fatalf("the CLI document lacks the §6 marker %q:\n%s", tc.marker, cliOut)
			}
			if cliOut != mcpDoc.Markdown {
				t.Errorf("CLI and MCP differ:\n--- cli ---\n%s\n--- mcp ---\n%s", cliOut, mcpDoc.Markdown)
			}
			if cliOut != string(dexBytes) {
				t.Errorf("CLI and the docs site differ:\n--- cli ---\n%s\n--- dex ---\n%s", cliOut, dexBytes)
			}
			if !tc.board {
				return
			}
			// 4. The board's Document tab (the serving checkout is main).
			h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{})
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/"+tc.ref+"/document?format=md", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
			}
			if board := rec.Body.String(); board != cliOut {
				t.Errorf("CLI and the board's Document tab differ:\n--- cli ---\n%s\n--- board ---\n%s", cliOut, board)
			}
		})
	}
}
