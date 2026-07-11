// Package gitlab is the GitLab adapter for the I-22 forge port: it fetches
// verdi's own CI job's ("verdi-evidence", I-8) artifact via GitLab's
// job-artifacts REST API and reports GitLab's generated-file attribute
// token and CI context.
package gitlab

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

// defaultJobName is verdi's own CI job name (I-8: "job/workflow
// verdi-evidence uploads the derived/<ref-slug>/<commit>/ tree as its
// artifact").
const defaultJobName = "verdi-evidence"

// Config configures Adapter. BaseURL and HTTPClient are overridable so
// tests can point the adapter at an httptest server with no network
// (CLAUDE.md: "No network in any test").
type Config struct {
	// BaseURL is the GitLab API v4 root, e.g.
	// "https://gitlab.example.com/api/v4". Defaults to
	// "https://gitlab.com/api/v4".
	BaseURL string
	// ProjectID is the numeric or URL-encoded-path project id GitLab's
	// API accepts in place of :id.
	ProjectID string
	// Token authenticates API calls (a CI_JOB_TOKEN or personal access
	// token) — read from CI-provided env vars by callers, never stored in
	// verdi.yaml (01 §Store manifest: "secrets come from env/CI vars").
	Token string
	// JobName is the CI job whose artifact is fetched. Defaults to
	// "verdi-evidence".
	JobName string
	// HTTPClient defaults to http.DefaultClient.
	HTTPClient *http.Client
	// Getenv defaults to os.Getenv; overridable for hermetic CIContext tests.
	Getenv func(string) string
}

// Adapter implements forge.Forge against the GitLab API.
type Adapter struct{ cfg Config }

// New returns an Adapter with cfg's defaults filled in.
func New(cfg Config) *Adapter {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://gitlab.com/api/v4"
	}
	if cfg.JobName == "" {
		cfg.JobName = defaultJobName
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = http.DefaultClient
	}
	if cfg.Getenv == nil {
		cfg.Getenv = os.Getenv
	}
	return &Adapter{cfg: cfg}
}

type pipeline struct {
	ID     int64  `json:"id"`
	Status string `json:"status"`
}

type job struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// FetchEvidenceBundle implements forge.Forge: find the latest successful
// pipeline for commit, find its verdi-evidence job, download and unzip
// that job's artifact.
func (a *Adapter) FetchEvidenceBundle(ctx context.Context, ref, commit string) (*forge.EvidenceBundle, error) {
	pipelineID, err := a.findPipeline(ctx, commit)
	if err != nil {
		return nil, err
	}
	jobID, err := a.findJob(ctx, pipelineID)
	if err != nil {
		return nil, err
	}
	data, err := a.downloadArtifact(ctx, jobID)
	if err != nil {
		return nil, err
	}
	bundle, err := forge.ExtractBundleFromZip(data)
	if err != nil {
		return nil, fmt.Errorf("gitlab: %w", err)
	}
	return bundle, nil
}

func (a *Adapter) findPipeline(ctx context.Context, commit string) (int64, error) {
	url := fmt.Sprintf("%s/projects/%s/pipelines?sha=%s&status=success", a.cfg.BaseURL, a.cfg.ProjectID, commit)
	var pipelines []pipeline
	if err := a.getJSON(ctx, url, &pipelines); err != nil {
		return 0, err
	}
	if len(pipelines) == 0 {
		return 0, fmt.Errorf("gitlab: no successful pipeline for commit %s: %w", commit, forge.ErrNoBundle)
	}
	return pipelines[0].ID, nil
}

func (a *Adapter) findJob(ctx context.Context, pipelineID int64) (int64, error) {
	url := fmt.Sprintf("%s/projects/%s/pipelines/%d/jobs?scope=success", a.cfg.BaseURL, a.cfg.ProjectID, pipelineID)
	var jobs []job
	if err := a.getJSON(ctx, url, &jobs); err != nil {
		return 0, err
	}
	for _, j := range jobs {
		if j.Name == a.cfg.JobName {
			return j.ID, nil
		}
	}
	return 0, fmt.Errorf("gitlab: pipeline %d has no successful %q job: %w", pipelineID, a.cfg.JobName, forge.ErrNoBundle)
}

