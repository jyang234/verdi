package forge

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
type EnvironmentReviewRow struct {
	ReviewerActor ProviderActor          `json:"reviewer_actor"`
	ProviderState EnvironmentReviewState `json:"provider_state"`
}

// EnvironmentReviewFacts is one forge observation of one workflow run
// attempt's environment-review facts (v2 ac-4, dc-5) — provider facts, not
// yet a countersign judgment (AC-1's port/judgment split applies here too).
//
// Supported is false for a forge with no environment-review concept at all
// (GitLab, co-1's "unsupported source"): every run/job/environment/review
// field must then be its zero value and UnsupportedReason names why —
// NormalizeEnvironmentReview turns that into a disclosed zero-row result,
// never an error that would break a close.
//
// Times are normalized UTC RFC3339Nano. GatedJobCreatedAt/GatedJobStartedAt
// are "" when the provider does not report them for this attempt's gated
// job (dc-5: "when the creation stamp is unavailable"); GatedJobFound
// records whether a job named the query's GatedJobName was found in this
// attempt's job list at all (false covers, among other things, a run
// cancelled before the gated job was ever created).
type EnvironmentReviewFacts struct {
	Supported         bool   `json:"supported"`
	UnsupportedReason string `json:"unsupported_reason"`

	Repository      string `json:"repository"`
	RunID           string `json:"run_id"`
	RunAttempt      int    `json:"run_attempt"`
	RunHeadSHA      string `json:"run_head_sha"`
	RunURL          string `json:"run_url"`
	EnvironmentID   string `json:"environment_id"`
	EnvironmentName string `json:"environment_name"`

	GatedJobFound     bool   `json:"gated_job_found"`
	GatedJobCreatedAt string `json:"gated_job_created_at"`
	GatedJobStartedAt string `json:"gated_job_started_at"`

	// EnvironmentPreventSelfReview mirrors GitHub's own `prevent_self_review`
	// field on the environment's `required_reviewers` protection rule; nil
	// when the provider does not report it (dc-5: "the adapter discloses
	// that setting when the forge reports it").
	EnvironmentPreventSelfReview *bool `json:"environment_prevent_self_review"`

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
		return rows[i].ProviderState < rows[j].ProviderState
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
		if err := requireValue("environment_review.unsupported_reason", f.UnsupportedReason); err != nil {
			return err
		}
		if f.RunID != "" || f.RunAttempt != 0 || f.RunHeadSHA != "" || f.RunURL != "" ||
			f.EnvironmentID != "" || f.EnvironmentName != "" || f.GatedJobFound ||
			f.GatedJobCreatedAt != "" || f.GatedJobStartedAt != "" ||
			f.EnvironmentPreventSelfReview != nil || len(f.Reviews) != 0 {
			return fmt.Errorf("forge: unsupported environment review facts must carry no run, job, environment, or review data")
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

	if f.UnsupportedReason != "" {
		return fmt.Errorf("forge: supported environment review facts must carry no unsupported_reason")
	}
	if err := requireValue("environment_review.run_id", f.RunID); err != nil {
		return err
	}
	if f.RunAttempt < 1 {
		return fmt.Errorf("forge: environment_review.run_attempt must be at least 1, got %d", f.RunAttempt)
	}
	if err := validateCandidateSHA("environment_review.run_head_sha", f.RunHeadSHA); err != nil {
		return err
	}
	if err := requireValue("environment_review.run_url", f.RunURL); err != nil {
		return err
	}
	if err := requireValue("environment_review.environment_id", f.EnvironmentID); err != nil {
		return err
	}
	if err := requireValue("environment_review.environment_name", f.EnvironmentName); err != nil {
		return err
	}
	if !f.GatedJobFound && (f.GatedJobCreatedAt != "" || f.GatedJobStartedAt != "") {
		return fmt.Errorf("forge: environment_review gated job stamps present without gated_job_found")
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

	seen := make(map[string]struct{}, len(f.Reviews))
	for i, row := range f.Reviews {
		prefix := fmt.Sprintf("forge: environment_review.reviews[%d]", i)
		if err := validateProviderActor(prefix, "github", row.ReviewerActor); err != nil {
			return err
		}
		if !row.ProviderState.valid() {
			return fmt.Errorf("%s.provider_state: unknown state %q", prefix, row.ProviderState)
		}
		key := row.ReviewerActor.Subject + "\x00" + string(row.ProviderState)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%s: duplicate (reviewer, state) pair for reviewer %q state %q", prefix, row.ReviewerActor.Subject, row.ProviderState)
		}
		seen[key] = struct{}{}
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

func (f EnvironmentReviewFacts) providerFactsDigest() (string, error) {
	identity := struct {
		Supported                    bool                   `json:"supported"`
		UnsupportedReason            string                 `json:"unsupported_reason"`
		Repository                   string                 `json:"repository"`
		RunID                        string                 `json:"run_id"`
		RunAttempt                   int                    `json:"run_attempt"`
		RunHeadSHA                   string                 `json:"run_head_sha"`
		RunURL                       string                 `json:"run_url"`
		EnvironmentID                string                 `json:"environment_id"`
		EnvironmentName              string                 `json:"environment_name"`
		GatedJobFound                bool                   `json:"gated_job_found"`
		GatedJobCreatedAt            string                 `json:"gated_job_created_at"`
		GatedJobStartedAt            string                 `json:"gated_job_started_at"`
		EnvironmentPreventSelfReview *bool                  `json:"environment_prevent_self_review"`
		Reviews                      []EnvironmentReviewRow `json:"reviews"`
	}{
		f.Supported, f.UnsupportedReason, f.Repository, f.RunID, f.RunAttempt, f.RunHeadSHA, f.RunURL,
		f.EnvironmentID, f.EnvironmentName, f.GatedJobFound, f.GatedJobCreatedAt, f.GatedJobStartedAt,
		f.EnvironmentPreventSelfReview, f.Reviews,
	}
	return canonjson.Digest(identity)
}

// NormalizeEnvironmentReview turns one workflow run attempt's
// environment-review facts into shared forge.Approval rows exactly per v2's
// mapping (ac-4, dc-5): only an approved review for facts' own environment,
// honored only on the run's first attempt, dated by the gated job's
// creation stamp — never its start — as a conservative lower bound.
//
// It is pure and deterministic: the same facts always produce the same
// rows and disclosure witnesses, with no error return — every adverse or
// unsupported input shape (an unsupported forge, a rerun, a rejected or
// pending or absent review, a missing creation stamp) is a legitimate
// zero-row-plus-disclosure outcome, never a defect. A malformed facts
// value is rejected earlier, by NewEnvironmentReviewFacts's own Validate.
//
// The second return is the top-level disclosure witness list — reasons no
// row was produced that a row's own ProviderWitnesses cannot carry, since
// there is no row to attach them to. It is nil when every row that would
// otherwise be suppressed simply never existed (no approved review at
// all), which needs no witness of its own.
func NormalizeEnvironmentReview(facts EnvironmentReviewFacts) ([]Approval, []string) {
	if !facts.Supported {
		return []Approval{}, []string{fmt.Sprintf(
			"environment-review:unsupported-forge: no rows produced: %s", facts.UnsupportedReason,
		)}
	}
	if facts.RunAttempt != 1 {
		return []Approval{}, []string{fmt.Sprintf(
			"environment-review:rerun-not-honored: run_id=%s run_attempt=%d: reruns are not honored, a retry needs a new dispatch and a new review",
			facts.RunID, facts.RunAttempt,
		)}
	}

	var approved []EnvironmentReviewRow
	for _, row := range facts.Reviews {
		if row.ProviderState == EnvironmentReviewApproved {
			approved = append(approved, row)
		}
	}
	if len(approved) == 0 {
		return []Approval{}, nil
	}

	if facts.GatedJobCreatedAt == "" {
		reason := "gated job creation stamp unavailable"
		if !facts.GatedJobFound {
			reason = "gated job not found in this run attempt's job list"
		}
		return []Approval{}, []string{fmt.Sprintf(
			"environment-review:creation-stamp-unavailable: run_id=%s run_attempt=%d: %s: no row produced (approval age would be unproven, and the shared countersign types cannot express unproven without changing internal/countersign, so this fails closed)",
			facts.RunID, facts.RunAttempt, reason,
		)}
	}

	rows := make([]Approval, 0, len(approved))
	for _, row := range approved {
		approvalID := fmt.Sprintf("github-environment-review:%s:%s:%d:%s:%s",
			facts.Repository, facts.RunID, facts.RunAttempt, facts.EnvironmentID, row.ReviewerActor.Subject)
		approvalRef := fmt.Sprintf("%s environment=%s", facts.RunURL, facts.EnvironmentName)

		witnesses := []ProviderWitness{
			{Name: "provider_state", Value: string(row.ProviderState)},
			{Name: "actor_user_id", Value: row.ReviewerActor.Subject},
			{Name: "run_id", Value: facts.RunID},
			{Name: "run_attempt", Value: "1"},
			{Name: "run_head_sha", Value: facts.RunHeadSHA},
			{Name: "run_url", Value: facts.RunURL},
			{Name: "environment_id", Value: facts.EnvironmentID},
			{Name: "environment_name", Value: facts.EnvironmentName},
			{Name: "gated_job_created_at", Value: facts.GatedJobCreatedAt},
			{Name: "approval_id_derivation", Value: "composite of repository, run id, run attempt, environment id, and reviewer id (github's review history carries no review id)"},
			{Name: "approved_at_derivation", Value: "the gated job's creation stamp: a conservative lower bound, since github's review history carries no review time; the job's start stamp is recorded separately below, never as the approval instant"},
			{Name: "state_derivation", Value: "github's approved review state normalized to the shared active state"},
		}
		if facts.GatedJobStartedAt != "" {
			witnesses = append(witnesses, ProviderWitness{Name: "gated_job_started_at", Value: facts.GatedJobStartedAt})
		}
		if facts.EnvironmentPreventSelfReview != nil {
			witnesses = append(witnesses, ProviderWitness{
				Name: "environment_prevent_self_review", Value: strconv.FormatBool(*facts.EnvironmentPreventSelfReview),
			})
		}

		rows = append(rows, Approval{
			ApprovalID: approvalID, ApprovalRef: approvalRef, State: ApprovalActive,
			ApprovedAt: facts.GatedJobCreatedAt, UpdatedAt: facts.GatedJobCreatedAt,
			CandidateSHA: facts.RunHeadSHA, Actor: row.ReviewerActor,
			ProviderWitnesses: witnesses,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ApprovalID < rows[j].ApprovalID })
	return rows, nil
}
