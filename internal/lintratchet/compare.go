package lintratchet

import "fmt"

// Kind is a violation's closed kind.
type Kind int

const (
	// KindNew: the current run reports a key more times than the committed
	// baseline allows.
	KindNew Kind = iota + 1
	// KindStale: the committed baseline allows a key more times than the
	// current run reports, so a fixed finding's allowance was left behind.
	KindStale
	// KindGrowth: the committed baseline allows a key more times than the
	// baseline at the merge base (or HEAD's first parent) did.
	KindGrowth
)

// String names the kind; a value outside the closed set renders as unknown,
// never as one of the three.
func (k Kind) String() string {
	switch k {
	case KindNew:
		return "new finding"
	case KindStale:
		return "stale allowance"
	case KindGrowth:
		return "baseline grew"
	default:
		return fmt.Sprintf("unknown violation kind %d", int(k))
	}
}

// Violation is one key a comparison fails. Have is the count on the side
// being checked (the current run for KindNew and KindStale, the committed
// baseline for KindGrowth); Allowed is what the other side allows.
type Violation struct {
	Kind    Kind
	Key     Key
	Have    int
	Allowed int
}

// Compare compares the current run's counts with the committed baseline's:
// every key the run reports more times than the baseline allows (KindNew),
// then every key the baseline allows more times than the run reports
// (KindStale), each in key order. Together they hold the baseline equal to
// the current findings.
func Compare(current, baseline Counts) []Violation {
	var out []Violation
	for _, k := range sortedKeys(current) {
		if current[k] > baseline[k] {
			out = append(out, Violation{Kind: KindNew, Key: k, Have: current[k], Allowed: baseline[k]})
		}
	}
	for _, k := range sortedKeys(baseline) {
		if baseline[k] > current[k] {
			out = append(out, Violation{Kind: KindStale, Key: k, Have: current[k], Allowed: baseline[k]})
		}
	}
	return out
}

// CompareGrowth returns every key the committed baseline allows more times
// than the earlier baseline did, in key order.
func CompareGrowth(baseline, earlier Counts) []Violation {
	var out []Violation
	for _, k := range sortedKeys(baseline) {
		if baseline[k] > earlier[k] {
			out = append(out, Violation{Kind: KindGrowth, Key: k, Have: baseline[k], Allowed: earlier[k]})
		}
	}
	return out
}
