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
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/forge"
	"github.com/jyang234/verdi/internal/gitx"
)

// perSpecDerivedKeyPrefix is the leading path segment every per-spec derived
// key carries: store.RefSlug(spec.ID), and a spec's id is always
// "spec/<name>" (RefSlug lowercases and maps "/" -> "--"), so the key is
// "spec--<name>/<commit>/<file>". This is the exact prefix the ONLY
// closure-time undecodable-disclosure surface keys by — evidence.
// QuarantinedRecords walking store.DerivedSpecDir(store.RefSlug(spec.ID)),
// rendered by the story closure gate (closuregate.go) and close --preflight
// (closepreflight.go). A fetched artifact also carries the branch-keyed
// per-service bundle under store.RefSlug(<git-ref>) (forge/zip.go), which no
// closure surface ever walks — so an undecodable file's key class decides
// whether any downstream surface will re-surface it.
const perSpecDerivedKeyPrefix = "spec--"

// classifyUndecodableKeys partitions sync's undecodable fetched-record keys
// (quarantineUnreachable's return) by whether the closure-time disclosure
// surface will ever re-surface them, so sync's own notice can state the honest
// situation PER KEY CLASS rather than promising every key a closure disclosure
// that fires only for per-spec dirs.
//
// A per-spec key (first path segment begins with perSpecDerivedKeyPrefix) goes
// to perSpec: its spec's own closure gate walks that dir (evidence.
// QuarantinedRecords over every commit subdir, reachable or not) and
// re-discloses the file, so sync's "excluded from the fold and disclosed at
// closure" claim holds there. Every other key — the branch-keyed per-service
// bundle and any non-spec key — goes to other: no closure surface walks it, so
// sync's notice is that file's ONLY disclosure. Input order is preserved (the
// caller's slice is already sorted), keeping both outputs deterministic.
func classifyUndecodableKeys(undecodable []string) (perSpec, other []string) {
	for _, key := range undecodable {
		seg := key
		if i := strings.IndexByte(key, '/'); i >= 0 {
			seg = key[:i]
		}
		if strings.HasPrefix(seg, perSpecDerivedKeyPrefix) {
			perSpec = append(perSpec, key)
		} else {
			other = append(other, key)
		}
	}
	return perSpec, other
}

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
// It returns the total number of records quarantined across the whole tree
// and the keys of any fetched record file that failed strict decode, for the
// caller's own disclosure lines. gitx.ReachableFromHEAD folds both "the
// commit does not exist at all" and "the commit exists but no ref reaches
// it" into an honest false, never an error — the exact X-15 hard-fail this
// story closes; only a genuine operational failure (root is not a git
// repository at all, or a re-encode fails) surfaces as an error here.
//
// spec/evidence-resilience finding 3: an undecodable fetched verdicts.json/
// runtime.json is quarantined-by-default — its key is returned in
// undecodable, its bytes are left verbatim on disk (kept, never dropped),
// and sync does NOT exit operationally. This restores the pre-ac-1 posture:
// before this story the fetch path wrote runtime.json to disk WITHOUT
// decoding it, so adding a strict decode here must not turn a malformed
// fetched file (a truncated partial write, an older-schema record — the
// debris a deleted branch's stale bundle carries) into a NEW sync-time
// operational failure on the exact path ac-1 hardens. The fold excludes such
// a file via directory reachability, and the closure gate's disclosure pass
// (evidence.QuarantinedRecords) surfaces it as undecodable — the same
// non-fatal posture the fold/disclosure side applies to quarantined data.
func quarantineUnreachable(ctx context.Context, root string, tree forge.DerivedTree, headCommit string) (quarantined int, undecodable []string, err error) {
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
		if decErr := artifact.DecodeStrictJSON(tree[key], &records); decErr != nil {
			// finding 3: quarantine-by-default, non-fatal. Leave the bytes
			// verbatim (continue without rewriting tree[key]) and note the key.
			undecodable = append(undecodable, key)
			continue
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
			reachable, rerr := gitx.ReachableFromHEAD(ctx, root, records[i].Provenance.Commit, headCommit)
			if rerr != nil {
				return total, undecodable, fmt.Errorf("sync: checking reachability of %s (from %s): %w", records[i].Provenance.Commit, key, rerr)
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
		data, merr := canonjson.Marshal(records)
		if merr != nil {
			return total, undecodable, fmt.Errorf("sync: re-encoding %s after quarantine: %w", key, merr)
		}
		tree[key] = data
		total += quarantinedHere
	}
	return total, undecodable, nil
}
