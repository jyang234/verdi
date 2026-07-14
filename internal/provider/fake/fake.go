package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/jyang234/verdi/internal/provider"
)

// Comment is one human-comment side effect a PublishRollup call fired,
// in the shape 04 §Jira adapter describes ("the criteria table plus a
// link to the MR/pipeline"). The fake records enough to let tests assert
// a comment fired and inspect its content; it makes no attempt at Jira's
// actual markup.
type Comment struct {
	Rollup provider.Rollup
}

// Provider is a configurable, in-memory StoryProvider (04 §Testing). It:
//   - resolves only refs seeded via SeedStory, returning a wrapped
//     provider.ErrNotFound for anything else (or an injected failure);
//   - records every PublishRollup call, keyed by (story, commit), so
//     republishing the same key updates the existing record rather than
//     creating a new one (idempotency);
//   - fires a Comment only when a publish's AC statuses differ from the
//     story's previously published statuses (comment-only-on-change),
//     read back from its own last-published state — never from Resolve;
//   - can be told to fail its next Resolve or PublishRollup call for a
//     given ref, to simulate 04's failure taxonomy on demand.
//
// Provider is safe for concurrent use.
type Provider struct {
	mu sync.Mutex

	stories     map[provider.StoryRef]provider.Story
	resolveErrs map[provider.StoryRef]error
	publishErrs map[provider.StoryRef][]error // queued, consumed FIFO, one per call

	records  map[provider.StoryRef]map[string]provider.Rollup // story -> commit -> rollup last published under that commit
	latest   map[provider.StoryRef]provider.Rollup            // story -> most recently published rollup, any commit
	comments map[provider.StoryRef][]Comment
}

// New returns an empty fake Provider: no stories seeded, nothing
// published.
func New() *Provider {
	return &Provider{
		stories:     make(map[provider.StoryRef]provider.Story),
		resolveErrs: make(map[provider.StoryRef]error),
		publishErrs: make(map[provider.StoryRef][]error),
		records:     make(map[provider.StoryRef]map[string]provider.Rollup),
		latest:      make(map[provider.StoryRef]provider.Rollup),
		comments:    make(map[provider.StoryRef][]Comment),
	}
}

// SeedStory makes story.Ref resolve to story.
func (p *Provider) SeedStory(story provider.Story) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stories[story.Ref] = story
}

// FailResolve makes every subsequent Resolve(ref) call return err, until
// ClearResolveFailure is called. Pass an error wrapping
// provider.ErrNotFound, provider.ErrUnauthorized, or
// provider.ErrUnavailable to simulate 04's failure taxonomy.
func (p *Provider) FailResolve(ref provider.StoryRef, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resolveErrs[ref] = err
}

// ClearResolveFailure removes an injected Resolve failure for ref.
func (p *Provider) ClearResolveFailure(ref provider.StoryRef) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.resolveErrs, ref)
}

// QueuePublishError makes the next call to PublishRollup for story
// return err instead of recording the publish; it is consumed after one
// use. Call it more than once to queue multiple consecutive failures.
func (p *Provider) QueuePublishError(story provider.StoryRef, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishErrs[story] = append(p.publishErrs[story], err)
}

// Resolve implements provider.StoryProvider.
func (p *Provider) Resolve(ctx context.Context, ref provider.StoryRef) (provider.Story, error) {
	if err := ctx.Err(); err != nil {
		return provider.Story{}, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if err, ok := p.resolveErrs[ref]; ok {
		return provider.Story{}, err
	}
	story, ok := p.stories[ref]
	if !ok {
		return provider.Story{}, fmt.Errorf("fake: resolve %s: %w", ref, provider.ErrNotFound)
	}
	return story, nil
}

// PublishRollup implements provider.StoryProvider.
func (p *Provider) PublishRollup(ctx context.Context, r provider.Rollup) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if queue := p.publishErrs[r.Story]; len(queue) > 0 {
		err := queue[0]
		p.publishErrs[r.Story] = queue[1:]
		return err
	}

	if p.records[r.Story] == nil {
		p.records[r.Story] = make(map[string]provider.Rollup)
	}
	p.records[r.Story][r.Commit] = r

	prev, hadPrev := p.latest[r.Story]
	changed := !hadPrev || criteriaStatusesChanged(prev.Criteria, r.Criteria)
	p.latest[r.Story] = r
	if changed {
		p.comments[r.Story] = append(p.comments[r.Story], Comment{Rollup: r})
	}
	return nil
}

// criteriaStatusesChanged reports whether the per-AC Status values differ
// between a and b, comparing by ID (order-independent, per 04
// §Semantics's "any AC status changed since the last publish"). A
// criterion appearing in one set but not the other counts as a change.
// Thin wrapper over provider.StatusesChanged (spec/shared-homes ac-5) —
// see there for the shared comparison this package's fake and the jira
// adapter both call with their own projection.
func criteriaStatusesChanged(a, b []provider.CriterionStatus) bool {
	return provider.StatusesChanged(a, b, func(c provider.CriterionStatus) (id, status string) {
		return c.ID, c.Status
	})
}

// PublishedField returns the most recently published rollup for story
// (the fake's equivalent of an adapter's own machine field), and whether
// anything has been published yet.
func (p *Provider) PublishedField(story provider.StoryRef) (provider.Rollup, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	r, ok := p.latest[story]
	return r, ok
}

// PublishRecordCount returns how many distinct commits have been
// published for story. Republishing the same (story, commit) does not
// increase it — the contract suite's idempotency assertion.
func (p *Provider) PublishRecordCount(story provider.StoryRef) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.records[story])
}

// CommentCount returns how many human comments have fired for story
// since the Provider was created.
func (p *Provider) CommentCount(story provider.StoryRef) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.comments[story])
}

// Comments returns a copy of every human comment fired for story, in
// publish order.
func (p *Provider) Comments(story provider.StoryRef) []Comment {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Comment, len(p.comments[story]))
	copy(out, p.comments[story])
	return out
}

var _ provider.StoryProvider = (*Provider)(nil)
