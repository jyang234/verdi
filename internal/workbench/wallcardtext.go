package workbench

import "strings"

// The wall canvas's card copy (spec/wall-canvas-v2 ac-1; the redesign
// handoff's "Stub card" and "Sticky" bullets): two pure projections of
// facts the canvas already renders, so the card lines and their tests
// share one source and the renderer stays a function of the projection.

// stubMetaText is the stub card's meta line, `resolves oq-2 · claims no AC
// yet` in the handoff's words: what the stub resolves, when it resolves
// anything, then what it claims — the claims part always present, so a
// stub that covers nothing says so instead of going silent. Ids keep
// their declaration order; an empty id is no claim.
func stubMetaText(resolves, acs []string) string {
	var parts []string
	if r := nonEmptyIDs(resolves); len(r) > 0 {
		parts = append(parts, "resolves "+strings.Join(r, ", "))
	}
	if a := nonEmptyIDs(acs); len(a) > 0 {
		parts = append(parts, "claims "+strings.Join(a, ", "))
	} else {
		parts = append(parts, "claims no AC yet")
	}
	return strings.Join(parts, " · ")
}

// stickyFootText is the sticky's footer line, `scratch · graduate or
// delete` in the handoff's words, worded by what the wall offers the
// sticky: graduation writes the spec, so it needs the domain live;
// deletion is a scratch write, so it needs authoring; a mirror or a
// sealed record offers neither and the line says only what the paper is.
func stickyFootText(authoring, domainLive bool) string {
	switch {
	case authoring && domainLive:
		return "scratch · graduate or delete"
	case authoring:
		return "scratch · delete"
	default:
		return "scratch"
	}
}

// nonEmptyIDs drops empty ids, keeping the rest in order.
func nonEmptyIDs(ids []string) []string {
	var out []string
	for _, id := range ids {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}
