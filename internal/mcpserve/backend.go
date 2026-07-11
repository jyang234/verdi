package mcpserve

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/index"
)

// Backend is the one real implementation behind every MCP tool: a store
// root plus the read (internal/index, internal/evidence,
// internal/storyresolve) and write (append-only JSONL) operations 05
// §MCP server's table names. It is a concrete struct, not an interface —
// there is exactly one implementation, it does no network I/O, and its
// filesystem/git dependencies are exercised directly (against a real,
// hermetic fixturegit-built repo) in tests, so no port/fake seam earns
// its keep here (04 §port pattern applies where a real dependency is
// unsafe or slow to exercise in tests; local git and the local
// filesystem are neither).
type Backend struct {
	// Root is the store root directory (internal/store.FindRoot's
	// result) — the directory whose child is .verdi/.
	Root string

	// writeMu serializes add_annotation, the one write path: two
	// concurrent connections calling it are ordinary (Server.Serve
	// spawns a goroutine per connection), and while a single O_APPEND
	// write() of one JSONL line is already atomic at the syscall level
	// on a local filesystem, this mutex removes any doubt and keeps the
	// D3 "one writer" story simple to reason about — the process-level
	// lock (I-12) keeps other PROCESSES out; this keeps this process's
	// own goroutines from interleaving.
	writeMu sync.Mutex
}

// buildIndex is the shared "read the current committed zone" step every
// read tool starts from. internal/index.Build has no persistence of its
// own (its own doc: "a fresh Index is always a fresh walk") — this
// package inherits that same policy rather than adding a cache tools
// would need to invalidate correctly; v0's scale envelope (01 §Scale
// envelope) does not call for one yet.
func (b *Backend) buildIndex() (*index.Index, error) {
	ix, err := index.Build(b.Root)
	if err != nil {
		return nil, fmt.Errorf("mcpserve: building index: %w", err)
	}
	return ix, nil
}

// annotationsDir is data/mutable/annotations/ under the store root
// (01 §Directory layout).
func (b *Backend) annotationsDir() string {
	return filepath.Join(b.Root, ".verdi", "data", "mutable", "annotations")
}

// annotationFileForTarget names the JSONL file a targeted annotation
// belongs in: <kind>--<name>.jsonl, keyed by the TARGET artifact's own
// kind/name (02 §Record schemas' literal path shape), independent of the
// pin's commit — every annotation pinned against any commit of the same
// artifact lives in the same stream.
func annotationFileForTarget(ref artifact.Ref) string {
	return fmt.Sprintf("%s--%s.jsonl", ref.Kind, ref.Name)
}

// annotationFileForBoard names the JSONL file a free-floating (board-only,
// no target) sticky belongs in, keyed by its board's story slug via the
// same store.RefSlug normative slugging every other story-keyed directory
// in the store uses (01 §notes: "Ref slugging is normative").
//
// Implementation note (not a ratified ledger entry — this phase's scope
// excludes editing PLAN.md): 02 §Record schemas' one worked example is a
// TARGETED annotation, so it only pins the "<kind>--<name>.jsonl" shape
// for that case; it does not specify a board-only sticky's file key. The
// smallest-reversible choice taken here treats "board" as a pseudo-kind
// paired with the story's ref slug, so a board-only sticky's file name is
// still exactly one deterministic function of the record's own fields,
// and add_annotation/list_annotations/list_tasks all agree on it.
func annotationFileForBoard(storySlug string) string {
	return fmt.Sprintf("board--%s.jsonl", storySlug)
}
