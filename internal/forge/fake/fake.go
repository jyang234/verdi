// Package fake is a hermetic, in-memory Forge double (04 §Testing's
// pattern applied to the I-22 forge port): no HTTP, no network, used by
// `verdi sync`'s own tests and anywhere else a Forge is needed without a
// real GitLab/GitHub server.
package fake

import (
	"context"
	"fmt"
	"sync"

	"github.com/jyang234/verdi/internal/forge"
)

// Forge is a configurable, in-memory forge.Forge.
type Forge struct {
	mu sync.Mutex

	bundles   map[string]forge.EvidenceBundle
	attribute string
	ci        forge.CIInfo
	openMRs   map[string][]forge.OpenMR    // targetBranch -> open MRs
	files     map[string]map[string][]byte // branch -> path -> content

	comments   map[string][]forge.Comment          // mrID -> comment feed
	threads    map[string][]forge.ThreadResolution // mrID -> thread resolutions
	nextCommID int
}

// New returns an empty Forge: no bundles seeded, GeneratedAttribute
// returns "fake-generated", CIContext returns a zero CIInfo, no open MRs.
func New() *Forge {
	return &Forge{
		bundles:    make(map[string]forge.EvidenceBundle),
		attribute:  "fake-generated",
		openMRs:    make(map[string][]forge.OpenMR),
		files:      make(map[string]map[string][]byte),
		comments:   make(map[string][]forge.Comment),
		threads:    make(map[string][]forge.ThreadResolution),
		nextCommID: 1,
	}
}

func bundleKey(ref, commit string) string { return ref + "@" + commit }

// SeedBundle makes FetchEvidenceBundle(ref, commit) succeed with bundle.
func (f *Forge) SeedBundle(ref, commit string, bundle forge.EvidenceBundle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.bundles[bundleKey(ref, commit)] = bundle
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
func (f *Forge) FetchEvidenceBundle(ctx context.Context, ref, commit string) (*forge.EvidenceBundle, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	b, ok := f.bundles[bundleKey(ref, commit)]
	if !ok {
		return nil, fmt.Errorf("fake: no bundle seeded for ref %q commit %q: %w", ref, commit, forge.ErrNoBundle)
	}
	return &b, nil
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
