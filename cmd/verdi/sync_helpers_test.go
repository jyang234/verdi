package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/OWNER/verdi/internal/upstream"
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
	if _, err := buildForge("bitbucket"); err == nil {
		t.Fatal("buildForge(unknown kind): want error, got nil")
	}
}

func TestBuildForge_Happy(t *testing.T) {
	for _, kind := range []string{"gitlab", "github"} {
		if _, err := buildForge(kind); err != nil {
			t.Errorf("buildForge(%q): %v", kind, err)
		}
	}
}

func TestResolveRefCommit_Negative_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := resolveRefCommit(context.Background(), dir); err == nil {
		t.Fatal("resolveRefCommit outside a git repo: want error, got nil")
	}
}
