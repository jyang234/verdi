package workbench

// Server-side rendering for the ASD workbench's Wave 6 additions (design
// §§3.1, 4.2, 6.2): the revision/posture header, the promoted four-area
// shell (the readiness pilot's exact presentation idioms — SI-125 plain
// labels, Step N of 4, exactly-three preview with the exact remaining
// count expanded inline, current-area-first ordering with explicit
// timing/dependency, plain state chips with the formal state retained in
// technical details), the typed-operation forms, and the on-demand
// provenance/semantic-review/context panels. Rendered INSIDE the board
// region so the page, the fragment, the snapshot, and every mutation
// response carry one identical projection.

import (
	stdhtml "html"
	"strconv"
	"strings"
)

// asdShellPreview mirrors the pilot's fixed three-item preview (SI-125).
const asdShellPreview = 3

// asdPlainState maps the formal three-valued state to its plain primary
// label — the pilot's exact participant-ratified words.
func asdPlainState(state string) string {
	switch state {
	case asdStateProven:
		return "Ready"
	case asdStateViolated:
		return "Needs attention"
	default:
		return "Not enough evidence yet"
	}
}

// writeASDState writes one plain state chip; the modifier class keeps the
// formal state machine-readable (e2e asserts chip classes, never label
// text).
func writeASDState(b *strings.Builder, state string) {
	b.WriteString(`<span class="readiness-state readiness-state--` + state + `">` + asdPlainState(state) + `</span>`)
}

// writeASDPosture renders the top bar's posture group (design §4.2;
// spec/chrome-and-tokens-v2 ac-2, dc-2): on a page about one spec the
// mode chip, the review feed's disclosure when the mode was stated
// without it, and the terminal status badge; then on every page the
// branch text and the posture disclosure — its summary the displayed
// bytes (spec pages) and the working tree's state, its panel the full
// posture: checkout, branch, worktree HEAD, accepted branch and HEAD,
// ahead/behind, divergence when both sides carry commits, and the base
// digest where the page has it — and, with refresh, the wall's manual
// Refresh control. It renders from the top bar's facts (specBarFacts,
// branchBarFacts), the one model the bar and the wall's snapshot share
// (SI-323 (3)), so the group is the fragment every refresh replaces.
//
// Each fact's test-id element carries exactly the text today's row showed
// for it, with its state (data-state proven|unproven); a disclosed reason
// today's row did not print sits in a sibling element, so the reason
// stays visible and ac-2's "same text" holds literally (ledger SI-332
// (2)). The summary is the row's own words — "displayed bytes: <word>
// (<state>)" and "working tree: <state>" — with the formal state hidden
// from the eye only when it repeats the word.
func writeASDPosture(b *strings.Builder, f *barFacts, refresh bool) {
	esc := stdhtml.EscapeString
	p := &f.Posture
	spec := f.Spec
	if spec != nil && spec.Unproven != "" {
		spec = nil // disclosed by the bar's class chip; no proven chips here
	}
	b.WriteString(`<section class="topbar-posture-group" id="asd-posture" data-testid="asd-posture" aria-label="Repository posture">`)
	if spec != nil {
		b.WriteString(`<span class="board-mode-tag board-mode-tag--` + esc(spec.Mode) + `">` + esc(spec.ModeLabel) + `</span>`)
		if spec.ModeDisclosure != "" {
			b.WriteString(`<span class="topbar-chip topbar-chip--disclosed" data-testid="topbar-mode-disclosure" title="` + esc(spec.ModeDisclosure) + `">review state unproven<span class="topbar-sr">: ` + esc(spec.ModeDisclosure) + `</span></span>`)
		}
		if spec.StatusBadge != "" {
			b.WriteString(`<span class="badge badge-` + esc(spec.StatusBadge) + ` board-status-badge" data-testid="board-status-badge">` + esc(spec.StatusBadgeLabel) + `</span>`)
		}
	}
	// On the authoring wall the branch text is the switcher (spec/wall-
	// strip-and-drawer-v2 ac-4; SI-368 (7)): the wall alone (refresh), on
	// a proven spec in its authoring room.
	writeTopBarBranch(b, p, refresh && spec != nil && spec.Mode == string(modeAuthoring))
	b.WriteString(`<details class="readiness-tech asd-posture-tech topbar-posture" data-testid="asd-posture-tech"><summary class="topbar-posture-summary" data-testid="topbar-posture">`)
	if spec != nil {
		// What the displayed bytes ARE (design §4.2): the plain word is
		// derived from the formal state alone, never from the mode. The
		// text is today's row's, verbatim; the formal state hides from the
		// eye when it only repeats the word.
		formal := "asd-posture-formal"
		if spec.Bytes.State == spec.Bytes.Word {
			formal += " topbar-sr"
		}
		b.WriteString(`<span class="asd-posture-bytes" data-testid="asd-posture-bytes" data-state="` + esc(spec.Bytes.State) + `">displayed bytes: ` + esc(spec.Bytes.Word) + ` <span class="` + formal + `">(` + esc(spec.Bytes.State) + `)</span></span>`)
		b.WriteString(`<span class="topbar-dot" aria-hidden="true">·</span>`)
	}
	b.WriteString(`<span class="asd-posture-tree" data-testid="asd-posture-tree" data-dirty="` + esc(p.Tree.State) + `">working tree: ` + esc(p.Tree.Text) + `</span>`)
	b.WriteString(`</summary><div class="topbar-posture-panel"><dl class="readiness-tech-facts">`)
	writeBarFact(b, "Checkout", "checkout", p.Checkout)
	writeBarFact(b, "Branch", "branch", p.Branch)
	if p.Detached && p.Branch.Unproven == "" {
		// Today's row printed an empty branch for a detached HEAD; the
		// bar says so beside it.
		writeBarWhy(b, "branch", "detached HEAD (no branch is checked out)", ` data-detached="true"`)
	}
	writeBarFact(b, "Worktree HEAD", "worktree-head", p.WorktreeHead)
	writeBarFact(b, "Accepted branch", "accepted-branch", p.AcceptedBranch)
	writeBarFact(b, "Accepted HEAD", "accepted-head", p.AcceptedHead)
	writeBarFact(b, "Ahead/behind", "ahead-behind", p.AheadBehind)
	if p.Divergence != nil {
		writeBarFact(b, "Divergence", "divergence", *p.Divergence)
	}
	if p.BaseDigest != nil {
		writeBarFact(b, "Base digest", "base-digest", *p.BaseDigest)
	}
	if p.Tree.Unproven != "" {
		// The summary's tree text is today's row's ("working tree:
		// unproven"); the reason is a row of the panel.
		b.WriteString(`<dt>Working tree</dt>`)
		writeBarWhy(b, "tree", p.Tree.Unproven, "")
	}
	b.WriteString(`</dl></div></details>`)
	if refresh {
		b.WriteString(`<button type="button" class="asd-refresh" id="asd-refresh" data-testid="asd-refresh">Refresh</button>`)
	}
	b.WriteString(`</section>`)
}

