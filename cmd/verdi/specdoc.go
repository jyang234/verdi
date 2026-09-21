// verdi spec doc SPEC_REF (Task 7, spec/spec-documents): the thin CLI
// consumer of internal/specdocload and internal/specdoc — it delegates
// picking the spec's bytes (the default branch's, a pinned commit's via
// --at, or the working tree's via --proposed), resolving its effective
// status, and gathering the coverage/claims/evidence facts to
// specdocload.Load (spec-documents wave 2 task 2: the same assembler the
// board's Document tab, the docs site, and the MCP get_document tool
// share, so the four outputs stay byte-identical), then writes the
// render. This file only wires the flags in and the rendered text out.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/readinesspilot"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
)

// vocab:identity — CLI usage grammar (identity arg placeholders)
const specDocForm = "verdi spec doc <spec-ref> [--kind spec|plan|tasks] [--format md|html] [--at <commit>] [--proposed] [--no-readiness] [-o <path>]"

// vocab:identity — CLI usage grammar (identity arg placeholders)
const specDocUsage = "usage: " + specDocForm

// cmdSpecDoc renders one spec as a document (spec/spec-documents ac-2).
// Exit 0 on a render, 2 on an unresolvable ref, commit, kind, format, or
// store. Never 1: a document is a projection, not a verdict.
func cmdSpecDoc(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("spec doc", flag.ContinueOnError)
	fs.SetOutput(stderr)
	kindFlag := fs.String("kind", "spec", "document kind: spec, plan, or tasks")
	formatFlag := fs.String("format", "md", "output format: md or html")
	atFlag := fs.String("at", "", "render the spec bytes at this commit")
	// vocab:identity — non-vocabulary homograph: "draft" names an unmerged branch's edit in English prose, never the model's "draft" lifecycle-state id
	proposedFlag := fs.Bool("proposed", false, "render the working tree's bytes (a design-branch draft)")
	outFlag := fs.String("o", "", "write the document to this path instead of stdout")
	noReadinessFlag := fs.Bool("no-readiness", false, "omit the Readiness section (spec/readiness-recovery ac-4/R-RR1-9)")

	// The single positional <spec-ref> may appear anywhere among the
	// flags: flag.FlagSet.Parse stops at the first non-flag token, so a
	// flag placed AFTER the ref would otherwise never reach fs.Parse at
	// all and would silently keep its zero value (fix round 1, F1).
	// Repeatedly parse the remainder instead: each round consumes every
	// flag up to the next non-flag token, then peels off exactly one
	// positional (the ref, the first time) before parsing again: a
	// second positional is a usage error, never a second, silently
	// accepted ref.
	ref := ""
	remaining := args
	for {
		if err := fs.Parse(remaining); err != nil {
			return 2
		}
		if fs.NArg() == 0 {
			break
		}
		if ref != "" {
			fmt.Fprintln(stderr, specDocUsage)
			return 2
		}
		ref = fs.Arg(0)
		remaining = fs.Args()[1:]
	}
	if ref == "" {
		fmt.Fprintln(stderr, specDocUsage)
		return 2
	}
	parsed, err := artifact.ParseRef(ref)
	if err != nil || parsed.Kind != artifact.KindSpec || parsed.Fragment() || parsed.Pinned() {
		fmt.Fprintf(stderr, "spec doc: %q is not a valid spec/<name> ref (use --at for a commit)\n", ref)
		return 2
	}
	kind, err := specdoc.ParseKind(*kindFlag)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	if *formatFlag != "md" && *formatFlag != "html" {
		fmt.Fprintf(stderr, "spec doc: --format must be md or html, got %q\n", *formatFlag)
		return 2
	}
	if *atFlag != "" && *proposedFlag {
		fmt.Fprintln(stderr, "spec doc: --at and --proposed cannot be combined")
		return 2
	}

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	if *outFlag != "" {
		inside, ierr := outPathInStore(root, *outFlag)
		if ierr != nil {
			fmt.Fprintln(stderr, "spec doc:", ierr)
			return 2
		}
		if inside {
			fmt.Fprintln(stderr, "spec doc: -o must not point inside the store (.verdi/): a rendered document is never a store artifact")
			return 2
		}
	}
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	ctx := context.Background()

	mode := specdocload.ModeAccepted
	switch {
	case *proposedFlag:
		mode = specdocload.ModeWorkingTree
	case *atFlag != "":
		mode = specdocload.ModeAt
	}

	// Readiness accompanies the accepted and working-tree readings by
	// default (spec/readiness-recovery ac-4/R-RR1-9): --no-readiness
	// omits it outright, and --at (a historical reading) never calls the
	// loader — a live derivation is not a fact about historical bytes,
	// the same rule the board Document tab and MCP get_document follow.
	// A loader failure never fails the render: it is a disclosure on
	// stderr ("spec doc: readiness: <err>") and the section states its
	// own absence — a document is not a verdict (R-RR1-9).
	var readiness *readinesspilot.Snapshot
	if !*noReadinessFlag && mode != specdocload.ModeAt {
		loader := readinessload.Loader{Root: root, Opts: readinessload.Options{BoardHref: workbench.BranchBoardHref}}
		snap, rerr := loader.Load(ctx, "spec/"+parsed.Name)
		if rerr != nil {
			fmt.Fprintln(stderr, "spec doc: readiness:", rerr)
		} else {
			readiness = &snap
		}
	}

	res, err := specdocload.Load(ctx, specdocload.Request{Root: root, Name: parsed.Name, Mode: mode, At: *atFlag, Kind: kind, Model: cfg.Model, Readiness: readiness})
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	for _, d := range res.Disclosures {
		fmt.Fprintln(stderr, "spec doc:", d)
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}

	var rendered string
	if *formatFlag == "html" {
		rendered, err = specdoc.RenderHTML(doc)
		if err != nil {
			fmt.Fprintln(stderr, "spec doc:", err)
			return 2
		}
	} else {
		rendered = specdoc.RenderMarkdown(doc)
	}
	if *outFlag != "" {
		if err := os.WriteFile(*outFlag, []byte(rendered), 0o644); err != nil {
			fmt.Fprintln(stderr, "spec doc:", err)
			return 2
		}
		return 0
	}
	if _, err := io.WriteString(stdout, rendered); err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	return 0
}

