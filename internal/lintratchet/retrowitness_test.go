package lintratchet

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
)

// The retro-witness's two commits (spec/strict-lint-target-v2 dc-4), spelled
// in full so no abbreviation can turn ambiguous. retroBefore still declares
// the two package-level readiness loader variables (cmd/verdi/serve.go:149
// and :150); retroAfter is the commit that removed them. Both build, and both
// are in main's history, so every full clone holds them (strict-lint-reach
// dc-2).
const (
	retroBefore = "410db101019797a35fc6c3ce8cd367864f38331c"
	retroAfter  = "e4d5b141a77e44e75702b9777d9e9c7c6bb4bf23"
)

// retroHolder is the package that declared the two variables, as a
// Finding's Package names it.
const retroHolder = "cmd/verdi"

// What a run without the pinned golangci-lint cannot show, as each skip
// reason says (S1-RR3).
const (
	retroUnshown     = "this run cannot show that the strict configuration reports serveReadinessLoader and serveReadinessDefaultSpec at 410db101 and neither at e4d5b141"
	lintRetroUnshown = "this run cannot show that the retro-witness's lint run returns a gated finding and refuses a package that does not type-check or a pattern that names no package"
)

// absentCommit is a well-formed commit id no repository here holds.
const absentCommit = "0123456789abcdef0123456789abcdef01234567"

// retroGlobals returns the two package-level variables the wave-1 review
// found by hand (process-audit PA-016).
func retroGlobals() []string {
	return []string{"serveReadinessLoader", "serveReadinessDefaultSpec"}
}

// retroPackages returns the packages that held them, as golangci-lint
// patterns (strict-lint-reach ac-1).
func retroPackages() []string {
	return []string{"./cmd/verdi/...", "./internal/readinessload/...", "./internal/store/..."}
}

// TestRetroWitness_ReadinessLoaderGlobals is the retro-witness
// (spec/strict-lint-reach ac-1, spec/strict-lint-target-v2 ac-3): the strict
// target reaches the class of defect that motivated it, two package-level
// readiness loader variables a wave-1 reviewer found by hand. It exports
// retroBefore and retroAfter from this repository's history with git
// archive, lints the packages that held the variables in each with this
// repository's .golangci.strict.yml and the Makefile's pinned golangci-lint,
// as make lint-strict runs it (linux/amd64, --issues-exit-code=0), offline,
// and requires gochecknoglobals to report both variables at retroBefore and
// neither at retroAfter (retroViolations).
//
// A clone that lacks either commit, a shallow clone, and a partial clone,
// whose export could fetch objects over the network, fail it before it looks
// for the linter, naming the commits, so none of them can pass it or skip it.
// A failed golangci-lint run, or a package that did not type-check, at either
// commit fails it too. It skips, saying what the run cannot show, only where
// the pinned golangci-lint is absent, as in CI's test jobs. CI job verify,
// which produces its evidence record, restores that binary (ledger SI-309),
// so there the record is pass or fail, and a skip would be recorded as
// abstain, never as a pass (dc-2). TestRetroWitnessDisposition proves how it
// ends in each case.
func TestRetroWitness_ReadinessLoaderGlobals(t *testing.T) {
	root := repoRoot(t)
	if err := retroHistory(t.Context(), root, retroBefore, retroAfter); err != nil {
		t.Fatalf("the retro-witness fails, and never passes or skips, without both commits in full history: %v", err)
	}
	bin := pinnedGolangciLint(t, root, retroUnshown)
	config := mustRead(t, filepath.Join(root, ".golangci.strict.yml"))

	lint := func(commit string) []Finding {
		t.Helper()
		tree := t.TempDir()
		if err := exportCommit(t.Context(), root, commit, tree); err != nil {
			t.Fatalf("exporting %s: %v", commit, err)
		}
		findings, err := lintRetroTree(t.Context(), bin, tree, config, retroPackages()...)
		if err != nil {
			t.Fatalf("linting %s: %v", commit, err)
		}
		for _, f := range findings {
			if slices.ContainsFunc(retroGlobals(), func(name string) bool { return reportsGlobal(f, name) }) {
				t.Logf("at %s: %s:%d: %s: %s", commit, f.File, f.Line, f.Key.Linter, f.Key.Message)
			}
		}
		return findings
	}
	before := lint(retroBefore)
	after := lint(retroAfter)
	for _, v := range retroViolations(before, after) {
		t.Error(v)
	}
}

