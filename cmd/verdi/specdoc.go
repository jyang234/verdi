// verdi spec doc SPEC_REF (Task 7, spec/spec-documents): the thin CLI
// consumer of internal/specdoc — it loads one spec's bytes (the default
// branch's, a pinned commit's via --at, or the working tree's via
// --proposed), resolves its effective status through the same
// specstate.Projector every other status decision in this package routes
// through, gathers the coverage/claims facts the spec's own stubs supply
// and (best effort) the matrix's evidence facts, and writes the render.
// It computes nothing specdoc itself does not already compute; this file
// only wires bytes in and text out.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/matrixprojection"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specstate"
	"github.com/jyang234/verdi/internal/store"
)

// vocab:identity — CLI usage grammar (identity arg placeholders)
const specDocForm = "verdi spec doc <spec-ref> [--kind spec|plan|tasks] [--format md|html] [--at <commit>] [--proposed] [-o <path>]"

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
	cfg, err := store.Open(root)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	ctx := context.Background()

	src, err := loadSpecDocSource(ctx, root, parsed.Name, *atFlag, *proposedFlag)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}
	fm, err := artifact.DecodeSpec(src.content)
	if err != nil {
		fmt.Fprintf(stderr, "spec doc: %s at %s: %v\n", parsed.String(), src.commit, err)
		return 2
	}
	_, body, err := artifact.SplitFrontmatter(src.content)
	if err != nil {
		fmt.Fprintln(stderr, "spec doc:", err)
		return 2
	}

	status := ""
	if res, rerr := specstate.NewProjector().Resolve(ctx, root, specstate.Candidate{Path: src.relPath, Content: src.content}); rerr == nil {
		status = string(res.ArtifactStatus())
	} else {
		fmt.Fprintf(stderr, "spec doc: status not resolved: %v\n", rerr)
	}

	facts := specdoc.FactsFromSpec(fm)
	// The matrix always evaluates HEAD, resolved once here — never
	// src.commit, which under --at names a possibly different (older or
	// newer) commit than HEAD (spec-documents wave-1 fix round, F1). When
	// HEAD cannot be resolved, evidence stays unavailable rather than
	// mislabeling its source.
	if head, herr := gitx.RevParse(ctx, root, "HEAD"); herr != nil {
		fmt.Fprintf(stderr, "spec doc: evidence not computed: resolving HEAD: %v\n", herr)
	} else if proj, perr := matrixprojection.Project(ctx, root, parsed.String(), *proposedFlag, cfg.Model); perr == nil {
		facts = specdoc.WithMatrix(facts, proj.Record, head)
	} else {
		fmt.Fprintf(stderr, "spec doc: evidence not computed: %v\n", perr)
	}

	doc, err := specdoc.Build(specdoc.Input{
		Spec:   fm,
		Body:   body,
		Status: status,
		Stamp:  specdoc.Stamp{Ref: parsed.String(), Commit: src.commit, Proposed: *proposedFlag},
		Facts:  facts,
		Model:  cfg.Model,
		Kind:   kind,
	})
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

// specDocSource is one spec's bytes, where they came from, and the full
// commit the stamp names.
type specDocSource struct {
	relPath string
	content []byte
	commit  string
}

// loadSpecDocSource picks the bytes: the default branch's (the accepted
// reading), a pinned commit's (--at), or the working tree's (--proposed,
// stamped with HEAD). Active zone first, archive second, in every mode.
func loadSpecDocSource(ctx context.Context, root, name, at string, proposed bool) (specDocSource, error) {
	if proposed {
		relPath, content, err := readSpecBytesEitherZone(root, name)
		if err != nil {
			return specDocSource{}, err
		}
		head, err := gitx.RevParse(ctx, root, "HEAD")
		if err != nil {
			return specDocSource{}, fmt.Errorf("resolving HEAD: %w", err)
		}
		return specDocSource{relPath: relPath, content: content, commit: head}, nil
	}
	rev := at
	if rev == "" {
		branch, ok := specstate.ResolveDefaultBranch(ctx, root)
		if !ok {
			return specDocSource{}, fmt.Errorf("the default branch could not be resolved; pass --at <commit> or --proposed")
		}
		rev = branch.Ref
	}
	commit, err := gitx.RevParse(ctx, root, rev)
	if err != nil {
		return specDocSource{}, fmt.Errorf("resolving %q: %w", rev, err)
	}
	for _, relPath := range []string{store.ActiveSpecRelPath(name), store.SpecRelPath(store.ZoneArchive, name)} {
		content, err := gitx.Show(ctx, root, commit, relPath)
		if err == nil {
			return specDocSource{relPath: relPath, content: content, commit: commit}, nil
		}
	}
	return specDocSource{}, fmt.Errorf("spec/%s not found at %s in either zone", name, commit)
}
