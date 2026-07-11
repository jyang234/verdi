package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// provisionStore builds a scratch store at storeRoot: testdata/corpus's
// committed zone (.verdi/{specs,adr,diagrams,attestations,waivers,
// conflicts}) as one real git commit (a fresh, throwaway repo — not
// fixturegit's golden-SHA-pinned build, since nothing here asserts a
// specific commit hash; it only needs REAL git history for gitx's
// RevParse/CommitDate/AddAll/CreateCommit, which `verdi serve`'s backend
// and commit-to-design both exercise), plus testdata/corpus's mutable/ and
// derived/ trees copied in UNTRACKED (VL-013: never git-add those).
//
// A minimal verdi.yaml is written too (not load-bearing for anything this
// suite drives, since storyresolve/evidence/commitdesign none of them
// require the manifest — kept for parity with a real store, and so a
// human poking at the scratch store with other verdi verbs sees a
// legible one).
func provisionStore(moduleRoot, storeRoot string) error {
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		return err
	}

	corpusDir := filepath.Join(moduleRoot, "testdata", "corpus")
	if err := copyTree(filepath.Join(corpusDir, ".verdi"), filepath.Join(storeRoot, ".verdi")); err != nil {
		return fmt.Errorf("copying committed zone: %w", err)
	}

	// Fold in testdata/svcfix as a real service root (it carries a
	// .flowmap.yaml plus a .flowmap/boundary-contract.json), so the built
	// dex site gains a by-service axis and a boundary-contract permalink
	// whose JSON is pretty-printed through chroma — the one e2e page with a
	// highlighted code block, which the dark-mode syntax-highlighting check
	// (05-dex.spec) needs something real to assert against.
	if err := copyTree(filepath.Join(moduleRoot, "testdata", "svcfix"), filepath.Join(storeRoot, "svcfix")); err != nil {
		return fmt.Errorf("copying svcfix service: %w", err)
	}

	manifest := "schema: verdi.layout/v1\nforge: gitlab\nproviders:\n  jira:\n    base_url: https://example.atlassian.net\n    rollup_field: customfield_00000\nservices:\n  discovery: flowmap\n"
	if err := os.WriteFile(filepath.Join(storeRoot, ".verdi", "verdi.yaml"), []byte(manifest), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(storeRoot, ".verdi", ".gitignore"), []byte("data/\n"), 0o644); err != nil {
		return err
	}
	gitattrs := ".verdi/specs/*/*/board.json          gitlab-generated\n.verdi/specs/*/*/rollup.json         gitlab-generated\n.verdi/specs/*/*/deviation-report.md gitlab-generated\n"
	if err := os.WriteFile(filepath.Join(storeRoot, ".gitattributes"), []byte(gitattrs), 0o644); err != nil {
		return err
	}

	if err := gitInitAndCommit(storeRoot); err != nil {
		return fmt.Errorf("git init/commit: %w", err)
	}

	if err := copyTree(filepath.Join(corpusDir, "mutable"), filepath.Join(storeRoot, ".verdi", "data", "mutable")); err != nil {
		return fmt.Errorf("copying mutable zone: %w", err)
	}
	if err := copyTree(filepath.Join(corpusDir, "derived"), filepath.Join(storeRoot, ".verdi", "data", "derived")); err != nil {
		return fmt.Errorf("copying derived zone: %w", err)
	}

	return nil
}

func gitInitAndCommit(dir string) error {
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=verdi-e2e", "GIT_AUTHOR_EMAIL=e2e@verdi.invalid", "GIT_AUTHOR_DATE=1704067200 +0000",
		"GIT_COMMITTER_NAME=verdi-e2e", "GIT_COMMITTER_EMAIL=e2e@verdi.invalid", "GIT_COMMITTER_DATE=1704067200 +0000",
	)
	run := func(args ...string) error {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %v: %w\n%s", args, err, out)
		}
		return nil
	}
	if err := run("init", "--quiet", "--initial-branch=main"); err != nil {
		return err
	}
	if err := run("add", "-A"); err != nil {
		return err
	}
	return run("commit", "--quiet", "--no-verify", "-m", "e2e scratch store: seeded from testdata/corpus")
}

// copyTree recursively copies every regular file under src to dst.
func copyTree(src, dst string) error {
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() // read-only source; close error is unactionable
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	// Propagate the write-path Close: a swallowed close on a written file
	// can hide a short/truncated copy (previously deferred and dropped).
	return out.Close()
}
