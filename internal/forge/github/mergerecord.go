package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jyang234/verdi/internal/forge"
)

// The wire types below are tolerant subsets of GitHub's published REST
// responses (ruling R-W1-8, applied to this read by plan R-PB-2): each
// declares only the members the merge-record read relies on, strictly typed,
// and every other member GitHub sends is ignored. Their shapes follow
// GitHub's OpenAPI description (github/rest-api-description) and the
// examples docs.github.com publishes from it.

// mergeRecordRepositoryJSON is the subset of "Get a repository" (GET
// /repos/{owner}/{repo}, the full-repository schema) the read needs: the
// forge's own default branch.
type mergeRecordRepositoryJSON struct {
	DefaultBranch string `json:"default_branch"`
}

// mergeRecordPullJSON is the subset of one pull-request-simple object of
// "List pull requests associated with a commit" (GET
// /repos/{owner}/{repo}/commits/{commit_sha}/pulls) the read needs. The
// simple object carries no `merged` boolean: a pull request is merged
// exactly when its state is closed and merged_at is non-null. merged_at and
// merge_commit_sha are nullable; Base is a pointer so a missing or null base
// is refused rather than read as an empty branch.
type mergeRecordPullJSON struct {
	Number         int64   `json:"number"`
	State          string  `json:"state"`
	MergedAt       *string `json:"merged_at"`
	MergeCommitSHA *string `json:"merge_commit_sha"`
	Base           *struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

// MergeRecords implements forge.Forge (SI-249; plan R-PB-2): the pull
// requests GitHub associates with commit, each with its state, base branch,
// merge commit, and merge time, plus the repository's default branch. It
// reads the repository first, then drains the commit's pull requests.
//
// commit must be a full lowercase SHA; anything else is refused before any
// request. Transport failures, rate limiting, and unexpected statuses wrap
// forge.ErrUnavailable as every other approval-domain read does, except a
// 404 on the repository read, which means the configured repository does not
// exist or the token cannot see it: a configuration defect, reported as an
// operational error. A JSON or schema violation, an unknown state, or a
// missing id is an operational error that does not wrap ErrUnavailable.
func (a *Adapter) MergeRecords(ctx context.Context, commit string) (forge.MergeRecordFacts, error) {
	if err := forge.ValidateMergeRecordCommit(commit); err != nil {
		return forge.MergeRecordFacts{}, fmt.Errorf("github: merge records: %w", err)
	}
	repoURL := fmt.Sprintf("%s/repos/%s/%s", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo)

	var repo mergeRecordRepositoryJSON
	if err := a.readMergeRecordRepository(ctx, repoURL, &repo); err != nil {
		return forge.MergeRecordFacts{}, fmt.Errorf("github: merge records: reading repository %s/%s: %w", a.cfg.Owner, a.cfg.Repo, err)
	}
	if repo.DefaultBranch == "" {
		return forge.MergeRecordFacts{}, fmt.Errorf("github: merge records: repository %s/%s reports no default_branch", a.cfg.Owner, a.cfg.Repo)
	}

	pulls, err := githubDrainStrictList(ctx, a, repoURL+"/commits/"+commit+"/pulls", func(page []mergeRecordPullJSON) ([]mergeRecordPullJSON, error) {
		if page == nil {
			return nil, fmt.Errorf("github: pull requests page must be a non-null array")
		}
		return page, nil
	})
	if err != nil {
		return forge.MergeRecordFacts{}, fmt.Errorf("github: merge records: reading pull requests associated with commit %s: %w", commit, err)
	}

	changes := make([]forge.ChangeRequestMerge, 0, len(pulls))
	for i, pull := range pulls {
		change, err := pullChangeRequestMerge(pull)
		if err != nil {
			return forge.MergeRecordFacts{}, fmt.Errorf("github: merge records: pull request entry %d for commit %s: %w", i, commit, err)
		}
		changes = append(changes, change)
	}

	facts, err := forge.NewMergeRecordFacts(forge.MergeRecordFacts{
		Supported:     true,
		Repository:    a.cfg.Owner + "/" + a.cfg.Repo,
		Commit:        commit,
		DefaultBranch: repo.DefaultBranch,
		Changes:       changes,
	}, a.cfg.Clock())
	if err != nil {
		return forge.MergeRecordFacts{}, fmt.Errorf("github: merge records: normalize facts: %w", err)
	}
	return facts, nil
}

// pullChangeRequestMerge maps one pull-request-simple object. merge_commit_sha
// is honored only for a merged pull request: an open pull request's
// merge_commit_sha is GitHub's test merge, and a closed unmerged one's is a
// stale test merge, and neither is ever a merge commit. A state outside
// {open, closed} fails closed.
func pullChangeRequestMerge(pull mergeRecordPullJSON) (forge.ChangeRequestMerge, error) {
	if pull.Number <= 0 {
		return forge.ChangeRequestMerge{}, fmt.Errorf("pull request carries no positive number")
	}
	id := strconv.FormatInt(pull.Number, 10)
	if pull.Base == nil {
		return forge.ChangeRequestMerge{}, fmt.Errorf("pull request %s carries no base", id)
	}
	change := forge.ChangeRequestMerge{ChangeID: id, TargetBranch: pull.Base.Ref}
	switch {
	case pull.State == "open":
		change.State = forge.ChangeRequestOpen
	case pull.State == "closed" && pull.MergedAt == nil:
		change.State = forge.ChangeRequestClosed
	case pull.State == "closed":
		change.State = forge.ChangeRequestMerged
		stamp, err := forge.NormalizeTimestamp(*pull.MergedAt)
		if err != nil {
			return forge.ChangeRequestMerge{}, fmt.Errorf("pull request %s merged_at: %w", id, err)
		}
		change.MergedAt = stamp
		if pull.MergeCommitSHA != nil {
			change.MergeCommitSHA = *pull.MergeCommitSHA
		}
	default:
		return forge.ChangeRequestMerge{}, fmt.Errorf("pull request %s: unknown state %q", id, pull.State)
	}
	return change, nil
}

// readMergeRecordRepository reads "Get a repository" through the approval
// decode seam's rules (DecodeApprovalJSON: tolerant subset, trailing data
// rejected) and getApprovalJSON's status classification, except that a 404
// is an operational error rather than unavailability (MergeRecords' doc).
func (a *Adapter) readMergeRecordRepository(ctx context.Context, requestURL string, out any) (err error) {
	response, err := a.transport.RawDo(ctx, http.MethodGet, requestURL, nil, a.setAuth)
	if err != nil {
		return fmt.Errorf("%w: github: GET %s: %w", forge.ErrUnavailable, requestURL, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			// vocab:identity — HTTP response-body operation, not a Verdi lifecycle class.
			err = errors.Join(err, fmt.Errorf("github: close GET %s response: %w", requestURL, closeErr))
		}
	}()
	if response.StatusCode == http.StatusNotFound {
		return fmt.Errorf("github: GET %s: status %s: the configured repository does not exist or the token cannot read it", requestURL, response.Status)
	}
	if err := a.classify(http.MethodGet, requestURL, http.StatusOK)(response, nil); err != nil {
		return err
	}
	if err := forge.DecodeApprovalJSON(response.Body, out); err != nil {
		return fmt.Errorf("github: GET %s: %w", requestURL, err)
	}
	return nil
}
