package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/gitx"
)

// artifactResult is get_artifact's result shape: the ref resolved,
// frontmatter as raw YAML text (untouched — 05's data-never-instructions
// note applies to every byte of it) and the markdown body separately, so
// a caller never has to re-split them itself.
type artifactResult struct {
	Ref         string `json:"ref"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Frontmatter string `json:"frontmatter"`
	Body        string `json:"body"`
}

// GetArtifact implements the get_artifact tool: resolve ref[@commit] to
// content + frontmatter. An unpinned ref resolves the current working
// tree; a pinned ref resolves that historical commit via git (index.Entry
// alone drops the raw frontmatter block, so this reads the backing file —
// current working tree or, for a pin, `git show` — directly rather than
// going through index.Entry.Body a second time).
func (b *Backend) GetArtifact(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args struct {
		Ref string `json:"ref"`
	}
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("get_artifact: malformed arguments: " + err.Error())
	}
	if args.Ref == "" {
		return toolError("get_artifact: ref is required")
	}

	ref, err := artifact.ParseRef(args.Ref)
	if err != nil {
		return toolError("get_artifact: " + err.Error())
	}

	ix, err := b.buildIndex()
	if err != nil {
		return toolError("get_artifact: " + err.Error())
	}

	unpinned := artifact.Ref{Kind: ref.Kind, Name: ref.Name}
	entry, ok := ix.Get(unpinned.String())
	if !ok {
		return toolError(fmt.Sprintf("get_artifact: %q not found in the index", unpinned))
	}

	var raw []byte
	if ref.Pinned() {
		relPath, rerr := filepath.Rel(b.Root, entry.Path)
		if rerr != nil {
			return toolError("get_artifact: " + rerr.Error())
		}
		data, serr := gitx.Show(ctx, b.Root, ref.Commit, filepath.ToSlash(relPath))
		if serr != nil {
			return toolError(fmt.Sprintf("get_artifact: resolving %s: %v", args.Ref, serr))
		}
		raw = data
	} else {
		data, rerr := os.ReadFile(entry.Path)
		if rerr != nil {
			return toolError(fmt.Sprintf("get_artifact: reading %s: %v", entry.Path, rerr))
		}
		raw = data
	}

	result := artifactResult{Ref: entry.Ref, Kind: entry.Kind, Title: entry.Title}
	if entry.Kind == externalKind {
		// External refs (index.Entry doc: "index-minted refs carry no
		// frontmatter of their own") — the raw file is whatever the
		// discovered upstream artifact is (boundary-contract.json,
		// .flowmap.yaml, an OpenAPI doc), never verdi frontmatter+body.
		result.Body = string(raw)
	} else {
		fm, body, ferr := artifact.SplitFrontmatter(raw)
		if ferr != nil {
			return toolError(fmt.Sprintf("get_artifact: splitting frontmatter: %v", ferr))
		}
		result.Frontmatter = string(fm)
		result.Body = string(body)
	}
	return toolJSON(result)
}

// externalKind is index.Entry.Kind's literal value for index-minted
// external refs (02 §External refs) — index.Entry documents the string
// but does not export a constant for it.
const externalKind = "external"
