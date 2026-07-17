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

	"github.com/jyang234/verdi/internal/fixturegit"
)

// corpusDir and violationsDir mirror internal/corpus and internal/index's
// own testdata references (../../testdata/... from this package).
const corpusDir = "../../examples/showcase"
const violationsDir = "../../testdata/violations"

// setupManifestYAML is a store manifest for the STANDALONE lint fixtures
// that build their own repo from scratch instead of chaining the committed
// examples/showcase tree (buildVL015Repo) — it is byte-identical to
// examples/showcase's own committed .verdi/verdi.yaml (layers.txt layer 1):
// forge: gitlab (so VL-012 expects gitlab-generated), a configured jira
// story-provider scheme (VL-005), and services.discovery: flowmap. The
// corpus-chaining harnesses (buildLintRepo, buildV2FixtureCorpusRepo) do
// NOT use this — they lint the committed manifest layers.txt already carries
// as layer 1, unmodified, so those gates prove the real store's own manifest
// rather than a synthetic variant.
const setupManifestYAML = `schema: verdi.layout/v1
forge: gitlab
providers:
  jira:
    base_url: https://example.atlassian.net
    rollup_field: customfield_00000
services:
  discovery: flowmap
`

// setupGitAttributes is the repository-root .gitattributes VL-012
// requires (02 §Repository plumbing's literal example, gitlab-generated
// token to match forge: gitlab) — byte-identical to examples/showcase's
// own committed .gitattributes, which layers.txt does not track (it lists
// .verdi/ content only), so the setup layer supplies it to the built repo.
const setupGitAttributes = `.verdi/specs/*/*/board.json          gitlab-generated
.verdi/specs/*/*/rollup.json         gitlab-generated
.verdi/specs/*/*/deviation-report.md gitlab-generated
`

// loansvcFlowmapYAML and loansvcBoundaryContractJSON satisfy
// examples/showcase's own `impacts: { ref: svc/loansvc/boundary-contract }`
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

// parseCorpusLayers reads examples/showcase/layers.txt (the same manifest
// internal/corpus and internal/index's own tests use) and returns, in
// ascending layer order, each layer's corpus-relative file paths as
// fixturegit Layers.
func parseCorpusLayers(t testing.TB) []fixturegit.Layer {
	t.Helper()
	f, err := os.Open(filepath.Join(corpusDir, "layers.txt"))
	if err != nil {
		t.Fatalf("opening layers.txt: %v", err)
	}
	defer func() { _ = f.Close() }()

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

// setupLayer is the fixed layer buildLintRepo/buildV2FixtureCorpusRepo add
// on top of the committed corpus: it carries ONLY the root .gitattributes
// (byte-identical to examples/showcase's committed .gitattributes), which
// VL-012 requires but which layers.txt does not track (layers.txt lists
// .verdi/ content only). The store manifest is deliberately NOT re-added
// here — the committed .verdi/verdi.yaml that layers.txt already carries as
// layer 1 stands unmodified, so the lint-clean gates prove the real store's
// own manifest, not a synthetic overwrite.
func setupLayer() fixturegit.Layer {
	return fixturegit.Layer{
		Files: map[string]string{
			".gitattributes": setupGitAttributes,
		},
		Message: "lint test setup: root .gitattributes",
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

// provisionMutableZone creates an empty (present, but recording nothing)
// data/mutable/annotations/ directory in root's untracked working tree —
// the default posture every buildLintRepo test repo gets, modeling an
// ordinary local checkout rather than VL-017's "bare CI clone" edge case
// (01 §Zones: the mutable zone is per-checkout, never committed). Tests
// that specifically exercise VL-017's mutable-zone-absent path build their
// own repo without calling this (or via a fresh t.TempDir()), rather than
// every other rule's test incidentally tripping VL-017 noise merely
// because buildLintRepo's corpus contains feature/story specs.
func provisionMutableZone(t *testing.T, root string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "data", "mutable", "annotations")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("provisioning mutable zone %s: %v", dir, err)
	}
}

// readTestdataFile reads path (relative to this package, e.g. under
// testdata/violations/) and fails the test on error.
func readTestdataFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading testdata file %s: %v", path, err)
	}
	return string(data)
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

