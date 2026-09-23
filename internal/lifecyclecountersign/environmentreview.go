package lifecyclecountersign

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/forge"
)

// CloseEnvironmentName is the protected GitHub environment whose review is a
// solo owner's close approval (SI-230; spec/vatc-forge-countersign-v2 ac-4,
// dc-5): .github/workflows/close.yml's close job declares `environment:
// close`. The normalizer does not pin the environment; this query does (L2b
// closure re-review, L2c integration notes).
const CloseEnvironmentName = "close"

// CloseGatedJobName is the name GitHub's jobs API reports for the gated
// close job, whose creation stamp dates the review (dc-5). close.yml's job
// id is `close` and it declares no job-level `name:`, so GitHub names the
// job by its id. It is a constant, never read from GITHUB_JOB, so a
// renamed or added job cannot change which job dates the review; the
// cmd/verdi close.yml pin test fails if the workflow stops matching it.
const CloseGatedJobName = "close"

// environmentReviewWitnessPrefix leads every witness this file contributes
// to the countersign record; the normalizer's own disclosures share it.
const environmentReviewWitnessPrefix = "environment-review:"

// currentRunEnvironmentReview adds the current CI run's environment-review
// approvals to snapshot (SI-230; v2 ac-4). Resolve calls it only when the
// governance kernel permitted the solo author/approver collapse (SI-233),
// so under team, high-assurance, or any profile whose rules keep
// separation, no environment review is ever requested or counted.
//
// The current run is the one the forge's own CI context names: on GitHub,
// GITHUB_RUN_ID and GITHUB_RUN_ATTEMPT (forge.CIInfo's Pipeline and Job).
// A missing or non-canonical value, an unavailable forge, and every adverse
// provider fact yield no approval and a disclosure, never an error that
// would hide the rest of the countersign. Facts that answer a different
// run, attempt, environment, or gated job than asked, and facts that break
// the facts contract, are forge contract violations and operational errors.
//
// The rows keep the run's head as their candidate. The countersign reducer
// counts a row only when that candidate equals both the open change's head
// and the locally evaluated commit (v2 dc-3), and every row's binding is
// also recorded here as a candidate-binding witness.
//
// It returns the snapshot the reducer must see and the witnesses the record
// must carry. It only reads: it never requests, creates, or writes an
// approval (co-3).
func (r Resolver) currentRunEnvironmentReview(ctx context.Context, snapshot forge.ApprovalSnapshot, localCandidateSHA string) (forge.ApprovalSnapshot, []string, error) {
	ci, err := r.Forge.CIContext(ctx)
	if err != nil {
		return forge.ApprovalSnapshot{}, nil, fmt.Errorf("lifecycle countersign: read the current run from the CI context: %w", err)
	}
	query, unresolved := currentRunQuery(ci)
	if unresolved != "" {
		return snapshot, []string{unresolved}, nil
	}
	subject := fmt.Sprintf("run_id=%s run_attempt=%d environment=%s gated_job=%s", query.RunID, query.RunAttempt, query.EnvironmentName, query.GatedJobName)

	facts, err := r.Forge.EnvironmentReview(ctx, query)
	if err != nil {
		if errors.Is(err, forge.ErrUnavailable) {
			return snapshot, []string{fmt.Sprintf(environmentReviewWitnessPrefix+"unavailable: %s: %v: no approval is read from this run", subject, err)}, nil
		}
		return forge.ApprovalSnapshot{}, nil, fmt.Errorf("lifecycle countersign: read the environment review of %s: %w", subject, err)
	}
	rows, disclosures, err := forge.NormalizeEnvironmentReview(facts)
	if err != nil {
		return forge.ApprovalSnapshot{}, nil, fmt.Errorf("lifecycle countersign: environment review of %s: %w", subject, err)
	}
	if facts.Supported && !answersQuery(facts, query) {
		return forge.ApprovalSnapshot{}, nil, fmt.Errorf("lifecycle countersign: environment review facts for run %s attempt %d environment %q gated job %q do not answer the query %s",
			facts.RunID, facts.RunAttempt, facts.EnvironmentName, facts.GatedJobName, subject)
	}

	witnesses := []string{fmt.Sprintf(environmentReviewWitnessPrefix+"observation: %s observed_at=%s provider_snapshot_id=%s change_provider_snapshot_id=%s",
		subject, facts.ObservedAt, facts.ProviderSnapshotID, snapshot.ProviderSnapshotID)}
	witnesses = append(witnesses, disclosures...)
	if len(rows) == 0 {
		return snapshot, witnesses, nil
	}
	if facts.Repository != snapshot.Repository {
		return snapshot, append(witnesses, fmt.Sprintf(environmentReviewWitnessPrefix+"repository-mismatch: %s review_repository=%s change_repository=%s: a review of another repository's run is no approval of this change",
			subject, facts.Repository, snapshot.Repository)), nil
	}
	for _, row := range rows {
		match := row.CandidateSHA == snapshot.CandidateSHA && row.CandidateSHA == localCandidateSHA
		witnesses = append(witnesses, fmt.Sprintf(environmentReviewWitnessPrefix+"candidate-binding: approval_id=%q run_head_sha=%q change_head_sha=%q local_candidate_sha=%q match=%t",
			row.ApprovalID, row.CandidateSHA, snapshot.CandidateSHA, localCandidateSHA, match))
	}

	observedAt, err := earliestStamp(snapshot.ObservedAt, facts.ObservedAt)
	if err != nil {
		return forge.ApprovalSnapshot{}, nil, fmt.Errorf("lifecycle countersign: environment review observation: %w", err)
	}
	approvals := append(append([]forge.Approval{}, snapshot.Approvals...), rows...)
	merged, err := forge.NewApprovalSnapshot(snapshot.Forge, snapshot.Repository, snapshot.ChangeID, snapshot.CandidateSHA, snapshot.CandidateAuthor, observedAt, approvals)
	if err != nil {
		return forge.ApprovalSnapshot{}, nil, fmt.Errorf("lifecycle countersign: add the environment review of %s to the approval set: %w", subject, err)
	}
	return merged, witnesses, nil
}

