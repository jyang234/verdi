// Sync-time quarantine of evidence records referencing a commit unreachable
// from HEAD (spec/evidence-resilience ac-1; X-15/X-11b, ledger L-N3). Split
// out of sync.go as its own topic (CLAUDE.md: one file ~= one topic),
// mirroring sync_ancestor.go's own precedent of isolating one
// ancestry-adjacent concern from bundle materialization/evaluation.
//
// This applies ONLY to the CI-pulled fetch path (runSync's forge.DerivedTree
// branch): a fetched artifact can carry keyed subdirs for ANY spec a CI run
// touched, including one whose evidence was produced on a feature branch
// that has since been deleted — the routine shape a merged PR's branch
// cleanup produces (X-15). --or-regen's local regeneration and --produce's
// self-hosted producer both always stamp provenance.commit as the exact
// commit sync is running at, which is trivially reachable from itself
// (gitx.IsAncestor's own self-inclusive semantics) — running this check on
// that path could never quarantine anything, so it is not invoked there.
package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/gitx"
)

// quarantineUnreachable scans tree's record-bearing files
// (evidence.RecordFileNames: verdicts.json, runtime.json — the exact set
// the fold reads records from) for any Evidence record whose
// provenance.commit is not reachable from headCommit in root's git
// history, and rewrites that record IN PLACE (tree is a map; the caller's
// copy is mutated directly) with a quarantine annotation — ac-1: "the
// record is kept ... never silently removed ... annotated with the
// quarantine reason", the smallest-reversible shape the story specifies.
//
// Every other file — every non-record derived file (review.json,
// boundary-diff.json, tests.json, toolchain.json) and every record file
// with nothing to quarantine — is left byte-for-byte untouched: only a
// record file that actually contains at least one newly-quarantined
// record is re-encoded (canonical JSON per CLAUDE.md).
//
// It returns the total number of records quarantined across the whole
// tree, for the caller's own disclosure line. gitx.ReachableFromHEAD folds
// both "the commit does not exist at all" and "the commit exists but no
// ref reaches it" into an honest false, never an error — the exact X-15
// hard-fail this story closes; only a genuine operational failure (root is
// not a git repository at all) surfaces as an error here.
func quarantineUnreachable(ctx context.Context, root string, tree forge.DerivedTree, headCommit string) (int, error) {
	recordFile := make(map[string]bool, len(evidence.RecordFileNames))
	for _, name := range evidence.RecordFileNames {
		recordFile[name] = true
	}

	keys := make([]string, 0, len(tree))
	for key := range tree {
		keys = append(keys, key)
	}
	sort.Strings(keys) // deterministic scan/quarantine order

	total := 0
	for _, key := range keys {
		if !recordFile[filepath.Base(key)] {
			continue
		}
		var records []artifact.Evidence
		if err := artifact.DecodeStrictJSON(tree[key], &records); err != nil {
			return 0, fmt.Errorf("sync: decoding fetched %s for quarantine check: %w", key, err)
		}

		quarantinedHere := 0
		for i := range records {
			// A record produced at headCommit itself is trivially
			// reachable (self-ancestor, gitx.IsAncestor's own documented
			// semantics) without consulting git at all — mirroring
			// fetchAncestorBundle's own "commit itself first" fast path
			// (sync_ancestor.go), so the overwhelmingly common case (a
			// bundle's own records all reference the commit it was
			// fetched at) never requires root to even BE a git repository,
			// exactly like the rest of this fetch path today.
			if records[i].Provenance.Commit == headCommit {
				continue
			}
			reachable, err := gitx.ReachableFromHEAD(ctx, root, records[i].Provenance.Commit, headCommit)
			if err != nil {
				return 0, fmt.Errorf("sync: checking reachability of %s (from %s): %w", records[i].Provenance.Commit, key, err)
			}
			if reachable {
				continue
			}
			records[i].Quarantine = &artifact.EvidenceQuarantine{
				Reason: fmt.Sprintf("provenance.commit %s is not reachable from %s at sync time (its source branch has likely since been deleted)", records[i].Provenance.Commit, headCommit),
			}
			quarantinedHere++
		}
		if quarantinedHere == 0 {
			continue // untouched: the original fetched bytes stand exactly as they were
		}
		data, err := canonjson.Marshal(records)
		if err != nil {
			return 0, fmt.Errorf("sync: re-encoding %s after quarantine: %w", key, err)
		}
		tree[key] = data
		total += quarantinedHere
	}
	return total, nil
}