// TestRetroWitnessDisposition re-runs the retro-witness in this test binary
// under each condition that must stop it and proves how it ends: a clone
// that lacks both commits and a shallow clone fail it, naming the commits,
// although PATH holds no golangci-lint, so neither can pass it or skip it; a
// full clone without the pinned golangci-lint skips it, saying what the run
// cannot show. A clone reaches the witness through GIT_DIR, git's own
// override of the repository it reads, and PATH holds git alone.
func TestRetroWitnessDisposition(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("locating git: %v", err)
	}
	gitOnly := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(gitOnly, "git")); err != nil {
		t.Fatal(err)
	}
	unrelated := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"README": "one\n"}, Message: "one"},
		{Files: map[string]string{"README": "two\n"}, Message: "two"},
	})
	shallow := fixturegit.ShallowClone(t, unrelated, 1)

	cases := []struct {
		name   string
		gitDir string // "" leaves git reading this repository
		skip   bool
		want   []string
	}{
		{name: "a clone that lacks both commits", gitDir: filepath.Join(unrelated.Dir, ".git"), want: []string{"lacks commit", retroBefore, retroAfter}},
		{name: "a shallow clone", gitDir: filepath.Join(shallow, ".git"), want: []string{"shallow", retroBefore, retroAfter}},
		{name: "a full clone without golangci-lint", skip: true, want: []string{"SKIP (disclosed, not a pass)", retroUnshown}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRetroWitness_ReadinessLoaderGlobals$", "-test.v", "-test.count=1")
			cmd.Env = append(os.Environ(), "PATH="+gitOnly)
			if tc.gitDir != "" {
				cmd.Env = append(cmd.Env, "GIT_DIR="+tc.gitDir)
			}
			out, err := cmd.CombinedOutput()
			got := string(out)
			ending, exitedZero := "--- FAIL: TestRetroWitness_ReadinessLoaderGlobals", false
			if tc.skip {
				ending, exitedZero = "--- SKIP: TestRetroWitness_ReadinessLoaderGlobals", true
			}
			if !strings.Contains(got, ending) || (err == nil) != exitedZero {
				t.Fatalf("the retro-witness did not end with %q (run error: %v):\n%s", ending, err, got)
			}
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("the retro-witness's output does not say %q:\n%s", w, got)
				}
			}
		})
	}
}

// global is a gochecknoglobals finding for the package-level variable name in
// package pkg.
func global(pkg, name string) Finding {
	return Finding{
		Key:  Key{Linter: "gochecknoglobals", Package: pkg, Message: name + " is a global variable", Source: "\t" + name + " int"},
		File: pkg + "/serve.go",
		Line: 1,
	}
}

