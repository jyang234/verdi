package contextresolve

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/contextevent"
	"github.com/jyang234/verdi/internal/sealedexec"
)

// Compiler is the one trusted port this service reaches (the 04 §port
// pattern: declared at the consumer). internal/contextcompile.Compiler
// satisfies it as a value.
type Compiler interface {
	Compile(ctx context.Context, root string, request contextcompile.Request) (contextcompile.Result, error)
}

// Service answers one read-only resolution question over the existing
// compiler. The zero value fails closed: construct with NewService
// (production) or newServiceWithCompiler (tests) before calling Resolve.
type Service struct {
	constructed bool
	compiler    Compiler
}

// NewService returns a Service wired to the real, production read-only
// context compiler.
func NewService() Service { return newServiceWithCompiler(contextcompile.NewCompiler()) }

// newServiceWithCompiler is the package-private construction seam this
// package's own tests use to inject a fake compiler (mirrors
// internal/contextcompile's own newCompilerWithPorts).
func newServiceWithCompiler(compiler Compiler) Service {
	return Service{constructed: true, compiler: compiler}
}

// Resolve recompiles the original request against root, requires that fresh
// canonical manifest to equal the supplied base manifest byte-for-byte,
// replays every installed expansion forward from the base revision/digest and
// an empty root, requires the replayed terminal state to equal the supplied
// terminal tuple, and only then resolves the requested ref (correction §2.2).
//
// The two non-proven codes are exclusive, and deliberately so. A caller whose
// base manifest is not what this store compiles to now is holding a lineage
// this store cannot judge at all, so the answer is manifest-stale and nothing
// else; adding lineage-inconsistent would assert a second fact the resolver is
// in no position to have proved. Symmetrically, every row relation and the
// terminal tuple are one finding: lineage-inconsistent.
//
// The two failure KINDS are different values. A returned error is an
// inability — no compiler, no store, an unencodable document — and the caller
// has no answer at all. A returned non-proven Result is an answer: the query
// looked, and what it found does not prove the ref. Collapsing the two would
// let "the compiler is broken" read as "your ref does not exist".
//
// Nothing here writes, and nothing here decides: the item is a fact about the
// state the caller described, never a permission to use it.
func (s Service) Resolve(ctx context.Context, root string, request Request) (Result, error) {
	if !s.constructed || s.compiler == nil {
		return Result{}, errors.New("contextresolve: service requires a compiler (construct with NewService)")
	}
	if ctx == nil {
		return Result{}, errors.New("contextresolve: resolve requires a context")
	}
	if strings.TrimSpace(root) == "" {
		return Result{}, errors.New("contextresolve: resolve requires a store root")
	}
	// A caller-constructed request has not passed the wire grammar, so it is
	// held to exactly the same one before any port is touched.
	if _, err := EncodeRequest(request); err != nil {
		return Result{}, fmt.Errorf("contextresolve: invalid resolve request: %w", err)
	}
	baseBytes, err := contextcompile.EncodeManifest(request.BaseManifest)
	if err != nil {
		return Result{}, fmt.Errorf("contextresolve: encode supplied base manifest: %w", err)
	}

	compiled, err := s.compiler.Compile(ctx, root, request.Compile)
	if err != nil {
		return Result{}, fmt.Errorf("contextresolve: recompile context: %w", err)
	}
	// The comparison is over the canonical encodings rather than the decoded
	// values: the manifest's own digest is a member of that encoding, so byte
	// equality is exactly the identity the rest of the lineage is chained to.
	freshBytes, err := contextcompile.EncodeManifest(compiled.Manifest)
	if err != nil {
		return Result{}, fmt.Errorf("contextresolve: encode recompiled manifest: %w", err)
	}
	if !bytes.Equal(freshBytes, baseBytes) {
		return nonProven(request.Ref, WitnessManifestStale), nil
	}
	// The replay starts from the STORE's own manifest rather than the struct
	// the caller handed over. The two are byte-identical by the line above, so
	// this is not a second source of truth — it is the one that cannot carry a
	// stale in-memory self digest a re-encode would silently correct.
	if err := replayLineage(compiled.Manifest, request); err != nil {
		return nonProven(request.Ref, WitnessLineageInconsistent), nil
	}

	return resolveRef(compiled, request.Ref)
}