// writeTopBarBranch writes the bar's branch text — a control of its own
// beside the posture text — in its three states: the branch's name, a
// detached HEAD said so, or unproven with the reason. With switcher, a
// proven branch name is the authoring wall's branch switcher (SI-368
// (7)): the button the wall's script opens the body-level #branch-menu
// from, under the test id the branch-switch guard's evidence locates.
func writeTopBarBranch(b *strings.Builder, p *barPosture, switcher bool) {
	esc := stdhtml.EscapeString
	switch {
	case p.Branch.Unproven != "":
		b.WriteString(`<span class="topbar-branch" data-testid="topbar-branch" data-state="unproven" title="` + esc(p.Branch.Unproven) + `">` + unprovenWord + `<span class="topbar-sr">: ` + esc(p.Branch.Unproven) + `</span></span>`)
	case p.Detached:
		b.WriteString(`<span class="topbar-branch" data-testid="topbar-branch" data-state="proven" data-detached="true">detached HEAD</span>`)
	case switcher:
		b.WriteString(`<span class="topbar-branch" data-testid="topbar-branch" data-state="proven"><button type="button" class="branch-switcher" data-testid="branch-switcher" aria-haspopup="menu" aria-controls="branch-menu" aria-expanded="false">` + esc(p.Branch.Text) + `</button></span>`)
	default:
		b.WriteString(`<span class="topbar-branch" data-testid="topbar-branch" data-state="proven">` + esc(p.Branch.Text) + `</span>`)
	}
}

// writeBarFact writes one row of the full posture: the label, then the
// fact's text exactly as today's row printed it, with its state
// (data-state proven|unproven), addressable by its slug
// (asd-posture-<slug>). A disclosed reason the text does not already
// carry follows as a sibling row (asd-posture-<slug>-why; SI-332 (2)).
func writeBarFact(b *strings.Builder, label, slug string, f barFact) {
	state := "proven"
	if f.Unproven != "" {
		state = unprovenWord
	}
	b.WriteString(`<dt>` + label + `</dt><dd data-testid="asd-posture-` + slug + `" data-state="` + state + `"><code>` + stdhtml.EscapeString(f.Text) + `</code></dd>`)
	if f.Unproven != "" && !strings.Contains(f.Text, f.Unproven) {
		writeBarWhy(b, slug, f.Unproven, "")
	}
}

// writeBarWhy writes a fact's disclosed reason as the row beside its
// test-id element, with any extra attributes.
func writeBarWhy(b *strings.Builder, slug, why, attrs string) {
	b.WriteString(`<dd class="topbar-fact-why" data-testid="asd-posture-` + slug + `-why"` + attrs + `>` + stdhtml.EscapeString(why) + `</dd>`)
}

// asdPostureHTML is the wall's posture group as its own fragment — what
// the wall's snapshot and every mutation response carry beside the
// region (SI-323 (3)), with the wall's Refresh control.
func asdPostureHTML(f *barFacts) string {
	var b strings.Builder
	writeASDPosture(&b, f, true)
	return b.String()
}

