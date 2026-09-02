package constitutionimpact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/policyconflict"
)

type coverageDoc struct {
	Schema              *string                  `json:"schema"`
	Accepted            *InventoryEvidence       `json:"accepted"`
	Proposed            *InventoryEvidence       `json:"proposed"`
	Layers              *[]LayerChange           `json:"layers"`
	Consumers           *[]coverageConsumerDoc   `json:"consumers"`
	Evaluations         *[]coverageEvaluationDoc `json:"evaluations"`
	SupplementalTargets *[]supplementalTargetDoc `json:"supplemental_targets"`
	State               *State                   `json:"state"`
	Reasons             *[]Reason                `json:"reasons"`
}

type coverageConsumerDoc struct {
	Identity           *string         `json:"identity"`
	Request            json.RawMessage `json:"request"`
	Environment        *string         `json:"environment"`
	GovernedOperations *[]string       `json:"governed_operations"`
}

type coverageEvaluationDoc struct {
	Consumer *coverageConsumerDoc `json:"consumer"`
	Report   *json.RawMessage     `json:"report,omitempty"`
	Refusal  *EvaluationRefusal   `json:"refusal,omitempty"`
}

type supplementalTargetDoc struct {
	Consumer *coverageConsumerDoc `json:"consumer"`
	Report   *json.RawMessage     `json:"report,omitempty"`
	Refusal  *EvaluationRefusal   `json:"refusal,omitempty"`
}

// EncodeCoverage returns the strict canonical coverage witness.
func EncodeCoverage(coverage Coverage) ([]byte, error) {
	doc, err := coverageDocFor(coverage)
	if err != nil {
		return nil, fmt.Errorf("constitutionimpact: encoding coverage: %w", err)
	}
	out, err := canonjson.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("constitutionimpact: encoding coverage: %w", err)
	}
	return out, nil
}

// DecodeCoverage strict-decodes one canonical coverage witness and delegates
// every nested request and report to its owning codec.
func DecodeCoverage(data []byte) (Coverage, error) {
	var doc coverageDoc
	if err := artifact.DecodeClosedJSON(data, &doc); err != nil {
		return Coverage{}, fmt.Errorf("constitutionimpact: decoding coverage: %w", err)
	}
	coverage, err := coverageFromDoc(doc)
	if err != nil {
		return Coverage{}, fmt.Errorf("constitutionimpact: decoding coverage: %w", err)
	}
	canonical, err := EncodeCoverage(coverage)
	if err != nil {
		return Coverage{}, fmt.Errorf("constitutionimpact: decoding coverage: %w", err)
	}
	if !bytes.Equal(canonical, data) {
		return Coverage{}, fmt.Errorf("constitutionimpact: decoding coverage: input bytes are not canonical")
	}
	return cloneCoverage(coverage), nil
}