// TestRetroViolations is retroViolations' happy and negative paths over
// hand-built findings. Each negative case names substrings one violation must
// contain.
func TestRetroViolations(t *testing.T) {
	const loader, spec = "serveReadinessLoader", "serveReadinessDefaultSpec"
	other := global(retroHolder, "serveOther")
	elsewhere := global("internal/store", "storeOther")
	misnamed := Finding{Key: Key{Linter: "errorlint", Package: retroHolder, Message: loader + " wraps an error with %v"}, File: "cmd/verdi/serve.go", Line: 2}
	longer := global(retroHolder, loader+"Cache")
	both := []Finding{global(retroHolder, loader), global(retroHolder, spec), other}

	cases := []struct {
		name          string
		before, after []Finding
		want          [][]string // each entry: substrings of one violation; nil wants none
	}{
		{name: "both reported before, neither after", before: both, after: []Finding{other, elsewhere}},
		{name: "the loader unreported before", before: []Finding{global(retroHolder, spec), other}, after: []Finding{other}, want: [][]string{{retroBefore, loader}}},
		{name: "the default spec unreported before", before: []Finding{global(retroHolder, loader), other}, after: []Finding{other}, want: [][]string{{retroBefore, spec}}},
		{name: "neither reported before", before: []Finding{other}, after: []Finding{other}, want: [][]string{{retroBefore, loader}, {retroBefore, spec}}},
		{name: "the loader still reported after", before: both, after: []Finding{global(retroHolder, loader), other}, want: [][]string{{retroAfter, loader}}},
		{name: "the default spec still reported after", before: both, after: []Finding{global("internal/store", spec), other}, want: [][]string{{retroAfter, spec}}},
		{name: "the loader named only by another linter before", before: []Finding{misnamed, global(retroHolder, spec), other}, after: []Finding{other}, want: [][]string{{retroBefore, loader}}},
		{name: "only a longer name containing the loader's before", before: []Finding{longer, global(retroHolder, spec), other}, after: []Finding{other}, want: [][]string{{retroBefore, loader}}},
		{name: "no package-level variable in cmd/verdi after", before: both, after: []Finding{elsewhere}, want: [][]string{{retroAfter, "no package-level variable in cmd/verdi"}}},
		{name: "nothing reported after", before: both, after: nil, want: [][]string{{retroAfter, "no package-level variable in cmd/verdi"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := retroViolations(tc.before, tc.after)
			if len(got) != len(tc.want) {
				t.Fatalf("retroViolations = %q, want %d violation(s) containing %q", got, len(tc.want), tc.want)
			}
			for _, subs := range tc.want {
				if !slices.ContainsFunc(got, func(v string) bool {
					return !slices.ContainsFunc(subs, func(s string) bool { return !strings.Contains(v, s) })
				}) {
					t.Errorf("retroViolations = %q, want one containing each of %q", got, subs)
				}
			}
		})
	}
}

// TestRetroHistory is retroHistory's happy and negative paths: this
// repository holds both commits; a clone lacking one, a clone lacking both, a
// shallow clone, a partial clone, and a directory outside any repository are
// each refused, naming what they lack.
func TestRetroHistory(t *testing.T) {
	root := repoRoot(t)
	unrelated := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"README": "one\n"}, Message: "one"},
		{Files: map[string]string{"README": "two\n"}, Message: "two"},
	})
	shallow := fixturegit.ShallowClone(t, unrelated, 1)
	partial := fixturegit.Build(t, []fixturegit.Layer{{Files: map[string]string{"README": "one\n"}, Message: "one"}})
	git(t, partial.Dir, "config", "core.repositoryformatversion", "1")
	git(t, partial.Dir, "config", "extensions.partialClone", "origin")

	cases := []struct {
		name    string
		dir     string
		commits []string
		want    []string // substrings of the error; nil wants no error
		notWant string
	}{
		{name: "this repository holds both commits", dir: root, commits: []string{retroBefore, retroAfter}},
		{name: "a clone that lacks one commit", dir: root, commits: []string{retroBefore, absentCommit}, want: []string{"lacks commit", absentCommit}, notWant: retroBefore},
		{name: "a clone that lacks both commits", dir: unrelated.Dir, commits: []string{retroBefore, retroAfter}, want: []string{"lacks commit", retroBefore, retroAfter}},
		{name: "a shallow clone that holds its commit", dir: shallow, commits: []string{unrelated.Head}, want: []string{"shallow", unrelated.Head}},
		{name: "a partial clone that holds its commit", dir: partial.Dir, commits: []string{partial.Head}, want: []string{"partial clone", "network", partial.Head}},
		{name: "a directory outside any repository", dir: t.TempDir(), commits: []string{retroBefore}, want: []string{"shallow"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := retroHistory(t.Context(), tc.dir, tc.commits...)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("retroHistory: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("retroHistory = nil, want an error containing %q", tc.want)
			}
			for _, w := range tc.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("retroHistory = %q, want it to contain %q", err, w)
				}
			}
			if tc.notWant != "" && strings.Contains(err.Error(), tc.notWant) {
				t.Errorf("retroHistory = %q, which names %s, a commit the clone holds", err, tc.notWant)
			}
		})
	}
}

