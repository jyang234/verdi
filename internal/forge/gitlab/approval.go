package gitlab

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/forge"
)

type approvalMergeRequestJSON struct {
	SHA       string `json:"sha"`
	ProjectID int64  `json:"project_id"`
	Author    struct {
		ID int64 `json:"id"`
	} `json:"author"`
}

type approvalsJSON struct {
	ApprovedBy []approvedByJSON `json:"approved_by"`
}

type approvedByJSON struct {
	User struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
	} `json:"user"`
	ApprovedAt string `json:"approved_at"`
}

// ListApprovals implements forge.Forge using GitLab's current active approver
// resource. Removed approvers are absent; this adapter invents no revocation.
func (a *Adapter) ListApprovals(ctx context.Context, changeID string) (forge.ApprovalSnapshot, error) {
	if changeID == "" {
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: change id is empty")
	}
	project := url.PathEscape(a.cfg.ProjectID)
	change := url.PathEscape(changeID)
	mergeRequestURL := fmt.Sprintf("%s/projects/%s/merge_requests/%s", a.cfg.BaseURL, project, change)
	var mergeRequest approvalMergeRequestJSON
	if _, err := a.getApprovalJSON(ctx, mergeRequestURL, &mergeRequest); err != nil {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: reading merge request %s head: %w", changeID, err)
	}
	if mergeRequest.ProjectID <= 0 {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: merge request %s carries no stable project id", changeID)
	}
	stableRepository := strconv.FormatInt(mergeRequest.ProjectID, 10)

	approvalsURL := mergeRequestURL + "/approvals"
	var response approvalsJSON
	headers, err := a.getApprovalJSON(ctx, approvalsURL, &response)
	if err != nil {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: reading merge request %s approvals: %w", changeID, err)
	}
	if headers.Get(nextPageHeader) != "" || headers.Get("Link") != "" {
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: approvals resource claimed an ambiguous pagination continuation")
	}
	if response.ApprovedBy == nil {
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: approvals response carries null or missing approved_by")
	}
	var currentMergeRequest approvalMergeRequestJSON
	if _, err := a.getApprovalJSON(ctx, mergeRequestURL, &currentMergeRequest); err != nil {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: rereading merge request %s head after approval collection: %w", changeID, err)
	}
	if currentMergeRequest.ProjectID <= 0 {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: merge request %s carries no stable project id after approval collection", changeID)
	}
	if currentMergeRequest.ProjectID != mergeRequest.ProjectID {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: merge request %s project changed during approval collection from %d to %d", changeID, mergeRequest.ProjectID, currentMergeRequest.ProjectID)
	}
	if currentMergeRequest.SHA != mergeRequest.SHA || currentMergeRequest.Author.ID != mergeRequest.Author.ID {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: merge request %s head or author changed during approval collection from head=%s author=%d to head=%s author=%d", changeID, mergeRequest.SHA, mergeRequest.Author.ID, currentMergeRequest.SHA, currentMergeRequest.Author.ID)
	}
	if mergeRequest.Author.ID <= 0 {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: merge request %s carries no stable author user id", changeID)
	}

	seenUsers := make(map[int64]struct{}, len(response.ApprovedBy))
	approvals := make([]forge.Approval, 0, len(response.ApprovedBy))
	for _, row := range response.ApprovedBy {
		if row.User.ID <= 0 {
			return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: approval carries no stable actor user id")
		}
		if _, exists := seenUsers[row.User.ID]; exists {
			return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: duplicate current approver user id %d", row.User.ID)
		}
		seenUsers[row.User.ID] = struct{}{}
		stamp, err := forge.NormalizeTimestamp(row.ApprovedAt)
		if err != nil {
			return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: user %d approved_at: %w", row.User.ID, err)
		}
		userID := strconv.FormatInt(row.User.ID, 10)
		approvalID, err := derivedApprovalID(stableRepository, changeID, userID, mergeRequest.SHA, stamp)
		if err != nil {
			return forge.ApprovalSnapshot{}, err
		}
		approvals = append(approvals, forge.Approval{
			ApprovalID: approvalID, ApprovalRef: approvalID, State: forge.ApprovalActive,
			ApprovedAt: stamp, UpdatedAt: stamp, CandidateSHA: mergeRequest.SHA,
			Actor: forge.ProviderActor{Scheme: "gitlab-user-id", Subject: userID},
			ProviderWitnesses: []forge.ProviderWitness{
				{Name: "actor_user_id", Value: userID},
				{Name: "approved_at", Value: stamp},
				{Name: "candidate_sha", Value: mergeRequest.SHA},
			},
		})
	}

	snapshot, err := forge.NewApprovalSnapshot("gitlab", stableRepository, changeID, mergeRequest.SHA, forge.ProviderActor{Scheme: "gitlab-user-id", Subject: strconv.FormatInt(mergeRequest.Author.ID, 10)}, a.cfg.Clock(), approvals)
	if err != nil {
		// vocab:identity — GitLab provider resource name, not a Verdi lifecycle class.
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: normalize merge request %s approvals: %w", changeID, err)
	}
	return snapshot, nil
}

