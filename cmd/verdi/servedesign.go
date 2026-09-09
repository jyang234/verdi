// The serve wiring's ASD design bridge (Wave 6 Task 2): the production
// value behind workbench.Deps.Design. It injects internal/designapp — the
// ONE application core CLI and MCP already route through — behind the
// workbench's consumer-owned port, encoding every clean read result with
// the SAME canonical encoder the CLI subcommands use (canonjson.Marshal,
// renderDesignAppResult's own path), so the browser action's bytes are the
// CLI's bytes (AC-2/CO-9 adapter conformance by construction). It
// contains no domain logic: call, encode, and a field-by-field view copy.
package main

import (
	"context"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/designapp"
	"github.com/jyang234/verdi/internal/draftmutation"
	"github.com/jyang234/verdi/internal/workbench"
)

// serveDesignBridge adapts designapp.Service to workbench.DesignBridge.
type serveDesignBridge struct {
	app designapp.Service
}

// newServeDesignBridge builds the production bridge over the production
// designapp service.
func newServeDesignBridge() workbench.DesignBridge {
	return &serveDesignBridge{app: designapp.NewService()}
}

// MutateDraft is a pass-through onto designapp.MutateDraft (itself the
// pass-through onto the one draftmutation kernel) — request, response,
// and diagnostic stay draftmutation's own closed union.
func (b *serveDesignBridge) MutateDraft(ctx context.Context, start string, request draftmutation.Request, actor draftmutation.Actor) (draftmutation.Response, *draftmutation.Error) {
	return b.app.MutateDraft(ctx, start, request, actor)
}

// encodeDesignOutcome projects one designapp read result into the
// workbench port's outcome union: canonical JSON on clean (the CLI's
// exact encoding), the typed Failure fields otherwise.
func encodeDesignOutcome(result any, appErr *designapp.Error) workbench.DesignReadOutcome {
	if appErr != nil {
		failure := appErr.Failure()
		return workbench.DesignReadOutcome{Failure: &workbench.DesignFailure{
			Classification: string(failure.Classification),
			Code:           failure.Code,
			Detail:         failure.Detail,
		}}
	}
	encoded, err := canonjson.Marshal(result)
	if err != nil {
		return workbench.DesignReadOutcome{Failure: &workbench.DesignFailure{
			Classification: string(designapp.ClassificationOperational),
			Code:           "result-invalid",
			Detail:         "encoding result: " + err.Error(),
		}}
	}
	return workbench.DesignReadOutcome{JSON: encoded}
}

func (b *serveDesignBridge) GetBoard(ctx context.Context, root, spec string) workbench.DesignReadOutcome {
	result, appErr := b.app.GetBoard(ctx, root, designapp.GetBoardRequest{Spec: spec})
	return encodeDesignOutcome(result, appErr)
}

func (b *serveDesignBridge) GetDesignContext(ctx context.Context, root, spec string, childStories []string) workbench.DesignReadOutcome {
	result, appErr := b.app.GetDesignContext(ctx, root, designapp.GetDesignContextRequest{Spec: spec, ChildStories: childStories})
	return encodeDesignOutcome(result, appErr)
}

func (b *serveDesignBridge) GetDesignCapabilities(ctx context.Context, root, spec string) (workbench.DesignReadOutcome, *workbench.DesignCapabilitiesView) {
	result, appErr := b.app.GetDesignCapabilities(ctx, root, designapp.GetDesignCapabilitiesRequest{Spec: spec})
	outcome := encodeDesignOutcome(result, appErr)
	if appErr != nil || result == nil {
		return outcome, nil
	}
	view := &workbench.DesignCapabilitiesView{
		Mutable:       result.Mutable,
		PolicyMode:    result.PolicyMode,
		PolicyDigest:  result.PolicyDigest,
		SpecState:     string(result.SpecState),
		CurrentDigest: result.CurrentDigest,
	}
	if result.MutabilityRefusal != nil {
		view.RefusalPrecondition = result.MutabilityRefusal.Precondition
		view.RefusalDetail = result.MutabilityRefusal.Detail
	}
	for _, op := range result.PermittedOperations {
		view.PermittedOperations = append(view.PermittedOperations, string(op))
	}
	return outcome, view
}

func (b *serveDesignBridge) GetDesignProvenance(ctx context.Context, root, spec string) workbench.DesignReadOutcome {
	result, appErr := b.app.GetDesignProvenance(ctx, root, designapp.GetDesignProvenanceRequest{Spec: spec})
	return encodeDesignOutcome(result, appErr)
}

func (b *serveDesignBridge) PrepareDesignReview(ctx context.Context, root, spec string) workbench.DesignReadOutcome {
	result, appErr := b.app.PrepareDesignReview(ctx, root, designapp.PrepareDesignReviewRequest{Spec: spec})
	return encodeDesignOutcome(result, appErr)
}
