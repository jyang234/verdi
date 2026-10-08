package workbench

// The wall's readiness facts beside its marks (spec/wall-strip-and-drawer-v2
// ac-4, ac-5, ac-7; ledger SI-368): the drawer's Readiness tab, served on
// demand from one readiness load and rendered by the readiness page's own
// body renderer in the wall's words, and each of its items' wall targets.

// readinessTarget is one Readiness tab item's wall target (SI-368 (16)):
// what clicking the item selects on the wall. Kind is one of the
// readinessTarget* kinds below; Value is the object id, the stub slug,
// the strip half (problem or outcome), or the slot's object kind.
type readinessTarget struct {
	Kind  string
	Value string
}

// The Readiness tab items' target kinds (SI-368 (16)): an object's card,
// a stub's card, a half of the case-file strip, an object column's add
// slot, or none — a plain row that selects nothing.
const (
	readinessTargetObject = "object"
	readinessTargetStub   = "stub"
	readinessTargetStrip  = "strip"
	readinessTargetSlot   = "slot"
	readinessTargetNone   = "none"
)
