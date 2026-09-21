// spec/spec-documents ac-6, wave 2 task 6 — extended by spec/readiness-
// recovery ac-4 (Task 3): the four-way document parity proof. The CLI
// (spec doc), the MCP tool (get_document), the docs site (dex), and the
// board's Document tab all render one spec ref's Markdown through the
// SAME shared loader (internal/specdocload), so their output must be
// byte-identical — not merely similar — whichever consumer a reader
// happens to be looking at. This file is that witness, plus its
// pinned-commit, no-readiness, and with-readiness arms. dex (the static
// docs site) never carries a readiness section at all — spec/spec-
// documents ac-4's own rule, "the docs site stays without readiness: a
// build-time artifact has no request to derive against" — so every
// readiness arm below excludes it from the byte-equality comparison,
// documented at each site rather than silently dropped.
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

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/workbench"
)

// TestDocumentParity_NoReadiness is spec/spec-documents ac-6 extended by
// spec/readiness-recovery ac-4/R-RR1-9's no-readiness arm: the CLI run
// with --no-readiness, the board and MCP wired with no loader at all,
// and the docs site (which never carries readiness) all agree byte-for-
// byte — the readiness section is uniformly and honestly absent, never
// present on some legs and missing on others. All four run over the same
// fixturegit store on its main checkout, where the working tree, HEAD,
// and the default branch coincide, so every consumer's reading is the
// accepted one.
func TestDocumentParity_NoReadiness(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSpecDocRepo(t)
	ctx := context.Background()
	env := []string{"CI_DEFAULT_BRANCH=main"}

	// 1. CLI, with --no-readiness: without it the CLI's own default loader
	// would derive readiness for real (spec/readiness-recovery ac-4), which
	// the other three legs below deliberately do not (no loader wired).
	bin := buildVerdiBinary(t)
	cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox", "--no-readiness")
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
	again, _, _ := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox", "--no-readiness")
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
// Extended by spec/readiness-recovery ac-4 (resolution g): the MCP leg is
// wired with a REAL, working ReadinessLoader for this exact ref — proving
// a pinned reading carries no readiness because ModeAt never calls the
// loader at all, not because none happened to be configured.
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

	// 2. MCP, pinned via the commit argument — a REAL loader is wired
	// (never a nil one), so "not supplied" below proves ModeAt's own
	// refusal, not an absent dependency.
	backend := &mcpserve.Backend{Root: repo.Dir, ReadinessLoader: readinessload.Loader{Root: repo.Dir, Opts: readinessload.Options{BoardHref: workbench.BranchBoardHref}}}
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
	if !strings.Contains(mcpDoc.Markdown, "## Readiness\n\nReadiness was not supplied for this render.") {
		t.Errorf("a pinned-commit render must state readiness absent even with a real loader wired:\n%s", mcpDoc.Markdown)
	}
	// The board is skipped here: it has no pinned-commit mode to compare
	// against (see the function doc comment).
}

// TestDocumentParity_BoardWithForeignReadinessSnapshot is ac-6 under a
// SHIPPED-SHAPED board wiring rather than an empty Deps (final-review F1),
// updated for spec/readiness-recovery ac-4 (Task 3): a REAL loader is
// wired (never a nil Deps.ReadinessLoader), but it returns a snapshot for
// ANOTHER spec — the ordinary case of a `verdi serve --context-request`
// process whose ONE startup request targets a different spec than the one
// this test renders. The loader hands that snapshot to
// specdoc.WithReadiness, which supplies it ONLY when snap.TargetRef names
// the spec being rendered (internal/specdoc/readiness.go). So a served
// process has two configurations, and this test pins the one where
// parity holds:
//
//   - TargetRef names a DIFFERENT spec (this test, and the ordinary case
//     — one loader's one request targets one spec, every other spec's
//     document is read with no readiness): WithReadiness returns the
//     facts untouched, the board renders "Readiness was not supplied for
//     this render." exactly as the CLI (run --no-readiness, so its own
//     default derivation never competes here), docs site, and MCP
//     (wired with no loader at all) do, and all four legs are
//     byte-identical. Proven here.
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

	// 1. CLI, with --no-readiness: this test's subject is the board's own
	// TargetRef guard, not the CLI's independent default derivation (proven
	// separately by TestDocumentParity_FourConsumersWithReadiness).
	bin := buildVerdiBinary(t)
	cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox", "--no-readiness")
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

	// 4. Board, wired the way serve.go wires it when the served process's
	// one --context-request targets a DIFFERENT spec: a real, working
	// loader that returns a snapshot for another ref. Every field carries
	// a marker string, so a leak is visible in the diff rather than merely
	// changing a byte.
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
	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{ReadinessLoader: fixedSnapshotLoader{snap: snap}})
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

