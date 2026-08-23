package experimentevaluator

import (
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/experiment"
)

func TestValidateAttemptAcceptsExactProcessProjectionAndEvaluatorFacts(t *testing.T) {
	completed := experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}
	wall := numericMeasurement(experiment.EvaluatorWallDurationMetricID, 25, "ns")
	rss := numericMeasurement(experiment.EvaluatorPeakRSSMetricID, 4096, "bytes")

	for _, test := range []struct {
		name    string
		input   ObserveInput
		attempt Attempt
	}{
		{
			name:  "warmup keeps exact process facts transient",
			input: ObserveInput{Request: validProtocolRequest(experiment.CycleWarmup)},
			attempt: Attempt{
				Outcome:             completed,
				ProcessMeasurements: []experiment.Measurement{wall},
				ProcessDisclosures:  []string{experiment.PeakRSSUnavailableDisclosure},
			},
		},
		{
			name:  "completed measurement preserves evaluator facts",
			input: ObserveInput{Request: validProtocolRequest(experiment.CycleMeasured)},
			attempt: Attempt{
				Outcome:             completed,
				ProcessMeasurements: []experiment.Measurement{wall, rss},
				ProcessDisclosures:  []string{},
				Observation: measuredAttemptObservation(
					validProtocolRequest(experiment.CycleMeasured),
					completed,
					[]experiment.Measurement{
						{ID: "latency", Value: experiment.NumberValue("18.0"), Unit: "ms", Source: experiment.SourceEvaluatorMeasured},
						wall,
						rss,
					},
					[]string{"evaluator-used-fixed-fixture"},
				),
			},
		},
		{
			name:  "candidate failure keeps only unavailable RSS projection",
			input: ObserveInput{Request: validProtocolRequest(experiment.CycleMeasured)},
			attempt: func() Attempt {
				witness := "candidate process crashed"
				outcome := experiment.CandidateOutcome{Kind: experiment.OutcomeCandidateCrash, Witness: &witness}
				return Attempt{
					Outcome:             outcome,
					ProcessMeasurements: []experiment.Measurement{wall},
					ProcessDisclosures:  []string{experiment.PeakRSSUnavailableDisclosure},
					Observation: measuredAttemptObservation(
						validProtocolRequest(experiment.CycleMeasured),
						outcome,
						[]experiment.Measurement{wall},
						[]string{experiment.PeakRSSUnavailableDisclosure},
					),
				}
			}(),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateAttempt(test.input, test.attempt); err != nil {
				t.Fatalf("ValidateAttempt: %v", err)
			}
		})
	}
}

