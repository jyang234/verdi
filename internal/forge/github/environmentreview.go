package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/forge"
)

// environmentReviewRunJSON is the tolerant subset of GitHub's workflow run
// object ac-4 needs (GitHub REST API docs, "Get a workflow run", GET
// /repos/{owner}/{repo}/actions/runs/{run_id}: id, run_attempt — "1 for
// first attempt and higher if the workflow was re-run", so the run
// endpoint reports the latest attempt — head_sha, html_url).
type environmentReviewRunJSON struct {
	ID         int64  `json:"id"`
	RunAttempt int    `json:"run_attempt"`
	HeadSHA    string `json:"head_sha"`
	HTMLURL    string `json:"html_url"`
}

// environmentReviewJobsResponse is "List jobs for a workflow run attempt"'s
// response wrapper (GitHub REST API docs, Actions > Workflow jobs).
type environmentReviewJobsResponse struct {
	TotalCount int                        `json:"total_count"`
	Jobs       []environmentReviewJobJSON `json:"jobs"`
}

// environmentReviewJobJSON is the tolerant subset of GitHub's workflow job
// object dc-5 needs: name (to find the gated job among the attempt's jobs,
// since a job carries no environment reference of its own) and
// created_at/started_at (dc-5's conservative-lower-bound approval instant
// and the separately witnessed start stamp).
type environmentReviewJobJSON struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	StartedAt string `json:"started_at"`
}

// environmentReviewHistoryEnvironmentJSON is one member of a review history
// entry's `environments` array (GitHub REST API docs, "Get the review
// history for a workflow run").
type environmentReviewHistoryEnvironmentJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// environmentReviewHistoryEntryJSON is one entry in GitHub's review history
// for a workflow run (REST `actions/runs/{run_id}/approvals`): one
// reviewer's one decision, covering every environment named in
// `environments` at once. GitHub's review history carries no review id and
// no review time (dc-5) — the entry itself is the only fact.
type environmentReviewHistoryEntryJSON struct {
	State        forge.EnvironmentReviewState              `json:"state"`
	Comment      string                                    `json:"comment"`
	Environments []environmentReviewHistoryEnvironmentJSON `json:"environments"`
	User         struct {
		ID int64 `json:"id"`
	} `json:"user"`
}

// environmentJSON is the tolerant subset of GitHub's "Get an environment"
// response ac-4/dc-5 need: id, name, and protection_rules (for the
// required_reviewers rule's prevent_self_review setting).
type environmentJSON struct {
	ID              int64                           `json:"id"`
	Name            string                          `json:"name"`
	ProtectionRules []environmentProtectionRuleJSON `json:"protection_rules"`
}

// environmentProtectionRuleJSON spans every documented protection_rules[]
// variant (GitHub REST API docs, "Get an environment" /
// "Create or update an environment": wait_timer, required_reviewers,
// branch_policy are discriminated by `type`) — only required_reviewers's
// own `prevent_self_review` is used here (dc-5: "the adapter discloses
// that setting when the forge reports it"). The `reviewers` array is
// decoded as opaque raw JSON: its members' shape is irrelevant to ac-4,
// which needs only the rule's self-review setting, not who may review.
type environmentProtectionRuleJSON struct {
	ID                int64             `json:"id"`
	NodeID            string            `json:"node_id"`
	Type              string            `json:"type"`
	WaitTimer         *int              `json:"wait_timer"`
	PreventSelfReview *bool             `json:"prevent_self_review"`
	Reviewers         []json.RawMessage `json:"reviewers"`
}

