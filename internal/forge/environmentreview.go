package forge

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
)

// EnvironmentReviewState is the closed provider-neutral vocabulary GitHub's
// environment review history reports (v2 ac-4, dc-5; GitHub REST API "Get
// the review history for a workflow run": `state` is `approved`,
// `rejected`, or `pending`). Unknown values fail closed at decode
// (co-1's amendment: "an environment review's composite identity replaces
// the provider id it lacks... adapters normalize provider-specific
// shapes").
type EnvironmentReviewState string

const (
	// EnvironmentReviewApproved is the only state NormalizeEnvironmentReview
	// ever turns into an Approval row (dc-5).
	EnvironmentReviewApproved EnvironmentReviewState = "approved"
	// EnvironmentReviewRejected means the provider recorded a rejection.
	EnvironmentReviewRejected EnvironmentReviewState = "rejected"
	// EnvironmentReviewPending means the reviewer has not yet decided.
	EnvironmentReviewPending EnvironmentReviewState = "pending"
)

// UnmarshalJSON rejects states outside the provider-neutral closed set,
// mirroring ApprovalState's decode contract.
func (s *EnvironmentReviewState) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("forge: decode environment review state: %w", err)
	}
	state := EnvironmentReviewState(value)
	if !state.valid() {
		return fmt.Errorf("forge: unknown environment review state %q", value)
	}
	*s = state
	return nil
}

func (s EnvironmentReviewState) valid() bool {
	switch s {
	case EnvironmentReviewApproved, EnvironmentReviewRejected, EnvironmentReviewPending:
		return true
	default:
		return false
	}
}

// CloseWorkflowPath is the repository path of the dispatch-only close
// workflow (plan R-CM-4, .github/workflows/close.yml). v2 ac-4 honors only
// an environment review "of the dispatch-only close workflow run" (L2b
// review I-3). GitHub reports a run's workflow either as this path or,
// as its published "Get a workflow run" example shows
// (".github/workflows/build.yml@main"), as the path followed by "@" and the
// ref the workflow file was read from; both name this workflow.
const CloseWorkflowPath = ".github/workflows/close.yml"

// WorkflowDispatchEvent is the only run event v2 ac-4 honors: the close
// workflow is dispatch-only.
const WorkflowDispatchEvent = "workflow_dispatch"

// IsCloseWorkflowPath reports whether a run's workflow path names
// CloseWorkflowPath, bare or with a non-empty "@<ref>" suffix.
func IsCloseWorkflowPath(path string) bool {
	if path == CloseWorkflowPath {
		return true
	}
	ref, ok := strings.CutPrefix(path, CloseWorkflowPath+"@")
	return ok && ref != ""
}

// The workflow run and job vocabularies GitHub documents. A run's status and
// conclusion are the enums of the workflow_run webhook payload's
// workflow_run object (GitHub's OpenAPI description, the same run schema the
// REST "Get a workflow run" response carries without an enum); a job's
// status and conclusion are the enums of the REST job schema ("List jobs for
// a workflow run attempt"). An unknown value of any of them fails closed
// (ruling R-W1-8: unknown values of fields Verdi relies on still fail).
var (
	workflowRunStatuses = map[string]bool{
		"requested": true, "in_progress": true, "completed": true, "queued": true, "pending": true, "waiting": true,
	}
	workflowRunConclusions = map[string]bool{
		"action_required": true, "cancelled": true, "failure": true, "neutral": true, "skipped": true,
		"stale": true, "success": true, "timed_out": true, "startup_failure": true,
	}
	workflowJobStatuses = map[string]bool{
		"queued": true, "in_progress": true, "completed": true, "waiting": true, "requested": true, "pending": true,
	}
	workflowJobConclusions = map[string]bool{
		"success": true, "failure": true, "neutral": true, "cancelled": true, "skipped": true,
		"timed_out": true, "action_required": true,
	}
)

