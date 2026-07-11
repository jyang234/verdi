package lint

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/fixturegit"
)

// corpusDir and violationsDir mirror internal/corpus and internal/index's
// own testdata references (../../testdata/... from this package).
const corpusDir = "../../testdata/corpus"
const violationsDir = "../../testdata/violations"

// setupManifestYAML is the store manifest every lint test's repo carries —
// forge: gitlab (so VL-012 expects gitlab-generated), a configured jira
// story-provider scheme (VL-005), and an empty gated_generated allowlist
// (VL-008). Kept separate from testdata/corpus itself (which carries no
// verdi.yaml of its own) so this package never touches phase 2/3's golden
// corpus fixture or its pinned SHAs.
const setupManifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
lint:
  gated_generated: []
derived:
  retention_days: 14
services:
  discovery: flowmap
`

// setupGitAttributes is the repository-root .gitattributes VL-012
// requires (02 §Repository plumbing's literal example, gitlab-generated
// token to match setupManifestYAML's forge: gitlab).
const setupGitAttributes = `.verdi/specs/*/*/board.json          gitlab-generated
.verdi/specs/*/*/rollup.json         gitlab-generated
.verdi/specs/*/*/deviation-report.md gitlab-generated
`

// loansvcFlowmapYAML and loansvcBoundaryContractJSON satisfy
// testdata/corpus's own `impacts: { ref: svc/loansvc/boundary-contract }`
// link (stale-decline/spec.md) — the corpus fixture names a "loansvc"
// service that phase 3's own svcfix fixture (named "svcfix") does not
// provide. Written directly to the built repo's working tree, untracked:
// store.DiscoverServices reads the filesystem directly, not git (01
// §notes: "the store reads them in place"), exactly like
// internal/index/golden_test.go's own svcfix overlay.
const loansvcFlowmapYAML = "version: 1\nservice: loansvc\n"
const loansvcBoundaryContractJSON = `{
  "service": "loansvc",
  "schema_version": "flowmap.boundary/v1",
  "entrypoints": { "http": [], "consumers": [] },
  "published": [],
  "consumed": [],
  "external_dependencies": [],
  "blind_spots": []
}
`

// parseCorpusLayers reads testdata/corpus/layers.txt (the same manifest
// internal/corpus and internal/index's own tests use) and returns, in
// ascending layer order, each layer's corpus-relative file paths as
// fixturegit Layers.
func parseCorpusLayers(t testing.TB) []fixturegit.Layer {
	t.Helper()
	f, err := os.Open(filepath.Join(corpusDir, "layers.txt"))
	if err != nil {
		t.Fatalf("opening layers.txt: %v", err)
	}
	defer f.Close()

	filesByLayer := map[int][]string{}
	var order []int
	seen := map[int]bool{}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			t.Fatalf("layers.txt: malformed line %q", line)
		}
		var n int
		if _, err := fmt.Sscanf(parts[0], "%d", &n); err != nil {
			t.Fatalf("layers.txt: bad layer number in %q: %v", line, err)
		}
		rel := strings.TrimSpace(parts[1])
		filesByLayer[n] = append(filesByLayer[n], rel)
		if !seen[n] {
			order = append(order, n)
			seen[n] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scanning layers.txt: %v", err)
	}
	sort.Ints(order)

	layers := make([]fixturegit.Layer, 0, len(order))
	for _, n := range order {
		files := map[string]string{}
		for _, rel := range filesByLayer[n] {
			data, err := os.ReadFile(filepath.Join(corpusDir, rel))
			if err != nil {
				t.Fatalf("reading corpus file %s (layer %d): %v", rel, n, err)
			}
			files[rel] = string(data)
		}
		layers = append(layers, fixturegit.Layer{Files: files, Message: fmt.Sprintf("layer %d", n)})
	}
	return layers
}

// setupLayer is the fixed fourth layer every lint test repo gets: the
// store manifest and root .gitattributes.
func setupLayer() fixturegit.Layer {
	return fixturegit.Layer{
		Files: map[string]string{
			".verdi/verdi.yaml": setupManifestYAML,
			".gitattributes":    setupGitAttributes,
		},
		Message: "phase 4 lint test setup: manifest + gitattributes",
	}
}

// overlayLayer reads every file under dir (recursively) into a fixturegit
// Layer, preserving relative paths — testdata/violations' overlay
// directories are already store-root-relative (.gitattributes at top,
// everything else under .verdi/), so this is a verbatim copy.
func overlayLayer(t *testing.T, dir, message string) fixturegit.Layer {
	t.Helper()
	files := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("reading overlay dir %s: %v", dir, err)
	}
	if len(files) == 0 {
		t.Fatalf("overlay dir %s contributed no files", dir)
	}
	return fixturegit.Layer{Files: files, Message: message}
}

// writeLoansvcFixture writes the untracked "loansvc" service discovery
// fixture into root's working tree (see loansvcFlowmapYAML's doc comment).
func writeLoansvcFixture(t *testing.T, root string) {
	t.Helper()
	writeTestFile(t, filepath.Join(root, "loansvc", ".flowmap.yaml"), loansvcFlowmapYAML)
	writeTestFile(t, filepath.Join(root, "loansvc", ".flowmap", "boundary-contract.json"), loansvcBoundaryContractJSON)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// buildLintRepo builds the corpus (3 golden layers) + the fixed setup
// layer + one additional layer per overlayDir (each layered as its own
// commit, in order), then overlays the loansvc discovery fixture
// untracked. Returns the built repo.
func buildLintRepo(t *testing.T, overlayDirs ...string) *fixturegit.Repo {
	t.Helper()
	layers := parseCorpusLayers(t)
	layers = append(layers, setupLayer())
	for i, dir := range overlayDirs {
		layers = append(layers, overlayLayer(t, dir, fmt.Sprintf("overlay %d: %s", i, dir)))
	}
	repo := fixturegit.Build(t, layers)
	writeLoansvcFixture(t, repo.Dir)
	return repo
}

// runLint runs every rule over root and fails the test on an operational
// error (BuildSnapshot/service-discovery failure) — the tests below only
// ever expect Finding-shaped problems.
func runLint(t *testing.T, root string, lctx Context, opts Options) []Finding {
	t.Helper()
	findings, err := NewEngine().Run(context.Background(), root, lctx, opts)
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	return findings
}

// findingsString renders findings for a test failure message.
func findingsString(findings []Finding) string {
	var b strings.Builder
	for _, f := range findings {
		b.WriteString(f.String())
		b.WriteString("\n")
	}
	return b.String()
}

// mustRemove removes path, failing the test on error — used to model a
// real git rename (remove + write, see internal/gitx's own diff_test.go
// for why `git mv` itself is avoided in these harnesses).
func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("removing %s: %v", path, err)
	}
}

// commitAll stages every change in dir and commits it under the same fixed
// identity fixturegit uses, so SHAs stay deterministic within a test run
// (though these ad hoc post-Build commits are never golden-pinned).
func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid", "GIT_AUTHOR_DATE=1704067200 +0000",
			"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid", "GIT_COMMITTER_DATE=1704067200 +0000",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("add", "-A")
	run("commit", "--quiet", "--no-verify", "-m", message)
}

// onlyRule asserts every finding in got has Rule == want and that there is
// at least one — "assert THE EXACT RULE fires (rule-id equality, and no
// unrelated rule storm)" (PLAN.md §4): multiple findings from the tested
// rule itself are fine (e.g. VL-004 enforced against a corpus that already
// has an unrelated draft spec plus the overlay's), but any other rule id
// appearing is the storm this guards against.
func onlyRule(t *testing.T, got []Finding, want string) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("no findings at all, want at least one %s", want)
	}
	for _, f := range got {
		if f.Rule != want {
			t.Fatalf("unexpected rule %s fired (rule storm): %s\nfull findings:\n%s", f.Rule, f.String(), findingsString(got))
		}
	}
}
