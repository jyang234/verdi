// Package fake is a hermetic, in-memory Forge double (04 §Testing's
// pattern applied to the I-22 forge port): no HTTP, no network, used by
// `verdi sync`'s own tests and anywhere else a Forge is needed without a
// real GitLab/GitHub server.
package fake

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jyang234/verdi/internal/forge"
)

// Forge is a configurable, in-memory forge.Forge.
type Forge struct {
	mu sync.Mutex

	bundles   map[string]forge.DerivedTree
	attribute string
	ci        forge.CIInfo
	openMRs   map[string][]forge.OpenMR    // targetBranch -> open MRs
	files     map[string]map[string][]byte // branch -> path -> content

	comments     map[string][]forge.Comment              // mrID -> comment feed
	threads      map[string][]forge.ThreadResolution     // mrID -> thread resolutions
	approvals    map[string]forge.ApprovalSnapshot       // changeID -> current facts
	envReviews   map[string]forge.EnvironmentReviewFacts // query key -> seeded facts
	mergeRecords map[string]forge.MergeRecordFacts       // commit -> seeded facts
	nextCommID   int
}

// New returns an empty Forge: no bundles seeded, GeneratedAttribute
// returns "fake-generated", CIContext returns a zero CIInfo, no open MRs.
func New() *Forge {
	return &Forge{
		bundles:      make(map[string]forge.DerivedTree),
		attribute:    "fake-generated",
		openMRs:      make(map[string][]forge.OpenMR),
		files:        make(map[string]map[string][]byte),
		comments:     make(map[string][]forge.Comment),
		threads:      make(map[string][]forge.ThreadResolution),
		approvals:    make(map[string]forge.ApprovalSnapshot),
		envReviews:   make(map[string]forge.EnvironmentReviewFacts),
		mergeRecords: make(map[string]forge.MergeRecordFacts),
		nextCommID:   1,
	}
}

// SeedApprovalSnapshot makes ListApprovals return snapshot for changeID.
func (f *Forge) SeedApprovalSnapshot(changeID string, snapshot forge.ApprovalSnapshot) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.approvals[changeID] = cloneApprovalSnapshot(snapshot)
}