// EnvironmentReview implements forge.Forge (v2 ac-4, dc-5): GitLab has no
// environment-review concept analogous to GitHub's protected-environment
// reviewer gate — environment reviews are a GitHub-only approval source by
// construction (dc-5: "environment reviews are not a countersign source
// under team or high-assurance profiles", and the mapping itself is keyed
// "github-environment-review"). This is co-1's "unsupported source": no
// network call is made, and NormalizeEnvironmentReview turns the disclosed
// result into a zero-row outcome, never an error that would break a close.
func (a *Adapter) EnvironmentReview(ctx context.Context, query forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, error) {
	if err := ctx.Err(); err != nil {
		return forge.EnvironmentReviewFacts{}, err
	}
	facts, err := forge.NewEnvironmentReviewFacts(forge.EnvironmentReviewFacts{
		Supported:         false,
		UnsupportedReason: "gitlab: environment reviews are a GitHub-only approval source (v2 dc-5); gitlab has no forge-recorded review of a protected-environment workflow run to report",
		Repository:        a.cfg.ProjectID,
	}, a.cfg.Clock())
	if err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("gitlab: environment review: normalize unsupported facts: %w", err)
	}
	return facts, nil
}

func (a *Adapter) getApprovalJSON(ctx context.Context, requestURL string, out any) (headers http.Header, err error) {
	response, err := a.transport.RawDo(ctx, http.MethodGet, requestURL, nil, a.setAuth)
	if err != nil {
		return nil, fmt.Errorf("%w: gitlab: GET %s: %w", forge.ErrUnavailable, requestURL, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			// vocab:identity — HTTP response-body operation, not a Verdi lifecycle class.
			err = errors.Join(err, fmt.Errorf("gitlab: close GET %s response: %w", requestURL, closeErr))
		}
	}()
	if err := a.classify(http.MethodGet, requestURL, http.StatusOK)(response, nil); err != nil {
		return nil, err
	}
	if err := forge.DecodeApprovalJSON(response.Body, out); err != nil {
		return nil, fmt.Errorf("gitlab: GET %s: %w", requestURL, err)
	}
	return response.Header.Clone(), nil
}

func derivedApprovalID(repository, changeID, userID, candidateSHA, approvedAt string) (string, error) {
	identity := struct {
		Repository   string `json:"repository"`
		ChangeID     string `json:"change_id"`
		UserID       string `json:"user_id"`
		CandidateSHA string `json:"candidate_sha"`
		ApprovedAt   string `json:"approved_at"`
	}{repository, changeID, userID, candidateSHA, approvedAt}
	digest, err := canonjson.Digest(identity)
	if err != nil {
		return "", fmt.Errorf("gitlab: derive approval id: %w", err)
	}
	return digest, nil
}
