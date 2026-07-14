// Package github is the GitHub adapter for the I-22 forge port: it fetches
// verdi's own CI workflow's ("verdi-evidence", I-8) artifact via GitHub's
// Actions artifacts REST API and reports GitHub's generated-file attribute
// token and CI context.
package github

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/httpjson"
)

// defaultArtifactName is verdi's own CI workflow's uploaded artifact name
// (I-8: "job/workflow verdi-evidence uploads the derived/<ref-slug>/<commit>/
// tree as its artifact").
const defaultArtifactName = "verdi-evidence"

// Config configures Adapter. BaseURL and HTTPClient are overridable so
// tests can point the adapter at an httptest server with no network.
type Config struct {
	// BaseURL is the GitHub REST API root, e.g.
	// "https://api.github.com". Defaults to "https://api.github.com".
	BaseURL string
	// Owner and Repo identify the repository.
	Owner string
	Repo  string
	// Token authenticates API calls (GITHUB_TOKEN or a PAT) — read from
	// CI-provided env vars by callers, never stored in verdi.yaml.
	Token string
	// ArtifactName is the workflow artifact fetched. Defaults to
	// "verdi-evidence".
	ArtifactName string
	// HTTPClient defaults to nil: the adapter rides internal/httpjson's
	// shared transport, which itself defaults to a client bounded by
	// httpjson.DefaultTimeout (30s, spec/forge-transport dc-2) when this
	// field is left nil. A caller-supplied client is used AS-IS.
	HTTPClient *http.Client
	// Getenv defaults to os.Getenv; overridable for hermetic CIContext tests.
	Getenv func(string) string
}

// Adapter implements forge.Forge against the GitHub REST API.
type Adapter struct {
	cfg       Config
	transport *httpjson.Client
}

// New returns an Adapter with cfg's defaults filled in.
func New(cfg Config) *Adapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.github.com"
	}
	if cfg.ArtifactName == "" {
		cfg.ArtifactName = defaultArtifactName
	}
	if cfg.Getenv == nil {
		cfg.Getenv = os.Getenv
	}
	return &Adapter{cfg: cfg, transport: &httpjson.Client{HTTPClient: cfg.HTTPClient}}
}

type runsResponse struct {
	WorkflowRuns []run `json:"workflow_runs"`
}

type run struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

type artifactsResponse struct {
	Artifacts []artifact `json:"artifacts"`
}

type artifact struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// FetchEvidenceBundle implements forge.Forge: a commit can carry MORE
// THAN ONE successful workflow run — GitHub Actions runs are scoped per
// workflow FILE (unlike GitLab, where one pipeline covers every job), and
// this repo has run more than one workflow against the same commit before
// (spec/remote-and-ci's original verify.yml + verdi-evidence.yml split;
// round 6/spec/close-verb folded the evidence-producing steps into
// verify.yml's own job, but a manual re-run or a future second workflow
// could still reintroduce more than one candidate run for one commit), so
// the head_sha query below may return more than one id. This tries every
// successful run for commit, in the order the API returns them, until one
// actually carries the wanted artifact — it never assumes the first
// successful run is the verdi-evidence one.
func (a *Adapter) FetchEvidenceBundle(ctx context.Context, ref, commit string) (forge.DerivedTree, error) {
	runIDs, err := a.findRuns(ctx, commit)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for _, runID := range runIDs {
		artifactID, err := a.findArtifact(ctx, runID)
		if err != nil {
			if errors.Is(err, forge.ErrNoBundle) {
				lastErr = err
				continue
			}
			return nil, err
		}
		data, err := a.downloadArtifact(ctx, artifactID)
		if err != nil {
			return nil, err
		}
		tree, err := forge.ExtractTreeFromZip(data)
		if err != nil {
			return nil, fmt.Errorf("github: %w", err)
		}
		return tree, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("github: no successful workflow run for commit %s: %w", commit, forge.ErrNoBundle)
	}
	return nil, lastErr
}

