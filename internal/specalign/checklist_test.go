// One named subtest per 00-index.md §v0 thin slice checklist bullet
// (deliverable 1b): each proves its bullet's shipped surface really
// exists and answers on THIS repo, so a regression names the checklist
// line it broke rather than a generic failure. This audits EXISTENCE —
// it deliberately does not re-run every behavioral test another
// package's own suite already owns (cmd/verdi/design_test.go,
// internal/align's tests, ...); this gate's job is proving the surface
// is really wired end to end here, not duplicating unit coverage.
package specalign

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/dex"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/workbench"
)

func TestV0ThinSliceChecklist(t *testing.T) {
	root := verdiRepoRoot

	// Checklist: "`verdi.yaml` + layout scaffold committed;
	// `.verdi/.gitignore` (`data/`)"
	t.Run("01_scaffold_verdiyaml_and_gitignore", func(t *testing.T) {
		if _, err := os.Stat(filepath.Join(root, ".verdi", "verdi.yaml")); err != nil {
			t.Fatalf(".verdi/verdi.yaml missing: %v", err)
		}
		gi, err := os.ReadFile(filepath.Join(root, ".verdi", ".gitignore"))
		if err != nil {
			t.Fatalf(".verdi/.gitignore missing: %v", err)
		}
		if strings.TrimRight(string(gi), "\n") != "data/" {
			t.Fatalf(".verdi/.gitignore must contain exactly %q, got %q", "data/\n", string(gi))
		}
	})

	// Checklist: "`artifactlint` VL-001..014, wired as a CI gate"
	//
	// The assertion is "no ERROR-severity (verdict) findings", not "zero
	// findings of any kind": a live self-hosted store legitimately carries
	// disclosure notices (SeverityDisclosure — e.g. VL-017's mutable-zone-
	// absent notice on any new-class spec authored in a checkout with no
	// data/mutable/ zone, by-design per its W2 adjudication). Those are the
	// three-valued-honesty channel, not lint failures (`verdi lint` still
	// exits 0), and they multiply as the store grows real work. A verdict
	// finding, by contrast, must still be zero.
	t.Run("02_artifactlint_no_error_severity_findings", func(t *testing.T) {
		ctx := context.Background()
		lctx := buildLintContext(ctx, root)
		findings, err := lint.NewEngine().Run(ctx, root, lctx, lint.Options{})
		if err != nil {
			t.Fatalf("lint.Engine.Run: %v", err)
		}
		var violations []lint.Finding
		for _, f := range findings {
			if f.Severity == lint.SeverityViolation {
				violations = append(violations, f)
			}
		}
		if len(violations) > 0 {
			var b strings.Builder
			for _, f := range violations {
				b.WriteString(f.String())
				b.WriteString("\n")
			}
			t.Fatalf("artifactlint found %d error-severity finding(s) running in-process against this repo's own store — want zero (disclosure notices are tolerated):\n%s", len(violations), b.String())
		}
	})

	// Checklist: "store walk + in-memory index; `search` and
	// `get_artifact` correct"
	t.Run("03_walk_index_six_specs_resolve", func(t *testing.T) {
		ix, err := index.Build(root)
		if err != nil {
			t.Fatalf("index.Build: %v", err)
		}
		want := []string{
			"spec/verdi-index",
			"spec/verdi-store-layout",
			"spec/verdi-artifact-contract",
			"spec/verdi-evidence-model",
			"spec/verdi-story-provider",
			"spec/verdi-surfaces",
		}
		for _, ref := range want {
			e, ok := ix.Get(ref)
			if !ok {
				t.Errorf("get_artifact: index does not resolve %s", ref)
				continue
			}
			if e.Kind != "spec" {
				t.Errorf("%s: Kind = %q, want %q", ref, e.Kind, "spec")
			}
		}
		results := ix.Search("artifact contract")
		found := false
		for _, r := range results {
			if r.Ref == "spec/verdi-artifact-contract" {
				found = true
			}
		}
		if !found {
			t.Errorf("search(%q) did not surface spec/verdi-artifact-contract among %d result(s)", "artifact contract", len(results))
		}
	})

	// Checklist: "`design start` -> board -> commit-to-design (VL-014
	// backstop) -> `accept` -> spec MR; `feature start` refuses
	// non-accepted specs"
	t.Run("04_lifecycle_verbs_are_real", func(t *testing.T) {
		for _, verb := range []string{"design", "accept", "feature", "board"} {
			_, stderr, code := runBinary(t, root, verb)
			assertNotOutOfV0(t, verb, stderr)
			if code != 2 {
				t.Errorf("verdi %s (no args): exit = %d, want 2 (usage error) — stderr: %q", verb, code, stderr)
			}
		}
	})

	// Checklist: "`sync --or-regen`, `matrix` (with `--preview`),
	// `align` (computed + judged, digest/integrity split)"
	t.Run("05_sync_matrix_preview_align_are_real", func(t *testing.T) {
		_, syncStderr, _ := runBinary(t, root, "sync", "--or-regen")
		assertNotOutOfV0(t, "sync", syncStderr)

		_, matrixStderr, _ := runBinary(t, root, "matrix", "--preview", "spec/verdi-index")
		assertNotOutOfV0(t, "matrix", matrixStderr)
		if strings.Contains(matrixStderr, "unexpected extra argument") {
			t.Errorf("matrix: --preview was rejected as an unrecognized extra argument (flag not accepted): %q", matrixStderr)
		}

		// align runs against a HERMETIC fixture (a fake judge on a
		// feature/<name> build branch), NEVER the live checkout: a plain
		// `go test ./...` must make no network call and leave no stray
		// deviation-report.md in this working tree (CLAUDE.md: "no network
		// in any test"; the judge is verdi.yaml's align.judge_cmd, which on
		// this repo is the live `claude -p` — off-limits to a test). The
		// fixture's own report lands inside its t.TempDir(), so this proves
		// align (computed + judged, report shape consumable) is really wired
		// end to end, hermetically.
		alignRoot := buildAlignFixtureRepo(t)
		alignStdout, alignStderr, alignCode := runBinary(t, alignRoot, "align")
		assertNotOutOfV0(t, "align", alignStderr)
		if alignCode != 0 {
			t.Fatalf("verdi align (hermetic fixture): exit = %d, want 0\nstdout: %q\nstderr: %q", alignCode, alignStdout, alignStderr)
		}
		reportPath := filepath.Join(alignRoot, ".verdi", "specs", "active", "align-smoke", "deviation-report.md")
		reportData, err := os.ReadFile(reportPath)
		if err != nil {
			t.Fatalf("verdi align: expected a deviation-report.md at %s: %v", reportPath, err)
		}
		fmBytes, _, err := artifact.SplitFrontmatter(reportData)
		if err != nil {
			t.Fatalf("verdi align: report at %s has no decodable frontmatter: %v", reportPath, err)
		}
		if _, err := artifact.DecodeDeviation(fmBytes); err != nil {
			t.Fatalf("verdi align: report shape is not consumable (DecodeDeviation): %v\n%s", err, reportData)
		}
	})

	// Checklist: "workbench: rendered corpus, verdict viewer with
	// cross-commit diff, board with autosave"
	t.Run("06_workbench_corpus_verdict_board_routes_wired", func(t *testing.T) {
		h := workbench.NewHandler(root)
		get := func(path string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			return rec
		}

		if rec := get("/"); rec.Code != http.StatusOK {
			t.Errorf("GET / = %d, want 200 (rendered corpus home)", rec.Code)
		}
		if rec := get("/a/spec/verdi-index"); rec.Code != http.StatusOK {
			t.Errorf("GET /a/spec/verdi-index = %d, want 200 (rendered corpus artifact page)", rec.Code)
		}
		if rec := get("/verdict/spec/verdi-index"); rec.Code >= http.StatusInternalServerError {
			t.Errorf("GET /verdict/... = %d, verdict viewer route not wired cleanly", rec.Code)
		}
		if rec := get("/board/smoke-test-key"); rec.Code >= http.StatusInternalServerError {
			t.Errorf("GET /board/... = %d, board route not wired cleanly", rec.Code)
		}
	})

	// Checklist: "`verdi serve` as the single writer (lock + socket);
	// `verdi mcp` shim; committed `.verdi/bin/` shims + `.mcp.json`"
	t.Run("07_serve_mcp_wired_and_shims_and_mcpjson", func(t *testing.T) {
		// serve/mcp both resolve the store root from "." before doing
		// anything else (socket bind, lock acquire, ...), so running
		// them from a rootless tempdir fails fast and honestly, without
		// starting a long-running process — proving the verb is wired
		// (not "not implemented") without needing to actually serve.
		rootless := t.TempDir()
		_, serveStderr, serveCode := runBinary(t, rootless, "serve")
		assertNotOutOfV0(t, "serve", serveStderr)
		if serveCode != 2 {
			t.Errorf("verdi serve (no store root): exit = %d, want 2", serveCode)
		}
		_, mcpStderr, mcpCode := runBinary(t, rootless, "mcp")
		assertNotOutOfV0(t, "mcp", mcpStderr)
		if mcpCode != 2 {
			t.Errorf("verdi mcp (no store root): exit = %d, want 2", mcpCode)
		}

		for _, shim := range []string{"verdi-mcp", "groundwork-mcp"} {
			p := filepath.Join(root, ".verdi", "bin", shim)
			if _, err := os.Stat(p); err != nil {
				t.Errorf(".verdi/bin/%s missing: %v", shim, err)
				continue
			}
			if out, err := exec.Command("sh", "-n", p).CombinedOutput(); err != nil {
				t.Errorf("sh -n %s: %v\n%s", p, err, out)
			}
		}

		data, err := os.ReadFile(filepath.Join(root, ".mcp.json"))
		if err != nil {
			t.Fatalf(".mcp.json missing: %v", err)
		}
		var doc struct {
			MCPServers map[string]struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"mcpServers"`
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf(".mcp.json: invalid JSON: %v", err)
		}
		for _, name := range []string{"verdi", "groundwork"} {
			s, ok := doc.MCPServers[name]
			if !ok {
				t.Errorf(".mcp.json: missing mcpServers.%s", name)
				continue
			}
			if s.Type != "stdio" {
				t.Errorf(".mcp.json: mcpServers.%s.type = %q, want %q", name, s.Type, "stdio")
			}
		}
	})

	// Checklist: "merge gate: accepted spec + no violated AC + fresh
	// fully-dispositioned alignment report (authoritative evidence
	// only)"
	t.Run("08_merge_gate_verb_present", func(t *testing.T) {
		_, stderr, code := runBinary(t, root, "gate")
		assertNotOutOfV0(t, "gate", stderr)
		if code == 0 {
			t.Errorf("verdi gate on main (not a build branch): exit = 0, expected a non-zero operational/verdict exit")
		}
	})

	// Checklist: "`rollup --publish` with the Jira adapter (field +
	// change-only comment)"
	t.Run("09_rollup_verb_present", func(t *testing.T) {
		_, stderr, _ := runBinary(t, root, "rollup")
		assertNotOutOfV0(t, "rollup", stderr)
		if !strings.Contains(stderr, "usage") {
			t.Errorf("verdi rollup (no args): expected a usage message, got %q", stderr)
		}
	})

	// Checklist: "`dex build` publishing to member-restricted Pages:
	// by-kind and by-service axes, temporal banners, backlinks, search
	// index, changelog"
	t.Run("10_dex_build_by_kind_and_by_service_axes", func(t *testing.T) {
		out := t.TempDir()
		if err := dex.Build(context.Background(), dex.Options{Root: root, OutDir: out}); err != nil {
			t.Fatalf("dex.Build: %v", err)
		}
		for _, p := range []string{
			filepath.Join("by-kind", "index.html"),
			filepath.Join("by-service", "index.html"),
		} {
			if _, err := os.Stat(filepath.Join(out, p)); err != nil {
				t.Errorf("dex build: missing %s: %v", p, err)
			}
		}
	})
}

// alignFakeJudgeScript is a tiny hermetic stand-in for the real `claude -p`
// judge, honoring spike S5's envelope shape (a JSON object whose `result`
// is itself the judge's `{"findings":[...]}` payload as a string). It reads
// nothing from the network and ignores its stdin prompt, returning an empty
// findings set deterministically — the established fake-judge pattern from
// internal/align's own tests, so `verdi align` runs its full judged section
// with no live LLM call (CLAUDE.md: "no network in any test").
const alignFakeJudgeScript = "#!/bin/sh\ncat <<'EOF'\n{\"is_error\":false,\"subtype\":\"success\",\"result\":\"{\\\"findings\\\":[]}\"}\nEOF\n"

// alignSmokeSpecMD is a minimal accepted feature spec with NO impacted
// services (so align's computed section is vacuously "(none)" and its
// RealRunner is constructed but never exec'd — no toolchain call, no
// network), letting `verdi align` reach and exercise the judged section
// against the fake judge and write a decodable deviation-report.md.
const alignSmokeSpecMD = `---
id: spec/align-smoke
kind: spec
class: feature
title: "Align Smoke"
status: accepted-pending-build
owners: [platform-team]
acceptance_criteria:
  - { id: ac-1, text: "static obligation holds", evidence: [static] }
frozen: { at: 2024-01-01, commit: 0000000000000000000000000000000000000a }
---
# body
`

// buildAlignFixtureRepo stands up a hermetic fixturegit store carrying one
// accepted feature spec and a fake judge (wired through verdi.yaml's
// align.judge_cmd), on the feature/<name> build branch `verdi build start`
// cuts (storyresolve.ResolveBuildSpec's inference target). Running `verdi
// align` here exercises the verb's full pipeline against a deterministic
// fake — never the live checkout, never the network — and its report lands
// inside this temp dir, so a plain `go test ./...` leaves no stray artifact
// in the developer's working tree. Returns the fixture's store root.
func buildAlignFixtureRepo(t *testing.T) string {
	t.Helper()

	judgeDir := t.TempDir()
	judgePath := filepath.Join(judgeDir, "fakejudge.sh")
	if err := os.WriteFile(judgePath, []byte(alignFakeJudgeScript), 0o755); err != nil {
		t.Fatalf("writing fake judge: %v", err)
	}

	// toolchain: is present so cmdAlign constructs a (real, but never
	// exec'd — the spec impacts no service) upstream.Runner; align.Compute
	// requires a non-nil Runner even when no service is impacted.
	verdiYAML := "schema: verdi.layout/v1\n" +
		"align:\n" +
		"  judge_cmd: [" + strconv.Quote(judgePath) + "]\n" +
		"  judge_required: false\n" +
		"toolchain:\n" +
		"  module: example.com/fake-toolchain\n" +
		"  commit: 0000000000000000000000000000000000000000\n"

	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/verdi.yaml":                       verdiYAML,
			".verdi/specs/active/align-smoke/spec.md": alignSmokeSpecMD,
		},
		Message: "scaffold + accepted spec",
	}})

	gitCheckoutBranch(t, repo.Dir, "feature/align-smoke")
	return repo.Dir
}

// gitCheckoutBranch cuts and switches to a new branch at HEAD in dir — the
// build-branch shape `verdi align` infers its spec from. A test
// infrastructure failure (not a verb-behavior result), so it fails the
// calling test outright.
func gitCheckoutBranch(t *testing.T, dir, branch string) {
	t.Helper()
	cmd := exec.Command("git", "checkout", "-q", "-b", branch)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b %s in %s: %v\n%s", branch, dir, err, out)
	}
}
