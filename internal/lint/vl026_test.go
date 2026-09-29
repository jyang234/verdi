package lint

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// vl026Clauses returns the sorted clause letters of got's VL-026
// findings, read from each message's "clause (x):" prefix, and fails the
// test on a VL-026 finding that names no clause.
func vl026Clauses(t *testing.T, got []Finding) []string {
	t.Helper()
	var clauses []string
	for _, f := range got {
		if f.Rule != "VL-026" {
			continue
		}
		if !strings.HasPrefix(f.Message, "clause (") || len(f.Message) < len("clause (x)") || f.Message[9] != ')' {
			t.Fatalf("VL-026 finding names no clause: %s", f.String())
		}
		clauses = append(clauses, f.Message[8:9])
	}
	sort.Strings(clauses)
	return clauses
}

func TestVL026_Base_Clean(t *testing.T) {
	findings := runRule(vl026{}, memSnapshot(t, cssBase()...))
	if len(findings) != 0 {
		t.Fatalf("VL-026 fired on the link-free base store:\n%s", findingsString(findings))
	}
}

// vl026ArchiveAccepted is an archive-zone spec whose own frontmatter
// claims a non-closed status. Under SI-277 the archive zone alone makes a
// spec closed for VL-026, so it is closed here; VL-002 separately rejects
// the placement.
func vl026ArchiveAccepted() memDoc {
	return cssSpec("archive", "css-archive-accepted", "feature", "accepted-pending-build", cssObjectsYAML)
}

// TestVL026_ClosedSpec pins SI-277's reading of "closed": the target
// spec's document sits in the archive zone, whatever its status: field
// says. An active-zone spec claiming `status: closed` is not closed for
// VL-026; that store is VL-002's to reject.
func TestVL026_ClosedSpec(t *testing.T) {
	snap := memSnapshot(t, append(cssBase(), vl026ArchiveAccepted())...)
	cases := []struct {
		name string
		ref  string
		want string // the closed spec's id, or "" for none
	}{
		{name: "archive zone, statusless", ref: "spec/css-closed-archive#ac-1", want: "spec/css-closed-archive"},
		{name: "archive zone, status accepted-pending-build", ref: "spec/css-archive-accepted#ac-1", want: "spec/css-archive-accepted"},
		{name: "pinned ref to an archive-zone spec", ref: "spec/css-closed-archive@" + cssSHA + "#ac-1", want: "spec/css-closed-archive"},
		{name: "active zone claiming status: closed", ref: "spec/css-closed-status#ac-1"},
		{name: "active zone, live", ref: "spec/css-live#ac-1"},
		{name: "spec that does not exist", ref: "spec/css-missing#ac-1"},
		{name: "not a spec", ref: "adr/0001-css"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := artifact.ParseRef(tc.ref)
			if err != nil {
				t.Fatalf("ParseRef(%q): %v", tc.ref, err)
			}
			got := ""
			if d := vl026ClosedSpec(snap, ref); d != nil {
				got = d.Base.ID
			}
			if got != tc.want {
				t.Fatalf("vl026ClosedSpec(%q) = %q, want %q", tc.ref, got, tc.want)
			}
		})
	}
}