// postureByteWord is the posture header's plain word for the displayed
// bytes, keyed on the formal state and nothing else (MVP release amendment
// R2: read-only mode is not a lifecycle verdict). StateFormal arrives in
// two vocabularies — specstate's state ids from the served board's view
// ("proposed", ...) and the legacy artifact status from the remote-ref
// render ("draft", ...) — so both spellings of the proposed state map to
// "proposed". Anything unrecognized, including an absent state, reads as
// unproven: the plain word never claims more than the formal state does.
func postureByteWord(stateFormal string) string {
	switch stateFormal {
	case "proposed", "draft":
		return "proposed"
	case "accepted-pending-build":
		return "accepted"
	case "closed":
		return "closed"
	case "superseded":
		return "superseded"
	default:
		return "unproven"
	}
}

// writeASDShell renders the promoted four-area shell (design §3.1,
// SI-125): orientation (Step N of 4), the process rail, the ranked focus
// queue (exactly three, exact remainder inline), the sequencing
// explainer (F-04), and completed checks — all from the derived shell's
// facts, nothing suppressed or reclassified.
func writeASDShell(b *strings.Builder, asd *asdView) {
	esc := stdhtml.EscapeString
	shell := asd.Shell
	b.WriteString(`<section class="asd-shell readiness-page" id="asd-shell" data-testid="asd-shell" aria-label="Design readiness">`)

	// Orientation: Where am I? What next? (F-01's two anchor questions.)
	b.WriteString(`<div class="readiness-orient asd-orient"><p class="readiness-eyebrow">Where this design stands</p>`)
	b.WriteString(`<p class="readiness-step" data-testid="asd-step">`)
	if shell.CurrentFocus == "" {
		b.WriteString(`All four steps are complete.`)
	} else {
		for i, area := range shell.Areas {
			if area.ID != shell.CurrentFocus {
				continue
			}
			b.WriteString(`Step ` + strconv.Itoa(i+1) + ` of 4 — ` + esc(area.Label))
			break
		}
	}
	b.WriteString(`</p></div>`)

	// The four-step rail (data-state carries the formal state; the chip
	// carries the plain label).
	b.WriteString(`<nav class="readiness-rail" aria-label="Design process rail"><ol class="readiness-rail-list">`)
	for i, area := range shell.Areas {
		focused := shell.CurrentFocus == area.ID
		b.WriteString(`<li class="readiness-station`)
		if focused {
			b.WriteString(` readiness-station--focus`)
		}
		b.WriteString(`" data-area-id="` + esc(string(area.ID)) + `" data-state="` + esc(area.State) + `">`)
		b.WriteString(`<a class="readiness-station-link" href="#asd-focus"`)
		if focused {
			b.WriteString(` aria-current="step"`)
		}
		b.WriteString(`><span class="readiness-station-num">` + strconv.Itoa(i+1) + `</span>`)
		b.WriteString(`<span class="readiness-station-label">` + esc(area.Label) + `</span>`)
		writeASDState(b, area.State)
		b.WriteString(`</a></li>`)
	}
	b.WriteString(`</ol></nav>`)

	// F-04: the fixed order is explained, never left to inference.
	b.WriteString(`<p class="asd-sequence-note" data-testid="asd-sequence-note">The four steps run in this order: define the work, define success, check constraints, then get approval. Later steps stay visible while an earlier step is unresolved, but the current step's items are what move this design forward now.</p>`)

	// Focus queue.
	b.WriteString(`<section class="readiness-queue asd-focus" id="asd-focus" aria-label="Focus next">`)
	b.WriteString(`<h2 class="readiness-heading">Focus next</h2>`)
	if len(shell.Attention) == 0 {
		b.WriteString(`<p class="readiness-queue-empty" data-testid="asd-queue-empty">Nothing needs attention: every check on this wall is proven.</p>`)
	} else {
		b.WriteString(`<p class="readiness-downstream" data-testid="asd-downstream">Known problems in later steps: ` + strconv.Itoa(shell.DownstreamViolated) + `</p>`)
		visible := shell.Attention
		var rest []asdConcern
		if len(visible) > asdShellPreview {
			rest = visible[asdShellPreview:]
			visible = visible[:asdShellPreview]
		}
		b.WriteString(`<ol class="readiness-queue-list">`)
		for i, c := range visible {
			b.WriteString(`<li>`)
			writeASDConcern(b, c, asd, i+1)
			b.WriteString(`</li>`)
		}
		b.WriteString(`</ol>`)
		if len(rest) > 0 {
			b.WriteString(`<details class="readiness-more" data-testid="asd-more"><summary class="readiness-more-summary">`)
			// vocab:identity — "-closed" is this disclosure's CSS class fragment (collapsed label), not the lifecycle state.
			b.WriteString(`<span class="readiness-more-closed">` + esc(asdCountLabel(len(rest), "more item", "more items")) + `</span>`)
			b.WriteString(`<span class="readiness-more-open">Show fewer</span></summary>`)
			b.WriteString(`<ol class="readiness-queue-list readiness-queue-rest" start="` + strconv.Itoa(asdShellPreview+1) + `">`)
			for i, c := range rest {
				b.WriteString(`<li>`)
				writeASDConcern(b, c, asd, asdShellPreview+i+1)
				b.WriteString(`</li>`)
			}
			b.WriteString(`</ol></details>`)
		}
	}
	b.WriteString(`</section>`)

	// Completed checks: every proven fact, lossless.
	b.WriteString(`<section class="readiness-completed asd-completed" aria-label="Completed checks"><h2 class="readiness-heading">Completed checks</h2>`)
	for _, c := range shell.All {
		if c.State != asdStateProven {
			continue
		}
		writeASDConcern(b, c, asd, 0)
	}
	b.WriteString(`</section>`)
	if shell.PolicySetupGuide != policyGuideNone {
		writePolicySetupGuide(b, shell.PolicySetupGuide, shell.PolicyCode, shell.PolicyDetail)
	}
	b.WriteString(`</section>`)
}

