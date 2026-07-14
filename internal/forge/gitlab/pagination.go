package gitlab

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// restPageSize is the per_page value every drained REST list request below
// carries (spec/forge-transport ac-2/dc-3: "per_page=100... walks pages to
// exhaustion").
const restPageSize = 100

// nextPageHeader is GitLab's own page-cursor signal (GitLab API docs,
// "Pagination": responses to a paginated GET carry X-Next-Page, empty on
// the last page) — the ONE pagination mechanism this adapter reads (dc-3
// picks page params over Link-style headers for GitLab; this picks
// X-Next-Page specifically, over hand-incrementing `page=N` and hoping,
// since GitLab tells the walker directly whether another page exists,
// consistent with GitHub's walker trusting its own server-told signal
// rather than re-deriving one).
const nextPageHeader = "X-Next-Page"

// withPageQuery returns rawURL with per_page=restPageSize and page=page set
// (added or overwritten).
func withPageQuery(rawURL string, page int) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// See github's withPerPage: rawURL is always built from trusted
		// config plus already-escaped segments, so this branch would only
		// ever be hit by a request that was already going to fail
		// downstream. Fall back rather than panic.
		return rawURL
	}
	q := u.Query()
	q.Set("per_page", strconv.Itoa(restPageSize))
	q.Set("page", strconv.Itoa(page))
	u.RawQuery = q.Encode()
	return u.String()
}

// gitlabDrainList walks a GitLab REST list endpoint to exhaustion (dc-3):
// per_page=100 plus page=N, following the X-Next-Page response header
// until it is empty. Every endpoint this adapter pages over
// (findPipeline, findJob's listing, ListOpenMRs, listDiscussions) decodes
// its page as a bare JSON array, so one type parameter (the item type)
// covers all of them — unlike GitHub, GitLab has no wrapping-object page
// shape here.
//
// A fake/response that never sets X-Next-Page at all (every existing
// contract-suite fixture, and a true last real page) stops after one
// request — co-1/ac-1's equivalence proof holds unchanged. A next-page
// number identical to the page just fetched fails loud instead of looping
// forever (mirrors github's same-URL guard): a broken/malicious server
// signature, not a legitimate pagination shape.
func gitlabDrainList[I any](ctx context.Context, a *Adapter, firstURL string) ([]I, error) {
	var all []I
	page := 1
	for {
		u := withPageQuery(firstURL, page)
		var items []I
		headers, err := a.transport.DoHeaders(ctx, http.MethodGet, u, nil, a.setAuth, a.classify(http.MethodGet, u, http.StatusOK), &items)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)

		next := headers.Get(nextPageHeader)
		if next == "" {
			break
		}
		nextPage, err := strconv.Atoi(next)
		if err != nil {
			// A malformed X-Next-Page must stop the walk cleanly, never
			// spin or panic — mirrors github's malformed-Link-header
			// handling (co-1's negative witness).
			break
		}
		if nextPage == page {
			return nil, fmt.Errorf("gitlab: pagination loop detected: X-Next-Page repeats page %d", page)
		}
		page = nextPage
	}
	return all, nil
}
