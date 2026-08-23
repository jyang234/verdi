package experimentevaluator

import (
	"reflect"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/experiment"
)

func TestProcessObservationsDisclosesUnavailablePeakRSS(t *testing.T) {
	measurements, disclosures, err := processObservations(17*time.Nanosecond, fakeProcessState{success: true})
	if err != nil {
		t.Fatalf("processObservations: %v", err)
	}
	if len(measurements) != 1 || measurements[0].ID != experiment.EvaluatorWallDurationMetricID || measurements[0].Value.String() != "17" {
		t.Fatalf("measurements = %+v, want only 17ns wall duration", measurements)
	}
	if want := []string{experiment.PeakRSSUnavailableDisclosure}; !reflect.DeepEqual(disclosures, want) {
		t.Fatalf("disclosures = %+v, want %+v", disclosures, want)
	}
}

func TestProcessObservationsRejectsNegativeDuration(t *testing.T) {
	_, _, err := processObservations(-time.Nanosecond, fakeProcessState{success: true})
	if err == nil {
		t.Fatal("processObservations error = nil, want negative duration rejection")
	}
}
