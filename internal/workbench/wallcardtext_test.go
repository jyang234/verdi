package workbench

import "testing"

// TestStubMetaText pins the stub card's meta line (spec/wall-canvas-v2
// ac-1; handoff "Stub card": `resolves oq-2 · claims no AC yet`): a pure
// projection of the stub's declared resolves and acceptance criteria, in
// declaration order, the claims part always present.
func TestStubMetaText(t *testing.T) {
	for _, tc := range []struct {
		name     string
		resolves []string
		acs      []string
		want     string
	}{
		{name: "a story stub covering one AC", acs: []string{"ac-1"}, want: "claims ac-1"},
		{name: "a story stub covering two ACs keeps declaration order", acs: []string{"ac-3", "ac-1"}, want: "claims ac-3, ac-1"},
		{name: "a spike stub resolving a question and claiming no AC", resolves: []string{"oq-2"}, want: "resolves oq-2 · claims no AC yet"},
		{name: "a spike stub resolving two questions and claiming an AC", resolves: []string{"oq-1", "oq-2"}, acs: []string{"ac-2"}, want: "resolves oq-1, oq-2 · claims ac-2"},
		{name: "a stub declaring nothing yet", want: "claims no AC yet"},
		{name: "an empty id is not a claim", acs: []string{""}, want: "claims no AC yet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stubMetaText(tc.resolves, tc.acs); got != tc.want {
				t.Errorf("stubMetaText(%q, %q) = %q, want %q", tc.resolves, tc.acs, got, tc.want)
			}
		})
	}
}

// TestStickyFootText pins the sticky's footer line (handoff "Sticky":
// `scratch · graduate or delete`), worded by what the wall offers the
// sticky: graduation needs the domain live, deletion needs authoring, and
// a mirror or a sealed record offers neither.
func TestStickyFootText(t *testing.T) {
	for _, tc := range []struct {
		name       string
		authoring  bool
		domainLive bool
		want       string
	}{
		{name: "authoring with the domain live", authoring: true, domainLive: true, want: "scratch · graduate or delete"},
		{name: "authoring under a domain refusal", authoring: true, want: "scratch · delete"},
		{name: "review or read-only", want: "scratch"},
		{name: "a live domain outside authoring cannot happen and offers nothing", domainLive: true, want: "scratch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stickyFootText(tc.authoring, tc.domainLive); got != tc.want {
				t.Errorf("stickyFootText(%v, %v) = %q, want %q", tc.authoring, tc.domainLive, got, tc.want)
			}
		})
	}
}