// policySetupGuideID is the in-page anchor the context/policy concern's
// destination link targets (boardspecasd.go's policy-forbidden case).
const policySetupGuideID = "asd-policy-guide"

// policySetupCommand is one read-only CLI inspection request the guide
// shows verbatim: the operation, its real operand shape (--request - with
// the JSON on stdin, exactly as cmd/verdi/context_constitution.go accepts
// it), and the result fields to read (internal/constitutionapp's own JSON
// tags). None of these operations adopts, proposes, or approves anything.
type policySetupCommand struct {
	Title   string
	Command string
	Request string
	Read    string
}

// shell renders the command as ONE complete, copyable shell block: the
// request rides a quoted here-document (<<'JSON' … JSON) so pasting it
// never leaves the CLI blocked waiting on stdin, and the delimiter is
// quoted so the shell expands nothing inside the JSON.
func (c policySetupCommand) shell() string {
	return c.Command + " <<'JSON'\n" + c.Request + "\nJSON"
}

var policySetupCommands = []policySetupCommand{
	{"Inspect", "verdi context constitution inspect --request -",
		`{"schema":"verdi.constitution-inspect-request/v1"}`,
		"accepted.adopted and proposed.adopted, each with its reason and exact Git identity."},
	{"Validate", "verdi context constitution validate --request -",
		`{"schema":"verdi.constitution-validate-request/v1"}`,
		"snapshot.adopted and its reason. exit 0 can still mean no policy is adopted."},
	{"Impact review", "verdi context constitution impact-review --request -",
		`{"schema":"verdi.constitution-impact-review-request/v1","targets":[]}`,
		"coverage.state and coverage.reasons. A missing consumer inventory leaves coverage unproven."},
	{"Submission preparation", "verdi context constitution submit-preparation --request -",
		`{"schema":"verdi.constitution-submit-preparation-request/v1","targets":[]}`,
		"ready_for_submission and blocking_reasons. false means preparation is incomplete."},
}

// policyGuidePlaceholderPath escapes one placeholder file path (its
// <profile-id> / <name> segment is angle-bracketed) for use as a
// writeReadinessFact label, which that helper writes as markup verbatim.
// Local to this guide: the helper's contract is unchanged, and every other
// caller passes a bracket-free literal.
func policyGuidePlaceholderPath(path string) string { return stdhtml.EscapeString(path) }