// EnvironmentReviewQuery names one workflow run attempt's gated job and its
// approval environment (v2 ac-4: "The query names the run, the attempt,
// the environment (`close`), and the gated job"). RunID and RunAttempt
// identify the exact attempt (GitHub carries no review id or review time,
// so the attempt is part of the composite approval identity — dc-5).
// GatedJobName is the job's declared name within that attempt whose
// creation/start stamps date the review (dc-5's conservative lower bound);
// GitHub's job objects carry no environment reference of their own, so the
// caller must name it.
type EnvironmentReviewQuery struct {
	RunID           string
	RunAttempt      int
	EnvironmentName string
	GatedJobName    string
}

// EnvironmentReviewRow is one reviewer's decision on the query's
// environment, exactly as the provider reports it — never yet reduced to a
// forge.Approval (NormalizeEnvironmentReview does that; dc-5).
// EnvironmentID is the environment id the review entry itself names (a
// canonical decimal), so the composite identity's environment component
// comes from the review, never from a later environment read (L2b review
// m-2); Validate requires it to equal the facts' EnvironmentID.
type EnvironmentReviewRow struct {
	ReviewerActor ProviderActor          `json:"reviewer_actor"`
	ProviderState EnvironmentReviewState `json:"provider_state"`
	EnvironmentID string                 `json:"environment_id"`
}

// EnvironmentReviewFacts is one forge observation of one workflow run
// attempt's environment-review facts (v2 ac-4, dc-5) — provider facts, not
// yet a countersign judgment (AC-1's port/judgment split applies here too).
//
// Supported is false for a forge adapter that does not implement
// environment reviews as an approval source (co-1's "unsupported source"):
// every run/job/environment/review field must then be its zero value and
// UnsupportedReason names why — NormalizeEnvironmentReview turns that into a
// disclosed zero-row result, never an error that would break a close.
//
// The run facts (event, workflow path, status, conclusion) and the gated
// job's facts (how many jobs of the attempt carry its name, and for exactly
// one: its id, status, conclusion, and stamps) are what lets the normalizer
// express v2 ac-4's exclusions (L2b review I-3): a run that is not the
// dispatch-only close workflow's, a cancelled or failed run, and a gated job
// that is not in progress or completed successfully all yield no row.
//
// Times are normalized UTC RFC3339Nano. GatedJobCreatedAt/GatedJobStartedAt
// are "" when the provider does not report them for this attempt's gated
// job (dc-5: "when the creation stamp is unavailable"); every gated-job
// field except GatedJobName and GatedJobCount is empty unless exactly one
// job of the attempt carries the gated job's name. RunStatus is "" when the
// provider reports none (GitHub's run status is nullable); RunConclusion
// and GatedJobConclusion are "" while the run or job has not concluded.
type EnvironmentReviewFacts struct {
	Supported         bool   `json:"supported"`
	UnsupportedReason string `json:"unsupported_reason"`

	Repository string `json:"repository"`
	RunID      string `json:"run_id"`
	// RunAttempt is the attempt the query names; LatestRunAttempt is the
	// run's own run_attempt, its latest attempt, read after the review
	// history (L2b review I-1). GitHub's review history carries no attempt,
	// so a review is honored only while the latest attempt is 1 and equals
	// the queried attempt.
	RunAttempt       int    `json:"run_attempt"`
	LatestRunAttempt int    `json:"latest_run_attempt"`
	RunHeadSHA       string `json:"run_head_sha"`
	RunURL           string `json:"run_url"`
	RunEvent         string `json:"run_event"`
	RunWorkflowPath  string `json:"run_workflow_path"`
	RunStatus        string `json:"run_status"`
	RunConclusion    string `json:"run_conclusion"`

	EnvironmentID   string `json:"environment_id"`
	EnvironmentName string `json:"environment_name"`
	// EnvironmentPreventSelfReview mirrors GitHub's own `prevent_self_review`
	// field on the environment's `required_reviewers` protection rule; nil
	// when the provider does not report it (dc-5: "the adapter discloses
	// that setting when the forge reports it").
	EnvironmentPreventSelfReview *bool `json:"environment_prevent_self_review"`

	GatedJobName       string `json:"gated_job_name"`
	GatedJobCount      int    `json:"gated_job_count"`
	GatedJobID         string `json:"gated_job_id"`
	GatedJobStatus     string `json:"gated_job_status"`
	GatedJobConclusion string `json:"gated_job_conclusion"`
	GatedJobCreatedAt  string `json:"gated_job_created_at"`
	GatedJobStartedAt  string `json:"gated_job_started_at"`

	Reviews []EnvironmentReviewRow `json:"reviews"`

	ObservedAt         string `json:"observed_at"`
	ProviderSnapshotID string `json:"provider_snapshot_id"`
}