func TestValidateAttemptRejectsIdentityCycleAndOutcomeMismatch(t *testing.T) {
	request := validProtocolRequest(experiment.CycleMeasured)
	completed := experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}
	wall := numericMeasurement(experiment.EvaluatorWallDurationMetricID, 25, "ns")
	valid := Attempt{
		Outcome:             completed,
		ProcessMeasurements: []experiment.Measurement{wall},
		ProcessDisclosures:  []string{experiment.PeakRSSUnavailableDisclosure},
		Observation: measuredAttemptObservation(
			request,
			completed,
			[]experiment.Measurement{wall},
			[]string{experiment.PeakRSSUnavailableDisclosure},
		),
	}

	for _, test := range []struct {
		name   string
		mutate func(*ObserveInput, *Attempt)
		want   string
	}{
		{name: "experiment digest", mutate: func(_ *ObserveInput, a *Attempt) {
			a.Observation.ExperimentDigest = "sha256:" + strings.Repeat("f", 64)
		}, want: "experiment digest"},
		{name: "run", mutate: func(_ *ObserveInput, a *Attempt) { a.Observation.Run = "run-2" }, want: "run"},
		{name: "candidate", mutate: func(_ *ObserveInput, a *Attempt) { a.Observation.Candidate = "candidate-b" }, want: "candidate"},
		{name: "round", mutate: func(_ *ObserveInput, a *Attempt) { a.Observation.Round = 2 }, want: "round"},
		{name: "measured observation missing", mutate: func(_ *ObserveInput, a *Attempt) { a.Observation = nil }, want: "requires an observation"},
		{name: "warmup observation present", mutate: func(input *ObserveInput, _ *Attempt) { input.Request.Cycle.Kind = experiment.CycleWarmup }, want: "warmup"},
		{name: "outcome", mutate: func(_ *ObserveInput, a *Attempt) {
			witness := "candidate timed out"
			a.Outcome = experiment.CandidateOutcome{Kind: experiment.OutcomeCandidateTimeout, Witness: &witness}
		}, want: "outcome"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := ObserveInput{Request: request}
			attempt := cloneAttempt(valid)
			test.mutate(&input, &attempt)
			if err := ValidateAttempt(input, attempt); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("ValidateAttempt error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateAttemptRejectsMissingForgedOrExtraProcessFacts(t *testing.T) {
	request := validProtocolRequest(experiment.CycleMeasured)
	completed := experiment.CandidateOutcome{Kind: experiment.OutcomeCompleted}
	wall := numericMeasurement(experiment.EvaluatorWallDurationMetricID, 25, "ns")
	valid := Attempt{
		Outcome:             completed,
		ProcessMeasurements: []experiment.Measurement{wall},
		ProcessDisclosures:  []string{experiment.PeakRSSUnavailableDisclosure},
		Observation: measuredAttemptObservation(
			request,
			completed,
			[]experiment.Measurement{wall},
			[]string{experiment.PeakRSSUnavailableDisclosure},
		),
	}

	for _, test := range []struct {
		name   string
		mutate func(*Attempt)
	}{
		{name: "missing attempt duration", mutate: func(a *Attempt) { a.ProcessMeasurements = nil }},
		{name: "wrong attempt duration unit", mutate: func(a *Attempt) { a.ProcessMeasurements[0].Unit = "ms" }},
		{name: "negative attempt duration", mutate: func(a *Attempt) { a.ProcessMeasurements[0].Value = experiment.NumberValue("-1") }},
		{name: "extra attempt process measurement", mutate: func(a *Attempt) {
			a.ProcessMeasurements = append(a.ProcessMeasurements, numericMeasurement("extra-process-fact", 1, "bytes"))
		}},
		{name: "missing unavailable RSS disclosure", mutate: func(a *Attempt) { a.ProcessDisclosures = nil }},
		{name: "extra attempt process disclosure", mutate: func(a *Attempt) { a.ProcessDisclosures = append(a.ProcessDisclosures, "extra-process-fact") }},
		{name: "missing observation duration", mutate: func(a *Attempt) { a.Observation.Measurements = nil }},
		{name: "forged observation duration", mutate: func(a *Attempt) { a.Observation.Measurements[0].Value = experiment.NumberValue("26") }},
		{name: "extra observation harness fact", mutate: func(a *Attempt) {
			a.Observation.Measurements = append(a.Observation.Measurements, numericMeasurement(experiment.EvaluatorPeakRSSMetricID, 4096, "bytes"))
		}},
		{name: "reserved observation fact claims evaluator custody", mutate: func(a *Attempt) { a.Observation.Measurements[0].Source = experiment.SourceEvaluatorMeasured }},
		{name: "missing observation process disclosure", mutate: func(a *Attempt) { a.Observation.Disclosures = nil }},
		{name: "duplicate observation process disclosure", mutate: func(a *Attempt) {
			a.Observation.Disclosures = append(a.Observation.Disclosures, experiment.PeakRSSUnavailableDisclosure)
		}},
		{name: "RSS-present observation has nil disclosures", mutate: func(a *Attempt) {
			rss := numericMeasurement(experiment.EvaluatorPeakRSSMetricID, 4096, "bytes")
			a.ProcessMeasurements = append(a.ProcessMeasurements, rss)
			a.ProcessDisclosures = nil
			a.Observation.Measurements = append(a.Observation.Measurements, rss)
			a.Observation.Disclosures = nil
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			attempt := cloneAttempt(valid)
			test.mutate(&attempt)
			if err := ValidateAttempt(ObserveInput{Request: request}, attempt); err == nil {
				t.Fatal("ValidateAttempt error = nil, want fixed process projection refusal")
			}
		})
	}
}

func measuredAttemptObservation(request experiment.EvaluatorRequest, outcome experiment.CandidateOutcome, measurements []experiment.Measurement, disclosures []string) *experiment.Observation {
	return &experiment.Observation{
		Schema:           experiment.ObservationSchemaV2,
		ExperimentDigest: request.ExperimentDigest,
		Run:              request.Run,
		Candidate:        request.Candidate,
		Round:            request.Cycle.Number,
		Outcome:          &outcome,
		Guards:           []experiment.GuardResult{},
		Measurements:     append([]experiment.Measurement(nil), measurements...),
		Disclosures:      append([]string(nil), disclosures...),
	}
}

func cloneAttempt(attempt Attempt) Attempt {
	clone := attempt
	clone.ProcessMeasurements = append([]experiment.Measurement(nil), attempt.ProcessMeasurements...)
	clone.ProcessDisclosures = append([]string(nil), attempt.ProcessDisclosures...)
	if attempt.Observation != nil {
		observation := *attempt.Observation
		observation.Guards = append([]experiment.GuardResult(nil), attempt.Observation.Guards...)
		observation.Measurements = append([]experiment.Measurement(nil), attempt.Observation.Measurements...)
		observation.Disclosures = append([]string(nil), attempt.Observation.Disclosures...)
		clone.Observation = &observation
	}
	return clone
}