func (a *Adapter) downloadArtifact(ctx context.Context, jobID int64) ([]byte, error) {
	url := fmt.Sprintf("%s/projects/%s/jobs/%d/artifacts", a.cfg.BaseURL, a.cfg.ProjectID, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: building artifact request: %w", err)
	}
	a.setAuth(req)

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: downloading job %d artifact: %w", jobID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("gitlab: job %d has no artifact: %w", jobID, forge.ErrNoBundle)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: downloading job %d artifact: unexpected status %s", jobID, resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("gitlab: reading job %d artifact: %w", jobID, err)
	}
	return data, nil
}

func (a *Adapter) getJSON(ctx context.Context, url string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("gitlab: building request: %w", err)
	}
	a.setAuth(req)

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab: GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gitlab: GET %s: unexpected status %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("gitlab: decoding response from %s: %w", url, err)
	}
	return nil
}

func (a *Adapter) setAuth(req *http.Request) {
	if a.cfg.Token != "" {
		req.Header.Set("PRIVATE-TOKEN", a.cfg.Token)
	}
}

// mergeRequestJSON is the subset of GitLab's merge request object
// ListOpenMRs needs (GitLab API: "List merge requests").
type mergeRequestJSON struct {
	IID          int64  `json:"iid"`
	SourceBranch string `json:"source_branch"`
	Title        string `json:"title"`
}

// ListOpenMRs implements forge.Forge: GitLab's "list merge requests"
// endpoint, filtered server-side to opened MRs targeting targetBranch.
func (a *Adapter) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests?state=opened&target_branch=%s",
		a.cfg.BaseURL, a.cfg.ProjectID, url.QueryEscape(targetBranch))
	var mrs []mergeRequestJSON
	if err := a.getJSON(ctx, reqURL, &mrs); err != nil {
		return nil, err
	}
	out := make([]forge.OpenMR, len(mrs))
	for i, m := range mrs {
		out[i] = forge.OpenMR{ID: strconv.FormatInt(m.IID, 10), SourceBranch: m.SourceBranch, Title: m.Title}
	}
	return out, nil
}

// repositoryFileJSON is the subset of GitLab's "Get file from repository"
// response FetchFileAtRef needs: base64-encoded content plus its encoding
// tag (GitLab always sets "base64" today, but the field is checked rather
// than assumed).
type repositoryFileJSON struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// FetchFileAtRef implements forge.Forge against GitLab's "Get file from
// repository" endpoint (base64-encoded content).
func (a *Adapter) FetchFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/repository/files/%s?ref=%s",
		a.cfg.BaseURL, a.cfg.ProjectID, url.PathEscape(path), url.QueryEscape(ref))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("gitlab: building file request: %w", err)
	}
	a.setAuth(req)

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gitlab: GET %s: %w", reqURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("gitlab: file %q not found at ref %q: %w", path, ref, forge.ErrFileNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gitlab: GET %s: unexpected status %s", reqURL, resp.Status)
	}

	var rf repositoryFileJSON
	if err := json.NewDecoder(resp.Body).Decode(&rf); err != nil {
		return nil, fmt.Errorf("gitlab: decoding file response from %s: %w", reqURL, err)
	}
	if rf.Encoding != "" && rf.Encoding != "base64" {
		return nil, fmt.Errorf("gitlab: file %q at ref %q: unsupported encoding %q", path, ref, rf.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(rf.Content, "\n", ""))
	if err != nil {
		return nil, fmt.Errorf("gitlab: decoding base64 content for %q at ref %q: %w", path, ref, err)
	}
	return data, nil
}

// GeneratedAttribute implements forge.Forge (02 §Repository plumbing,
// VL-012).
func (a *Adapter) GeneratedAttribute() string { return "gitlab-generated" }

// CIContext implements forge.Forge, reading GitLab CI's own predefined
// variables: CI_DEFAULT_BRANCH, CI_MERGE_REQUEST_IID (presence signals an
// MR pipeline), CI_MERGE_REQUEST_TARGET_BRANCH_NAME.
func (a *Adapter) CIContext(ctx context.Context) (forge.CIInfo, error) {
	if err := ctx.Err(); err != nil {
		return forge.CIInfo{}, err
	}
	info := forge.CIInfo{
		DefaultBranch: a.cfg.Getenv("CI_DEFAULT_BRANCH"),
	}
	if mrIID := a.cfg.Getenv("CI_MERGE_REQUEST_IID"); mrIID != "" {
		info.IsMergeRequest = true
		info.TargetBranch = a.cfg.Getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")
	}
	return info, nil
}

var _ forge.Forge = (*Adapter)(nil)
