package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jyang234/verdi/internal/forge"
)

// The wire types below are tolerant subsets of GitLab's documented REST
// responses (ruling R-W1-8, applied to this read by plan R-PB-2): each
// declares only the members the merge-record read relies on, strictly typed,
// and every other member GitLab sends is ignored.
//
// DOC-DERIVED, UNVERIFIED AGAINST LIVE, like this package's comment methods:
// the shapes follow gitlab-org/gitlab doc/api (projects.md "Retrieve a
// project", commits.md "List merge requests associated with a commit", and
// merge_requests.md for the merge request attributes that commits.md's
// example omits, such as merged_at). No live GitLab response was read.

// mergeRecordProjectJSON is the subset of "Retrieve a project" (GET
// /projects/:id) the read needs: the project's stable numeric id and its
// default branch (null for a project with no repository content).
type mergeRecordProjectJSON struct {
	ID            int64  `json:"id"`
	DefaultBranch string `json:"default_branch"`
}

// mergeRecordMergeRequestJSON is the subset of one merge request of "List
// merge requests associated with a commit" (GET
// /projects/:id/repository/commits/:sha/merge_requests) the read needs.
// merge_commit_sha and merged_at are nullable. target_project_id must be the
// queried project's id. squash_commit_sha is deliberately not modeled: a
// squash commit is never a merge commit.
type mergeRecordMergeRequestJSON struct {
	IID             int64   `json:"iid"`
	State           string  `json:"state"`
	TargetBranch    string  `json:"target_branch"`
	TargetProjectID int64   `json:"target_project_id"`
	MergeCommitSHA  *string `json:"merge_commit_sha"`
	MergedAt        *string `json:"merged_at"`
}

// MergeRecords implements forge.Forge (SI-249; plan R-PB-2): the merge
// requests GitLab associates with commit, each with the state, target
// branch, merge commit, and merge time GitLab reports for it, plus the
// project's default branch. It reads the project first, then drains the
// commit's merge requests. A reported merge is not proof that GitLab created
// the merge commit (mergeRequestChangeRequestMerge's doc; lane EF review F1).
//
// commit must be a full lowercase SHA; anything else is refused before any
// request. Transport failures, rate limiting, and unexpected statuses wrap
// forge.ErrUnavailable as every other approval-domain read does, except a
// 404 on the project read, which means the configured project does not
// exist or the token cannot see it: a configuration defect, reported as an
// operational error. A JSON or schema violation, an unknown state, a missing
// id, or a merge request that targets another project is an operational
// error that does not wrap ErrUnavailable.
func (a *Adapter) MergeRecords(ctx context.Context, commit string) (forge.MergeRecordFacts, error) {
	if err := forge.ValidateMergeRecordCommit(commit); err != nil {
		// vocab:identity — forge merge-record diagnostic: a provider's merge of a change request, not a Verdi lifecycle verb.
		return forge.MergeRecordFacts{}, fmt.Errorf("gitlab: merge records: %w", err)
	}
	projectURL := fmt.Sprintf("%s/projects/%s", a.cfg.BaseURL, url.PathEscape(a.cfg.ProjectID))

	var project mergeRecordProjectJSON
	if err := a.readMergeRecordProject(ctx, projectURL, &project); err != nil {
		// vocab:identity — forge merge-record diagnostic: a provider's merge of a change request, not a Verdi lifecycle verb.
		return forge.MergeRecordFacts{}, fmt.Errorf("gitlab: merge records: reading project %s: %w", a.cfg.ProjectID, err)
	}
	if project.ID <= 0 {
		// vocab:identity — forge merge-record diagnostic: a provider's merge of a change request, not a Verdi lifecycle verb.
		return forge.MergeRecordFacts{}, fmt.Errorf("gitlab: merge records: project %s carries no stable id", a.cfg.ProjectID)
	}
	if project.DefaultBranch == "" {
		// vocab:identity — forge merge-record diagnostic: a provider's merge of a change request, not a Verdi lifecycle verb.
		return forge.MergeRecordFacts{}, fmt.Errorf("gitlab: merge records: project %s reports no default_branch", a.cfg.ProjectID)
	}

	mergeRequests, err := gitlabDrainStrictList[mergeRecordMergeRequestJSON](ctx, a, projectURL+"/repository/commits/"+commit+"/merge_requests")
	if err != nil {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.MergeRecordFacts{}, fmt.Errorf("gitlab: merge records: reading merge requests associated with commit %s: %w", commit, err)
	}

	changes := make([]forge.ChangeRequestMerge, 0, len(mergeRequests))
	for i, mergeRequest := range mergeRequests {
		change, err := mergeRequestChangeRequestMerge(mergeRequest, project.ID)
		if err != nil {
			// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
			return forge.MergeRecordFacts{}, fmt.Errorf("gitlab: merge records: merge request entry %d for commit %s: %w", i, commit, err)
		}
		changes = append(changes, change)
	}

	facts, err := forge.NewMergeRecordFacts(forge.MergeRecordFacts{
		Supported:     true,
		Repository:    strconv.FormatInt(project.ID, 10),
		Commit:        commit,
		DefaultBranch: project.DefaultBranch,
		Changes:       changes,
	}, a.cfg.Clock())
	if err != nil {
		// vocab:identity — forge merge-record diagnostic: a provider's merge of a change request, not a Verdi lifecycle verb.
		return forge.MergeRecordFacts{}, fmt.Errorf("gitlab: merge records: normalize facts: %w", err)
	}
	return facts, nil
}