// TestVL026_ConflictSuperseded pins VL-026's one read of a conflict's own
// status enum (open -> superseded | dismissed, 02 §Kind registry): only
// the exact value superseded counts, as decode's enum does.
func TestVL026_ConflictSuperseded(t *testing.T) {
	cases := []struct {
		status artifact.Status
		want   bool
	}{
		{status: "superseded", want: true},
		{status: "open"},
		{status: "dismissed"},
		{status: "Superseded"},
		{status: ""},
	}
	for _, tc := range cases {
		t.Run(string(tc.status), func(t *testing.T) {
			if got := vl026ConflictSuperseded(&artifact.ConflictFrontmatter{Status: tc.status}); got != tc.want {
				t.Fatalf("vl026ConflictSuperseded(status %q) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// TestVL026_Clauses drives every clause a–g through its happy and negative
// paths. Each case adds its subject documents to cssBase and names the
// exact clause letters VL-026 must report (nil: none), plus substrings
// the findings must carry, so a case passes only when the right clause
// fires for the right reason.
func TestVL026_Clauses(t *testing.T) {
	cases := []struct {
		name    string
		subject []memDoc
		want    []string
		msgs    []string
	}{
		// Clause (a): a feature or component spec's top-level links target
		// no object fragment, of any link type (BL-66).
		{
			name:    "a: feature top-level implements fragment",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", linksYAML("implements", "spec/css-live#ac-1"))},
			want:    []string{"a"},
			msgs:    []string{`"spec/css-live#ac-1"`, "#ac-1", "02 §Link taxonomy", "VL-026"},
		},
		{
			name:    "a: component top-level depends-on fragment",
			subject: []memDoc{cssSpec("active", "css-subject", "component", "", linksYAML("depends-on", "spec/css-live#dc-1"))},
			want:    []string{"a"},
			msgs:    []string{`"spec/css-live#dc-1"`},
		},
		{
			name:    "a: feature top-level pinned fragment",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", linksYAML("depends-on", "spec/css-live@"+cssSHA+"#ac-1"))},
			want:    []string{"a"},
		},
		{
			name:    "a: feature top-level fragment to an undeclared object still names the fragment",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", linksYAML("implements", "spec/css-missing#ac-1"))},
			want:    []string{"a"},
		},
		{
			name:    "a negative: story top-level implements fragment",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("implements", "spec/css-live#ac-1"))},
		},
		{
			name: "a negative: feature top-level whole-artifact, tracker, and external links",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", linksYAML(
				"depends-on", "spec/css-live",
				"story", "jira:LOAN-1",
				"impacts", "svc/loansvc/boundary-contract",
			))},
		},
		{
			name:    "a negative: a feature decision's own fragment link is not top-level",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-live#dc-1"))},
		},

		// Clause (b): no top-level supersedes link, on any artifact,
		// targets an object of a closed spec.
		{
			name:    "b: story top-level supersedes an object of an archive-zone spec",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("supersedes", "spec/css-closed-archive#ac-1"))},
			want:    []string{"b"},
			msgs:    []string{`"spec/css-closed-archive#ac-1"`, "spec/css-closed-archive", "belongs on a decision", "02 §Link taxonomy", "VL-026"},
		},
		{
			name:    "b: top-level supersedes an object of an archive-zone spec whose status is not closed",
			subject: []memDoc{vl026ArchiveAccepted(), cssSpec("active", "css-subject", "story", "", linksYAML("supersedes", "spec/css-archive-accepted#dc-1"))},
			want:    []string{"b"},
		},
		{
			name:    "b negative (SI-277): top-level supersedes an object of an active-zone spec claiming status: closed",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("supersedes", "spec/css-closed-status#dc-1"))},
		},
		{
			name:    "b: ADR top-level supersedes an object of a closed spec",
			subject: []memDoc{cssADR("0002-css", linksYAML("supersedes", "spec/css-closed-archive#dc-1"))},
			want:    []string{"b"},
		},
		{
			name:    "b: pinned top-level supersedes an object of a closed spec",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("supersedes", "spec/css-closed-archive@"+cssSHA+"#ac-1"))},
			want:    []string{"b"},
		},
		{
			name:    "a and b: feature top-level supersedes an object of a closed spec",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", linksYAML("supersedes", "spec/css-closed-archive#ac-1"))},
			want:    []string{"a", "b"},
		},
		{
			name:    "b negative: top-level supersedes an object of a live spec",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("supersedes", "spec/css-live#ac-1"))},
		},
		{
			name:    "b negative: top-level supersedes a whole closed spec",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("supersedes", "spec/css-closed-archive"))},
		},
		{
			name:    "b negative: top-level implements an object of a closed spec",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("implements", "spec/css-closed-archive#ac-1"))},
		},
		{
			name:    "b negative: top-level supersedes an object of a spec that does not exist",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("supersedes", "spec/css-missing#ac-1"))},
		},

		// Clause (c): a decision's supersedes link to an object of a closed
		// spec targets a declared acceptance criterion or decision.
		{
			name:    "c: decision supersedes a closed spec's constraint",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive#co-1"))},
			want:    []string{"c"},
			msgs:    []string{"decisions[dc-9]", `"spec/css-closed-archive#co-1"`, "a constraint", "02 §Link taxonomy", "VL-026"},
		},
		{
			name:    "c: decision supersedes a closed spec's open question",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive#oq-1"))},
			want:    []string{"c"},
			msgs:    []string{"an open question"},
		},
		{
			name:    "c: decision supersedes a closed spec's stub",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive#retry-path"))},
			want:    []string{"c"},
			msgs:    []string{"a stub"},
		},
		{
			name:    "c: decision supersedes an undeclared id of a closed spec",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive#ac-9"))},
			want:    []string{"c"},
			msgs:    []string{"not declared"},
		},
		{
			name:    "c: story decision supersedes a constraint of an archive-zone spec whose status is not closed",
			subject: []memDoc{vl026ArchiveAccepted(), cssSpec("active", "css-subject", "story", "", decisionYAML("supersedes", "spec/css-archive-accepted#co-1"))},
			want:    []string{"c"},
		},
		{
			name:    "c negative (SI-277): decision supersedes a constraint of an active-zone spec claiming status: closed",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", decisionYAML("supersedes", "spec/css-closed-status#co-1"))},
		},
		{
			name:    "c negative: decision supersedes a closed spec's acceptance criterion",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive#ac-1"))},
		},
		{
			name:    "c negative: decision supersedes a closed spec's decision",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", decisionYAML("supersedes", "spec/css-closed-archive#dc-1"))},
		},
		{
			name:    "c negative: decision supersedes a live spec's constraint",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-live#co-1"))},
		},
		{
			name:    "c negative: decision exempts a closed spec's constraint",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("exempts", "spec/css-closed-archive#co-1"))},
		},
		{
			name:    "c negative: decision supersedes an ADR",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "adr/0001-css"))},
		},

		// Clause (d): a conflict's fragment challenges all name one spec.
		{
			name:    "d: fragment challenges name two specs",
			subject: []memDoc{cssConflict("css-subject", "open", "", "spec/css-closed-archive#ac-1", "spec/css-closed-status#dc-1")},
			want:    []string{"d"},
			msgs:    []string{"spec/css-closed-archive", "spec/css-closed-status", "one spec", "02 §Link taxonomy", "VL-026"},
		},
		{
			name:    "d negative: fragment challenges name two objects of one spec",
			subject: []memDoc{cssConflict("css-subject", "open", "", "spec/css-closed-archive#ac-1", "spec/css-closed-archive#dc-1")},
		},
		{
			name:    "d negative: one fragment challenge beside a whole-artifact challenge of another artifact",
			subject: []memDoc{cssConflict("css-subject", "open", "", "spec/css-closed-archive#ac-1", "adr/0001-css")},
		},
		{
			name:    "d negative: whole-artifact challenges of two artifacts",
			subject: []memDoc{cssConflict("css-subject", "open", "", "spec/css-closed-archive", "adr/0001-css")},
		},

		// Clause (e): a superseded conflict whose challenges include an
		// object fragment carries resolved_by naming an existing spec.
		{
			name:    "e: superseded fragment conflict without resolved_by",
			subject: []memDoc{cssConflict("css-subject", "superseded", "", "spec/css-closed-archive#ac-1")},
			want:    []string{"e"},
			// "but no resolved_by" pins the missing-field diagnostic: an
			// absent resolved_by is reported as absent, never as
			// `resolved_by "" does not name a spec`.
			msgs: []string{"but no resolved_by", "§Kind registry", "VL-026"},
		},
		{
			name:    "e: superseded mixed fragment and whole-artifact conflict without resolved_by",
			subject: []memDoc{cssConflict("css-subject", "superseded", "", "spec/css-closed-archive#ac-1", "adr/0001-css")},
			want:    []string{"e"},
			msgs:    []string{"but no resolved_by"},
		},
		{
			name:    "e: resolved_by names a spec that does not exist",
			subject: []memDoc{cssConflict("css-subject", "superseded", "spec/css-missing", "spec/css-closed-archive#ac-1")},
			want:    []string{"e"},
			msgs:    []string{`"spec/css-missing"`},
		},
		{
			name:    "e and f: resolved_by names an existing ADR, not a spec",
			subject: []memDoc{cssConflict("css-subject", "superseded", "adr/0001-css", "spec/css-closed-archive#ac-1")},
			want:    []string{"e", "f"},
		},
		{
			name:    "e negative: resolved_by names an active-zone spec",
			subject: []memDoc{cssConflict("css-subject", "superseded", "spec/css-live", "spec/css-closed-archive#ac-1")},
		},
		{
			name:    "e negative: resolved_by names an archive-zone spec",
			subject: []memDoc{cssConflict("css-subject", "superseded", "spec/css-closed-archive", "spec/css-closed-status#ac-1")},
		},
		{
			name:    "e negative: open fragment conflict without resolved_by",
			subject: []memDoc{cssConflict("css-subject", "open", "", "spec/css-closed-archive#ac-1")},
		},
		{
			name:    "e negative: dismissed fragment conflict without resolved_by",
			subject: []memDoc{cssConflict("css-subject", "dismissed", "", "spec/css-closed-archive#ac-1")},
		},
		{
			name:    "e negative: superseded whole-artifact conflict without resolved_by",
			subject: []memDoc{cssConflict("css-subject", "superseded", "", "adr/0001-css")},
		},

		// Clause (f): every conflict's resolved_by is within SI-269's decode
		// scope (artifact.ConflictFrontmatter.ValidateResolvedBy).
		{
			name:    "f: open conflict carries resolved_by",
			subject: []memDoc{cssConflict("css-subject", "open", "spec/css-live", "spec/css-closed-archive#ac-1")},
			want:    []string{"f"},
			msgs:    []string{"resolved_by", "SI-269", "VL-026"},
		},
		{
			name:    "f: dismissed conflict carries resolved_by",
			subject: []memDoc{cssConflict("css-subject", "dismissed", "spec/css-live", "spec/css-closed-archive#ac-1")},
			want:    []string{"f"},
		},
		{
			name:    "f: superseded whole-artifact conflict carries resolved_by",
			subject: []memDoc{cssConflict("css-subject", "superseded", "spec/css-live", "adr/0001-css")},
			want:    []string{"f"},
		},
		{
			name:    "f: resolved_by is pinned",
			subject: []memDoc{cssConflict("css-subject", "superseded", "spec/css-live@"+cssSHA, "spec/css-closed-archive#ac-1")},
			want:    []string{"f"},
		},
		{
			name:    "f: resolved_by carries a fragment",
			subject: []memDoc{cssConflict("css-subject", "superseded", "spec/css-live#dc-1", "spec/css-closed-archive#ac-1")},
			want:    []string{"f"},
		},

		// Clause (g): a decision's supersedes link to an object of a closed
		// spec is unpinned (SI-271).
		{
			name:    "g: decision supersedes a pinned object of an archive-zone spec",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive@"+cssSHA+"#ac-1"))},
			want:    []string{"g"},
			msgs:    []string{"decisions[dc-9]", cssSHA, "SI-271", "VL-026"},
		},
		{
			name:    "g: decision supersedes a pinned object of an archive-zone spec whose status is not closed",
			subject: []memDoc{vl026ArchiveAccepted(), cssSpec("active", "css-subject", "story", "", decisionYAML("supersedes", "spec/css-archive-accepted@"+cssSHA+"#dc-1"))},
			want:    []string{"g"},
		},
		{
			name:    "g negative (SI-277): decision supersedes a pinned object of an active-zone spec claiming status: closed",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", decisionYAML("supersedes", "spec/css-closed-status@"+cssSHA+"#dc-1"))},
		},
		{
			name:    "c and g: decision supersedes a pinned constraint of a closed spec",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive@"+cssSHA+"#co-1"))},
			want:    []string{"c", "g"},
		},
		{
			name:    "g negative: decision supersedes a pinned object of a live spec",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-live@"+cssSHA+"#ac-1"))},
		},

		// The conflict half of SI-271 is VL-003's (Link.ValidateFor), never
		// duplicated here: a conflict whose only defect is a pinned fragment
		// challenge draws no VL-026 finding (TestVL003_ChallengesFragment
		// proves VL-003 refuses it).
		{
			name:    "g negative: a conflict's pinned fragment challenge is VL-003's",
			subject: []memDoc{cssConflict("css-subject", "open", "", "spec/css-closed-archive@"+cssSHA+"#ac-1")},
		},

		// Every clause at once is reported clause by clause.
		{
			name: "all: one finding per failing clause",
			subject: []memDoc{
				cssSpec("active", "css-subject", "feature", "", linksYAML("supersedes", "spec/css-closed-archive#ac-1")+decisionYAML(
					"supersedes", "spec/css-closed-archive#co-1",
					"supersedes", "spec/css-closed-archive@"+cssSHA+"#ac-1",
				)),
				cssConflict("css-subject-two", "superseded", "", "spec/css-closed-archive#ac-1", "spec/css-closed-status#ac-1"),
				cssConflict("css-subject-three", "open", "spec/css-live", "spec/css-closed-archive#ac-1"),
			},
			want: []string{"a", "b", "c", "d", "e", "f", "g"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := memSnapshot(t, append(cssBase(), tc.subject...)...)
			findings := runRule(vl026{}, snap)
			got := vl026Clauses(t, findings)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("VL-026 clauses = %v, want %v:\n%s", got, want, findingsString(findings))
			}
			for _, f := range findings {
				if f.Rule != "VL-026" {
					t.Errorf("VL-026's Check returned a %s finding: %s", f.Rule, f.String())
				}
				if f.Severity != SeverityViolation {
					t.Errorf("VL-026 finding is not a violation: %s", f.String())
				}
				if !strings.Contains(f.Message, "02 §Link taxonomy") || !strings.HasSuffix(f.Message, "VL-026)") {
					t.Errorf("VL-026 finding does not cite 02 §Link taxonomy and VL-026: %s", f.String())
				}
			}
			for _, m := range tc.msgs {
				found := false
				for _, f := range findings {
					if strings.Contains(f.Message, m) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("no VL-026 finding carries %q:\n%s", m, findingsString(findings))
				}
			}
		})
	}
}

