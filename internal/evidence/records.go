package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// commitDirRe matches a derived tree's commit-named subdirectories
// (01 §Directory layout: derived/<ref-slug>/<commit>/), the same shape as
// artifact's own (unexported) commit sha pattern.
var commitDirRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// RecordFileNames are the derived-tree files under one commit directory
// that carry verdi.evidence/v1 records (01 §Directory layout). verdicts.json
// carries static/behavioral (and, for a hand-assembled fixture, any other
// kind); runtime.json is spec/runtime-evidence dc-2's sibling file — "a
// runtime.json per owning-spec key alongside verdicts.json" — so that
// `verdi sync`'s forge fetch (internal/forge.DerivedTree; zip.go's
// bundleFileNames) carries a real service's probe output through
// unchanged. Both are loaded and merged into one record set here: this is
// the missing link spec/runtime-evidence closes — the fold
// (internal/evidence/fold.go) already handles kind: runtime records, but
// nothing ever loaded one until now.
//
// Exported (spec/evidence-resilience ac-1) so sync's own write-time
// quarantine pass (cmd/verdi/sync_quarantine.go) scans exactly the same
// file set this reader loads records from, rather than a second,
// independently-maintained list that could silently drift from this one.
var RecordFileNames = []string{"verdicts.json", "runtime.json"}

// RecordFile identifies one derived-tree record file LoadRecords actually
// read: its slash-separated path relative to derivedRoot (e.g.
// "<commit>/verdicts.json") and the sha256 content digest
// ("sha256:<hex>") of the exact bytes read. It exists so a fold consumer
// that must RECEIPT its inputs (spec/evidence-slot dc-3: "the derived-tree
// path probed with the digests of any record files read") can cite what
// this loader read without a second, drifting derived-tree walk of its
// own (evidence-slot co-3: one fold, one reader).
type RecordFile struct {
	Path   string
	Digest string
}

// LoadRecords loads every evidence record found in derivedRoot's immediate
// commit-named subdirectories (both verdicts.json and runtime.json,
// RecordFileNames) and keeps only those whose provenance.commit is
// commit itself or a real ancestor of commit in gitDir's history (03 §The
// fold: "current ... whose commit is an ancestor of C"). Both provenance
// classes (ci and local) are returned — Fold decides which to trust via its
// Preview flag.
//
// A derivedRoot that does not exist on disk is not an error: a story that
// has never been synced yet has no derived data, which the fold reads
// honestly as "no records" rather than failing operationally.
func LoadRecords(ctx context.Context, gitDir, derivedRoot, commit string) ([]artifact.Evidence, error) {
	out, _, err := LoadRecordsWithSources(ctx, gitDir, derivedRoot, commit)
	return out, err
}

