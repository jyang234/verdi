package evidence

import (
	"fmt"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// WaiverInput bundles RenderWaiver/RenderWaiverReaffirm's inputs
// (spec/verb-surfaces ac-1/ac-2): every field here is structure the
// caller (cmd/verdi's waive verb) already resolved — neither renderer
// does any resolution or I/O, and neither invents a rationale (mirroring
// evidence.AttestationScaffold's own dc-2-style discipline: the human's
// words are the human's words).
type WaiverInput struct {
	// StorySlug is store.RefSlug(story.Story) — the <storySlug> half of
	// waivers/<storySlug>/<acID>.md (03 §Attestations and waivers).
	StorySlug string
	// ACID is the acceptance-criterion id the waiver covers (e.g. "ac-1").
	ACID string
	// StoryRefArg is the raw <story-ref> the operator typed at the CLI —
	// echoed into the title so the record reads back what was invoked.
	StoryRefArg string
	// VerifiesRef is the resolved story spec's own canonical ref (e.g.
	// "spec/retry-worker") — the verifies edge's target.
	VerifiesRef string
	// Owners is copied verbatim from the resolved story spec's own
	// owners: (attest.go's own precedent — never invented).
	Owners []string
	// Reason is the operator's --rationale text, verbatim.
	Reason string
	// Expiry is the operator's --expires value (YYYY-MM-DD), or "" when
	// none was given.
	Expiry string
	// Frozen is the frozen stamp for THIS write: At is today (YYYY-MM-DD),
	// Commit is git HEAD at invocation time — the same attest.go
	// convenience-stamp convention (dc-2/ADJ-30 precedent), refreshed on
	// every reaffirm since each is its own fresh committed record (guide
	// 8.4: "a new committed record with a fresh rationale").
	Frozen artifact.Frozen
}

// waiverReaffirmationLogMarker delimits the waiver body's mechanically-
// owned reaffirmation log (spec/verb-surfaces ac-2): everything from this
// marker line to the end of the file is machine-maintained — appended to
// by RenderWaiverReaffirm, never reparsed as free prose — so accumulating
// history never requires understanding arbitrary markdown structure, only
// finding one fixed line.
const waiverReaffirmationLogMarker = "<!-- verdi:waiver-reaffirmation-log -->"

// waiverLogHeading is the heading immediately following the marker line —
// part of the machine-owned block, never hand-edited independently of it.
const waiverLogHeading = "## Reaffirmation log"

// waiverLogKindWaived and waiverLogKindReaffirmed are the two log-entry
// verbs RenderWaiver/RenderWaiverReaffirm mint. Deliberately NOT model
// lifecycle-verb ids (they never appear in any model.yaml transition), so
// they carry no DisplayVerb obligation of their own — a plain, fixed,
// internal vocabulary for this one mechanically-generated log.
const (
	waiverLogKindWaived     = "waived"
	waiverLogKindReaffirmed = "reaffirmed"
)

// waiverLogEntry renders one reaffirmation-log line: a dated, kind-prefixed
// record of a waive/reaffirm invocation's rationale and expiry, legible on
// its own without decoding YAML.
func waiverLogEntry(kind, at, reason, expiry string) string {
	expiryPart := "no expiry"
	if expiry != "" {
		expiryPart = "expires " + expiry
	}
	return fmt.Sprintf("- %s: %s — %s (%s)", at, kind, artifact.YAMLDoubleQuote(reason), expiryPart)
}

// extractReaffirmationLog returns the machine-owned log block (the marker
// line through the end of body, trimmed of trailing whitespace) from an
// existing waiver's body text, and whether the marker was found at all. A
// hand-authored waiver — or one this package never rendered — legitimately
// has no marker; that is not an error, just an empty prior log to append
// the first entry to (RenderWaiverReaffirm's caller never fabricates
// history that was never recorded).
func extractReaffirmationLog(body string) (log string, found bool) {
	i := strings.Index(body, waiverReaffirmationLogMarker)
	if i < 0 {
		return "", false
	}
	return strings.TrimRight(body[i:], "\n") + "\n", true
}

// renderWaiverFile assembles the complete, strict-decodable waiver file's
// bytes from already-resolved frontmatter fields and a complete body
// (rationale paragraph plus the full reaffirmation-log block, both already
// composed by the caller). Hand-rendered, never yaml.Marshal'd — the
// module-wide posture (internal/align/render.go, internal/evidence/
// attestations.go's RenderAttestationScaffold) — so field order and
// flow-mapping style are pinned exactly, byte for byte.
func renderWaiverFile(in WaiverInput, body string) string {
	id := fmt.Sprintf("waiver/%s--%s", in.StorySlug, in.ACID)
	title := fmt.Sprintf("waiver: %s %s", in.StoryRefArg, in.ACID)

	quotedOwners := make([]string, len(in.Owners))
	for i, o := range in.Owners {
		quotedOwners[i] = artifact.YAMLDoubleQuote(o)
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "id: %s\n", id)
	b.WriteString("kind: waiver\n")
	fmt.Fprintf(&b, "title: %s\n", artifact.YAMLDoubleQuote(title))
	fmt.Fprintf(&b, "owners: [%s]\n", strings.Join(quotedOwners, ", "))
	b.WriteString("status: active\n")
	fmt.Fprintf(&b, "reason: %s\n", artifact.YAMLDoubleQuote(in.Reason))
	if in.Expiry != "" {
		fmt.Fprintf(&b, "expiry: %s\n", in.Expiry)
	}
	b.WriteString("links:\n")
	fmt.Fprintf(&b, "  - { type: verifies, ref: %s }\n", artifact.YAMLDoubleQuote(in.VerifiesRef))
	fmt.Fprintf(&b, "frozen: { at: %s, commit: %s }\n", in.Frozen.At, in.Frozen.Commit)
	b.WriteString("---\n")
	b.WriteString(body)
	return b.String()
}

// RenderWaiver renders a complete, strict-decodable, freshly-created
// waiver file's bytes (spec/verb-surfaces ac-1): frontmatter carrying
// status: active, the given reason/expiry, owners copied verbatim, a
// single verifies link, and a frozen stamp; a body whose reaffirmation
// log carries exactly the one "waived" entry this creation is. Never
// invents rationale text — in.Reason is the operator's own words,
// unchanged.
func RenderWaiver(in WaiverInput) string {
	var body strings.Builder
	fmt.Fprintf(&body, "This AC is waived: %s\n\n", in.Reason)
	body.WriteString(waiverReaffirmationLogMarker + "\n")
	body.WriteString(waiverLogHeading + "\n\n")
	body.WriteString(waiverLogEntry(waiverLogKindWaived, in.Frozen.At, in.Reason, in.Expiry) + "\n")
	return renderWaiverFile(in, body.String())
}

// RenderWaiverReaffirm renders the same waiver rewritten in place for a
// reaffirmation (spec/verb-surfaces ac-2): frontmatter reason/expiry/
// status(reset to active)/frozen all reflect the fresh invocation;
// existingBody is the PRIOR file's own body text (read by the caller
// before this call), whose reaffirmation-log block is preserved verbatim
// and gains exactly one new "reaffirmed" entry appended after it. A prior
// body carrying no marker (hand-authored, or predating this mechanism)
// contributes no fabricated history — the log simply starts here, with
// this one entry.
func RenderWaiverReaffirm(existingBody string, in WaiverInput) string {
	priorLog, found := extractReaffirmationLog(existingBody)

	var body strings.Builder
	fmt.Fprintf(&body, "This AC is waived: %s\n\n", in.Reason)
	if found {
		body.WriteString(priorLog)
	} else {
		body.WriteString(waiverReaffirmationLogMarker + "\n")
		body.WriteString(waiverLogHeading + "\n\n")
	}
	body.WriteString(waiverLogEntry(waiverLogKindReaffirmed, in.Frozen.At, in.Reason, in.Expiry) + "\n")
	return renderWaiverFile(in, body.String())
}