// findRuns returns every successful workflow run's id for commit, in the
// order GitHub's API lists them. Drains every page (dc-3): a commit whose
// matching run is NOT among the first 100 runs GitHub returns (an
// active/busy repo) must still be found, not silently missed.
func (a *Adapter) findRuns(ctx context.Context, commit string) ([]int64, error) {
	listURL := fmt.Sprintf("%s/repos/%s/%s/actions/runs?head_sha=%s&status=success", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, commit)
	runs, err := githubDrainList(ctx, a, listURL, func(p runsResponse) []run { return p.WorkflowRuns })
	if err != nil {
		return nil, err
	}
	var ids []int64
	for _, r := range runs {
		if r.Conclusion == "success" {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("github: no successful workflow run for commit %s: %w", commit, forge.ErrNoBundle)
	}
	return ids, nil
}

func (a *Adapter) findArtifact(ctx context.Context, runID int64) (int64, error) {
	listURL := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/artifacts", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, runID)
	artifacts, err := githubDrainList(ctx, a, listURL, func(p artifactsResponse) []artifact { return p.Artifacts })
	if err != nil {
		return 0, err
	}
	for _, art := range artifacts {
		if art.Name == a.cfg.ArtifactName {
			return art.ID, nil
		}
	}
	return 0, fmt.Errorf("github: run %d has no %q artifact: %w", runID, a.cfg.ArtifactName, forge.ErrNoBundle)
}

// downloadArtifact rides httpjson.Client.RawDo rather than Do: the response
// body is a binary zip, not a JSON payload this package should decode
// (dc-1's tolerant-subset decode does not apply — there is nothing to
// decode).
func (a *Adapter) downloadArtifact(ctx context.Context, artifactID int64) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/artifacts/%d/zip", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, artifactID)
	resp, err := a.transport.RawDo(ctx, http.MethodGet, url, nil, a.setAuth)
	if err != nil {
		return nil, fmt.Errorf("github: downloading artifact %d: %w", artifactID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		return nil, fmt.Errorf("github: artifact %d unavailable: %w", artifactID, forge.ErrNoBundle)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: downloading artifact %d: unexpected status %s", artifactID, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("github: reading artifact %d: %w", artifactID, err)
	}
	return data, nil
}

// getJSON/postJSON are the thin bindings the ac-1 seam calls for: they bind
// httpjson.Client.Do to this adapter's own auth header, error prefix, and
// status classification (2xx-family success; 429 named as rate-limited,
// since forge carries no unavailable-style sentinel today — spec/forge-
// transport ac-3; anything else a generic "unexpected status" error) —
// httpjson itself owns none of that taxonomy (dc-1).
func (a *Adapter) getJSON(ctx context.Context, url string, out interface{}) error {
	return a.transport.Do(ctx, http.MethodGet, url, nil, a.setAuth, a.classify(http.MethodGet, url, http.StatusOK), out)
}

// postJSON mirrors getJSON for the write direction: encode body as the
// JSON request payload, decode the response into out. GitHub's create
// endpoints (comments) reply 201 Created rather than 200, so the success
// status is a parameter (classify's third argument).
func (a *Adapter) postJSON(ctx context.Context, url string, body, out interface{}) error {
	return a.transport.Do(ctx, http.MethodPost, url, body, a.setAuth, a.classifyPost(url), out)
}

func (a *Adapter) setAuth(req *http.Request) {
	if a.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	}
}

