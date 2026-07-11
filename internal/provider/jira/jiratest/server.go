// Package jiratest is a minimal, in-memory, hermetic stand-in for the
// subset of the Jira Cloud REST API v3 the internal/provider/jira adapter
// uses: GET/PUT an issue, POST a comment. It exists so the adapter's own
// tests, its providertest.Harness, and cmd/verdi's rollup end-to-end tests
// can all drive real HTTP calls through net/http/httptest with no live
// network (CLAUDE.md: "No network in any test") and no duplicated mock
// logic between those three call sites.
//
// Server deliberately does not enforce Jira's real field-filtering
// (?fields=...) or authentication: it always returns every field it holds
// for an issue, and never itself checks the Authorization header. Adapter
// requests only decode the fields they need, so this simplification is
// invisible to a conforming client; failure-table scenarios (401/403/5xx)
// are exercised through ForceStatus instead of a real credential check.
package jiratest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
)

// issueState is one issue's server-side state.
type issueState struct {
	exists bool // false only for a key SeedNotFound pinned as missing

	summary, status string
	field           *string // current machine-field value; nil = unset
	comments        []string
	commitsWritten  map[string]bool // every distinct commit ever PUT, for PublishRecordCount-style observability
}

// Server is the mock. Embedding *httptest.Server exposes URL and Client()
// directly.
type Server struct {
	*httptest.Server

	mu          sync.Mutex
	rollupField string
	issues      map[string]*issueState
	forced      map[string][]int // key -> queued forced HTTP statuses, consumed FIFO
}

// NewServer starts a Server. rollupField is the custom field id the
// adapter under test is configured to read/write (verdi.yaml's
// providers.jira.rollup_field).
func NewServer(rollupField string) *Server {
	s := &Server{
		rollupField: rollupField,
		issues:      make(map[string]*issueState),
		forced:      make(map[string][]int),
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.handle))
	return s
}

// SeedIssue makes key resolve with the given summary/status. The issue's
// "self" link is not a parameter: this mock always serves a realistic,
// machine-facing REST self URL (see selfFor) that no test can shape, so
// test convenience can never masquerade as the human Story.URL — the
// adapter derives Story.URL from its own BaseURL, never from "self".
func (s *Server) SeedIssue(key, summary, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issues[key] = &issueState{exists: true, summary: summary, status: status, commitsWritten: make(map[string]bool)}
}

// selfFor returns the machine-facing REST resource URL Jira Cloud puts in
// an issue's "self" field: the /rest/api/3/issue/... endpoint, rooted at
// this server's own base. It is deliberately not the human browse link, so
// a client that (wrongly) mapped Story.URL from "self" would visibly fail.
func (s *Server) selfFor(key string) string {
	return s.Server.URL + "/rest/api/3/issue/" + key
}

// SeedNotFound pins key so every request touching it 404s, without a prior
// SeedIssue call. Any key that was never seeded at all (neither SeedIssue
// nor SeedNotFound) instead auto-vivifies as an empty, existing issue on
// first touch — the PublishRollup contract-suite cases publish to a story
// ref that was never resolved first, and a real Jira issue that a CI job
// is publishing rollups to obviously exists.
func (s *Server) SeedNotFound(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issues[key] = &issueState{exists: false}
}

// ForceStatus queues one forced HTTP status for the next request touching
// key (any verb), consumed after one use — for exercising 04's failure
// table (401/403/5xx) without a real broken server.
func (s *Server) ForceStatus(key string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.forced[key] = append(s.forced[key], status)
}

// FieldValue returns key's current machine-field value (the raw compact
// JSON string the adapter wrote) and whether it has been set at all.
func (s *Server) FieldValue(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.issues[key]
	if !ok || st.field == nil {
		return "", false
	}
	return *st.field, true
}

// CommentCount returns how many human comments have been posted for key.
func (s *Server) CommentCount(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.issues[key]
	if !ok {
		return 0
	}
	return len(st.comments)
}

