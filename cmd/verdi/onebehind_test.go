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

// TestEvaluateOneBehindReport_ChangeShapes is clause (2)'s status half (L3b
// review m-1): HEAD's sole change must ADD or MODIFY the spec's own report.
// A report renamed into place from another spec, a deleted report, and a
// file-to-symlink type change each touch exactly that one path yet are
// refused by name; a mode-only change is refused too, by the covers clause,
// since it carries the parent's bytes, whose covers can never name the
// parent itself. Each row first proves its diff really has the one-entry
// shape it names, so it can never degrade into a two-path refusal.
func TestEvaluateOneBehindReport_ChangeShapes(t *testing.T) {
	ctx := context.Background()
	rel := store.DeviationReportRelPath(store.ZoneActive, oneBehindReportSpecName)
	cases := []struct {
		name string
		// build commits the shape on top of repo.Head and returns HEAD.
		build      func(t *testing.T, repo *fixturegit.Repo) string
		wantStatus string
		wantReason string
	}{
		{
			name: "another spec's report renamed into this path, covers edited to the parent",
			build: func(t *testing.T, repo *fixturegit.Repo) string {
				otherRel := store.DeviationReportRelPath(store.ZoneActive, "other-spec")
				// A long, unchanged body keeps git's rename detection above
				// its threshold across the covers edit.
				body := strings.Repeat("a line of report body that the rename keeps unchanged\n", 60)
				writeOneBehindFile(t, filepath.Join(repo.Dir, filepath.FromSlash(otherRel)), oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML)+body)
				parent := commitAllOnCurrentBranch(t, repo.Dir, "another spec's report")
				if err := os.MkdirAll(filepath.Dir(filepath.Join(repo.Dir, filepath.FromSlash(rel))), 0o755); err != nil {
					t.Fatal(err)
				}
				runGitCmd(t, repo.Dir, "mv", otherRel, rel)
				writeOneBehindFile(t, filepath.Join(repo.Dir, filepath.FromSlash(rel)), oneBehindReportContent(parent, oneBehindDispositionedFindingYAML)+body)
				return commitAllOnCurrentBranch(t, repo.Dir, "rename another spec's report into place")
			},
			wantStatus: "R",
			wantReason: "(status R)",
		},
		{
			name: "the report deleted",
			build: func(t *testing.T, repo *fixturegit.Repo) string {
				commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML))
				runGitCmd(t, repo.Dir, "rm", "-q", rel)
				return commitAllOnCurrentBranch(t, repo.Dir, "delete the report")
			},
			wantStatus: "D",
			wantReason: "(status D)",
		},
		{
			name: "the report replaced by a symlink (a type change)",
			build: func(t *testing.T, repo *fixturegit.Repo) string {
				commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML))
				full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
				if err := os.Remove(full); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../other-spec/deviation-report.md", full); err != nil {
					t.Fatal(err)
				}
				return commitAllOnCurrentBranch(t, repo.Dir, "replace the report with a symlink")
			},
			wantStatus: "T",
			wantReason: "(status T)",
		},
		{
			name: "a mode-only change to the report",
			build: func(t *testing.T, repo *fixturegit.Repo) string {
				commitOneBehindReport(t, ctx, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML))
				if err := os.Chmod(filepath.Join(repo.Dir, filepath.FromSlash(rel)), 0o755); err != nil {
					t.Fatal(err)
				}
				return commitAllOnCurrentBranch(t, repo.Dir, "make the report executable")
			},
			wantStatus: "M",
			wantReason: "not HEAD's parent",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := oneBehindBaseRepo(t)
			head := tc.build(t, repo)
			parent, err := gitx.RevParse(ctx, repo.Dir, head+"^")
			if err != nil {
				t.Fatalf("RevParse(%s^): %v", head, err)
			}
			entries, err := gitx.DiffNameStatus(ctx, repo.Dir, parent, head)
			if err != nil {
				t.Fatalf("DiffNameStatus: %v", err)
			}
			if len(entries) != 1 || entries[0].Path != rel || entries[0].Status != tc.wantStatus {
				t.Fatalf("fixture diff = %+v, want the single %s entry for %s", entries, tc.wantStatus, rel)
			}

			got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
			if err != nil {
				t.Fatalf("evaluateOneBehindReport: %v, want a named refusal, not an operational error", err)
			}
			if got.Accepted {
				t.Fatalf("Accepted = true, want false for a %s change", tc.wantStatus)
			}
			if !strings.Contains(got.Reason, tc.wantReason) {
				t.Fatalf("Reason = %q, want it to contain %q", got.Reason, tc.wantReason)
			}
		})
	}
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
		// detail is how the reason says the file differs, ahead of "the
		// report HEAD <sha> commits".
		detail string
	}{
		{
			name:   "a disposition amended after the report commit",
			detail: "is not byte-identical to",
			mutate: func(t *testing.T, reportPath, parent string) {
				writeOneBehindFile(t, reportPath, oneBehindRenderedReport(t, parent, artifact.FindingAcceptedDeviation, "not fixed after all"))
			},
		},
		{
			name:   "a finding retracted to undispositioned after the report commit",
			detail: "is not byte-identical to",
			mutate: func(t *testing.T, reportPath, parent string) {
				writeOneBehindFile(t, reportPath, oneBehindRenderedReport(t, parent, "", ""))
			},
		},
		{
			name:   "one byte appended to the committed report",
			detail: "is not byte-identical to",
			mutate: func(t *testing.T, reportPath, parent string) {
				writeOneBehindFile(t, reportPath, oneBehindRenderedReport(t, parent, artifact.FindingFixed, "")+"\n")
			},
		},
		{
			name:   "the report deleted from the working tree",
			detail: "is absent, unlike",
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
			if want := reportPath + " " + tc.detail + " the report HEAD " + head + " commits"; !strings.Contains(got.Reason, want) {
				t.Fatalf("Reason = %q, want it to say %q", got.Reason, want)
			}
		})
	}
}

func writeOneBehindFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestEvaluateOneBehindReport_NestedStore is clause (2)'s coordinate system
// (L3b review m-4): the store root may sit below the git root (store.FindRoot
// walks up to the nearest .verdi), while DiffNameStatus and Show answer in
// repository-root-relative paths, so the predicate re-bases the report's
// store-relative path through gitx.RepoPrefix, as requireCleanIndex does.
// The diff.relative rows (BL-34; L3b re-review RR-7) prove those paths stay
// repository-root-relative whatever diff.relative says.
func TestEvaluateOneBehindReport_NestedStore(t *testing.T) {
	ctx := context.Background()
	rel := store.DeviationReportRelPath(store.ZoneActive, oneBehindReportSpecName)
	const manifest = "schema: verdi.layout/v1\nforge: gitlab\n"

	t.Run("a single nested store accepts its own committed report", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files:   map[string]string{"sub/.verdi/verdi.yaml": manifest},
			Message: "nested store",
		}})
		sub := filepath.Join(repo.Dir, "sub")
		writeOneBehindFile(t, filepath.Join(sub, filepath.FromSlash(rel)), oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML))
		head := commitAllOnCurrentBranch(t, repo.Dir, "R in the nested store")

		got, err := evaluateOneBehindReport(ctx, sub, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if !got.Accepted || got.Parent != repo.Head {
			t.Fatalf("got Accepted=%v Parent=%q Reason=%q, want the nested store's own one-behind report accepted with parent %q", got.Accepted, got.Parent, got.Reason, repo.Head)
		}
	})

	t.Run("a nested store refuses the root store's same-named report", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				".verdi/verdi.yaml":     manifest,
				"sub/.verdi/verdi.yaml": manifest,
			},
			Message: "two stores",
		}})
		sub := filepath.Join(repo.Dir, "sub")
		content := oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML)
		writeOneBehindFile(t, filepath.Join(repo.Dir, filepath.FromSlash(rel)), content)
		head := commitAllOnCurrentBranch(t, repo.Dir, "R in the ROOT store")
		// The nested store's own working-tree file holds the very same
		// bytes, so only the path clause can tell the two stores apart.
		writeOneBehindFile(t, filepath.Join(sub, filepath.FromSlash(rel)), content)

		got, err := evaluateOneBehindReport(ctx, sub, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false: the nested store accepted the root store's report")
		}
		if want := "not an added/modified sub/" + rel; !strings.Contains(got.Reason, want) {
			t.Fatalf("Reason = %q, want the path clause naming the nested store's own report (%q)", got.Reason, want)
		}
	})

	t.Run("diff.relative never hides a change outside a nested store", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files:   map[string]string{"sub/.verdi/verdi.yaml": manifest},
			Message: "nested store",
		}})
		runGitCmd(t, repo.Dir, "config", "diff.relative", "true")
		sub := filepath.Join(repo.Dir, "sub")
		writeOneBehindFile(t, filepath.Join(sub, filepath.FromSlash(rel)), oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML))
		writeOneBehindFile(t, filepath.Join(repo.Dir, "main.go"), "package main\n")
		head := commitAllOnCurrentBranch(t, repo.Dir, "R plus code outside the nested store")

		got, err := evaluateOneBehindReport(ctx, sub, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false: a commit that also changes code outside the store is never SI-231's shape")
		}
		// The diff itself must see main.go (gitx passes --no-relative), so
		// the refusal is the path count, never an accident of a hidden path.
		if want := "changes 2 path(s), not exactly one"; !strings.Contains(got.Reason, want) {
			t.Fatalf("Reason = %q, want the path-count clause (%q): the change outside the store must be counted", got.Reason, want)
		}
	})

	// BL-34 (L3b residual 1): diff.relative no longer makes a nested store
	// refuse its own genuine report, at the predicate or at condition 4.
	t.Run("diff.relative accepts a nested store's genuine one-behind report", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files:   map[string]string{"sub/.verdi/verdi.yaml": manifest},
			Message: "nested store",
		}})
		runGitCmd(t, repo.Dir, "config", "diff.relative", "true")
		sub := filepath.Join(repo.Dir, "sub")
		writeOneBehindFile(t, filepath.Join(sub, filepath.FromSlash(rel)), oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML))
		head := commitAllOnCurrentBranch(t, repo.Dir, "R in the nested store")

		got, err := evaluateOneBehindReport(ctx, sub, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if !got.Accepted || got.Parent != repo.Head {
			t.Fatalf("got Accepted=%v Parent=%q Reason=%q, want the nested store's own one-behind report accepted under diff.relative with parent %q", got.Accepted, got.Parent, got.Reason, repo.Head)
		}

		spec := &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/" + oneBehindReportSpecName}}
		cond, err := checkDispositionCompleteCondition(ctx, sub, spec, head)
		if err != nil {
			t.Fatalf("checkDispositionCompleteCondition: %v", err)
		}
		if !cond.OK {
			t.Fatalf("condition 4 OK = false under diff.relative, want true; Reason=%q", cond.Reason)
		}
	})

	// L3b re-review RR-7 made permanent: under diff.relative, run from sub/,
	// plain git reports sub/sub/.verdi/…/deviation-report.md as
	// sub/.verdi/…/deviation-report.md, which is exactly this nested store's
	// re-based path, and only the covers clause refused. With
	// repository-root paths the path clause itself refuses.
	t.Run("diff.relative cannot make a deeper store's report collide with this store's", func(t *testing.T) {
		repo := fixturegit.Build(t, []fixturegit.Layer{{
			Files: map[string]string{
				"sub/.verdi/verdi.yaml":     manifest,
				"sub/sub/.verdi/verdi.yaml": manifest,
				// This store's own report, committed before the parent: the
				// unchanged file the predicate would read at HEAD.
				"sub/" + rel: oneBehindReportContent(strings.Repeat("a", 40), oneBehindDispositionedFindingYAML),
			},
			Message: "two nested stores",
		}})
		runGitCmd(t, repo.Dir, "config", "diff.relative", "true")
		sub := filepath.Join(repo.Dir, "sub")
		writeOneBehindFile(t, filepath.Join(sub, "sub", filepath.FromSlash(rel)), oneBehindReportContent(repo.Head, oneBehindDispositionedFindingYAML))
		head := commitAllOnCurrentBranch(t, repo.Dir, "R in the deeper store")

		got, err := evaluateOneBehindReport(ctx, sub, oneBehindReportSpecName, head)
		if err != nil {
			t.Fatalf("evaluateOneBehindReport: %v", err)
		}
		if got.Accepted {
			t.Fatal("Accepted = true, want false: the deeper store's report is not this store's")
		}
		if want := "sole changed path is sub/sub/" + rel; !strings.Contains(got.Reason, want) {
			t.Fatalf("Reason = %q, want the path clause naming the deeper store's repository-root path (%q)", got.Reason, want)
		}
	})
}

