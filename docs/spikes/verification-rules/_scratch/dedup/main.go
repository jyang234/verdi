// Command dedup is spike/verification-rules oq-1 evidence, REVISION per
// lane-verification-rules-review.md VR-1: the review found the 277-object
// corpus holds only 206 distinct texts (71 byte-duplicates, 70 of them
// from the five -vN version-revision directories carrying every AC/
// constraint forward unchanged, per oq-4's own finding), and that 5 of
// the original top-20 hand labels are duplicate copies of other labeled
// rows, so the real calibration sample is 15 distinct judgments, not 20.
// This program recomputes the histogram/threshold sweep on three
// populations — raw (all 277), distinct-text (dedup by exact text,
// keeping the highest-claims/lowest-rank representative), and
// live-distinct (distinct text AND excluding any object whose spec
// directory is itself the ref of some OTHER spec's `links: {type:
// supersedes}` edge, i.e. a frozen predecessor revision that can never be
// rewritten, oq-4) — and reports the deduplicated top-20 sample's
// remaining distinct judgments for the FP-rate sweep, using this spike's
// own already-recorded hand labels (labels.md), not a re-judgment.
//
// Usage: go run ./docs/spikes/verification-rules/_scratch/dedup [root]
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

type item struct {
	Spec   string
	Kind   string
	ID     string
	Text   string
	Claims int
}

var (
	neverNoRe = regexp.MustCompile(`(?i)\b(never|no)\b`)
	andRe     = regexp.MustCompile(`(?i)\band\b`)
)

