// Package github is the GitHub adapter for the I-22 forge port: it fetches
// verdi's own CI workflow's ("verdi-evidence", I-8) artifact via GitHub's
// Actions artifacts REST API and reports GitHub's generated-file attribute
// token and CI context.
package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/OWNER/verdi/internal/forge"
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

// FetchEvidenceBundle implements forge.Forge: find the latest successful
// workflow run for commit, find its verdi-evidence artifact, download and
// unzip it.
func (a *Adapter) FetchEvidenceBundle(ctx context.Context, ref, commit string) (*forge.EvidenceBundle, error) {
	runID, err := a.findRun(ctx, commit)
	if err != nil {
		return nil, err
	}
	artifactID, err := a.findArtifact(ctx, runID)
	if err != nil {
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

func (a *Adapter) findRun(ctx context.Context, commit string) (int64, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?head_sha=%s&status=success", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, commit)
	var resp runsResponse
	if err := a.getJSON(ctx, url, &resp); err != nil {
		return 0, err
	}
	for _, r := range resp.WorkflowRuns {
		if r.Conclusion == "success" {
			return r.ID, nil
		}
	}
	return 0, fmt.Errorf("github: no successful workflow run for commit %s: %w", commit, forge.ErrNoBundle)
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
func (a *Adapter) CIContext(ctx context.Context) (forge.CIInfo, error) {
	if err := ctx.Err(); err != nil {
		return forge.CIInfo{}, err
	}
	info := forge.CIInfo{
		DefaultBranch: a.cfg.Getenv("VERDI_GITHUB_DEFAULT_BRANCH"),
	}
	if a.cfg.Getenv("GITHUB_EVENT_NAME") == "pull_request" {
		info.IsMergeRequest = true
		info.TargetBranch = a.cfg.Getenv("GITHUB_BASE_REF")
	}
	return info, nil
}

var _ forge.Forge = (*Adapter)(nil)
