package experimentevaluator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/jyang234/verdi/internal/experiment"
)

type processState interface {
	Success() bool
	SysUsage() any
}

func processObservations(duration time.Duration, state processState) ([]experiment.Measurement, []string, error) {
	if duration < 0 {
		return nil, nil, fmt.Errorf("negative wall duration %s", duration)
	}
	measurements := []experiment.Measurement{numericMeasurement(
		experiment.EvaluatorWallDurationMetricID,
		duration.Nanoseconds(),
		"ns",
	)}
	peakRSS, available, err := peakRSSBytes(state)
	if err != nil {
		return nil, nil, fmt.Errorf("peak RSS: %w", err)
	}
	if !available {
		return measurements, []string{experiment.PeakRSSUnavailableDisclosure}, nil
	}
	measurements = append(measurements, numericMeasurement(
		experiment.EvaluatorPeakRSSMetricID,
		peakRSS,
		"bytes",
	))
	return measurements, []string{}, nil
}

func numericMeasurement(id string, value int64, unit string) experiment.Measurement {
	return experiment.Measurement{
		ID:     id,
		Value:  experiment.NumberValue(json.Number(strconv.FormatInt(value, 10))),
		Unit:   unit,
		Source: experiment.SourceHarnessMeasured,
	}
}
