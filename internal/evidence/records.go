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

// derivedRecordFiles are the derived-tree files under one commit directory
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
var derivedRecordFiles = []string{"verdicts.json", "runtime.json"}

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
// derivedRecordFiles) and keeps only those whose provenance.commit is
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
// order, then derivedRecordFiles order within a commit directory).
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

		isAncestor, err := gitx.IsAncestor(ctx, gitDir, recordCommit, commit)
		if err != nil {
			return nil, nil, fmt.Errorf("evidence: checking ancestry of %s: %w", recordCommit, err)
		}
		if !isAncestor {
			continue
		}

		for _, name := range derivedRecordFiles {
			recs, digest, err := loadEvidenceArray(filepath.Join(derivedRoot, recordCommit, name))
			if err != nil {
				return nil, nil, err
			}
			if digest != "" {
				sources = append(sources, RecordFile{Path: recordCommit + "/" + name, Digest: digest})
			}
			out = append(out, recs...)
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

// loadEvidenceArray strict-decodes each record in a verdi.evidence/v1 array
// file (verdicts.json or runtime.json alike — both are the same schema, one
// array of records, 03 §Evidence records). A commit directory with no such
// file yet is not an error (empty slice, empty digest, nil error); a file
// that exists but fails to decode is a real, surfaced error — a derived
// record that is on disk but broken is worse than absent. digest is the
// sha256 of the exact bytes read ("sha256:<hex>"), non-empty exactly when
// the file existed.
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
		return nil, "", fmt.Errorf("evidence: unmarshaling %s: %w", path, err)
	}

	out := make([]artifact.Evidence, 0, len(raw))
	for i, rm := range raw {
		rec, err := artifact.DecodeEvidence(rm)
		if err != nil {
			return nil, "", fmt.Errorf("evidence: %s record %d: %w", path, i, err)
		}
		out = append(out, *rec)
	}
	return out, digest, nil
}

// ExcludedCommitDirs reports every commit-named subdirectory of derivedRoot
// that exists on disk but was excluded from LoadRecordsWithSources's own
// output because it is neither commit itself nor a real ancestor of commit
// in gitDir's history — the exact ancestry check LoadRecordsWithSources
// already performs per entry (this file's own loop), captured here instead
// of silently discarded. A fold consumer's disclosure can name this "found
// but excluded (stale)" state for free (spec/close-preflight dc-4): the
// walk and the ancestry check are the identical ones LoadRecordsWithSources
// runs, so this never risks disagreeing with what the fold actually
// excluded, and it changes no verdict — it is a diagnostic listing only.
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
		isAncestor, err := gitx.IsAncestor(ctx, gitDir, e.Name(), commit)
		if err != nil {
			return nil, fmt.Errorf("evidence: checking ancestry of %s: %w", e.Name(), err)
		}
		if !isAncestor {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// recordSortKey is a deterministic composite key for LoadRecords's output
// ordering — not used by the fold's grouping/ordering logic itself
// (Current owns that).
func recordSortKey(r artifact.Evidence) string {
	return string(r.Kind) + "\x00" + string(r.Provenance.Source) + "\x00" + r.Provenance.Commit + "\x00" +
		r.Provenance.Pipeline + "\x00" + r.Provenance.Job + "\x00" + r.Producer + "\x00" + r.Witness
}
