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
// DID resolve is never silently discarded: it is named, value and origin
// both, in a nonblocking statements-deferred disclosure Finding (see
// deferralDisclosure), and its own now-superseded missing-statement
// BLOCKING finding (present only when nothing was resolved at all) is
// dropped — superseded by the deferral that just resolved it, not left
// blocking a candidate the request explicitly asked to defer.
//
// The returned fields are ordered statements first, then objects in their
// Plan order (spec-import-contract.md: "Map order: statements, then
// objects in source/explicit insertion order"). Task 3's Preview consumes
// this same slice and reports it inside a SHA-256-over-canonical-JSON
// digest, so the order is part of what that digest freezes.
func prepareCandidateFields(request Request, plan Plan) ([]Field, []Finding) {
	fields := make([]Field, 0, len(plan.Fields))

	if !request.DeferStatements {
		fields = append(fields, plan.Fields...)
		findings := make([]Finding, len(plan.Findings))
		copy(findings, plan.Findings)
		return fields, findings
	}

	displaced := map[string]Field{}
	var objects []Field
	for _, f := range plan.Fields {
		if f.Target == "problem" || f.Target == "outcome" {
			displaced[f.Target] = f
			continue
		}
		objects = append(objects, f)
	}
	for _, target := range []string{"problem", "outcome"} {
		fields = append(fields, deferredField(target))
	}
	fields = append(fields, objects...)

	findings := make([]Finding, 0, len(plan.Findings)+2)
	for _, f := range plan.Findings {
		if f.Code == FindingMissingStatement && (f.Target == "problem" || f.Target == "outcome") {
			continue // superseded: the loop above just resolved this target
		}
		findings = append(findings, f)
	}
	for _, target := range []string{"problem", "outcome"} {
		orig, ok := displaced[target]
		findings = append(findings, deferralDisclosure(target, orig, ok))
	}
	return fields, findings
}

// deferralDisclosure builds the nonblocking statements-deferred Finding for
// target, stating only what this call establishes: a generated placeholder
// was inserted, and — when Normalize had already resolved a value —
// naming that value and its ORIGIN as displaced from the candidate.
//
// It deliberately claims neither that the value is "source text" nor that
// it "was retained". Neither is establishable here. An origin of
// user-added carries no span and appears in no source at all, so calling
// it a source quotation would fabricate an attribution the contract
// explicitly forbids ("Placeholders are visibly incomplete and not source
// quotations"); and retention is a COVERAGE disposition, which Compose
// neither computes nor returns — plan.Coverage still records a displaced
// span as mapped, and recomputing it from the candidate-ready fields is
// Task 3's Preview obligation. A disclosure that asserted the disposition
// would contradict the only coverage record the same call holds.
func deferralDisclosure(target string, displaced Field, wasDisplaced bool) Finding {
	msg := fmt.Sprintf("%s statement deferred; a generated placeholder was inserted instead", target)
	if wasDisplaced {
		msg = fmt.Sprintf("%s statement deferred; a generated placeholder was inserted and the previously resolved %s value %q was displaced from the candidate", target, displaced.Origin, displaced.Text)
	}
	return Finding{Code: FindingStatementsDeferred, Target: target, Message: msg, Blocking: false}
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
