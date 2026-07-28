package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/forge"
	forgefake "github.com/jyang234/verdi/internal/forge/fake"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/workbench"
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

	t.Run("review_unavailable transport failure (workbench board)", func(t *testing.T) {
		root := newSeamReviewFixture(t)
		_, notice, err := workbench.LoadProjection(context.Background(), root, seamReviewSpecName, erroringSeamFeed{}, "", nil)
		if err != nil {
			t.Fatalf("LoadProjection: %v", err)
		}
		want := disclosure.Render(disclosure.ReviewUnavailableTransport(errSeamTransport))
		if notice != want {
			t.Fatalf("board review notice = %q, want the shared seam's rendering %q", notice, want)
		}
	})

	t.Run("review_unavailable transport failure (mcp list_annotations)", func(t *testing.T) {
		root := newSeamReviewFixture(t)
		t.Setenv("CI_DEFAULT_BRANCH", "main")
		b := &mcpserve.Backend{Root: root, Forge: erroringSeamForge{forgefake.New()}}
		got := listAnnotationsReviewUnavailable(t, b)
		want := disclosure.Render(disclosure.ReviewUnavailableTransport(errSeamTransport))
		if got != want {
			t.Fatalf("list_annotations review_unavailable = %q, want the shared seam's rendering %q", got, want)
		}
	})
}

// TestDisclosureSeam_AC2_TransportFailureIdenticalAcrossSurfaces is
// spec/disclosure-seam-v2#ac-2 applied to the review-feed call site's
// SECOND state. The startup-time configured-but-no-credentials state was
// already shared (one reviewUnavailableReason feeding both surfaces); the
// render-time transport failure was not — the board hand-authored "review
// feed unavailable: <err>" while list_annotations hand-authored "review
// population unavailable: <err>", so one underlying state read in two
// vocabularies. Both now construct disclosure.ReviewUnavailableTransport
// at their existing decision point, so an equivalent state is
// byte-identical across the two surfaces by construction.
func TestDisclosureSeam_AC2_TransportFailureIdenticalAcrossSurfaces(t *testing.T) {
	root := newSeamReviewFixture(t)
	t.Setenv("CI_DEFAULT_BRANCH", "main")

	_, boardNotice, err := workbench.LoadProjection(context.Background(), root, seamReviewSpecName, erroringSeamFeed{}, "", nil)
	if err != nil {
		t.Fatalf("LoadProjection: %v", err)
	}
	mcpNotice := listAnnotationsReviewUnavailable(t, &mcpserve.Backend{Root: root, Forge: erroringSeamForge{forgefake.New()}})

	if boardNotice != mcpNotice {
		t.Fatalf("equivalent transport-failure states did not produce identical text:\n  board: %q\n  mcp:   %q", boardNotice, mcpNotice)
	}
}

// errSeamTransport is the single forge failure both surfaces' doubles
// report, so the two renderings are describing genuinely the same state.
var errSeamTransport = errors.New("forge: simulated transport failure")

const seamReviewSpecName = "loan-update"

const seamReviewSpecMD = `---
id: spec/loan-update
kind: spec
class: feature
title: "Loan update"
status: draft
owners: [platform-team]
story: jira:LOAN-1482
acceptance_criteria:
  - { id: ac-2, text: "a borrower can see the change reflected", evidence: [static] }
---
# body
`

// newSeamReviewFixture builds one hermetic fixture both review surfaces
// accept: a draft spec declaring an object id, checked out on its design
// branch (review population only ever applies there).
func newSeamReviewFixture(t *testing.T) string {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/specs/active/" + seamReviewSpecName + "/spec.md": seamReviewSpecMD},
		Message: "draft spec",
	}})
	if err := gitx.CheckoutNewBranch(context.Background(), repo.Dir, "design/"+seamReviewSpecName); err != nil {
		t.Fatalf("CheckoutNewBranch: %v", err)
	}
	return repo.Dir
}

// erroringSeamFeed is a workbench.CommentFeed whose call always fails —
// the board's render-time transport failure.
type erroringSeamFeed struct{}

func (erroringSeamFeed) ListMRComments(context.Context, string) ([]workbench.MRComment, bool, error) {
	return nil, false, errSeamTransport
}

// erroringSeamForge is a hermetic forge whose MR discovery fails — the
// mcp surface's render-time transport failure. It reports the SAME error
// as erroringSeamFeed so the two surfaces describe one equivalent state.
type erroringSeamForge struct{ *forgefake.Forge }

func (erroringSeamForge) ListOpenMRs(context.Context, string) ([]forge.OpenMR, error) {
	return nil, errSeamTransport
}

// listAnnotationsReviewUnavailable drives the real list_annotations tool
// and returns its review_unavailable disclosure field.
func listAnnotationsReviewUnavailable(t *testing.T, b *mcpserve.Backend) string {
	t.Helper()
	args, err := json.Marshal(map[string]string{"ref": "spec/" + seamReviewSpecName})
	if err != nil {
		t.Fatal(err)
	}
	result := b.ListAnnotations(context.Background(), args)
	if isErr, _ := result["isError"].(bool); isErr {
		t.Fatalf("list_annotations returned an error result: %#v", result)
	}
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("list_annotations result has no content: %#v", result)
	}
	text, _ := content[0]["text"].(string)
	var decoded struct {
		ReviewUnavailable string `json:"review_unavailable"`
	}
	if err := json.Unmarshal([]byte(text), &decoded); err != nil {
		t.Fatalf("decoding list_annotations result %q: %v", text, err)
	}
	return decoded.ReviewUnavailable
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
