// Package github is the GitHub adapter for the I-22 forge port: it fetches
// verdi's own CI workflow's ("verdi-evidence", I-8) artifact via GitHub's
// Actions artifacts REST API and reports GitHub's generated-file attribute
// token and CI context.
package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/forge"
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
	// HTTPClient defaults to http.DefaultClient.
	HTTPClient *http.Client
	// Getenv defaults to os.Getenv; overridable for hermetic CIContext tests.
	Getenv func(string) string
}

// Adapter implements forge.Forge against the GitHub REST API.
type Adapter struct{ cfg Config }

// New returns an Adapter with cfg's defaults filled in.
func New(cfg Config) *Adapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.github.com"
	}
	if cfg.ArtifactName == "" {
		cfg.ArtifactName = defaultArtifactName
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.Getenv == nil {
		cfg.Getenv = os.Getenv
	}
	return &Adapter{cfg: cfg}
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
// workflow FILE (unlike GitLab, where one pipeline covers every job), so
// once this repo runs both verify.yml and verdi-evidence.yml on the same
// push/PR (spec/remote-and-ci), the head_sha query below returns both.
// This tries every successful run for commit, in the order the API
// returns them, until one actually carries the wanted artifact — it never
// assumes the first successful run is the verdi-evidence one.
func (a *Adapter) FetchEvidenceBundle(ctx context.Context, ref, commit string) (*forge.EvidenceBundle, error) {
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
		bundle, err := forge.ExtractBundleFromZip(data)
		if err != nil {
			return nil, fmt.Errorf("github: %w", err)
		}
		return bundle, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("github: no successful workflow run for commit %s: %w", commit, forge.ErrNoBundle)
	}
	return nil, lastErr
}

// findRuns returns every successful workflow run's id for commit, in the
// order GitHub's API lists them.
func (a *Adapter) findRuns(ctx context.Context, commit string) ([]int64, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?head_sha=%s&status=success", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, commit)
	var resp runsResponse
	if err := a.getJSON(ctx, url, &resp); err != nil {
		return nil, err
	}
	var ids []int64
	for _, r := range resp.WorkflowRuns {
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
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/artifacts", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, runID)
	var resp artifactsResponse
	if err := a.getJSON(ctx, url, &resp); err != nil {
		return 0, err
	}
	for _, art := range resp.Artifacts {
		if art.Name == a.cfg.ArtifactName {
			return art.ID, nil
		}
	}
	return 0, fmt.Errorf("github: run %d has no %q artifact: %w", runID, a.cfg.ArtifactName, forge.ErrNoBundle)
}

func (a *Adapter) downloadArtifact(ctx context.Context, artifactID int64) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/artifacts/%d/zip", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, artifactID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("github: building artifact request: %w", err)
	}
	a.setAuth(req)

	resp, err := a.cfg.HTTPClient.Do(req)
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

func (a *Adapter) getJSON(ctx context.Context, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("github: building request: %w", err)
	}
	a.setAuth(req)

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github: GET %s: unexpected status %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("github: decoding response from %s: %w", url, err)
	}
	return nil
}

func (a *Adapter) setAuth(req *http.Request) {
	if a.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	}
}

// postJSON mirrors getJSON for the write direction: encode body as the
// JSON request payload, decode the response into out.
func (a *Adapter) postJSON(ctx context.Context, url string, body, out interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("github: encoding request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("github: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	a.setAuth(req)

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("github: POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("github: POST %s: unexpected status %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("github: decoding response from %s: %w", url, err)
	}
	return nil
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
// endpoint, filtered server-side to open PRs based on targetBranch.
func (a *Adapter) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	reqURL := fmt.Sprintf("%s/repos/%s/%s/pulls?state=open&base=%s",
		a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, url.QueryEscape(targetBranch))
	var prs []pullRequestJSON
	if err := a.getJSON(ctx, reqURL, &prs); err != nil {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("github: building file request: %w", err)
	}
	a.setAuth(req)

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github: GET %s: %w", reqURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("github: file %q not found at ref %q: %w", path, ref, forge.ErrFileNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github: GET %s: unexpected status %s", reqURL, resp.Status)
	}

	var rc repoContentJSON
	if err := json.NewDecoder(resp.Body).Decode(&rc); err != nil {
		return nil, fmt.Errorf("github: decoding content response from %s: %w", reqURL, err)
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
// only for the thread-id/resolution join.
const reviewThreadsQuery = `query($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviewThreads(first: 100) {
        nodes {
          id
          isResolved
          resolvedBy { login }
          comments(first: 100) {
            nodes { databaseId }
          }
        }
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

type reviewThreadNode struct {
	ID         string `json:"id"`
	IsResolved bool   `json:"isResolved"`
	ResolvedBy *struct {
		Login string `json:"login"`
	} `json:"resolvedBy"`
	Comments struct {
		Nodes []struct {
			DatabaseID int64 `json:"databaseId"`
		} `json:"nodes"`
	} `json:"comments"`
}

type reviewThreadsResponse struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				ReviewThreads struct {
					Nodes []reviewThreadNode `json:"nodes"`
				} `json:"reviewThreads"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
	Errors []graphQLError `json:"errors"`
}

// fetchReviewThreads runs reviewThreadsQuery against GitHub's GraphQL v4
// endpoint (BaseURL + "/graphql" — the real API serves GraphQL on the same
// host as REST, at a fixed sibling path, so no separate config knob is
// needed; httptest doubles serve it from the same mux). Shared by
// ListComments (thread-id grouping only) and GetThreadResolution (full
// resolution state) — one query, two consumers, no duplicated transport
// code (CLAUDE.md).
func (a *Adapter) fetchReviewThreads(ctx context.Context, mrID string) ([]reviewThreadNode, error) {
	number, err := strconv.Atoi(mrID)
	if err != nil {
		return nil, fmt.Errorf("github: mrID %q is not a PR number: %w", mrID, err)
	}
	reqBody := graphQLRequest{
		Query:     reviewThreadsQuery,
		Variables: map[string]any{"owner": a.cfg.Owner, "repo": a.cfg.Repo, "number": number},
	}
	var parsed reviewThreadsResponse
	if err := a.postJSON(ctx, a.cfg.BaseURL+"/graphql", reqBody, &parsed); err != nil {
		return nil, fmt.Errorf("github: GraphQL reviewThreads query: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("github: GraphQL reviewThreads query failed: %s", parsed.Errors[0].Message)
	}
	return parsed.Data.Repository.PullRequest.ReviewThreads.Nodes, nil
}

// ListComments implements forge.Forge: merges GitHub's two comment
// universes (S6 finding) — diff-anchored REST `pulls/{mrID}/comments` and
// general REST `issues/{mrID}/comments` — into one feed, joining each diff
// comment to its GraphQL thread id (fetchReviewThreads) so
// GetThreadResolution's entries can be matched back to it. General
// comments carry no thread id at all (ThreadID stays "") — GitHub's model
// has no resolution concept for them.
func (a *Adapter) ListComments(ctx context.Context, mrID string) ([]forge.Comment, error) {
	var diff []reviewCommentJSON
	diffURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%s/comments", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, mrID)
	if err := a.getJSON(ctx, diffURL, &diff); err != nil {
		return nil, err
	}
	var general []issueCommentJSON
	generalURL := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, mrID)
	if err := a.getJSON(ctx, generalURL, &general); err != nil {
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
