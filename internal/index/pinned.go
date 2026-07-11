package index

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
)

// GetPinned resolves a pinned ref (kind/name@commit) to the artifact's
// content as it existed at that commit, via gitx.Show — distinct from Get,
// which only ever sees the current working tree. The ref's path is taken
// from the current index (the same committed-zone kind/name normally
// resolves to a stable path across history; a spec crossing
// active/archive between the pinned commit and now is a known limitation
// out of phase 3's scope — later phases that need cross-move pinned
// resolution should widen this).
func (ix *Index) GetPinned(ctx context.Context, ref artifact.Ref) (*Entry, error) {
	if !ref.Pinned() {
		return nil, fmt.Errorf("index: GetPinned: ref %q must be pinned (kind/name@commit)", ref)
	}

	// artifact.Ref's Kind is closed to the six committed-zone kinds
	// (Kind.Valid()), so unpinned always names a committed-zone-shaped ref
	// ("<kind>/<name>") — external "svc/..." refs (three path segments)
	// cannot be expressed through this type and so never reach this path.
	unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}
	current, ok := ix.Get(unpinned.String())
	if !ok {
		return nil, fmt.Errorf("index: GetPinned(%s): %q not found in the current index", ref, unpinned)
	}

	relPath, err := filepath.Rel(ix.root, current.Path)
	if err != nil {
		return nil, fmt.Errorf("index: GetPinned(%s): %w", ref, err)
	}

	data, err := gitx.Show(ctx, ix.root, ref.Commit, filepath.ToSlash(relPath))
	if err != nil {
		return nil, fmt.Errorf("index: GetPinned(%s): %w", ref, err)
	}

	entry, err := decodeEntry(current.Kind, data, current.Path)
	if err != nil {
		return nil, fmt.Errorf("index: GetPinned(%s): decoding historical content: %w", ref, err)
	}
	return entry, nil
}
