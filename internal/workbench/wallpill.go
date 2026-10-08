package workbench

// The readiness pill's facts (spec/wall-strip-and-drawer-v2 ac-4; ledger
// SI-368 (2)): the current step and the unresolved count, read from the
// readiness the wall's composed refresh already loaded for its marks, or
// the marks' reason where that readiness cannot be read. Facts only; the
// pill's markup is a renderer's (lane F3a).

// wallPill is the readiness pill's facts (SI-368 (2)): the current step's
// position (1 to 4; 0 when every step is proven) and the count of
// unresolved concerns — the snapshot's whole Focus next list (SI-350 (1))
// — or, when the wall's readiness cannot be read, the one reason, under
// the marks' rules.
type wallPill struct {
	Step        int    `json:"step"`
	Unresolved  int    `json:"unresolved"`
	Unavailable string `json:"unavailable,omitempty"`
}

// deriveWallPill derives the pill from the readiness a composed refresh
// already loaded (SI-368 (2)): the marks' input reason when that
// readiness cannot be read for this wall (readinessUnreadable), else its
// current step and unresolved count. A stub collision, which leaves only
// the marks unable to name one card, leaves the pill readable.
func deriveWallPill(in wallMarksInput) wallPill {
	if reason := readinessUnreadable(in); reason != "" {
		return wallPill{Unavailable: reason}
	}
	step, _ := readinessCurrentStep(*in.Readiness)
	return wallPill{Step: step, Unresolved: len(in.Readiness.Attention)}
}

// fixedPill is the pill of a wall whose marks are fixed — for the server
// instance, or the sealed render: the marks' one reason (SI-364 (3)), so
// every response that carries them carries the same pill; nil when the
// marks are not fixed, where only a composed refresh derives the pill.
func fixedPill(marks *wallMarks) *wallPill {
	if marks == nil {
		return nil
	}
	return &wallPill{Unavailable: marks.Unavailable}
}