// TestExportCommit is exportCommit's happy and negative paths over a
// fixturegit repository: the commit's tree, exactly, and a refusal naming a
// commit the repository lacks.
func TestExportCommit(t *testing.T) {
	files := map[string]string{"go.mod": "module example.com/x\n", "a/b/c.go": "package b\n", "README": "x\n"}
	repo := fixturegit.Build(t, []fixturegit.Layer{{Files: files, Message: "one"}})

	t.Run("the commit's tree", func(t *testing.T) {
		dst := t.TempDir()
		if err := exportCommit(t.Context(), repo.Dir, repo.Head, dst); err != nil {
			t.Fatalf("exportCommit: %v", err)
		}
		if got := treeFiles(t, dst); !mapsEqual(got, files) {
			t.Fatalf("exported %v, want %v", got, files)
		}
	})
	t.Run("a commit the repository lacks", func(t *testing.T) {
		err := exportCommit(t.Context(), repo.Dir, absentCommit, t.TempDir())
		if err == nil || !strings.Contains(err.Error(), absentCommit) {
			t.Fatalf("exportCommit = %v, want an error naming %s", err, absentCommit)
		}
	})
}

// treeFiles returns every regular file below dir, by slash-separated relative
// path, with its content.
func treeFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	got := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		got[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return got
}

// mapsEqual reports whether a and b hold the same entries.
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// tarEntry is one entry of a hand-built tar stream: its header and, for a
// regular file, its content.
type tarEntry struct {
	header tar.Header
	body   string
}