// vl026LocusString renders a finding's self-declared wall placement for
// comparison: "none" (nil, off the wall), "spec" (SpecLocus), or
// "object:<id>" (ObjectLocus).
func vl026LocusString(l *WallLocus) string {
	switch {
	case l == nil:
		return "none"
	case l.Object == "":
		return "spec"
	default:
		return "object:" + l.Object
	}
}

// vl026TwoDecisionsYAML declares two decisions of the subject spec, each
// with one supersedes edge that breaks a different decision clause: dc-7
// supersedes a closed spec's constraint (c), and dc-8 supersedes a pinned
// acceptance criterion of a closed spec (g).
const vl026TwoDecisionsYAML = `decisions:
  - id: dc-7
    text: "replaces the constraint"
    anchor: "#dc-7"
    links:
      - { type: supersedes, ref: "spec/css-closed-archive#co-1" }
  - id: dc-8
    text: "replaces the criterion"
    anchor: "#dc-8"
    links:
      - { type: supersedes, ref: "spec/css-closed-archive@` + cssSHA + `#ac-1" }
`

// TestVL026_Loci pins each clause's self-declared wall placement
// (Finding.Locus; spec/badge-computes dc-3): (c) and (g) concern a
// decision's supersedes edge and badge that decision's card; (a) and (b)
// concern top-level links and badge the case file when the carrier is a
// spec, and declare nothing when it is not; (d), (e), and (f) concern a
// conflict, which has no wall, and declare nothing.
func TestVL026_Loci(t *testing.T) {
	cases := []struct {
		name    string
		subject []memDoc
		want    []string // sorted "clause=locus"
	}{
		{
			name:    "a: feature top-level fragment badges the case file",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", linksYAML("implements", "spec/css-live#ac-1"))},
			want:    []string{"a=spec"},
		},
		{
			name:    "a: component top-level fragment badges the case file",
			subject: []memDoc{cssSpec("active", "css-subject", "component", "", linksYAML("depends-on", "spec/css-live#dc-1"))},
			want:    []string{"a=spec"},
		},
		{
			name:    "b: story spec top-level supersedes badges the case file",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", linksYAML("supersedes", "spec/css-closed-archive#ac-1"))},
			want:    []string{"b=spec"},
		},
		{
			name:    "b: ADR top-level supersedes declares no locus",
			subject: []memDoc{cssADR("0002-css", linksYAML("supersedes", "spec/css-closed-archive#dc-1"))},
			want:    []string{"b=none"},
		},
		{
			name:    "a and b: feature top-level supersedes badges the case file twice",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", linksYAML("supersedes", "spec/css-closed-archive#ac-1"))},
			want:    []string{"a=spec", "b=spec"},
		},
		{
			name:    "c: decision edge badges its decision",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive#co-1"))},
			want:    []string{"c=object:dc-9"},
		},
		{
			name:    "g: pinned decision edge badges its decision",
			subject: []memDoc{cssSpec("active", "css-subject", "story", "", decisionYAML("supersedes", "spec/css-closed-archive@"+cssSHA+"#dc-1"))},
			want:    []string{"g=object:dc-9"},
		},
		{
			name:    "c and g: one decision edge badges its decision for each clause",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", decisionYAML("supersedes", "spec/css-closed-archive@"+cssSHA+"#co-1"))},
			want:    []string{"c=object:dc-9", "g=object:dc-9"},
		},
		{
			name:    "c and g: each decision's edge badges its own decision",
			subject: []memDoc{cssSpec("active", "css-subject", "feature", "", vl026TwoDecisionsYAML)},
			want:    []string{"c=object:dc-7", "g=object:dc-8"},
		},
		{
			name:    "d: conflict declares no locus",
			subject: []memDoc{cssConflict("css-subject", "open", "", "spec/css-closed-archive#ac-1", "spec/css-closed-status#dc-1")},
			want:    []string{"d=none"},
		},
		{
			name:    "e: missing resolved_by declares no locus",
			subject: []memDoc{cssConflict("css-subject", "superseded", "", "spec/css-closed-archive#ac-1")},
			want:    []string{"e=none"},
		},
		{
			name:    "e: resolved_by naming no spec declares no locus",
			subject: []memDoc{cssConflict("css-subject", "superseded", "spec/css-missing", "spec/css-closed-archive#ac-1")},
			want:    []string{"e=none"},
		},
		{
			name:    "f: resolved_by outside decode scope declares no locus",
			subject: []memDoc{cssConflict("css-subject", "open", "spec/css-live", "spec/css-closed-archive#ac-1")},
			want:    []string{"f=none"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRule(vl026{}, memSnapshot(t, append(cssBase(), tc.subject...)...))
			vl026Clauses(t, findings) // fails on a finding that names no clause
			var got []string
			for _, f := range findings {
				got = append(got, f.Message[8:9]+"="+vl026LocusString(f.Locus))
			}
			sort.Strings(got)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("VL-026 clause loci = %v, want %v:\n%s", got, tc.want, findingsString(findings))
			}
		})
	}
}

