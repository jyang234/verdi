package specimport

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
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
// internal/designscaffold's embedded canonical templates.md (feature.md,
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

// composeNative handles native format: plan.Native, produced by
// Normalize's own normalizeNative, is the candidate (spec-import-
// contract.md, "Candidate and validation": "plan.Native is the candidate.
// Valid native content and stable IDs remain byte-identical"). Re-running
// normalizeNative here — the identical, already-tested Task 1 structural
// gate, never a second copy of its checks — is Compose's own defense
// against a caller passing a Plan that does not actually correspond to
// request (Normalize's own "no service trusts prior decoding" posture,
// applied one layer up); the shared lint.CheckCandidate seam then applies
// every corpus-aware check (strict decode is already proven by
// normalizeNative; anchors, requiredness, ref resolution, duplicate
// identity, tracker-scheme configuredness) identically to the external
// path — "no grandfathering" for an old native input.
func composeNative(ctx context.Context, root string, request Request, plan Plan) ([]byte, []Finding, error) {
	if len(plan.Native) == 0 {
		return nil, nil, fmt.Errorf("specimport: compose: native format requires a non-empty plan.Native (request/plan mismatch)")
	}
	if _, err := normalizeNative(request, plan.Native); err != nil {
		return nil, []Finding{{Code: FindingInvalidCandidate, Target: "native", Message: err.Error(), Blocking: true}}, nil
	}

	relPath := store.ActiveSpecRelPath(request.Target.Slug)
	lintFindings, err := lint.CheckCandidate(ctx, root, relPath, plan.Native)
	if err != nil {
		return nil, nil, fmt.Errorf("specimport: compose: checking native candidate: %w", err)
	}
	findings := translateLintFindings(lintFindings, relPath)
	if hasBlocking(findings) {
		return nil, findings, nil
	}
	return plan.Native, findings, nil
}

// composeExternal handles markdown-v1/f13-reference-v1/manual-v1: resolve
// the target class's template through the existing store/model/scaffold
// seams, render it, mirror every candidate-ready Field into both
// frontmatter and body via typed splice operations, remove the scaffold's
// generated placeholder criterion/stub (frontmatter entry, body section,
// and the stub referencing it), then run the shared structural
// (splice.Validate) and corpus-aware (lint.CheckCandidate) gates.
func composeExternal(ctx context.Context, root string, request Request, plan Plan) ([]byte, []Finding, error) {
	cfg, err := store.Open(root)
	if err != nil {
		return nil, nil, fmt.Errorf("specimport: compose: opening store: %w", err)
	}

	class, ok := cfg.Model.Classes[request.Target.Class]
	if !ok || class.Template == "" {
		return nil, []Finding{{
			Code:     FindingUnsupportedStructure,
			Target:   "target.class",
			Message:  fmt.Sprintf("the store's operating model declares no usable template for class %q; the model/template combination cannot express this candidate", request.Target.Class),
			Blocking: true,
		}}, nil
	}
	tmpl, err := designscaffold.LoadTemplate(root, class.Template)
	if err != nil {
		return nil, nil, fmt.Errorf("specimport: compose: loading template %q: %w", class.Template, err)
	}

	fields, findings := prepareCandidateFields(request, plan)

	rendered, blocking := renderScaffold(tmpl, request)
	if blocking != nil {
		return nil, append(findings, *blocking), nil
	}

	if decoded, err := artifact.DecodeSpec(mustFrontmatterBytes(rendered)); err != nil {
		return nil, append(findings, Finding{Code: FindingUnsupportedStructure, Message: fmt.Sprintf("rendered scaffold does not decode: %v", err), Blocking: true}), nil
	} else if err := designscaffold.CheckClass(decoded, artifact.SpecClass(request.Target.Class)); err != nil {
		return nil, append(findings, Finding{Code: FindingUnsupportedStructure, Target: "target.class", Message: fmt.Sprintf("the model's template binding is incompatible with the requested class: %v", err), Blocking: true}), nil
	}

	result, blocking, err := applyCandidateEdits(rendered, request, fields)
	if err != nil {
		return nil, nil, err
	}
	if blocking != nil {
		return nil, append(findings, *blocking), nil
	}

	if err := splice.Validate(result); err != nil {
		return nil, append(findings, Finding{Code: FindingInvalidCandidate, Message: err.Error(), Blocking: true}), nil
	}

	relPath := store.ActiveSpecRelPath(request.Target.Slug)
	lintFindings, err := lint.CheckCandidate(ctx, root, relPath, result)
	if err != nil {
		return nil, nil, fmt.Errorf("specimport: compose: checking candidate: %w", err)
	}
	findings = append(findings, translateLintFindings(lintFindings, relPath)...)

	if hasBlocking(findings) {
		return nil, findings, nil
	}
	return result, findings, nil
}