// writePolicySetupGuide renders the inline, read-only policy guide (the
// destination of the policy-forbidden context/policy notice): plain words
// first — what the refusal means and why review is blocked — then the
// expandable technical detail and the read-only CLI checks. kind selects
// the variant the refusal's own discriminant justifies (boardspecasd.go's
// policyGuideKind): the not-adopted variant states the serving-checkout
// fact only, directs inspection of the accepted and proposed snapshots
// first, and points at verdi policy adopt --starter (the starter's files
// listed as a conditional detail); the
// no-design-assistance variant carries the refusal detail verbatim and
// never describes the resolved policy as absent or unaccepted. It is
// markup only: no form, no button, no fetch wiring, no route — it adopts
// nothing, synchronizes nothing, and preserves every mode's restrictions.
// Proposed or validated is never presented as accepted; missing authority
// stays blocked. The "workbench reported" quote is code + ": " + detail
// (wave-1 ledger R-5) — code and detail arrive separately so the prefix
// appears exactly once (ac-5); an absent code degrades to the bare detail.
func writePolicySetupGuide(b *strings.Builder, kind policyGuideKind, code, detail string) {
	esc := stdhtml.EscapeString
	reported := detail
	if code != "" {
		reported = code + ": " + detail
	}
	reportedHTML := `<code data-testid="asd-policy-guide-report">` + esc(reported) + `</code>`
	b.WriteString(`<section class="asd-policy-guide" id="` + policySetupGuideID + `" data-testid="` + policySetupGuideID + `" data-policy-guide="` + esc(string(kind)) + `" aria-label="Policy setup guide">`)
	b.WriteString(`<h2 class="readiness-heading">Policy setup guide</h2>`)
	switch kind {
	case policyGuideNotAdopted:
		// The refusal is resolved on the serving checkout's filesystem
		// (draftmutation.ConstitutionPolicySource over identity.Checkout):
		// it proves no adopted .verdi/policy in THIS checkout's tree and
		// nothing about the default branch, which may already carry
		// accepted policy this branch was cut before.
		b.WriteString(`<p class="readiness-summary">This checkout carries no adopted policy authority: the workbench resolved <code>.verdi/policy</code> in the tree it is serving and found none. The workbench reported: ` + reportedHTML + `. That is a fact about this checkout only, not about the default branch. Ordinary human editing does not require policy; this board&#39;s read-only restrictions still apply. A semantic review packet and delegated-agent design assistance need project policy, so review&#39;s missing-policy refusal remains until this checkout resolves governing policy. Loading or editing proposed policy files is not acceptance and does not by itself authorize review.</p>`)
		// vocab:identity — non-vocabulary homograph: "pull, merge or rebase" names the Git operations the workbench never runs, never the `merge` lifecycle transition word
		b.WriteString(`<p class="readiness-summary">Inspect first: run the read-only Inspect check below and read <code>accepted.adopted</code> against <code>proposed.adopted</code>. If the accepted snapshot is adopted, the project already has policy authority: inspect why this checkout lacks the accepted policy. An older branch may need updating through the project&#39;s own process; the workbench does not pull, merge or rebase anything and infers no cause. Only when the accepted snapshot is also not adopted does initial setup apply.</p>`)
		// vocab:identity — non-vocabulary homograph: the forge's owner's merge to the default branch (the acceptance decision), never the `merge` lifecycle transition word
		b.WriteString(`<p class="ritual-note">Initial setup is one verb: run <code>verdi policy adopt --starter [--profile solo|team]</code> from the project root. It writes a starter constitution, one profile, one policy, and the consumers inventory, and commits exactly those files on a <code>policy/adopt</code> branch. This guide adopts nothing and the workbench has no adoption control; a policy directory that is proposed or validated is not accepted: acceptance is the owner&#39;s merge to the default branch through the project&#39;s own review process.</p>`)

		b.WriteString(`<details class="readiness-tech"><summary>Files the starter writes (or author by hand, only when no policy is accepted)</summary><dl class="readiness-tech-facts">`)
		writeReadinessFact(b, ".verdi/policy/constitution.md", "selects the governance profile and declares the project's role, transition, evidence, subject and adapter catalogs.")
		writeReadinessFact(b, policyGuidePlaceholderPath(".verdi/policy/profiles/<profile-id>.md"), "declares the supported identity trust sources, role mappings and approval requirements. A mapping is not proof that anyone was authenticated or approved a change.")
		writeReadinessFact(b, policyGuidePlaceholderPath(".verdi/policy/policies/<name>.md"), "carries the project's requirements. Overlays, exemptions and dispositions only when actually needed.")
		writeReadinessFact(b, ".verdi/constitution/consumers.json", "declares the real registered consumers impact coverage needs. Never fabricate an empty or baseline inventory to make preparation look complete.")
		b.WriteString(`</dl><p class="ritual-note">Run the verb only after the Inspect check confirms no accepted policy. It leaves you on the policy/adopt branch; open the project&#39;s own review from there. Do not copy a fixture&#39;s identities, approvals or trust facts into a real project. <code>verdi context constitution propose</code> amends one policy, overlay or exemption; <code>verdi policy adopt --starter</code> creates the initial constitution and profile.</p></details>`)
	default:
		b.WriteString(`<p class="readiness-summary">Policy authority resolved for this project, but it does not grant design assistance. The workbench reported: ` + reportedHTML + `. Ordinary human editing does not require policy; this board&#39;s read-only restrictions still apply. A semantic review packet and delegated-agent design assistance need a policy that grants them, so review stays blocked until the project&#39;s policy does.</p>`)
		// vocab:identity — non-vocabulary homograph: the forge's owner's merge to the default branch (the acceptance decision), never the `merge` lifecycle transition word
		b.WriteString(`<p class="ritual-note">This guide changes nothing; the workbench has no policy control. A policy change that is proposed or validated is not accepted: acceptance is the owner&#39;s merge to the default branch through the project&#39;s own review process.</p>`)

		b.WriteString(`<details class="readiness-tech"><summary>What design assistance needs</summary><dl class="readiness-tech-facts">`)
		writeReadinessFact(b, "design_assistance payload", "exactly one policy in the project's effective policy must carry the typed design_assistance payload (its mode selects what delegated agents may do). Without it, the effective policy resolves but grants no design assistance.")
		writeReadinessFact(b, policyGuidePlaceholderPath(".verdi/policy/policies/<name>.md"), "where a project policy's payloads live. Propose the change on a proposal branch through the project's own review; verdi context constitution propose amends one policy, overlay or exemption.")
		b.WriteString(`</dl></details>`)
	}

	b.WriteString(`<details class="readiness-tech"><summary>Read-only checks (CLI, from the project root)</summary>`)
	b.WriteString(`<p class="ritual-note">Each block is one complete command: the request JSON rides a quoted here-document on stdin. Read the returned fields, not just the exit code: a successful command is not readiness, and readiness is not acceptance.</p>`)
	b.WriteString(`<dl class="readiness-tech-facts">`)
	for _, c := range policySetupCommands {
		b.WriteString(`<dt>` + esc(c.Title) + `</dt><dd><pre class="asd-policy-guide-cmd"><code>` + esc(c.shell()) + `</code></pre><p>Read: ` + esc(c.Read) + `</p></dd>`)
	}
	b.WriteString(`</dl><p class="ritual-note">An empty <code>targets</code> list asks for no supplemental previews; it does not waive registered-consumer coverage. Neither <code>adopted: true</code> on the proposed snapshot nor <code>ready_for_submission: true</code> is review approval or acceptance on the default branch.</p></details>`)
	b.WriteString(`<span hidden data-policy-guide-end></span></section>`)
}

