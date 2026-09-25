package artifact

import (
	"fmt"
	"strings"
	"testing"
)

// conflictDoc builds a conflict frontmatter document for the closed-spec
// object supersession tests: status, one `challenges` link per ref, the
// frozen stamp a resolved conflict requires, and any extra lines (such as
// resolved_by).
func conflictDoc(status string, refs []string, extra string) []byte {
	links := make([]Link, len(refs))
	for i, r := range refs {
		links[i] = Link{Type: LinkChallenges, Ref: r}
	}
	return conflictDocWithLinks(status, links, extra)
}

// conflictDocWithLinks is conflictDoc with the `links:` block given link by
// link, so a test can mix `challenges` with other link types.
func conflictDocWithLinks(status string, links []Link, extra string) []byte {
	var b strings.Builder
	b.WriteString("id: conflict/closed-object-replaced\nkind: conflict\ntitle: \"A successor replaces a closed criterion\"\n")
	b.WriteString("status: " + status + "\nowners: [platform-team]\nlinks:\n")
	for _, l := range links {
		fmt.Fprintf(&b, "  - { type: %s, ref: %q }\n", l.Type, l.Ref)
	}
	if status != "open" {
		b.WriteString("frozen: { at: 2026-09-25, commit: 3e91ab2 }\n")
	}
	b.WriteString(extra)
	return []byte(b.String())
}

// TestLink_ValidateFor_Happy: a conflict's `challenges` link may target an
// object fragment of a spec (02 §Link taxonomy, the `challenges` row and the
// paragraph after the closed spec-object edge vocabulary); whole-artifact
// challenges and the closed five-value vocabulary keep working everywhere.
func TestLink_ValidateFor_Happy(t *testing.T) {
	cases := []struct {
		owner Kind
		link  Link
	}{
		{KindConflict, Link{Type: LinkChallenges, Ref: "spec/home-status-glance#ac-1"}},
		{KindConflict, Link{Type: LinkChallenges, Ref: "spec/workbench-legibility#dc-4"}},
		{KindConflict, Link{Type: LinkChallenges, Ref: "spec/stale-decline"}},
		{KindConflict, Link{Type: LinkChallenges, Ref: "adr/0001-old"}},
		{KindConflict, Link{Type: LinkSupersedes, Ref: "spec/loan-update#ac-1"}},
		{KindSpec, Link{Type: LinkImplements, Ref: "spec/loan-update#ac-1"}},
		{KindSpec, Link{Type: LinkChallenges, Ref: "adr/0001-old"}},
		{KindADR, Link{Type: LinkSupersedes, Ref: "adr/0001-old"}},
	}
	for _, tc := range cases {
		t.Run(string(tc.owner)+"/"+string(tc.link.Type)+"/"+tc.link.Ref, func(t *testing.T) {
			if err := tc.link.ValidateFor(tc.owner); err != nil {
				t.Fatalf("ValidateFor(%s, %+v): %v", tc.owner, tc.link, err)
			}
		})
	}
}

