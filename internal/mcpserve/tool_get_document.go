package mcpserve

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specdoc"
	"github.com/jyang234/verdi/internal/specdocload"
	"github.com/jyang234/verdi/internal/store"
)

type getDocumentArgs struct {
	Ref    string `json:"ref"`
	Kind   string `json:"kind"`
	Commit string `json:"commit"`
}

type getDocumentResult struct {
	Ref         string   `json:"ref"`
	Kind        string   `json:"kind"`
	Commit      string   `json:"commit"`
	Engine      string   `json:"engine"`
	Proposed    bool     `json:"proposed"`
	Markdown    string   `json:"markdown"`
	Disclosures []string `json:"disclosures"`
}

// GetDocument renders a spec as its Markdown document (spec/spec-documents
// ac-5). Read-only: it adds nothing to the write surface. The default
// reading is the accepted bytes on the default branch; a pinned ref or a
// commit argument reads that commit. The result is canonical JSON, so two
// calls over unchanged state are byte-identical.
func (b *Backend) GetDocument(ctx context.Context, argsRaw json.RawMessage) map[string]any {
	var args getDocumentArgs
	if err := strictUnmarshal(argsRaw, &args); err != nil {
		return toolError("get_document: " + err.Error())
	}
	if args.Ref == "" {
		return toolError("get_document: ref is required")
	}
	ref, err := artifact.ParseRef(args.Ref)
	if err != nil || ref.Kind != artifact.KindSpec || ref.Fragment() {
		return toolError(fmt.Sprintf("get_document: %q is not a spec/<name> ref (optionally @commit)", args.Ref))
	}
	commit := args.Commit
	if ref.Pinned() {
		if commit != "" && commit != ref.Commit {
			return toolError(fmt.Sprintf("get_document: ref pin %s and commit %s disagree", ref.Commit, commit))
		}
		commit = ref.Commit
	}
	kindName := args.Kind
	if kindName == "" {
		kindName = string(specdoc.KindSpec)
	}
	kind, err := specdoc.ParseKind(kindName)
	if err != nil {
		return toolError("get_document: " + err.Error())
	}

	var mdl *model.Model
	if cfg, cerr := store.Open(b.Root); cerr == nil {
		mdl = cfg.Model
	}
	mode := specdocload.ModeAccepted
	if commit != "" {
		mode = specdocload.ModeAt
	}
	res, err := specdocload.Load(ctx, specdocload.Request{Root: b.Root, Name: ref.Name, Mode: mode, At: commit, Kind: kind, Model: mdl})
	if err != nil {
		return toolError("get_document: " + err.Error())
	}
	doc, err := specdoc.Build(res.Input)
	if err != nil {
		return toolError("get_document: " + err.Error())
	}
	disclosures := res.Disclosures
	if disclosures == nil {
		disclosures = []string{}
	}
	return toolJSON(getDocumentResult{
		Ref:         doc.Stamp.Ref,
		Kind:        string(doc.Kind),
		Commit:      doc.Stamp.Commit,
		Engine:      doc.Stamp.Engine,
		Proposed:    doc.Stamp.Proposed,
		Markdown:    specdoc.RenderMarkdown(doc),
		Disclosures: disclosures,
	})
}
