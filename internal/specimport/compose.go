package specimport

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/lint"
)

// Compose-time Finding codes (spec-import-contract.md, "Errors and browser
// behavior"): the closed codes errors.go's own doc comment names as
// "produced by later tasks' candidate/preview/apply logic, never by this
// package [Normalize]" — Compose is that later logic for three of them.
// current-spec-changed and import-record-missing belong to a still-later
// task (ReadRecord) and are never produced here.
const (
	// FindingInvalidCandidate reports the composed candidate itself is
	// structurally or semantically invalid — a splice.Validate failure, or
	// a lint.CheckCandidate finding targeting the candidate's own path.
	FindingInvalidCandidate = "invalid-candidate"
	// FindingExistingCorpus reports a lint.CheckCandidate finding about a
	// dependency the candidate references (a parent feature, an
	// implements/resolves target) rather than the candidate itself —
	// distinguished from FindingInvalidCandidate so a caller can tell "your
	// own content is broken" from "something you depend on is broken."
	FindingExistingCorpus = "existing-corpus-finding"
	// FindingStatementsDeferred is the nonblocking disclosure
	// prepareCandidateFields emits once per explicitly deferred statement
	// (spec-import-contract.md: "Successful apply with deferral always ...
	// emits a nonblocking statements-deferred disclosure").
	FindingStatementsDeferred = "statements-deferred"
)

// OriginGeneratedDeferral marks a Field prepareCandidateFields synthesized
// for an explicitly deferred statement (schema.go's OriginNative doc
// comment: "generated-deferral is a Task 2 Compose-time origin, never
// produced by Normalize").
const OriginGeneratedDeferral = "generated-deferral"

// placeholderStubSlug and placeholderACID are the exact literals
// internal/designscaffold's embedded canonical templates (feature.md,
// story.md) render for their one generated placeholder stub/acceptance
// criterion — the content Compose must never retain as a real imported
// requirement (spec-import-contract.md: "Remove the generated placeholder
// criterion ... never retain them as real imported requirements").
const (
	placeholderStubSlug = "todo-replace-stub-slug"
	placeholderACID     = "ac-1"
)

// Compose renders and validates the candidate spec bytes for request+plan
// (spec-import-contract.md, "Shared internal interfaces": "Compose ...
// owns the existing renderer/splice integration and shared candidate lint
// checks. It does not write"). It never touches Git, the filesystem
// (beyond read-only store/template resolution under root), or the
// network, and never mutates request or plan.
//
// Compose dispatches on request.Format: native mode (composeNative, this
// package's compose_native.go) treats plan.Native as the candidate
// verbatim; every other format (composeExternal, compose_external.go)
// renders and edits a fresh scaffold. Both paths share this file's
// Finding-code constants and the translateLintFindings/hasBlocking
// helpers below, and compose_deferral.go's prepareCandidateFields.
//
// Compose returns nil bytes whenever the returned Findings carry at least
// one Blocking entry — an incomplete or invalid candidate is never
// returned as best-effort bytes a caller could mistake for one ready to
// commit (pinned by TestCompose_BlockingFindings_ReturnsNilBytes). A
// non-nil error is reserved for a genuinely operational failure (an
// unreadable store, a request that fails its own Validate) — every
// content-level defect in the candidate itself is a Finding, mirroring
// lint.Engine.Run's own error-vs-Finding split.
func Compose(ctx context.Context, root string, request Request, plan Plan) ([]byte, []Finding, error) {
	if err := request.Validate(); err != nil {
		return nil, nil, err
	}
	if request.Format == FormatNative {
		return composeNative(ctx, root, request, plan)
	}
	return composeExternal(ctx, root, request, plan)
}

// hasBlocking reports whether any finding is Blocking.
func hasBlocking(findings []Finding) bool {
	for _, f := range findings {
		if f.Blocking {
			return true
		}
	}
	return false
}

// translateLintFindings maps lint.Finding (Rule/Path/Message/Severity/
// Locus) onto specimport.Finding (Code/Target/Message/Blocking) — the one
// place this package interprets an existing VL-xxx result rather than
// reimplementing it. FindingInvalidCandidate names a finding about the
// candidate's own path; FindingExistingCorpus names one about a
// dependency the candidate itself does not live at (spec-import-
// contract.md: "surface corrupt/unresolvable dependencies explicitly").
// The underlying VL-xxx rule id is kept in Message, never discarded
// (spec-import-contract.md: "Underlying VL identifiers remain in the
// message/target").
func translateLintFindings(findings []lint.Finding, candidateRelPath string) []Finding {
	out := make([]Finding, 0, len(findings))
	for _, f := range findings {
		code := FindingExistingCorpus
		if f.Path == candidateRelPath {
			code = FindingInvalidCandidate
		}
		target := f.Path
		if f.Locus != nil && f.Locus.Object != "" {
			target = f.Locus.Object
		}
		out = append(out, Finding{
			Code:     code,
			Target:   target,
			Message:  fmt.Sprintf("%s: %s", f.Rule, f.Message),
			Blocking: f.Severity == lint.SeverityViolation,
		})
	}
	return out
}
