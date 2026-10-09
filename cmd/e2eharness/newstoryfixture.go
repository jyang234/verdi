package main

// newStoryFixture (spec/new-story-dialog-v2; ledger SI-369 (10), (11)): the
// one harness store whose sealed feature carries all three coverage states
// the New story dialog lists — a criterion covered by a declared stub, one
// covered only by an accepted story implementing it, and one covered by
// nothing — under the plain vocabulary preset. No shared-store feature has
// that mix (the F6 facts survey's R10), and adding one there would join
// other suites' columns and calls to action, so this is a SEPARATE,
// hermetic, REAL store — indexfailure.go's in-process pattern (ADJ-40):
// the production workbench handler over a store with main checked out, a
// bare local origin whose HEAD names main (so the default branch resolves,
// the feature's wall is sealed, and Create's branch cut is local, with
// nothing pushed anywhere but that origin), started on the control
// server's GET /newstory-fixture, reused thereafter, and stopped (its
// store removed) with the harness. Test-only.
//
// Two read-only control routes witness the store from outside the browser
// (SI-369 (11): nothing written until Create):
//
//   - GET /newstory-fixture/refs answers one canonical JSON object,
//     {"porcelain": [...], "refs": [...]}: refs is `git for-each-ref
//     --format='%(objectname) %(refname)'` and porcelain is `git status
//     --porcelain --untracked-files=all`, one array element per output
//     line, exactly as git prints it (a leading status column kept), and
//     an empty array, never null, when there is none.
//   - GET /newstory-fixture/show?ref=<ref>&path=<path> answers the blob at
//     <ref>:<path>, byte-exact, or 404 when the ref, or the file at that
//     ref, does not exist (a directory is not a file). A ref is a plain
//     ref or branch name; a path is store-relative and traversal-free;
//     anything else is 400.
//
// Both answer 409 until the fixture is started, never starting it, and
// write nothing: status runs with --no-optional-locks, so it never
// refreshes the index file either.
//
// The New story dialog's Playwright file (e2e/tests/95-new-story-
// dialog.spec.ts) keeps its own copies of these names; change them
// together.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/initwizard"
	"github.com/jyang234/verdi/internal/workbench"
)

// The fixture's names and paths. ac-1 is covered by the stub only, ac-2
// by the story only, ac-3 by nothing.
const (
	newStoryFeatureName = "escrow-analysis"
	newStoryStoryName   = "escrow-analysis-notice"
	newStoryStubSlug    = "escrow-analysis-run"
	newStoryFeaturePath = ".verdi/specs/active/" + newStoryFeatureName + "/spec.md"
	newStoryStoryPath   = ".verdi/specs/active/" + newStoryStoryName + "/spec.md"
)

// newStoryFeatureSpec is the sealed feature: accepted-pending-build on
// main, three criteria, and one declared stub listing ac-1 alone.
const newStoryFeatureSpec = `---
id: spec/escrow-analysis
kind: spec
title: "Escrow analysis notices"
owners: [platform-team]
class: feature
status: accepted-pending-build
story: jira:ESC-40
problem: { text: "a borrower learns the result of the annual escrow analysis only from a changed monthly payment", anchor: problem }
outcome: { text: "every escrow account is analyzed once a year, the borrower hears the result promptly, and a surplus comes back without a phone call", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "every active escrow account is analyzed once a year", evidence: [behavioral], anchor: ac-1 }
  - { id: ac-2, text: "the borrower is notified of a shortage or surplus within one business day of the analysis", evidence: [behavioral], anchor: ac-2 }
  - { id: ac-3, text: "a surplus over the refund threshold is refunded to the borrower automatically", evidence: [behavioral], anchor: ac-3 }
stubs:
  - { slug: escrow-analysis-run, acceptance_criteria: [ac-1] }
frozen: { at: 2026-01-01, commit: 0000000000000000000000000000000000000f06 }
---
# Escrow analysis notices

## Problem

A borrower learns the result of the annual escrow analysis only from a changed monthly payment.

## Outcome

Every escrow account is analyzed once a year, the borrower hears the result promptly, and a surplus comes back without a
phone call.

## ac-1

Every active escrow account is analyzed once a year.

## ac-2

The borrower is notified of a shortage or surplus within one business day of the analysis.

## ac-3

A surplus over the refund threshold is refunded to the borrower automatically.
`