// LoadRecordsWithSources is LoadRecords plus a manifest of the record
// files it actually read (existing files under ancestor-or-self commit
// directories), each with the content digest of the exact bytes decoded.
// It is the SAME single walk — LoadRecords delegates here — so a receipt
// built from the manifest can never disagree with the records loaded.
// The manifest order is deterministic (os.ReadDir's sorted directory
// order, then RecordFileNames order within a commit directory).
//
// A record file that fails strict CONTENT decode (a truncated partial write or
// an older-schema record) under a reachable commit directory is EXCLUDED from
// the returned records and omitted from the manifest — never a fold-time
// operational error (spec/evidence-resilience finding 1): degradation is
// reachability-independent, and the excluded file is disclosed through
// QuarantinedRecords (undecodableDisclosures) on the closure surfaces. A
// genuine I/O read failure is still surfaced operationally.
func LoadRecordsWithSources(ctx context.Context, gitDir, derivedRoot, commit string) ([]artifact.Evidence, []RecordFile, error) {
	entries, err := os.ReadDir(derivedRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("evidence: reading %s: %w", derivedRoot, err)
	}

	var out []artifact.Evidence
	var sources []RecordFile
	for _, e := range entries {
		if !e.IsDir() || !commitDirRe.MatchString(e.Name()) {
			continue
		}
		recordCommit := e.Name()

		// spec/evidence-resilience ac-2 (X-15): gitx.ReachableFromHEAD,
		// not the plain gitx.IsAncestor this used to call, so a commit-
		// named directory that resolves to no real commit at all (a
		// deleted, since-gc'd branch's tip — the exact shape that used to
		// hard-fail here with git's own "fatal: Not a valid commit name")
		// is folded into "not reachable" and excluded, the same as any
		// other real-but-non-ancestor commit already is — never an
		// operational error. A gitDir that is not a git repository at all
		// is still a real, surfaced error.
		reachable, err := gitx.ReachableFromHEAD(ctx, gitDir, recordCommit, commit)
		if err != nil {
			return nil, nil, fmt.Errorf("evidence: checking ancestry of %s: %w", recordCommit, err)
		}
		if !reachable {
			continue
		}

		for _, name := range RecordFileNames {
			recs, digest, err := loadEvidenceArray(filepath.Join(derivedRoot, recordCommit, name))
			if err != nil {
				// spec/evidence-resilience finding 1 (FIX): an undecodable record
				// FILE under a REACHABLE commit dir is EXCLUDED from the fold,
				// never a fold-time operational error — degradation is
				// reachability-independent so sync's "kept verbatim; excluded from
				// the fold and disclosed at closure" claim (sync_quarantine.go)
				// holds for EVERY undecodable record file, not only those under an
				// unreachable dir this walk already skips above. The same file is
				// surfaced as a disclosed-undecodable entry by QuarantinedRecords,
				// which the closure gate and close --preflight render
				// (undecodableDisclosures) — nothing is dropped silently on the
				// closure surfaces. Returning the decode error here instead would
				// defer the exact operational brick ac-2 removes from sync time to
				// closure/preflight/merge-gate/matrix/rollup time. A genuine I/O
				// READ failure (not errUndecodableRecord) is NOT degraded — it
				// stays operational, since only a content-decode failure has a
				// "disclosed at closure" analog the sync side's claim speaks to.
				if errors.Is(err, errUndecodableRecord) {
					continue
				}
				return nil, nil, err
			}
			if digest != "" {
				sources = append(sources, RecordFile{Path: recordCommit + "/" + name, Digest: digest})
			}
			// spec/evidence-resilience ac-1 (finding 1): a record `verdi sync`
			// annotated as quarantined is never authoritative evidence — a
			// SECOND exclusion signal alongside the directory-reachability
			// check above (belt and suspenders: EITHER signal excludes). This
			// makes artifact/evidence.go:85-96's doc claim true for the case
			// the annotation and directory disagree — a fetched artifact whose
			// subdir key differs from the record's own provenance.commit, or
			// hand-placed derived data, leaving an annotated record under a
			// REACHABLE directory that this reachability check alone would let
			// through and silently count as proven. The file is still recorded
			// in sources above (its bytes were read); only its quarantined
			// records are withheld from the fold's authoritative set.
			for i := range recs {
				if recs[i].Quarantine != nil {
					continue
				}
				out = append(out, recs[i])
			}
		}
	}

	// Deterministic output order, independent of os.ReadDir's directory
	// iteration order: Current()'s (pipeline, job) reduction is itself
	// order-independent, but callers (matrix's rendering, tests) benefit
	// from a stable, content-derived order rather than one incidentally
	// tied to directory listing order.
	sort.SliceStable(out, func(i, j int) bool { return recordSortKey(out[i]) < recordSortKey(out[j]) })
	return out, sources, nil
}

// errUndecodableRecord marks a record file whose CONTENT failed strict decode
// — malformed or truncated JSON, or an older-schema record that fails strict
// decode — as distinct from a genuine I/O failure READING the file.
// loadEvidenceArray wraps both content-decode failure modes (json.Unmarshal
// and artifact.DecodeEvidence) with it so LoadRecordsWithSources can degrade a
// content-decode failure to a reachability-independent fold exclusion
// (spec/evidence-resilience finding 1) while still surfacing a real read
// failure operationally. It mirrors the sync side's own "undecodable" notion,
// which decodes in-memory bundle bytes (sync_quarantine.go) with no read step
// at all — so only a content-decode failure has a "disclosed at closure"
// analog for that side's claim to speak truthfully about.
var errUndecodableRecord = errors.New("record file content is undecodable")

