package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/OWNER/verdi/internal/provider"
)

// Config configures Adapter. BaseURL and HTTPClient are overridable so
// tests can point the adapter at an httptest server with no network
// (CLAUDE.md: "No network in any test").
type Config struct {
	// BaseURL is the Jira Cloud site root the adapter connects to, e.g.
	// "https://example.atlassian.net" (verdi.yaml's providers.jira.base_url,
	// I-4-style: ids/URLs live in the manifest, never credentials).
	BaseURL string
	// RollupField is the custom field id the machine payload is written to
	// (verdi.yaml's providers.jira.rollup_field, e.g. "customfield_10050").
	RollupField string
	// Token authenticates API calls, sent as a bearer token. Read from
	// VERDI_JIRA_TOKEN by the caller (04 §Jira adapter: "Secrets: token via
	// VERDI_JIRA_TOKEN ... never in verdi.yaml") — this package never reads
	// the environment for it itself, so it stays testable with an
	// arbitrary or absent token.
	Token string
	// HTTPClient defaults to http.DefaultClient.
	HTTPClient *http.Client
	// Getenv defaults to os.Getenv; overridden in tests so the MR/pipeline
	// link in the human comment is hermetically testable.
	Getenv func(string) string
}

// Adapter implements provider.StoryProvider against the Jira Cloud REST
// API v3 (04 §Jira adapter).
type Adapter struct{ cfg Config }

// New returns an Adapter with cfg's defaults filled in.
func New(cfg Config) *Adapter {
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.Getenv == nil {
		cfg.Getenv = os.Getenv
	}
	return &Adapter{cfg: cfg}
}

var _ provider.StoryProvider = (*Adapter)(nil)

// issueResponse is the subset of a Jira issue GET response this adapter
// reads. It is decoded with the standard (non-strict) json package,
// deliberately: Jira's real payloads carry many fields this adapter does
// not care about, unlike verdi's own internal schemas which go through the
// strict-decode seam in internal/artifact.
type issueResponse struct {
	Key string `json:"key"`
	// Self is Jira's own issue-identifying URL from the GET response. It is
	// used as Story.URL unchanged (see Resolve's doc comment) rather than
	// derived by string surgery from BaseURL.
	Self   string `json:"self"`
	Fields struct {
		Summary string `json:"summary"`
		Status  struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

// Resolve implements provider.StoryProvider (04 §Jira adapter: "GET
// /rest/api/3/issue/{key} -> key, summary, status, URL").
//
// URL is mapped from the response's own "self" field rather than
// constructed as BaseURL+"/browse/"+key: Jira's REST responses carry "self"
// as the issue's own canonical URL, and reading it through unchanged means
// this adapter never has to invent site-root parsing 04 doesn't specify.
func (a *Adapter) Resolve(ctx context.Context, ref provider.StoryRef) (provider.Story, error) {
	_, key, err := provider.ParseStoryRef(ref)
	if err != nil {
		return provider.Story{}, err
	}

	var resp issueResponse
	path := fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,status", key)
	if err := a.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return provider.Story{}, fmt.Errorf("jira: resolve %s: %w", ref, err)
	}

	return provider.Story{
		Ref:    ref,
		Title:  resp.Fields.Summary,
		Status: resp.Fields.Status.Name,
		URL:    resp.Self,
	}, nil
}

// rollupPayload is the machine field's compact JSON payload (04 §Jira
// adapter, verbatim): "{ commit, eligible, criteria: [{id, status}] }".
type rollupPayload struct {
	Commit   string             `json:"commit"`
	Eligible bool               `json:"eligible"`
	Criteria []criterionPayload `json:"criteria"`
}

type criterionPayload struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func toCriteriaPayload(cs []provider.CriterionStatus) []criterionPayload {
	out := make([]criterionPayload, len(cs))
	for i, c := range cs {
		out[i] = criterionPayload{ID: c.ID, Status: c.Status}
	}
	return out
}

// criteriaChanged reports whether the per-AC Status values differ between a
// and b, comparing by ID (order-independent) — mirrors
// internal/provider/fake's criteriaStatusesChanged so both the fake and
// this adapter implement 04 §Semantics's "any AC status changed since the
// last publish" identically. A criterion appearing in one set but not the
// other counts as a change.
func criteriaChanged(a, b []criterionPayload) bool {
	statusesByID := func(cs []criterionPayload) map[string]string {
		m := make(map[string]string, len(cs))
		for _, c := range cs {
			m[c.ID] = c.Status
		}
		return m
	}
	am, bm := statusesByID(a), statusesByID(b)
	if len(am) != len(bm) {
		return true
	}
	for id, st := range am {
		if bm[id] != st {
			return true
		}
	}
	return false
}

// PublishRollup implements provider.StoryProvider (04 §Jira adapter +
// §Semantics). It reads the adapter's own previously published field back
// first (change detection, never a second read of tracker-owned data),
// writes the machine field unconditionally (idempotent overwrite on
// (story, commit)), and posts a human comment only when AC statuses
// changed since the last publish — including the very first publish
// (ledger I-26: "the PM learns the initial state").
func (a *Adapter) PublishRollup(ctx context.Context, r provider.Rollup) error {
	_, key, err := provider.ParseStoryRef(r.Story)
	if err != nil {
		return err
	}

	prev, err := a.readOwnField(ctx, key)
	if err != nil {
		return fmt.Errorf("jira: publish rollup for %s: %w", r.Story, err)
	}

	next := rollupPayload{
		Commit:   r.Commit,
		Eligible: r.Eligible,
		Criteria: toCriteriaPayload(r.Criteria),
	}
	changed := prev == nil || criteriaChanged(prev.Criteria, next.Criteria)

	if err := a.writeField(ctx, key, next); err != nil {
		return fmt.Errorf("jira: publish rollup for %s: %w", r.Story, err)
	}

	if changed {
		if err := a.postComment(ctx, key, r); err != nil {
			return fmt.Errorf("jira: publish rollup for %s: %w", r.Story, err)
		}
	}
	return nil
}

// fieldsRawResponse is used only to read back the adapter's own rollup
// field: fields are decoded as raw JSON first (rather than a fixed struct)
// because a Jira issue's fields object mixes types (strings, nested
// objects, null) that a single Go type cannot represent uniformly.
type fieldsRawResponse struct {
	Fields map[string]json.RawMessage `json:"fields"`
}

// readOwnField reads back this adapter's own machine field for key. A nil
// result with a nil error means no previous state — either the field was
// never set, or it decodes to JSON null/empty string — which PublishRollup
// treats as "first publish" (I-26).
func (a *Adapter) readOwnField(ctx context.Context, key string) (*rollupPayload, error) {
	var resp fieldsRawResponse
	path := fmt.Sprintf("/rest/api/3/issue/%s?fields=%s", key, a.cfg.RollupField)
	if err := a.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, fmt.Errorf("reading rollup field: %w", err)
	}

	raw, ok := resp.Fields[a.cfg.RollupField]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var fieldStr string
	if err := json.Unmarshal(raw, &fieldStr); err != nil {
		return nil, fmt.Errorf("rollup field value is not a JSON string: %w", err)
	}
	if fieldStr == "" {
		return nil, nil
	}
	var payload rollupPayload
	if err := json.Unmarshal([]byte(fieldStr), &payload); err != nil {
		return nil, fmt.Errorf("decoding previous rollup payload: %w", err)
	}
	return &payload, nil
}

