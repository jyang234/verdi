package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/OWNER/verdi/internal/disclosure"
	"github.com/OWNER/verdi/internal/lint"
)

// TestDisclosureSeam_AC1_RenderThroughTheSharedSeam is spec/disclosure-
// seam-v2#ac-1's behavioral exerciser: "the three call sites render
// through the one seam." Each call site's disclosure output must equal
// exactly what disclosure.Render(disclosure.New(...)) produces from that
// call site's own known inputs — proof the text comes from the shared
// seam, not a locally re-authored format string (the earlier
// spec/disclosure-seam attempt's own insufficiency; see
// conflict/disclosure-seam-rename-insufficient). The merge gate's
// reportGateConditions (gate.go) and the closure gate's own condition loop
// (closuregate.go) are both real call sites through the same seam; the
// closure gate's is additionally pinned end-to-end against a real fixture
// by TestRunClosureGate_PendingSupersessionDisclosedUnproven
// (closuregate_test.go).
func TestDisclosureSeam_AC1_RenderThroughTheSharedSeam(t *testing.T) {
	t.Run("lint.Finding", func(t *testing.T) {
		f := lint.Finding{
			Rule: "VL-017", Path: "spec/example",
			Severity: lint.SeverityDisclosure,
			Message:  "example input is absent",
		}
		want := disclosure.Render(disclosure.New("lint:VL-017", "spec/example", "example input is absent"))
		if got := f.String(); got != want {
			t.Fatalf("Finding.String() = %q, want the shared seam's rendering %q", got, want)
		}
	})

	t.Run("gate disclosed condition (merge gate)", func(t *testing.T) {
		var buf bytes.Buffer
		reportGateConditions(&buf, []gateCondition{{
			Name: "example condition", Disclosed: true,
			Source: "gate:example", Reason: "example input is absent",
		}})
		want := disclosure.Render(disclosure.New("gate:example", "", "example input is absent")) + "\ngate: PASS\n"
		if got := buf.String(); got != want {
			t.Fatalf("reportGateConditions output = %q, want the shared seam's rendering %q", got, want)
		}
	})

	t.Run("review_unavailable (mcp/workbench)", func(t *testing.T) {
		got := reviewUnavailableReason("gitlab")
		want := disclosure.Render(disclosure.New("mcp:review-feed", "",
			`forge "gitlab" is configured (verdi.yaml) but no credentials are available to reach it; review state cannot be shown`))
		if got != want {
			t.Fatalf("reviewUnavailableReason() = %q, want the shared seam's rendering %q", got, want)
		}
	})

	t.Run("review_unavailable structured value is the rendered line's own input", func(t *testing.T) {
		// spec/disclosures-panel ac-1: the /disclosures page enumerates
		// reviewUnavailableDisclosure (via workbench.Deps.Disclosures);
		// the board/mcp line renders reviewUnavailableReason. One decision
		// point: the line IS the structured value rendered, so the panel
		// item and the chrome notice can never drift.
		if got, want := reviewUnavailableReason("gitlab"), disclosure.Render(reviewUnavailableDisclosure("gitlab")); got != want {
			t.Fatalf("reviewUnavailableReason() = %q, want Render(reviewUnavailableDisclosure()) = %q", got, want)
		}
	})
}

// TestDisclosureSeam_AC2_EquivalentStatesProduceIdenticalText is
// spec/disclosure-seam-v2#ac-2's behavioral exerciser: "equivalent states
// produce identical text." Given the same underlying source/text fact,
// rendered independently through lint's Finding.String() and gate's
// reportGateConditions, the two call sites' disclosure output is
// byte-identical — the literal bar spec/disclosure-legibility#ac-1 sets,
// now satisfiable because both share one renderer instead of two
// independently hand-aligned string literals (spec/disclosure-seam's own
// rung-3 finding: see conflict/disclosure-seam-rename-insufficient, where
// the equivalent exerciser genuinely failed before this seam existed).
func TestDisclosureSeam_AC2_EquivalentStatesProduceIdenticalText(t *testing.T) {
	const (
		rule = "999"
		text = "the same required input is absent"
	)

	// lint always sources as "lint:"+Rule; give gate the identical source
	// so the two are describing the SAME Disclosure (same source, empty
	// scope, same text) — a genuinely equivalent state, not merely a
	// similar one.
	lintText := lint.Finding{Rule: rule, Severity: lint.SeverityDisclosure, Message: text}.String()

	var buf bytes.Buffer
	reportGateConditions(&buf, []gateCondition{{Disclosed: true, Source: "lint:" + rule, Reason: text}})
	gateLine := strings.SplitN(buf.String(), "\n", 2)[0]

	if lintText != gateLine {
		t.Fatalf("equivalent states did not produce identical text:\n  lint: %q\n  gate: %q", lintText, gateLine)
	}
}
