package mcpserve

import (
	"context"
	"fmt"
	"sync"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardio"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/recovery"
)

// ReadinessLoader is the per-request readiness port (spec/readiness-
// recovery ac-2/ac-4, 04 §port pattern: defined at this consumer — an
// identical interface is declared independently in internal/workbench,
// never a shared interface package). get_document asks for its OWN
// rendered ref on every call (R-RR1-8), never a shared or foreign ref.
// internal/readinessload.Loader is the one production implementation;
// nil means no loader is wired (a headless verdi mcp with nothing
// configured), which get_document renders as an honest "not supplied"
// rather than an error.
type ReadinessLoader interface {
	Load(ctx context.Context, ref string) (readinesspilot.Snapshot, error)
}

// RecoveryLoader is the per-request readiness-recovery port
// (spec/readiness-recovery-v2 ac-8, ac-10's MCP half, 04 §port pattern:
// defined at this consumer). internal/recovery.Loader is the one
// production implementation; nil means no loader is wired, which
// get_recovery renders as an honest "recovery projection not supplied"
// toolError rather than a fabricated empty projection.
type RecoveryLoader interface {
	Load(ctx context.Context, ref string) (recovery.Projection, error)
}

// Backend is the one real implementation behind every MCP tool: a store
// root plus the read (internal/index, internal/evidence,
// internal/storyresolve) and write (append-only JSONL) operations 05
// §MCP server's table names. It is a concrete struct, not an interface —
// there is exactly one implementation, its filesystem/git dependencies
// are exercised directly (against a real, hermetic fixturegit-built repo)
// in tests, so no port/fake seam earns its keep here for THOSE (04 §port
// pattern applies where a real dependency is unsafe or slow to exercise
// in tests; local git and the local filesystem are neither) — Forge is
// the one exception (V1-P7): a genuine network dependency, so it stays
// the I-22 port interface, nil by default (no live review population;
// every other tool is unaffected), set by cmd/verdi's serve.go/mcp.go
// when a real forge is configured/reachable, and driven by a hermetic
// fake/httptest double in tests (review_test.go).
type Backend struct {
	// Root is the store root directory (internal/store.FindRoot's
	// result) — the directory whose child is .verdi/.
	Root string

	// Forge is used only by list_annotations' review-sticky mirrored
	// population (review.go) — read-only (ListOpenMRs/ListComments/
	// GetThreadResolution only; never PostComment, which stays exclusively
	// the board's authoring-side concern, V1-P6). nil is a fully valid
	// zero value: every tool degrades to "no review population" rather
	// than erroring.
	Forge forge.Forge

	// ReadinessLoader derives readiness fresh for get_document's own
	// rendered ref on its live readings only — the accepted bytes and the
	// working tree (spec/readiness-recovery ac-2/ac-4). A pinned-commit
	// render never calls it: a live derivation is not a fact about
	// historical bytes (final-review F10). nil means no loader is wired:
	// the document states the absence rather than erroring.
	ReadinessLoader ReadinessLoader

	// RecoveryLoader derives get_recovery's read-only readiness-recovery
	// projection fresh, per request (spec/readiness-recovery-v2 ac-8,
	// ac-10). This tool carries no apply/choice argument at all (co-3):
	// nothing this tool call can express ever reaches either executor,
	// structurally, not merely by convention. nil means no loader is
	// wired: the tool states the absence rather than erroring.
	RecoveryLoader RecoveryLoader

	// ReviewUnavailable, when non-empty, is the disclosed reason a
	// CONFIGURED forge (named in verdi.yaml) could not be reached to build
	// a live Forge (I-1(b)). Set by cmd/verdi's serve.go/mcp.go when
	// forgeBestEffort finds a configured kind but no credentials. It lets
	// list_annotations distinguish "no forge configured" (Forge nil,
	// ReviewUnavailable "" — silent, legitimate) from "configured but
	// unavailable" (Forge nil, ReviewUnavailable set — a disclosure field
	// in the response, never silence: constitution 2/10).
	ReviewUnavailable string

	// writeMu serializes the two tools that take it — add_annotation
	// (tool_add_annotation.go) and import_apply (tool_import.go): two
	// concurrent connections calling either are ordinary (Server.Serve
	// spawns a goroutine per connection), and while a single O_APPEND
	// write() of one JSONL line is already atomic at the syscall level
	// on a local filesystem, this mutex removes any doubt and keeps the
	// D3 "one writer" story simple to reason about — the process-level
	// lock (I-12) keeps other PROCESSES out; this keeps this process's
	// own goroutines from interleaving.
	//
	// mutate_draft deliberately does NOT take it: its entire write runs
	// inside the checkout-wide writer lock (draftmutation.WithWriterLock,
	// internal/draftmutation/service.go), which already excludes this
	// process's other goroutines as well as other processes. import_apply
	// holds this mutex as well, around a call whose own body acquires
	// that same checkout-wide lock (internal/specimport/publish.go); the
	// two only ever nest in that one order — mutex, then file lock — so
	// the pair cannot invert.
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
// (01 §Directory layout). Delegates to internal/boardio — see that
// package's doc comment for why this moved out of mcpserve once
// internal/workbench needed the identical annotation-stream I/O
// (CLAUDE.md: shared code lives in a shared internal/ package).
func (b *Backend) annotationsDir() string {
	return boardio.AnnotationsDir(b.Root)
}

// annotationFileForTarget names the JSONL file a targeted annotation
// belongs in (boardio.AnnotationFileForTarget).
func annotationFileForTarget(ref artifact.Ref) string {
	return boardio.AnnotationFileForTarget(ref)
}

// annotationFileForBoard names the JSONL file a free-floating (board-only,
// no target) sticky belongs in, keyed by its board's story slug via the
// same store.RefSlug normative slugging every other story-keyed directory
// in the store uses (01 §notes: "Ref slugging is normative";
// boardio.AnnotationFileForBoard).
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
	return boardio.AnnotationFileForBoard(storySlug)
}