// writeASDConcern renders one shell row (spec/spec-documents ac-12,
// R-W4-7): the plain primary line is the source-derived guidance (F-03)
// when the row carries one, otherwise the summary; when both exist the
// fact stays visible as a secondary line, never hidden behind the
// instruction. The plain state chip, the plain human-review label (F-05),
// and the complete technical details follow; explicit timing/dependency
// (F-02) rides the disclosure as a Timing row and data-timing on the
// article — proven rows carry none. No derivation changes here.
func writeASDConcern(b *strings.Builder, c asdConcern, asd *asdView, rank int) {
	esc := stdhtml.EscapeString
	class := "readiness-row"
	if rank > 0 {
		class = "readiness-card"
	}
	// Explicit timing and dependency (F-02): current-step rows say "now";
	// later-step rows name what they wait on. Same condition the former
	// inline stage-line span used.
	timing, timingFact := "", ""
	if asd.Shell.CurrentFocus != "" && c.State != asdStateProven {
		if c.Area == asd.Shell.CurrentFocus {
			timing, timingFact = "now", "now"
		} else if asdAreaAfter(c.Area, asd.Shell.CurrentFocus) {
			timing, timingFact = "later", "later — waits on "+asdAreaLabels[asd.Shell.CurrentFocus]
		}
	}
	b.WriteString(`<article class="` + class + ` readiness-concern--` + esc(c.State) + `" data-concern-id="` + esc(c.ID) + `" data-area-id="` + esc(string(c.Area)) + `"`)
	if timing != "" {
		b.WriteString(` data-timing="` + timing + `"`)
	}
	b.WriteString(`>`)
	if rank > 0 {
		b.WriteString(`<span class="readiness-rank">` + strconv.Itoa(rank) + `</span>`)
	}
	b.WriteString(`<div class="readiness-copy">`)
	b.WriteString(`<p class="readiness-stage">` + esc(asdAreaLabels[c.Area]) + `</p>`)
	if c.HumanReview {
		b.WriteString(`<p class="asd-human-review" data-testid="asd-human-review">Human review</p>`)
	}
	if c.Guidance != "" {
		b.WriteString(`<p class="readiness-summary" data-testid="asd-guidance-` + esc(c.ID) + `">` + esc(c.Guidance) + `</p>`)
	} else {
		b.WriteString(`<p class="readiness-summary" data-testid="asd-summary-` + esc(c.ID) + `">` + esc(c.Summary) + `</p>`)
	}
	writeASDState(b, c.State)
	if c.Guidance != "" {
		b.WriteString(`<p class="asd-fact" data-testid="asd-fact-` + esc(c.ID) + `">` + esc(c.Summary) + `</p>`)
	}
	b.WriteString(`<details class="readiness-tech"><summary>Technical details</summary><dl class="readiness-tech-facts">`)
	writeReadinessFact(b, "State", c.State)
	writeReadinessFact(b, "Concern", c.ID)
	writeReadinessFact(b, "Area", string(c.Area))
	writeReadinessFact(b, "Blocking", strconv.FormatBool(c.Blocking))
	if timingFact != "" {
		writeReadinessFact(b, "Timing", timingFact)
	}
	if len(c.Witnesses) > 0 {
		b.WriteString(`<dt>Witnesses</dt><dd><ul class="readiness-witnesses">`)
		for _, wtn := range c.Witnesses {
			b.WriteString(`<li><code>` + esc(wtn) + `</code></li>`)
		}
		b.WriteString(`</ul></dd>`)
	}
	b.WriteString(`</dl></details>`)
	if c.Dest != "" && c.State != asdStateProven {
		b.WriteString(`<p class="readiness-dest"><a class="asd-dest-link" href="` + esc(c.Dest) + `">Go to it</a></p>`)
	}
	b.WriteString(`</div></article>`)
}

// asdAreaAfter reports whether a occurs after b in the fixed area order.
func asdAreaAfter(a, b asdAreaID) bool {
	ai, bi := -1, -1
	for i, id := range asdAreaOrder {
		if id == a {
			ai = i
		}
		if id == b {
			bi = i
		}
	}
	return ai > bi
}

