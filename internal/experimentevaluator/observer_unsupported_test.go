//go:build !linux

package experimentevaluator

import "testing"

func TestPeakRSSBytesUnsupportedReportsUnavailableWithoutSkip(t *testing.T) {
	got, ok, err := peakRSSBytes(fakeProcessState{success: true, usage: struct{}{}})
	if err != nil {
		t.Fatalf("peakRSSBytes: %v", err)
	}
	if ok || got != 0 {
		t.Fatalf("peakRSSBytes = %d, %v; want 0, false", got, ok)
	}
}