// TestEvaluateOneBehindReport_ShallowCheckout is clause (1) at a shallow
// boundary (L3b review m-5): a depth-1 clone cannot see HEAD's parent, so
// whether HEAD has exactly one parent is unknown, and the refusal says so
// rather than calling HEAD "a root commit or a merge". A depth-2 clone sees
// the parent and accepts.
func TestEvaluateOneBehindReport_ShallowCheckout(t *testing.T) {
	ctx := context.Background()
	src := oneBehindBaseRepo(t)
	head := commitOneBehindReport(t, ctx, src.Dir, oneBehindReportSpecName, oneBehindReportContent(src.Head, oneBehindDispositionedFindingYAML))

	for _, tc := range []struct {
		depth        string
		wantAccepted bool
	}{
		{depth: "1", wantAccepted: false},
		{depth: "2", wantAccepted: true},
	} {
		t.Run("depth "+tc.depth, func(t *testing.T) {
			parentDir := t.TempDir()
			runGitCmd(t, parentDir, "clone", "--quiet", "--depth", tc.depth, "file://"+src.Dir, "clone")
			clone := filepath.Join(parentDir, "clone")
			if shallow, err := gitx.IsShallow(ctx, clone); err != nil || !shallow {
				t.Fatalf("fixture: IsShallow(clone) = %v, %v, want a shallow clone", shallow, err)
			}

			got, err := evaluateOneBehindReport(ctx, clone, oneBehindReportSpecName, head)
			if err != nil {
				t.Fatalf("evaluateOneBehindReport: %v", err)
			}
			if got.Accepted != tc.wantAccepted {
				t.Fatalf("Accepted = %v, want %v; Reason=%q", got.Accepted, tc.wantAccepted, got.Reason)
			}
			if tc.wantAccepted {
				return
			}
			if !strings.Contains(got.Reason, "shallow checkout") {
				t.Fatalf("Reason = %q, want it to name the shallow checkout", got.Reason)
			}
			if strings.Contains(got.Reason, "a root commit or a merge") {
				t.Fatalf("Reason = %q, want no root-or-merge claim at a shallow boundary, where the parent count is unknown", got.Reason)
			}
		})
	}
}