func coverageDocFor(coverage Coverage) (coverageDoc, error) {
	if err := validateCoverageEnvelope(coverage); err != nil {
		return coverageDoc{}, err
	}
	consumers := make([]coverageConsumerDoc, len(coverage.Consumers))
	for i, consumer := range coverage.Consumers {
		row, err := coverageConsumerDocFor(consumer)
		if err != nil {
			return coverageDoc{}, fmt.Errorf("consumers[%d]: %w", i, err)
		}
		consumers[i] = row
	}
	evaluations := make([]coverageEvaluationDoc, len(coverage.Evaluations))
	for i, evaluation := range coverage.Evaluations {
		consumer, err := coverageConsumerDocFor(evaluation.Consumer)
		if err != nil {
			return coverageDoc{}, fmt.Errorf("evaluations[%d].consumer: %w", i, err)
		}
		if *consumer.Identity != evaluation.ConsumerIdentity {
			return coverageDoc{}, fmt.Errorf("evaluations[%d]: consumer identity mismatch", i)
		}
		row := coverageEvaluationDoc{Consumer: &consumer}
		if evaluation.Report != nil {
			report, err := policyconflict.EncodeReport(*evaluation.Report)
			if err != nil {
				return coverageDoc{}, fmt.Errorf("evaluations[%d].report: %w", i, err)
			}
			raw := json.RawMessage(bytes.TrimSuffix(report, []byte{'\n'}))
			row.Report = &raw
		} else {
			refusalCopy := cloneRefusal(evaluation.Refusal)
			row.Refusal = refusalCopy
		}
		evaluations[i] = row
	}
	supplemental := make([]supplementalTargetDoc, len(coverage.SupplementalTargets))
	for i, target := range coverage.SupplementalTargets {
		consumer, err := coverageConsumerDocFor(target.Consumer)
		if err != nil {
			return coverageDoc{}, fmt.Errorf("supplemental_targets[%d].consumer: %w", i, err)
		}
		row := supplementalTargetDoc{Consumer: &consumer}
		if target.Result != nil {
			report, ok := canonicalReport(*target.Result)
			if !ok {
				return coverageDoc{}, fmt.Errorf("supplemental_targets[%d].report is not a canonical policyconflict result", i)
			}
			encoded, _ := policyconflict.EncodeReport(report)
			raw := json.RawMessage(bytes.TrimSuffix(encoded, []byte{'\n'}))
			row.Report = &raw
		} else {
			row.Refusal = cloneRefusal(target.Refusal)
		}
		supplemental[i] = row
	}
	schema := coverage.Schema
	layers := make([]LayerChange, len(coverage.Layers))
	copy(layers, coverage.Layers)
	state := coverage.State
	reasons := cloneReasons(coverage.Reasons)
	accepted, proposed := coverage.Accepted, coverage.Proposed
	return coverageDoc{
		Schema: &schema, Accepted: &accepted, Proposed: &proposed, Layers: &layers,
		Consumers: &consumers, Evaluations: &evaluations, SupplementalTargets: &supplemental,
		State: &state, Reasons: &reasons,
	}, nil
}

func coverageFromDoc(doc coverageDoc) (Coverage, error) {
	if doc.Schema == nil || doc.Accepted == nil || doc.Proposed == nil || doc.Layers == nil ||
		doc.Consumers == nil || doc.Evaluations == nil || doc.SupplementalTargets == nil ||
		doc.State == nil || doc.Reasons == nil {
		return Coverage{}, fmt.Errorf("all top-level fields are mandatory")
	}
	consumers := make([]Consumer, len(*doc.Consumers))
	for i, row := range *doc.Consumers {
		identity, consumer, err := consumerFromCoverageDoc(row)
		if err != nil {
			return Coverage{}, fmt.Errorf("consumers[%d]: %w", i, err)
		}
		if derived, _ := consumer.Identity(); identity != derived {
			return Coverage{}, fmt.Errorf("consumers[%d]: identity mismatch", i)
		}
		consumers[i] = consumer
	}
	evaluations := make([]EvaluationCoverage, len(*doc.Evaluations))
	for i, row := range *doc.Evaluations {
		if row.Consumer == nil || (row.Report == nil) == (row.Refusal == nil) {
			return Coverage{}, fmt.Errorf("evaluations[%d]: consumer and exactly one of report/refusal are mandatory", i)
		}
		identity, consumer, err := consumerFromCoverageDoc(*row.Consumer)
		if err != nil {
			return Coverage{}, fmt.Errorf("evaluations[%d].consumer: %w", i, err)
		}
		decoded := EvaluationCoverage{ConsumerIdentity: identity, Consumer: consumer}
		if row.Report != nil {
			report, err := policyconflict.DecodeReport(append(append([]byte(nil), (*row.Report)...), '\n'))
			if err != nil {
				return Coverage{}, fmt.Errorf("evaluations[%d].report: %w", i, err)
			}
			decoded.Report = &report
		} else {
			decoded.Refusal = cloneRefusal(row.Refusal)
		}
		evaluations[i] = decoded
	}
	supplemental := make([]SupplementalTarget, len(*doc.SupplementalTargets))
	for i, row := range *doc.SupplementalTargets {
		if row.Consumer == nil || (row.Report == nil) == (row.Refusal == nil) {
			return Coverage{}, fmt.Errorf("supplemental_targets[%d]: consumer and exactly one of report/refusal are mandatory", i)
		}
		_, consumer, err := consumerFromCoverageDoc(*row.Consumer)
		if err != nil {
			return Coverage{}, fmt.Errorf("supplemental_targets[%d].consumer: %w", i, err)
		}
		target := SupplementalTarget{Consumer: consumer}
		if row.Report != nil {
			reportBytes := append(append([]byte(nil), (*row.Report)...), '\n')
			report, err := policyconflict.DecodeReport(reportBytes)
			if err != nil {
				return Coverage{}, fmt.Errorf("supplemental_targets[%d].report: %w", i, err)
			}
			target.Result = &policyconflict.Result{Report: report, ReportBytes: reportBytes}
		} else {
			target.Refusal = cloneRefusal(row.Refusal)
		}
		supplemental[i] = target
	}
	coverage := Coverage{
		Schema: *doc.Schema, Accepted: *doc.Accepted, Proposed: *doc.Proposed,
		Layers: cloneLayers(*doc.Layers), Consumers: consumers,
		Evaluations: evaluations, SupplementalTargets: supplemental,
		State: *doc.State, Reasons: cloneReasons(*doc.Reasons),
	}
	if err := validateCoverageEnvelope(coverage); err != nil {
		return Coverage{}, err
	}
	return coverage, nil
}