// TestLink_ValidateFor_Negative: a fragment `challenges` link fails closed on
// every kind but a conflict, a conflict's fragment challenge must name a spec
// object, and every other non-vocabulary fragment link still fails closed on a
// conflict too.
func TestLink_ValidateFor_Negative(t *testing.T) {
	// ValidateFor's own refusals carry text Link.Validate's never does, so
	// each case below proves which branch refused it.
	const (
		specObject = "but a conflict's fragment challenges name a spec object, spec/<name>#<object-id> (02 §Link taxonomy)"
		pinned     = "targets pinned object fragment"
	)
	cases := []struct {
		owner Kind
		link  Link
		want  string
	}{
		{KindSpec, Link{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindADR, Link{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindDiagram, Link{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindAttestation, Link{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindWaiver, Link{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindReaffirmation, Link{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindObligation, Link{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{Kind("bogus"), Link{Type: LinkChallenges, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindConflict, Link{Type: LinkChallenges, Ref: "adr/0001-old#dc-1"}, specObject},
		{KindConflict, Link{Type: LinkChallenges, Ref: "diagram/loan-flow#ac-1"}, specObject},
		{KindConflict, Link{Type: LinkChallenges, Ref: "spec/loan-update@3e91ab2#ac-1"}, pinned},
		{KindConflict, Link{Type: LinkVerifies, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindConflict, Link{Type: LinkAnnotates, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindConflict, Link{Type: LinkImpacts, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindConflict, Link{Type: LinkDerivedFrom, Ref: "spec/loan-update#ac-1"}, "closed spec-object edge vocabulary"},
		{KindConflict, Link{Type: LinkChallenges, Ref: ""}, "empty ref"},
		{KindConflict, Link{Type: LinkChallenges, Ref: "not-a-ref"}, "missing the '/'"},
		{KindConflict, Link{Type: LinkChallenges, Ref: "spec/loan-update#"}, "trailing '#'"},
		{KindConflict, Link{Type: LinkChallenges, Ref: "spec/loan-update#AC_1"}, "invalid fragment object id"},
		{KindConflict, Link{Type: "bogus", Ref: "spec/loan-update#ac-1"}, "unknown link type"},
	}
	for _, tc := range cases {
		t.Run(string(tc.owner)+"/"+string(tc.link.Type)+"/"+tc.link.Ref, func(t *testing.T) {
			err := tc.link.ValidateFor(tc.owner)
			if err == nil {
				t.Fatalf("ValidateFor(%s, %+v): want error, got nil", tc.owner, tc.link)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("ValidateFor(%s, %+v) = %q, want it to contain %q", tc.owner, tc.link, err, tc.want)
			}
		})
	}
}

// TestConflict_PinnedFragmentChallenge_Refused: per SI-271 a conflict's
// fragment challenge names an unpinned spec object (02 §Common frontmatter:
// refs inside links are unpinned), so a pinned one fails closed naming the
// pin, both through ValidateFor and through the conflict's decode. A pinned
// whole-artifact challenge on a conflict keeps Link.Validate's verdict (the
// control), and so does a pinned fragment on every link type ValidateFor
// does not widen (TestLink_ValidateFor_IsValidateElsewhere).
func TestConflict_PinnedFragmentChallenge_Refused(t *testing.T) {
	const full = "3e91ab2c4d5e6f708192a3b4c5d6e7f801234567"
	cases := []struct{ ref, commit string }{
		{"spec/home-status-glance@3e91ab2#ac-1", "3e91ab2"},
		{"spec/home-status-glance@" + full + "#ac-1", full},
		{"spec/home-status-glance@" + full + "#dc-2", full},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			wants := []string{"pinned object fragment", fmt.Sprintf("pinned at %q", tc.commit), "02 §Common frontmatter", "SI-271"}
			check := func(what string, err error) {
				t.Helper()
				if err == nil {
					t.Fatalf("%s(challenges %q): want error, got nil", what, tc.ref)
				}
				for _, want := range wants {
					if !strings.Contains(err.Error(), want) {
						t.Fatalf("%s(challenges %q) = %q, want it to contain %q", what, tc.ref, err, want)
					}
				}
			}
			check("ValidateFor", Link{Type: LinkChallenges, Ref: tc.ref}.ValidateFor(KindConflict))
			for _, status := range []string{"open", "superseded"} {
				_, err := DecodeConflict(conflictDoc(status, []string{tc.ref}, ""))
				check("DecodeConflict/"+status, err)
			}
		})
	}
	t.Run("control: pinned whole-artifact challenge", func(t *testing.T) {
		l := Link{Type: LinkChallenges, Ref: "spec/home-status-glance@" + full}
		if err := l.Validate(); err != nil {
			t.Fatalf("Validate(%+v): %v; this control assumes Link.Validate accepts a pinned whole-artifact ref", l, err)
		}
		if err := l.ValidateFor(KindConflict); err != nil {
			t.Fatalf("ValidateFor(conflict, %+v): %v, want Link.Validate's nil", l, err)
		}
		if _, err := DecodeConflict(conflictDoc("open", []string{l.Ref}, "")); err != nil {
			t.Fatalf("DecodeConflict(challenges %q): %v", l.Ref, err)
		}
	})
}

// TestLink_ValidateFor_IsValidateElsewhere proves ValidateFor behaves exactly
// like Link.Validate (same verdict, same diagnostic) for every owner kind and
// link except a conflict's fragment `challenges` link — the one context 02
// §Link taxonomy widens — so the closed five-value vocabulary is unchanged in
// every other context.
func TestLink_ValidateFor_IsValidateElsewhere(t *testing.T) {
	owners := []Kind{KindSpec, KindADR, KindDiagram, KindAttestation, KindWaiver, KindConflict, KindReaffirmation, KindObligation, Kind("bogus")}
	var links []Link
	for _, lt := range []LinkType{
		LinkImplements, LinkResolves, LinkSupersedes, LinkExempts, LinkVerifies, LinkDerivedFrom,
		LinkAnnotates, LinkDependsOn, LinkStory, LinkImpacts, LinkChallenges, "bogus",
	} {
		for _, ref := range []string{
			"", "not-a-ref", "spec/foo", "spec/foo@3e91ab2", "adr/0001-old", "spec/loan-update#ac-1",
			"spec/loan-update@3e91ab2#ac-1", "adr/0001-old#dc-1", "spec/loan-update#", "jira:LOAN-1482",
			"svc/loansvc/boundary-contract",
		} {
			links = append(links, Link{Type: lt, Ref: ref})
		}
	}
	errText := func(err error) string {
		if err == nil {
			return "<nil>"
		}
		return err.Error()
	}
	for _, owner := range owners {
		for _, l := range links {
			if owner == KindConflict && l.Type == LinkChallenges {
				if ref, err := ParseRef(l.Ref); err == nil && ref.Fragment() {
					continue // the one widened context, covered by the tables above
				}
			}
			if got, want := errText(l.ValidateFor(owner)), errText(l.Validate()); got != want {
				t.Errorf("ValidateFor(%s, %+v) = %s, Validate() = %s", owner, l, got, want)
			}
		}
	}
}

// TestDecodeConflict_FragmentChallenges_Happy: a conflict filed under 03
// §Challenging closed decisions may name closed-spec objects as fragments,
// and a superseded one may carry resolved_by (02 §Kind registry; SI-269).
// resolved_by's presence is VL-026's, not the decode's.
func TestDecodeConflict_FragmentChallenges_Happy(t *testing.T) {
	cases := []struct {
		name, status   string
		refs           []string
		extra          string
		wantResolvedBy string
	}{
		{"open, filed early", "open", []string{"spec/home-status-glance#ac-1"}, "", ""},
		{"superseded with resolved_by", "superseded", []string{"spec/home-status-glance#ac-1", "spec/home-status-glance#dc-2"}, "resolved_by: spec/home-status-v2\n", "spec/home-status-v2"},
		{"superseded without resolved_by", "superseded", []string{"spec/home-status-glance#ac-1"}, "", ""},
		{"dismissed", "dismissed", []string{"spec/home-status-glance#ac-1"}, "", ""},
		{"mixed fragment and whole-artifact challenges", "superseded", []string{"spec/home-status-glance#ac-1", "spec/home-status-glance"}, "resolved_by: spec/home-status-v2\n", "spec/home-status-v2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fm, err := DecodeConflict(conflictDoc(tc.status, tc.refs, tc.extra))
			if err != nil {
				t.Fatalf("DecodeConflict: %v", err)
			}
			if fm.ResolvedBy != tc.wantResolvedBy {
				t.Fatalf("ResolvedBy = %q, want %q", fm.ResolvedBy, tc.wantResolvedBy)
			}
		})
	}
}

// TestDecodeConflict_FragmentChallenges_Negative: a conflict's fragment
// challenge names an unpinned spec object; any other kind's fragment, a
// pinned one (SI-271), and a malformed one fail closed, each with its own
// diagnostic.
func TestDecodeConflict_FragmentChallenges_Negative(t *testing.T) {
	const specObject = "but a conflict's fragment challenges name a spec object, spec/<name>#<object-id> (02 §Link taxonomy)"
	cases := []struct{ ref, want string }{
		{"adr/0001-old#dc-1", specObject},
		{"diagram/loan-flow#ac-1", specObject},
		{"spec/home-status-glance@3e91ab2#ac-1", "targets pinned object fragment"},
		{"spec/home-status-glance#", "has a trailing '#' with no object id"},
	}
	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			_, err := DecodeConflict(conflictDoc("open", []string{tc.ref}, ""))
			if err == nil {
				t.Fatalf("DecodeConflict(challenges %q): want error, got nil", tc.ref)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DecodeConflict(challenges %q) = %q, want it to contain %q", tc.ref, err, tc.want)
			}
		})
	}
}

// TestDecodeConflict_ResolvedBy_Negative: per SI-269 resolved_by is accepted
// only on a superseded conflict whose challenges name an object fragment,
// and only as an unpinned spec/<name> ref with no fragment. Every other use
// fails closed, citing 02 §Kind registry; each case asserts the text of the
// one ValidateResolvedBy branch that refuses it.
func TestDecodeConflict_ResolvedBy_Negative(t *testing.T) {
	fragment := []string{"spec/home-status-glance#ac-1"}
	const (
		scope = "is accepted only on a conflict whose challenges links name an object fragment, and none of this conflict's do"
		// shape is the parsed-ref branch's text; unparsed is the
		// unparseable-ref branch's, which wraps ParseRef's own error.
		shape    = "must be an unpinned spec/<name> ref with no fragment (02 §Kind registry)"
		unparsed = "must be an unpinned spec/<name> ref (02 §Kind registry): artifact: "
	)
	status := func(s string) string {
		return fmt.Sprintf("is accepted only on a conflict whose status is superseded, not %q", s)
	}
	cases := []struct {
		name, status string
		refs         []string
		resolvedBy   string
		wants        []string
	}{
		{"open conflict", "open", fragment, "spec/home-status-v2", []string{status("open")}},
		{"dismissed conflict", "dismissed", fragment, "spec/home-status-v2", []string{status("dismissed")}},
		{"superseded with only whole-artifact challenges", "superseded", []string{"spec/home-status-glance", "adr/0001-old"}, "spec/home-status-v2", []string{scope}},
		{"pinned ref", "superseded", fragment, "spec/home-status-v2@3e91ab2", []string{shape}},
		{"fragment ref", "superseded", fragment, "spec/home-status-v2#dc-1", []string{shape}},
		{"pinned fragment ref", "superseded", fragment, "spec/home-status-v2@3e91ab2#dc-1", []string{shape}},
		{"adr ref", "superseded", fragment, "adr/0002-outbox-events", []string{shape}},
		{"conflict ref", "superseded", fragment, "conflict/other-conflict", []string{shape}},
		{"external ref", "superseded", fragment, "svc/loansvc/boundary-contract", []string{unparsed, `unknown kind "svc"`}},
		{"tracker ref", "superseded", fragment, "jira:LOAN-1482", []string{unparsed, "missing the '/'"}},
		{"bare name", "superseded", fragment, "home-status-v2", []string{unparsed, "missing the '/'"}},
		{"not kebab-case", "superseded", fragment, "spec/Home_Status", []string{unparsed, "must be kebab-case"}},
		{"unknown kind", "superseded", fragment, "specs/home-status-v2", []string{unparsed, `unknown kind "specs"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeConflict(conflictDoc(tc.status, tc.refs, fmt.Sprintf("resolved_by: %q\n", tc.resolvedBy)))
			if err == nil {
				t.Fatalf("DecodeConflict(resolved_by %q): want error, got nil", tc.resolvedBy)
			}
			for _, want := range append([]string{"resolved_by", "02 §Kind registry"}, tc.wants...) {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("DecodeConflict(resolved_by %q) = %q, want it to contain %q", tc.resolvedBy, err, want)
				}
			}
		})
	}
}

// TestDecodeConflict_ResolvedBy_EmptyIsAbsent pins SI-269's reading that an
// explicitly empty or null resolved_by decodes as absent: it names no
// resolution, so no conflict refuses it, in scope or out. Whether a
// superseded conflict with fragment challenges then lacks one is VL-026's.
func TestDecodeConflict_ResolvedBy_EmptyIsAbsent(t *testing.T) {
	fragment := []string{"spec/home-status-glance#ac-1"}
	whole := []string{"spec/home-status-glance", "adr/0001-old"}
	contexts := []struct {
		name, status string
		refs         []string
	}{
		{"open", "open", fragment},
		{"dismissed", "dismissed", fragment},
		{"superseded, whole-artifact challenges only", "superseded", whole},
		{"superseded, fragment challenges", "superseded", fragment},
	}
	for _, value := range []string{`resolved_by: ""`, `resolved_by: ''`, "resolved_by: null", "resolved_by: ~", "resolved_by:"} {
		for _, c := range contexts {
			t.Run(value+"/"+c.name, func(t *testing.T) {
				fm, err := DecodeConflict(conflictDoc(c.status, c.refs, value+"\n"))
				if err != nil {
					t.Fatalf("DecodeConflict(%s on a %s conflict): %v, want it to decode as absent (SI-269)", value, c.name, err)
				}
				if fm.ResolvedBy != "" {
					t.Fatalf("ResolvedBy = %q, want empty", fm.ResolvedBy)
				}
			})
		}
	}
}

// TestDecodeConflict_ResolvedBy_ScopeCountsOnlyChallenges: SI-269 scopes
// resolved_by to a superseded conflict whose `challenges` name an object
// fragment, so a fragment reached through any other link type does not
// bring resolved_by into scope. Each refused document's control swaps only
// that link's type to `challenges` and decodes.
func TestDecodeConflict_ResolvedBy_ScopeCountsOnlyChallenges(t *testing.T) {
	const (
		whole    = "spec/home-status-glance"
		fragment = "spec/home-status-glance#ac-1"
		extra    = "resolved_by: spec/home-status-v2\n"
		scope    = "is accepted only on a conflict whose challenges links name an object fragment, and none of this conflict's do"
	)
	for _, lt := range []LinkType{LinkDependsOn, LinkSupersedes, LinkImplements} {
		t.Run(string(lt), func(t *testing.T) {
			ctrl := []Link{{Type: LinkChallenges, Ref: whole}, {Type: LinkChallenges, Ref: fragment}}
			if _, err := DecodeConflict(conflictDocWithLinks("superseded", ctrl, extra)); err != nil {
				t.Fatalf("control decode (fragment through challenges): %v", err)
			}
			refused := []Link{{Type: LinkChallenges, Ref: whole}, {Type: lt, Ref: fragment}}
			_, err := DecodeConflict(conflictDocWithLinks("superseded", refused, extra))
			if err == nil {
				t.Fatalf("DecodeConflict(fragment only through %s, resolved_by): want error, got nil", lt)
			}
			if !strings.Contains(err.Error(), scope) {
				t.Fatalf("DecodeConflict(fragment only through %s, resolved_by) = %q, want it to contain %q", lt, err, scope)
			}
		})
	}
}

// TestDecodeConflict_ResolvedBy_StrictDecodeOnly: lint decodes through
// DecodeStrict alone, which knows the field on a conflict.
func TestDecodeConflict_ResolvedBy_StrictDecodeOnly(t *testing.T) {
	var fm ConflictFrontmatter
	doc := conflictDoc("superseded", []string{"spec/home-status-glance#ac-1"}, "resolved_by: spec/home-status-v2\n")
	if err := DecodeStrict(doc, &fm); err != nil {
		t.Fatalf("DecodeStrict: %v", err)
	}
	if fm.ResolvedBy != "spec/home-status-v2" {
		t.Fatalf("ResolvedBy = %q, want spec/home-status-v2", fm.ResolvedBy)
	}
}

// storyWithTopLevelLink adds l after the story fixture's one top-level link
// (its required implements edge).
func storyWithTopLevelLink(t *testing.T, l string) []byte {
	t.Helper()
	const implements = "  - { type: implements, ref: \"spec/loan-update#ac-1\" }\n"
	if strings.Count(storySpecYAML, implements) != 1 {
		t.Fatal("storySpecYAML no longer carries the one top-level implements link this test extends")
	}
	return []byte(strings.Replace(storySpecYAML, implements, implements+l, 1))
}

// TestFragmentChallenges_RefusedOutsideConflict: only a conflict's `links:`
// may carry a fragment `challenges` edge; a feature's or story's top-level
// links, a decision object's links, and an ADR's links still fail closed with
// the closed-vocabulary diagnostic. Each case has a control proving the same
// document decodes with a whole-artifact challenge or a vocabulary edge.
func TestFragmentChallenges_RefusedOutsideConflict(t *testing.T) {
	const fragment = "  - { type: challenges, ref: \"spec/home-status-glance#ac-1\" }\n"
	const whole = "  - { type: challenges, ref: \"spec/home-status-glance\" }\n"
	decisionWith := func(linkType string) []byte {
		return []byte(featureSpecDraftYAML + "decisions:\n  - { id: dc-1, text: \"replace ac-1\", anchor: \"#dc-1\", links: [ { type: " + linkType + ", ref: \"spec/home-status-glance#ac-1\" } ] }\n")
	}
	cases := []struct {
		name          string
		decode        func([]byte) error
		refused, ctrl []byte
	}{
		{"feature top-level", decodeSpecErr, []byte(featureSpecDraftYAML + "links:\n" + fragment), []byte(featureSpecDraftYAML + "links:\n" + whole)},
		{"story top-level", decodeSpecErr, storyWithTopLevelLink(t, fragment), storyWithTopLevelLink(t, whole)},
		{"feature decision", decodeSpecErr, decisionWith("challenges"), decisionWith("supersedes")},
		{"adr", decodeADRErr, []byte(adrProposedYAML + "links:\n" + fragment), []byte(adrProposedYAML + "links:\n" + whole)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.decode(tc.ctrl); err != nil {
				t.Fatalf("control decode: %v", err)
			}
			err := tc.decode(tc.refused)
			if err == nil {
				t.Fatal("fragment challenges outside a conflict: want error, got nil")
			}
			if !strings.Contains(err.Error(), "closed spec-object edge vocabulary") {
				t.Fatalf("error %q does not cite the closed spec-object edge vocabulary", err)
			}
		})
	}
}

// TestResolvedBy_OnlyOnConflict: resolved_by is a conflict field; strict
// decode (KnownFields) refuses it on a spec or an ADR. Each case has a
// control proving the same document decodes without it.
func TestResolvedBy_OnlyOnConflict(t *testing.T) {
	const line = "resolved_by: spec/home-status-v2\n"
	cases := []struct {
		name   string
		decode func([]byte) error
		doc    string
	}{
		{"feature spec", decodeSpecErr, featureSpecDraftYAML},
		{"story spec", decodeSpecErr, storySpecYAML},
		{"component spec", decodeSpecErr, componentSpecActiveYAML},
		{"proposed adr", decodeADRErr, adrProposedYAML},
		{"accepted adr", decodeADRErr, adrAcceptedYAML},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.decode([]byte(tc.doc)); err != nil {
				t.Fatalf("control decode: %v", err)
			}
			err := tc.decode([]byte(tc.doc + line))
			if err == nil {
				t.Fatal("resolved_by outside a conflict: want error, got nil")
			}
			if !strings.Contains(err.Error(), "field resolved_by not found") {
				t.Fatalf("error %q is not strict decode's unknown-field refusal of resolved_by", err)
			}
		})
	}
}

func decodeSpecErr(b []byte) error { _, err := DecodeSpec(b); return err }

func decodeADRErr(b []byte) error { _, err := DecodeADR(b); return err }
