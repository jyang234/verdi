package main

// indexDatesFixture (spec/index-data ac-3; SI-296): the one harness store
// whose index entries carry KNOWN, DIFFERENT last-change dates on both
// sides of a fixed clock, served by the shipped binary's own `verdi serve`
// with VERDI_NOW set to that clock — so the served index page itself (the
// one source; SI-297's data-last-change / data-quiet carriers on each
// directory entry) shows ages and quiet marks that never depend on the
// real date. Every shared-store commit carries one deterministic date, so
// ages there say nothing; mutating the shared store would disturb every
// other suite. This is therefore a SEPARATE, hermetic, REAL store —
// emptyglance.go's and unprovenboard.go's pattern: git init on main, a
// dated main history, a bare local origin whose HEAD names main (a
// provable default branch), and dated design branches — started lazily on
// the control server's GET /index-dates-fixture (control.go), reused
// thereafter, and stopped (its store removed) with the harness. Test-only.
//
// Under indexDatesNow the served index says:
//
//	spec/dated-landed          active component   2024-04-26  landed 50 days back; main's tip is 20 days back
//	spec/dated-edited          active component   2024-05-21  landed 45 days back, edited in place 25 days back
//	spec/dated-desk-component  on the desk        2024-05-16  a status: draft component, quiet (30 days)
//	spec/dated-draft-13        on the desk        2024-06-02  not quiet
//	spec/dated-draft-14        on the desk        2024-06-01  not quiet (the boundary is exclusive)
//	spec/dated-draft-15        on the desk        2024-05-31  quiet
//
// The index story's Playwright files mirror these names and dates
// (e2e/tests/fixtures.ts, lane F7); change them together.

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// verdiNowEnvVar is `verdi serve`'s clock seam (cmd/verdi's fixedClockEnv,
// SI-296): an RFC 3339 instant the served workbench decides every age and
// quiet mark against, disclosed on its pages.
const verdiNowEnvVar = "VERDI_NOW"

// indexDatesNow is the dated store's fixed clock — its serve's VERDI_NOW.
var indexDatesNow = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

// indexDatesCommit is one dated commit: files (repo-relative) written and
// committed ageDays before indexDatesNow.
type indexDatesCommit struct {
	ageDays int
	files   map[string]string
	message string
}

// indexDatesSpecPath is a spec's active-zone path.
func indexDatesSpecPath(name string) string {
	return ".verdi/specs/active/" + name + "/spec.md"
}

// indexDatesComponent is a minimal valid component spec; body follows the
// heading.
func indexDatesComponent(name, status, body string) string {
	return fmt.Sprintf("---\nid: spec/%s\nkind: spec\nclass: component\ntitle: \"%s (dated e2e fixture)\"\nstatus: %s\nowners: [platform-team]\n---\n# %s\n%s", name, name, status, name, body)
}

// indexDatesMainHistory is main's first-parent history, oldest first: every
// default-branch entry lands on its own date, one is then edited in place,
// and an unrelated commit leaves main's tip later than every landing.
var indexDatesMainHistory = []indexDatesCommit{
	{ageDays: 60, message: "dated store: manifest", files: map[string]string{
		".verdi/verdi.yaml": emptyStoreManifest,
		".verdi/.gitignore": "data/\n",
	}},
	{ageDays: 50, message: "dated-landed lands", files: map[string]string{
		indexDatesSpecPath("dated-landed"): indexDatesComponent("dated-landed", "active", ""),
	}},
	{ageDays: 45, message: "dated-edited lands", files: map[string]string{
		indexDatesSpecPath("dated-edited"): indexDatesComponent("dated-edited", "active", ""),
	}},
	{ageDays: 30, message: "dated-desk-component lands as a draft", files: map[string]string{
		indexDatesSpecPath("dated-desk-component"): indexDatesComponent("dated-desk-component", "draft", ""),
	}},
	{ageDays: 25, message: "dated-edited: an in-place edit lands", files: map[string]string{
		indexDatesSpecPath("dated-edited"): indexDatesComponent("dated-edited", "active", "\nAn in-place edit that landed later.\n"),
	}},
	{ageDays: 20, message: "an unrelated later main commit", files: map[string]string{
		"NOTES.md": "An unrelated main commit: main's tip is no entry's landing commit.\n",
	}},
}

// indexDatesDrafts are the design-branch drafts, each cut from main's tip
// and committed once, ageDays before indexDatesNow — either side of the
// fourteen-day quiet boundary.
var indexDatesDrafts = []struct {
	name    string
	ageDays int
}{
	{name: "dated-draft-13", ageDays: 13},
	{name: "dated-draft-14", ageDays: 14},
	{name: "dated-draft-15", ageDays: 15},
}

// indexDatesGitDate renders the instant ageDays before indexDatesNow in
// git's "<unix-seconds> <offset>" form (commitAt's argument).
func indexDatesGitDate(ageDays int) string {
	return fmt.Sprintf("%d +0000", indexDatesNow.Add(-time.Duration(ageDays)*24*time.Hour).Unix())
}

