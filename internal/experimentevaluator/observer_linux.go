//go:build linux

package experimentevaluator

import (
	"fmt"
	"math"
	"syscall"
)

func peakRSSBytes(state processState) (int64, bool, error) {
	if state == nil {
		return 0, false, nil
	}
	usage, ok := state.SysUsage().(*syscall.Rusage)
	if !ok || usage == nil {
		return 0, false, nil
	}
	peakKiB := int64(usage.Maxrss)
	if peakKiB < 0 {
		return 0, false, fmt.Errorf("negative Linux ru_maxrss %d", peakKiB)
	}
	if peakKiB > math.MaxInt64/1024 {
		return 0, false, fmt.Errorf("linux ru_maxrss %d KiB overflows bytes", peakKiB)
	}
	return peakKiB * 1024, true, nil
}
