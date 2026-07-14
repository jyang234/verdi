package github

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// restPageSize is the per_page value every drained REST list request below
// carries (spec/forge-transport ac-2/dc-3: "per_page=100... walks pages to
// exhaustion").
const restPageSize = 100

// withPerPage returns rawURL with per_page=restPageSize set (added or
// overwritten — a caller never hand-writes per_page itself, so this always
// adds it fresh).
func withPerPage(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		// Callers build rawURL from fmt.Sprintf over trusted config plus
		// url.QueryEscape'd path segments; an unparseable result here would
		// already have failed the request build downstream. Fall back to
		// the unmodified URL rather than panicking — the request that
		// follows will surface the real error.
		return rawURL
	}
	q := u.Query()
	q.Set("per_page", strconv.Itoa(restPageSize))
	u.RawQuery = q.Encode()
	return u.String()
}

// parseLinkNext extracts the rel="next" URL from a GitHub Link response
// header (RFC 8288 shape: `<url>; rel="next", <url>; rel="last"`). A
// missing header, a header with no rel="next" member, or a malformed one
// (unparseable segment) all resolve to "" — the walk's stop signal — rather
// than an error: a malformed/absent Link header must stop the walk cleanly,
// never spin (spec/forge-transport co-1's negative witness).
func parseLinkNext(header string) string {
	if header == "" {
		return ""
	}
	for _, segment := range strings.Split(header, ",") {
		parts := strings.Split(segment, ";")
		if len(parts) < 2 {
			continue
		}
		urlPart := strings.TrimSpace(parts[0])
		if !strings.HasPrefix(urlPart, "<") || !strings.HasSuffix(urlPart, ">") {
			continue
		}
		isNext := false
		for _, param := range parts[1:] {
			param = strings.TrimSpace(param)
			if param == `rel="next"` {
				isNext = true
				break
			}
		}
		if isNext {
			return urlPart[1 : len(urlPart)-1]
		}
	}
	return ""
}

// githubDrainList walks a GitHub REST list endpoint to exhaustion (dc-3):
// per_page=100 on the first request, then the Link header's rel="next" URL
// verbatim (already carrying whatever query params GitHub itself minted)
// until no rel="next" member is present — which is also what a single-page
// contract-suite fake (no Link header set at all) naturally produces after
// one request, so those fakes keep passing unchanged (co-1/ac-1's
// equivalence proof).
//
// T is one page's decoded JSON shape (a bare array for most endpoints
// here, e.g. []pullRequestJSON; a wrapping object for the two whose page
// is `{"workflow_runs": [...]}` / `{"artifacts": [...]}`); items extracts
// that page's list of I out of the decoded T.
//
// A next-page URL identical to the URL just fetched fails loud instead of
// looping forever — a broken/malicious server's signature, not a
// legitimate pagination shape, and not something a hard page-count cap
// should paper over (co-1: "bounded by test design, not a production
// cap").
func githubDrainList[T any, I any](ctx context.Context, a *Adapter, firstURL string, items func(T) []I) ([]I, error) {
	var all []I
	next := withPerPage(firstURL)
	for next != "" {
		u := next
		var page T
		headers, err := a.transport.DoHeaders(ctx, http.MethodGet, u, nil, a.setAuth, a.classify(http.MethodGet, u, http.StatusOK), &page)
		if err != nil {
			return nil, err
		}
		all = append(all, items(page)...)

		next = parseLinkNext(headers.Get("Link"))
		if next == u {
			return nil, fmt.Errorf("github: pagination loop detected: Link rel=\"next\" repeats %s", u)
		}
	}
	return all, nil
}
