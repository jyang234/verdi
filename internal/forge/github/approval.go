package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/forge"
)

type approvalPullJSON struct {
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

type approvalReviewJSON struct {
	ID          int64  `json:"id"`
	NodeID      string `json:"node_id"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
	CommitID    string `json:"commit_id"`
	User        struct {
		ID    int64  `json:"id"`
		Login string `json:"login"`
	} `json:"user"`
}

// ListApprovals implements forge.Forge using the current PR head plus every
// page of immutable review rows. Known non-approval review states are ignored.
func (a *Adapter) ListApprovals(ctx context.Context, changeID string) (forge.ApprovalSnapshot, error) {
	if changeID == "" {
		return forge.ApprovalSnapshot{}, fmt.Errorf("github: change id is empty")
	}
	escapedChange := url.PathEscape(changeID)
	pullURL := fmt.Sprintf("%s/repos/%s/%s/pulls/%s", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo, escapedChange)
	var pull approvalPullJSON
	if _, err := a.getApprovalJSON(ctx, pullURL, &pull); err != nil {
		return forge.ApprovalSnapshot{}, fmt.Errorf("github: reading pull request %s head: %w", changeID, err)
	}

	reviewsURL := pullURL + "/reviews"
	reviews, err := a.drainApprovalReviews(ctx, reviewsURL)
	if err != nil {
		return forge.ApprovalSnapshot{}, fmt.Errorf("github: reading pull request %s reviews: %w", changeID, err)
	}
	var currentPull approvalPullJSON
	if _, err := a.getApprovalJSON(ctx, pullURL, &currentPull); err != nil {
		return forge.ApprovalSnapshot{}, fmt.Errorf("github: rereading pull request %s head after approval collection: %w", changeID, err)
	}
	if currentPull.Head.SHA != pull.Head.SHA {
		return forge.ApprovalSnapshot{}, fmt.Errorf("github: pull request %s head changed during approval collection from %s to %s", changeID, pull.Head.SHA, currentPull.Head.SHA)
	}

	seenReviewIDs := make(map[int64]struct{}, len(reviews))
	approvals := make([]forge.Approval, 0, len(reviews))
	for _, review := range reviews {
		if review.ID <= 0 {
			return forge.ApprovalSnapshot{}, fmt.Errorf("github: review carries missing immutable id")
		}
		if _, exists := seenReviewIDs[review.ID]; exists {
			return forge.ApprovalSnapshot{}, fmt.Errorf("github: duplicate review id %d", review.ID)
		}
		seenReviewIDs[review.ID] = struct{}{}

		state, keep, err := normalizeReviewState(review.State)
		if err != nil {
			return forge.ApprovalSnapshot{}, err
		}
		if !keep {
			continue
		}
		if review.NodeID == "" {
			return forge.ApprovalSnapshot{}, fmt.Errorf("github: review %d carries no immutable node ref", review.ID)
		}
		if review.User.ID <= 0 {
			return forge.ApprovalSnapshot{}, fmt.Errorf("github: review %d carries no stable actor user id", review.ID)
		}
		stamp, err := forge.NormalizeTimestamp(review.SubmittedAt)
		if err != nil {
			return forge.ApprovalSnapshot{}, fmt.Errorf("github: review %d submitted_at: %w", review.ID, err)
		}
		approvalID := strconv.FormatInt(review.ID, 10)
		userID := strconv.FormatInt(review.User.ID, 10)
		approvals = append(approvals, forge.Approval{
			ApprovalID: approvalID, ApprovalRef: review.NodeID, State: state,
			ApprovedAt: stamp, UpdatedAt: stamp, CandidateSHA: review.CommitID,
			Actor: forge.ProviderActor{Scheme: "github-user-id", Subject: userID},
			ProviderWitnesses: []forge.ProviderWitness{
				{Name: "actor_user_id", Value: userID},
				{Name: "review_commit_id", Value: review.CommitID},
				{Name: "review_id", Value: approvalID},
				{Name: "review_node_id", Value: review.NodeID},
				{Name: "review_state", Value: review.State},
				{Name: "review_submitted_at", Value: stamp},
			},
		})
	}

	repository := a.cfg.Owner + "/" + a.cfg.Repo
	snapshot, err := forge.NewApprovalSnapshot("github", repository, changeID, pull.Head.SHA, a.cfg.Clock(), approvals)
	if err != nil {
		return forge.ApprovalSnapshot{}, fmt.Errorf("github: normalize pull request %s approvals: %w", changeID, err)
	}
	return snapshot, nil
}

func (a *Adapter) drainApprovalReviews(ctx context.Context, firstURL string) ([]approvalReviewJSON, error) {
	var all []approvalReviewJSON
	next := withPerPage(firstURL)
	visited := make(map[string]struct{})
	for next != "" {
		current := next
		if _, exists := visited[current]; exists {
			return nil, fmt.Errorf("github: approval pagination cycle detected: %s was already visited", current)
		}
		visited[current] = struct{}{}
		var page []approvalReviewJSON
		headers, err := a.getApprovalJSON(ctx, current, &page)
		if err != nil {
			return nil, err
		}
		if page == nil {
			return nil, fmt.Errorf("github: reviews page must be a non-null array")
		}
		all = append(all, page...)
		next, err = approvalNextLink(headers.Get("Link"))
		if err != nil {
			return nil, err
		}
	}
	return all, nil
}

func approvalNextLink(header string) (string, error) {
	next := ""
	for _, member := range strings.Split(header, ",") {
		if !strings.Contains(member, `rel="next"`) {
			continue
		}
		candidate := parseLinkNext(member)
		if candidate == "" {
			return "", fmt.Errorf("github: malformed approval pagination Link claims rel=\"next\"")
		}
		if _, err := url.ParseRequestURI(candidate); err != nil {
			return "", fmt.Errorf("github: malformed approval pagination next URL %q: %w", candidate, err)
		}
		if next != "" && candidate != next {
			return "", fmt.Errorf("github: approval pagination Link carries multiple distinct next continuations %q and %q", next, candidate)
		}
		next = candidate
	}
	return next, nil
}

func (a *Adapter) getApprovalJSON(ctx context.Context, requestURL string, out any) (headers http.Header, err error) {
	response, err := a.transport.RawDo(ctx, http.MethodGet, requestURL, nil, a.setAuth)
	if err != nil {
		return nil, fmt.Errorf("github: GET %s: %w", requestURL, err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			// vocab:identity — HTTP response-body operation, not a Verdi lifecycle class.
			err = errors.Join(err, fmt.Errorf("github: close GET %s response: %w", requestURL, closeErr))
		}
	}()
	if err := a.classify(http.MethodGet, requestURL, http.StatusOK)(response, nil); err != nil {
		return nil, err
	}
	if err := forge.DecodeApprovalJSON(response.Body, out); err != nil {
		return nil, fmt.Errorf("github: GET %s: %w", requestURL, err)
	}
	return response.Header.Clone(), nil
}

func normalizeReviewState(providerState string) (forge.ApprovalState, bool, error) {
	switch providerState {
	case "APPROVED":
		return forge.ApprovalActive, true, nil
	case "DISMISSED":
		return forge.ApprovalDismissed, true, nil
	case "CHANGES_REQUESTED", "COMMENTED", "PENDING":
		return "", false, nil
	default:
		return "", false, fmt.Errorf("github: unknown review state %q", providerState)
	}
}