// NewEnvironmentReviewFacts normalizes ordering and observation time,
// validates every supported fact, and derives a provider snapshot digest
// that deliberately excludes observation time — mirroring
// NewApprovalSnapshot's contract (ac-1) for this ac-4 fact source.
func NewEnvironmentReviewFacts(draft EnvironmentReviewFacts, observedAt time.Time) (EnvironmentReviewFacts, error) {
	stamp, err := NormalizeTimestamp(observedAt.Format(time.RFC3339Nano))
	if err != nil {
		return EnvironmentReviewFacts{}, fmt.Errorf("forge: environment review observed_at: %w", err)
	}
	facts := draft
	facts.ObservedAt = stamp

	rows := append([]EnvironmentReviewRow(nil), draft.Reviews...)
	if rows == nil {
		rows = []EnvironmentReviewRow{}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ReviewerActor.Subject != rows[j].ReviewerActor.Subject {
			return rows[i].ReviewerActor.Subject < rows[j].ReviewerActor.Subject
		}
		if rows[i].ProviderState != rows[j].ProviderState {
			return rows[i].ProviderState < rows[j].ProviderState
		}
		return rows[i].EnvironmentID < rows[j].EnvironmentID
	})
	facts.Reviews = rows

	facts.ProviderSnapshotID, err = facts.providerFactsDigest()
	if err != nil {
		return EnvironmentReviewFacts{}, fmt.Errorf("forge: environment review provider snapshot identity: %w", err)
	}
	if err := facts.Validate(); err != nil {
		return EnvironmentReviewFacts{}, err
	}
	return facts, nil
}

// Validate checks the closed provider-neutral environment-review contract.
func (f EnvironmentReviewFacts) Validate() error {
	if err := requireValue("environment_review.repository", f.Repository); err != nil {
		return err
	}
	if _, err := normalizedTime("forge: environment_review.observed_at", f.ObservedAt); err != nil {
		return err
	}
	if err := validateDigest(f.ProviderSnapshotID); err != nil {
		return err
	}
	if f.Reviews == nil {
		return fmt.Errorf("forge: environment_review.reviews must be non-null")
	}
	if !f.Supported {
		if err := f.validateUnsupported(); err != nil {
			return err
		}
	} else if err := f.validateSupported(); err != nil {
		return err
	}
	wantDigest, err := f.providerFactsDigest()
	if err != nil {
		return fmt.Errorf("forge: recompute environment review provider snapshot identity: %w", err)
	}
	if f.ProviderSnapshotID != wantDigest {
		return fmt.Errorf("forge: environment_review.provider_snapshot_id does not match normalized facts")
	}
	return nil
}

