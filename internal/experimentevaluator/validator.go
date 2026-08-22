package experimentevaluator

import (
	"fmt"
	"strconv"

	"github.com/jyang234/verdi/internal/experiment"
)

// ValidateAttempt fails closed unless attempt is the exact harness-owned
// projection of input.Request. Evaluator-owned completed-result facts remain
// governed by the existing observation validators.
func ValidateAttempt(input ObserveInput, attempt Attempt) error {
	if err := input.Request.Validate(); err != nil {
		return fmt.Errorf("experimentevaluator: validate attempt request: %w", err)
	}
	if err := attempt.Outcome.Validate(); err != nil {
		return fmt.Errorf("experimentevaluator: validate attempt outcome: %w", err)
	}
	if err := validateProcessProjection(attempt.ProcessMeasurements, attempt.ProcessDisclosures); err != nil {
		return fmt.Errorf("experimentevaluator: validate attempt process facts: %w", err)
	}

	switch input.Request.Cycle.Kind {
	case experiment.CycleWarmup:
		if attempt.Observation != nil {
			return fmt.Errorf("experimentevaluator: warmup attempt returned an observation")
		}
		return nil
	case experiment.CycleMeasured:
		if attempt.Observation == nil {
			return fmt.Errorf("experimentevaluator: measured attempt requires an observation")
		}
	default:
		return fmt.Errorf("experimentevaluator: unknown attempt cycle kind %q", input.Request.Cycle.Kind)
	}

	observation := *attempt.Observation
	if err := observation.Validate(); err != nil {
		return fmt.Errorf("experimentevaluator: validate attempt observation: %w", err)
	}
	if observation.Schema != experiment.ObservationSchemaV2 {
		return fmt.Errorf("experimentevaluator: observation schema %q does not match measured schema %q", observation.Schema, experiment.ObservationSchemaV2)
	}
	if observation.ExperimentDigest != input.Request.ExperimentDigest {
		return fmt.Errorf("experimentevaluator: observation experiment digest %q does not match request %q", observation.ExperimentDigest, input.Request.ExperimentDigest)
	}
	if observation.Run != input.Request.Run {
		return fmt.Errorf("experimentevaluator: observation run %q does not match request %q", observation.Run, input.Request.Run)
	}
	if observation.Candidate != input.Request.Candidate {
		return fmt.Errorf("experimentevaluator: observation candidate %q does not match request %q", observation.Candidate, input.Request.Candidate)
	}
	if observation.Round != input.Request.Cycle.Number {
		return fmt.Errorf("experimentevaluator: observation round %d does not match request cycle %d", observation.Round, input.Request.Cycle.Number)
	}
	if !sameOutcome(attempt.Outcome, *observation.Outcome) {
		return fmt.Errorf("experimentevaluator: observation outcome does not match attempt outcome")
	}
	if err := validateObservationProcessProjection(observation, attempt.ProcessMeasurements, attempt.ProcessDisclosures); err != nil {
		return fmt.Errorf("experimentevaluator: validate observation process facts: %w", err)
	}
	return nil
}

func validateProcessProjection(measurements []experiment.Measurement, disclosures []string) error {
	if len(measurements) < 1 || len(measurements) > 2 {
		return fmt.Errorf("expected wall duration and optional peak RSS, got %d measurements", len(measurements))
	}
	if err := validateFixedProcessMeasurement(measurements[0], experiment.EvaluatorWallDurationMetricID, "ns"); err != nil {
		return err
	}
	if len(measurements) == 2 {
		if err := validateFixedProcessMeasurement(measurements[1], experiment.EvaluatorPeakRSSMetricID, "bytes"); err != nil {
			return err
		}
		if len(disclosures) != 0 {
			return fmt.Errorf("peak RSS measurement forbids process disclosures")
		}
		return nil
	}
	if len(disclosures) != 1 || disclosures[0] != experiment.PeakRSSUnavailableDisclosure {
		return fmt.Errorf("missing peak RSS requires exactly disclosure %q", experiment.PeakRSSUnavailableDisclosure)
	}
	return nil
}

func validateFixedProcessMeasurement(measurement experiment.Measurement, id, unit string) error {
	if err := measurement.Validate(); err != nil {
		return err
	}
	if measurement.ID != id || measurement.Source != experiment.SourceHarnessMeasured || measurement.Unit != unit || measurement.Value.IsBool() {
		return fmt.Errorf("process measurement %q must be %q numeric %s from %q", measurement.ID, id, unit, experiment.SourceHarnessMeasured)
	}
	value, err := strconv.ParseInt(measurement.Value.String(), 10, 64)
	if err != nil || value < 0 {
		return fmt.Errorf("process measurement %q value %q must be a nonnegative integer", id, measurement.Value.String())
	}
	return nil
}

func validateObservationProcessProjection(observation experiment.Observation, measurements []experiment.Measurement, disclosures []string) error {
	observed := make([]experiment.Measurement, 0, 2)
	for _, measurement := range observation.Measurements {
		if isFixedProcessMeasurement(measurement.ID) {
			if measurement.Source != experiment.SourceHarnessMeasured {
				return fmt.Errorf("reserved process measurement %q does not have harness custody", measurement.ID)
			}
			observed = append(observed, measurement)
			continue
		}
		if measurement.Source == experiment.SourceHarnessMeasured {
			return fmt.Errorf("unexpected harness process measurement %q", measurement.ID)
		}
	}
	if len(observed) != len(measurements) {
		return fmt.Errorf("observation carries %d fixed process measurements, attempt carries %d", len(observed), len(measurements))
	}
	for i := range measurements {
		if !sameMeasurement(observed[i], measurements[i]) {
			return fmt.Errorf("observation process measurement %d does not match attempt projection", i)
		}
	}

	disclosureCount := 0
	for _, disclosure := range observation.Disclosures {
		if disclosure == experiment.PeakRSSUnavailableDisclosure {
			disclosureCount++
		}
	}
	wantDisclosureCount := len(disclosures)
	if disclosureCount != wantDisclosureCount {
		return fmt.Errorf("observation carries %d peak-RSS-unavailable disclosures, attempt carries %d", disclosureCount, wantDisclosureCount)
	}
	return nil
}

func isFixedProcessMeasurement(id string) bool {
	return id == experiment.EvaluatorWallDurationMetricID || id == experiment.EvaluatorPeakRSSMetricID
}

func sameMeasurement(left, right experiment.Measurement) bool {
	return left.ID == right.ID &&
		left.Unit == right.Unit &&
		left.Source == right.Source &&
		left.Value.IsBool() == right.Value.IsBool() &&
		left.Value.String() == right.Value.String()
}

func sameOutcome(left, right experiment.CandidateOutcome) bool {
	if left.Kind != right.Kind || (left.Witness == nil) != (right.Witness == nil) {
		return false
	}
	return left.Witness == nil || *left.Witness == *right.Witness
}
