// The directory home's in-review feed (spec/directory-home dc-4): the
// adapters behind workbench.OpenMRLister, the consumer-defined port the
// home page's per-render forge consultation goes through. Three
// implementations, mirroring reviewfeed.go's wiring states exactly:
//
//   - forgeOpenMRs: the real forge adapter (forge.Forge.ListOpenMRs, the
//     one branch-scoped MR-listing mechanism this repo already ships).
//   - httpOpenMRFeed: the hermetic harness double (VERDI_OPENMR_FEED, a
//     loopback URL served by cmd/e2eharness) — strict-decoded JSON, no
//     real forge, no network beyond localhost (co-2).
//   - unavailableOpenMRs: the configured-but-unreachable forge (I-1(b)) —
//     every consultation errors with the disclosed reason, which the home
//     page renders as its "MR status unavailable" notice rather than
//     silently reading "no open MRs".
//
// It lives in cmd/verdi (not internal/workbench) so the workbench never
// imports internal/forge — the same dependency direction reviewfeed.go
// documents.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/workbench"
)

// forgeOpenMRs adapts forge.Forge.ListOpenMRs onto workbench.OpenMRLister:
// the source branch of every open MR targeting the store's resolved
// default branch, consulted fresh per call (dc-4: per-render).
type forgeOpenMRs struct {
	f    forge.Forge
	root string
}

// newForgeOpenMRs wraps f for the checkout rooted at root.
func newForgeOpenMRs(f forge.Forge, root string) *forgeOpenMRs {
	return &forgeOpenMRs{f: f, root: root}
}

func (a *forgeOpenMRs) OpenMRSourceBranches(ctx context.Context) ([]string, error) {
	defaultBranch := lint.ResolveDefaultBranch(ctx, a.root)
	if defaultBranch == "" {
		return nil, errors.New("verdi: cannot resolve the default branch to list open MRs against (no origin/HEAD configured)")
	}
	mrs, err := a.f.ListOpenMRs(ctx, defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("verdi: listing open MRs targeting %s: %w", defaultBranch, err)
	}
	branches := make([]string, 0, len(mrs))
	for _, mr := range mrs {
		branches = append(branches, mr.SourceBranch)
	}
	sort.Strings(branches)
	return branches, nil
}

var _ workbench.OpenMRLister = (*forgeOpenMRs)(nil)

// openMRFeedEntry is one open MR in the canned harness feed's JSON shape —
// the fields forge.OpenMR carries, snake-cased.
type openMRFeedEntry struct {
	ID           string `json:"id"`
	SourceBranch string `json:"source_branch"`
	Title        string `json:"title"`
}

// httpOpenMRFeed is the hermetic harness double: it GETs url per
// consultation and strict-decodes a JSON array of openMRFeedEntry
// (DisallowUnknownFields + trailing-data rejection, CLAUDE.md). Any
// failure — connection refused, non-200, malformed body — surfaces as an
// error the home page degrades to its disclosed notice, which is exactly
// what the e2e suite's outage toggle exercises.
type httpOpenMRFeed struct {
	url string
}

func (h httpOpenMRFeed) OpenMRSourceBranches(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.url, nil)
	if err != nil {
		return nil, fmt.Errorf("verdi: building open-MR feed request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("verdi: fetching open-MR feed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // read-only response body; close error is unactionable
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("verdi: open-MR feed %s returned status %d", h.url, resp.StatusCode)
	}

	dec := json.NewDecoder(resp.Body)
	dec.DisallowUnknownFields()
	var entries []openMRFeedEntry
	if err := dec.Decode(&entries); err != nil {
		return nil, fmt.Errorf("verdi: decoding open-MR feed: %w", err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("verdi: open-MR feed carries trailing data after the entry array")
	}

	branches := make([]string, 0, len(entries))
	for _, e := range entries {
		branches = append(branches, e.SourceBranch)
	}
	sort.Strings(branches)
	return branches, nil
}

var _ workbench.OpenMRLister = httpOpenMRFeed{}

// unavailableOpenMRs is the configured-but-unreachable state (I-1(b)): a
// forge is named in verdi.yaml but no live adapter could be built, so
// every consultation errors with that disclosed reason — the home page
// renders its "MR status unavailable" notice instead of silently reading
// the missing input as "nothing in review".
type unavailableOpenMRs struct {
	reason string
}

func (u unavailableOpenMRs) OpenMRSourceBranches(context.Context) ([]string, error) {
	return nil, errors.New(u.reason)
}

var _ workbench.OpenMRLister = unavailableOpenMRs{}