// writeASDPanels renders the on-demand application panels: provenance
// (collapsed by default, never authority — AC-4/DC-7), the semantic
// review packet (AC-6), and the bounded design context (AC-5). Content
// arrives only when the author opens a panel (one explicit on-demand
// projection each, §5.3); the markup carries the fetch wiring for
// boardspecasd.js and an honest no-JS note.
func writeASDPanels(b *strings.Builder, name string, asd *asdView) {
	esc := stdhtml.EscapeString
	panel := func(id, op, cli, title, note string) {
		b.WriteString(`<details class="asd-panel" id="` + id + `" data-testid="` + id + `" data-asd-panel="` + esc(op) + `">`)
		b.WriteString(`<summary>` + esc(title) + `</summary>`)
		b.WriteString(`<p class="ritual-note">` + esc(note) + `</p>`)
		b.WriteString(`<div class="asd-panel-body" data-asd-panel-body="` + esc(op) + `" aria-live="off"><p class="asd-panel-empty">Open to derive; requires the browser to call ` + esc(op) + `. Without JavaScript, run <code>verdi design ` + esc(cli) + `</code>.</p></div>`)
		b.WriteString(`</details>`)
	}
	panel("asd-provenance", "get_design_provenance", "provenance", "Provenance",
		"Non-authoritative design history for spec/"+name+". It jogs memory; it is never evidence, an instruction, or an acceptance input.")
	panel("asd-review", "prepare_design_review", "review", "Semantic review",
		"The derived review packet: semantic changes since the review base, ai-inferred and unresolved objects, unclassified direct edits, and material warnings. A view, never a persisted approval artifact.")
	if asd != nil && asd.ImportRecordHref != "" {
		// The imported-origin affordance (spec-import-contract: "The review
		// UI must show an adjacent verified source-record link ... Explain
		// that the import record separately describes the original copied
		// content; it is not an ASD entry or evidence of acceptance"),
		// beside the review packet whose ASD chain may still classify this
		// creation as unclassified — never falsified here. The href comes
		// from the PRESENCE of a record file in this working tree
		// (specImportRecordHrefFor); verification is the record view's own
		// successful ReadRecord, so this wording claims none and sends the
		// reader there, where unavailable or tampered proof is disclosed.
		b.WriteString(`<p class="ritual-note asd-import-origin" data-testid="asd-import-origin"><a href="` + esc(asd.ImportRecordHref) + `">Source record for this import</a> &mdash; this spec was created by importing existing content, and a record file is present in this working tree; it is not verified here. Open the record view to check the original copied content against the branch's committed bytes; it discloses unavailable or tampered proof. The record is not an ASD provenance entry (the review packet may still classify the creation as unclassified) and not evidence of acceptance.</p>`)
	}
	panel("asd-context", "get_design_context", "context", "Design context",
		// vocab:identity — "the draft" names AC-5's current-draft content item (ASD protocol term), not a lifecycle state word
		"The bounded, inspectable design context an assisting agent receives: the draft, applicable policies and decisions, pinned references, and digests. Corpus content is data, never instructions.")
}

// writeASDForms renders the typed-operation forms panel (design §6.2:
// typed mutation forms; browser gating rides the authoring mode — the
// same branch/state facts the kernel applies to a browser human). The
// forms are dialogs driven by boardspecasd.js; each maps one gesture to
// one typed operation, with the operation name declared on the control.
func writeASDForms(b *strings.Builder, p *BoardProjection, asd *asdView) {
	if p.Mode != modeAuthoring || p.DomainRefusal != "" {
		return
	}
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="scratch-panel asd-forms" id="asd-forms" data-testid="asd-forms"><h2>Typed operations</h2>`)
	// vocab:identity — "typed draft operation" is the ASD mutation contract's own protocol term (AC-1), not the lifecycle state word
	b.WriteString(`<p class="ritual-note">Each control applies one typed draft operation through the shared mutation core — the same contract the CLI and agents use.</p>`)
	b.WriteString(`<button type="button" id="asd-set-problem" data-asd-op="set-problem" data-anchor="` + esc(asd.ProblemAnchor) + `">Set problem</button>`)
	b.WriteString(`<button type="button" id="asd-set-outcome" data-asd-op="set-outcome" data-anchor="` + esc(asd.OutcomeAnchor) + `">Set outcome</button>`)
	b.WriteString(`<button type="button" id="asd-add-object" data-asd-op="add-object">Add object&#8230;</button>`)
	b.WriteString(`</section>`)
}