func countClaims(text string) int {
	listP := 1 + strings.Count(text, ",") + strings.Count(text, ";")
	nnP := len(neverNoRe.FindAllString(text, -1))
	if nnP < 1 {
		nnP = 1
	}
	andP := 1 + len(andRe.FindAllString(text, -1))
	claims := listP
	if nnP > claims {
		claims = nnP
	}
	if andP > claims {
		claims = andP
	}
	return claims
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	specsDir := filepath.Join(root, ".verdi", "specs", "active")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dedup: reading %s: %v\n", specsDir, err)
		os.Exit(2)
	}

	var items []item
	frozen := map[string]bool{} // spec dir names that are supersedes-link TARGETS

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		specPath := filepath.Join(specsDir, e.Name(), "spec.md")
		raw, err := os.ReadFile(specPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dedup: %s: read: %v\n", e.Name(), err)
			os.Exit(2)
		}
		fm, _, err := artifact.SplitFrontmatter(raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dedup: %s: split: %v\n", e.Name(), err)
			os.Exit(2)
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			fmt.Fprintf(os.Stderr, "dedup: %s: decode: %v\n", e.Name(), err)
			os.Exit(2)
		}
		for _, l := range spec.Links {
			if string(l.Type) == "supersedes" {
				// l.Ref is "spec/<name>" — record the bare name as frozen.
				name := strings.TrimPrefix(l.Ref, "spec/")
				frozen[name] = true
			}
		}
		for _, ac := range spec.AcceptanceCriteria {
			items = append(items, item{e.Name(), "ac", ac.ID, ac.Text, countClaims(ac.Text)})
		}
		for _, co := range spec.Constraints {
			items = append(items, item{e.Name(), "co", co.ID, co.Text, countClaims(co.Text)})
		}
	}

	fmt.Printf("raw objects scored: %d\n", len(items))
	fmt.Printf("frozen-predecessor directories (supersedes-link targets): %d -> %v\n", len(frozen), sortedFrozenList(frozen))

	// Distinct-text dedup: stable, keep the FIRST-encountered (directory
	// iteration order from os.ReadDir, which is lexicographic) instance of
	// each exact text as the representative.
	seenText := map[string]bool{}
	var distinct []item
	for _, it := range items {
		if seenText[it.Text] {
			continue
		}
		seenText[it.Text] = true
		distinct = append(distinct, it)
	}
	fmt.Printf("distinct texts: %d\n", len(distinct))

	// Live-distinct: distinct text AND the representative's own directory
	// is not itself a frozen-predecessor target. Since a carried object's
	// text is byte-identical across every revision in its family (oq-4),
	// dedup-by-text alone already collapses a frozen copy into whichever
	// instance is encountered first; this second pass additionally drops
	// any distinct text whose FIRST-seen representative happens to be a
	// frozen directory, re-pointing to a live sibling with the same text
	// when one exists, and dropping it outright when none does.
	var liveDistinct []item
	for _, d := range distinct {
		if !frozen[d.Spec] {
			liveDistinct = append(liveDistinct, d)
			continue
		}
		// d's representative is itself frozen — look for a live sibling
		// with the identical text.
		found := false
		for _, it := range items {
			if it.Text == d.Text && !frozen[it.Spec] {
				liveDistinct = append(liveDistinct, it)
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("  DROPPED (frozen-only, no live copy): %s#%s\n", d.Spec, d.ID)
		}
	}
	fmt.Printf("live-distinct texts (excluding frozen predecessors, re-pointed to a live sibling where one exists): %d\n", len(liveDistinct))

	sweep := func(label string, pop []item) {
		fmt.Printf("\nthreshold sweep on %s (n=%d):\n", label, len(pop))
		for t := 13; t <= 19; t++ {
			n := 0
			for _, it := range pop {
				if it.Claims > t {
					n++
				}
			}
			fmt.Printf("  T=%d: %d\n", t, n)
		}
	}
	sweep("raw-277", items)
	sweep("distinct-206", distinct)
	sweep("live-distinct", liveDistinct)

	// Deduplicated top-20 hand-label sample: rank the original top-20 by
	// (claims desc, spec, id) exactly as measure/main.go did, then collapse
	// consecutive-or-not duplicate texts, keeping the first (highest-rank)
	// occurrence, and apply this spike's own recorded labels.md verdicts.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Claims != items[j].Claims {
			return items[i].Claims > items[j].Claims
		}
		if items[i].Spec != items[j].Spec {
			return items[i].Spec < items[j].Spec
		}
		return items[i].ID < items[j].ID
	})
	top20 := items[:20]
	labels := map[string]bool{ // spec#id -> TRUE(true)/FALSE(false), from labels.md
		"ai-assisted-spec-design#co-3": true, "readiness-recovery#ac-8": true,
		"ai-assisted-spec-design#co-9": true, "ai-assisted-spec-design#co-5": true,
		"readiness-recovery#ac-9": true, "sealed-claude-vatc-events#ac-3": false,
		"sealed-codex-execution#ac-3": false, "ai-assisted-spec-design#co-2": true,
		"comparative-spike-experiments#ac-6": true, "comparative-spike-experiments-v2#ac-6": true,
		"comparative-spike-experiments-v3#ac-6": true, "spec-documents#ac-1": true,
		"sealed-claude-vatc-events#ac-2": false, "context-integrity#ac-2": false,
		"context-integrity-v2#ac-2": false, "context-receipts-review#ac-1": false,
		"readiness-recovery#ac-1": false, "comparative-spike-experiments#ac-1": true,
		"comparative-spike-experiments-v2#ac-1": true, "comparative-spike-experiments-v3#ac-1": true,
	}
	seenT20 := map[string]bool{}
	var distinctSample []item
	fmt.Println("\ntop-20 sample, deduplicated:")
	for i, it := range top20 {
		key := it.Spec + "#" + it.ID
		dup := seenT20[it.Text]
		seenT20[it.Text] = true
		tag := "DISTINCT"
		if dup {
			tag = "DUP"
		} else {
			distinctSample = append(distinctSample, it)
		}
		fmt.Printf("  #%-2d claims=%-2d %-6s label=%-5v %s\n", i+1, it.Claims, tag, labels[key], key)
	}
	fmt.Printf("distinct judgments in top-20 sample: %d\n", len(distinctSample))

	trueCt, falseCt := 0, 0
	for _, it := range distinctSample {
		if labels[it.Spec+"#"+it.ID] {
			trueCt++
		} else {
			falseCt++
		}
	}
	fmt.Printf("of which TRUE=%d FALSE=%d (unconditioned FP rate %.1f%%)\n", trueCt, falseCt, 100*float64(falseCt)/float64(len(distinctSample)))

	fmt.Println("\nFP rate by threshold on the deduplicated 15-judgment sample:")
	for t := 13; t <= 19; t++ {
		var n, f int
		for _, it := range distinctSample {
			if it.Claims > t {
				n++
				if !labels[it.Spec+"#"+it.ID] {
					f++
				}
			}
		}
		rate := 0.0
		if n > 0 {
			rate = 100 * float64(f) / float64(n)
		}
		fmt.Printf("  T=%d: sample=%d false=%d rate=%.1f%%\n", t, n, f, rate)
	}
}

func sortedFrozenList(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
