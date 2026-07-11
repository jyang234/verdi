package evidence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
)

// commitDirRe matches a derived tree's commit-named subdirectories
// (01 §Directory layout: derived/<ref-slug>/<commit>/), the same shape as
// artifact's own (unexported) commit sha pattern.
var commitDirRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// LoadRecords loads every verdicts.json record found in derivedRoot's
// immediate commit-named subdirectories and keeps only those whose
// provenance.commit is commit itself or a real ancestor of commit in
// gitDir's history (03 §The fold: "current ... whose commit is an
// ancestor of C"). Both provenance classes (ci and local) are returned —
// Fold decides which to trust via its Preview flag.
//
// A derivedRoot that does not exist on disk is not an error: a story that
// has never been synced yet has no derived data, which the fold reads
// honestly as "no records" rather than failing operationally.
func LoadRecords(ctx context.Context, gitDir, derivedRoot, commit string) ([]artifact.Evidence, error) {
	entries, err := os.ReadDir(derivedRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("evidence: reading %s: %w", derivedRoot, err)
	}

	var out []artifact.Evidence
	for _, e := range entries {
		if !e.IsDir() || !commitDirRe.MatchString(e.Name()) {
			continue
		}
		recordCommit := e.Name()

		isAncestor, err := gitx.IsAncestor(ctx, gitDir, recordCommit, commit)
		if err != nil {
			return nil, fmt.Errorf("evidence: checking ancestry of %s: %w", recordCommit, err)
		}
		if !isAncestor {
			continue
		}

		recs, err := loadVerdicts(filepath.Join(derivedRoot, recordCommit, "verdicts.json"))
		if err != nil {
			return nil, err
		}
		out = append(out, recs...)
	}

	// Deterministic output order, independent of os.ReadDir's directory
	// iteration order: Current()'s (pipeline, job) reduction is itself
	// order-independent, but callers (matrix's rendering, tests) benefit
	// from a stable, content-derived order rather than one incidentally
	// tied to directory listing order.
	sort.SliceStable(out, func(i, j int) bool { return recordSortKey(out[i]) < recordSortKey(out[j]) })
	return out, nil
}

// loadVerdicts strict-decodes each record in a verdicts.json array. A
// commit directory with no verdicts.json yet is not an error (empty
// slice, nil error); a verdicts.json that exists but fails to decode is a
// real, surfaced error — a derived record that is on disk but broken is
// worse than absent.
func loadVerdicts(path string) ([]artifact.Evidence, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("evidence: reading %s: %w", path, err)
	}

	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("evidence: unmarshaling %s: %w", path, err)
	}

	out := make([]artifact.Evidence, 0, len(raw))
	for i, rm := range raw {
		rec, err := artifact.DecodeEvidence(rm)
		if err != nil {
			return nil, fmt.Errorf("evidence: %s record %d: %w", path, i, err)
		}
		out = append(out, *rec)
	}
	return out, nil
}

// recordSortKey is a deterministic composite key for LoadRecords's output
// ordering — not used by the fold's grouping/ordering logic itself
// (Current owns that).
func recordSortKey(r artifact.Evidence) string {
	return string(r.Kind) + "\x00" + string(r.Provenance.Source) + "\x00" + r.Provenance.Commit + "\x00" +
		r.Provenance.Pipeline + "\x00" + r.Provenance.Job + "\x00" + r.Producer + "\x00" + r.Witness
}