// tarOf returns the tar stream of entries.
func tarOf(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		h := e.header
		h.Size = int64(len(e.body))
		if err := tw.WriteHeader(&h); err != nil {
			t.Fatalf("writing header %q: %v", h.Name, err)
		}
		if _, err := io.WriteString(tw, e.body); err != nil {
			t.Fatalf("writing %q: %v", h.Name, err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestExtractTar is extractTar's happy and negative paths over hand-built tar
// streams.
func TestExtractTar(t *testing.T) {
	dir := func(name string) tarEntry {
		return tarEntry{header: tar.Header{Typeflag: tar.TypeDir, Name: name, Mode: 0o755}}
	}
	file := func(name, body string) tarEntry {
		return tarEntry{header: tar.Header{Typeflag: tar.TypeReg, Name: name, Mode: 0o644}, body: body}
	}
	globalHeader := tarEntry{header: tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header", PAXRecords: map[string]string{"comment": retroBefore}}}
	symlink := tarEntry{header: tar.Header{Typeflag: tar.TypeSymlink, Name: "l", Linkname: "f.go"}}
	// A 2,000-byte file cut 1,000 bytes into its content: one 512-byte
	// header block, then half the data.
	truncated := tarOf(t, file("big.go", strings.Repeat("x", 2000)))[:512+1000]

	cases := []struct {
		name   string
		stream []byte
		want   map[string]string
		errHas string // a substring of the error; "" wants none
	}{
		{name: "a directory and a file", stream: tarOf(t, dir("a/"), file("a/f.go", "package a\n")), want: map[string]string{"a/f.go": "package a\n"}},
		{name: "git archive's global header", stream: tarOf(t, globalHeader, file("f.go", "package f\n")), want: map[string]string{"f.go": "package f\n"}},
		{name: "a file whose directory has no entry", stream: tarOf(t, file("x/y/z.go", "package y\n")), want: map[string]string{"x/y/z.go": "package y\n"}},
		{name: "an empty stream", stream: tarOf(t), want: map[string]string{}},
		{name: "a symbolic link", stream: tarOf(t, symlink), errHas: "tar type"},
		{name: "a path above the export", stream: tarOf(t, file("../escape.go", "x")), errHas: "outside the export"},
		{name: "an absolute path", stream: tarOf(t, file("/abs.go", "x")), errHas: "outside the export"},
		{name: "the same file twice", stream: tarOf(t, file("f.go", "a"), file("f.go", "b")), errHas: "f.go"},
		{name: "a truncated stream", stream: truncated, errHas: "unexpected EOF"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			err := extractTar(bytes.NewReader(tc.stream), dst)
			if tc.errHas != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errHas) {
					t.Fatalf("extractTar = %v, want an error containing %q", err, tc.errHas)
				}
				return
			}
			if err != nil {
				t.Fatalf("extractTar: %v", err)
			}
			if got := treeFiles(t, dst); !mapsEqual(got, tc.want) {
				t.Fatalf("extracted %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLintRetroTree is lintRetroTree's happy and negative paths over small
// modules, linted by the pinned golangci-lint with this repository's strict
// configuration: a gated finding comes back keyed, while a package that does
// not type-check and a pattern naming no package are errors, never an empty
// result. It skips where the pinned golangci-lint is absent.
func TestLintRetroTree(t *testing.T) {
	root := repoRoot(t)
	bin := pinnedGolangciLint(t, root, lintRetroUnshown)
	config := mustRead(t, filepath.Join(root, ".golangci.strict.yml"))
	module := func(t *testing.T, files map[string]string) string {
		t.Helper()
		tree := t.TempDir()
		files["go.mod"] = "module example.com/retro\n\ngo 1.25\n"
		for name, body := range files {
			path := filepath.Join(tree, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return tree
	}

	t.Run("a package-level variable", func(t *testing.T) {
		tree := module(t, map[string]string{"g/g.go": "package g\n\nvar Counter int\n"})
		findings, err := lintRetroTree(t.Context(), bin, tree, config, "./g/...")
		if err != nil {
			t.Fatalf("lintRetroTree: %v", err)
		}
		want := []Finding{{Key: Key{Linter: "gochecknoglobals", Package: "g", Message: "Counter is a global variable", Source: "var Counter int"}, File: "g/g.go", Line: 3}}
		if !slices.Equal(findings, want) {
			t.Fatalf("lintRetroTree = %+v, want %+v", findings, want)
		}
	})
	t.Run("a package that does not type-check", func(t *testing.T) {
		tree := module(t, map[string]string{"b/b.go": "package b\n\nvar _ = undefinedName\n"})
		_, err := lintRetroTree(t.Context(), bin, tree, config, "./b/...")
		if err == nil || !strings.Contains(err.Error(), "typecheck") {
			t.Fatalf("lintRetroTree = %v, want an error naming the typecheck failure", err)
		}
	})
	t.Run("a pattern that names no package", func(t *testing.T) {
		tree := module(t, map[string]string{"g/g.go": "package g\n"})
		_, err := lintRetroTree(t.Context(), bin, tree, config, "./nowhere/...")
		if err == nil || !strings.Contains(err.Error(), "golangci-lint run") {
			t.Fatalf("lintRetroTree = %v, want an error naming the failed golangci-lint run", err)
		}
	})
}

// retroHistory returns why the repository at dir cannot give the witness
// commits from full history, or nil when it can, and names the commits: a
// shallow clone has cut history, a partial clone would fetch a missing object
// over the network as soon as git read it, and a clone without a commit
// cannot export it. The shallow and partial checks come first, because
// looking a commit up in a partial clone is itself such a read.
func retroHistory(ctx context.Context, dir string, commits ...string) error {
	shallow, err := gitx.IsShallow(ctx, dir)
	if err != nil {
		return fmt.Errorf("checking whether this clone is shallow: %w", err)
	}
	if shallow {
		return fmt.Errorf("this clone is shallow, so it cannot be relied on to hold %s; the witness needs a full clone", strings.Join(commits, " and "))
	}
	switch promisor, err := gitx.ConfigValue(ctx, dir, "extensions.partialClone"); {
	case err == nil:
		return fmt.Errorf("this clone is a partial clone of %q, so reading %s could fetch missing objects over the network, which no test may reach; the witness needs a full clone", promisor, strings.Join(commits, " and "))
	case !errors.Is(err, gitx.ErrConfigUnset):
		return fmt.Errorf("checking whether this clone is a partial clone: %w", err)
	}
	var missing []string
	for _, c := range commits {
		ok, err := gitx.CommitExists(ctx, dir, c)
		if err != nil {
			return fmt.Errorf("looking for commit %s: %w", c, err)
		}
		if !ok {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("this clone lacks commit %s, which the witness exports from this repository's history; any full clone holds it", strings.Join(missing, " and "))
	}
	return nil
}

// exportCommit writes the tree of commit, as git archive exports it from the
// repository at repo, into dst.
func exportCommit(ctx context.Context, repo, commit, dst string) error {
	cmd := exec.CommandContext(ctx, "git", "archive", "--format=tar", commit)
	cmd.Dir = repo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("git archive %s: %w", commit, err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("git archive %s: %w", commit, err)
	}
	extractErr := extractTar(stdout, dst)
	// Read what extraction left, so git never blocks writing and Wait returns.
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("git archive %s: %w: %s", commit, err, strings.TrimSpace(stderr.String()))
	}
	if extractErr != nil {
		return fmt.Errorf("extracting git archive %s: %w", commit, extractErr)
	}
	return nil
}

// extractTar writes the directories and regular files of the tar stream r
// below dst. It refuses an entry whose path leaves dst, a file written twice,
// and any entry type git archive does not write for this repository's
// commits, a symbolic link among them, so an export is never silently
// partial. git archive's global header, which records the commit id, holds no
// file.
func extractTar(r io.Reader, dst string) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(h.Name)) {
			return fmt.Errorf("entry %q names a path outside the export", h.Name)
		}
		path := filepath.Join(dst, filepath.FromSlash(h.Name))
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := writeExportedFile(tr, path, h.FileInfo().Mode().Perm()); err != nil {
				return fmt.Errorf("entry %q: %w", h.Name, err)
			}
		default:
			return fmt.Errorf("entry %q has tar type %q, which the export does not write", h.Name, h.Typeflag)
		}
	}
}

// writeExportedFile writes r's bytes to a new file at path with mode perm,
// creating its directory; a file already at path is an error.
func writeExportedFile(r io.Reader, path string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// lintRetroTree runs the pinned golangci-lint bin over patterns in tree with
// the strict configuration config, written to the tree's root so golangci-lint
// reports paths relative to it, as make lint-strict runs it (linux/amd64,
// --issues-exit-code=0), offline, and returns the findings. A nonzero exit, a
// run error in the report, or a package that did not type-check is an error,
// never an empty result.
func lintRetroTree(ctx context.Context, bin, tree string, config []byte, patterns ...string) ([]Finding, error) {
	cfg := filepath.Join(tree, ".golangci.strict.yml")
	if err := os.WriteFile(cfg, config, 0o644); err != nil {
		return nil, fmt.Errorf("writing the strict configuration: %w", err)
	}
	report := filepath.Join(tree, "retro-witness-report.json")
	if out, err := strictLintRun(ctx, bin, tree, cfg, report, patterns...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("golangci-lint run over %s: %w, which is a failure, never an empty report:\n%s", strings.Join(patterns, " "), err, out)
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return nil, fmt.Errorf("reading golangci-lint's report: %w", err)
	}
	return ParseReport(data)
}

// retroViolations returns every way the findings at retroBefore and
// retroAfter depart from the witness's claim: gochecknoglobals reports each
// of retroGlobals at retroBefore and none of them at retroAfter, where it
// still reports another package-level variable in retroHolder, so its silence
// about the two is a reading of the package that held them, never that of a
// package it did not analyze.
func retroViolations(before, after []Finding) []string {
	var out []string
	for _, name := range retroGlobals() {
		if !slices.ContainsFunc(before, func(f Finding) bool { return reportsGlobal(f, name) }) {
			out = append(out, fmt.Sprintf("at %s gochecknoglobals does not report %s, which that commit declares at package level", retroBefore, name))
		}
		for _, f := range after {
			if reportsGlobal(f, name) {
				out = append(out, fmt.Sprintf("at %s, the commit that removed it, gochecknoglobals still reports %s (%s:%d: %s)", retroAfter, name, f.File, f.Line, f.Key.Message))
			}
		}
	}
	if !slices.ContainsFunc(after, func(f Finding) bool { return f.Key.Linter == "gochecknoglobals" && f.Key.Package == retroHolder }) {
		out = append(out, fmt.Sprintf("at %s gochecknoglobals reports no package-level variable in %s, so its silence about %s does not show it analyzed the package that held them", retroAfter, retroHolder, strings.Join(retroGlobals(), " and ")))
	}
	return out
}

// reportsGlobal reports whether f is gochecknoglobals reporting the
// package-level variable name: its message names name as a whole word.
func reportsGlobal(f Finding, name string) bool {
	return f.Key.Linter == "gochecknoglobals" && regexp.MustCompile(`\b`+regexp.QuoteMeta(name)+`\b`).MatchString(f.Key.Message)
}
