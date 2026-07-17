// verdi accept diagram/<name> (spec/proposal-artifact ac-3, dc-2): the
// diagram-ref half of the accept verb's dispatch (accept.go). Narrower than
// the spec ritual — a class: proposal diagram carries no ACs or stubs to
// match against — it requires class: proposal and status: proposed, flips
// status to accepted, and writes frozen: {at, commit}, mirroring the spec
// ritual's core mechanical flip (status-line replace, frozen append,
// self-validate before write, auto-commit) but with no stub-match, no
// CODEOWNERS routing, and no supersedes cascade. Kept in its own file per
// the lint.go/sync.go/matrix.go/dex.go/blastradius.go/stubmatch.go/
// supersede.go convention accept.go's own doc comment already names.
//
// # spec/proposal-artifact ac-2's write-path inventory
//
// ac-2 requires every real write path that can persist a kind: diagram
// file to be named and shown to touch frontmatter bytes only, never the
// mermaid body. As of this story, the inventory is:
//
//  1. runAcceptDiagram below (this file) — the ONE write path this story
//     adds. It reads the whole file, artifact.SplitFrontmatter's the
//     frontmatter from the body ONCE, edits the frontmatter slice alone
//     (a status-line regexp.ReplaceAll plus an appended frozen: line —
//     never a parse/re-marshal of the whole struct), and re-emits
//     "---\n" + editedFrontmatter + "\n---\n" + the ORIGINAL body slice,
//     untouched. See TestRunAccept_Diagram_ByteIdentityRegression
//     (acceptdiagram_test.go) for the SHA-256 regression proof.
//  2. cmd/verdi/accept.go's spec ritual — writes only
//     .verdi/specs/active/<name>/spec.md; it never touches
//     .verdi/diagrams (this file's runAccept dispatch sends diagram/...
//     refs here instead, before the spec ritual's own file I/O runs).
//  3. cmd/verdi/design.go's "verdi design start" scaffold — writes only
//     .verdi/specs/active/<name>/spec.md (runDesignStart); it never
//     scaffolds a diagram file. Named explicitly so this enumeration does
//     not silently omit it (D6-18): today, "verdi design start" has NO
//     diagram-scaffolding path at all.
//  4. internal/workbench's board/spec save API (board.go, boardspecapi.go,
//     boardpin.go) — every write there goes through boardSpecServer.
//     specDir(name), hardcoded to .verdi/specs/active/<name>. The board
//     can PIN a diagram as a link (an ordinary links: edge into a spec,
//     per spec/diagram-proposals' own problem statement) but never writes
//     bytes into the diagram file itself. AMENDED by spec/board-editor:
//     the diagram-proposal EDITOR (boarddiagram.go) is now a real write
//     path into .verdi/diagrams/<name>.mermaid — the mirror image of this
//     inventory's rule: its every write (save, structural op, reset)
//     replaces the BODY with the author's exact bytes and splices the
//     frontmatter prefix back UNTOUCHED (boardDiagramServer.writeBody;
//     spec/board-editor ac-3's static obligation is its byte-identity
//     proof). Named explicitly rather than silently omitted, matching
//     this story's own obligation (spec/proposal-artifact ac-2--static):
//     "an enumeration that turns up a path this story does not yet cover
//     must say so explicitly."
//  5. cmd/e2eharness/provision.go's copyTree/copyFile — copies
//     examples/showcase/.verdi/diagrams/loansvc-topology.mermaid wholesale
//     into the e2e harness's throwaway scratch store via a raw io.Copy (no
//     parse, no re-marshal) — byte-preserving, and not a write path into
//     this repository's own store in any case (it targets a disposable
//     test fixture tree, never re-entering .verdi/diagrams here).
//
// No other package under internal/ or cmd/ constructs or writes mermaid
// diagram content anywhere in this repository as of this story (dex only
// reads and renders diagram artifacts into its own separate HTML output
// directory, internal/render/markdown.go's RenderMermaidBlock; it never
// writes back into .verdi/diagrams).
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// proposedStatusLineRe matches a diagram's `status: proposed` frontmatter
// line, tolerating an optional surrounding quote — mirroring accept.go's
// own draftStatusLineRe for the spec ritual.
var proposedStatusLineRe = regexp.MustCompile(`(?m)^status:\s*"?proposed"?\s*$`)

