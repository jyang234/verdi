package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/align"
	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// TestEvaluateOneBehindReport is SI-231's static register for the ONE
// predicate (ledger contract item 1): every negative clause shape refuses,
// naming the offending clause, and the accepted shape returns HEAD's
// parent, the decoded report, and its body for a caller to freeze without a
// second read.

// oneBehindReportSpecName is this file's own isolated spec name — every
// case here builds its own fresh fixturegit repo, so nothing here is ever
// shared with another file's "stale-decline" fixtures.
const oneBehindReportSpecName = "one-behind-spec"

const oneBehindDispositionedFindingYAML = `  - { id: f-1, kind: computed, text: "boundary holds", disposition: fixed }
`
const oneBehindUndispositionedFindingYAML = `  - { id: f-1, kind: computed, text: "boundary holds" }
`

// oneBehindBaseRepo builds a one-layer fixturegit repo with a minimal store
// manifest and no deviation-report.md yet — the "parent" commit
// (repo.Head) each case in this file commits its own report (or unrelated
// changes) on top of.
func oneBehindBaseRepo(t *testing.T) *fixturegit.Repo {
	t.Helper()
	return fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\nforge: gitlab\n"},
		Message: "store root",
	}})
}

func oneBehindReportContent(covers, findingsYAML string) string {
	return fmt.Sprintf(`---
schema: verdi.deviation/v1
covers: %s
findings:
%s
digest: sha256:%s
---
# Alignment report
`, covers, findingsYAML, strings.Repeat("0", 64))
}

// oneBehindRenderedReport renders a legal one-finding (f-1) report through
// the production renderer, so the body carries the finding's rendered line
// `verdi disposition` locates before it amends a disposition — the
// hand-written oneBehindReportContent body ("# Alignment report" alone)
// cannot be amended by that verb.
func oneBehindRenderedReport(t *testing.T, covers string, disposition artifact.FindingDisposition, note string) string {
	t.Helper()
	findings := []artifact.Finding{{ID: "f-1", Kind: artifact.FindingComputed, Text: "boundary holds", Disposition: disposition, Note: note}}
	fm := &artifact.DeviationFrontmatter{Schema: "verdi.deviation/v1", Covers: covers, Findings: findings, Digest: "sha256:" + strings.Repeat("0", 64)}
	if err := fm.Validate(); err != nil {
		t.Fatalf("oneBehindRenderedReport fixture is invalid: %v", err)
	}
	return string(align.RenderMarkdown(fm, align.RenderBody(findings, nil, nil, nil, nil, nil)))
}

// oneBehindWorkingTreeDivergence is the refusal phrase SI-231's working-tree
// clause names (ledger row as amended at L3b review I-1, ruling R-W1-9).
const oneBehindWorkingTreeDivergence = "the working-tree report differs from the committed report; commit or discard the change"

// commitOneBehindReport writes deviation-report.md for specName with
// content, stages ONLY that path, and commits ONLY that path — the exact
// shape a real closure-MR "commit R" (03 §Closure ritual) leaves: a
// single-parent commit whose sole change is the report file.
func commitOneBehindReport(t *testing.T, ctx context.Context, dir, specName, content string) string {
	t.Helper()
	rel := store.DeviationReportRelPath(store.ZoneActive, specName)
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	if err := gitx.AddPaths(ctx, dir, rel); err != nil {
		t.Fatalf("AddPaths(%s): %v", rel, err)
	}
	sha, err := gitx.CreateCommitPaths(ctx, dir, "commit the dispositioned alignment report", rel)
	if err != nil {
		t.Fatalf("CreateCommitPaths(%s): %v", rel, err)
	}
	return sha
}

