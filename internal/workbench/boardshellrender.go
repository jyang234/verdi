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
	"fmt"
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

// writeASDPosture renders the revision/posture header (design §4.2):
// checkout, branch, worktree HEAD, accepted HEAD, clean/dirty,
// ahead/behind when resolvable, and whether the displayed bytes are
// proposed or accepted — with the mode stamp and terminal status badge
// riding here so every posture fact refreshes with the projection.
func writeASDPosture(b *strings.Builder, p *BoardProjection, git *boardGitState, asd *asdView) {
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="asd-posture" id="asd-posture" data-testid="asd-posture" aria-label="Repository posture">`)
	b.WriteString(`<span class="board-mode-tag board-mode-tag--` + esc(string(p.Mode)) + `">` + esc(modeStampLabel(p)) + `</span>`)
	if badge := terminalStatusBadge(p.Status); badge != "" {
		label := badge
		if p.StatusLabel != "" {
			label = p.StatusLabel
		}
		b.WriteString(`<span class="badge badge-` + esc(badge) + ` board-status-badge" data-testid="board-status-badge">` + esc(label) + `</span>`)
	}
	// What the displayed bytes ARE (design §4.2): the plain word is derived
	// from the formal state alone, never from the mode.
	b.WriteString(`<span class="asd-posture-bytes" data-testid="asd-posture-bytes" data-state="` + esc(asd.StateFormal) + `">displayed bytes: ` + esc(postureByteWord(asd.StateFormal)) + ` <span class="asd-posture-formal">(` + esc(asd.StateFormal) + `)</span></span>`)
	dirtyWord, dirtyState := "clean", "clean"
	if asd.Dirty {
		dirtyWord, dirtyState = "uncommitted changes", "dirty"
	}
	b.WriteString(`<span class="asd-posture-tree" data-testid="asd-posture-tree" data-dirty="` + dirtyState + `">working tree: ` + dirtyWord + `</span>`)
	b.WriteString(`<button type="button" class="asd-refresh" id="asd-refresh" data-testid="asd-refresh">Refresh</button>`)
	b.WriteString(`<details class="readiness-tech asd-posture-tech" data-testid="asd-posture-tech"><summary>Repository details</summary><dl class="readiness-tech-facts">`)
	writeReadinessFact(b, "Checkout", asd.Checkout)
	writeReadinessFact(b, "Branch", asd.Branch)
	writeReadinessFact(b, "Worktree HEAD", orUnproven(asd.WorktreeHead))
	writeReadinessFact(b, "Accepted branch", orUnproven(asd.DefaultBranch))
	writeReadinessFact(b, "Accepted HEAD", orUnproven(asd.AcceptedHead))
	if asd.AheadBehindKnown {
		writeReadinessFact(b, "Ahead/behind", fmt.Sprintf("%d ahead, %d behind %s", asd.Ahead, asd.Behind, asd.DefaultBranch))
		if asd.Ahead > 0 && asd.Behind > 0 {
			writeReadinessFact(b, "Divergence", "diverged: both sides carry commits the other lacks")
		}
	} else {
		writeReadinessFact(b, "Ahead/behind", "unproven: the accepted branch could not be resolved")
	}
	writeReadinessFact(b, "Base digest", asd.BaseDigest)
	b.WriteString(`</dl></details>`)
	b.WriteString(`</section>`)
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

func orUnproven(v string) string {
	if v == "" {
		return "unproven"
	}
	return v
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
		writePolicySetupGuide(b, shell.PolicySetupGuide, shell.PolicyDetail)
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
// first, and names the manual initial files as a conditional detail; the
// no-design-assistance variant carries the refusal detail verbatim and
// never describes the resolved policy as absent or unaccepted. It is
// markup only: no form, no button, no fetch wiring, no route — it adopts
// nothing, synchronizes nothing, and preserves every mode's restrictions.
// Proposed or validated is never presented as accepted; missing authority
// stays blocked.
func writePolicySetupGuide(b *strings.Builder, kind policyGuideKind, detail string) {
	esc := stdhtml.EscapeString
	b.WriteString(`<section class="asd-policy-guide" id="` + policySetupGuideID + `" data-testid="` + policySetupGuideID + `" data-policy-guide="` + esc(string(kind)) + `" aria-label="Policy setup guide">`)
	b.WriteString(`<h2 class="readiness-heading">Policy setup guide</h2>`)
	switch kind {
	case policyGuideNotAdopted:
		// The refusal is resolved on the serving checkout's filesystem
		// (draftmutation.ConstitutionPolicySource over identity.Checkout):
		// it proves no adopted .verdi/policy in THIS checkout's tree and
		// nothing about the default branch, which may already carry
		// accepted policy this branch was cut before.
		b.WriteString(`<p class="readiness-summary">This checkout carries no adopted policy authority: the workbench resolved <code>.verdi/policy</code> in the tree it is serving and found none. The workbench reported: <code>` + esc(detail) + `</code>. That is a fact about this checkout only, not about the default branch. Ordinary human editing does not require policy; this board&#39;s read-only restrictions still apply. A semantic review packet and delegated-agent design assistance need project policy, so review&#39;s missing-policy refusal remains until this checkout resolves governing policy. Loading or editing proposed policy files is not acceptance and does not by itself authorize review.</p>`)
		// vocab:identity — non-vocabulary homograph: "pull, merge or rebase" names the Git operations the workbench never runs, never the `merge` lifecycle transition word
		b.WriteString(`<p class="readiness-summary">Inspect first: run the read-only Inspect check below and read <code>accepted.adopted</code> against <code>proposed.adopted</code>. If the accepted snapshot is adopted, the project already has policy authority: inspect why this checkout lacks the accepted policy. An older branch may need updating through the project&#39;s own process; the workbench does not pull, merge or rebase anything and infers no cause. Only when the accepted snapshot is also not adopted does initial setup apply.</p>`)
		// vocab:identity — non-vocabulary homograph: the forge's owner's merge to the default branch (the acceptance decision), never the `merge` lifecycle transition word
		b.WriteString(`<p class="ritual-note">Initial setup is manual and reviewed. This guide adopts nothing; the workbench has no setup wizard and no adoption control. A policy directory that is proposed or validated is not accepted: acceptance is the owner&#39;s merge to the default branch through the project&#39;s own review process.</p>`)

		b.WriteString(`<details class="readiness-tech"><summary>Files to author (manual initial setup, only when no policy is accepted)</summary><dl class="readiness-tech-facts">`)
		writeReadinessFact(b, ".verdi/policy/constitution.md", "selects the governance profile and declares the project's role, transition, evidence, subject and adapter catalogs.")
		writeReadinessFact(b, policyGuidePlaceholderPath(".verdi/policy/profiles/<profile-id>.md"), "declares the supported identity trust sources, role mappings and approval requirements. A mapping is not proof that anyone was authenticated or approved a change.")
		writeReadinessFact(b, policyGuidePlaceholderPath(".verdi/policy/policies/<name>.md"), "carries the project's requirements. Overlays, exemptions and dispositions only when actually needed.")
		writeReadinessFact(b, ".verdi/constitution/consumers.json", "declares the real registered consumers impact coverage needs. Never fabricate an empty or baseline inventory to make preparation look complete.")
		b.WriteString(`</dl><p class="ritual-note">Author these only after the Inspect check confirms no accepted policy. Keep them on a proposal branch. Do not copy a fixture&#39;s identities, approvals or trust facts into a real project. <code>verdi context constitution propose</code> amends one policy, overlay or exemption; it does not create the initial constitution or profile.</p></details>`)
	default:
		b.WriteString(`<p class="readiness-summary">Policy authority resolved for this project, but it does not grant design assistance. The workbench reported: <code>` + esc(detail) + `</code>. Ordinary human editing does not require policy; this board&#39;s read-only restrictions still apply. A semantic review packet and delegated-agent design assistance need a policy that grants them, so review stays blocked until the project&#39;s policy does.</p>`)
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

// writeASDConcern renders one shell row: plain summary, plain state chip,
// explicit timing/dependency (F-02), source-derived guidance (F-03), the
// plain human-review label (F-05), and the complete technical details.
func writeASDConcern(b *strings.Builder, c asdConcern, asd *asdView, rank int) {
	esc := stdhtml.EscapeString
	class := "readiness-row"
	if rank > 0 {
		class = "readiness-card"
	}
	b.WriteString(`<article class="` + class + ` readiness-concern--` + esc(c.State) + `" data-concern-id="` + esc(c.ID) + `" data-area-id="` + esc(string(c.Area)) + `">`)
	if rank > 0 {
		b.WriteString(`<span class="readiness-rank">` + strconv.Itoa(rank) + `</span>`)
	}
	b.WriteString(`<div class="readiness-copy">`)
	b.WriteString(`<p class="readiness-stage">` + esc(asdAreaLabels[c.Area]))
	// Explicit timing and dependency (F-02): current-step rows say "now";
	// later-step rows name what they wait on.
	if asd.Shell.CurrentFocus != "" && c.State != asdStateProven {
		if c.Area == asd.Shell.CurrentFocus {
			b.WriteString(` <span class="asd-timing asd-timing--now" data-timing="now">· now</span>`)
		} else if asdAreaAfter(c.Area, asd.Shell.CurrentFocus) {
			b.WriteString(` <span class="asd-timing asd-timing--later" data-timing="later">· later — waits on ` + esc(asdAreaLabels[asd.Shell.CurrentFocus]) + `</span>`)
		}
	}
	b.WriteString(`</p>`)
	if c.HumanReview {
		b.WriteString(`<p class="asd-human-review" data-testid="asd-human-review">Human review</p>`)
	}
	b.WriteString(`<p class="readiness-summary">` + esc(c.Summary) + `</p>`)
	writeASDState(b, c.State)
	if c.Guidance != "" {
		b.WriteString(`<p class="asd-guidance" data-testid="asd-guidance-` + esc(c.ID) + `">` + esc(c.Guidance) + `</p>`)
	}
	b.WriteString(`<details class="readiness-tech"><summary>Technical details</summary><dl class="readiness-tech-facts">`)
	writeReadinessFact(b, "State", c.State)
	writeReadinessFact(b, "Concern", c.ID)
	writeReadinessFact(b, "Area", string(c.Area))
	writeReadinessFact(b, "Blocking", strconv.FormatBool(c.Blocking))
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
		// creation as unclassified — never falsified here.
		b.WriteString(`<p class="ritual-note asd-import-origin" data-testid="asd-import-origin"><a href="` + esc(asd.ImportRecordHref) + `">Source record for this import</a> &mdash; this spec was created by importing existing content. The record separately describes the original copied content, verified against the branch's committed bytes; it is not an ASD provenance entry (the review packet may still classify the creation as unclassified) and not evidence of acceptance.</p>`)
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