// runAcceptDiagram is diagram/<name>'s accept ritual: given an already-
// parsed diagram ref, reads and decodes the diagram file, refuses every
// illegal target (naming it and the reason), then performs the mechanical
// proposed -> accepted flip and frozen stamp, auto-committing the result
// exactly as the spec ritual does.
func runAcceptDiagram(ctx context.Context, root string, ref artifact.Ref, stdout, stderr io.Writer) int {
	diagPath := filepath.Join(root, ".verdi", "diagrams", ref.Name+".mermaid")
	raw, err := os.ReadFile(diagPath)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: does not resolve to a diagram (%v)\n", ref.String(), err)
		return 2
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: %v\n", diagPath, err)
		return 2
	}
	diag, err := artifact.DecodeDiagram(fm)
	if err != nil {
		fmt.Fprintf(stderr, "accept: %s: %v\n", diagPath, err)
		return 2
	}

	if diag.Class != artifact.DiagramClassProposal {
		fmt.Fprintf(stderr, "accept: %s: is not a class: proposal diagram (it is an incumbent, authored-living diagram — there is no acceptance for it)\n", ref.String())
		return 1
	}
	if diag.Status != "proposed" {
		fmt.Fprintf(stderr, "accept: %s: status is %q, not proposed; only a proposed proposal can be accepted\n", ref.String(), diag.Status)
		return 1
	}

	if !proposedStatusLineRe.Match(fm) {
		fmt.Fprintf(stderr, "accept: %s: internal error: decoded status is proposed, but no status: proposed frontmatter line was found to flip\n", diagPath)
		return 2
	}
	if n := len(proposedStatusLineRe.FindAllIndex(fm, -1)); n != 1 {
		fmt.Fprintf(stderr, "accept: %s: internal error: expected exactly one status: proposed line, found %d\n", diagPath, n)
		return 2
	}

	preFlipHead, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	at, err := gitx.CommitDateOnly(ctx, root, preFlipHead)
	if err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	frozenLine := fmt.Sprintf("frozen: { at: %s, commit: %s }", at, preFlipHead)
	newFm := proposedStatusLineRe.ReplaceAll(fm, []byte("status: accepted"))
	newFm = append(newFm, []byte("\n"+frozenLine)...)

	// Self-validate the flipped content before writing anything to disk
	// (CLAUDE.md: "never fake success") — the same discipline accept.go's
	// spec ritual applies to its own flip.
	flipped, err := artifact.DecodeDiagram(newFm)
	if err != nil {
		fmt.Fprintln(stderr, "accept: internal error: flipped diagram frontmatter failed self-validation:", err)
		return 2
	}
	if flipped.Status != "accepted" || flipped.Frozen == nil || flipped.Frozen.Commit != preFlipHead {
		fmt.Fprintln(stderr, "accept: internal error: flipped diagram frontmatter does not carry the expected status/frozen stamp")
		return 2
	}

	// The body is spliced back byte-for-byte (spec/proposal-artifact ac-2:
	// the splice ethos applied to diagrams) — body was captured verbatim by
	// SplitFrontmatter above and is never re-serialized or reformatted.
	newContent := "---\n" + string(newFm) + "\n---\n" + string(body)
	if err := os.WriteFile(diagPath, []byte(newContent), 0o644); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	// D6-33 (folded in alongside the spec ritual's identical fix, accept.go):
	// stage exactly diagPath, the one file this ritual modified — never
	// gitx.AddAll's `git add -A` sweep of the rest of the working tree.
	if err := gitx.AddPaths(ctx, root, diagPath); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}
	if _, err := gitx.CreateCommit(ctx, root, fmt.Sprintf("accept: %s proposed -> accepted", ref.String())); err != nil {
		fmt.Fprintln(stderr, "accept:", err)
		return 2
	}

	fmt.Fprintf(stdout, "accept: %s status: proposed -> accepted\n", ref.String())
	fmt.Fprintf(stdout, "accept: frozen: { at: %s, commit: %s }\n", at, preFlipHead)
	return 0
}
