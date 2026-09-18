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
	Ref      string `json:"ref"`
	Kind     string `json:"kind"`
	Commit   string `json:"commit"`
	Proposed bool   `json:"proposed"`
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
	// The commit ARGUMENT names a commit, so it is held to the same rule
	// the pinned ref form holds Ref.Commit to (artifact.ValidCommit, the
	// exported form of ref.go's commitRe) — refused here as an argument
	// error rather than handed to `git rev-parse --verify` downstream
	// (final-review F2). Two things this closes: the documented contract
	// ("a full commit sha") was unenforced, so `commit: "HEAD"` or a
	// branch name silently resolved and behaved unlike the pinned form;
	// and an option-shaped value ("--git-dir") reached git as a flag
	// rather than as a revision. ParseRef already applies the identical
	// rule to the pin, so the two forms now agree by construction.
	if args.Commit != "" && !artifact.ValidCommit(args.Commit) {
		return toolError(fmt.Sprintf("get_document: commit %q must be 7-40 lowercase hex characters", args.Commit))
	}
	commit := args.Commit
	if ref.Pinned() {
		if commit != "" && commit != ref.Commit {
			return toolError(fmt.Sprintf("get_document: ref pin %s and commit %s disagree", ref.Commit, commit))
		}
		commit = ref.Commit
	}
	// proposed selects the loader's working-tree mode (R-W3-9), which reads
	// the serving checkout's live bytes — incompatible with a request for a
	// SPECIFIC commit's bytes, whether that commit arrived as the `commit`
	// argument or as a ref pin (both are folded into `commit` above by this
	// point), so this check covers both forms with the one guard. Silently
	// letting `proposed` win would drop the caller's pin without saying so;
	// silently letting the pin win would render historical bytes while
	// still reporting the working-tree Stamp.Proposed derivation the
	// tooldefs.go description promises — refused by name instead.
	if args.Proposed && commit != "" {
		return toolError("get_document: proposed and commit are mutually exclusive")
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
	if args.Proposed {
		mode = specdocload.ModeWorkingTree
	}
	// Readiness is a LIVE fact about the serving checkout — the snapshot
	// verdi serve built at startup (R-W3-3) — so it accompanies the
	// accepted and working-tree readings only. A pinned commit asks for
	// a historical document, and specdoc.WithReadiness gates on
	// TargetRef alone, not on mode: passing the snapshot here rendered
	// the pinned commit's bytes beside today's readiness section, two
	// different commits in one document (final-review F10). The pinned
	// reading now states the absence ("Readiness was not supplied for
	// this render.") rather than supplying a fact that is not about the
	// bytes being rendered.
	readiness := b.Readiness
	if mode == specdocload.ModeAt {
		readiness = nil
	}
	res, err := specdocload.Load(ctx, specdocload.Request{Root: b.Root, Name: ref.Name, Mode: mode, At: commit, Kind: kind, Model: mdl, Readiness: readiness})
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
