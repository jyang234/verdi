// Package gitlab is the GitLab adapter for the I-22 forge port: it fetches
// verdi's own CI job's ("verdi-evidence", I-8) artifact via GitLab's
// job-artifacts REST API and reports GitLab's generated-file attribute
// token and CI context.
//
// V1-P7's comment-round-trip methods (ListComments, PostComment,
// GetThreadResolution) are DOC-DERIVED, UNVERIFIED AGAINST LIVE — carried
// forward verbatim from V1-P0's spike S6
// (docs/spikes/v1/spike-s6-findings.md): no GitLab credentials were
// available in the build environment, so every JSON shape below was
// assembled from https://docs.gitlab.com/ee/api/discussions.html, never
// exercised against a real GitLab server (GitHub's equivalent methods
// WERE live-verified; see internal/forge/github). The contract suite
// (internal/forge/forgetest) proves this adapter matches the documented
// shape; it does not prove it matches a live GitLab API. Re-verify against
// a live instance before trusting this adapter in production — a
// disclosed residual, not a silent one (constitution 2/10).
package gitlab

import (
	"bytes"
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

// postJSON mirrors getJSON for the write direction: encode body as the
// JSON request payload, decode the response into out.
func (a *Adapter) postJSON(ctx context.Context, url string, body, out interface{}) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("gitlab: encoding request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("gitlab: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	a.setAuth(req)

	resp, err := a.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("gitlab: POST %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("gitlab: POST %s: unexpected status %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("gitlab: decoding response from %s: %w", url, err)
	}
	return nil
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

// notePositionJSON is a GitLab DiffNote's diff-anchor position (S6 capture
// `gitlab/01-doc-derived-UNVERIFIED-list-discussions.json`, DOC-DERIVED —
// see the package doc note below). NewLine is a pointer since GitLab's own
// docs (per the capture's `_field_notes`) do not document whether/how it
// gets nulled on a rewritten line.
type notePositionJSON struct {
	NewPath      string `json:"new_path"`
	NewLine      *int   `json:"new_line"`
	BaseSHA      string `json:"base_sha"`
	StartSHA     string `json:"start_sha"`
	HeadSHA      string `json:"head_sha"`
	PositionType string `json:"position_type"`
}

// noteJSON is one GitLab discussion Note (S6 capture, DOC-DERIVED,
// UNVERIFIED against a live GitLab instance — see this file's package doc
// note): Resolvable/Resolved/ResolvedBy are documented as plain fields on
// the Note itself, no separate GraphQL leg the way GitHub requires (S6 Q2)
// — that asymmetry is itself unverified, disclosed forward rather than
// assumed.
type noteJSON struct {
	ID     int64  `json:"id"`
	Body   string `json:"body"`
	Author struct {
		Username string `json:"username"`
	} `json:"author"`
	CreatedAt  string `json:"created_at"`
	Resolvable bool   `json:"resolvable"`
	Resolved   bool   `json:"resolved"`
	ResolvedBy *struct {
		Username string `json:"username"`
	} `json:"resolved_by"`
	Position *notePositionJSON `json:"position,omitempty"`
}

// discussionJSON is one GitLab Discussion — a wrapper around one or more
// Notes (S6 capture, DOC-DERIVED). IndividualNote true marks a bare,
// non-resolvable comment (S6's "two comment universes": GitLab's
// individual_note:true is the direct analogue of GitHub's separate
// issues/comments universe).
type discussionJSON struct {
	ID             string     `json:"id"`
	IndividualNote bool       `json:"individual_note"`
	Notes          []noteJSON `json:"notes"`
}

// listDiscussions is the one GET both ListComments and GetThreadResolution
// read from — GitLab's docs show resolution state living directly on the
// same discussions listing response (S6 capture
// `gitlab/03-doc-derived-UNVERIFIED-resolve-discussion-response.json`),
// unlike GitHub's separate GraphQL query.
//
// DOC-DERIVED, UNVERIFIED AGAINST LIVE (S6 disclosure, carried forward
// here verbatim): no GitLab credentials were available in the spike
// environment, so every shape this method decodes was assembled from
// https://docs.gitlab.com/ee/api/discussions.html, never exercised against
// a real GitLab server. The contract suite proves this adapter matches
// the DOCUMENTED shape; it does NOT prove the adapter matches a LIVE
// GitLab API. Re-verify against a live instance before trusting this
// adapter in production (S6 findings.md, PLAN-V1.md §5 V1-P7 spike
// findings block).
func (a *Adapter) listDiscussions(ctx context.Context, mrID string) ([]discussionJSON, error) {
	reqURL := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions", a.cfg.BaseURL, a.cfg.ProjectID, mrID)
	var discussions []discussionJSON
	if err := a.getJSON(ctx, reqURL, &discussions); err != nil {
		return nil, err
	}
	return discussions, nil
}

// ListComments implements forge.Forge against GitLab's discussions listing
// (DOC-DERIVED, UNVERIFIED — see listDiscussions). Merges both comment
// universes: a resolvable DiffNote's ThreadID is its owning discussion's
// id; an individual_note:true note carries ThreadID "".
func (a *Adapter) ListComments(ctx context.Context, mrID string) ([]forge.Comment, error) {
	discussions, err := a.listDiscussions(ctx, mrID)
	if err != nil {
		return nil, err
	}
	var out []forge.Comment
	for _, d := range discussions {
		threadID := ""
		if !d.IndividualNote {
			threadID = d.ID
		}
		for _, n := range d.Notes {
			c := forge.Comment{
				ID: strconv.FormatInt(n.ID, 10), ThreadID: threadID, Body: n.Body,
				Author: n.Author.Username, CreatedAt: n.CreatedAt,
			}
			if n.Position != nil {
				c.Path = n.Position.NewPath
				if n.Position.NewLine != nil {
					c.Line = *n.Position.NewLine
				}
			}
			out = append(out, c)
		}
	}
	return out, nil
}

// GetThreadResolution implements forge.Forge against GitLab's discussions
// listing (DOC-DERIVED, UNVERIFIED — see listDiscussions). Only discussions
// GitLab's own docs mark resolvable (individual_note:false AND the
// representative note's resolvable:true) become a substantive thread here
// — a plain individual_note carries no resolution concept at all, per
// docs, mirroring GitHub's reviewThreads-only population.
func (a *Adapter) GetThreadResolution(ctx context.Context, mrID string) ([]forge.ThreadResolution, error) {
	discussions, err := a.listDiscussions(ctx, mrID)
	if err != nil {
		return nil, err
	}
	var out []forge.ThreadResolution
	for _, d := range discussions {
		if d.IndividualNote || len(d.Notes) == 0 || !d.Notes[0].Resolvable {
			continue
		}
		tr := forge.ThreadResolution{ThreadID: d.ID, Resolved: d.Notes[0].Resolved}
		if d.Notes[0].ResolvedBy != nil {
			tr.ResolvedBy = d.Notes[0].ResolvedBy.Username
		}
		out = append(out, tr)
	}
	return out, nil
}

// mrDiffRefsJSON is the subset of GitLab's merge request object
// PostComment needs: the three shas a diff-anchored note's position
// requires (S6 Q1: "the caller must first fetch the MR's diff_refs...
// before posting, a heavier precondition than GitHub's single commit_id").
type mrDiffRefsJSON struct {
	DiffRefs struct {
		BaseSHA  string `json:"base_sha"`
		StartSHA string `json:"start_sha"`
		HeadSHA  string `json:"head_sha"`
	} `json:"diff_refs"`
}

type createDiscussionPositionRequest struct {
	PositionType string `json:"position_type"`
	BaseSHA      string `json:"base_sha"`
	StartSHA     string `json:"start_sha"`
	HeadSHA      string `json:"head_sha"`
	OldPath      string `json:"old_path"`
	NewPath      string `json:"new_path"`
	NewLine      int    `json:"new_line"`
}

type createDiscussionRequest struct {
	Body     string                           `json:"body"`
	Position *createDiscussionPositionRequest `json:"position,omitempty"`
}

// PostComment implements forge.Forge against GitLab's create-discussion
// endpoint (DOC-DERIVED, UNVERIFIED — S6 capture
// `gitlab/02-doc-derived-UNVERIFIED-post-discussion-request.json`): a
// diff-anchored comment (target != nil) pre-fetches diff_refs and posts a
// full position hash; a general comment (target == nil) posts body alone.
func (a *Adapter) PostComment(ctx context.Context, mrID, body string, target *forge.CommentTarget) (forge.Comment, error) {
	reqBody := createDiscussionRequest{Body: body}
	if target != nil {
		var mr mrDiffRefsJSON
		mrURL := fmt.Sprintf("%s/projects/%s/merge_requests/%s", a.cfg.BaseURL, a.cfg.ProjectID, mrID)
		if err := a.getJSON(ctx, mrURL, &mr); err != nil {
			return forge.Comment{}, fmt.Errorf("gitlab: resolving MR diff_refs to post a diff comment: %w", err)
		}
		reqBody.Position = &createDiscussionPositionRequest{
			PositionType: "text", BaseSHA: mr.DiffRefs.BaseSHA, StartSHA: mr.DiffRefs.StartSHA, HeadSHA: mr.DiffRefs.HeadSHA,
			OldPath: target.Path, NewPath: target.Path, NewLine: target.Line,
		}
	}

	var created discussionJSON
	postURL := fmt.Sprintf("%s/projects/%s/merge_requests/%s/discussions", a.cfg.BaseURL, a.cfg.ProjectID, mrID)
	if err := a.postJSON(ctx, postURL, reqBody, &created); err != nil {
		return forge.Comment{}, err
	}
	if len(created.Notes) == 0 {
		return forge.Comment{}, fmt.Errorf("gitlab: POST %s: response discussion carries no notes", postURL)
	}
	n := created.Notes[0]
	threadID := ""
	if !created.IndividualNote {
		threadID = created.ID
	}
	c := forge.Comment{ID: strconv.FormatInt(n.ID, 10), ThreadID: threadID, Body: n.Body, Author: n.Author.Username, CreatedAt: n.CreatedAt}
	if target != nil {
		c.Path = target.Path
		c.Line = target.Line
	}
	return c, nil
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