// stripReadinessSection removes the "## Readiness" section from doc: its
// heading through whichever comes first of the next "## " heading (never
// actually reached — Readiness is always the last section, for both the
// spec and tasks kinds, specdoc/kind.go's Sections()) or the "\n---\n"
// rule RenderMarkdown appends after the last section, ahead of the
// "Derived from the spec's objects; not authority. Ref ..." provenance
// stamp (internal/specdoc/markdown.go). Fix round 1 F2 (task-4-review.md):
// the original version had no second terminator, so with no further "## "
// heading the strip ran to EOF and silently discarded that stamp line too
// — this version keeps it, so what's compared is "identical outside the
// Readiness section" in fact, not merely in the comment claiming it.
func stripReadinessSection(t *testing.T, doc string) string {
	t.Helper()
	const heading = "## Readiness"
	start := strings.Index(doc, heading)
	if start < 0 {
		t.Fatalf("document has no %q heading:\n%s", heading, doc)
	}
	rest := doc[start+len(heading):]
	end := len(rest)
	for _, terminator := range []string{"## ", "\n---\n"} {
		if i := strings.Index(rest, terminator); i >= 0 && i < end {
			end = i
		}
	}
	return doc[:start] + rest[end:]
}

// fixedSnapshotLoader is a test-only readiness loader returning the same
// snapshot for any ref (never an error) — it structurally satisfies both
// workbench.ReadinessLoader and mcpserve.ReadinessLoader (the `-er`
// pattern: each consumer declares an identical interface independently,
// never a shared one, so one concrete fake type serves both here). The
// guard that keeps a foreign snapshot from leaking
// (specdoc.WithReadiness's TargetRef tripwire, R-RR1-8) is exercised by
// the guard itself, not by this fake refusing a mismatched ref.
type fixedSnapshotLoader struct{ snap readinesspilot.Snapshot }

func (f fixedSnapshotLoader) Load(context.Context, string) (readinesspilot.Snapshot, error) {
	return f.snap, nil
}

// TestDocumentParity_FourConsumersWithReadiness is spec/readiness-recovery
// ac-4 (resolution g), superseding the narrower R-W3-3 proof it grew from:
// the CLI's own default derivation, MCP wired with `Backend{ReadinessLoader:
// real}`, and the board wired with `Deps{ReadinessLoader: real}` — three
// INDEPENDENTLY CONSTRUCTED internal/readinessload.Loader values, never one
// shared snapshot — each derive readiness for the SAME ref at the SAME HEAD
// and must produce byte-identical Markdown, carrying a genuinely populated
// Readiness section (ac-2's purity/determinism promise, end to end across
// three different calling code paths). dex is excluded: spec/spec-documents
// ac-4's own rule is that the static docs site never carries readiness at
// all (a build-time artifact has no request to derive against), so it is
// not part of this arm's byte-equality comparison.
func TestDocumentParity_FourConsumersWithReadiness(t *testing.T) {
	t.Setenv("CI_DEFAULT_BRANCH", "main")
	repo := buildSpecDocRepo(t)
	ctx := context.Background()
	env := []string{"CI_DEFAULT_BRANCH=main"}

	// 1. CLI — default: no --no-readiness, so its own internal loader
	// derives for real.
	bin := buildVerdiBinary(t)
	cliOut, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("cli exit %d: %s", code, stderr)
	}

	// 2. MCP, wired with its own independently-constructed real loader —
	// the exact same construction cmd/verdi's mcp.go and specdoc.go use.
	backend := &mcpserve.Backend{Root: repo.Dir, ReadinessLoader: readinessload.Loader{Root: repo.Dir, Opts: readinessload.Options{BoardHref: workbench.BranchBoardHref}}}
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

	// 3. Board, wired with ITS OWN independently-constructed real loader —
	// never the same Go value as MCP's above, proving parity is a property
	// of the derivation, not of sharing one loader instance.
	h := workbench.NewHandlerWith(repo.Dir, workbench.Deps{ReadinessLoader: readinessload.Loader{Root: repo.Dir, Opts: readinessload.Options{BoardHref: workbench.BranchBoardHref}}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/lockbox/document?format=md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("board: %d %s", rec.Code, rec.Body.String())
	}
	board := rec.Body.String()

	if cliOut != mcpDoc.Markdown {
		t.Errorf("CLI and MCP differ under independently-derived readiness:\n--- cli ---\n%s\n--- mcp ---\n%s", cliOut, mcpDoc.Markdown)
	}
	if cliOut != board {
		t.Errorf("CLI and board differ under independently-derived readiness:\n--- cli ---\n%s\n--- board ---\n%s", cliOut, board)
	}
	for name, doc := range map[string]string{"cli": cliOut, "board": board, "mcp": mcpDoc.Markdown} {
		if !strings.Contains(doc, "## Readiness") || strings.Contains(doc, "Readiness was not supplied for this render.") {
			t.Fatalf("%s must carry a populated Readiness section:\n%s", name, doc)
		}
		if !strings.Contains(doc, "Source: readiness snapshot for") {
			t.Fatalf("%s's populated Readiness section must name its own source line:\n%s", name, doc)
		}
	}

	// Turning readiness on changes EXACTLY the Readiness section, nowhere
	// else in the document: stripped of that section, this leg's own
	// --no-readiness render (the SAME ref, the SAME repo) must be
	// byte-identical to the populated CLI render stripped the same way.
	cliNoReadiness, stderr2, code2 := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox", "--no-readiness")
	if code2 != 0 {
		t.Fatalf("cli --no-readiness exit %d: %s", code2, stderr2)
	}
	if stripReadinessSection(t, cliOut) != stripReadinessSection(t, cliNoReadiness) {
		t.Errorf("readiness-populated and --no-readiness renders diverge by more than the Readiness section:\n--- populated (stripped) ---\n%s\n--- --no-readiness (stripped) ---\n%s", stripReadinessSection(t, cliOut), stripReadinessSection(t, cliNoReadiness))
	}
}

