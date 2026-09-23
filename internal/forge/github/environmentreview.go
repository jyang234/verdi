package github

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jyang234/verdi/internal/forge"
)

// The wire types below are tolerant subsets of GitHub's published REST
// responses (ruling R-W1-8): each declares only the members v2 ac-4 relies
// on, strictly typed, and every other member GitHub sends is ignored. Their
// shapes follow GitHub's OpenAPI description (github/rest-api-description)
// and the examples docs.github.com publishes from it.

// environmentReviewRunJSON is the subset of "Get a workflow run" (GET
// /repos/{owner}/{repo}/actions/runs/{run_id}) ac-4 needs: id; run_attempt,
// "1 for first attempt and higher if the workflow was re-run", so the run
// endpoint reports the latest attempt; head_sha and html_url; and event,
// path, status, and conclusion, which express ac-4's dispatch-only close
// workflow, cancelled-run, and failed-run exclusions (L2b review I-3).
// status and conclusion are nullable; null decodes as "".
type environmentReviewRunJSON struct {
	ID         int64  `json:"id"`
	RunAttempt int    `json:"run_attempt"`
	HeadSHA    string `json:"head_sha"`
	HTMLURL    string `json:"html_url"`
	Event      string `json:"event"`
	Path       string `json:"path"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}

// environmentReviewJobsPage is one page of "List jobs for a workflow run
// attempt" (GET .../actions/runs/{run_id}/attempts/{attempt_number}/jobs):
// total_count and jobs are required members, so a page missing either, or
// a null page, is refused (L2b review m-8).
type environmentReviewJobsPage struct {
	TotalCount *int                       `json:"total_count"`
	Jobs       []environmentReviewJobJSON `json:"jobs"`
}

// environmentReviewJobJSON is the subset of GitHub's workflow job object
// dc-5 needs: name (a job carries no environment reference of its own, so
// the gated job is found by the name the query gives), id, status, and
// conclusion (I-3, m-4), and created_at/started_at (dc-5's conservative
// lower bound and the separately witnessed start). conclusion is nullable.
type environmentReviewJobJSON struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	CreatedAt  string `json:"created_at"`
	StartedAt  string `json:"started_at"`
}

// environmentReviewHistoryEnvironmentJSON is one member of a review history
// entry's `environments` array ("Get the review history for a workflow
// run", GET .../actions/runs/{run_id}/approvals).
type environmentReviewHistoryEnvironmentJSON struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// environmentReviewHistoryEntryJSON is one entry of GitHub's review history
// for a workflow run: one reviewer's one decision, covering every
// environment named in `environments` at once. GitHub's review history
// carries no review id, no review time, and no attempt (dc-5), so this type
// declares none: the entry itself is the only fact.
type environmentReviewHistoryEntryJSON struct {
	State        forge.EnvironmentReviewState              `json:"state"`
	Environments []environmentReviewHistoryEnvironmentJSON `json:"environments"`
	User         struct {
		ID int64 `json:"id"`
	} `json:"user"`
}

// environmentJSON is the subset of "Get an environment" (GET
// /repos/{owner}/{repo}/environments/{environment_name}) ac-4/dc-5 need:
// id, name, and protection_rules, for the required_reviewers rule's
// prevent_self_review setting (dc-5: "the adapter discloses that setting
// when the forge reports it").
type environmentJSON struct {
	ID              int64                           `json:"id"`
	Name            string                          `json:"name"`
	ProtectionRules []environmentProtectionRuleJSON `json:"protection_rules"`
}

// environmentProtectionRuleJSON is the subset of one protection_rules[]
// member (wait_timer, required_reviewers, and branch_policy variants,
// discriminated by `type`) the adapter reads.
type environmentProtectionRuleJSON struct {
	Type              string `json:"type"`
	PreventSelfReview *bool  `json:"prevent_self_review"`
}

// EnvironmentReview implements forge.Forge (v2 ac-4, dc-5). It reads, in
// this order, the run's review history (filtered to the named environment
// by id), the queried attempt's jobs (to find the gated job's id, status,
// conclusion, and creation and start stamps), the named environment's id and
// self-review setting, and last the run itself: its latest attempt, head
// commit, URL, event, workflow path, status, and conclusion.
//
// The run is read after the review history on purpose (L2b review I-1):
// GitHub's review history carries no attempt, so it can be attributed to
// attempt 1 only if no rerun existed when it was read, and a latest attempt
// of 1 observed afterwards proves exactly that.
//
// Every call rides the approval decode seam (getApprovalJSON/
// DecodeApprovalJSON): GitHub's responses are an open contract (ruling
// R-W1-8), so members this adapter does not model are ignored while trailing
// data, unknown review states, run and job statuses and conclusions, and
// missing ids are still rejected (co-1).
func (a *Adapter) EnvironmentReview(ctx context.Context, query forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, error) {
	runID, err := validateEnvironmentReviewQuery(query)
	if err != nil {
		return forge.EnvironmentReviewFacts{}, err
	}
	base := fmt.Sprintf("%s/repos/%s/%s", a.cfg.BaseURL, a.cfg.Owner, a.cfg.Repo)

	entries, err := a.readReviewHistory(ctx, fmt.Sprintf("%s/actions/runs/%d/approvals", base, runID))
	if err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: reading run %d review history: %w", runID, err)
	}

	jobs, err := a.readAttemptJobs(ctx, fmt.Sprintf("%s/actions/runs/%d/attempts/%d/jobs", base, runID, query.RunAttempt))
	if err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: reading run %d attempt %d jobs: %w", runID, query.RunAttempt, err)
	}

	var env environmentJSON
	if _, err := a.getApprovalJSON(ctx, fmt.Sprintf("%s/environments/%s", base, url.PathEscape(query.EnvironmentName)), &env); err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: reading environment %q: %w", query.EnvironmentName, err)
	}
	if env.ID <= 0 {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: environment %q carries no stable id", query.EnvironmentName)
	}
	// GitHub environment names are case-insensitive (L2b review m-2).
	if !strings.EqualFold(env.Name, query.EnvironmentName) {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: environment %q reported name %q", query.EnvironmentName, env.Name)
	}

	rows, err := environmentReviewRows(entries, env.ID, query.EnvironmentName)
	if err != nil {
		return forge.EnvironmentReviewFacts{}, err
	}

	var run environmentReviewRunJSON
	if _, err := a.getApprovalJSON(ctx, fmt.Sprintf("%s/actions/runs/%d", base, runID), &run); err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: reading run %d: %w", runID, err)
	}
	if run.ID != runID {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: run %d reported id %d", runID, run.ID)
	}
	if run.RunAttempt < 1 {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: run %d carries no run_attempt", runID)
	}

	draft := forge.EnvironmentReviewFacts{
		Supported:                    true,
		Repository:                   a.cfg.Owner + "/" + a.cfg.Repo,
		RunID:                        strconv.FormatInt(runID, 10),
		RunAttempt:                   query.RunAttempt,
		LatestRunAttempt:             run.RunAttempt,
		RunHeadSHA:                   run.HeadSHA,
		RunURL:                       run.HTMLURL,
		RunEvent:                     run.Event,
		RunWorkflowPath:              run.Path,
		RunStatus:                    run.Status,
		RunConclusion:                run.Conclusion,
		EnvironmentID:                strconv.FormatInt(env.ID, 10),
		EnvironmentName:              env.Name,
		EnvironmentPreventSelfReview: preventSelfReview(env),
		GatedJobName:                 query.GatedJobName,
		Reviews:                      rows,
	}
	if err := gatedJobFacts(&draft, jobs); err != nil {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("github: environment review: run %d attempt %d: %w", runID, query.RunAttempt, err)
	}
	facts, err := forge.NewEnvironmentReviewFacts(draft, a.cfg.Clock())
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

// readReviewHistory drains the review history. GitHub documents no
// pagination for this endpoint; a Link continuation, if one is ever sent,
// is still followed under the strict approval pagination rules. A null
// page is refused (L2b review m-8).
func (a *Adapter) readReviewHistory(ctx context.Context, historyURL string) ([]environmentReviewHistoryEntryJSON, error) {
	return githubDrainStrictList(ctx, a, historyURL, func(page []environmentReviewHistoryEntryJSON) ([]environmentReviewHistoryEntryJSON, error) {
		if page == nil {
			return nil, fmt.Errorf("github: review history page must be a non-null array")
		}
		return page, nil
	})
}

// readAttemptJobs drains the attempt's jobs, refusing a page without
// total_count or a non-null jobs array, pages that disagree on total_count,
// and a drained list whose length is not that total (L2b review m-8).
func (a *Adapter) readAttemptJobs(ctx context.Context, jobsURL string) ([]environmentReviewJobJSON, error) {
	total := -1
	jobs, err := githubDrainStrictList(ctx, a, jobsURL, func(page environmentReviewJobsPage) ([]environmentReviewJobJSON, error) {
		if page.TotalCount == nil || page.Jobs == nil {
			return nil, fmt.Errorf("github: jobs page must carry total_count and a non-null jobs array")
		}
		if total >= 0 && *page.TotalCount != total {
			return nil, fmt.Errorf("github: jobs pages disagree on total_count: %d then %d", total, *page.TotalCount)
		}
		total = *page.TotalCount
		return page.Jobs, nil
	})
	if err != nil {
		return nil, err
	}
	if len(jobs) != total {
		return nil, fmt.Errorf("github: jobs total_count %d does not match the %d jobs listed", total, len(jobs))
	}
	return jobs, nil
}

// gatedJobFacts records how many of the attempt's jobs carry the gated
// job's name and, for exactly one, its id, status, conclusion, and stamps
// (L2b review I-3, m-4). Zero or several such jobs are facts the
// normalizer refuses with a disclosure, never an error.
func gatedJobFacts(draft *forge.EnvironmentReviewFacts, jobs []environmentReviewJobJSON) error {
	var gated []environmentReviewJobJSON
	for _, job := range jobs {
		if job.Name == draft.GatedJobName {
			gated = append(gated, job)
		}
	}
	draft.GatedJobCount = len(gated)
	if len(gated) != 1 {
		return nil
	}
	job := gated[0]
	if job.ID <= 0 {
		return fmt.Errorf("gated job %q carries no stable id", draft.GatedJobName)
	}
	draft.GatedJobID = strconv.FormatInt(job.ID, 10)
	draft.GatedJobStatus = job.Status
	draft.GatedJobConclusion = job.Conclusion
	if job.CreatedAt != "" {
		stamp, err := forge.NormalizeTimestamp(job.CreatedAt)
		if err != nil {
			return fmt.Errorf("gated job created_at: %w", err)
		}
		draft.GatedJobCreatedAt = stamp
	}
	if job.StartedAt != "" {
		stamp, err := forge.NormalizeTimestamp(job.StartedAt)
		if err != nil {
			return fmt.Errorf("gated job started_at: %w", err)
		}
		draft.GatedJobStartedAt = stamp
	}
	return nil
}

// preventSelfReview returns the required_reviewers rule's
// prevent_self_review setting, or nil when GitHub does not report one.
func preventSelfReview(env environmentJSON) *bool {
	var setting *bool
	for _, rule := range env.ProtectionRules {
		if rule.Type == "required_reviewers" && rule.PreventSelfReview != nil {
			setting = rule.PreventSelfReview
		}
	}
	return setting
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
