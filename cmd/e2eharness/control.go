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
//   - GET  /empty-glance-fixture returns the URL of a separate, hermetic,
//     in-process workbench instance backed by a REAL minimal store (git
//     init + .verdi/verdi.yaml, zero specs) whose three glance buckets are
//     all empty through the real refindex.ComputeIndex pipeline
//     (spec/home-status-glance ac-3/co-1; ADJ-40) — see emptyglance.go's
//     own doc comment for why this is isolated rather than mutating the
//     shared store above.

import (
	"log"
	"net/http"
	"strings"
	"sync"
)

// The control server's loopback address is resolved in main.go's run()
// (ports.go's resolvePorts, D6-28) and passed to main.go's own use, not
// held here — e2e/tests/fixtures.ts's CONTROL_URL derives the matching
// value via e2e/ports.ts's mirror of the same derivation.

// openMRFeedJSON is the canned happy-path feed: the board suite's design
// branch carries the one open MR.
const openMRFeedJSON = `[{"id":"mr-9","source_branch":"design/refi-decline-flow","title":"Refinancing decline flow"}]` + "\n"

// controlServer holds the toggleable feed state, the store the
// delete-branch endpoint mutates, and the lazily-started empty-glance
// fixture (emptyglance.go).
type controlServer struct {
	storeRoot string

	mu     sync.Mutex
	outage bool

	emptyGlance *emptyGlanceFixture
}

func newControlServer(storeRoot string) *controlServer {
	return &controlServer{storeRoot: storeRoot, emptyGlance: newEmptyGlanceFixture()}
}

// handler wires the four endpoints onto a fresh mux.
func (c *controlServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/openmrs", c.openMRs)
	mux.HandleFunc("/outage", c.triggerOutage)
	mux.HandleFunc("/delete-branch", c.deleteBranch)
	mux.HandleFunc("/empty-glance-fixture", c.emptyGlance.handler)
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
