package specdoc

import "github.com/jyang234/verdi/internal/readinesspilot"

// ReadinessFacts is the readiness snapshot as the document reports it:
// the areas in their fixed order, the current focus, and the attention
// queue in the snapshot's own order. Supplied only when the snapshot
// targets this document's spec (spec/spec-documents ac-1: "the readiness
// snapshot when present").
type ReadinessFacts struct {
	TargetRef    string
	Head         string
	CurrentFocus string
	StaleNotice  string
	Areas        []ReadinessArea
	Attention    []ReadinessConcern
}

// ReadinessArea is one of the four ordered areas with its state.
type ReadinessArea struct {
	ID    string
	Label string
	State string
}

// ReadinessConcern is one attention-queue entry.
type ReadinessConcern struct {
	ID        string
	Area      string
	State     string
	Summary   string
	Blocking  bool
	Timing    string
	Witnesses []string
}

// WithReadiness copies f and supplies readiness when snap targets ref.
// A snapshot for another spec leaves Readiness nil, which renders as
// "not supplied" — never as another spec's facts.
func WithReadiness(f Facts, snap readinesspilot.Snapshot, ref string) Facts {
	out := f
	if snap.TargetRef != ref {
		return out
	}
	rf := &ReadinessFacts{
		TargetRef:    snap.TargetRef,
		Head:         snap.Head,
		CurrentFocus: string(snap.CurrentFocus),
		StaleNotice:  snap.StaleNotice,
	}
	for _, a := range snap.Areas {
		rf.Areas = append(rf.Areas, ReadinessArea{ID: string(a.ID), Label: a.Label, State: string(a.State)})
	}
	for _, c := range snap.Attention {
		rf.Attention = append(rf.Attention, ReadinessConcern{
			ID:        c.ID,
			Area:      string(c.Area),
			State:     string(c.State),
			Summary:   c.Summary,
			Blocking:  c.Blocking,
			Timing:    string(c.Timing),
			Witnesses: append([]string(nil), c.Witnesses...),
		})
	}
	out.Readiness = rf
	return out
}