// currentRunQuery builds the environment-review query for the run the CI
// context names, pinned to the close environment and gated job. When the
// run cannot be named exactly, it returns a disclosure instead: a missing
// run id (outside CI, or a CI context without one), or a run id or attempt
// that is not a canonical positive base-10 number.
func currentRunQuery(ci forge.CIInfo) (forge.EnvironmentReviewQuery, string) {
	unresolved := func(reason string) string {
		// vocab:identity — "close" names the GitHub workflow file close.yml, a provider resource, not a Verdi lifecycle class.
		return fmt.Sprintf(environmentReviewWitnessPrefix+"current-run-unresolved: run_id=%q run_attempt=%q: %s; only the close workflow run's own environment review is a solo approval, so none is requested",
			ci.Pipeline, ci.Job, reason)
	}
	if ci.Pipeline == "" {
		return forge.EnvironmentReviewQuery{}, unresolved("the CI context names no current run (GITHUB_RUN_ID)")
	}
	if _, ok := canonicalPositive(ci.Pipeline, 64); !ok {
		return forge.EnvironmentReviewQuery{}, unresolved("the current run id is not a canonical positive base-10 number")
	}
	attempt, ok := canonicalPositive(ci.Job, 0)
	if !ok {
		return forge.EnvironmentReviewQuery{}, unresolved("the current run attempt (GITHUB_RUN_ATTEMPT) is missing or not a canonical positive base-10 number")
	}
	return forge.EnvironmentReviewQuery{
		RunID: ci.Pipeline, RunAttempt: int(attempt),
		EnvironmentName: CloseEnvironmentName, GatedJobName: CloseGatedJobName,
	}, ""
}

// canonicalPositive parses value as a positive base-10 number of bitSize
// bits (0 is int's size) in its one canonical spelling ("7", never "07" or
// "+7").
func canonicalPositive(value string, bitSize int) (int64, bool) {
	number, err := strconv.ParseInt(value, 10, bitSize)
	return number, err == nil && number > 0 && strconv.FormatInt(number, 10) == value
}

// answersQuery reports whether supported facts describe exactly the queried
// run attempt, environment, and gated job. GitHub environment names are
// case-insensitive.
func answersQuery(facts forge.EnvironmentReviewFacts, query forge.EnvironmentReviewQuery) bool {
	return facts.RunID == query.RunID && facts.RunAttempt == query.RunAttempt &&
		strings.EqualFold(facts.EnvironmentName, query.EnvironmentName) && facts.GatedJobName == query.GatedJobName
}

// earliestStamp returns the earlier of two normalized observation stamps.
// The merged approval set is dated by its oldest observation, so its
// observation age can only be overstated.
func earliestStamp(a, b string) (time.Time, error) {
	left, err := time.Parse(time.RFC3339Nano, a)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse observation stamp %q: %w", a, err)
	}
	right, err := time.Parse(time.RFC3339Nano, b)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse observation stamp %q: %w", b, err)
	}
	if right.Before(left) {
		return right, nil
	}
	return left, nil
}
