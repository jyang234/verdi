package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/upstream"
)

func TestLoadManifest_Happy(t *testing.T) {
	root := buildTestStore(t)
	m, err := loadManifest(root)
	if err != nil {
		t.Fatalf("loadManifest: %v", err)
	}
	if m.Forge != "gitlab" {
		t.Errorf("Forge = %q, want gitlab", m.Forge)
	}
	if m.Toolchain == nil || m.Toolchain.Module == "" {
		t.Errorf("Toolchain = %+v, want a populated toolchain block", m.Toolchain)
	}
}

func TestLoadManifest_Negative(t *testing.T) {
	root := t.TempDir()
	if _, err := loadManifest(root); err == nil {
		t.Fatal("loadManifest with no verdi.yaml: want error, got nil")
	}

	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".verdi", "verdi.yaml"), []byte("schema: not-the-right-schema\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadManifest(root); err == nil {
		t.Fatal("loadManifest with a bad schema: want error, got nil")
	}
}

func TestDecodeBundleFile_Negative(t *testing.T) {
	dir := t.TempDir()
	var out []int
	if err := decodeBundleFile(dir, "missing.json", &out); err == nil {
		t.Fatal("decodeBundleFile(missing file): want error, got nil")
	}

	if err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := decodeBundleFile(dir, "bad.json", &out); err == nil {
		t.Fatal("decodeBundleFile(malformed json): want error, got nil")
	}
}

func TestEvaluateBundle_Negative_MissingFiles(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := evaluateBundle(syncDeps{Stdout: &stdout, Stderr: &stderr}, dir)
	if code != 2 {
		t.Fatalf("evaluateBundle(no files): exit = %d, want 2", code)
	}
}

func TestLoadSpecACs_Happy(t *testing.T) {
	root := buildTestStore(t)
	acs, err := loadSpecACs(root, "spec/stale-decline")
	if err != nil {
		t.Fatalf("loadSpecACs: %v", err)
	}
	for _, want := range []string{"ac-1", "ac-2", "ac-3", "ac-4"} {
		if !acs[want] {
			t.Errorf("loadSpecACs missing %s: %v", want, acs)
		}
	}
}

func TestLoadSpecACs_Negative(t *testing.T) {
	root := buildTestStore(t)
	if _, err := loadSpecACs(root, "spec/does-not-exist"); err == nil {
		t.Fatal("loadSpecACs(unknown spec): want error, got nil")
	}
	if _, err := loadSpecACs(root, "not a valid ref"); err == nil {
		t.Fatal("loadSpecACs(malformed ref): want error, got nil")
	}
}

func TestListGoldenFlows_Happy(t *testing.T) {
	flows, err := listGoldenFlows(svcfixSrcDir)
	if err != nil {
		t.Fatalf("listGoldenFlows: %v", err)
	}
	if !flows["refund-flow"] {
		t.Errorf("listGoldenFlows = %v, want refund-flow present", flows)
	}
}

func TestListGoldenFlows_NoDirIsEmptyNotError(t *testing.T) {
	flows, err := listGoldenFlows(t.TempDir())
	if err != nil {
		t.Fatalf("listGoldenFlows(no testdata/flows dir): %v", err)
	}
	if len(flows) != 0 {
		t.Errorf("listGoldenFlows(no dir) = %v, want empty", flows)
	}
}

func TestWriteTempGraph_HappyAndCleanup(t *testing.T) {
	g := &upstream.Graph{Stamp: "deadbeef", Algo: "rta"}
	path, cleanup, err := writeTempGraph(g)
	if err != nil {
		t.Fatalf("writeTempGraph: %v", err)
	}
	defer cleanup()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("writeTempGraph did not create %s: %v", path, err)
	}
	decoded, err := upstream.DecodeGraph(mustRead(t, path))
	if err != nil {
		t.Fatalf("decoding scratch graph: %v", err)
	}
	if decoded.Stamp != "deadbeef" {
		t.Errorf("decoded.Stamp = %q, want deadbeef", decoded.Stamp)
	}

	cleanup()
	if _, err := os.Stat(path); err == nil {
		t.Error("cleanup did not remove the scratch graph file")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestGithubRepoName(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "acme/svcfix")
	if got := githubRepoName(); got != "svcfix" {
		t.Errorf("githubRepoName() = %q, want svcfix", got)
	}

	t.Setenv("GITHUB_REPOSITORY", "")
	if got := githubRepoName(); got != "" {
		t.Errorf("githubRepoName() (unset) = %q, want empty", got)
	}
}

func TestBuildForge_Negative_UnknownKind(t *testing.T) {
	if _, err := buildForge("bitbucket", ""); err == nil {
		t.Fatal("buildForge(unknown kind): want error, got nil")
	}
}

