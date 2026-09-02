package constitutionimpact

import (
	"bytes"
	"sort"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/policyconflict"
)

// Complete closes a plan over results returned by the existing conflict
// evaluator. Supplemental targets are copied into the witness but cannot add,
// remove, or satisfy a registered union row.
func (p Plan) Complete(evaluations []Evaluation, supplemental []SupplementalTarget) Coverage {
	reasons := cloneReasons(p.initialReasons)
	groups := make(map[string][]Evaluation, len(evaluations))
	for _, evaluation := range evaluations {
		groups[evaluation.ConsumerIdentity] = append(groups[evaluation.ConsumerIdentity], evaluation)
	}
	registered := make(map[string]bool, len(p.consumers))
	for _, consumer := range p.consumers {
		registered[consumer.identity] = true
	}
	extras := make([]string, 0)
	for identity := range groups {
		if !registered[identity] {
			extras = append(extras, identity)
		}
	}
	if len(extras) != 0 {
		sort.Strings(extras)
		reasons = append(reasons, Reason{Code: ReasonEvaluationExtra, Witnesses: extras})
	}

	rows := make([]EvaluationCoverage, 0, len(p.consumers))
	for _, expected := range p.consumers {
		row := EvaluationCoverage{
			ConsumerIdentity: expected.identity,
			Consumer:         cloneConsumer(expected.consumer),
		}
		claimed := groups[expected.identity]
		switch len(claimed) {
		case 0:
			reasons = append(reasons, Reason{Code: ReasonEvaluationOmitted, Witnesses: []string{expected.identity}})
			row.Refusal = refusal(ReasonEvaluationOmitted, expected.identity)
		case 1:
			completeEvaluation(&row, expected, claimed[0], p.proposed, &reasons)
		default:
			reasons = append(reasons, Reason{Code: ReasonEvaluationDuplicate, Witnesses: []string{expected.identity}})
			row.Refusal = refusal(ReasonEvaluationDuplicate, expected.identity)
		}
		rows = append(rows, row)
	}

	coverageConsumers := make([]Consumer, len(p.consumers))
	for i := range p.consumers {
		coverageConsumers[i] = cloneConsumer(p.consumers[i].consumer)
	}
	supplementalCopy := cloneSupplemental(supplemental)
	reasons = normalizedReasons(reasons)
	return Coverage{
		Schema:              CoverageSchema,
		Accepted:            p.accepted,
		Proposed:            p.proposed,
		Layers:              append([]LayerChange(nil), p.layers...),
		Consumers:           coverageConsumers,
		Evaluations:         rows,
		SupplementalTargets: supplementalCopy,
		State:               stateForReasons(reasons),
		Reasons:             reasons,
	}
}

func completeEvaluation(row *EvaluationCoverage, expected plannedConsumer, evaluation Evaluation, proposed InventoryEvidence, reasons *[]Reason) {
	doc, err := consumerDocFor(evaluation.Consumer)
	if err != nil {
		*reasons = append(*reasons, Reason{Code: ReasonEvaluationIdentityMismatch, Witnesses: []string{expected.identity}})
		row.Refusal = refusal(ReasonEvaluationIdentityMismatch, expected.identity)
		return
	}
	derived, err := consumerIdentityFromDoc(doc)
	if err != nil || derived != evaluation.ConsumerIdentity {
		*reasons = append(*reasons, Reason{Code: ReasonEvaluationIdentityMismatch, Witnesses: []string{expected.identity}})
		row.Refusal = refusal(ReasonEvaluationIdentityMismatch, expected.identity)
		return
	}
	canonical, err := canonjson.Marshal(doc)
	if err != nil || !bytes.Equal(canonical, expected.canonical) {
		*reasons = append(*reasons, Reason{Code: ReasonEvaluationIdentityMismatch, Witnesses: []string{expected.identity}})
		row.Refusal = refusal(ReasonEvaluationIdentityMismatch, expected.identity)
		return
	}
	if (evaluation.Result == nil) == (evaluation.Refusal == nil) {
		*reasons = append(*reasons, Reason{Code: ReasonEvaluationResultInvalid, Witnesses: []string{expected.identity}})
		row.Refusal = refusal(ReasonEvaluationResultInvalid, expected.identity)
		return
	}
	if evaluation.Refusal != nil {
		if evaluation.Refusal.Code != ReasonEvaluationUnresolved {
			*reasons = append(*reasons, Reason{Code: ReasonEvaluationResultInvalid, Witnesses: []string{expected.identity}})
			row.Refusal = refusal(ReasonEvaluationResultInvalid, expected.identity)
			return
		}
		witnesses := cloneStrings(evaluation.Refusal.Witnesses)
		sort.Strings(witnesses)
		row.Refusal = &EvaluationRefusal{Code: ReasonEvaluationUnresolved, Witnesses: witnesses}
		*reasons = append(*reasons, Reason{Code: ReasonEvaluationUnresolved, Witnesses: []string{expected.identity}})
		return
	}
	row.AcceptedManifestDigest = evaluation.AcceptedManifestDigest
	report, valid := canonicalReport(*evaluation.Result)
	if !valid {
		*reasons = append(*reasons, Reason{Code: ReasonEvaluationResultInvalid, Witnesses: []string{expected.identity}})
		row.Refusal = refusal(ReasonEvaluationResultInvalid, expected.identity)
		return
	}
	if evaluation.AcceptedManifestDigest == "" || report.Input.Target.Accepted == nil ||
		report.Input.Target.Accepted.ManifestDigest != evaluation.AcceptedManifestDigest ||
		report.Input.Target.Kind != policyconflict.TargetAcceptedContext ||
		!report.Input.Repository.Head.Known || report.Input.Repository.Head.Value != proposed.Commit ||
		(proposed.ConstitutionDigest != "" && report.Input.ConstitutionDigest != proposed.ConstitutionDigest) {
		*reasons = append(*reasons, Reason{Code: ReasonEvaluationOperandMismatch, Witnesses: []string{expected.identity}})
		row.Refusal = refusal(ReasonEvaluationOperandMismatch, expected.identity)
		return
	}
	row.Report = &report
}

