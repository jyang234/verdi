package main

// readinessAllProvenFixture (Task 5 F-01 correction review, Finding 2):
// the main harness store deliberately provisions ONE mixed startup
// snapshot — the pilot's real posture — so the all-proven browser
// posture ("All four steps are complete.", empty focus, no current
// step) can never be exercised against it. Rather than add a runtime
// fixture mode to `verdi serve` or mutate the shared store, this spawns
// a SEPARATE, hermetic, in-process workbench instance following the
// established isolated-fixture pattern (emptyglance.go): the REAL
// workbench.NewHandlerWith wiring with a strictly valid, immutable,
// fully proven readinesspilot.Snapshot injected through the same
// Deps.Readiness seam production serve uses. Loopback only, started
// lazily on the control server's GET /readiness-all-proven-fixture and
// reused thereafter. Test-only; production serve behavior is untouched.

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/workbench"
)

// readinessAllProvenHead is the fixture's pinned startup HEAD — a fixed
// literal, so the browser oracle can assert the exact stale-notice text.
const readinessAllProvenHead = "e2eallproven0001"

// readinessAllProvenSnapshot builds the strictly valid, fully proven
// snapshot: one proven concern per area (the contract forbids a vacuous
// area), no attention, no current focus.
func readinessAllProvenSnapshot() readinesspilot.Snapshot {
	proven := func(id string, area readinesspilot.AreaID, blocking bool, summary string, witnesses []string) readinesspilot.Concern {
		return readinesspilot.Concern{
			ID: id, Area: area, State: readinesspilot.StateProven,
			Blocking: blocking, Timing: readinesspilot.TimingCurrent,
			Summary: summary, Witnesses: witnesses,
			Destination: readinesspilot.Destination{CLI: []string{}},
		}
	}
	return readinesspilot.Snapshot{
		TargetRef:     "spec/all-proven-fixture",
		TargetTitle:   "Fully proven pilot flow",
		TargetClass:   "story",
		Branch:        "design/all-proven-fixture",
		Head:          readinessAllProvenHead,
		RequestDigest: "sha256:" + strings.Repeat("cd", 32),
		Areas: []readinesspilot.Area{
			{ID: readinesspilot.AreaShape, Label: "Define the work", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaSuccess, Label: "Define success", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaContext, Label: "Check constraints", State: readinesspilot.StateProven},
			{ID: readinesspilot.AreaReview, Label: "Get approval", State: readinesspilot.StateProven},
		},
		CurrentFocus: "",
		Attention:    []readinesspilot.Concern{},
		AllConcerns: []readinesspilot.Concern{
			proven("shape/problem", readinesspilot.AreaShape, true,
				"Problem statement is present", []string{}),
			proven("success/contributor/static", readinesspilot.AreaSuccess, false,
				"Journey evidence contributor static", []string{"static evidence recorded"}),
			proven("context/verdict", readinesspilot.AreaContext, true,
				"Policy-conflict verdict", []string{}),
			proven("review/action", readinesspilot.AreaReview, true,
				"Lifecycle and safe-action posture can advance review", []string{"journey-advance"}),
		},
		StaleNotice: "Startup snapshot at " + readinessAllProvenHead + "; restart verdi serve after an edit.",
	}
}

// readinessAllProvenFixture lazily starts its isolated server and
// remembers its bound URL — the same start-once cache shape as
// emptyGlanceFixture.
type readinessAllProvenFixture struct {
	mu  sync.Mutex
	url string
}

func newReadinessAllProvenFixture() *readinessAllProvenFixture {
	return &readinessAllProvenFixture{}
}

// handler answers GET with the fixture's URL as a plain-text body,
// starting the isolated server on the first call.
func (f *readinessAllProvenFixture) handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	url, err := f.ensureStarted()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(url))
}

// ensureStarted validates the snapshot, binds a loopback listener, and
// serves the real workbench handler with the snapshot injected through
// Deps.Readiness — exactly the production seam.
func (f *readinessAllProvenFixture) ensureStarted() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.url != "" {
		return f.url, nil
	}

	snap := readinessAllProvenSnapshot()
	if err := snap.Validate(); err != nil {
		return "", fmt.Errorf("all-proven fixture snapshot is invalid: %w", err)
	}
	// An empty scratch root: this fixture serves only /readiness and its
	// assets; the root exists so the real handler wiring has a store path.
	root, err := os.MkdirTemp("", "verdi-e2e-readiness-allproven-*")
	if err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	srv := &http.Server{Handler: workbench.NewHandlerWith(root, workbench.Deps{Readiness: &snap})}
	go func() { _ = srv.Serve(ln) }()

	f.url = "http://" + ln.Addr().String() + "/"
	return f.url, nil
}
