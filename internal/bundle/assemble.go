package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/upstream"
)

// ServiceBundle is one service's contribution to a derived bundle: the
// evidence records BuildVerdicts joined for it, its review artifact (nil
// if no review was computed — e.g. a service with no policy.json), and its
// computed boundary diff (nil/empty if no branch contract was diffed).
type ServiceBundle struct {
	ServiceName  string
	Verdicts     []artifact.Evidence
	Review       *upstream.Review
	BoundaryDiff []upstream.BoundaryDiffEntry
}

// filePerm is the file mode every bundle file is written with — readable
// by the owner and group, matching the rest of the store's committed and
// derived files.
const filePerm = 0o644

// Assemble writes the four bundle files (01 §Directory layout:
// verdicts.json, boundary-diff.json, tests.json, review.json) into dir,
// which must already exist (callers create
// data/derived/<ref-slug>/<commit>/ themselves — this package owns file
// content, not store layout). Every file is written via internal/canonjson
// for byte-identical, deterministic output.
//
// tests is the whole regeneration's test summary (typically one `go test
// -json` run per impacted service's module, merged via
// MergeTestSummaries if there is more than one) — a single tests.json
// covers the whole bundle, not one per service, since v0's fixture never
// exercises more than one service module at a time.
func Assemble(dir string, services []ServiceBundle, tests *TestSummary) error {
	if dir == "" {
		return fmt.Errorf("bundle: Assemble: dir must not be empty")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return fmt.Errorf("bundle: Assemble: dir %q does not exist or is not a directory: %w", dir, err)
	}

	var verdicts []artifact.Evidence
	var reviews []*upstream.Review
	var diffs []upstream.BoundaryDiffEntry
	for _, s := range services {
		verdicts = append(verdicts, s.Verdicts...)
		if s.Review != nil {
			reviews = append(reviews, s.Review)
		}
		diffs = append(diffs, s.BoundaryDiff...)
	}
	// Every array-shaped file writes as `[]` rather than `null` when
	// empty: an assembled bundle is a complete, honest artifact even for
	// a service with no obligations, no review, or no contract change,
	// never a JSON null a consumer must special-case.
	if verdicts == nil {
		verdicts = []artifact.Evidence{}
	}
	if reviews == nil {
		reviews = []*upstream.Review{}
	}
	if diffs == nil {
		diffs = []upstream.BoundaryDiffEntry{}
	}
	if tests == nil {
		return fmt.Errorf("bundle: Assemble: tests summary must not be nil")
	}
	if tests.Packages == nil {
		tests.Packages = []PackageResult{}
	}

	if err := writeCanon(filepath.Join(dir, "verdicts.json"), verdicts); err != nil {
		return err
	}
	if err := writeCanon(filepath.Join(dir, "review.json"), reviews); err != nil {
		return err
	}
	if err := writeCanon(filepath.Join(dir, "boundary-diff.json"), diffs); err != nil {
		return err
	}
	if err := writeCanon(filepath.Join(dir, "tests.json"), tests); err != nil {
		return err
	}
	return nil
}

func writeCanon(path string, v interface{}) error {
	data, err := canonjson.Marshal(v)
	if err != nil {
		return fmt.Errorf("bundle: marshaling %s: %w", filepath.Base(path), err)
	}
	if err := os.WriteFile(path, data, filePerm); err != nil {
		return fmt.Errorf("bundle: writing %s: %w", path, err)
	}
	return nil
}

// MergeTestSummaries combines multiple TestSummary values (one per
// impacted service's `go test -json` run) into one: package results are
// concatenated in input order, and Suite is "fail" if any input summary
// failed. Returns an empty (Suite: "pass", no packages) summary for a nil
// or empty input, never a nil pointer — a regeneration that touched no Go
// module still produces a valid, honest tests.json.
func MergeTestSummaries(summaries []*TestSummary) *TestSummary {
	merged := &TestSummary{Schema: testsSchema, Suite: "pass"}
	for _, s := range summaries {
		if s == nil {
			continue
		}
		merged.Packages = append(merged.Packages, s.Packages...)
		if s.Suite == "fail" {
			merged.Suite = "fail"
		}
	}
	return merged
}