// outPathInStore reports whether out's real, absolute path lies inside
// root's .verdi/ store directory (spec-documents wave-1 fix round, F3): a
// rendered document is a projection, never a store artifact, so -o must
// never write into the store it was rendered from. filepath.Rel between
// the two resolved absolute paths names the containment: a result of "."
// or one that never starts with ".." is inside.
//
// Both sides are resolved through resolveExistingPrefix, not bare
// filepath.Abs, before the Rel comparison: root is frequently reached
// through a symlinked ancestor (a macOS temp/build directory — e.g.
// testing.T.TempDir()'s own /var/folders/... — is one; so is any
// developer checkout under a symlinked home or mount), and store.FindRoot
// resolves it (via os.Getwd()) to that symlink's real target. Comparing
// the real store path against an -o argument built from the same
// unresolved, symlinked logical path would wrongly compute "outside" and
// let the write through the very check meant to refuse it.
func outPathInStore(root, out string) (bool, error) {
	absStore, err := filepath.Abs(filepath.Join(root, ".verdi"))
	if err != nil {
		return false, fmt.Errorf("resolving the store path: %w", err)
	}
	realStore, err := resolveExistingPrefix(absStore)
	if err != nil {
		return false, fmt.Errorf("resolving the store path: %w", err)
	}
	absOut, err := filepath.Abs(out)
	if err != nil {
		return false, fmt.Errorf("resolving -o path: %w", err)
	}
	realOut, err := resolveExistingPrefix(absOut)
	if err != nil {
		return false, fmt.Errorf("resolving -o path: %w", err)
	}
	rel, err := filepath.Rel(realStore, realOut)
	if err != nil {
		return false, fmt.Errorf("comparing -o path to the store: %w", err)
	}
	if rel == "." {
		return true, nil
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)), nil
}

// resolveExistingPrefix returns path (already absolute and clean) with
// its longest existing ancestor — path itself, if it exists — resolved
// through filepath.EvalSymlinks, and any non-existent tail reattached
// unresolved: a path component that does not exist yet cannot itself be
// a symlink, and -o's own target file is typically not expected to exist
// before the render writes it.
func resolveExistingPrefix(path string) (string, error) {
	for p := path; ; {
		resolved, err := filepath.EvalSymlinks(p)
		if err == nil {
			if p == path {
				return resolved, nil
			}
			suffix, rerr := filepath.Rel(p, path)
			if rerr != nil {
				return "", rerr
			}
			return filepath.Join(resolved, suffix), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(p)
		if parent == p {
			return "", err
		}
		p = parent
	}
}