func canonicalReport(result policyconflict.Result) (policyconflict.Report, bool) {
	encoded, err := policyconflict.EncodeReport(result.Report)
	if err != nil || !bytes.Equal(encoded, result.ReportBytes) {
		return policyconflict.Report{}, false
	}
	decoded, err := policyconflict.DecodeReport(encoded)
	if err != nil {
		return policyconflict.Report{}, false
	}
	return decoded, true
}

func cloneSupplemental(in []SupplementalTarget) []SupplementalTarget {
	out := make([]SupplementalTarget, len(in))
	for i, target := range in {
		out[i].Request = cloneRequest(target.Request)
		out[i].AcceptedManifestDigest = target.AcceptedManifestDigest
		if target.Result != nil {
			if report, ok := canonicalReport(*target.Result); ok {
				out[i].Result = &policyconflict.Result{
					Report: report, ReportBytes: append([]byte(nil), target.Result.ReportBytes...),
				}
			}
		}
		if target.Refusal != nil {
			out[i].Refusal = &EvaluationRefusal{Code: target.Refusal.Code, Witnesses: cloneStrings(target.Refusal.Witnesses)}
			sort.Strings(out[i].Refusal.Witnesses)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, _ := supplementalRequestDigest(out[i].Request)
		right, _ := supplementalRequestDigest(out[j].Request)
		return left < right
	})
	if out == nil {
		return []SupplementalTarget{}
	}
	return out
}

func refusal(code ReasonCode, witness string) *EvaluationRefusal {
	return &EvaluationRefusal{Code: code, Witnesses: []string{witness}}
}

func normalizedReasons(in []Reason) []Reason {
	byCode := make(map[ReasonCode]map[string]bool, len(in))
	for _, reason := range in {
		if byCode[reason.Code] == nil {
			byCode[reason.Code] = map[string]bool{}
		}
		for _, witness := range reason.Witnesses {
			byCode[reason.Code][witness] = true
		}
	}
	codes := make([]ReasonCode, 0, len(byCode))
	for code := range byCode {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool { return codes[i] < codes[j] })
	out := make([]Reason, len(codes))
	for i, code := range codes {
		witnesses := make([]string, 0, len(byCode[code]))
		for witness := range byCode[code] {
			witnesses = append(witnesses, witness)
		}
		sort.Strings(witnesses)
		out[i] = Reason{Code: code, Witnesses: witnesses}
	}
	return out
}

func stateForReasons(reasons []Reason) State {
	state := StateProven
	for _, reason := range reasons {
		switch reason.Code {
		case ReasonInventoryDuplicate, ReasonEvaluationOmitted, ReasonEvaluationExtra,
			ReasonEvaluationDuplicate, ReasonEvaluationIdentityMismatch,
			ReasonEvaluationResultInvalid, ReasonEvaluationOperandMismatch:
			return StateViolatedWithWitness
		default:
			state = StateDisclosedUnproven
		}
	}
	return state
}