// loadEvidenceArray strict-decodes each record in a verdi.evidence/v1 array
// file (verdicts.json or runtime.json alike — both are the same schema, one
// array of records, 03 §Evidence records). A commit directory with no such
// file yet is not an error (empty slice, empty digest, nil error). A file
// that exists but whose CONTENT fails strict decode returns an error wrapping
// errUndecodableRecord — degraded to a fold exclusion by LoadRecordsWithSources
// (never a fold-time operational brick, finding 1) and surfaced verbatim by
// QuarantinedRecords's disclosure pass; a genuine I/O failure READING the file
// returns a plain (non-errUndecodableRecord) error the loader still surfaces
// operationally. digest is the sha256 of the exact bytes read ("sha256:<hex>"),
// non-empty exactly when the file existed.
func loadEvidenceArray(path string) (recs []artifact.Evidence, digest string, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("evidence: reading %s: %w", path, err)
	}
	sum := sha256.Sum256(data)
	digest = "sha256:" + hex.EncodeToString(sum[:])

	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, "", fmt.Errorf("evidence: unmarshaling %s: %w: %w", path, errUndecodableRecord, err)
	}

	out := make([]artifact.Evidence, 0, len(raw))
	for i, rm := range raw {
		rec, err := artifact.DecodeEvidence(rm)
		if err != nil {
			return nil, "", fmt.Errorf("evidence: %s record %d: %w: %w", path, i, errUndecodableRecord, err)
		}
		out = append(out, *rec)
	}
	return out, digest, nil
}

