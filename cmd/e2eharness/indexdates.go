package main

// The index-dates control endpoints (spec/index-data ac-3): a harness-side,
// in-process window onto the SAME on-disk scratch store `verdi serve`
// serves, computed through refindex's own production seam
// (refindex.ComputeIndex over NewGitRunner/NewStateResolver) — never the
// served subprocess's own HTTP responses, which carry no visible age or
// quiet markup at all yet (rendering the mark is a LATER story's job; this
// lane's own hard boundary forbids adding any visible UI). A later
// Playwright spec asserts an entry's age and quiet mark deterministically
// by calling GET /index-dates under a clock it set through POST /clock,
// instead of scraping rendered HTML — the "harness control/inspect JSON
// endpoint" this lane's own preflight named as the sanctioned alternative.
//
//   - POST /clock        body {"now": "<RFC3339>"}, or an empty body to
//     clear the override back to the real wall clock. Sets the harness
//     clock every following GET /index-dates call reads.
//   - GET  /clock         {"now": "<RFC3339>"} — the clock's current answer.
//   - GET  /index-dates   the current ComputeIndex() result over storeRoot,
//     each entry's date/disclosure/quiet decided against the clock above.

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/refindex"
)

// harnessClock is the settable "now" GET /index-dates reads — the
// harness's own instance of the exact seam internal/workbench's
// HomeDeps.Clock carries in-process (nil there means the wall clock read
// at render time; here, an unset override means the same thing) — so a
// Playwright spec can fix "now" without reaching into the SEPARATE `verdi
// serve` subprocess at all.
type harnessClock struct {
	mu  sync.Mutex
	now *time.Time
}

// Now returns the overridden instant if one is set, otherwise the real
// wall clock — never a value frozen at harnessClock construction time.
func (c *harnessClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.now != nil {
		return *c.now
	}
	return time.Now()
}

// Set fixes the clock at t until the next Set or Clear.
func (c *harnessClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = &t
}

// Clear removes any override, returning Now to the real wall clock.
func (c *harnessClock) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = nil
}

// indexDateEntry is one GET /index-dates row: a minimal, JSON-stable
// projection of refindex.Entry carrying only what a date/quiet assertion
// needs, never the whole internal struct.
type indexDateEntry struct {
	Ref           string `json:"ref"`
	StatusGroup   string `json:"statusGroup"`
	Date          string `json:"date,omitempty"`
	DateDisclosed string `json:"dateDisclosed,omitempty"`
	Quiet         bool   `json:"quiet"`
}

// computeIndexDates runs the real production ComputeIndex over storeRoot
// and decorates each entry with its quiet verdict against now — the exact
// seam a real `verdi serve` render uses (refindex.NewGitRunner,
// refindex.NewStateResolver), so what this endpoint reports is what the
// served store actually contains, never a second, hand-rolled computation.
func computeIndexDates(ctx context.Context, storeRoot string, now time.Time) ([]indexDateEntry, error) {
	entries, err := refindex.ComputeIndex(ctx, storeRoot, refindex.NewGitRunner(), refindex.NewStateResolver())
	if err != nil {
		return nil, err
	}
	out := make([]indexDateEntry, 0, len(entries))
	for _, e := range entries {
		item := indexDateEntry{
			Ref:         e.Ref,
			StatusGroup: string(e.StatusGroup),
			Date:        e.Date,
			Quiet:       refindex.IsQuiet(e, now),
		}
		if e.DateDisclosed != nil {
			item.DateDisclosed = disclosure.Render(*e.DateDisclosed)
		}
		out = append(out, item)
	}
	return out, nil
}

// clockRequestBody is POST /clock's JSON body shape.
type clockRequestBody struct {
	Now string `json:"now"`
}

// clockHandler answers GET/POST /clock against c's own harnessClock.
func (c *controlServer) clockHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"now": c.clock.Now().UTC().Format(time.RFC3339)})
	case http.MethodPost:
		var body clockRequestBody
		if r.ContentLength != 0 {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
				return
			}
		}
		if body.Now == "" {
			c.clock.Clear()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		t, err := time.Parse(time.RFC3339, body.Now)
		if err != nil {
			http.Error(w, "now must be RFC3339: "+err.Error(), http.StatusBadRequest)
			return
		}
		c.clock.Set(t)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// indexDatesHandler answers GET /index-dates: computeIndexDates over c's
// own storeRoot and clock.
func (c *controlServer) indexDatesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	entries, err := computeIndexDates(r.Context(), c.storeRoot, c.clock.Now())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}