func coverageConsumerDocFor(consumer Consumer) (coverageConsumerDoc, error) {
	doc, err := consumerDocFor(consumer)
	if err != nil {
		return coverageConsumerDoc{}, err
	}
	identity, err := consumerIdentityFromDoc(doc)
	if err != nil {
		return coverageConsumerDoc{}, err
	}
	return coverageConsumerDoc{
		Identity: &identity, Request: doc.Request, Environment: doc.Environment,
		GovernedOperations: doc.GovernedOperations,
	}, nil
}

func consumerFromCoverageDoc(doc coverageConsumerDoc) (string, Consumer, error) {
	if doc.Identity == nil {
		return "", Consumer{}, fmt.Errorf("identity is mandatory")
	}
	consumer, err := consumerFromDoc(consumerDoc{
		Request: doc.Request, Environment: doc.Environment, GovernedOperations: doc.GovernedOperations,
	})
	return *doc.Identity, consumer, err
}

func validateCoverageEnvelope(coverage Coverage) error {
	if coverage.Schema != CoverageSchema {
		return fmt.Errorf("schema %q, want %q", coverage.Schema, CoverageSchema)
	}
	if err := validateEvidence("accepted", coverage.Accepted); err != nil {
		return err
	}
	if err := validateEvidence("proposed", coverage.Proposed); err != nil {
		return err
	}
	layers, err := canonicalLayerChanges(coverage.Layers)
	if err != nil {
		return err
	}
	if !equalLayers(layers, coverage.Layers) {
		return fmt.Errorf("layers are not in canonical order")
	}
	if coverage.Consumers == nil || coverage.Evaluations == nil || coverage.SupplementalTargets == nil || coverage.Reasons == nil {
		return fmt.Errorf("consumers, evaluations, supplemental_targets, and reasons must be non-nil")
	}
	consumerIDs := make([]string, len(coverage.Consumers))
	for i, consumer := range coverage.Consumers {
		identity, err := consumer.Identity()
		if err != nil {
			return fmt.Errorf("consumers[%d]: %w", i, err)
		}
		consumerIDs[i] = identity
		if i > 0 && consumerIDs[i-1] >= identity {
			return fmt.Errorf("consumers must be sorted and unique by identity")
		}
	}
	if len(coverage.Evaluations) != len(coverage.Consumers) {
		return fmt.Errorf("evaluations must contain exactly one row per consumer")
	}
	for i, evaluation := range coverage.Evaluations {
		if evaluation.ConsumerIdentity != consumerIDs[i] {
			return fmt.Errorf("evaluations[%d] does not match consumers[%d]", i, i)
		}
		identity, err := evaluation.Consumer.Identity()
		if err != nil || identity != consumerIDs[i] {
			return fmt.Errorf("evaluations[%d] consumer identity mismatch", i)
		}
		if (evaluation.Report == nil) == (evaluation.Refusal == nil) {
			return fmt.Errorf("evaluations[%d] must carry exactly one report/refusal", i)
		}
		if evaluation.Report != nil {
			if _, err := policyconflict.EncodeReport(*evaluation.Report); err != nil {
				return fmt.Errorf("evaluations[%d].report: %w", i, err)
			}
			if !reportMatchesEvidence(*evaluation.Report, coverage.Proposed) {
				return fmt.Errorf("evaluations[%d].report is bound to different operands", i)
			}
		} else if err := validateRefusal(evaluation.Refusal); err != nil {
			return fmt.Errorf("evaluations[%d].refusal: %w", i, err)
		} else if !reasonHasWitness(coverage.Reasons, evaluation.Refusal.Code, evaluation.ConsumerIdentity) {
			return fmt.Errorf("evaluations[%d].refusal has no matching coverage reason", i)
		}
	}
	previousSupplemental := ""
	for i, target := range coverage.SupplementalTargets {
		identity, err := target.Consumer.Identity()
		if err != nil {
			return fmt.Errorf("supplemental_targets[%d].consumer: %w", i, err)
		}
		if i > 0 && previousSupplemental > identity {
			return fmt.Errorf("supplemental_targets are not in canonical order")
		}
		previousSupplemental = identity
		if (target.Result == nil) == (target.Refusal == nil) {
			return fmt.Errorf("supplemental_targets[%d] must carry exactly one result/refusal", i)
		}
		if target.Result != nil {
			if _, ok := canonicalReport(*target.Result); !ok {
				return fmt.Errorf("supplemental_targets[%d].result is not canonical", i)
			}
		} else if err := validateRefusal(target.Refusal); err != nil {
			return fmt.Errorf("supplemental_targets[%d].refusal: %w", i, err)
		}
	}
	normalized := normalizedReasons(coverage.Reasons)
	if !equalReasons(normalized, coverage.Reasons) {
		return fmt.Errorf("reasons are not canonical")
	}
	for _, reason := range coverage.Reasons {
		if !knownReason(reason.Code) {
			return fmt.Errorf("unknown reason code %q", reason.Code)
		}
	}
	if want := stateForReasons(coverage.Reasons); coverage.State != want {
		return fmt.Errorf("state %q does not equal derived state %q", coverage.State, want)
	}
	if err := validateEvidenceReasons(coverage); err != nil {
		return err
	}
	return nil
}