// newStoryStorySpec is the accepted story implementing ac-2 — the
// criterion's only coverage.
const newStoryStorySpec = `---
id: spec/escrow-analysis-notice
kind: spec
title: "Escrow analysis notice"
owners: [platform-team]
class: story
status: accepted-pending-build
story: jira:ESC-41
problem: { text: "a shortage or surplus reaches the borrower only through a changed payment", anchor: problem }
outcome: { text: "the borrower is told the analysis result within one business day", anchor: outcome }
acceptance_criteria:
  - { id: ac-1, text: "a notice naming the shortage or surplus goes out within one business day of the analysis", evidence: [behavioral], anchor: ac-1 }
links:
  - { type: implements, ref: "spec/escrow-analysis#ac-2" }
frozen: { at: 2026-01-01, commit: 0000000000000000000000000000000000000f06 }
---
# Escrow analysis notice

## Problem

A shortage or surplus reaches the borrower only through a changed payment.

## Outcome

The borrower is told the analysis result within one business day.

## ac-1

A notice naming the shortage or surplus goes out within one business day of the analysis.
`

// provisionNewStoryStore builds the store under parent and returns its
// root (parent/store; its bare origin is parent/origin.git): one commit on
// main holding the manifest, the plain preset's model.yaml, the feature,
// and the story, pushed to the origin whose HEAD names main — main left
// checked out, the serving checkout's usual posture. Both repositories are
// made by initRepo (BL-148).
func provisionNewStoryStore(ctx context.Context, parent string) (string, error) {
	root := filepath.Join(parent, "store")
	originDir := filepath.Join(parent, "origin.git")
	if err := initRepo(ctx, root, false); err != nil {
		return "", err
	}
	// A repo-local identity (provision_vocab.go's reason): Create's
	// plumbing commit runs without deterministicGitEnv, so a host with no
	// usable identity of its own would otherwise refuse it.
	for _, kv := range [][2]string{{"user.name", "verdi-e2e"}, {"user.email", "e2e@verdi.invalid"}} {
		if err := runGit(ctx, root, nil, "config", kv[0], kv[1]); err != nil {
			return "", err
		}
	}
	files := map[string]string{
		".verdi/verdi.yaml": emptyStoreManifest,
		".verdi/.gitignore": "data/\n",
		".verdi/model.yaml": string(initwizard.RenderModelYAML(initwizard.PlainPreset())),
		newStoryFeaturePath: newStoryFeatureSpec,
		newStoryStoryPath:   newStoryStorySpec,
	}
	for rel, content := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			return "", err
		}
	}
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "--quiet", "--no-verify", "-m", "new-story store: a sealed feature, its accepted story, the plain preset"},
	} {
		if err := runGit(ctx, root, nil, args...); err != nil {
			return "", err
		}
	}
	if err := initRepo(ctx, originDir, true); err != nil {
		return "", err
	}
	for _, args := range [][]string{
		{"remote", "add", "origin", originDir},
		{"push", "--quiet", "--set-upstream", "origin", "main"},
		{"remote", "set-head", "origin", "main"},
	} {
		if err := runGit(ctx, root, nil, args...); err != nil {
			return "", err
		}
	}
	return root, nil
}

// newStoryFixture lazily provisions the store and serves it in-process —
// indexfailure.go's start-once shape, plus the store root its read-only
// routes inspect.
type newStoryFixture struct {
	mu   sync.Mutex
	url  string
	root string // the store; "" until started, and again after stop()
	tmp  string // the store's parent directory, removed by stop()
	srv  *http.Server
}

func newNewStoryFixture() *newStoryFixture { return &newStoryFixture{} }