// ExcludedCommitDirs reports every commit-named subdirectory of derivedRoot
// that exists on disk but was excluded from LoadRecordsWithSources's own
// output because it is not reachable from commit in gitDir's history — the
// exact reachability check LoadRecordsWithSources already performs per
// entry (this file's own loop), captured here instead of silently
// discarded. A fold consumer's disclosure can name this "found but
// excluded (stale)" state for free (spec/close-preflight dc-4): the walk
// and the reachability check are the identical ones LoadRecordsWithSources
// runs, so this never risks disagreeing with what the fold actually
// excluded, and it changes no verdict — it is a diagnostic listing only.
// This covers BOTH a real, merely-diverged sibling commit and a commit
// that resolves to no real object at all (spec/evidence-resilience ac-2,
// X-15) alike — gitx.ReachableFromHEAD folds both into the same excluded
// bucket; QuarantinedRecords is the sibling function a caller wanting the
// excluded records themselves (not just their commit names) reaches for.
//
// A derivedRoot that does not exist on disk yields (nil, nil) — the same
// never-synced authoring state LoadRecordsWithSources treats as "no
// records", not an error. Output is sorted lexicographically (deterministic,
// independent of os.ReadDir's own listing order).
func ExcludedCommitDirs(ctx context.Context, gitDir, derivedRoot, commit string) ([]string, error) {
	entries, err := os.ReadDir(derivedRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("evidence: reading %s: %w", derivedRoot, err)
	}

	var out []string
	for _, e := range entries {
		if !e.IsDir() || !commitDirRe.MatchString(e.Name()) {
			continue
		}
		reachable, err := gitx.ReachableFromHEAD(ctx, gitDir, e.Name(), commit)
		if err != nil {
			return nil, fmt.Errorf("evidence: checking ancestry of %s: %w", e.Name(), err)
		}
		if !reachable {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// UndecodableFile names a derived-tree record file that exists but failed
// strict decode — a truncated partial write or an older-schema record, the
// debris a stale poisoned bundle left behind once its source branch was
// deleted after its PR merged (spec/evidence-resilience ac-2, finding 2).
// QuarantinedRecords surfaces it as a disclosed entry rather than an
// operational error: the closure gate's disclosure pass must never brick on
// exactly the degraded-evidence shape this story exists to make non-fatal.
type UndecodableFile struct {
	// Path is the file's slash-separated path relative to derivedRoot
	// ("<commit>/verdicts.json").
	Path string
	// Reason is the strict-decode failure, verbatim.
	Reason string
}

// QuarantinedRecords returns every evidence record under derivedRoot's
// commit-named subdirectories that the fold excludes as non-authoritative,
// on EITHER of the two quarantine signals (spec/evidence-resilience), plus a
// list of any record file that failed strict decode:
//
//   - directory signal: the containing commit-named directory is not
//     reachable from commit (self or ancestor) in gitDir's history — the
//     exact records LoadRecordsWithSources excludes by reachability;
//   - annotation signal (ac-1, finding 1): the record carries a `verdi sync`
//     quarantine annotation (artifact.Evidence.Quarantine), even under a
//     REACHABLE directory — surfaced here so a record the fold now excludes
//     on the annotation alone is disclosed rather than left silent.
//
// Records are returned as full artifact.Evidence values (not bare commit
// names, ExcludedCommitDirs's own projection) so a disclosure consumer can
// read each excluded record's evidence_for and name which acceptance
// criterion it would have evidenced (ac-2: "reads a quarantined record as a
// per-record disclosed-unproven against the acceptance criterion it would
// have evidenced").
//
// A record file that fails strict decode inside this walk degrades to an
// UndecodableFile entry, NEVER an error return (ac-2, finding 2): this is a
// disclosure-only read that must never turn stale debris into an operational
// closure-gate failure. The one genuine operational failure still surfaced
// as an error is gitDir not being a git repository at all (ReachableFromHEAD).
//
// This never makes any record authoritative — it changes no verdict; it is a
// read of what the fold already excludes, for legibility. A derivedRoot that
// does not exist on disk yields (nil, nil, nil), matching
// LoadRecordsWithSources's and ExcludedCommitDirs's own never-synced posture.
// Both outputs are sorted deterministically, independent of os.ReadDir's
// listing order.
func QuarantinedRecords(ctx context.Context, gitDir, derivedRoot, commit string) ([]artifact.Evidence, []UndecodableFile, error) {
	entries, err := os.ReadDir(derivedRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("evidence: reading %s: %w", derivedRoot, err)
	}

	var out []artifact.Evidence
	var undecodable []UndecodableFile
	for _, e := range entries {
		if !e.IsDir() || !commitDirRe.MatchString(e.Name()) {
			continue
		}
		reachable, err := gitx.ReachableFromHEAD(ctx, gitDir, e.Name(), commit)
		if err != nil {
			return nil, nil, fmt.Errorf("evidence: checking ancestry of %s: %w", e.Name(), err)
		}
		for _, name := range RecordFileNames {
			recs, _, lerr := loadEvidenceArray(filepath.Join(derivedRoot, e.Name(), name))
			if lerr != nil {
				// finding 1/2: an undecodable record file is disclosed here,
				// never an operational error — for BOTH an unreachable dir's
				// debris AND (since finding 1's fix) a REACHABLE dir's undecodable
				// file, which LoadRecordsWithSources now EXCLUDES from the fold
				// rather than erroring on. This disclosure-only walk is the same
				// surface the closure gate and close --preflight render
				// (undecodableDisclosures) for either case; reachability no longer
				// gates whether the fold's own reader errors first, so this walk
				// is now the primary disclosure of a reachable dir's undecodable
				// file, not merely standalone robustness against one the fold
				// missed. A genuine read failure surfaces here too, which this
				// disclosure-only pass never escalates to an operational error
				// (its sole operational failure stays ReachableFromHEAD's
				// not-a-repo case).
				undecodable = append(undecodable, UndecodableFile{Path: e.Name() + "/" + name, Reason: lerr.Error()})
				continue
			}
			for i := range recs {
				// Either signal excludes: every record under an unreachable
				// dir (directory signal), and any annotated record even under
				// a reachable dir (annotation signal, finding 1).
				if !reachable || recs[i].Quarantine != nil {
					out = append(out, recs[i])
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return recordSortKey(out[i]) < recordSortKey(out[j]) })
	sort.SliceStable(undecodable, func(i, j int) bool { return undecodable[i].Path < undecodable[j].Path })
	return out, undecodable, nil
}

// recordSortKey is a deterministic composite key for LoadRecords's output
// ordering — not used by the fold's grouping/ordering logic itself
// (Current owns that).
func recordSortKey(r artifact.Evidence) string {
	return string(r.Kind) + "\x00" + string(r.Provenance.Source) + "\x00" + r.Provenance.Commit + "\x00" +
		r.Provenance.Pipeline + "\x00" + r.Provenance.Job + "\x00" + r.Producer + "\x00" + r.Witness
}
