package workbench

import (
	"strings"
	"testing"
)

// The wall canvas's card markup for spec/wall-canvas-v2 ac-1 (lane F2a;
// ledger SI-350 (3), (8)): the stub's slug stays the
// `<span class="stub-tab">` the Go pins and specs 30-32 hook, byte for
// byte, and is the card's first child; the stub's pin anchors its
// coverage yarn and is no handle; the stub carries its meta line and the
// sticky its footer. The page's status pill host and the selection asset
// are wallselectasset_test.go's.

// TestWallCanvas_StubSlugFirstThenPinAndMeta: the slug span is the stub
// card's first child, unchanged; the pin follows it as a span that is
// neither a button nor a `.yarn-handle` (dc-3: an anchor, not a handle);
// the meta line projects data-acs / data-resolves.
func TestWallCanvas_StubSlugFirstThenPinAndMeta(t *testing.T) {
	for _, mode := range []boardModeKind{modeAuthoring, modeReview, modeReadOnly} {
		p := scopingRenderProjection(t, mode)
		body := renderBoardRegion(p, &boardGitState{}, testASDView())
		for _, want := range []string{
			`data-resolves="" style="left:952px;top:40px"><span class="stub-tab">plain-one</span><span class="stub-pushpin" aria-hidden="true"></span><span class="card-kind">`,
			`<p class="stub-meta" data-testid="stub-meta-plain-one">claims ac-1</p>`,
			`<span class="stub-tab">spike-one</span><span class="stub-pushpin" aria-hidden="true"></span>`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: stub card markup missing %q", mode, want)
			}
		}
		// The spike stub's meta line names what it resolves.
		if !strings.Contains(body, `<p class="stub-meta" data-testid="stub-meta-spike-one">resolves `) {
			t.Errorf("%s: spike stub meta line missing or not a resolves line", mode)
		}
		// The pin is an anchor, never a handle: no button, no handle class,
		// no handle testid on any stub card.
		stubs := stubCardsOf(body)
		if len(stubs) != 3 {
			t.Fatalf("%s: sliced %d stub cards, want 3", mode, len(stubs))
		}
		for _, stub := range stubs {
			pin := sliceBetween(stub, `<span class="stub-pushpin"`, `</span>`)
			if pin == "" {
				t.Errorf("%s: a stub card renders no pin:\n%s", mode, stub)
				continue
			}
			for _, forbidden := range []string{"yarn-handle", "<button", "data-testid=\"yarn-handle-"} {
				if strings.Contains(pin, forbidden) {
					t.Errorf("%s: the stub pin carries %q, a handle's mark:\n%s", mode, forbidden, pin)
				}
			}
			if strings.Count(stub, `class="yarn-handle`) != 0 {
				t.Errorf("%s: a stub card carries a yarn handle:\n%s", mode, stub)
			}
		}
	}
}

// TestWallCanvas_StickyFooterPerMode: every scratch sticky carries the
// footer line, worded by what the wall offers it.
func TestWallCanvas_StickyFooterPerMode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		mode    boardModeKind
		refusal string
		want    string
	}{
		{name: "authoring, domain live", mode: modeAuthoring, want: `<span class="sticky-foot">scratch · graduate or delete</span></div>`},
		{name: "authoring under a domain refusal", mode: modeAuthoring, refusal: "wrong branch", want: `<span class="sticky-foot">scratch · delete</span></div>`},
		{name: "review", mode: modeReview, want: `<span class="sticky-foot">scratch</span></div>`},
		{name: "read-only", mode: modeReadOnly, want: `<span class="sticky-foot">scratch</span></div>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj := &BoardProjection{
				Spec: "s", Mode: tc.mode, DomainRefusal: tc.refusal,
				Cards:    []cardView{{ID: "dc-1", Kind: "decision", Text: "x"}},
				Stickies: []scratchStickyView{{ID: "a-01J8Z0K3AAAAAAAAAAAAAAAAAA", Type: "comment", Body: "b", Author: "nadia"}},
			}
			body := renderBoardRegion(proj, &boardGitState{}, testASDView())
			sticky := sliceBetween(body, `<div class="sticky sticky--comment"`, `</div>`)
			if !strings.HasSuffix(sticky, tc.want) {
				t.Errorf("sticky does not end with its footer %q:\n%s", tc.want, sticky)
			}
			if strings.Count(body, `class="sticky-foot"`) != 1 {
				t.Errorf("want exactly one sticky footer, got %d", strings.Count(body, `class="sticky-foot"`))
			}
		})
	}
}

// stubCardsOf slices every stub card's run out of a rendered region: from
// its opening tag to the next paper or chip the canvas writes after it
// (stickies and yarn chips follow the stubs), or the canvas's end.
func stubCardsOf(body string) []string {
	var out []string
	rest := body
	for {
		at := strings.Index(rest, `<div class="stubcard`)
		if at < 0 {
			return out
		}
		rest = rest[at+1:]
		end := len(rest)
		for _, next := range []string{`<div class="stubcard`, `<div class="sticky`, `<div class="yarn-chip`, `</div><aside`} {
			if i := strings.Index(rest, next); i >= 0 && i < end {
				end = i
			}
		}
		out = append(out, "<"+rest[:end])
		rest = rest[end:]
	}
}

// sliceBetween returns the run of s from the first open through the first
// close that follows it, both included, or "" when either is absent.
func sliceBetween(s, open, close string) string {
	at := strings.Index(s, open)
	if at < 0 {
		return ""
	}
	end := strings.Index(s[at:], close)
	if end < 0 {
		return ""
	}
	return s[at : at+end+len(close)]
}
