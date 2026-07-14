package main

// The e2e control server (spec/directory-home co-2): a loopback-only
// helper the Playwright suite drives to exercise the directory home's two
// mid-session shapes — nothing here ever leaves 127.0.0.1.
//
//   - GET  /openmrs        the hermetic open-MR feed `verdi serve` consults
//     per render (VERDI_OPENMR_FEED): one open MR whose source branch is
//     design/refi-decline-flow, so exactly that directory entry chips
//     "in review" (ac-2). Strict JSON, the httpOpenMRFeed shape.
//   - POST /outage         flips the feed to 503 for the rest of the run —
//     the "forge unreachable" degradation (ac-2's disclosed absence).
//   - POST /delete-branch  ?branch=design/<name> deletes that local design
//     branch from the scratch store — the deleted-mid-session shape whose
//     stale directory link must resolve to the disclosed 404 (ac-3).
//     Design-namespace branches only; anything else is refused.

import (
	"log"
	"net/http"
	"strings"
	"sync"
)

// controlAddr is the control server's fixed loopback address, bound by
// e2e/tests/fixtures.ts (CONTROL_URL).
const controlAddr = "127.0.0.1:4177"

// openMRFeedJSON is the canned happy-path feed: the board suite's design
// branch carries the one open MR.
const openMRFeedJSON = `[{"id":"mr-9","source_branch":"design/refi-decline-flow","title":"Refinancing decline flow"}]` + "\n"

// controlServer holds the toggleable feed state and the store the
// delete-branch endpoint mutates.
type controlServer struct {
	storeRoot string

	mu     sync.Mutex
	outage bool
}

func newControlServer(storeRoot string) *controlServer {
	return &controlServer{storeRoot: storeRoot}
}

// handler wires the three endpoints onto a fresh mux.
func (c *controlServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/openmrs", c.openMRs)
	mux.HandleFunc("/outage", c.triggerOutage)
	mux.HandleFunc("/delete-branch", c.deleteBranch)
	return mux
}

func (c *controlServer) openMRs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.mu.Lock()
	down := c.outage
	c.mu.Unlock()
	if down {
		http.Error(w, "simulated forge outage", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(openMRFeedJSON))
}

func (c *controlServer) triggerOutage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.mu.Lock()
	c.outage = true
	c.mu.Unlock()
	log.Println("e2eharness: control — open-MR feed outage enabled")
	w.WriteHeader(http.StatusNoContent)
}

func (c *controlServer) deleteBranch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	branch := r.URL.Query().Get("branch")
	if !strings.HasPrefix(branch, "design/") {
		http.Error(w, "only design/* branches may be deleted", http.StatusBadRequest)
		return
	}
	if err := runGit(c.storeRoot, nil, "branch", "-D", branch); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("e2eharness: control — deleted branch %s", branch)
	w.WriteHeader(http.StatusNoContent)
}