// writeField overwrites the machine field with payload's compact JSON
// encoding — a plain PUT, always issued (idempotent: writing the same
// value twice leaves the field in the same state).
func (a *Adapter) writeField(ctx context.Context, key string, payload rollupPayload) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encoding rollup payload: %w", err)
	}
	body := map[string]interface{}{
		"fields": map[string]string{a.cfg.RollupField: string(encoded)},
	}
	path := fmt.Sprintf("/rest/api/3/issue/%s", key)
	if err := a.doJSON(ctx, http.MethodPut, path, body, nil); err != nil {
		return fmt.Errorf("writing rollup field: %w", err)
	}
	return nil
}

// postComment posts the human comment: the criteria table plus an
// MR/pipeline link from CI env vars when present (04 §Jira adapter).
func (a *Adapter) postComment(ctx context.Context, key string, r provider.Rollup) error {
	body := map[string]interface{}{"body": buildCommentADF(r, a.cfg.Getenv)}
	path := fmt.Sprintf("/rest/api/3/issue/%s/comment", key)
	if err := a.doJSON(ctx, http.MethodPost, path, body, nil); err != nil {
		return fmt.Errorf("posting comment: %w", err)
	}
	return nil
}

// doJSON issues an HTTP request against a.cfg.BaseURL+path, optionally
// encoding body as the JSON request body and decoding the response into
// out (skipped when out is nil). Every failure is classified into 04's
// failure-table sentinels.
func (a *Adapter) doJSON(ctx context.Context, method, path string, body, out interface{}) error {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.cfg.BaseURL+path, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if a.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	}

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		// A network failure, DNS failure, connection refusal, or a context
		// deadline/cancellation all read as "the tracker could not be
		// reached" (04's failure table: "Unavailable/timeout").
		return fmt.Errorf("%s %s: %w: %v", method, path, provider.ErrUnavailable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		if out == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decoding response from %s %s: %w", method, path, err)
		}
		return nil
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%s %s: %w", method, path, provider.ErrNotFound)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%s %s: %w", method, path, provider.ErrUnauthorized)
	case resp.StatusCode >= 500:
		return fmt.Errorf("%s %s: %w: status %s", method, path, provider.ErrUnavailable, resp.Status)
	default:
		return fmt.Errorf("%s %s: unexpected status %s", method, path, resp.Status)
	}
}