func TestEvaluateOneBehindReport(t *testing.T) {
	ctx := context.Background()

	t.Run("accepted shape", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		parent := repo.Head
		head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(parent, oneBehindDispositionedFindingYAML))

		got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if !got.Accepted {
			t.Fatalf("Accepted = false, want true; Reason=%q", got.Reason)
		}
		if got.Parent != parent {
			t.Fatalf("Parent = %q, want %q", got.Parent, parent)
		}
		if got.Report == nil || got.Report.Covers != parent {
			t.Fatalf("Report = %+v, want Covers %q", got.Report, parent)
		}
		if len(got.Report.Findings) != 1 || got.Report.Findings[0].ID != "f-1" {
			t.Fatalf("Report.Findings = %+v, want the single f-1 finding", got.Report.Findings)
		}
	})

	t.Run("merge commit", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		checkoutBranch(t, repo.Dir, "side")
		if err := os.WriteFile(filepath.Join(repo.Dir, "side.txt"), []byte("side\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		sideHead := commitAllOnCurrentBranch(t, repo.Dir, "side change")
		if err := gitx.Checkout(ctx, repo.Dir, "main"); err != nil {
			t.Fatalf("Checkout(main): %v", err)
		}
		if err := os.WriteFile(filepath.Join(repo.Dir, "main.txt"), []byte("main\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		commitAllOnCurrentBranch(t, repo.Dir, "main change")
		runGitCmd(t, repo.Dir, "merge", "--quiet", "--no-ff", "-m", "merge side "+sideHead, "side")
		mergeHead, err := gitx.RevParse(ctx, repo.Dir, "HEAD")
		if err != nil {
			t.Fatalf("RevParse(HEAD): %v", err)
		}

		got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, mergeHead)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false (a merge commit has two parents)")
		}
		if !strings.Contains(got.Reason, "single-parent commit") {
			t.Fatalf("Reason = %q, want it to name the single-parent clause", got.Reason)
		}
	})

	t.Run("a second path changed", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		parent := repo.Head
		rel := store.DeviationReportRelPath(store.ZoneActive, oneBehindReportSpecName)
		full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(oneBehindReportContent(parent, oneBehindDispositionedFindingYAML)), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo.Dir, "other.txt"), []byte("other\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		head := commitAllOnCurrentBranch(t, repo.Dir, "report + unrelated file (X-16 shape)")

		got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false (two paths changed)")
		}
		if !strings.Contains(got.Reason, "2 path") {
			t.Fatalf("Reason = %q, want it to name the path count", got.Reason)
		}
	})

	t.Run("covers is not the parent", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent("0000000000000000000000000000000000000b", oneBehindDispositionedFindingYAML))

		got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false (covers is not HEAD's parent)")
		}
		if !strings.Contains(got.Reason, "covers 0000000000000000000000000000000000000b, not HEAD's parent") {
			t.Fatalf("Reason = %q, want it to name the covers mismatch", got.Reason)
		}
	})

	t.Run("an undispositioned finding", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		parent := repo.Head
		head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(parent, oneBehindUndispositionedFindingYAML))

		got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false (an undispositioned finding)")
		}
		if !strings.Contains(got.Reason, "undispositioned finding(s) [f-1]") {
			t.Fatalf("Reason = %q, want it to name the undispositioned finding", got.Reason)
		}
	})

	t.Run("frozen", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		parent := repo.Head
		content := "---\nschema: verdi.deviation/v1\ncovers: " + parent + "\nfindings:\n" +
			oneBehindDispositionedFindingYAML +
			"frozen: { at: 2024-01-01, commit: " + parent + " }\ndigest: sha256:" + strings.Repeat("0", 64) + "\n---\n# Alignment report\n"
		head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, content)

		got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false (report is already frozen)")
		}
		if !strings.Contains(got.Reason, "already frozen") {
			t.Fatalf("Reason = %q, want it to name the frozen clause", got.Reason)
		}
	})

	t.Run("no report at HEAD", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		if err := os.WriteFile(filepath.Join(repo.Dir, "unrelated.txt"), []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		head := commitAllOnCurrentBranch(t, repo.Dir, "unrelated change")

		got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false (HEAD carries no report change at all)")
		}
		if !strings.Contains(got.Reason, "unrelated.txt") {
			t.Fatalf("Reason = %q, want it to name the actual sole changed path", got.Reason)
		}
	})

	t.Run("the report of a different spec changed", func(t *testing.T) {
		repo := oneBehindBaseRepo(t)
		parent := repo.Head
		head := commitOneBehindReport(t, ctx, repo.Dir, "some-other-spec", oneBehindReportContent(parent, oneBehindDispositionedFindingYAML))

		got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false (the committed report belongs to a different spec)")
		}
		if !strings.Contains(got.Reason, "some-other-spec") {
			t.Fatalf("Reason = %q, want it to name the other spec's own report path", got.Reason)
		}
	})
}

// TestEvaluateOneBehindReport_WorkingTreeMustEqualHEAD is SI-231's
// working-tree clause (ledger row as amended at L3b review I-1, ruling
// R-W1-9): an otherwise accepted one-behind commit is refused, by name,
// whenever the spec's deviation-report.md on disk is not byte-identical to
// the report HEAD commits — otherwise close would freeze HEAD's bytes over
// an operator's uncommitted disposition change and report it preserved.
func TestEvaluateOneBehindReport_WorkingTreeMustEqualHEAD(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		mutate func(t *testing.T, reportPath, parent string)
	}{
		{
			name: "a disposition amended after the report commit",
			mutate: func(t *testing.T, reportPath, parent string) {
				writeOneBehindFile(t, reportPath, oneBehindRenderedReport(t, parent, artifact.FindingAcceptedDeviation, "not fixed after all"))
			},
		},
		{
			name: "a finding retracted to undispositioned after the report commit",
			mutate: func(t *testing.T, reportPath, parent string) {
				writeOneBehindFile(t, reportPath, oneBehindRenderedReport(t, parent, "", ""))
			},
		},
		{
			name: "one byte appended to the committed report",
			mutate: func(t *testing.T, reportPath, parent string) {
				writeOneBehindFile(t, reportPath, oneBehindRenderedReport(t, parent, artifact.FindingFixed, "")+"\n")
			},
		},
		{
			name: "the report deleted from the working tree",
			mutate: func(t *testing.T, reportPath, _ string) {
				if err := os.Remove(reportPath); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := oneBehindBaseRepo(t)
			parent := repo.Head
			head := commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindRenderedReport(t, parent, artifact.FindingFixed, ""))
			reportPath := store.DeviationReportPath(repo.Dir, store.ZoneActive, oneBehindReportSpecName)

			clean, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
			if err != nil {
				t.Fatalf("evaluateOneBehindReport (clean working tree): %v", err)
			}
			if !clean.Accepted {
				t.Fatalf("clean working tree: Accepted = false, want true (the fixture must isolate the working-tree clause); Reason=%q", clean.Reason)
			}

			tc.mutate(t, reportPath, parent)
			got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
			if err != nil {
				t.Fatalf("evaluateOneBehindReport: %v", err)
			}
			if got.Accepted {
				t.Fatal("Accepted = true, want false (the working-tree report is not HEAD's committed report)")
			}
			if !strings.Contains(got.Reason, oneBehindWorkingTreeDivergence) {
				t.Fatalf("Reason = %q, want it to name the working-tree clause %q", got.Reason, oneBehindWorkingTreeDivergence)
			}
		})
	}
}

func writeOneBehindFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