// handler answers GET with the fixture's base URL as a plain-text body,
// starting the isolated server on the first call.
func (f *newStoryFixture) handler(w http.ResponseWriter, r *http.Request) {
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

// ensureStarted provisions the store in a fresh temporary directory and
// serves it through the production workbench.NewHandler, returning the URL
// on every call thereafter, unchanged. A failed start removes what it
// created.
func (f *newStoryFixture) ensureStarted(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}
	tmp, err := os.MkdirTemp("", "verdi-e2e-new-story-*")
	if err != nil {
		return "", err
	}
	root, err := provisionNewStoryStore(ctx, tmp)
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", fmt.Errorf("provisioning the new-story store: %w", err)
	}
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		_ = os.RemoveAll(tmp)
		return "", err
	}
	srv := &http.Server{Handler: workbench.NewHandler(root)}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "e2eharness: new-story fixture server: %v\n", err)
		}
	}()
	f.url, f.root, f.tmp, f.srv = "http://"+ln.Addr().String()+"/", root, tmp, srv
	return f.url, nil
}

// startedRoot is the store root, or "" when the fixture is not started.
func (f *newStoryFixture) startedRoot() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.root
}

// newStoryRefs is GET /newstory-fixture/refs's answer; its fields are in
// key order, so the encoding is canonical.
type newStoryRefs struct {
	Porcelain []string `json:"porcelain"`
	Refs      []string `json:"refs"`
}

// refsHandler answers GET /newstory-fixture/refs: the store's refs and
// porcelain, read without writing anything.
func (f *newStoryFixture) refsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	root := f.startedRoot()
	if root == "" {
		http.Error(w, "new-story fixture not started yet (GET /newstory-fixture first)", http.StatusConflict)
		return
	}
	refs, err := gitRawOutput(r.Context(), root, "for-each-ref", "--format=%(objectname) %(refname)")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	porcelain, err := gitRawOutput(r.Context(), root, "--no-optional-locks", "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(newStoryRefs{Porcelain: outputLines(porcelain), Refs: outputLines(refs)}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(body.Bytes())
}

// showRefRe is the ref grammar /show accepts: a plain ref or branch name,
// never an option, a revision expression, or a <ref>:<path> pair.
var showRefRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]*$`)

// showHandler answers GET /newstory-fixture/show?ref=&path=: the blob at
// ref:path, byte-exact, or 404.
func (f *newStoryFixture) showHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ref := r.URL.Query().Get("ref")
	rel := r.URL.Query().Get("path")
	if !showRefRe.MatchString(ref) || strings.Contains(ref, "..") {
		http.Error(w, "ref must be a plain ref or branch name", http.StatusBadRequest)
		return
	}
	if rel == "" || filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") || containsDotDot(rel) || strings.ContainsAny(rel, "\x00\r\n") {
		http.Error(w, "path must be a store-relative, traversal-free path", http.StatusBadRequest)
		return
	}
	root := f.startedRoot()
	if root == "" {
		http.Error(w, "new-story fixture not started yet (GET /newstory-fixture first)", http.StatusConflict)
		return
	}
	blob, err := gitRawOutput(r.Context(), root, "cat-file", "blob", ref+":"+rel)
	if err != nil {
		if r.Context().Err() != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Error(w, "no file "+rel+" at "+ref, http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(blob)
}

// outputLines splits a command's output into its lines, each exactly as
// printed, with no element for the final newline; no output is an empty,
// non-nil list.
func outputLines(raw []byte) []string {
	s := strings.TrimSuffix(string(raw), "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

// stop closes the isolated server and removes the fixture's temporary
// store — and forgets its URL and root, so the read-only routes refuse
// again. Safe when never started, and idempotent.
func (f *newStoryFixture) stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.srv != nil {
		_ = f.srv.Close()
		f.srv = nil
	}
	if f.tmp != "" {
		_ = os.RemoveAll(f.tmp)
		f.tmp = ""
	}
	f.url, f.root = "", ""
}
