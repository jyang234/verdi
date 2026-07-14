package main

// The e2e inspection server (spec/draft-boards co-2/ac-2): a loopback-only,
// READ-ONLY window into the scratch store's git and file state, so the
// Playwright suite can witness what the browser cannot see — that an
// authoring edit under one /b/ address landed in its own branch's managed
// worktree only, and that the serving checkout's working tree was not
// disturbed by the whole exchange. Nothing here mutates anything; nothing
// leaves 127.0.0.1.
//
//   - GET /porcelain   {"branch": <serving checkout's current branch>,
//     "porcelain": <git status --porcelain over the serve root>}
//   - GET /file?path=  the store-relative file's bytes (404 when absent).
//     Paths are store-relative and traversal-free; the suite reads managed
//     worktree files under .verdi/data/worktrees/ through this.

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// inspectAddr is the inspection server's fixed loopback address, bound by
// e2e/tests/fixtures.ts (INSPECT_URL).
const inspectAddr = "127.0.0.1:4178"

// inspectHandler wires the two read-only endpoints onto a fresh mux.
func inspectHandler(storeRoot string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/porcelain", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		branch, err := gitOutput(storeRoot, "rev-parse", "--abbrev-ref", "HEAD")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		porcelain, err := gitOutput(storeRoot, "status", "--porcelain")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"branch":    strings.TrimSpace(branch),
			"porcelain": porcelain,
		})
	})
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		rel := r.URL.Query().Get("path")
		if rel == "" || filepath.IsAbs(rel) || containsDotDot(rel) {
			http.Error(w, "path must be a store-relative, traversal-free path", http.StatusBadRequest)
			return
		}
		data, err := os.ReadFile(filepath.Join(storeRoot, filepath.FromSlash(rel)))
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "no such file", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
	})
	return mux
}

// gitOutput runs one read-only git query in dir and returns its stdout.
func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// containsDotDot reports whether any slash-separated segment of rel is
// "..".
func containsDotDot(rel string) bool {
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}