// classify builds the httpjson.Classify getJSON binds: wantStatus is the
// one success status (200 for every GitHub GET this adapter issues).
func (a *Adapter) classify(method, url string, wantStatus int) httpjson.Classify {
	return func(resp *http.Response, transportErr error) error {
		if transportErr != nil {
			return fmt.Errorf("github: %s %s: %w", method, url, transportErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("github: %s %s: rate limited: status %s", method, url, resp.Status)
		}
		if resp.StatusCode != wantStatus {
			return fmt.Errorf("github: %s %s: unexpected status %s", method, url, resp.Status)
		}
		return nil
	}
}

// classifyPost mirrors classify for POST, whose success is 200 OR 201
// depending on the endpoint (GitHub's create-comment endpoints reply 201;
// the GraphQL endpoint replies 200).
func (a *Adapter) classifyPost(url string) httpjson.Classify {
	return func(resp *http.Response, transportErr error) error {
		if transportErr != nil {
			return fmt.Errorf("github: POST %s: %w", url, transportErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("github: POST %s: rate limited: status %s", url, resp.Status)
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("github: POST %s: unexpected status %s", url, resp.Status)
		}
		return nil
	}
}

// pullRequestJSON is the subset of GitHub's pull request object
// ListOpenMRs needs (GitHub API: "List pull requests").
type pullRequestJSON struct {
	Number int64  `json:"number"`
	Title  string `json:"title"`
	Head   struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

// ListOpenMRs implements forge.Forge: GitHub's "list pull requests"
// endpoint, filtered server-side to open PRs based on targetBranch. Drains
// every page (dc-3): an open PR beyond the default page size must still be
// seen by pendingsupersession.go's scan, not silently dropped.
func (a *Adapter) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&base=%s",
		a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, url.QueryEscape(targetBranch))
	prs, err := githubDrainList(ctx, a, reqURL, func(p []pullRequestJSON) []pullRequestJSON { return p })
	if err != nil {
		return nil, err
	}
	out := make([]forge.OpenMR, len(prs))
	for i, p := range prs {
		out[i] = forge.OpenMR{ID: strconv.FormatInt(p.Number, 10), SourceBranch: p.Head.Ref, Title: p.Title}
	}
	return out, nil
}

// repoContentJSON is the subset of GitHub's "Get repository content"
// response FetchFileAtRef needs: base64-encoded content plus its encoding
// tag.
type repoContentJSON struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// FetchFileAtRef implements forge.Forge against GitHub's "Get repository
// content" endpoint (base64-encoded content for a file path).
func (a *Adapter) FetchFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s",
		a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, path, url.QueryEscape(ref))

	classify := func(resp *http.Response, transportErr error) error {
		if transportErr != nil {
			return fmt.Errorf("github: GET %s: %w", reqURL, transportErr)
		}
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("github: file %q not found at ref %q: %w", path, ref, forge.ErrFileNotFound)
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("github: GET %s: rate limited: status %s", reqURL, resp.Status)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("github: GET %s: unexpected status %s", reqURL, resp.Status)
		}
		return nil
	}

	var rc repoContentJSON
	if err := a.transport.Do(ctx, http.MethodGet, reqURL, nil, a.setAuth, classify, &rc); err != nil {
		return nil, err
	}
	if rc.Encoding != "" && rc.Encoding != "base64" {
		return nil, fmt.Errorf("github: file %q at ref %q: unsupported encoding %q", path, ref, rc.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(rc.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("github: decoding base64 content for %q at ref %q: %w", path, ref, err)
	}
	return data, nil
}

// reviewCommentJSON is the subset of GitHub's diff-anchored PR review
// comment object (REST `pulls/comments`, S6 capture
// `github/01-list-review-comments-REST.json`) ListComments/PostComment
// need. Line is a pointer because GitHub nulls it once a force-push
// breaks the original commit's ancestry (S6 Q4, capture
// `09-...-after-force-push.json`).
type reviewCommentJSON struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	Path string `json:"path"`
	Line *int   `json:"line"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	CreatedAt string `json:"created_at"`
}

// issueCommentJSON is the subset of GitHub's general (non-diff) PR
// conversation comment object (REST `issues/comments`, S6 capture
// `github/02-list-issue-comments-REST.json`) ListComments/PostComment
// need — no path/line at all, the "two comment universes" finding
// (comments.go's package doc).
type issueCommentJSON struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	CreatedAt string `json:"created_at"`
}

// reviewThreadsQuery is the GraphQL query resolution state requires (S6
// Q2, live-verified: "isResolved... exist only via the GraphQL
// reviewThreads query"; capture `github/03-review-threads-GraphQL-before-resolve.json`).
// databaseId lets the REST-sourced diff comments above (ListComments) be
// grouped into the same threads this query reports resolution for,
// without ListComments itself depending on GraphQL for body/author/path —
// only for the thread-id/resolution join. after:$cursor plus the outer
// pageInfo drains reviewThreads to exhaustion (dc-3: "an unresolved state
// can hide" past first:100); each node's own comments pageInfo is carried
// back too, so fetchReviewThreads can tell which threads need their INNER
// comments cursor walked as well (threadCommentsQuery below) — comments
// beyond first:100 on one thread would otherwise never be joined to a diff
// comment ListComments read from REST.
const reviewThreadsQuery = `query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          isResolved
          resolvedBy { login }
          comments(first: 100) {
            pageInfo { hasNextPage endCursor }
            nodes { databaseId }
          }
        }
      }
    }
  }
}`

// threadCommentsQuery continues one thread's comments cursor past
// reviewThreadsQuery's own first:100 (dc-3's inner walk). GitHub's global
// object identification (`node(id: ID!)`) re-fetches the same
// PullRequestReviewThread node fetchReviewThreads already has the id for,
// asking only for the next comments page — no second reviewThreads round
// trip, no owner/repo/number needed again.
const threadCommentsQuery = `query($threadID: ID!, $cursor: String) {
  node(id: $threadID) {
    ... on PullRequestReviewThread {
      comments(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes { databaseId }
      }
    }
  }
}`

type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type graphQLError struct {
	Message string `json:"message"`
}

type graphQLPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

type threadCommentNode struct {
	DatabaseID int64 `json:"databaseId"`
}

type reviewThreadNode struct {
	ID         string `json:"id"`
	IsResolved bool   `json:"isResolved"`
	ResolvedBy *struct {
		Login string `json:"login"`
	} `json:"resolvedBy"`
	Comments struct {
		PageInfo graphQLPageInfo     `json:"pageInfo"`
		Nodes    []threadCommentNode `json:"nodes"`
	} `json:"comments"`
}

type reviewThreadsResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					PageInfo graphQLPageInfo    `json:"pageInfo"`
					Nodes    []reviewThreadNode `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

