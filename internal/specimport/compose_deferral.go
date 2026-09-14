package specimport

import (
	"fmt"

	"github.com/jyang234/verdi/internal/designscaffold"
)

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