// .gitmodules formats for the lib submodule the submodule rows below
// record; %s is its url, the local path of oneBehindLibRepo's repository.
const (
	oneBehindPlainGitmodules    = "[submodule \"lib\"]\n\tpath = lib\n\turl = %s\n"
	oneBehindIgnoringGitmodules = "[submodule \"lib\"]\n\tpath = lib\n\turl = %s\n\tignore = all\n"
)

// oneBehindLibRepo builds the local second repository the submodule rows
// point lib at, returning its path and its two commits A and B. Nothing is
// ever cloned from it, so no file:// protocol and no network is involved.
func oneBehindLibRepo(t *testing.T) (dir, commitA, commitB string) {
	t.Helper()
	lib := fixturegit.Build(t, []fixturegit.Layer{
		{Files: map[string]string{"lib.go": "package lib // A\n"}, Message: "lib commit A"},
		{Files: map[string]string{"lib.go": "package lib // B\n"}, Message: "lib commit B"},
	})
	return lib.Dir, lib.Heads[0], lib.Heads[1]
}

// stageOneBehindGitlink stages the gitlink (mode 160000) lib at sha, the
// index entry `git submodule update` + `git add lib` leaves. lib/ is kept an
// empty directory, as an uninitialized submodule's is, so a later
// `git add -A` leaves the entry alone rather than staging its deletion.
func stageOneBehindGitlink(t *testing.T, dir, sha string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatalf("mkdir lib: %v", err)
	}
	runGitCmd(t, dir, "update-index", "--add", "--cacheinfo", "160000,"+sha+",lib")
}

// commitOneBehindReportWithGitlinkBump is RR-8's commit R (L3b re-review
// N-1): the spec's report written and staged together with the gitlink lib
// bumped to sha, committed as ONE single-parent commit. Returns R.
func commitOneBehindReportWithGitlinkBump(t *testing.T, dir, specName, content, sha string) string {
	t.Helper()
	rel := store.DeviationReportRelPath(store.ZoneActive, specName)
	writeOneBehindFile(t, filepath.Join(dir, filepath.FromSlash(rel)), content)
	runGitCmd(t, dir, "add", "--", rel)
	stageOneBehindGitlink(t, dir, sha)
	runGitCmd(t, dir, "commit", "--quiet", "--no-verify", "-m", "commit the report and bump lib")
	return strings.TrimSpace(gitOutput(t, dir, "rev-parse", "HEAD"))
}