// renderScaffold instantiates the target class's template against
// request's identity fields, always with the shared
// designscaffold.DefaultProblem/DefaultOutcome placeholders regardless of
// what was actually mapped (applyCandidateEdits mirrors the real — or
// explicitly deferred — text in afterward): a fresh render is byte-for-
// byte what every other scaffold caller already produces, so its own
// class-specific requiredness (a story's >=1 implements/resolves edge)
// is satisfied identically. Story's mandatory edge(s) are rendered
// directly from request.Links here — never synthesized (spec-import-
// contract.md: "no TODO tracker is synthesized") — because
// splice.ApplyDraftMutations validates the STARTING buffer before any
// operation runs, so a story rendered with no edge at all would already
// refuse before Compose's own edit pipeline had a chance to add one.
func renderScaffold(tmpl []byte, request Request) (string, *Finding) {
	specRef := "spec/" + request.Target.Slug
	var rendered string
	var err error
	switch artifact.SpecClass(request.Target.Class) {
	case artifact.ClassFeature:
		rendered, err = designscaffold.Feature(tmpl, specRef, request.Target.Story, request.Target.Title, designscaffold.DefaultProblem, designscaffold.DefaultOutcome)
	case artifact.ClassStory:
		links := make([]designscaffold.StoryLink, 0, len(request.Links))
		for _, l := range request.Links {
			links = append(links, designscaffold.StoryLink{Type: artifact.LinkType(l.Type), Ref: l.Ref})
		}
		rendered, err = designscaffold.Story(tmpl, specRef, request.Target.Story, request.Target.Title, false, links, designscaffold.DefaultProblem, designscaffold.DefaultOutcome)
	default:
		// Unreachable: Request.Validate already restricts target.class to
		// feature or story (validateTarget). Fail closed rather than panic
		// if that invariant is ever broken upstream.
		return "", &Finding{Code: FindingUnsupportedStructure, Target: "target.class", Message: fmt.Sprintf("internal: unreachable target class %q", request.Target.Class), Blocking: true}
	}
	if err != nil {
		return "", &Finding{Code: FindingUnsupportedStructure, Target: "template", Message: fmt.Sprintf("the resolved template cannot render this candidate: %v", err), Blocking: true}
	}
	return rendered, nil
}