// buildLintRepo builds the committed corpus (every layers.txt layer,
// including its committed .verdi/verdi.yaml) + the fixed setup layer (root
// .gitattributes only) + one additional layer per overlayDir (each layered
// as its own commit, in order), then overlays the loansvc discovery fixture
// untracked and provisions a present-but-empty mutable zone. Returns the
// built repo.
//
// The mutable zone is materialized present-but-empty rather than populated
// with examples/showcase's committed mutable/derived fixtures: the harness
// thus lints the committed tree with an empty mutable zone, which is sound
// because VL-017 — the only rule that reads the mutable zone (no rule reads
// the derived zone) — keys off zone PRESENCE, so a present-empty zone makes
// it run and disclose nothing rather than pass vacuously, and the committed
// annotations are all agent-task / board-only / grandfathered-target and so
// inert to VL-017 regardless. Populating the shared zone here would also
// collide with the per-test annotations vl017_test writes into it. VL-017's
// mutable-absent disclosure path is proven separately by
// TestV2FixtureCorpus_BareClone_OnlyVL017Disclosures.
func buildLintRepo(t *testing.T, overlayDirs ...string) *fixturegit.Repo {
	t.Helper()
	layers := parseCorpusLayers(t)
	layers = append(layers, setupLayer())
	for i, dir := range overlayDirs {
		layers = append(layers, overlayLayer(t, dir, fmt.Sprintf("overlay %d: %s", i, dir)))
	}
	repo := fixturegit.Build(t, layers)
	writeLoansvcFixture(t, repo.Dir)
	provisionMutableZone(t, repo.Dir)
	return repo
}

// knownCorpusBaselineFindings is the VL-020 tolerance list Task 1.2
// introduced for the merged examples/showcase corpus's four then-real
// findings (escrow-notify(-v2), refi-rate-check-2024's (ac, kind) pairs —
// dex-only surface fixtures folded from testdata/dexoverlay, deliberately
// never lint-clean at the time). public-rollout-plan Task 1.5 authored
// real obligations for all four
// (.verdi/obligations/escrow-notify/ac-1--behavioral.md;
// .verdi/obligations/escrow-notify-v2/ac-1--behavioral.md;
// .verdi/obligations/refi-rate-check-2024/ac-1--{static,behavioral}.md),
// so the corpus now genuinely produces zero VL-020 findings on its own —
// this list ends EMPTY, as Task 1.2's own doc comment anticipated
// ("Task 1.8 ... is expected to author real obligations for these and
// delete this filter"; landing earlier, in Task 1.5, changes nothing
// about the mechanism itself). The mechanism (map + filterKnownBaseline)
// stays rather than being deleted outright: it is still the single shared
// entry point every buildLintRepo/buildV2FixtureCorpusRepo-based test
// routes through, and remains available, empty, for any future genuinely
// pre-existing corpus debt — an empty map is a no-op filter, not a
// silently reintroduced tolerance.
//
// spec/attest-helper's VL-022 (an attestation's verifies edge must resolve
// to the (story, AC) its own path/slug implies) is exactly that future
// case, twice over: two committed showcase attestations carry a real
// `verifies` edge to a class: feature spec — R4-I-11's own "feature
// outcome attestation" convention (keyed by the FEATURE's own slug, never
// a story-ref slug at all), which predates VL-022 and sits outside a
// story-scoped rule's own remit (dc-5: "verdi attest scaffolds STORY
// attestations only" — VL-022 mirrors that same story-only scope).
// Genuine pre-existing corpus facts, not silently patched around: fixing
// them would mean either editing committed corpus fixture content well
// outside spec/attest-helper's brief (examples/showcase is shared by
// internal/corpus/internal/index's own tests too) or teaching VL-022 to
// special-case feature-class targets, which the frozen contract's own AC-3
// text does not carve out. Disclosed here, mirroring this map's own
// established purpose exactly as its doc comment above anticipates:
//   - .verdi/attestations/jira-loan-1482/ac-2.md (layers.txt layer 2,
//     reached by every buildLintRepo-based test): verifies
//     spec/stale-decline.
//   - .verdi/attestations/escrow-autopay/ac-1.md (buildV2FixtureCorpusRepo
//     only, v2clean_test.go): verifies spec/escrow-autopay.
var knownCorpusBaselineFindings = map[[3]string]bool{
	{"VL-022", ".verdi/attestations/jira-loan-1482/ac-2.md", `attestation attestation/jira-loan-1482--ac-2 verifies spec/stale-decline, a feature-class spec, not a STORY — verdi attest scaffolds STORY attestations only (spec/attest-helper dc-5)`}:  true,
	{"VL-022", ".verdi/attestations/escrow-autopay/ac-1.md", `attestation attestation/escrow-autopay--ac-1 verifies spec/escrow-autopay, a feature-class spec, not a STORY — verdi attest scaffolds STORY attestations only (spec/attest-helper dc-5)`}: true,
}