// provisionIndexDatesStore builds the dated store under parent and returns
// its root (parent/store; its bare origin is parent/origin.git): main's
// dated history, the origin whose HEAD names main, then each dated design
// branch — main left checked out, the serving checkout's usual posture.
func provisionIndexDatesStore(ctx context.Context, parent string) (string, error) {
	root := filepath.Join(parent, "store")
	originDir := filepath.Join(parent, "origin.git")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		return "", err
	}
	for _, c := range indexDatesMainHistory {
		if err := commitIndexDatesFiles(ctx, root, c); err != nil {
			return "", err
		}
	}

	// A bare local origin whose HEAD names main (emptyglance.go's
	// load-bearing reasoning): the default branch resolves, so each draft's
	// merged check and every landing commit are proven against it.
	if err := runGit(ctx, "", nil, "init", "--bare", "--quiet", "--initial-branch=main", originDir); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "remote", "add", "origin", originDir); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "push", "--quiet", "--set-upstream", "origin", "main"); err != nil {
		return "", err
	}
	if err := runGit(ctx, root, nil, "remote", "set-head", "origin", "main"); err != nil {
		return "", err
	}

	for _, d := range indexDatesDrafts {
		if err := runGit(ctx, root, nil, "checkout", "--quiet", "-b", "design/"+d.name, "main"); err != nil {
			return "", err
		}
		draft := indexDatesCommit{ageDays: d.ageDays, message: "design start: " + d.name, files: map[string]string{
			indexDatesSpecPath(d.name): indexDatesComponent(d.name, "draft", ""),
		}}
		if err := commitIndexDatesFiles(ctx, root, draft); err != nil {
			return "", err
		}
	}
	if err := runGit(ctx, root, nil, "checkout", "--quiet", "main"); err != nil {
		return "", err
	}
	return root, nil
}

// commitIndexDatesFiles writes c's files under root and commits them at
// c's date (commitAt, git.go).
func commitIndexDatesFiles(ctx context.Context, root string, c indexDatesCommit) error {
	for rel, content := range c.files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return err
		}
	}
	if err := runGit(ctx, root, nil, "add", "-A"); err != nil {
		return err
	}
	return commitAt(ctx, root, indexDatesGitDate(c.ageDays), "commit", "--quiet", "--no-verify", "-m", c.message)
}

// indexDatesServeEnv is the dated serve's environment: hermeticServeEnv
// (every ambient serve injection seam — VERDI_NOW included — and CI
// identity variable stripped) plus this fixture's own fixed clock, exactly
// once.
func indexDatesServeEnv(ambient []string) []string {
	return append(hermeticServeEnv(ambient), verdiNowEnvVar+"="+indexDatesNow.Format(time.RFC3339))
}

// indexDatesFixture lazily provisions the dated store, builds the binary,
// starts its serve, and remembers the bound URL — unprovenboard.go's
// start-once shape, plus the temporary directory stop() removes.
type indexDatesFixture struct {
	moduleRoot string

	mu     sync.Mutex
	url    string
	tmp    string   // the store's parent directory, removed by stop()
	env    []string // the exact environment handed to exec; inspected by tests
	cancel context.CancelFunc
	done   chan error
}

func newIndexDatesFixture(moduleRoot string) *indexDatesFixture {
	return &indexDatesFixture{moduleRoot: moduleRoot}
}

// handler answers GET with the fixture's base URL as a plain-text body,
// starting the dated serve on the first call.
func (f *indexDatesFixture) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url, err := f.ensureStarted(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(url))
}

// ensureStarted provisions the dated store in a fresh temporary directory,
// builds the binary from moduleRoot exactly as main.go's buildBinary does,
// starts `verdi serve --http <loopback>` over it under indexDatesServeEnv,
// waits for its healthz, and returns the URL on every call thereafter,
// unchanged. A failed start removes what it created.
func (f *indexDatesFixture) ensureStarted(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}

	tmp, err := os.MkdirTemp("", "verdi-e2e-index-dates-*")
	if err != nil {
		return "", err
	}
	url, err := f.start(ctx, tmp)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	f.url, f.tmp = url, tmp
	return url, nil
}

// start is ensureStarted's work inside tmp; on success it records the
// child's env, cancel, and done.
func (f *indexDatesFixture) start(ctx context.Context, tmp string) (string, error) {
	root, err := provisionIndexDatesStore(ctx, tmp)
	if err != nil {
		return "", fmt.Errorf("provisioning the index-dates store: %w", err)
	}
	binPath := filepath.Join(tmp, "verdi")
	if err := buildBinary(ctx, f.moduleRoot, binPath); err != nil {
		return "", fmt.Errorf("building verdi binary for the index-dates fixture: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	childCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(childCtx, binPath, "serve", "--http", addr)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Dir = root
	env := indexDatesServeEnv(os.Environ())
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		cancel()
		return "", fmt.Errorf("starting verdi serve for the index-dates fixture: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	url := "http://" + addr + "/"
	if err := waitHealthy(ctx, url+"healthz", 20*time.Second); err != nil {
		cancel()
		<-done
		return "", fmt.Errorf("waiting for the index-dates fixture's verdi serve: %w", err)
	}
	f.env, f.cancel, f.done = env, cancel, done
	return url, nil
}

// stop terminates the dated serve (SIGTERM, then the WaitDelay force-kill),
// waits for it, and removes the fixture's temporary store — and forgets its
// URL, which no longer serves anything. Safe when never started, and
// idempotent.
func (f *indexDatesFixture) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cancel != nil {
		f.cancel()
		<-f.done
		f.cancel = nil
	}
	if f.tmp != "" {
		_ = os.RemoveAll(f.tmp)
		f.tmp = ""
	}
	f.url = ""
}