// writeASDEditStubDialog renders the in-place stub correction dialog
// (F-06's fix: capability-driven correction of an existing stub through
// the same typed transaction — a slug rename is one atomic
// [remove-stub, add-stub] batch; a binding change is one edit-stub).
//
// The AC/question checkbox lists rendered here are the PAGE-LOAD
// inventory only (the dialog lives outside the snapshot-replaced region):
// boardspecasd.js re-derives them from the fresh region's object cards at
// every open (Codex correction round 1, finding 3), so this markup is the
// no-JS-visible initial state, never the live source of truth.
func writeASDEditStubDialog(b *strings.Builder, p *BoardProjection) {
	esc := stdhtml.EscapeString
	b.WriteString(`<div role="dialog" aria-label="Correct stub" class="board-dialog asd-stub-dialog" id="asd-stub-dialog" hidden>`)
	b.WriteString(`<h2>Correct stub</h2>`)
	b.WriteString(`<p class="ritual-note">Corrects the declared stub in place through one typed transaction. Renaming the slug replaces the stub atomically (remove-stub + add-stub in one batch).</p>`)
	b.WriteString(`<div class="field"><label for="asd-stub-slug">Slug</label><input id="asd-stub-slug" data-testid="asd-stub-slug" autocomplete="off" spellcheck="false">`)
	b.WriteString(`<span class="field-hint">kebab-case (the spec name grammar)</span></div>`)
	b.WriteString(`<p class="asd-field-error" id="asd-stub-slug-error" data-testid="asd-stub-slug-error" role="alert" hidden></p>`)
	spikeWord := p.words.word("spike")
	b.WriteString(`<label class="asd-stub-spike"><input type="checkbox" id="asd-stub-spike"> ` + esc(spikeWord) + `</label>`)
	b.WriteString(`<fieldset class="asd-stub-acs" id="asd-stub-acs"><legend>Covers acceptance criteria</legend>`)
	for _, c := range p.Cards {
		if c.Kind != "acceptance-criterion" {
			continue
		}
		b.WriteString(`<label><input type="checkbox" data-asd-stub-ac="` + esc(c.ID) + `"> ` + esc(c.ID) + `</label>`)
	}
	b.WriteString(`</fieldset>`)
	b.WriteString(`<fieldset class="asd-stub-oqs" id="asd-stub-oqs"><legend>Resolves open questions</legend>`)
	for _, c := range p.Cards {
		if c.Kind != "open-question" {
			continue
		}
		b.WriteString(`<label><input type="checkbox" data-asd-stub-oq="` + esc(c.ID) + `"> ` + esc(c.ID) + `</label>`)
	}
	b.WriteString(`</fieldset>`)
	b.WriteString(`<div class="dialog-actions"><button type="button" id="asd-stub-ok" class="btn-primary" data-testid="asd-stub-ok">Apply</button>`)
	b.WriteString(`<button type="button" id="asd-stub-cancel">Cancel</button></div>`)
	b.WriteString(`</div>`)
}

// writeASDTextDialog renders the shared set-problem/set-outcome/add-object
// dialog shell; boardspecasd.js retargets it per gesture.
func writeASDTextDialog(b *strings.Builder) {
	b.WriteString(`<div role="dialog" aria-label="Typed operation" class="board-dialog asd-op-dialog" id="asd-op-dialog" hidden>`)
	b.WriteString(`<h2 id="asd-op-title">Typed operation</h2>`)
	b.WriteString(`<p class="ritual-note" id="asd-op-note"></p>`)
	b.WriteString(`<div class="field" id="asd-op-kind-field" hidden><label for="asd-op-kind">Kind</label><select id="asd-op-kind">`)
	b.WriteString(`<option value="add-ac">acceptance criterion</option><option value="add-constraint">constraint</option><option value="add-decision">decision</option><option value="add-question">open question</option>`)
	b.WriteString(`</select><span class="field-hint" id="asd-op-id-preview" data-testid="asd-op-id-preview"></span></div>`)
	b.WriteString(`<div class="field"><label for="asd-op-text">Text</label><textarea id="asd-op-text" data-testid="asd-op-text"></textarea></div>`)
	b.WriteString(`<p class="asd-field-error" id="asd-op-error" data-testid="asd-op-error" role="alert" hidden></p>`)
	b.WriteString(`<div class="dialog-actions"><button type="button" id="asd-op-ok" class="btn-primary" data-testid="asd-op-ok">Apply</button>`)
	b.WriteString(`<button type="button" id="asd-op-cancel">Cancel</button></div>`)
	b.WriteString(`</div>`)
}

// writeASDImpactDialog renders the graduation impact preview dialog
// (F-08's fix, adjudication 6): the exact resulting refs, paths,
// relationships, downstream facts, and unknowns — all server-derived
// data composed by boardspecasd.js from data attributes, shown BEFORE the
// durable mutation, never a UI-computed guess.
func writeASDImpactDialog(b *strings.Builder) {
	b.WriteString(`<div role="alertdialog" aria-label="Graduation impact" class="board-dialog asd-impact-dialog" id="asd-impact-dialog" hidden aria-describedby="asd-impact-body">`)
	b.WriteString(`<h2 id="asd-impact-title">Graduation impact</h2>`)
	b.WriteString(`<div id="asd-impact-body" data-testid="asd-impact-body"></div>`)
	b.WriteString(`<p class="asd-field-error" id="asd-impact-error" data-testid="asd-impact-error" role="alert" hidden></p>`)
	b.WriteString(`<div class="dialog-actions"><button type="button" id="asd-impact-ok" class="btn-primary" data-testid="asd-impact-ok">Graduate</button>`)
	b.WriteString(`<button type="button" id="asd-impact-cancel">Cancel</button></div>`)
	b.WriteString(`</div>`)
}