// TestBuildForge_Happy proves both forge kinds still build successfully
// given a resolvable identifier. The env vars are explicitly cleared so
// this test is hermetic even when `go test` itself runs inside real GitHub
// Actions (where GITHUB_REPOSITORY/GITHUB_REPOSITORY_OWNER are genuinely
// set) — the point is proving buildForge succeeds from the ORIGIN URL
// fallback, not incidentally from the runner's own environment.
func TestBuildForge_Happy(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY_OWNER", "")
	t.Setenv("GITHUB_REPOSITORY", "")
	if _, err := buildForge("gitlab", ""); err != nil {
		t.Errorf("buildForge(gitlab): %v", err)
	}
	// github now REQUIRES a resolvable identifier (spec/sync-local-flow
	// dc-2): the prior "empty remote URL still builds a forge" case
	// encoded exactly the silent-empty-identifier gap this story fixes,
	// and is deliberately replaced — see
	// TestBuildForge_Github_Negative_UnresolvableIdentifier below.
	if _, err := buildForge("github", "https://github.com/acme/svcfix.git"); err != nil {
		t.Errorf("buildForge(github, resolvable origin): %v", err)
	}
}

// TestBuildForge_Github_Negative_UnresolvableIdentifier proves buildForge
// itself refuses (rather than silently building a doomed adapter around
// two empty strings) when neither the CI env nor the origin URL identifies
// a GitHub repository — dc-2's fix, encoded directly at the construction
// seam sync.go's cmdSync calls unconditionally.
func TestBuildForge_Github_Negative_UnresolvableIdentifier(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY_OWNER", "")
	t.Setenv("GITHUB_REPOSITORY", "")
	if _, err := buildForge("github", ""); err == nil {
		t.Fatal("buildForge(github, no identifier anywhere): want error, got nil")
	}
}

// TestGithubOwnerRepo covers D6-14: the github owner/repo resolves from
// GitHub Actions' env when set, and otherwise falls back to the origin
// remote URL for a local run.
func TestGithubOwnerRepo(t *testing.T) {
	// Env is authoritative when set — the origin URL is not consulted.
	t.Setenv("GITHUB_REPOSITORY_OWNER", "envowner")
	t.Setenv("GITHUB_REPOSITORY", "envowner/envrepo")
	if o, r, err := githubOwnerRepo("https://github.com/urlowner/urlrepo.git"); o != "envowner" || r != "envrepo" || err != nil {
		t.Errorf("env should win: githubOwnerRepo = (%q, %q, %v), want (envowner, envrepo, nil)", o, r, err)
	}

	// Env unset → origin remote URL fallback (D6-14).
	t.Setenv("GITHUB_REPOSITORY_OWNER", "")
	t.Setenv("GITHUB_REPOSITORY", "")
	if o, r, err := githubOwnerRepo("git@github.com:urlowner/urlrepo.git"); o != "urlowner" || r != "urlrepo" || err != nil {
		t.Errorf("origin fallback: githubOwnerRepo = (%q, %q, %v), want (urlowner, urlrepo, nil)", o, r, err)
	}

	// Neither env nor a resolvable URL → the legible refusal (spec/
	// sync-local-flow ac-1), never the silently-returned empty pair the
	// prior "honest can't-identify case" comment named — that assumption
	// (some caller declines to build a doomed forge) is false for sync.go,
	// the one direct, ungated buildForge caller (dc-2).
	if o, r, err := githubOwnerRepo(""); o != "" || r != "" || err == nil {
		t.Errorf("no identifier: githubOwnerRepo = (%q, %q, %v), want (\"\", \"\", a non-nil error)", o, r, err)
	}
}

// TestGithubOwnerRepo_Negative_RefusesNamingEverySource proves the ac-1
// refusal names every source it tried — both env vars and the origin
// remote URL (or its documented absence) — not merely that some error
// occurred.
func TestGithubOwnerRepo_Negative_RefusesNamingEverySource(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY_OWNER", "")
	t.Setenv("GITHUB_REPOSITORY", "")

	t.Run("no origin remote at all", func(t *testing.T) {
		_, _, err := githubOwnerRepo("")
		if err == nil {
			t.Fatal("githubOwnerRepo(no env, no origin): want error, got nil")
		}
		for _, want := range []string{"GITHUB_REPOSITORY_OWNER", "GITHUB_REPOSITORY", "origin"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error = %q, want it to name %q", err.Error(), want)
			}
		}
	})

	t.Run("origin remote present but not github.com", func(t *testing.T) {
		const nonGithubRemote = "https://gitlab.com/urlowner/urlrepo.git"
		_, _, err := githubOwnerRepo(nonGithubRemote)
		if err == nil {
			t.Fatal("githubOwnerRepo(non-github origin): want error, got nil")
		}
		for _, want := range []string{"GITHUB_REPOSITORY_OWNER", "GITHUB_REPOSITORY", nonGithubRemote} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error = %q, want it to name %q", err.Error(), want)
			}
		}
	})
}

func TestResolveRefCommit_Negative_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := resolveRefCommit(context.Background(), dir); err == nil {
		t.Fatal("resolveRefCommit outside a git repo: want error, got nil")
	}
}