// assertOneBehindSubmoduleFixture proves parent..head really changes the two
// paths RR-8 names (the report added, lib bumped), and that plain
// `git diff --name-status` in dir, under dir's configuration, reports
// plainDiff: the row's ignore setting must really blind plain git.
func assertOneBehindSubmoduleFixture(t *testing.T, dir, parent, head, reportRel, plainDiff string) {
	t.Helper()
	if got, want := strings.TrimSpace(gitOutput(t, dir, "diff", "--name-status", "--ignore-submodules=none", parent, head)), "A\t"+reportRel+"\nM\tlib"; got != want {
		t.Fatalf("fixture: the true diff = %q, want %q", got, want)
	}
	if got := strings.TrimSpace(gitOutput(t, dir, "diff", "--name-status", parent, head)); got != plainDiff {
		t.Fatalf("fixture: plain `git diff --name-status` = %q, want %q", got, plainDiff)
	}
}

// TestEvaluateOneBehindReport_SubmoduleBumpIsNeverHidden is L3b re-review
// N-1 at the predicate: a commit R that adds the spec's report AND bumps a
// submodule changes two paths, so it is never SI-231's shape, whatever
// submodule `ignore` setting would hide the bump from plain git. Before gitx
// passed --ignore-submodules=none, each ignore row was ACCEPTED: close froze
// the report and its covers named a parent whose code HEAD no longer has.
func TestEvaluateOneBehindReport_SubmoduleBumpIsNeverHidden(t *testing.T) {
	ctx := context.Background()
	rel := store.DeviationReportRelPath(store.ZoneActive, oneBehindReportSpecName)
	reportOnly := "A\t" + rel
	cases := []struct {
		name       string
		gitmodules string
		config     [][2]string
		plainDiff  string
	}{
		{name: "no configuration (control)", gitmodules: oneBehindPlainGitmodules, plainDiff: reportOnly + "\nM\tlib"},
		{name: "local diff.ignoreSubmodules=all", gitmodules: oneBehindPlainGitmodules, config: [][2]string{{"diff.ignoreSubmodules", "all"}}, plainDiff: reportOnly},
		{name: "local submodule.<name>.ignore=all", gitmodules: oneBehindPlainGitmodules, config: [][2]string{{"submodule.lib.ignore", "all"}}, plainDiff: reportOnly},
		{name: "committed .gitmodules ignore = all", gitmodules: oneBehindIgnoringGitmodules, plainDiff: reportOnly},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := oneBehindBaseRepo(t)
			libDir, libA, libB := oneBehindLibRepo(t)
			writeOneBehindFile(t, filepath.Join(repo.Dir, ".gitmodules"), fmt.Sprintf(tc.gitmodules, libDir))
			runGitCmd(t, repo.Dir, "add", "--", ".gitmodules")
			stageOneBehindGitlink(t, repo.Dir, libA)
			runGitCmd(t, repo.Dir, "commit", "--quiet", "--no-verify", "-m", "record lib at commit A")
			parent := strings.TrimSpace(gitOutput(t, repo.Dir, "rev-parse", "HEAD"))
			head := commitOneBehindReportWithGitlinkBump(t, repo.Dir, oneBehindReportSpecName, oneBehindReportContent(parent, oneBehindDispositionedFindingYAML), libB)
			for _, kv := range tc.config {
				runGitCmd(t, repo.Dir, "config", kv[0], kv[1])
			}
			assertOneBehindSubmoduleFixture(t, repo.Dir, parent, head, rel, tc.plainDiff)

			got, err := evaluateOneBehindReport(ctx, repo.Dir, oneBehindReportSpecName, head)
			if err != nil {
				t.Fatalf("evaluateOneBehindReport: %v", err)
			}
			if got.Accepted {
				t.Fatal("Accepted = true, want false: R also bumps the submodule lib, so HEAD's code is not the audited parent's")
			}
			if want := "changes 2 path(s), not exactly one"; !strings.Contains(got.Reason, want) {
				t.Fatalf("Reason = %q, want the path-count clause (%q)", got.Reason, want)
			}
		})
	}
}
