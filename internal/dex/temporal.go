package dex

import (
	"context"
	"fmt"
	"strings"

	"github.com/OWNER/verdi/internal/gitx"
)

// temporalClass is one of 01 §Temporal classes' three classes. dex derives
// it from data every artifact already carries — Frozen presence, and
// whether the entry is index-minted external (machine-discovered, never
// authored) — rather than hardcoding a per-kind table, so the rendering
// stays honest even for a kind this package has never special-cased.
type temporalClass int

const (
	classLivingGated temporalClass = iota
	classAuthoredLiving
	classFrozen
)

// classify implements 01 §Temporal classes' three-row table generically:
//
//   - a Frozen stamp present -> frozen (01: "feature specs at acceptance,
//     board.json, rollup.json, ... attestations" — every one of those
//     kinds requires Frozen once in that state, per internal/artifact's own
//     requireFrozen calls).
//   - no Frozen stamp, but the entry is index-minted external (boundary
//     contracts, obligations, OpenAPI docs) -> living-gated: these are
//     rediscovered fresh on every dex build from the tree's current
//     state, the same currency guarantee 01 gives goldens and boundary
//     contracts ("CI currency gate: regenerate + fail on drift").
//   - otherwise -> authored-living (component specs, draft feature specs,
//     diagrams, ADR index pages): maintained by humans, kept honest by
//     showing last-modified from git.
func classify(kind string, frozen bool) temporalClass {
	switch {
	case frozen:
		return classFrozen
	case kind == "external":
		return classLivingGated
	default:
		return classAuthoredLiving
	}
}

// bannerClass maps a temporal class to the temporal stamp's class-specific
// CSS hook (style.css's .temporal--* rules) — presentation only: the banner
// text itself (livingGatedBanner/frozenBanner/authoredLivingBanner below)
// is the honest record and never varies with styling.
func bannerClass(c temporalClass) string {
	switch c {
	case classFrozen:
		return "temporal--frozen"
	case classAuthoredLiving:
		return "temporal--authored-living"
	default:
		return "temporal--living-gated"
	}
}

// displayRef shortens a pinned ref's sha for display ("adr/0012@3e91ab2c…"
// -> "adr/0012@3e91ab2"). The full form always remains what the copy
// button actually copies (data-copy-ref) and announces (title/aria-label);
// this only trims the visible label.
func displayRef(ref string) string {
	i := strings.LastIndexByte(ref, '@')
	if i < 0 {
		return ref
	}
	return ref[:i+1] + shortSHA(ref[i+1:])
}

// buildStamp is the dex build's own commit + date — resolved once per
// Build call from the given commit (never time.Now(), per Phase 12's
// determinism rule) — the "main @ <sha> · <date>" every living-gated page
// (including every synthetic index/listing/changelog/search page, which
// are all regenerated fresh on every build) carries.
type buildStamp struct {
	SHA  string
	Date string // YYYY-MM-DD, truncated from git's ISO-8601 commit date
}

// resolveBuildStamp resolves commit's short sha and its own commit date
// (git's committer date, ISO-8601, date part only) in root — the one
// git-derived "now" dex is allowed to stamp pages with.
func resolveBuildStamp(ctx context.Context, root, commit string) (buildStamp, error) {
	sha, err := gitx.RevParse(ctx, root, commit)
	if err != nil {
		return buildStamp{}, fmt.Errorf("dex: resolving commit %q: %w", commit, err)
	}
	date, err := gitx.CommitDate(ctx, root, sha)
	if err != nil {
		return buildStamp{}, fmt.Errorf("dex: resolving commit date for %q: %w", sha, err)
	}
	return buildStamp{SHA: sha, Date: dateOnly(date)}, nil
}

// dateOnly truncates an ISO-8601 timestamp ("2024-01-01T00:00:00+00:00")
// to its date part ("2024-01-01").
func dateOnly(iso string) string {
	if i := strings.IndexByte(iso, 'T'); i >= 0 {
		return iso[:i]
	}
	return iso
}

// shortSHA returns sha's first 7 characters (git's conventional short
// form), or sha unchanged if it is already shorter.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// livingGatedBanner is 05 §Verdi-dex's literal template: "main @ <sha> ·
// <date>" — every living-gated page (external refs, and every dex-synthesized
// index/listing/changelog/search page).
func livingGatedBanner(stamp buildStamp) string {
	return fmt.Sprintf("main @ %s · %s", shortSHA(stamp.SHA), stamp.Date)
}

// frozenBanner is 01/05's literal template: "point-in-time record ·
// frozen <date> @ <commit>", stamped from the artifact's own frontmatter
// (never recomputed — the frozen stamp is the honest record of when it
// stopped claiming currency).
func frozenBanner(at, commit string) string {
	return fmt.Sprintf("point-in-time record · frozen %s @ %s", at, shortSHA(commit))
}

// authoredLivingBanner reports c (the artifact path's most recent commit,
// via gitx.LastCommit) as "last-modified <date> · <sha>" — 01's
// "authored-living pages show last-modified from git".
func authoredLivingBanner(c gitx.Commit) string {
	return fmt.Sprintf("last-modified %s · %s", dateOnly(c.Date), shortSHA(c.SHA))
}

// noHistoryBanner is authoredLivingBanner's honest fallback when git has no
// history at all for a path as of the build commit (e.g. a file the build
// commit itself introduced, so LastCommit's log query legitimately comes
// back empty for any earlier revision boundary) — never silently omitted or
// guessed (constitution 2's three-valued honesty: this is the
// "disclosed-as-unproven" branch, not silence).
const noHistoryBanner = "last-modified: unknown (no git history found for this path)"