func (f EnvironmentReviewFacts) validateUnsupported() error {
	if err := requireValue("environment_review.unsupported_reason", f.UnsupportedReason); err != nil {
		return err
	}
	if f.RunID != "" || f.RunAttempt != 0 || f.LatestRunAttempt != 0 || f.RunHeadSHA != "" || f.RunURL != "" ||
		f.RunEvent != "" || f.RunWorkflowPath != "" || f.RunStatus != "" || f.RunConclusion != "" ||
		f.EnvironmentID != "" || f.EnvironmentName != "" || f.EnvironmentPreventSelfReview != nil ||
		f.GatedJobName != "" || f.GatedJobCount != 0 || f.GatedJobID != "" || f.GatedJobStatus != "" ||
		f.GatedJobConclusion != "" || f.GatedJobCreatedAt != "" || f.GatedJobStartedAt != "" ||
		len(f.Reviews) != 0 {
		return fmt.Errorf("forge: unsupported environment review facts must carry no run, job, environment, or review data")
	}
	return nil
}

func (f EnvironmentReviewFacts) validateSupported() error {
	if f.UnsupportedReason != "" {
		return fmt.Errorf("forge: supported environment review facts must carry no unsupported_reason")
	}
	if err := validateCanonicalID("environment_review.run_id", f.RunID); err != nil {
		return err
	}
	if f.RunAttempt < 1 {
		return fmt.Errorf("forge: environment_review.run_attempt must be at least 1, got %d", f.RunAttempt)
	}
	if f.LatestRunAttempt < 1 {
		return fmt.Errorf("forge: environment_review.latest_run_attempt must be at least 1, got %d", f.LatestRunAttempt)
	}
	if err := validateCandidateSHA("environment_review.run_head_sha", f.RunHeadSHA); err != nil {
		return err
	}
	for field, value := range map[string]string{
		"environment_review.run_url":           f.RunURL,
		"environment_review.run_event":         f.RunEvent,
		"environment_review.run_workflow_path": f.RunWorkflowPath,
		"environment_review.environment_name":  f.EnvironmentName,
		"environment_review.gated_job_name":    f.GatedJobName,
	} {
		if err := requireValue(field, value); err != nil {
			return err
		}
	}
	if f.RunStatus != "" && !workflowRunStatuses[f.RunStatus] {
		return fmt.Errorf("forge: environment_review.run_status: unknown workflow run status %q", f.RunStatus)
	}
	if f.RunConclusion != "" && !workflowRunConclusions[f.RunConclusion] {
		return fmt.Errorf("forge: environment_review.run_conclusion: unknown workflow run conclusion %q", f.RunConclusion)
	}
	if err := validateCanonicalID("environment_review.environment_id", f.EnvironmentID); err != nil {
		return err
	}
	if err := f.validateGatedJob(); err != nil {
		return err
	}
	return f.validateReviews()
}

func (f EnvironmentReviewFacts) validateGatedJob() error {
	if f.GatedJobCount < 0 {
		return fmt.Errorf("forge: environment_review.gated_job_count must not be negative, got %d", f.GatedJobCount)
	}
	if f.GatedJobCount != 1 {
		if f.GatedJobID != "" || f.GatedJobStatus != "" || f.GatedJobConclusion != "" || f.GatedJobCreatedAt != "" || f.GatedJobStartedAt != "" {
			return fmt.Errorf("forge: environment_review gated job details present although %d jobs carry the gated job's name", f.GatedJobCount)
		}
		return nil
	}
	if err := validateCanonicalID("environment_review.gated_job_id", f.GatedJobID); err != nil {
		return err
	}
	if !workflowJobStatuses[f.GatedJobStatus] {
		return fmt.Errorf("forge: environment_review.gated_job_status: unknown workflow job status %q", f.GatedJobStatus)
	}
	if f.GatedJobConclusion != "" && !workflowJobConclusions[f.GatedJobConclusion] {
		return fmt.Errorf("forge: environment_review.gated_job_conclusion: unknown workflow job conclusion %q", f.GatedJobConclusion)
	}
	if f.GatedJobCreatedAt != "" {
		if _, err := normalizedTime("forge: environment_review.gated_job_created_at", f.GatedJobCreatedAt); err != nil {
			return err
		}
	}
	if f.GatedJobStartedAt != "" {
		if _, err := normalizedTime("forge: environment_review.gated_job_started_at", f.GatedJobStartedAt); err != nil {
			return err
		}
	}
	return nil
}