// mergeRequestChangeRequestMerge maps one merge request. GitLab's `locked`
// state is the transient state of a merge in progress: a merge in progress
// is not a merge, so it maps to open. Any state outside opened, closed,
// merged, and locked fails closed. merge_commit_sha and merged_at are honored
// only when merged; a fast-forward merge reports a null merge_commit_sha, so
// its facts carry no merge commit and it stays unproven.
//
// GitLab's merged state means only that GitLab reports the merge request
// merged. It also covers a push-detected merge — GitLab marks a merge
// request merged when a push to its target branch contains its commits and,
// per lane EF review F1, records the pushed commit as merge_commit_sha — and
// no member of this response tells that from a merge GitLab performed. So a
// reported merge_commit_sha does NOT by itself prove GitLab created the
// commit (see the pending owner ruling on SI-249 evidence, lane EF review
// F1).
//
// The merge request's target_project_id must be the queried project
// (projectID, from "Retrieve a project"), mirroring the GitHub adapter's
// base-repository check (lane EF review F2): a merge into another project's
// default branch is not a merge into this one's. A missing target_project_id
// or a different one is a provider contract violation — an operational
// error, never unavailability and never a merge request silently dropped.
func mergeRequestChangeRequestMerge(mergeRequest mergeRecordMergeRequestJSON, projectID int64) (forge.ChangeRequestMerge, error) {
	if mergeRequest.IID <= 0 {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ChangeRequestMerge{}, fmt.Errorf("merge request carries no positive iid")
	}
	id := strconv.FormatInt(mergeRequest.IID, 10)
	if mergeRequest.TargetProjectID <= 0 {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ChangeRequestMerge{}, fmt.Errorf("merge request %s carries no positive target_project_id", id)
	}
	if mergeRequest.TargetProjectID != projectID {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ChangeRequestMerge{}, fmt.Errorf("merge request %s targets project id %d, not the queried project id %d: a provider contract violation",
			id, mergeRequest.TargetProjectID, projectID)
	}
	change := forge.ChangeRequestMerge{ChangeID: id, TargetBranch: mergeRequest.TargetBranch}
	switch mergeRequest.State {
	case "opened", "locked":
		change.State = forge.ChangeRequestOpen
	case "closed":
		change.State = forge.ChangeRequestClosed
	case "merged":
		change.State = forge.ChangeRequestMerged
		if mergeRequest.MergedAt == nil {
			// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
			return forge.ChangeRequestMerge{}, fmt.Errorf("merged merge request %s carries no merged_at", id)
		}
		stamp, err := forge.NormalizeTimestamp(*mergeRequest.MergedAt)
		if err != nil {
			// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
			return forge.ChangeRequestMerge{}, fmt.Errorf("merge request %s merged_at: %w", id, err)
		}
		change.MergedAt = stamp
		if mergeRequest.MergeCommitSHA != nil {
			change.MergeCommitSHA = *mergeRequest.MergeCommitSHA
		}
	default:
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ChangeRequestMerge{}, fmt.Errorf("merge request %s: unknown state %q", id, mergeRequest.State)
	}
	return change, nil
}

