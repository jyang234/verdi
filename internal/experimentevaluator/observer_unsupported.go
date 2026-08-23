//go:build !linux

package experimentevaluator

func peakRSSBytes(processState) (int64, bool, error) {
	return 0, false, nil
}