// EnvironmentReview implements forge.Forge (v2 ac-4, dc-5). It reads, in
// this order, the run's review history (filtered to the named environment
// by id), the queried attempt's jobs (to find the gated job's creation and
// start stamps), the named environment's id and self-review setting, and
// last the run itself: its latest attempt, head commit, and URL.
//
// The run is read after the review history on purpose (L2b review I-1):
// GitHub's review history carries no attempt, so it can be attributed to
// attempt 1 only if no rerun existed when it was read, and a latest attempt
// of 1 observed afterwards proves exactly that.
//
// Every call rides the approval decode seam (getApprovalJSON/
// DecodeApprovalJSON): GitHub's responses are an open contract (ruling
// R-W1-8), so members this adapter does not model are ignored while trailing
// data, unknown review states, and missing ids are still rejected (co-1).
func (a *Adapter) EnvironmentReview(ctx context.Context, query forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, error) {
	runID, err := validateEnvironmentReviewQuery(query)
	if err != nil {
		return forge.EnvironmentReviewFacts{}, err
	}
	base := fmt.Sprintf("%s/repos/%s/%s", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo)

	historyURL := fmt.Sprintf("%s/actions/runs/%d/approvals", base, runID)
	entries, err := githubDrainStrictList(ctx, a, historyURL, func(p []environmentReviewHistoryEntryJSON) []environmentReviewHistoryEntryJSON { return p })
	if err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: reading run %d review history: %w", runID, err)
	}

	jobsURL := fmt.Sprintf("%s/actions/runs/%d/attempts/%d/jobs", base, runID, query.RunAttempt)
	jobs, err := githubDrainStrictList(ctx, a, jobsURL, func(p environmentReviewJobsResponse) []environmentReviewJobJSON { return p.Jobs })
	if err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: reading run %d attempt %d jobs: %w", runID, query.RunAttempt, err)
	}
	var gatedJobFound bool
	var gatedJobCreatedAt, gatedJobStartedAt string
	for _, j := range jobs {
		if j.Name != query.GatedJobName {
			continue
		}
		if gatedJobFound {
			return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: run %d attempt %d carries more than one job named %q", runID, query.RunAttempt, query.GatedJobName)
		}
		gatedJobFound = true
		if j.CreatedAt != "" {
			gatedJobCreatedAt, err = forge.NormalizeTimestamp(j.CreatedAt)
			if err != nil {
				return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: gated job created_at: %w", err)
			}
		}
		if j.StartedAt != "" {
			gatedJobStartedAt, err = forge.NormalizeTimestamp(j.StartedAt)
			if err != nil {
				return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: gated job started_at: %w", err)
			}
		}
	}

	envURL := fmt.Sprintf("%s/environments/%s", base, url.PathEscape(query.EnvironmentName))
	var env environmentJSON
	if _, err := a.getApprovalJSON(ctx, envURL, &env); err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: reading environment %q: %w", query.EnvironmentName, err)
	}
	if env.ID <= 0 {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: environment %q carries no stable id", query.EnvironmentName)
	}
	if !strings.EqualFold(env.Name, query.EnvironmentName) {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: environment %q reported name %q", query.EnvironmentName, env.Name)
	}
	var preventSelfReview *bool
	for _, rule := range env.ProtectionRules {
		if rule.Type != "required_reviewers" {
			continue
		}
		if rule.PreventSelfReview != nil {
			preventSelfReview = rule.PreventSelfReview
		}
	}

	rows, err := environmentReviewRows(entries, env.ID, query.EnvironmentName)
	if err != nil {
		return forge.EnvironmentReviewFacts{}, err
	}

	runURL := fmt.Sprintf("%s/actions/runs/%d", base, runID)
	var run environmentReviewRunJSON
	if _, err := a.getApprovalJSON(ctx, runURL, &run); err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: reading run %d: %w", runID, err)
	}
	if run.ID != runID {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: run %d reported id %d", runID, run.ID)
	}
	if run.RunAttempt < 1 {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: run %d carries no run_attempt", runID)
	}
	if run.HeadSHA == "" {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: run %d carries no head sha", runID)
	}
	if run.HTMLURL == "" {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: run %d carries no html url", runID)
	}

	facts, err := forge.NewEnvironmentReviewFacts(forge.EnvironmentReviewFacts{
		Supported:                    true,
		Repository:                   a.cfg.Owner + "/" + a.cfg.Repo,
		RunID:                        strconv.FormatInt(runID, 10),
		RunAttempt:                   query.RunAttempt,
		LatestRunAttempt:             run.RunAttempt,
		RunHeadSHA:                   run.HeadSHA,
		RunURL:                       run.HTMLURL,
		EnvironmentID:                strconv.FormatInt(env.ID, 10),
		EnvironmentName:              env.Name,
		GatedJobFound:                gatedJobFound,
		GatedJobCreatedAt:            gatedJobCreatedAt,
		GatedJobStartedAt:            gatedJobStartedAt,
		EnvironmentPreventSelfReview: preventSelfReview,
		Reviews:                      rows,
	}, a.cfg.Clock())
	if err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: normalize facts: %w", err)
	}
	return facts, nil
}

// validateEnvironmentReviewQuery checks the query and returns its run id as
// the canonical positive base-10 number (L2b review m-1).
func validateEnvironmentReviewQuery(query forge.EnvironmentReviewQuery) (int64, error) {
	if query.RunAttempt < 1 {
		return 0, fmt.Errorf("github: environment review: run attempt %d is not a positive attempt number", query.RunAttempt)
	}
	if query.EnvironmentName == "" {
		return 0, fmt.Errorf("github: environment review: environment name is empty")
	}
	if query.GatedJobName == "" {
		return 0, fmt.Errorf("github: environment review: gated job name is empty")
	}
	runID, err := strconv.ParseInt(query.RunID, 10, 64)
	if err != nil || runID <= 0 || strconv.FormatInt(runID, 10) != query.RunID {
		return 0, fmt.Errorf("github: environment review: run id %q is not a canonical positive base-10 id", query.RunID)
	}
	return runID, nil
}

// environmentReviewRows keeps the review-history entries for environment
// envID. An entry belongs to it only when one of the entry's own
// environments carries that id (names are case-insensitive and renameable,
// ids are not; L2b review m-2), and the row's environment id is taken from
// that entry. An entry naming no environment, or an environment without an
// id, is refused rather than silently dropped.
func environmentReviewRows(entries []environmentReviewHistoryEntryJSON, envID int64, envName string) ([]forge.EnvironmentReviewRow, error) {
	var rows []forge.EnvironmentReviewRow
	for i, entry := range entries {
		if len(entry.Environments) == 0 {
			return nil, fmt.Errorf("github: environment review: review history entry %d names no environment", i)
		}
		matched := int64(0)
		for _, e := range entry.Environments {
			if e.ID <= 0 {
				return nil, fmt.Errorf("github: environment review: review history entry %d names an environment with no stable id", i)
			}
			if e.ID == envID {
				matched = e.ID
			}
		}
		if matched == 0 {
			continue
		}
		if entry.User.ID <= 0 {
			return nil, fmt.Errorf("github: environment review: review history entry for environment %q carries no stable reviewer id", envName)
		}
		rows = append(rows, forge.EnvironmentReviewRow{
			ReviewerActor: forge.ProviderActor{Scheme: "github-user-id", Subject: strconv.FormatInt(entry.User.ID, 10)},
			ProviderState: entry.State,
			EnvironmentID: strconv.FormatInt(matched, 10),
		})
	}
	return rows, nil
}