// TestVL026_FindingPaths proves each finding is filed against the
// document that carries the offending link or field, never the target.
func TestVL026_FindingPaths(t *testing.T) {
	subject := cssSpec("active", "css-subject", "feature", "", linksYAML("implements", "spec/css-live#ac-1"))
	conflict := cssConflict("css-subject", "superseded", "", "spec/css-closed-archive#ac-1")
	findings := runRule(vl026{}, memSnapshot(t, append(cssBase(), subject, conflict)...))
	paths := map[string]bool{}
	for _, f := range findings {
		paths[f.Path] = true
	}
	for _, want := range []string{subject.relPath, conflict.relPath} {
		if !paths[want] {
			t.Errorf("no VL-026 finding filed against %s:\n%s", want, findingsString(findings))
		}
	}
	if len(paths) != 2 {
		t.Errorf("VL-026 findings filed against %d paths, want 2:\n%s", len(paths), findingsString(findings))
	}
}

// TestVL026_SkipsUndecodedDocuments proves a document VL-001 refuses is
// never read by VL-026 (its fields are zero, so reading them would invent
// a verdict).
func TestVL026_SkipsUndecodedDocuments(t *testing.T) {
	snap := memSnapshot(t, cssBase()...)
	snap.Docs = append(snap.Docs, &Document{Kind: "conflict", RelPath: ".verdi/conflicts/broken.md", DecodeErr: fmt.Errorf("does not decode")})
	if findings := runRule(vl026{}, snap); len(findings) != 0 {
		t.Fatalf("VL-026 fired on an undecoded document:\n%s", findingsString(findings))
	}
}

