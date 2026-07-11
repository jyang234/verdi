// verdi accept <spec-ref> (05 §CLI, PLAN.md Phase 7): the design branch's
// final action — mechanically flips a draft feature spec's
// `status: draft -> accepted-pending-build` and writes the frozen stamp
// (`commit` = the content-final sha it supersedes, `at` = that commit's
// own committer date — never wall clock), then commits the flip. Merging
// the resulting spec MR to main *is* acceptance (03 §Lifecycle: two MRs).
// Kept in its own file per the lint.go/sync.go/matrix.go/dex.go
// convention.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/OWNER/verdi/internal/artifact"
	"github.com/OWNER/verdi/internal/gitx"
	"github.com/OWNER/verdi/internal/store"
)

// draftStatusLineRe matches the scaffold's own `status: draft` frontmatter
// line (design.go's scaffoldDraftSpec always writes exactly this form),
// tolerating an optional surrounding quote so a human's re-quoting edit
// during the design branch does not break the flip.
var draftStatusLineRe = regexp.MustCompile(`(?m)^status:\s*"?draft"?\s*$`)

// cmdAccept is `verdi accept`'s entry point, invoked by dispatch.go.
func cmdAccept(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "accept: usage: verdi accept <spec-ref> (e.g. spec/stale-decline)")
		return 2
	}
	specArg := args[0]

	ctx := context.Background()
	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	return runAccept(ctx, root, specArg, stdout, stderr)
}

// runAccept is the testable core: given an already-resolved root, run the
// whole accept ritual and return the exit code (CLAUDE.md: 0 clean,
// 1 verdict — the spec fails an accept precondition — 2 operational).
func runAccept(ctx context.Context, root, specArg string, stdout, stderr io.Writer) int {
	ref, err := artifact.ParseRef(specArg)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Pinned() {
		fmt.Fprintf(stderr, "accept: %q is not a spec ref (want spec/<name>, e.g. spec/stale-decline)\n", specArg)
		return 2
	}

	specPath := filepath.Join(root, ".verdi", "specs", "active", ref.Name, "spec.md")
	raw, err := os.ReadFile(specPath)
	if err != nil {
		fmt.Fprintf(stderr, "accept: reading %s: %v\n", specPath, err)
		return 2
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: %v\n", specPath, err)
		return 2
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: %v\n", specPath, err)
		return 2
	}

	if spec.Class != artifact.ClassFeature {
		fmt.Fprintf(stderr, "accept: %s is a %s spec (no story, no acceptance criteria); only a feature spec can be accepted\n", ref.String(), spec.Class)
		return 1
	}
	if spec.Status != "draft" {
		fmt.Fprintf(stderr, "accept: %s status is %q, not draft; only a draft feature spec can be accepted\n", ref.String(), spec.Status)
		return 1
	}

	if !draftStatusLineRe.Match(fm) {
		fmt.Fprintf(stderr, "accept: %s: internal error: decoded status is draft, but no status: draft frontmatter line was found to flip\n", specPath)
		return 2
	}
	if n := len(draftStatusLineRe.FindAllIndex(fm, -1)); n != 1 {
		fmt.Fprintf(stderr, "accept: %s: internal error: expected exactly one status: draft line, found %d\n", specPath, n)
		return 2
	}

	preFlipHead, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	commitDate, err := gitx.CommitDate(ctx, root, preFlipHead)
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	if len(commitDate) < 10 {
		fmt.Fprintf(stderr, "accept: internal error: commit date %q too short to derive a YYYY-MM-DD frozen.at\n", commitDate)
		return 2
	}
	at := commitDate[:10]

	newFm := draftStatusLineRe.ReplaceAll(fm, []byte("status: accepted-pending-build"))
	newFm = append(newFm, []byte(fmt.Sprintf("\nfrozen: { at: %s, commit: %s }", at, preFlipHead))...)

	// Self-validate the flipped content before writing anything to disk
	// (CLAUDE.md: "never fake success").
	flipped, err := artifact.DecodeSpec(newFm)
	if err != nil {
		fmt.Fprintln(stderr, "accept: internal error: flipped frontmatter failed self-validation:", err)
		return 2
	}
	if flipped.Status != "accepted-pending-build" || flipped.Frozen == nil || flipped.Frozen.Commit != preFlipHead {
		fmt.Fprintln(stderr, "accept: internal error: flipped frontmatter does not carry the expected status/frozen stamp")
		return 2
	}

	newContent := "---\n" + string(newFm) + "\n---\n" + string(body)
	if err := os.WriteFile(specPath, []byte(newContent), 0o644); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	if err := gitx.AddAll(ctx, root); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	if _, err := gitx.CreateCommit(ctx, root, fmt.Sprintf("accept: %s draft -> accepted-pending-build", ref.String())); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	fmt.Fprintf(stdout, "accept: %s status: draft -> accepted-pending-build\n", ref.String())
	fmt.Fprintf(stdout, "accept: frozen: { at: %s, commit: %s }\n", at, preFlipHead)
	return 0
}