// replayLineage replays every installed row forward from the base manifest's
// own revision and digest and an EMPTY prior expansion root, and requires the
// state it arrives at to be exactly the supplied terminal tuple.
//
// Every transition identity comes from internal/sealedexec's one owning pure
// helper (SI-182). This package restates no request-id, child-manifest,
// expansion or expansion-root preimage: a second copy of those algorithms is
// precisely the drift that would let a rewritten lineage replay cleanly, and
// a resolver that had drifted the same way as its own copy would agree with
// itself rather than with the producer.
//
// The flight identity handed to that helper is the request's EXPLICIT one,
// never a value read off the row under check. A lineage whose rows all name
// some other flight is internally perfect and still refused, which is the
// whole point: identity is bound by the durable dispatch, not asserted by the
// evidence it authorizes.
//
// A supplied data item is never accepted because it strict-decodes. Its exact
// canonical bytes are an operand of the child-manifest and expansion digests,
// so a tampered item cannot reach a matching transition identity.
func replayLineage(base contextcompile.Manifest, request Request) error {
	if base.Revisions.Context <= 0 {
		return errors.New("the base manifest carries no positive context revision")
	}
	key := sealedexec.ExecutionKey{
		Flight: request.Identity.Flight,
		Lane:   request.Identity.Lane,
		Epoch:  request.Identity.Epoch,
	}
	revision := uint64(base.Revisions.Context)
	digest := base.Digest
	root := ""

	seen := make(map[string]bool, len(request.Expansions))
	var lastGlobal uint64
	for i, row := range request.Expansions {
		ack := row.TerminalAck
		if ack.Flight != request.Identity.Flight || ack.Lane != request.Identity.Lane ||
			ack.Epoch != request.Identity.Epoch || ack.Session != request.Identity.Session {
			return fmt.Errorf("row %d: acknowledgment does not name the dispatch-bound flight identity", i)
		}
		if ack.Kind != contextevent.KindChildManifest {
			return fmt.Errorf("row %d: acknowledgment is not a child-manifest acknowledgment", i)
		}
		// The child-manifest event is appended before the install advances the
		// shared state, so it is acknowledged AT the parent revision.
		if ack.ManifestRevision != revision {
			return fmt.Errorf("row %d: acknowledgment revision does not match the replayed parent", i)
		}
		if ack.GlobalSequence <= lastGlobal {
			return fmt.Errorf("row %d: acknowledgment global sequence does not advance", i)
		}
		lastGlobal = ack.GlobalSequence

		if seen[row.RequestID] {
			return fmt.Errorf("row %d: request id repeats an earlier row", i)
		}
		seen[row.RequestID] = true

		if row.ParentRevision != revision || row.ParentManifestDigest != digest {
			return fmt.Errorf("row %d: parent identity does not match the replayed state", i)
		}
		if row.ChildRevision != revision+1 {
			return fmt.Errorf("row %d: child revision is not one past its parent", i)
		}

		proof, err := sealedexec.ProveInstalledExpansion(sealedexec.InstalledExpansionInput{
			Key:                  key,
			ParentRevision:       revision,
			ParentManifestDigest: digest,
			Ref:                  row.Ref,
			Purpose:              row.Purpose,
			Item:                 row.Data,
			PriorExpansionRoot:   root,
		})
		if err != nil {
			return fmt.Errorf("row %d: prove installed expansion: %w", i, err)
		}
		if row.RequestID != proof.RequestID {
			return fmt.Errorf("row %d: request id does not match the replayed state", i)
		}
		if row.ChildManifestDigest != proof.ChildManifestDigest {
			return fmt.Errorf("row %d: child manifest digest does not match the replayed transition", i)
		}
		if row.ExpansionDigest != proof.ExpansionDigest {
			return fmt.Errorf("row %d: expansion digest does not match the replayed transition", i)
		}
		if row.ExpansionRoot != proof.ExpansionRoot {
			return fmt.Errorf("row %d: expansion root does not chain to its predecessor", i)
		}
		revision, digest, root = row.ChildRevision, row.ChildManifestDigest, row.ExpansionRoot
	}

	replayed := Terminal{Revision: revision, ManifestDigest: digest, ExpansionRoot: root}
	if request.Terminal != replayed {
		return errors.New("the supplied terminal state is not the replayed terminal state")
	}
	return nil
}

// resolveRef selects the one data item the ref names in the freshly compiled
// state, or the closed reason no single item answers to it.
//
// The candidates are the compile's own, never the items the lineage says were
// installed: an installed row is evidence about a transition, not a second
// source of context, and admitting it would turn the ordinary happy path —
// where a row installs the very item the compile already carries — into a
// self-inflicted ambiguity.
//
// The excluded ledger participates because an excluded candidate is a real
// answer: the ref exists in this compile's universe and carries no payload in
// this phase or scope, which is "inapplicable" rather than "absent". Any
// other multiplicity is refused rather than resolved — a query that picked
// one of two candidates would be deciding, and deciding is not this query's
// job.
func resolveRef(compiled contextcompile.Result, ref string) (Result, error) {
	var items []contextcompile.DataItem
	for _, item := range compiled.DataItems {
		if item.Ref != nil && *item.Ref == ref {
			items = append(items, item)
		}
	}
	excluded := 0
	for _, entry := range compiled.Manifest.Excluded {
		if entry.Ref != nil && *entry.Ref == ref {
			excluded++
		}
	}
	switch total := len(items) + excluded; {
	case total == 0:
		return nonProven(ref, WitnessRefAbsent), nil
	case total > 1:
		return nonProven(ref, WitnessRefAmbiguous), nil
	case len(items) == 0:
		return nonProven(ref, WitnessRefInapplicable), nil
	}
	item := items[0]
	result := Result{Schema: ResultSchema, State: StateProven, Ref: ref, Item: &item, Witnesses: []Witness{}}
	if _, err := EncodeResult(result); err != nil {
		return Result{}, fmt.Errorf("contextresolve: encode resolved item: %w", err)
	}
	return result, nil
}

// nonProven builds the closed non-proven arm. Ordering is left to
// EncodeResult, which sorts by the one representation §2.2 names.
func nonProven(ref string, codes ...WitnessCode) Result {
	witnesses := make([]Witness, 0, len(codes))
	for _, code := range codes {
		witnesses = append(witnesses, Witness{Schema: WitnessSchema, Code: code})
	}
	return Result{Schema: ResultSchema, State: StateNonProven, Ref: ref, Witnesses: witnesses}
}