// TestDocumentParity_ServedWithContextRequest is ac-4 under the one
// shipped posture no other parity arm covers: `verdi serve
// --context-request <a request for spec X>`, whose ONE loader
// (cmd/verdi/serve.go) is threaded into the board, the board's Document
// tab and MCP alike. R-RR1-15 fixes what that loader owes each ref:
//
//   - Spec Y, which the request does not name, derives exactly as if no
//     request existed — so the CLI (which never has one), MCP and the
//     board are byte-identical, ac-4's own clause. Before the fix the
//     shared loader refused every ref but X, so Y's readiness was a loader
//     error on both served surfaces while the CLI rendered it in full: a
//     reachable byte divergence in normal operation, which this arm pins.
//   - Spec X, the request's own, keeps the whole request path on both
//     served surfaces (they agree byte for byte), and diverges from a CLI
//     that carries no request — in EXACTLY the Readiness section and
//     nowhere else, which stripReadinessSection states rather than
//     assumes. That divergence is a RECORDED SPEC CONFLICT, not an
//     accepted exception: ac-4 promises byte-identical readiness on all
//     four surfaces, ac-3 keeps --context-request as a served-only startup
//     pre-run, and spec/spec-documents ac-2 (which declares `verdi spec
//     doc`'s flag set; verdi-surfaces §CLI has no row for it) gives it no
//     request flag, so the CLI cannot answer the question the served
//     surfaces answer for X. After a commit makes the request's `expected`
//     claim stale (SI-214) the served side reads the stale-expected
//     witness instead of an evaluation — the same conflict, second shape. Ledger SI-215 (R-RRF-4) records the conflict as
//     disclosed-as-unproven for ac-4's startup-spec CLI arm and routes
//     the resolution (a CLI request flag, or an amendment defining parity
//     over equivalent request inputs) to the owner with SI-211. Until
//     that lands this arm pins the SHIPPED divergence so a change in it
//     is visible, and asserts nothing about its conformance.
//
// Both request-bound legs run through Options.ConflictProvider (R-RR1-14's
// hermetic seam, readinessLoadPassProviderFunc) so no judge process is
// ever launchable here (co-1), and they are INDEPENDENTLY constructed, so
// byte-equality is a property of the derivation rather than of a shared
// Go value. dex is excluded throughout: the static docs site never
// carries readiness at all (spec/spec-documents ac-4).
func TestDocumentParity_ServedWithContextRequest(t *testing.T) {
	repo := buildContextCompileRepo(t, map[string]string{
		".verdi/specs/active/feature-alpha/spec.md": contextFeatureAlphaSpec(t),
		".verdi/specs/active/lockbox/spec.md":       specDocFixture,
	})
	env := []string{"CI_DEFAULT_BRANCH=main"}
	bin := buildVerdiBinary(t)
	requestPath := writeContextRequestFile(t, repo.Dir, "readiness-request.json", contextRequestBytes(t, "spec/feature-alpha", contextcompile.PhaseDesign, nil))

	// The loader cmdServeWithDeps builds when --context-request is given:
	// root, BoardHref, the request path — plus the hermetic conflict seam
	// in place of the real provider. A fresh value per leg, never shared.
	servedLoader := func() readinessload.Loader {
		return readinessload.Loader{Root: repo.Dir, Opts: readinessload.Options{
			BoardHref:          workbench.BranchBoardHref,
			ContextRequestPath: requestPath,
			ConflictProvider:   readinessLoadPassProviderFunc(t),
		}}
	}

	// --- spec Y: not the request's spec ---------------------------------
	cliY, stderr, code := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/lockbox")
	if code != 0 {
		t.Fatalf("cli exit %d: %s", code, stderr)
	}
	mcpY := mcpDocumentMarkdown(t, &mcpserve.Backend{Root: repo.Dir, ReadinessLoader: servedLoader()}, "spec/lockbox")
	boardY := boardDocumentMarkdown(t, workbench.NewHandlerWith(repo.Dir, workbench.Deps{ReadinessLoader: servedLoader()}), "lockbox")

	if cliY != mcpY {
		t.Errorf("spec Y: CLI and MCP-under-a-foreign-request differ:\n--- cli ---\n%s\n--- mcp ---\n%s", cliY, mcpY)
	}
	if cliY != boardY {
		t.Errorf("spec Y: CLI and board-under-a-foreign-request differ:\n--- cli ---\n%s\n--- board ---\n%s", cliY, boardY)
	}
	for name, doc := range map[string]string{"cli": cliY, "mcp": mcpY, "board": boardY} {
		if !strings.Contains(doc, "Source: readiness snapshot for `spec/lockbox`") || strings.Contains(doc, "Readiness was not supplied for this render.") {
			t.Fatalf("spec Y's %s render must carry ITS OWN populated Readiness section:\n%s", name, doc)
		}
	}

	// --- spec X: the request's own spec ---------------------------------
	cliX, stderrX, codeX := runVerdiBinary(t, bin, repo.Dir, env, "spec", "doc", "spec/feature-alpha")
	if codeX != 0 {
		t.Fatalf("cli exit %d: %s", codeX, stderrX)
	}
	mcpX := mcpDocumentMarkdown(t, &mcpserve.Backend{Root: repo.Dir, ReadinessLoader: servedLoader()}, "spec/feature-alpha")
	boardX := boardDocumentMarkdown(t, workbench.NewHandlerWith(repo.Dir, workbench.Deps{ReadinessLoader: servedLoader()}), "feature-alpha")

	if mcpX != boardX {
		t.Errorf("spec X: the two served surfaces differ under the request they share:\n--- mcp ---\n%s\n--- board ---\n%s", mcpX, boardX)
	}
	if !strings.Contains(boardX, "Source: readiness snapshot for `spec/feature-alpha`") {
		t.Fatalf("spec X's served render must carry its own populated Readiness section:\n%s", boardX)
	}
	if cliX == boardX {
		t.Fatalf("spec X read identically with and without the startup request — the served loader is not applying it to its own spec:\n%s", boardX)
	}
	if stripReadinessSection(t, cliX) != stripReadinessSection(t, boardX) {
		t.Errorf("spec X: a startup request must change EXACTLY the Readiness section:\n--- cli (stripped) ---\n%s\n--- board (stripped) ---\n%s", stripReadinessSection(t, cliX), stripReadinessSection(t, boardX))
	}
}

// mcpDocumentMarkdown returns get_document's rendered Markdown for ref
// through backend — the same decode dance the arms above spell out inline,
// factored here for the two-spec arm that needs it four times.
func mcpDocumentMarkdown(t *testing.T, backend *mcpserve.Backend, ref string) string {
	t.Helper()
	args, err := json.Marshal(map[string]string{"ref": ref})
	if err != nil {
		t.Fatal(err)
	}
	res := backend.GetDocument(context.Background(), json.RawMessage(args))
	var payload struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	// The refusal diagnostic below prints raw, so a marshal failure must
	// not be swallowed into an empty one (final-review F8).
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshalling the mcp tool result: %v", err)
	}
	if err := json.Unmarshal(raw, &payload); err != nil || payload.IsError || len(payload.Content) == 0 {
		t.Fatalf("mcp result for %s: %s", ref, raw)
	}
	var doc struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal([]byte(payload.Content[0].Text), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Markdown
}

// boardDocumentMarkdown returns the board Document tab's Markdown for the
// spec named name.
func boardDocumentMarkdown(t *testing.T, h http.Handler, name string) string {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/board/spec/"+name+"/document?format=md", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("board document for %s: %d %s", name, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}
