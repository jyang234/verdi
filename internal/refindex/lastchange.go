package refindex

import (
	"context"
	"fmt"
	"sort"

	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/specstate"
)

// The two DateDisclosed sources (spec/index-data ac-1; SI-297): a date that
// was asked for and could not be read, and a date that was never asked for
// because the entry's effective state carries no landing commit to date.
const (
	dateUnreadableSource = "refindex:date-unreadable"
	dateUnprovenSource   = "refindex:date-unproven"
)

// dateBatch is one walk's single CommitDates answer: every date that walk
// needs, read through the port in ONE call (co-1's one computation per
// render; one git process per walk through gitx.CommitDates, never one per
// entry). err is the batch read's own failure, disclosed on every entry the
// batch dates, never propagated.
type dateBatch struct {
	dates map[string]string
	err   error
}

// readDateBatch asks deps for the committer date of every rev in revs in
// one port call — deduplicated and sorted, so the call is deterministic.
// With no revs it makes no call at all.
func readDateBatch(ctx context.Context, deps GitRunner, root string, revs []string) dateBatch {
	asked := sortedUnique(revs)
	if len(asked) == 0 {
		return dateBatch{}
	}
	dates, err := deps.CommitDates(ctx, root, asked)
	return dateBatch{dates: dates, err: err}
}

// dateFor returns rev's committer date for the entry named ref, or ac-1's
// per-entry disclosure — never a zero or current-date stand-in — when the
// batch read failed, rev is absent from its answer, or the answer is empty
// or not a committer date (B1-R6: an empty or unparsable answer is
// unreadable, never a readable date).
func (b dateBatch) dateFor(ref, rev string) (string, *disclosure.Disclosure) {
	if b.err != nil {
		return "", dateDisclosure(dateUnreadableSource, ref, fmt.Sprintf("last-change date unreadable: %v", b.err))
	}
	date, ok := b.dates[rev]
	if !ok || date == "" {
		return "", dateDisclosure(dateUnreadableSource, ref, fmt.Sprintf("last-change date unreadable: no committer date could be read for %s", rev))
	}
	if _, err := parseLastChange(date); err != nil {
		return "", dateDisclosure(dateUnreadableSource, ref, fmt.Sprintf("last-change date unreadable: %q read for %s is not a committer date", date, rev))
	}
	return date, nil
}

// landingCommit is r's landing commit — the commit where the entry's
// current bytes landed on the default branch (dc-1: specstate's baseline) —
// or "" when r carries none.
func landingCommit(r specstate.Result) string {
	if r.Baseline == nil {
		return ""
	}
	return r.Baseline.LandingCommit
}

// landingDate is a default-branch entry's date (dc-1): its landing
// commit's committer date from the walk's one batch. A result with no
// landing commit is disclosed as unproven (SI-297), never read and never
// defaulted.
func landingDate(ref string, r specstate.Result, batch dateBatch) (string, *disclosure.Disclosure) {
	landing := landingCommit(r)
	if landing == "" {
		return "", unprovenDateDisclosure(ref, r)
	}
	return batch.dateFor(ref, landing)
}

// unprovenDateDisclosure names why a default-branch entry has no landing
// commit to date (B1-R3, SI-297). specstate returns no baseline on an
// unproven result — even when it did compute the landing commit and the
// state is unproven for another reason (an incomplete corpus scan, a
// link-only successor) — so the disclosure names the unproven effective
// state with specstate's own disclosure, never a blanket claim that no
// landing commit exists. Any other state without a landing commit
// (unreachable for a default-branch entry today) is named as that state.
// This package never reads the landing commit itself: recovering those
// dates is BL-99's additive specstate field.
func unprovenDateDisclosure(ref string, r specstate.Result) *disclosure.Disclosure {
	if r.State != specstate.Unproven {
		return dateDisclosure(dateUnprovenSource, ref, fmt.Sprintf("last-change date unproven: specstate returned no landing commit for this entry's %s state", r.State))
	}
	text := "last-change date unproven: this entry's effective lifecycle state is unproven"
	if len(r.Disclosures) > 0 {
		text += " (" + joinDisclosures(r.Disclosures) + ")"
	}
	return dateDisclosure(dateUnprovenSource, ref, text)
}

func dateDisclosure(source, ref, text string) *disclosure.Disclosure {
	d := disclosure.New(source, ref, text)
	return &d
}

// sortedUnique returns revs without empty or repeated values, sorted.
func sortedUnique(revs []string) []string {
	seen := make(map[string]bool, len(revs))
	out := make([]string, 0, len(revs))
	for _, rev := range revs {
		if rev == "" || seen[rev] {
			continue
		}
		seen[rev] = true
		out = append(out, rev)
	}
	sort.Strings(out)
	return out
}