type threadCommentsResponse struct {
	Data struct {
		Node struct {
			Comments struct {
				PageInfo graphQLPageInfo     `json:"pageInfo"`
				Nodes    []threadCommentNode `json:"nodes"`
			} `json:"comments"`
		} `json:"node"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// graphQLCursor renders a walker's cursor variable: "" (the initial/no-
// cursor state) becomes GraphQL null, not the literal empty string — a
// server that treats `after: ""` differently from `after: null` must still
// see the standard "no cursor yet" shape on the first request.
func graphQLCursor(cursor string) any {
	if cursor == "" {
		return nil
	}
	return cursor
}

// fetchReviewThreads runs reviewThreadsQuery against GitHub's GraphQL v4
// endpoint (BaseURL + "/graphql" — the real API serves GraphQL on the same
// host as REST, at a fixed sibling path, so no separate config knob is
// needed; httptest doubles serve it from the same mux). Shared by
// ListComments (thread-id grouping only) and GetThreadResolution (full
// resolution state) — one query, two consumers, no duplicated transport
// code (CLAUDE.md). Drains BOTH cursors (dc-3): the outer reviewThreads
// list via pageInfo/endCursor, and — for any thread whose OWN comments
// page is not exhausted — that thread's inner comments cursor too, via
// fetchThreadCommentsOverflow.
func (a *Adapter) fetchReviewThreads(ctx context.Context, mrID string) ([]reviewThreadNode, error) {
	number, err := strconv.Atoi(mrID)
	if err != nil {
		return nil, fmt.Errorf("github: mrID %q is not a PR number: %w", mrID, err)
	}

	var all []reviewThreadNode
	cursor := ""
	for {
		reqBody := graphQLRequest{
			Query:     reviewThreadsQuery,
			Variables: map[string]any{"owner": a.cfg.Owner, "repo": a.cfg.Repo, "number": number, "cursor": graphQLCursor(cursor)},
		}
		var parsed reviewThreadsResponse
		if err := a.postJSON(ctx, a.cfg.BaseURL+"/graphql", reqBody, &parsed); err != nil {
			return nil, fmt.Errorf("github: GraphQL reviewThreads query: %w", err)
		}
		if len(parsed.Errors) > 0 {
			return nil, fmt.Errorf("github: GraphQL reviewThreads query failed: %s", parsed.Errors[0].Message)
		}
		all = append(all, parsed.Data.Repository.PullRequest.ReviewThreads.Nodes...)

		pi := parsed.Data.Repository.PullRequest.ReviewThreads.PageInfo
		if !pi.HasNextPage {
			break
		}
		if pi.EndCursor == cursor {
			return nil, fmt.Errorf("github: GraphQL reviewThreads pagination loop detected: endCursor repeats %q", cursor)
		}
		cursor = pi.EndCursor
	}

	// dc-3's inner walk: a thread's own comments page not yet exhausted
	// cannot itself flip isResolved (thread-level field — see
	// GetThreadResolution's doc comment: resolution here is NOT
	// comment-derived), but it CAN hide a diff comment's databaseId from
	// ListComments' thread-id join, so it is drained too rather than
	// silently left at first:100.
	for i := range all {
		if !all[i].Comments.PageInfo.HasNextPage {
			continue
		}
		more, err := a.fetchThreadCommentsOverflow(ctx, all[i].ID, all[i].Comments.PageInfo.EndCursor)
		if err != nil {
			return nil, err
		}
		all[i].Comments.Nodes = append(all[i].Comments.Nodes, more...)
	}

	return all, nil
}

// fetchThreadCommentsOverflow continues threadID's comments cursor past
// wherever reviewThreadsQuery's own embedded first:100 page left off (dc-3
// inner walk); startCursor is that page's endCursor, never "" (the caller
// only invokes this when hasNextPage was true, i.e. an endCursor exists).
func (a *Adapter) fetchThreadCommentsOverflow(ctx context.Context, threadID, startCursor string) ([]threadCommentNode, error) {
	var all []threadCommentNode
	cursor := startCursor
	for {
		reqBody := graphQLRequest{
			Query:     threadCommentsQuery,
			Variables: map[string]any{"threadID": threadID, "cursor": graphQLCursor(cursor)},
		}
		var parsed threadCommentsResponse
		if err := a.postJSON(ctx, a.cfg.BaseURL+"/graphql", reqBody, &parsed); err != nil {
			return nil, fmt.Errorf("github: GraphQL thread %q comments query: %w", threadID, err)
		}
		if len(parsed.Errors) > 0 {
			return nil, fmt.Errorf("github: GraphQL thread %q comments query failed: %s", threadID, parsed.Errors[0].Message)
		}
		all = append(all, parsed.Data.Node.Comments.Nodes...)

		pi := parsed.Data.Node.Comments.PageInfo
		if !pi.HasNextPage {
			break
		}
		if pi.EndCursor == cursor {
			return nil, fmt.Errorf("github: GraphQL thread %q comments pagination loop detected: endCursor repeats %q", threadID, cursor)
		}
		cursor = pi.EndCursor
	}
	return all, nil
}

// ListComments implements forge.Forge: merges GitHub's two comment
// universes (S6 finding) — diff-anchored REST `pulls/{mrID}/comments` and
// general REST `issues/{mrID}/comments` — into one feed, joining each diff
// comment to its GraphQL thread id (fetchReviewThreads) so
// GetThreadResolution's entries can be matched back to it. General
// comments carry no thread id at all (ThreadID stays "") — GitHub's model
// has no resolution concept for them. Both feeds drain every page (dc-3).
func (a *Adapter) ListComments(ctx context.Context, mrID string) ([]forge.Comment, error) {
	diffURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%s/comments", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, mrID)
	diff, err := githubDrainList(ctx, a, diffURL, func(p []reviewCommentJSON) []reviewCommentJSON { return p })
	if err != nil {
		return nil, err
	}
	generalURL := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, mrID)
	general, err := githubDrainList(ctx, a, generalURL, func(p []issueCommentJSON) []issueCommentJSON { return p })
	if err != nil {
		return nil, err
	}

	threadByDBID := make(map[int64]string)
	if len(diff) > 0 {
		threads, err := a.fetchReviewThreads(ctx, mrID)
		if err != nil {
			return nil, err
		}
		for _, th := range threads {
			for _, c := range th.Comments.Nodes {
				threadByDBID[c.DatabaseID] = th.ID
			}
		}
	}

	out := make([]forge.Comment, 0, len(diff)+len(general))
	for _, c := range diff {
		line := 0
		if c.Line != nil {
			line = *c.Line
		}
		out = append(out, forge.Comment{
			ID: strconv.FormatInt(c.ID, 10), ThreadID: threadByDBID[c.ID], Body: c.Body,
			Author: c.User.Login, CreatedAt: c.CreatedAt, Path: c.Path, Line: line,
		})
	}
	for _, c := range general {
		out = append(out, forge.Comment{
			ID: strconv.FormatInt(c.ID, 10), Body: c.Body, Author: c.User.Login, CreatedAt: c.CreatedAt,
		})
	}
	return out, nil
}

// GetThreadResolution implements forge.Forge: GitHub's resolution state,
// GraphQL-only (S6 Q2, live-verified) — REST carries no resolution field
// whatsoever. Every reviewThreads node IS a substantive thread by
// GitHub's own model (there is no unresolvable-diff-thread concept), so
// every node fetchReviewThreads returns becomes one entry here.
func (a *Adapter) GetThreadResolution(ctx context.Context, mrID string) ([]forge.ThreadResolution, error) {
	threads, err := a.fetchReviewThreads(ctx, mrID)
	if err != nil {
		return nil, err
	}
	out := make([]forge.ThreadResolution, len(threads))
	for i, th := range threads {
		tr := forge.ThreadResolution{ThreadID: th.ID, Resolved: th.IsResolved}
		if th.ResolvedBy != nil {
			tr.ResolvedBy = th.ResolvedBy.Login
		}
		out[i] = tr
	}
	return out, nil
}

// createReviewCommentRequest is the POST body `pulls/{mrID}/comments`
// requires: body plus the head commit sha, path, line, and side (S6 Q1:
// "Posting requires body, commit_id (head sha), path, line").
type createReviewCommentRequest struct {
	Body     string `json:"body"`
	CommitID string `json:"commit_id"`
	Path     string `json:"path"`
	Line     int    `json:"line"`
	Side     string `json:"side"`
}

type createIssueCommentRequest struct {
	Body string `json:"body"`
}

// pullHeadJSON is the subset of GitHub's pull request object PostComment
// needs to learn the head sha `commit_id` requires.
type pullHeadJSON struct {
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

// PostComment implements forge.Forge: a diff-anchored comment (target !=
// nil) via `pulls/{mrID}/comments`, or a general comment (target == nil)
// via `issues/{mrID}/comments`. KNOWN SIMPLIFICATION: a freshly posted
// diff comment's ThreadID is left "" — GitHub's REST create response
// carries no GraphQL thread id, and minting one would require a second
// GraphQL round-trip whose result (a brand-new, always-unresolved thread)
// no caller in this phase's scope needs immediately after posting; a
// caller that needs the thread id re-lists via ListComments, which joins
// it from fetchReviewThreads as usual.
func (a *Adapter) PostComment(ctx context.Context, mrID, body string, target *forge.CommentTarget) (forge.Comment, error) {
	if target == nil {
		return a.postIssueComment(ctx, mrID, body)
	}
	return a.postReviewComment(ctx, mrID, body, *target)
}

func (a *Adapter) postReviewComment(ctx context.Context, mrID, body string, target forge.CommentTarget) (forge.Comment, error) {
	var pr pullHeadJSON
	prURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%s", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, mrID)
	if err := a.getJSON(ctx, prURL, &pr); err != nil {
		return forge.Comment{}, fmt.Errorf("github: resolving PR head sha to post a review comment: %w", err)
	}

	reqBody := createReviewCommentRequest{Body: body, CommitID: pr.Head.SHA, Path: target.Path, Line: target.Line, Side: "RIGHT"}
	var created reviewCommentJSON
	postURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%s/comments", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, mrID)
	if err := a.postJSON(ctx, postURL, reqBody, &created); err != nil {
		return forge.Comment{}, err
	}
	return forge.Comment{ID: strconv.FormatInt(created.ID, 10), Body: created.Body, Author: created.User.Login, CreatedAt: created.CreatedAt, Path: created.Path, Line: target.Line}, nil
}

func (a *Adapter) postIssueComment(ctx context.Context, mrID, body string) (forge.Comment, error) {
	reqBody := createIssueCommentRequest{Body: body}
	var created issueCommentJSON
	postURL := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, mrID)
	if err := a.postJSON(ctx, postURL, reqBody, &created); err != nil {
		return forge.Comment{}, err
	}
	return forge.Comment{ID: strconv.FormatInt(created.ID, 10), Body: created.Body, Author: created.User.Login, CreatedAt: created.CreatedAt}, nil
}

// GeneratedAttribute implements forge.Forge (02 §Repository plumbing,
// VL-012).
func (a *Adapter) GeneratedAttribute() string { return "linguist-generated" }

// CIContext implements forge.Forge, reading GitHub Actions' own default
// environment variables: GITHUB_EVENT_NAME (presence of "pull_request"
// signals a PR run), GITHUB_BASE_REF (the PR's target branch). GitHub
// Actions has no default-branch env var; DefaultBranch reads
// VERDI_GITHUB_DEFAULT_BRANCH, an operator-set fallback (documented
// limitation — GitHub does not expose this without an API call this
// package deliberately avoids making from CIContext, which must stay a
// pure, offline env-var read).
//
// Pipeline reads GITHUB_RUN_ID (a unique id per workflow run — GitHub
// Actions' nearest analogue of GitLab's pipeline id). Job reads
// GITHUB_RUN_ATTEMPT rather than GITHUB_JOB: GITHUB_JOB is the constant
// job-key string from the workflow YAML (e.g. "verdi-evidence") and does
// not change across a re-run, so it cannot order retries; GITHUB_RUN_ATTEMPT
// increments by 1 each time a run is re-run and is GitHub's closest
// analogue of GitLab's monotonically-increasing per-retry CI_JOB_ID (03
// §The fold's (pipeline id, job id) ordering, I-25) — a disclosed choice,
// since GitHub exposes no job-scoped numeric id as an env var at all.
func (a *Adapter) CIContext(ctx context.Context) (forge.CIInfo, error) {
	if err := ctx.Err(); err != nil {
		return forge.CIInfo{}, err
	}
	info := forge.CIInfo{
		DefaultBranch: a.cfg.Getenv("VERDI_GITHUB_DEFAULT_BRANCH"),
		Pipeline:      a.cfg.Getenv("GITHUB_RUN_ID"),
		Job:           a.cfg.Getenv("GITHUB_RUN_ATTEMPT"),
	}
	if a.cfg.Getenv("GITHUB_EVENT_NAME") == "pull_request" {
		info.IsMergeRequest = true
		info.TargetBranch = a.cfg.Getenv("GITHUB_BASE_REF")
	}
	return info, nil
}

var _ forge.Forge = (*Adapter)(nil)