// mustFrontmatterBytes splits rendered's frontmatter, panicking only on a
// shape every real class template (canonical or a validated override)
// always produces — Render already succeeded, so a missing "---"
// delimiter here would be a packaging defect in the template itself, not
// a candidate-content condition; callers still route the FIRST split
// (this one) through DecodeSpec's own error before trusting the result,
// so a template that somehow still fails is reported as an unsupported
// candidate, never a panic that reaches a caller.
func mustFrontmatterBytes(rendered string) []byte {
	fm, _, err := artifact.SplitFrontmatter([]byte(rendered))
	if err != nil {
		panic(fmt.Sprintf("specimport: compose: rendered scaffold has no frontmatter delimiters: %v", err))
	}
	return fm
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

// prepareCandidateFields returns the candidate-ready Fields and Findings
// for request+plan: pure, deterministic, and read-only over both
// arguments (spec-import-contract.md; plan.md Task 2: "Keep any Plan
// transformation needed for deferral/candidate preparation in a shared
// pure unexported helper ... so Task 3's Preview can consume it and
// expose the same generated-deferral origins"). Task 3's Preview calls
// this identically, so the two surfaces can never diverge on which fields
// exist or which generated-deferral origins they carry.
//
// When request.DeferStatements is true, EACH of "problem" and "outcome"
// is unconditionally replaced by a generated-deferral placeholder — the
// shared designscaffold.DefaultProblem/DefaultOutcome constant, Origin
// OriginGeneratedDeferral — regardless of whether Normalize already
// resolved real text for it: explicit PAIR deferral is a deliberate,
// all-or-nothing request-level choice (spec-import-contract.md: "Explicit
// deferral inserts both shared TODO constants"), not merely a fallback
// for what automatic recognition failed to find. A statement Normalize
// DID resolve is never silently discarded: it is named, text and all, in
// a nonblocking statements-deferred disclosure Finding, and its own
// now-superseded missing-statement BLOCKING finding (present only when
// nothing was resolved at all) is dropped — superseded by the deferral
// that just resolved it, not left blocking a candidate the request
// explicitly asked to defer.
func prepareCandidateFields(request Request, plan Plan) ([]Field, []Finding) {
	fields := make([]Field, 0, len(plan.Fields))

	if !request.DeferStatements {
		fields = append(fields, plan.Fields...)
		findings := make([]Finding, len(plan.Findings))
		copy(findings, plan.Findings)
		return fields, findings
	}

	displaced := map[string]Field{}
	for _, f := range plan.Fields {
		if f.Target == "problem" || f.Target == "outcome" {
			displaced[f.Target] = f
			continue
		}
		fields = append(fields, f)
	}
	for _, target := range []string{"problem", "outcome"} {
		fields = append(fields, deferredField(target))
	}

	findings := make([]Finding, 0, len(plan.Findings)+2)
	for _, f := range plan.Findings {
		if f.Code == FindingMissingStatement && (f.Target == "problem" || f.Target == "outcome") {
			continue // superseded: the loop above just resolved this target
		}
		findings = append(findings, f)
	}
	for _, target := range []string{"problem", "outcome"} {
		msg := fmt.Sprintf("%s statement deferred; a generated placeholder was inserted instead", target)
		if orig, ok := displaced[target]; ok {
			msg = fmt.Sprintf("%s statement deferred; the recognized source text %q was retained rather than used", target, orig.Text)
		}
		findings = append(findings, Finding{Code: FindingStatementsDeferred, Target: target, Message: msg, Blocking: false})
	}
	return fields, findings
}

// deferredField builds one generated-deferral placeholder Field for
// target ("problem" or "outcome"): the shared designscaffold constant,
// carrying no Spans/Evidence of its own (it is not source-derived).
func deferredField(target string) Field {
	text := designscaffold.DefaultProblem
	if target == "outcome" {
		text = designscaffold.DefaultOutcome
	}
	return Field{Target: target, Text: text, Origin: OriginGeneratedDeferral}
}

// applyCandidateEdits mirrors every candidate-ready field into rendered's
// frontmatter and body, and removes the scaffold's generated placeholder
// stub/criterion. It runs several rounds over splice's typed operations —
// never a hand-rolled parser of its own — each its own Parse/edits/Apply
// round over the PREVIOUS round's result: a later round's edits must
// never be computed from a parse a still-pending earlier edit would
// invalidate. Concretely, two hazards this ordering avoids (both found by
// this function's own RED tests, not merely anticipated):
//
//   - a real object field appended at "ac-1" must never be computed from
//     the same static parse that is also removing the placeholder "ac-1"
//     entry it would otherwise collide with (its insertion point would be
//     computed relative to an entry about to disappear); and
//   - two object fields appended in the same batch, when their shared
//     block does not exist yet, would BOTH compute "insert a brand new
//     block here" from the same unmodified parse and collide (a
//     duplicate frontmatter key) — so every object field gets its own
//     Parse/Apply round.
//
// Rounds:
//
//  1. splice.ApplyDraftMutations (the existing typed draft-mutation
//     batch), against the PRISTINE render, which — placeholder stub/
//     criterion included — already satisfies splice.Validate exactly as
//     every other scaffold caller's fresh render does: remove the
//     placeholder stub, set the resolved Problem/Outcome ATTRIBUTE
//     (frontmatter only — ops.go has no attribute-level primitive of its
//     own), and, for a feature, add any explicit request.Links (a
//     story's own links were already rendered directly into the scaffold
//     — see renderScaffold — since ApplyDraftMutations validates the
//     STARTING buffer, before this round could ever add a story's
//     mandatory edge itself).
//  2. ops.go's RemoveObjectEntry plus this package's new RemoveSection,
//     in one batch from one Parse: remove the placeholder criterion's
//     frontmatter entry AND its now-orphaned body section together — the
//     placeholder body heading is unambiguous here because nothing has
//     been appended yet that could also slugify to "ac-1".
//  3. This package's new SetSectionText, in one batch from one Parse:
//     mirror the resolved Problem/Outcome text into their (already
//     existing, disjoint) body sections.
//  4. ops.go's AppendObject, ONE Parse/Apply round PER candidate-ready
//     object field, over the previous field's own result: append it
//     (AppendObject creates both frontmatter entry and body section at
//     once, so nothing it adds can ever be orphaned).
//
// A splice error at any round is translated into a blocking Finding —
// almost always the template/candidate combination could not be composed
// (an override template missing the expected placeholder shape) — never a
// raw Go error, and never silently dropped (spec-import-contract.md:
// "Never silently drop an operation to make validation pass"). The
// returned *Finding is nil exactly when result is usable.
func applyCandidateEdits(rendered string, request Request, fields []Field) ([]byte, *Finding, error) {
	problemField, outcomeField, objectFields := splitFields(fields)

	pass1 := []designprovenance.Operation{{Op: designprovenance.OpRemoveStub, Slug: placeholderStubSlug}}
	if problemField != nil {
		pass1 = append(pass1, designprovenance.Operation{Op: designprovenance.OpSetProblem, Text: problemField.Text, Anchor: "problem"})
	}
	if outcomeField != nil {
		pass1 = append(pass1, designprovenance.Operation{Op: designprovenance.OpSetOutcome, Text: outcomeField.Text, Anchor: "outcome"})
	}
	if request.Target.Class == string(artifact.ClassFeature) {
		for _, l := range request.Links {
			pass1 = append(pass1, designprovenance.Operation{Op: designprovenance.OpAddLink, Source: "spec", Type: artifact.LinkType(l.Type), Ref: l.Ref})
		}
	}

	pass1Result, err := splice.ApplyDraftMutations([]byte(rendered), pass1)
	if err != nil {
		return nil, &Finding{Code: FindingInvalidCandidate, Message: fmt.Sprintf("preparing candidate: %v", err), Blocking: true}, nil
	}

	pass2Doc, err := splice.Parse(pass1Result)
	if err != nil {
		return nil, nil, fmt.Errorf("specimport: compose: re-parsing after pass 1: %w", err)
	}
	removeFM, err := pass2Doc.RemoveObjectEntry(placeholderACID)
	if err != nil {
		return nil, &Finding{Code: FindingUnsupportedStructure, Message: err.Error(), Blocking: true}, nil
	}
	removeBody, err := pass2Doc.RemoveSection(placeholderACID)
	if err != nil {
		return nil, &Finding{Code: FindingUnsupportedStructure, Message: err.Error(), Blocking: true}, nil
	}
	pass2Result, err := pass2Doc.Apply([]splice.Edit{removeFM, removeBody})
	if err != nil {
		return nil, &Finding{Code: FindingInvalidCandidate, Message: fmt.Sprintf("removing the placeholder acceptance criterion: %v", err), Blocking: true}, nil
	}

	if problemField != nil || outcomeField != nil {
		pass3Doc, err := splice.Parse(pass2Result)
		if err != nil {
			return nil, nil, fmt.Errorf("specimport: compose: re-parsing after pass 2: %w", err)
		}
		var edits []splice.Edit
		if problemField != nil {
			e, err := pass3Doc.SetSectionText("problem", problemField.Text)
			if err != nil {
				return nil, &Finding{Code: FindingUnsupportedStructure, Target: "problem", Message: err.Error(), Blocking: true}, nil
			}
			edits = append(edits, e)
		}
		if outcomeField != nil {
			e, err := pass3Doc.SetSectionText("outcome", outcomeField.Text)
			if err != nil {
				return nil, &Finding{Code: FindingUnsupportedStructure, Target: "outcome", Message: err.Error(), Blocking: true}, nil
			}
			edits = append(edits, e)
		}
		pass2Result, err = pass3Doc.Apply(edits)
		if err != nil {
			return nil, &Finding{Code: FindingInvalidCandidate, Message: fmt.Sprintf("mirroring statement text into the body: %v", err), Blocking: true}, nil
		}
	}

	// Pass 4: append every candidate-ready object field, ONE PER Parse/
	// Apply round rather than one shared batch. AppendObject's own
	// insertion point (the end of an existing block's last element, or a
	// brand new block at the frontmatter's close) is computed from
	// whatever the CURRENT tree looks like — two fields appended from the
	// SAME static parse would both compute the identical "block does not
	// exist yet, insert one at the frontmatter close" edit (or the
	// identical "after the current last element" edit), colliding the
	// instant more than one field targets the same still-empty block.
	// Re-parsing after every single append is the simplest correct fix,
	// and object counts here are always small (one request's worth of
	// mapped statements).
	result := pass2Result
	for _, f := range objectFields {
		doc, err := splice.Parse(result)
		if err != nil {
			return nil, nil, fmt.Errorf("specimport: compose: re-parsing before appending %s: %w", f.Target, err)
		}
		objEdits, err := doc.AppendObject(f.Target, f.Text, evidenceKinds(f.Evidence))
		if err != nil {
			return nil, &Finding{Code: FindingUnsupportedStructure, Target: f.Target, Message: err.Error(), Blocking: true}, nil
		}
		result, err = doc.Apply(objEdits)
		if err != nil {
			return nil, &Finding{Code: FindingInvalidCandidate, Target: f.Target, Message: fmt.Sprintf("appending %s: %v", f.Target, err), Blocking: true}, nil
		}
	}
	return result, nil, nil
}

// splitFields separates fields (already resolved by prepareCandidateFields)
// into the problem/outcome attribute fields (nil when absent — a blocking
// missing-statement finding is already in play in that case) and every
// remaining object field (ac-/co-/dc-/oq-), in their given order — Plan's
// own documented order (spec-import-contract.md: "Map order: statements,
// then objects in source/explicit insertion order; no map-iteration
// dependence"), never re-sorted here.
func splitFields(fields []Field) (problem, outcome *Field, objects []Field) {
	for i := range fields {
		f := fields[i]
		switch f.Target {
		case "problem":
			problem = &f
		case "outcome":
			outcome = &f
		default:
			objects = append(objects, f)
		}
	}
	return problem, outcome, objects
}

// evidenceKinds converts a Field's plain-string evidence (already
// validated against the closed kind vocabulary by Request.Validate/
// Normalize) into the typed artifact.EvidenceKind slice AppendObject
// expects.
func evidenceKinds(kinds []string) []artifact.EvidenceKind {
	if len(kinds) == 0 {
		return nil
	}
	out := make([]artifact.EvidenceKind, len(kinds))
	for i, k := range kinds {
		out[i] = artifact.EvidenceKind(k)
	}
	return out
}