// filterKnownBaseline strips knownCorpusBaselineFindings (see its doc
// comment) from findings. Every direct NewEngine().Run(...) call site in
// this package's tests must route its result through this (or through
// runLint, which already does) — buildLintRepo and buildV2FixtureCorpusRepo
// both chain examples/showcase's real layers.txt content, so both inherit
// the same baseline.
func filterKnownBaseline(findings []Finding) []Finding {
	out := findings[:0:0]
	for _, f := range findings {
		if knownCorpusBaselineFindings[[3]string{f.Rule, f.Path, f.Message}] {
			continue
		}
		out = append(out, f)
	}
	return out
}

// runLint runs every rule over root and fails the test on an operational
// error (BuildSnapshot/service-discovery failure) — the tests below only
// ever expect Finding-shaped problems. Strips knownCorpusBaselineFindings
// (see its doc comment) before returning.
func runLint(t *testing.T, root string, lctx Context, opts Options) []Finding {
	t.Helper()
	findings, err := NewEngine().Run(context.Background(), root, lctx, opts)
	if err != nil {
		t.Fatalf("Engine.Run: %v", err)
	}
	return filterKnownBaseline(findings)
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

// commitPaths stages exactly the named paths and commits them under the
// fixed fixturegit identity — unlike commitAll's `git add -A`, it never
// sweeps in the untracked discovery/mutable-zone fixtures buildLintRepo
// leaves in the working tree (which would otherwise pollute a branch's diff).
func commitPaths(t *testing.T, dir, message string, paths ...string) {
	t.Helper()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Verdi Fixture", "GIT_AUTHOR_EMAIL=fixture@verdi.invalid", "GIT_AUTHOR_DATE=1704067200 +0000",
			"GIT_COMMITTER_NAME=Verdi Fixture", "GIT_COMMITTER_EMAIL=fixture@verdi.invalid", "GIT_COMMITTER_DATE=1704067200 +0000",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(append([]string{"add"}, paths...)...)
	run("commit", "--quiet", "--no-verify", "-m", message)
}

// gitCheckoutNewBranch cuts and switches to a new branch at the current
// HEAD under the same fixed identity fixturegit uses.
func gitCheckoutNewBranch(t *testing.T, dir, name string) {
	t.Helper()
	cmd := exec.Command("git", "checkout", "-b", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git checkout -b %s: %v\n%s", name, err, out)
	}
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

// onlyRules is onlyRule's multi-rule sibling: asserts every finding in got
// has Rule in the allowed set (and that there is at least one) — for a
// fixture that legitimately trips more than one rule at once (e.g. an ad hoc
// fixture predating VL-020 that, having no backing obligation, now also
// correctly trips it alongside whatever rule the fixture was originally
// built to test). Any rule id outside the allowed set is still the storm
// onlyRule guards against.
func onlyRules(t *testing.T, got []Finding, allowed ...string) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("no findings at all, want at least one of %v", allowed)
	}
	ok := make(map[string]bool, len(allowed))
	for _, r := range allowed {
		ok[r] = true
	}
	for _, f := range got {
		if !ok[f.Rule] {
			t.Fatalf("unexpected rule %s fired (rule storm): %s\nfull findings:\n%s", f.Rule, f.String(), findingsString(got))
		}
	}
}

// countRule counts how many findings in got have the given Rule id.
func countRule(got []Finding, rule string) int {
	n := 0
	for _, f := range got {
		if f.Rule == rule {
			n++
		}
	}
	return n
}