func (f EnvironmentReviewFacts) validateReviews() error {
	seen := make(map[string]struct{}, len(f.Reviews))
	for i, row := range f.Reviews {
		prefix := fmt.Sprintf("forge: environment_review.reviews[%d]", i)
		if err := validateProviderActor(prefix, "github", row.ReviewerActor); err != nil {
			return err
		}
		if !row.ProviderState.valid() {
			return fmt.Errorf("%s.provider_state: unknown state %q", prefix, row.ProviderState)
		}
		if row.EnvironmentID != f.EnvironmentID {
			return fmt.Errorf("%s.environment_id %q is not the observed environment %q", prefix, row.EnvironmentID, f.EnvironmentID)
		}
		// Once a run has been rerun its history spans attempts, so the same
		// reviewer's same decision can legitimately recur (L2b review m-10);
		// the normalizer then refuses every row. While the latest attempt is
		// the first, a repeated (reviewer, state) pair would give two rows
		// one composite identity, which co-1 rejects.
		key := row.ReviewerActor.Subject + "\x00" + string(row.ProviderState)
		if _, exists := seen[key]; exists && f.LatestRunAttempt == 1 {
			return fmt.Errorf("%s: duplicate (reviewer, state) pair for reviewer %q state %q", prefix, row.ReviewerActor.Subject, row.ProviderState)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// providerFactsDigest is the digest of every provider fact, deliberately
// excluding the observation stamp and the digest itself.
func (f EnvironmentReviewFacts) providerFactsDigest() (string, error) {
	identity := f
	identity.ObservedAt = ""
	identity.ProviderSnapshotID = ""
	return canonjson.Digest(identity)
}

// NormalizeEnvironmentReview turns one workflow run attempt's
// environment-review facts into shared forge.Approval rows exactly per v2's
// mapping (ac-4, dc-5): only an approved review for facts' own environment,
// of a workflow_dispatch run of the close workflow that is neither cancelled
// nor failed, honored only while the run's first attempt is its latest, with
// exactly one gated job in progress or completed successfully, dated by that
// job's creation stamp — never its start — as a conservative lower bound.
//
// It first validates facts and returns an error for a value that breaks the
// facts contract (a hand-built or corrupted value is an operational defect,
// never a verdict), so a row is only ever derived from validated facts
// (L2b review m-3). For valid facts it is pure and deterministic: the same
// facts always produce the same rows and disclosure witnesses, and every
// adverse or unsupported shape is a zero-row-plus-disclosure outcome, never
// an error.
//
// The second return is the top-level disclosure witness list, since a
// refused row has no ProviderWitnesses to carry them: every reason no row
// was produced, in a fixed order, then one disclosure per rejected or
// pending review (L2b review m-5). It is nil when rows are produced and no
// review failed to approve.
func NormalizeEnvironmentReview(facts EnvironmentReviewFacts) ([]Approval, []string, error) {
	if err := facts.Validate(); err != nil {
		return nil, nil, fmt.Errorf("forge: normalize environment review: %w", err)
	}
	if !facts.Supported {
		return []Approval{}, []string{fmt.Sprintf(
			"environment-review:unsupported-forge: no rows produced: %s", facts.UnsupportedReason,
		)}, nil
	}

	refusals := environmentReviewRefusals(facts)
	var approved []EnvironmentReviewRow
	var reviewDisclosures []string
	for _, row := range facts.Reviews {
		if row.ProviderState == EnvironmentReviewApproved {
			approved = append(approved, row)
			continue
		}
		// A rejected or pending review is never an approval, and says so
		// (L2b review m-5), whether or not another reviewer approved.
		reviewDisclosures = append(reviewDisclosures, fmt.Sprintf(
			"environment-review:review-not-approved: run_id=%s run_attempt=%d environment_id=%s reviewer=%s state=%s: only an approved review is an approval",
			facts.RunID, facts.RunAttempt, row.EnvironmentID, row.ReviewerActor.Subject, row.ProviderState))
	}
	if len(approved) == 0 {
		refusals = append(refusals, noApprovedReviewDisclosure(facts))
	}
	if len(refusals) > 0 {
		return []Approval{}, append(refusals, reviewDisclosures...), nil
	}

	rows := make([]Approval, 0, len(approved))
	for _, row := range approved {
		rows = append(rows, environmentReviewApproval(facts, row))
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ApprovalID < rows[j].ApprovalID })
	return rows, reviewDisclosures, nil
}

// environmentReviewRefusals lists, in a fixed order, every v2 ac-4
// exclusion the run and its gated job fall under.
func environmentReviewRefusals(f EnvironmentReviewFacts) []string {
	subject := fmt.Sprintf("run_id=%s run_attempt=%d", f.RunID, f.RunAttempt)
	var out []string
	if f.LatestRunAttempt != 1 || f.RunAttempt != f.LatestRunAttempt {
		out = append(out, fmt.Sprintf(
			"environment-review:rerun-not-honored: %s latest_run_attempt=%d: only a run whose latest attempt is its first is honored, because GitHub's review history carries no attempt; a retry needs a new dispatch and a new review",
			subject, f.LatestRunAttempt))
	}
	if f.RunEvent != WorkflowDispatchEvent {
		out = append(out, fmt.Sprintf(
			"environment-review:not-workflow-dispatch: %s event=%s: only a %s run of the close workflow is honored",
			subject, f.RunEvent, WorkflowDispatchEvent))
	}
	if !IsCloseWorkflowPath(f.RunWorkflowPath) {
		out = append(out, fmt.Sprintf(
			"environment-review:not-close-workflow: %s workflow_path=%s: only the dispatch-only close workflow %s is honored",
			subject, f.RunWorkflowPath, CloseWorkflowPath))
	}
	if !workflowRunEligible(f.RunStatus, f.RunConclusion) {
		out = append(out, fmt.Sprintf(
			"environment-review:run-not-eligible: %s status=%s conclusion=%s: only a run that has not concluded, or concluded success, is honored; a cancelled or failed run yields no approval",
			subject, orNone(f.RunStatus), orNone(f.RunConclusion)))
	}
	gatedJob := fmt.Sprintf("%s gated_job=%s", subject, f.GatedJobName)
	switch {
	case f.GatedJobCount == 0:
		out = append(out, fmt.Sprintf(
			"environment-review:gated-job-not-found: %s: no job of this attempt carries the gated job's name, so no creation stamp dates the review",
			gatedJob))
	case f.GatedJobCount > 1:
		out = append(out, fmt.Sprintf(
			"environment-review:gated-job-ambiguous: %s count=%d: more than one job of this attempt carries the gated job's name, so no single creation stamp dates the review",
			gatedJob, f.GatedJobCount))
	default:
		gatedJob = fmt.Sprintf("%s gated_job_id=%s", gatedJob, f.GatedJobID)
		if !workflowJobEligible(f.GatedJobStatus, f.GatedJobConclusion) {
			out = append(out, fmt.Sprintf(
				"environment-review:gated-job-not-eligible: %s status=%s conclusion=%s: only a gated job in progress or completed successfully is honored",
				gatedJob, f.GatedJobStatus, orNone(f.GatedJobConclusion)))
		}
		out = append(out, stampRefusals(f, gatedJob)...)
	}
	return out
}

// stampRefusals refuses a row unless the gated job's creation stamp is a
// proven lower bound on the review (L2b review I-2): it must exist (SI-232),
// the start stamp must exist so the bound can be checked, and it must be
// neither later than the job's start — a job cannot start before its review,
// so a later creation stamp would understate approval age — nor later than
// the observation.
func stampRefusals(f EnvironmentReviewFacts, gatedJob string) []string {
	var out []string
	if f.GatedJobCreatedAt == "" {
		out = append(out, fmt.Sprintf(
			"environment-review:creation-stamp-unavailable: %s: the gated job carries no creation stamp, so approval age would be unproven, and the shared countersign types cannot express unproven without changing internal/countersign, so no row is produced (SI-232)",
			gatedJob))
	}
	if f.GatedJobStartedAt == "" {
		out = append(out, fmt.Sprintf(
			"environment-review:start-stamp-unavailable: %s: the gated job carries no start stamp, so its creation stamp cannot be proven not later than the review; no row is produced",
			gatedJob))
	}
	if f.GatedJobCreatedAt == "" {
		return out
	}
	if f.GatedJobStartedAt != "" && stampLater(f.GatedJobCreatedAt, f.GatedJobStartedAt) {
		out = append(out, fmt.Sprintf(
			"environment-review:creation-after-start: %s created_at=%s started_at=%s: a job cannot start before its review, so a creation stamp later than the start is not a lower bound on the review; no row is produced",
			gatedJob, f.GatedJobCreatedAt, f.GatedJobStartedAt))
	}
	if stampLater(f.GatedJobCreatedAt, f.ObservedAt) {
		out = append(out, fmt.Sprintf(
			"environment-review:creation-after-observation: %s created_at=%s observed_at=%s: a creation stamp later than the observation is not a lower bound on the review; no row is produced",
			gatedJob, f.GatedJobCreatedAt, f.ObservedAt))
	}
	return out
}

// stampLater reports whether stamp a is later than stamp b. Validate has
// already proved both are normalized RFC3339Nano; were either unparseable,
// it reports true, so the row is refused rather than admitted.
func stampLater(a, b string) bool {
	left, errLeft := time.Parse(time.RFC3339Nano, a)
	right, errRight := time.Parse(time.RFC3339Nano, b)
	return errLeft != nil || errRight != nil || left.After(right)
}

// noApprovedReviewDisclosure says why a run with no approved entry for the
// environment yields no row, including the bypass case: an administrator
// who bypasses the protection rule leaves no approved review entry at all
// (L2b review I-3).
func noApprovedReviewDisclosure(f EnvironmentReviewFacts) string {
	var rejected, pending int
	for _, row := range f.Reviews {
		switch row.ProviderState {
		case EnvironmentReviewRejected:
			rejected++
		case EnvironmentReviewPending:
			pending++
		}
	}
	return fmt.Sprintf(
		"environment-review:no-approved-review: run_id=%s run_attempt=%d environment=%s environment_id=%s reviews=%d rejected=%d pending=%d: the review history holds no approved entry for this environment; a rejected, pending, or absent review, or a protection rule an administrator bypassed, leaves none, so no row is produced",
		f.RunID, f.RunAttempt, f.EnvironmentName, f.EnvironmentID, len(f.Reviews), rejected, pending)
}

// workflowRunEligible reports whether a run is neither cancelled nor
// failed: it has not concluded, or it concluded success. Every other
// documented conclusion (failure, cancelled, timed_out, startup_failure,
// action_required, neutral, skipped, stale) is not an unambiguous success,
// and a run with no reported status is unproven, so both refuse.
func workflowRunEligible(status, conclusion string) bool {
	if conclusion == "" {
		return status != "" && status != "completed"
	}
	return status == "completed" && conclusion == "success"
}

// workflowJobEligible reports whether the gated job is in progress or
// completed successfully (L2b review I-3).
func workflowJobEligible(status, conclusion string) bool {
	return (status == "in_progress" && conclusion == "") || (status == "completed" && conclusion == "success")
}

func orNone(value string) string {
	if value == "" {
		return "none"
	}
	return value
}

// environmentReviewApproval maps one approved review of eligible facts to
// the shared row (v2 ac-4 body "Environment review rows").
func environmentReviewApproval(facts EnvironmentReviewFacts, row EnvironmentReviewRow) Approval {
	approvalID := fmt.Sprintf("github-environment-review:%s:%s:%d:%s:%s",
		facts.Repository, facts.RunID, facts.RunAttempt, row.EnvironmentID, row.ReviewerActor.Subject)
	approvalRef := fmt.Sprintf("%s environment=%s", facts.RunURL, facts.EnvironmentName)

	witnesses := []ProviderWitness{
		{Name: "provider_state", Value: string(row.ProviderState)},
		{Name: "actor_user_id", Value: row.ReviewerActor.Subject},
		{Name: "run_id", Value: facts.RunID},
		{Name: "run_attempt", Value: strconv.Itoa(facts.RunAttempt)},
		{Name: "run_latest_attempt", Value: strconv.Itoa(facts.LatestRunAttempt)},
		{Name: "run_head_sha", Value: facts.RunHeadSHA},
		{Name: "run_url", Value: facts.RunURL},
		{Name: "run_event", Value: facts.RunEvent},
		{Name: "run_workflow_path", Value: facts.RunWorkflowPath},
		{Name: "run_status", Value: facts.RunStatus},
		{Name: "environment_id", Value: row.EnvironmentID},
		{Name: "environment_name", Value: facts.EnvironmentName},
		{Name: "gated_job_name", Value: facts.GatedJobName},
		{Name: "gated_job_id", Value: facts.GatedJobID},
		{Name: "gated_job_status", Value: facts.GatedJobStatus},
		{Name: "gated_job_created_at", Value: facts.GatedJobCreatedAt},
		// Every derived field is disclosed (v2 dc-5; L2b review m-5).
		{Name: "approval_id_derivation", Value: "composite of repository, run id, run attempt, environment id, and reviewer id (github's review history carries no review id)"},
		{Name: "approval_ref_derivation", Value: "the run's url and the environment's name (github's review history carries no review url)"},
		{Name: "approved_at_derivation", Value: "the gated job's creation stamp: a conservative lower bound, since github's review history carries no review time; the job's start stamp is recorded separately below, never as the approval instant"},
		{Name: "updated_at_derivation", Value: "equal to approved_at: github's review history records no review update time"},
		{Name: "state_derivation", Value: "github's approved review state normalized to the shared active state"},
		{Name: "candidate_sha_derivation", Value: "the run's head commit: an environment review approves one exact workflow run"},
	}
	if facts.RunConclusion != "" {
		witnesses = append(witnesses, ProviderWitness{Name: "run_conclusion", Value: facts.RunConclusion})
	}
	if facts.GatedJobConclusion != "" {
		witnesses = append(witnesses, ProviderWitness{Name: "gated_job_conclusion", Value: facts.GatedJobConclusion})
	}
	if facts.GatedJobStartedAt != "" {
		witnesses = append(witnesses, ProviderWitness{Name: "gated_job_started_at", Value: facts.GatedJobStartedAt})
	}
	if facts.EnvironmentPreventSelfReview != nil {
		witnesses = append(witnesses, ProviderWitness{
			Name: "environment_prevent_self_review", Value: strconv.FormatBool(*facts.EnvironmentPreventSelfReview),
		})
	}
	sort.Slice(witnesses, func(i, j int) bool { return witnesses[i].Name < witnesses[j].Name })

	return Approval{
		ApprovalID: approvalID, ApprovalRef: approvalRef, State: ApprovalActive,
		ApprovedAt: facts.GatedJobCreatedAt, UpdatedAt: facts.GatedJobCreatedAt,
		CandidateSHA: facts.RunHeadSHA, Actor: row.ReviewerActor,
		ProviderWitnesses: witnesses,
	}
}

// validateCanonicalID requires a provider numeric id in canonical positive
// base-10 form, so one provider object has exactly one textual identity
// (L2b review m-1: "+555", "0555", and "555" must not name three reviews).
func validateCanonicalID(field, value string) error {
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 || strconv.FormatInt(number, 10) != value {
		return fmt.Errorf("forge: %s must be a canonical positive base-10 provider id, got %q", field, value)
	}
	return nil
}
