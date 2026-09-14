package specimport

import (
	"context"
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/designprovenance"
	"github.com/jyang234/verdi/internal/designscaffold"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

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

	decoded, err := artifact.DecodeSpec(mustFrontmatterBytes(rendered))
	if err != nil {
		return nil, append(findings, Finding{Code: FindingUnsupportedStructure, Message: fmt.Sprintf("rendered scaffold does not decode: %v", err), Blocking: true}), nil
	}
	if err := designscaffold.CheckClass(decoded, artifact.SpecClass(request.Target.Class)); err != nil {
		return nil, append(findings, Finding{Code: FindingUnsupportedStructure, Target: "target.class", Message: fmt.Sprintf("the model's template binding is incompatible with the requested class: %v", err), Blocking: true}), nil
	}

	result, blocking, err := applyCandidateEdits(rendered, decoded, request, fields)
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
//  5. splice.ApplyDraftMutations' existing edit-object operations over the
//     COMPLETED candidate, rewriting every appended object's anchor to its
//     bare id (see setBareObjectAnchors).
//
// Every placeholder removal in rounds 1-2 is conditional on `scaffold` —
// the rendered scaffold's own decoded frontmatter — actually declaring that
// placeholder. The canonical story template renders no stubs: block at all,
// and a store override template may legitimately declare neither
// placeholder: absence is not a defect to report, because the contract's
// stated post-condition is that the imported spec HAS no placeholder stub,
// which such a template already satisfies. What is never conditional is the
// error handling: a placeholder that IS declared and cannot be removed
// still fails closed.
//
// A splice error at any round is translated into a blocking Finding —
// almost always the template/candidate combination could not be composed
// (an override template missing the expected placeholder shape) — never a
// raw Go error, and never silently dropped (spec-import-contract.md:
// "Never silently drop an operation to make validation pass"). The
// returned *Finding is nil exactly when result is usable.
func applyCandidateEdits(rendered string, scaffold *artifact.SpecFrontmatter, request Request, fields []Field) ([]byte, *Finding, error) {
	problemField, outcomeField, objectFields := splitFields(fields)

	var pass1 []designprovenance.Operation
	if scaffoldDeclaresStub(scaffold, placeholderStubSlug) {
		pass1 = append(pass1, designprovenance.Operation{Op: designprovenance.OpRemoveStub, Slug: placeholderStubSlug})
	}
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

	pass1Result := []byte(rendered)
	if len(pass1) > 0 {
		var err error
		pass1Result, err = splice.ApplyDraftMutations([]byte(rendered), pass1)
		if err != nil {
			return nil, &Finding{Code: FindingInvalidCandidate, Message: fmt.Sprintf("preparing candidate: %v", err), Blocking: true}, nil
		}
	}

	pass2Result := pass1Result
	if scaffoldDeclaresAC(scaffold, placeholderACID) {
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
		pass2Result, err = pass2Doc.Apply([]splice.Edit{removeFM, removeBody})
		if err != nil {
			return nil, &Finding{Code: FindingInvalidCandidate, Message: fmt.Sprintf("removing the placeholder acceptance criterion: %v", err), Blocking: true}, nil
		}
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

	result, blocking, err := setBareObjectAnchors(result, objectFields)
	if err != nil || blocking != nil {
		return nil, blocking, err
	}
	return result, nil, nil
}

// setBareObjectAnchors rewrites every appended object's anchor to its bare
// id — the contract's declared candidate output ("Generated anchors are bare
// problem, outcome and object IDs with ## Problem, ## Outcome, ## ac-1,
// etc."). AppendObject writes "#<id>" instead, which the slug-symmetric
// ResolveObjectAnchors also accepts; its semantics are deliberately left
// alone for every other caller and this rewrite is applied only to the
// candidate being prepared, through splice's own existing edit-object
// operations (edit-ac/-constraint/-decision/-question), never a parser of
// this package's own. Object body headings are already "## <id>", so no
// body edit is needed and none is made.
//
// One ApplyDraftMutations batch is enough here — unlike pass 4's appends, an
// edit replaces an element that already exists, and ApplyDraftMutations
// re-parses between operations itself. It runs over the COMPLETED candidate,
// so its own validate-before-write gate on the starting buffer sees a whole
// spec rather than one stripped of its placeholder criterion mid-pipeline.
func setBareObjectAnchors(candidate []byte, objectFields []Field) ([]byte, *Finding, error) {
	ops := make([]designprovenance.Operation, 0, len(objectFields))
	for _, f := range objectFields {
		op, err := editObjectKind(f.Target)
		if err != nil {
			return nil, &Finding{Code: FindingUnsupportedStructure, Target: f.Target, Message: err.Error(), Blocking: true}, nil
		}
		operation := designprovenance.Operation{Op: op, ID: f.Target, Text: f.Text, Anchor: f.Target}
		if op == designprovenance.OpEditAC {
			operation.Evidence = evidenceKinds(f.Evidence)
		}
		ops = append(ops, operation)
	}
	if len(ops) == 0 {
		return candidate, nil, nil
	}
	result, err := splice.ApplyDraftMutations(candidate, ops)
	if err != nil {
		return nil, &Finding{Code: FindingInvalidCandidate, Message: fmt.Sprintf("setting generated object anchors: %v", err), Blocking: true}, nil
	}
	return result, nil, nil
}

// editObjectKind maps an object id's kind prefix onto splice's existing
// edit-object operation for that block. An unknown prefix fails closed
// rather than silently skipping the anchor rewrite; Normalize only ever
// mints the four ids below.
func editObjectKind(id string) (designprovenance.OperationKind, error) {
	switch {
	case strings.HasPrefix(id, "ac-"):
		return designprovenance.OpEditAC, nil
	case strings.HasPrefix(id, "co-"):
		return designprovenance.OpEditConstraint, nil
	case strings.HasPrefix(id, "dc-"):
		return designprovenance.OpEditDecision, nil
	case strings.HasPrefix(id, "oq-"):
		return designprovenance.OpEditQuestion, nil
	default:
		return "", fmt.Errorf("no object block owns id %q", id)
	}
}

// scaffoldDeclaresStub reports whether the rendered scaffold's frontmatter
// carries a stub with slug — the canonical story template declares none at
// all, and a store override template may legitimately declare none either.
func scaffoldDeclaresStub(scaffold *artifact.SpecFrontmatter, slug string) bool {
	for _, s := range scaffold.Stubs {
		if s.Slug == slug {
			return true
		}
	}
	return false
}

// scaffoldDeclaresAC reports whether the rendered scaffold's frontmatter
// declares an acceptance criterion with id (the generated placeholder
// criterion whose entry and orphaned body section are removed together).
func scaffoldDeclaresAC(scaffold *artifact.SpecFrontmatter, id string) bool {
	for _, ac := range scaffold.AcceptanceCriteria {
		if ac.ID == id {
			return true
		}
	}
	return false
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