// TestVL026_GrandfatherFlagDoesNotSkip pins that the OQ-3 grandfather
// flag, whose 02 scope is VL-001..VL-006, does not exempt a document from
// VL-026.
func TestVL026_GrandfatherFlagDoesNotSkip(t *testing.T) {
	subject := cssSpec("archive", "css-subject", "feature", "", linksYAML("implements", "spec/css-live#ac-1"))
	snap := memSnapshot(t, append(cssBase(), subject)...)
	for _, d := range snap.Docs {
		if strings.HasPrefix(d.RelPath, ".verdi/specs/archive/") {
			d.Grandfathered = true
		}
	}
	got := vl026Clauses(t, runRule(vl026{}, snap))
	if strings.Join(got, ",") != "a" {
		t.Fatalf("VL-026 clauses on a grandfathered archive feature = %v, want [a]", got)
	}
}

// vl026EngineConflictFrozen is the frozen stamp the committed showcase
// conflict conflict/pii-outbox-leak carries: its commit is a stable
// fixturegit SHA reachable in every buildLintRepo repository, so a
// resolved overlay conflict reusing it passes VL-009.
const vl026EngineConflictFrozen = "frozen: { at: 2026-03-12, commit: 78e3161594fb31fdad17f2ea8a96b52f33dbf0f3 }\n"

