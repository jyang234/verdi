// Command measure is spike/verification-rules oq-1 evidence: a mechanical
// clause-enumeration counter over every active spec's acceptance criteria
// and constraints. It is throwaway spike code (VL-016 fence,
// docs/spikes/verification-rules/) and is never wired into any real lint
// rule; the real under-enumeration lint (if adopted) is built under test in
// spec/verification-rules' own feature plan.
//
// Usage: go run ./docs/spikes/verification-rules/_scratch/measure [root]
// root defaults to "." and must be the verdi module root (the directory
// containing .verdi/specs/active).
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

// claimItem is one acceptance-criterion or constraint text, decoded
// through the real artifact package (never a bespoke YAML parse), with its
// three mechanical claim-count proxies attached.
type claimItem struct {
	Spec string
	Kind string // "ac" or "co"
	ID   string
	Text string

	// ListProxy: 1 + count(',') + count(';') — each comma or semicolon is
	// read as a list-item boundary.
	ListProxy int
	// NeverNoProxy: count of whole-word "never"/"no" occurrences (floor 1)
	// — each is read as an independent prohibition claim.
	NeverNoProxy int
	// AndProxy: 1 + count of whole-word "and" occurrences — each "and" is
	// read as joining one more clause/verb phrase.
	AndProxy int
	// Claims is max(ListProxy, NeverNoProxy, AndProxy) — the combined
	// mechanical claim count this spike scores and thresholds. Taking the
	// max (not the sum) avoids triple-counting the same underlying
	// boundary when more than one proxy fires on it (e.g. an
	// Oxford-comma "and" sits at a comma the ListProxy already counted).
	Claims int
}

var (
	neverNoRe = regexp.MustCompile(`(?i)\b(never|no)\b`)
	andRe     = regexp.MustCompile(`(?i)\band\b`)
)

func countClaims(text string) (listP, nnP, andP, claims int) {
	listP = 1 + strings.Count(text, ",") + strings.Count(text, ";")
	nnP = len(neverNoRe.FindAllString(text, -1))
	if nnP < 1 {
		nnP = 1
	}
	andP = 1 + len(andRe.FindAllString(text, -1))
	claims = listP
	if nnP > claims {
		claims = nnP
	}
	if andP > claims {
		claims = andP
	}
	return
}

func main() {
	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	specsDir := filepath.Join(root, ".verdi", "specs", "active")
	entries, err := os.ReadDir(specsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "measure: reading %s: %v\n", specsDir, err)
		os.Exit(2)
	}

	var dirCount int
	var items []claimItem
	var decodeErrs []string
	var sentenceCount int // diagnostic: texts containing a mid-text ". " (multi-sentence)

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirCount++
		specPath := filepath.Join(specsDir, e.Name(), "spec.md")
		raw, err := os.ReadFile(specPath)
		if err != nil {
			decodeErrs = append(decodeErrs, fmt.Sprintf("%s: read: %v", e.Name(), err))
			continue
		}
		fm, _, err := artifact.SplitFrontmatter(raw)
		if err != nil {
			decodeErrs = append(decodeErrs, fmt.Sprintf("%s: split-frontmatter: %v", e.Name(), err))
			continue
		}
		spec, err := artifact.DecodeSpec(fm)
		if err != nil {
			decodeErrs = append(decodeErrs, fmt.Sprintf("%s: decode-spec: %v", e.Name(), err))
			continue
		}
		for _, ac := range spec.AcceptanceCriteria {
			if strings.Contains(strings.TrimSuffix(ac.Text, "."), ". ") {
				sentenceCount++
			}
			l, n, a, c := countClaims(ac.Text)
			items = append(items, claimItem{e.Name(), "ac", ac.ID, ac.Text, l, n, a, c})
		}
		for _, co := range spec.Constraints {
			if strings.Contains(strings.TrimSuffix(co.Text, "."), ". ") {
				sentenceCount++
			}
			l, n, a, c := countClaims(co.Text)
			items = append(items, claimItem{e.Name(), "co", co.ID, co.Text, l, n, a, c})
		}
	}

	fmt.Printf("active spec directories found: %d\n", dirCount)
	fmt.Printf("decode errors: %d\n", len(decodeErrs))
	for _, m := range decodeErrs {
		fmt.Printf("  DECODE-ERROR: %s\n", m)
	}
	fmt.Printf("criteria+constraints scored: %d\n", len(items))
	fmt.Printf("texts containing an internal '. ' (candidate multi-sentence): %d\n", sentenceCount)

	// Histogram of Claims value.
	hist := map[int]int{}
	maxClaims := 0
	for _, it := range items {
		hist[it.Claims]++
		if it.Claims > maxClaims {
			maxClaims = it.Claims
		}
	}
	fmt.Println("\nhistogram (claims -> count):")
	for c := 0; c <= maxClaims; c++ {
		if hist[c] == 0 {
			continue
		}
		fmt.Printf("  %2d claims: %3d  %s\n", c, hist[c], strings.Repeat("#", hist[c]))
	}

	// Threshold sweep: for each candidate threshold T, how many items score
	// claims > T (the candidate backlog size at that threshold).
	fmt.Println("\nthreshold sweep (T -> count with claims > T):")
	for t := 1; t <= 24; t++ {
		n := 0
		for _, it := range items {
			if it.Claims > t {
				n++
			}
		}
		fmt.Printf("  T=%d: %d\n", t, n)
	}

	// Top 20 by Claims, ties broken by Spec then ID for determinism.
	sort.Slice(items, func(i, j int) bool {
		if items[i].Claims != items[j].Claims {
			return items[i].Claims > items[j].Claims
		}
		if items[i].Spec != items[j].Spec {
			return items[i].Spec < items[j].Spec
		}
		return items[i].ID < items[j].ID
	})
	n := 20
	if len(items) < n {
		n = len(items)
	}
	fmt.Println("\ntop 20 by mechanical claim count:")
	for i := 0; i < n; i++ {
		it := items[i]
		fmt.Printf("\n#%d claims=%d (list=%d never/no=%d and=%d) %s#%s\n", i+1, it.Claims, it.ListProxy, it.NeverNoProxy, it.AndProxy, it.Spec, it.ID)
		fmt.Printf("    %s\n", it.Text)
	}
}
