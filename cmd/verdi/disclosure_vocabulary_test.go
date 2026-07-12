package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/lint"
)

// TestDisclosureVocabulary_TextuallyIdentical is spec/disclosure-seam#ac-1's
// declared behavioral exerciser, written literally against the AC's own
// text: "the three existing disclosure call sites (lint notice, gate
// [NOTICE], mcp/workbench review_unavailable) emit textually identical
// phrasing for equivalent states."
//
// This test is EXPECTED TO FAIL, and is committed failing, deliberately —
// see the story's own commit history and round5-divergences.md's D-9. The
// story's minimal scoping ("rename tokens in place", no new shared type or
// package) achieves a shared LEADING TOKEN across all three call sites
// ("disclosed-unproven" now appears in every one of them — see
// internal/lint/finding.go, cmd/verdi/gate.go's [DISCLOSED-UNPROVEN] tag,
// and cmd/verdi/gate_threads.go's reviewUnavailableReason). It cannot
// achieve textually IDENTICAL phrasing, because the three call sites hold
// structurally different data at their own point of rendering:
//
//   - lint.Finding{Rule, Path, Message} renders ONE line combining all
//     three fields: "disclosed-unproven: VL-xxx <path>: <message>".
//   - gateCondition{Name, Reason} renders a TWO-line bracketed block:
//     "[DISCLOSED-UNPROVEN] <name>\n       <reason>".
//   - review_unavailable is a BARE sentence with no Rule/Path/Name-
//     equivalent fields at all: "disclosed-unproven: forge ... is
//     configured ...".
//
// No string rename can unify these three SHAPES without first unifying
// what data each call site has on hand when it renders — exactly DC-1's
// claim in spec/disclosure-legibility ("the rendered-state shape has to
// exist as a real seam other producers can call into before any one view
// can enumerate through it"). This failure is the story's rung-3 trigger:
// see .verdi/conflicts/disclosure-seam-rename-insufficient.md and
// spec/disclosure-seam-v2, which supersedes this story to build the
// shared internal/disclosure seam the spike (docs/spikes/v1/
// disclosure-enumeration-spike.md) specifies.
func TestDisclosureVocabulary_TextuallyIdentical(t *testing.T) {
	const (
		input = "REQUIRED_INPUT"
		fact  = "is absent, so this check could not run"
	)

	// lint's renderer: a Finding with Severity: SeverityDisclosure, given
	// the equivalent fact via its Message field.
	lintText := lint.Finding{
		Rule:     "VL-000",
		Path:     "spec/example",
		Severity: lint.SeverityDisclosure,
		Message:  fmt.Sprintf("%s %s", input, fact),
	}.String()

	// gate's renderer: a disclosed gateCondition, given the equivalent fact
	// via its Reason field.
	var buf bytes.Buffer
	reportGateConditions(&buf, []gateCondition{{
		Name:      "example condition",
		Disclosed: true,
		Reason:    fmt.Sprintf("%s %s", input, fact),
	}})
	gateText := strings.TrimSpace(buf.String())

	// mcp/workbench's renderer: reviewUnavailableReason is the one function
	// both surfaces already share (gate_threads.go's doc comment: "one
	// message shared by the board chrome and the mcp list_annotations
	// disclosure field") — the closest existing analogue to an equivalent
	// disclosed-unproven fact this call site can express.
	mcpText := reviewUnavailableReason(input)

	if lintText == gateText && gateText == mcpText {
		t.Fatalf("unexpected: rename-in-place achieved textually identical phrasing across all three call sites; ac-1 satisfied by rename alone (this would contradict the story's own recorded finding — re-check D-9)")
	}

	t.Logf("lint: %q", lintText)
	t.Logf("gate: %q", gateText)
	t.Logf("mcp:  %q", mcpText)

	// The AC's literal bar: identical phrasing for an equivalent state.
	// This is the failing assertion — see the doc comment above for why it
	// is expected to fail and is committed failing as the story's rung-3
	// evidence.
	if lintText != gateText || gateText != mcpText {
		t.Fatalf(`spec/disclosure-seam#ac-1 NOT satisfied by rename-in-place: the three call sites do not emit textually identical phrasing for an equivalent disclosed-unproven state.

  lint: %s
  gate: %s
  mcp:  %s

A shared leading token ("disclosed-unproven") was achieved, but the surrounding shape was not: lint combines rule+path+message on one line, the gate renders a two-line name/reason bracketed block, and review_unavailable is a bare sentence with no rule/path/name fields at all. Unifying these requires a shared data shape (a Disclosure struct) and a shared Render function — the seam DC-1 describes and this story's own minimal scoping ("without introducing a new shared type or package") explicitly declined to build. This is the story's rung-3 finding: see .verdi/conflicts/disclosure-seam-rename-insufficient.md and spec/disclosure-seam-v2.`, lintText, gateText, mcpText)
	}
}