// ListApprovals implements forge.Forge.
func (f *Forge) ListApprovals(ctx context.Context, changeID string) (forge.ApprovalSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return forge.ApprovalSnapshot{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	snapshot, ok := f.approvals[changeID]
	if !ok {
		return forge.ApprovalSnapshot{}, fmt.Errorf("fake: no approval snapshot seeded for change %q", changeID)
	}
	return cloneApprovalSnapshot(snapshot), nil
}

func cloneApprovalSnapshot(snapshot forge.ApprovalSnapshot) forge.ApprovalSnapshot {
	snapshot.Approvals = append([]forge.Approval(nil), snapshot.Approvals...)
	for i := range snapshot.Approvals {
		if snapshot.Approvals[i].ProviderWitnesses != nil {
			snapshot.Approvals[i].ProviderWitnesses = append([]forge.ProviderWitness(nil), snapshot.Approvals[i].ProviderWitnesses...)
		}
	}
	return snapshot
}

// SeedEnvironmentReviewFacts makes EnvironmentReview(query) return facts for
// the exact query tuple (run id, run attempt, environment name, gated job
// name) — mirroring SeedApprovalSnapshot's seed-or-error pattern (v2 ac-4).
// It refuses facts that break the facts contract, and supported facts that
// answer a different run, attempt, environment, or gated job than query, so
// the fake can never hand a consumer an observation a real adapter could not
// produce (L2b review m-3).
func (f *Forge) SeedEnvironmentReviewFacts(query forge.EnvironmentReviewQuery, facts forge.EnvironmentReviewFacts) error {
	if err := facts.Validate(); err != nil {
		return fmt.Errorf("fake: seed environment review facts: %w", err)
	}
	if facts.Supported && !environmentReviewFactsAnswer(query, facts) {
		return fmt.Errorf("fake: seeded environment review facts (run %q attempt %d environment %q gated job %q) do not answer query run %q attempt %d environment %q gated job %q",
			facts.RunID, facts.RunAttempt, facts.EnvironmentName, facts.GatedJobName, query.RunID, query.RunAttempt, query.EnvironmentName, query.GatedJobName)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envReviews[environmentReviewKey(query)] = cloneEnvironmentReviewFacts(facts)
	return nil
}

// environmentReviewFactsAnswer reports whether supported facts observe the
// run, attempt, environment, and gated job query names (GitHub environment
// names are case-insensitive).
func environmentReviewFactsAnswer(query forge.EnvironmentReviewQuery, facts forge.EnvironmentReviewFacts) bool {
	return facts.RunID == query.RunID && facts.RunAttempt == query.RunAttempt &&
		strings.EqualFold(facts.EnvironmentName, query.EnvironmentName) && facts.GatedJobName == query.GatedJobName
}

// EnvironmentReview implements forge.Forge.
func (f *Forge) EnvironmentReview(ctx context.Context, query forge.EnvironmentReviewQuery) (forge.EnvironmentReviewFacts, error) {
	if err := ctx.Err(); err != nil {
		return forge.EnvironmentReviewFacts{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	facts, ok := f.envReviews[environmentReviewKey(query)]
	if !ok {
		return forge.EnvironmentReviewFacts{}, fmt.Errorf("fake: no environment review facts seeded for run %q attempt %d environment %q gated job %q", query.RunID, query.RunAttempt, query.EnvironmentName, query.GatedJobName)
	}
	return cloneEnvironmentReviewFacts(facts), nil
}

func environmentReviewKey(q forge.EnvironmentReviewQuery) string {
	return fmt.Sprintf("%s\x00%d\x00%s\x00%s", q.RunID, q.RunAttempt, q.EnvironmentName, q.GatedJobName)
}

func cloneEnvironmentReviewFacts(facts forge.EnvironmentReviewFacts) forge.EnvironmentReviewFacts {
	facts.Reviews = append([]forge.EnvironmentReviewRow(nil), facts.Reviews...)
	if facts.EnvironmentPreventSelfReview != nil {
		v := *facts.EnvironmentPreventSelfReview
		facts.EnvironmentPreventSelfReview = &v
	}
	return facts
}

// SeedMergeRecordFacts makes MergeRecords(commit) return facts (SI-249; plan
// R-PB-2), mirroring SeedEnvironmentReviewFacts' seed-or-error pattern. It
// refuses facts that break the facts contract and facts observing another
// commit than the one they are seeded for, so the fake can never hand a
// consumer an observation a real adapter could not produce.
func (f *Forge) SeedMergeRecordFacts(commit string, facts forge.MergeRecordFacts) error {
	if err := facts.Validate(); err != nil {
		return fmt.Errorf("fake: seed merge record facts: %w", err)
	}
	if facts.Commit != commit {
		return fmt.Errorf("fake: seeded merge record facts observe commit %q, not %q", facts.Commit, commit)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mergeRecords[commit] = cloneMergeRecordFacts(facts)
	return nil
}

// MergeRecords implements forge.Forge: a clone of the facts seeded for
// commit, or an error for a commit nothing was seeded for.
func (f *Forge) MergeRecords(ctx context.Context, commit string) (forge.MergeRecordFacts, error) {
	if err := ctx.Err(); err != nil {
		return forge.MergeRecordFacts{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	facts, ok := f.mergeRecords[commit]
	if !ok {
		return forge.MergeRecordFacts{}, fmt.Errorf("fake: no merge record facts seeded for commit %q", commit)
	}
	return cloneMergeRecordFacts(facts), nil
}

func cloneMergeRecordFacts(facts forge.MergeRecordFacts) forge.MergeRecordFacts {
	facts.Changes = append([]forge.ChangeRequestMerge{}, facts.Changes...)
	return facts
}

func bundleKey(ref, commit string) string { return ref + "@" + commit }

// SeedBundle makes FetchEvidenceBundle(ref, commit) succeed with tree — the
// derived subtree a real CI run would have uploaded for (ref, commit),
// keyed by path relative to data/derived/.
func (f *Forge) SeedBundle(ref, commit string, tree forge.DerivedTree) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles[bundleKey(ref, commit)] = tree
}

// SetGeneratedAttribute overrides GeneratedAttribute's return value.
func (f *Forge) SetGeneratedAttribute(attr string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.attribute = attr
}

// SetCIContext overrides CIContext's return value.
func (f *Forge) SetCIContext(info forge.CIInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.ci = info
}

// FetchEvidenceBundle implements forge.Forge.
func (f *Forge) FetchEvidenceBundle(ctx context.Context, ref, commit string) (forge.DerivedTree, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	tree, ok := f.bundles[bundleKey(ref, commit)]
	if !ok {
		return nil, fmt.Errorf("fake: no bundle seeded for ref %q commit %q: %w", ref, commit, forge.ErrNoBundle)
	}
	// Return an independent copy so a caller mutating the map cannot
	// corrupt the fake's seeded state.
	out := make(forge.DerivedTree, len(tree))
	for k, v := range tree {
		out[k] = v
	}
	return out, nil
}

// GeneratedAttribute implements forge.Forge.
func (f *Forge) GeneratedAttribute() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attribute
}

// CIContext implements forge.Forge.
func (f *Forge) CIContext(ctx context.Context) (forge.CIInfo, error) {
	if err := ctx.Err(); err != nil {
		return forge.CIInfo{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.ci, nil
}

// SeedOpenMR registers mr as open against targetBranch, so
// ListOpenMRs(ctx, targetBranch) returns it.
func (f *Forge) SeedOpenMR(targetBranch string, mr forge.OpenMR) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.openMRs[targetBranch] = append(f.openMRs[targetBranch], mr)
}

// SeedFile makes FetchFileAtRef(ref, path) succeed with content.
func (f *Forge) SeedFile(ref, path string, content []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.files[ref] == nil {
		f.files[ref] = make(map[string][]byte)
	}
	f.files[ref][path] = content
}

// ListOpenMRs implements forge.Forge.
func (f *Forge) ListOpenMRs(ctx context.Context, targetBranch string) ([]forge.OpenMR, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]forge.OpenMR, len(f.openMRs[targetBranch]))
	copy(out, f.openMRs[targetBranch])
	return out, nil
}

// FetchFileAtRef implements forge.Forge.
func (f *Forge) FetchFileAtRef(ctx context.Context, ref, path string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	byPath, ok := f.files[ref]
	if !ok {
		return nil, fmt.Errorf("fake: no files seeded for ref %q: %w", ref, forge.ErrFileNotFound)
	}
	content, ok := byPath[path]
	if !ok {
		return nil, fmt.Errorf("fake: no file %q seeded at ref %q: %w", path, ref, forge.ErrFileNotFound)
	}
	return content, nil
}

// SeedComment registers c as already present in mrID's comment feed
// (ListComments). If c.ThreadID is non-empty and no ThreadResolution has
// been seeded for it yet, an unresolved entry is created automatically —
// mirroring both real forges, where a diff-anchored comment always
// belongs to a thread that exists (unresolved) from the moment it is
// created.
func (f *Forge) SeedComment(mrID string, c forge.Comment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.comments[mrID] = append(f.comments[mrID], c)
	if c.ThreadID == "" {
		return
	}
	for _, tr := range f.threads[mrID] {
		if tr.ThreadID == c.ThreadID {
			return
		}
	}
	f.threads[mrID] = append(f.threads[mrID], forge.ThreadResolution{ThreadID: c.ThreadID})
}

// SeedThreadResolution sets threadID's resolution state on mrID directly
// (overwriting any auto-created entry from SeedComment).
func (f *Forge) SeedThreadResolution(mrID string, tr forge.ThreadResolution) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, existing := range f.threads[mrID] {
		if existing.ThreadID == tr.ThreadID {
			f.threads[mrID][i] = tr
			return
		}
	}
	f.threads[mrID] = append(f.threads[mrID], tr)
}

// ListComments implements forge.Forge: the full seeded feed for mrID,
// unfiltered (never dropping an unanchored comment — 05 §Review stickies
// and forge round-trip's inbox-tray guarantee starts at the port).
func (f *Forge) ListComments(ctx context.Context, mrID string) ([]forge.Comment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]forge.Comment, len(f.comments[mrID]))
	copy(out, f.comments[mrID])
	return out, nil
}

// PostComment implements forge.Forge: appends a new comment (general if
// target is nil, diff-anchored — and belonging to a freshly minted,
// unresolved thread — otherwise) and returns it.
func (f *Forge) PostComment(ctx context.Context, mrID, body string, target *forge.CommentTarget) (forge.Comment, error) {
	if err := ctx.Err(); err != nil {
		return forge.Comment{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	id := fmt.Sprintf("fake-comment-%d", f.nextCommID)
	f.nextCommID++
	c := forge.Comment{ID: id, Body: body, Author: "fake-user"}
	if target != nil {
		c.Path = target.Path
		c.Line = target.Line
		c.ThreadID = "fake-thread-" + id
		f.threads[mrID] = append(f.threads[mrID], forge.ThreadResolution{ThreadID: c.ThreadID})
	}
	f.comments[mrID] = append(f.comments[mrID], c)
	return c, nil
}

// GetThreadResolution implements forge.Forge.
func (f *Forge) GetThreadResolution(ctx context.Context, mrID string) ([]forge.ThreadResolution, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]forge.ThreadResolution, len(f.threads[mrID]))
	copy(out, f.threads[mrID])
	return out, nil
}

var _ forge.Forge = (*Forge)(nil)