func validateEvidence(name string, evidence InventoryEvidence) error {
	if evidence.Commit == "" || evidence.Tree == "" {
		return fmt.Errorf("%s evidence commit and tree are mandatory", name)
	}
	switch evidence.Presence {
	case PresencePresent:
		if evidence.InventoryDigest == "" {
			return fmt.Errorf("%s present inventory digest is mandatory", name)
		}
	case PresenceMissing, PresenceUnavailable:
		if evidence.InventoryDigest != "" || evidence.ConstitutionDigest != "" {
			return fmt.Errorf("%s absent inventory cannot carry artifact digests", name)
		}
	default:
		return fmt.Errorf("%s unknown inventory presence %q", name, evidence.Presence)
	}
	return nil
}

func validateEvidenceReasons(coverage Coverage) error {
	has := func(code ReasonCode) bool {
		for _, reason := range coverage.Reasons {
			if reason.Code == code {
				return true
			}
		}
		return false
	}
	checks := []struct {
		need bool
		code ReasonCode
	}{
		{coverage.Accepted.Presence == PresenceMissing, ReasonAcceptedInventoryMissing},
		{coverage.Proposed.Presence == PresenceMissing, ReasonProposedInventoryMissing},
		{coverage.Accepted.Presence == PresenceUnavailable, ReasonAcceptedTreeUnavailable},
		{coverage.Proposed.Presence == PresenceUnavailable, ReasonProposedTreeUnavailable},
		{coverage.Accepted.Presence == PresencePresent && coverage.Accepted.ConstitutionDigest == "", ReasonAcceptedCatalogUnavailable},
		{coverage.Proposed.Presence == PresencePresent && coverage.Proposed.ConstitutionDigest == "", ReasonProposedCatalogUnavailable},
		{len(coverage.Layers) != 0 && len(coverage.Consumers) == 0 && coverage.Accepted.Presence == PresencePresent && coverage.Proposed.Presence == PresencePresent, ReasonConsumerUniverseEmpty},
	}
	for _, check := range checks {
		if check.need != has(check.code) {
			return fmt.Errorf("reason %q presence does not match evidence", check.code)
		}
	}
	return nil
}

