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
	SHA string `json:"sha"`
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
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: reading merge request %s head: %w", changeID, err)
	}

	approvalsURL := mergeRequestURL + "/approvals"
	var response approvalsJSON
	headers, err := a.getApprovalJSON(ctx, approvalsURL, &response)
	if err != nil {
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: reading merge request %s approvals: %w", changeID, err)
	}
	if headers.Get(nextPageHeader) != "" || headers.Get("Link") != "" {
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: approvals resource claimed an ambiguous pagination continuation")
	}
	if response.ApprovedBy == nil {
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: approvals response carries null or missing approved_by")
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
		approvalID, err := derivedApprovalID(a.cfg.ProjectID, changeID, userID, mergeRequest.SHA, stamp)
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

	snapshot, err := forge.NewApprovalSnapshot("gitlab", a.cfg.ProjectID, changeID, mergeRequest.SHA, a.cfg.Clock(), approvals)
	if err != nil {
		return forge.ApprovalSnapshot{}, fmt.Errorf("gitlab: normalize merge request %s approvals: %w", changeID, err)
	}
	return snapshot, nil
}

func (a *Adapter) getApprovalJSON(ctx context.Context, requestURL string, out any) (headers http.Header, err error) {
	response, err := a.transport.RawDo(ctx, http.MethodGet, requestURL, nil, a.setAuth)
	if err != nil {
		return nil, fmt.Errorf("gitlab: GET %s: %w", requestURL, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
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
