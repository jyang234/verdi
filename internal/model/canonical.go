package model

// canonicalModel is the Go-literal twin of the embedded canonical.yaml
// (Task 6, embed.go): today's hardcoded operating model, expressed as a
// Model value directly, with no YAML/decode dependency of its own. It
// exists so validate.go's checkFrontier (Task 5) has a canonical shape
// to compare against without this package depending on its own embedded
// asset — Task 6 embeds canonical.yaml separately (via go:embed) and
// proves, by an equality test, that decoding it produces exactly this
// value (TestCanonicalYAMLMatchesGoLiteral, embed_test.go). Keeping both
// avoids a chicken-and-egg dependency between decode-time frontier
// validation and the embedded-default machinery it must hold still
// against; see the plan's Task 5 Step 3 for this split's rationale.
//
// Every field here is load-bearing for two things at once: it must
// describe today's ACTUAL hardcoded model exactly (Task 6's parity
// tests check the states/verbs halves against internal/artifact/
// status.go and cmd/verdi/dispatch.go's own exported facts), and it
// must itself satisfy every kernel validation rule (it round-trips
// through DecodeModel via canonical.yaml in canonical_test.go).
//
// classes: feature and story only. `component` (internal/artifact's
// third real SpecClass) is a DISCLOSED, deliberate omission, not an
// oversight: component specs carry no problem/outcome/acceptance_
// criteria/stubs at all (artifact.SpecFrontmatter.validateComponent:
// "component spec must not carry feature/story-only fields (02: 'no
// object model')"), have no scaffold/template anywhere in the code
// today (grep-verified: no Component-shaped designscaffold function, no
// component.md template file), and live under a DIFFERENT status enum
// (specComponentStatuses: draft/active/superseded) than the
// feature+story lifecycle this model describes. Every Class in this map
// must carry a non-empty Template (a kernel rule, validate.go) — since
// component has no real template to name, including it here would force
// inventing one, which the parity claim ("canonical.yaml expresses
// today's model exactly") forbids. Adding component's own lifecycle and
// class entry is left to whichever phase gives it a real scaffold.
var canonicalModel = Model{
	Schema: modelSchema,
	Classes: map[string]Class{
		"feature": {
			Display:    "Feature",
			Decomposes: "stubs",
			Template:   "feature.md",
		},
		"story": {
			Display:  "Story",
			Parent:   "feature",
			Template: "story.md",
		},
	},
	// lifecycle: one block per class (table C-2), both identical —
	// feature and story share ONE status enum in the code today
	// (internal/artifact/status.go's specFeatureStatuses literally
	// backs both classes' Validate paths), so both blocks describe the
	// same states/terminal/transitions rather than this schema having
	// any way to declare a shared lifecycle once (guide §5.2's own
	// worked example duplicates identically shaped blocks across
	// classes the same way).
	Lifecycle: map[string]Lifecycle{
		"feature": canonicalSpecLifecycle(),
		"story":   canonicalSpecLifecycle(),
	},
}

// canonicalSpecLifecycle is the one lifecycle shared by the feature and
// story classes today: states/terminal mirror internal/artifact/
// status.go's specFeatureStatuses set (draft, accepted-pending-build,
// closed, superseded; terminal: closed, superseded) exactly (parity-
// tested, Task 6). Its two transitions are the only ritual verbs that
// actually flip a spec's status field in cmd/verdi today (grep-verified:
// accept.go's draftStatusLineRe flip, close.go's
// closeAcceptedStatusLineRe flip) — `build start` (buildstart.go) cuts a
// branch without touching status, so it is NOT modeled as a transition,
// and the accepted-pending-build -> superseded flip a predecessor spec
// undergoes when its successor is ACCEPTED (supersede.go,
// supersedePredecessors, called from within the accept ritual on a
// DIFFERENT spec object) stays kernel per the guide's own §8.3
// ("accepting v2 flips v1's status to superseded" — a side effect of
// accept, never its own verb-transition) — so `superseded` is reachable
// here only via its Terminal membership, exactly like the guide's own
// worked epic/task example never models a transition into `superseded`
// either.
func canonicalSpecLifecycle() Lifecycle {
	return Lifecycle{
		States:   []string{"draft", "accepted-pending-build", "closed", "superseded"},
		Terminal: []string{"closed", "superseded"},
		Transitions: []Transition{
			{
				Verb: "accept",
				From: "draft",
				To:   "accepted-pending-build",
				Obligations: []Obligation{
					{Scheme: "attestation", Kind: "author-vouch"},
				},
			},
			{
				Verb: "close",
				From: "accepted-pending-build",
				To:   "closed",
				Obligations: []Obligation{
					{Scheme: "attestation", Kind: "countersign", Count: 1},
					{Scheme: "behavioral", Kind: "fold-green"},
				},
			},
		},
	}
}