// Comments returns a copy of every comment body (flattened to plain text)
// posted for key, in post order.
func (s *Server) Comments(key string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.issues[key]
	if !ok {
		return nil
	}
	out := make([]string, len(st.comments))
	copy(out, st.comments)
	return out
}

// PublishedCommitCount returns how many distinct commits have ever been
// written to key's machine field — the server-side observability the
// story-provider contract suite's publish-idempotency case needs (a real
// Jira field only holds the latest value; this is the mock's own audit
// trail of PUT calls, not something a production adapter reads back).
func (s *Server) PublishedCommitCount(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.issues[key]
	if !ok {
		return 0
	}
	return len(st.commitsWritten)
}

func parseIssuePath(p string) (key, sub string, ok bool) {
	const prefix = "/rest/api/3/issue/"
	if !strings.HasPrefix(p, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(p, prefix)
	if rest == "" {
		return "", "", false
	}
	if idx := strings.Index(rest, "/"); idx >= 0 {
		return rest[:idx], rest[idx+1:], true
	}
	return rest, "", true
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	key, sub, ok := parseIssuePath(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	s.mu.Lock()
	if queue := s.forced[key]; len(queue) > 0 {
		status := queue[0]
		s.forced[key] = queue[1:]
		s.mu.Unlock()
		w.WriteHeader(status)
		return
	}
	st, seeded := s.issues[key]
	if seeded && !st.exists {
		s.mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if !seeded {
		st = &issueState{exists: true, commitsWritten: make(map[string]bool)}
		s.issues[key] = st
	}
	s.mu.Unlock()

	switch {
	case r.Method == http.MethodGet && sub == "":
		s.handleGetIssue(w, key, st)
	case r.Method == http.MethodPut && sub == "":
		s.handlePutIssue(w, r, st)
	case r.Method == http.MethodPost && sub == "comment":
		s.handlePostComment(w, r, st)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleGetIssue(w http.ResponseWriter, key string, st *issueState) {
	s.mu.Lock()
	var fieldVal interface{}
	if st.field != nil {
		fieldVal = *st.field
	}
	resp := map[string]interface{}{
		"key":  key,
		"self": s.selfFor(key),
		"fields": map[string]interface{}{
			"summary":     st.summary,
			"status":      map[string]string{"name": st.status},
			s.rollupField: fieldVal,
		},
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handlePutIssue(w http.ResponseWriter, r *http.Request, st *issueState) {
	var body struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	val, ok := body.Fields[s.rollupField]
	if !ok {
		http.Error(w, "missing rollup field in PUT body", http.StatusBadRequest)
		return
	}
	var mini struct {
		Commit string `json:"commit"`
	}
	if err := json.Unmarshal([]byte(val), &mini); err != nil {
		http.Error(w, "rollup field value is not valid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	st.field = &val
	if mini.Commit != "" {
		st.commitsWritten[mini.Commit] = true
	}
	s.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePostComment(w http.ResponseWriter, r *http.Request, st *issueState) {
	var body struct {
		Body map[string]interface{} `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	text := strings.Join(extractADFText(body.Body), "\n")

	s.mu.Lock()
	st.comments = append(st.comments, text)
	n := len(st.comments)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": fmt.Sprintf("comment-%d", n)})
}

// extractADFText walks a minimal Atlassian Document Format document
// (doc -> content[paragraph] -> content[text]) and returns each text
// node's content in document order — the inverse of the jira package's
// buildCommentADF, for test assertions.
func extractADFText(doc map[string]interface{}) []string {
	var out []string
	content, _ := doc["content"].([]interface{})
	for _, node := range content {
		m, ok := node.(map[string]interface{})
		if !ok {
			continue
		}
		inner, _ := m["content"].([]interface{})
		for _, tn := range inner {
			tm, ok := tn.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := tm["text"].(string); ok {
				out = append(out, text)
			}
		}
	}
	return out
}
