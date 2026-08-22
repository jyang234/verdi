//go:build linux

package experimentevaluator

import (
	"math"
	"syscall"
	"testing"
)

func TestPeakRSSBytesLinuxUsesRusageKilobytes(t *testing.T) {
	got, ok, err := peakRSSBytes(fakeProcessState{success: true, usage: &syscall.Rusage{Maxrss: 321}})
	if err != nil {
		t.Fatalf("peakRSSBytes: %v", err)
	}
	if !ok || got != 321*1024 {
		t.Fatalf("peakRSSBytes = %d, %v; want %d, true", got, ok, 321*1024)
	}
}

func TestPeakRSSBytesLinuxDisclosesUnavailableState(t *testing.T) {
	got, ok, err := peakRSSBytes(fakeProcessState{success: true})
	if err != nil {
		t.Fatalf("peakRSSBytes: %v", err)
	}
	if ok || got != 0 {
		t.Fatalf("peakRSSBytes = %d, %v; want 0, false", got, ok)
	}
}

func TestPeakRSSBytesLinuxRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		maxRSS int64
	}{
		{name: "negative", maxRSS: -1},
		{name: "byte conversion overflow", maxRSS: math.MaxInt64/1024 + 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := peakRSSBytes(fakeProcessState{success: true, usage: &syscall.Rusage{Maxrss: tt.maxRSS}})
			if err == nil {
				t.Fatal("peakRSSBytes error = nil, want invalid Linux ru_maxrss error")
			}
		})
	}
}