func reasonHasWitness(reasons []Reason, code ReasonCode, witness string) bool {
	for _, reason := range reasons {
		if reason.Code != code {
			continue
		}
		for _, candidate := range reason.Witnesses {
			if candidate == witness {
				return true
			}
		}
	}
	return false
}

func validateRefusal(refusal *EvaluationRefusal) error {
	if refusal == nil || !knownReason(refusal.Code) || refusal.Witnesses == nil {
		return fmt.Errorf("invalid refusal")
	}
	if refusal.Code != ReasonEvaluationUnresolved && refusal.Code != ReasonEvaluationOmitted &&
		refusal.Code != ReasonEvaluationDuplicate && refusal.Code != ReasonEvaluationIdentityMismatch &&
		refusal.Code != ReasonEvaluationResultInvalid && refusal.Code != ReasonEvaluationOperandMismatch {
		return fmt.Errorf("reason %q is not an evaluation refusal", refusal.Code)
	}
	if !sort.StringsAreSorted(refusal.Witnesses) {
		return fmt.Errorf("witnesses are not sorted")
	}
	for i := 1; i < len(refusal.Witnesses); i++ {
		if refusal.Witnesses[i-1] == refusal.Witnesses[i] {
			return fmt.Errorf("witnesses are not unique")
		}
	}
	return nil
}

func reportMatchesEvidence(report policyconflict.Report, proposed InventoryEvidence) bool {
	return report.Input.Target.Kind == policyconflict.TargetAcceptedContext &&
		report.Input.Repository.Head.Known && report.Input.Repository.Head.Value == proposed.Commit &&
		(proposed.ConstitutionDigest == "" || report.Input.ConstitutionDigest == proposed.ConstitutionDigest)
}

func knownReason(code ReasonCode) bool {
	switch code {
	case ReasonAcceptedInventoryMissing, ReasonProposedInventoryMissing,
		ReasonAcceptedTreeUnavailable, ReasonProposedTreeUnavailable,
		ReasonAcceptedCatalogUnavailable, ReasonProposedCatalogUnavailable,
		ReasonConsumerUniverseEmpty, ReasonInventoryDuplicate,
		ReasonEvaluationOmitted, ReasonEvaluationExtra, ReasonEvaluationDuplicate,
		ReasonEvaluationIdentityMismatch, ReasonEvaluationResultInvalid,
		ReasonEvaluationOperandMismatch, ReasonEvaluationUnresolved:
		return true
	default:
		return false
	}
}

func cloneRefusal(in *EvaluationRefusal) *EvaluationRefusal {
	if in == nil {
		return nil
	}
	return &EvaluationRefusal{Code: in.Code, Witnesses: cloneStrings(in.Witnesses)}
}

func cloneCoverage(in Coverage) Coverage {
	out := in
	out.Layers = cloneLayers(in.Layers)
	out.Consumers = make([]Consumer, len(in.Consumers))
	for i := range in.Consumers {
		out.Consumers[i] = cloneConsumer(in.Consumers[i])
	}
	out.Evaluations = make([]EvaluationCoverage, len(in.Evaluations))
	for i, row := range in.Evaluations {
		out.Evaluations[i] = EvaluationCoverage{ConsumerIdentity: row.ConsumerIdentity, Consumer: cloneConsumer(row.Consumer), Refusal: cloneRefusal(row.Refusal)}
		if row.Report != nil {
			encoded, _ := policyconflict.EncodeReport(*row.Report)
			decoded, _ := policyconflict.DecodeReport(encoded)
			out.Evaluations[i].Report = &decoded
		}
	}
	out.SupplementalTargets = cloneSupplemental(in.SupplementalTargets)
	out.Reasons = cloneReasons(in.Reasons)
	return out
}

func equalLayers(left, right []LayerChange) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func cloneLayers(in []LayerChange) []LayerChange {
	if in == nil {
		return nil
	}
	out := make([]LayerChange, len(in))
	copy(out, in)
	return out
}

func equalReasons(left, right []Reason) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i].Code != right[i].Code || len(left[i].Witnesses) != len(right[i].Witnesses) {
			return false
		}
		for j := range left[i].Witnesses {
			if left[i].Witnesses[j] != right[i].Witnesses[j] {
				return false
			}
		}
	}
	return true
}