// readMergeRecordProject reads "Retrieve a project" through the approval
// decode seam's rules (DecodeApprovalJSON: tolerant subset, trailing data
// rejected) and getApprovalJSON's status classification, except that a 404
// is an operational error rather than unavailability (MergeRecords' doc).
func (a *Adapter) readMergeRecordProject(ctx context.Context, requestURL string, out any) (err error) {
	response, err := a.transport.RawDo(ctx, http.MethodGet, requestURL, nil, a.setAuth)
	if err != nil {
		return fmt.Errorf("%w: gitlab: GET %s: %w", forge.ErrUnavailable, requestURL, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			// vocab:identity — HTTP response-body operation, not a Verdi lifecycle class.
			err = errors.Join(err, fmt.Errorf("gitlab: close GET %s response: %w", requestURL, closeErr))
		}
	}()
	if response.StatusCode == http.StatusNotFound {
		return fmt.Errorf("gitlab: GET %s: status %s: the configured project does not exist or the token cannot read it", requestURL, response.Status)
	}
	if err := a.classify(http.MethodGet, requestURL, http.StatusOK)(response, nil); err != nil {
		return err
	}
	if err := forge.DecodeApprovalJSON(response.Body, out); err != nil {
		return fmt.Errorf("gitlab: GET %s: %w", requestURL, err)
	}
	return nil
}

// gitlabDrainStrictList mirrors gitlabDrainList's X-Next-Page walk
// (per_page=100 plus page=N), but rides the approval decode seam
// (getApprovalJSON/forge.DecodeApprovalJSON: a tolerant subset that still
// rejects trailing data, where httpjson ignores it) and fails closed where
// gitlabDrainList stops quietly: a null page, a malformed X-Next-Page, and an
// X-Next-Page naming a page already read are rejected (co-1: ambiguous
// pagination is rejected), mirroring github's githubDrainStrictList.
func gitlabDrainStrictList[I any](ctx context.Context, a *Adapter, firstURL string) ([]I, error) {
	var all []I
	visited := make(map[int]struct{})
	for page := 1; ; {
		if _, exists := visited[page]; exists {
			return nil, fmt.Errorf("gitlab: pagination cycle detected: X-Next-Page names page %d again", page)
		}
		visited[page] = struct{}{}
		current := withPageQuery(firstURL, page)
		var items []I
		headers, err := a.getApprovalJSON(ctx, current, &items)
		if err != nil {
			return nil, err
		}
		if items == nil {
			return nil, fmt.Errorf("gitlab: GET %s: page must be a non-null array", current)
		}
		all = append(all, items...)

		next := headers.Get(nextPageHeader)
		if next == "" {
			return all, nil
		}
		nextPage, err := strconv.Atoi(next)
		if err != nil || nextPage < 1 || strconv.Itoa(nextPage) != next {
			return nil, fmt.Errorf("gitlab: GET %s: malformed %s %q", current, nextPageHeader, next)
		}
		page = nextPage
	}
}
