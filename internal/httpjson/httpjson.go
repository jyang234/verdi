// Package httpjson is the shared HTTP-JSON transport seam (spec/forge-transport
// ac-1, dc-1): the one place that builds an outbound JSON request, applies a
// caller-supplied auth/header hook, bounds the call with a deadline, hands
// the completed round trip to a caller-supplied status classifier, and
// decodes a successful response. internal/forge/github, internal/forge/gitlab,
// and internal/provider/jira all ride it; no adapter carries its own copy of
// this plumbing.
//
// DECODE POLICY (code-health dc-1, the single disclosure site): tolerant
// decode for FOREIGN payloads is POLICY, not a violation. Strict decode
// (DisallowUnknownFields + trailing-data rejection, internal/artifact's
// seam) is for verdi-owned artifacts and pinned upstream-CLI JSON — decode
// failures there mean verdi produced or consumed a malformed artifact of
// its own. A forge or tracker API response is a FOREIGN payload: this
// package decodes it as a tolerant subset via encoding/json's default
// decoder, ignoring both unknown fields and any data trailing the decoded
// value. DisallowUnknownFields here would turn every upstream field
// addition — a GitHub/GitLab/Jira release note, not a verdi change — into a
// verdi outage. This is the ONLY place that decode policy is stated for the
// three adapters that ride this seam.
//
// RETRIES: this package does NOT own retries. No retry logic exists in the
// callers it replaces, and none is added here (code-health dc-2,
// witness-scoped: only pagination drain, the timeout, 429 classification,
// and the bundle-pin refusal are in scope for spec/forge-transport, and
// retries are none of those).
package httpjson

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultTimeout is the seam's default client deadline (dc-2): an obvious,
// generous ceiling for a single REST/GraphQL call. It applies only when a
// Client's HTTPClient field is nil; a caller-supplied HTTPClient — however
// it is configured, including an explicit zero Timeout — is used AS-IS, so
// existing injected-client test fixtures and any future tuning keep working
// without a code change here.
const DefaultTimeout = 30 * time.Second

// defaultClient is the client every Client with a nil HTTPClient shares.
// Stateless (no cookies, no redirect customization) so sharing it across
// calls and adapters is safe.
var defaultClient = &http.Client{Timeout: DefaultTimeout}

// Classify inspects one completed round trip and decides its outcome.
// transportErr is the error http.Client.Do returned (nil on a successful
// round trip); resp is the received response (nil when transportErr != nil,
// non-nil and unread otherwise — its body is available on the resp passed
// in). Classify owns the ENTIRE decision: which statuses are success, which
// sentinel a failure maps to, and the error's message prefix (dc-1: "forge
// and tracker keep their own sentinel taxonomies"). A nil return means
// success — Do proceeds to decode resp's body into its out parameter. A
// non-nil return aborts Do with that error, unchanged; Do never rewraps it.
//
// Classify must return a non-nil error whenever transportErr != nil — the
// call failed regardless of what Classify decides its shape should be.
type Classify func(resp *http.Response, transportErr error) error

// Client is the shared transport. The zero Client is ready to use: a nil
// HTTPClient makes every call ride defaultClient (DefaultTimeout).
type Client struct {
	// HTTPClient issues requests. nil selects a default client bounded by
	// DefaultTimeout (dc-2). A non-nil value — including one with a zero
	// Timeout, e.g. an httptest fixture's dedicated client — is used
	// exactly as given; this package never mutates or wraps it.
	HTTPClient *http.Client
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return defaultClient
}

// RawDo builds a request for method/url — encoding body as the JSON request
// payload and setting Content-Type when body is non-nil, leaving the
// request bodyless when body is nil — applies setAuth (nil is a valid no-op
// header-setter), and executes it against the resolved client. It returns
// the raw, unread *http.Response for callers whose response is not a JSON
// payload this package should decode (e.g. a binary artifact download) or
// that need response handling Do's Classify/decode shape does not fit. The
// caller MUST close resp.Body when err is nil.
func (c *Client) RawDo(ctx context.Context, method, url string, body interface{}, setAuth func(*http.Request)) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("httpjson: encoding request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("httpjson: building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if setAuth != nil {
		setAuth(req)
	}

	return c.httpClient().Do(req)
}

// Do issues one request (RawDo's request build) and, once the round trip
// completes, hands it to classify. A non-nil classify result is returned
// unchanged — success or failure, classify's sentinel and message stand.
// Only when classify reports success (nil, and thus only reachable when
// transportErr == nil) does Do decode resp's body into out — nil out drains
// and discards the body instead, matching a caller that only cares about
// the status (e.g. jira's field-write PUT). Decode failures are NOT run
// through classify: a malformed response body from a forge/tracker that
// otherwise reported success is a shape verdi's own subset type got wrong,
// not a forge-reported condition, so it is wrapped generically.
//
// classify must be non-nil.
func (c *Client) Do(ctx context.Context, method, url string, body interface{}, setAuth func(*http.Request), classify Classify, out interface{}) error {
	resp, err := c.RawDo(ctx, method, url, body, setAuth)
	if err != nil {
		if cerr := classify(nil, err); cerr != nil {
			return cerr
		}
		// classify chose not to override a transport failure — should not
		// happen for a well-behaved classifier (see Classify's doc), but
		// still surfaced rather than silently swallowed.
		return fmt.Errorf("httpjson: %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	if cerr := classify(resp, nil); cerr != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return cerr
	}

	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	// Tolerant-subset decode — see the package doc's DECODE POLICY section,
	// the single disclosure site for all three adapters riding this seam.
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("httpjson: decoding response from %s %s: %w", method, url, err)
	}
	return nil
}