// vl026EngineConflict is a conflict for the full-engine tests, challenging
// the archived showcase story spec/refi-rate-check-2024's ac-1.
func vl026EngineConflict(status, extra string) string {
	return "---\nid: conflict/css-engine\nkind: conflict\ntitle: \"closed-spec object supersession engine fixture\"\nstatus: " + status +
		"\nowners: [platform-team]\nlinks:\n  - { type: challenges, ref: \"spec/refi-rate-check-2024#ac-1\" }\n" + extra +
		"---\n# Conflict: closed-spec object supersession engine fixture\n"
}

// TestVL026_Engine_ResolvedFragmentConflict_Clean proves, through the
// real walk, decode, and every registered rule, that a superseded
// conflict challenging an archived spec's acceptance criterion as a
// fragment, resolved by an existing spec, lints fully clean: the amended
// VL-003 accepts the fragment challenge and resolves it, and VL-026's
// shape holds.
func TestVL026_Engine_ResolvedFragmentConflict_Clean(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/conflicts/css-engine.md",
		vl026EngineConflict("superseded", "resolved_by: spec/stale-decline\n"+vl026EngineConflictFrozen))
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0:\n%s", len(findings), findingsString(findings))
	}
}

// TestVL026_Engine_Registered proves VL-026 runs in the engine: the same
// conflict without resolved_by fails clause (e), and nothing else fires.
func TestVL026_Engine_Registered(t *testing.T) {
	dir := adHocOverlayDir(t, ".verdi/conflicts/css-engine.md",
		vl026EngineConflict("superseded", vl026EngineConflictFrozen))
	repo := buildLintRepo(t, dir)
	findings := runLint(t, repo.Dir, Context{}, Options{})
	onlyRule(t, findings, "VL-026")
	if got := vl026Clauses(t, findings); strings.Join(got, ",") != "e" {
		t.Fatalf("VL-026 clauses = %v, want [e]:\n%s", got, findingsString(findings))
	}
	if findings[0].Path != ".verdi/conflicts/css-engine.md" {
		t.Fatalf("finding path = %q, want the conflict's", findings[0].Path)
	}
}
