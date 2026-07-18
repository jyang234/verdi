// The verdict viewer: GET /verdict/{story...} — pick two derived snapshots
// of a story (each a commit-named subdirectory of
// data/derived/<ref-slug>/, holding one verdicts.json — 01 §Directory
// layout, 03 §Evidence records) and render a side-by-side, per-AC diff.
// With no ?a=&b= query, the page lists every available snapshot commit so
// a human can pick two; with both given, it renders the diff table.
package workbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/storyresolve"
)

// commitDirRe matches a derived tree's commit-named subdirectories,
// mirroring internal/evidence's own (private) pattern.
var commitDirRe = regexp.MustCompile(`^[0-9a-f]{7,40}$`)

// verdictHandler serves the viewer. mdl is the store's resolved operating
// model: the empty-picker copy below speaks the story class word —
// display prose, resolved (vocabulary.go); nil serves the bare id.
func verdictHandler(root string, mdl *model.Model) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		storyArg := r.PathValue("story")
		if storyArg == "" {
			http.NotFound(w, r)
			return
		}
		spec, err := storyresolve.Resolve(root, storyArg)
		if err != nil {
			renderError(w, http.StatusNotFound, err)
			return
		}

		derivedRoot := store.DerivedSpecDir(root, store.RefSlug(spec.ID))
		commits, err := listSnapshotCommits(derivedRoot)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}

		a := r.URL.Query().Get("a")
		b := r.URL.Query().Get("b")

		var extra bytes.Buffer
		extra.WriteString(`<section class="verdict-viewer">`)
		if a == "" || b == "" {
			writeSnapshotPicker(&extra, storyArg, commits, classWords{m: mdl})
		} else {
			recA, errA := loadSnapshot(derivedRoot, a)
			recB, errB := loadSnapshot(derivedRoot, b)
			if errA != nil || errB != nil {
				renderError(w, http.StatusNotFound, firstErr(errA, errB))
				return
			}
			writeSnapshotDiff(&extra, spec, a, recA, b, recB)
		}
		extra.WriteString(`</section>`)

		specRef, _ := artifact.ParseRef(spec.ID)
		page := pageData{
			Title:     "Verdict viewer: " + spec.ID,
			Nav:       template.HTML(`<a href="/">index</a> <a href="/a/spec/` + stdhtml.EscapeString(specRef.Name) + `">spec</a>`),
			BodyHTML:  "",
			ExtraHTML: template.HTML(extra.String()),
		}
		out, err := renderPage(page)
		if err != nil {
			renderError(w, http.StatusInternalServerError, err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out) // response body write; post-header error is unactionable
	}
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

// listSnapshotCommits lists derivedRoot's commit-named subdirectories,
// sorted for deterministic rendering. A missing derivedRoot is not an
// error (a story with no derived snapshots yet) — returns an empty list.
func listSnapshotCommits(derivedRoot string) ([]string, error) {
	entries, err := os.ReadDir(derivedRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && commitDirRe.MatchString(e.Name()) {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// loadSnapshot strict-decodes derivedRoot/commit/verdicts.json — an array
// of verdi.evidence/v1 records, each decoded through the same strict seam
// internal/evidence.LoadRecords uses (that function itself is
// evidence-package-private; this is a small, local reimplementation of
// the same "decode a JSON array of Evidence records" step, not the fold
// logic around it).
func loadSnapshot(derivedRoot, commit string) ([]artifact.Evidence, error) {
	if !commitDirRe.MatchString(commit) {
		return nil, os.ErrNotExist
	}
	path := filepath.Join(derivedRoot, commit, "verdicts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("workbench: unmarshaling %s: %w", path, err)
	}
	out := make([]artifact.Evidence, 0, len(raw))
	for i, rm := range raw {
		rec, err := artifact.DecodeEvidence(rm)
		if err != nil {
			return nil, fmt.Errorf("workbench: %s record %d: %w", path, i, err)
		}
		out = append(out, *rec)
	}
	return out, nil
}

func writeSnapshotPicker(buf *bytes.Buffer, storyArg string, commits []string, words classWords) {
	buf.WriteString("<p>Pick two snapshots to diff:</p><ul>")
	for _, c := range commits {
		buf.WriteString("<li>")
		buf.WriteString(stdhtml.EscapeString(c))
		buf.WriteString("</li>")
	}
	buf.WriteString("</ul>")
	if len(commits) >= 2 {
		buf.WriteString(`<form method="get" class="snapshot-picker">`)
		buf.WriteString(`<label>A <select name="a">`)
		for _, c := range commits {
			buf.WriteString(`<option value="` + stdhtml.EscapeString(c) + `">` + stdhtml.EscapeString(c) + `</option>`)
		}
		buf.WriteString(`</select></label> `)
		buf.WriteString(`<label>B <select name="b">`)
		for i := len(commits) - 1; i >= 0; i-- {
			buf.WriteString(`<option value="` + stdhtml.EscapeString(commits[i]) + `">` + stdhtml.EscapeString(commits[i]) + `</option>`)
		}
		buf.WriteString(`</select></label> `)
		buf.WriteString(`<button type="submit">Diff</button></form>`)
	} else {
		// "story" is display prose here (the class word), resolved like
		// every other class-word site; the /verdict/{story} route and
		// storyArg stay identity.
		buf.WriteString("<p>Fewer than two snapshots exist yet for this " + stdhtml.EscapeString(words.word("story")) + ".</p>")
	}
}

// evKey is one (kind, verdict, witness) tuple — the diff's atomic unit,
// since two records for the same AC and kind but different witnesses (or
// verdicts) are genuinely different evidence, not a duplicate.
type evKey struct{ Kind, Verdict, Witness string }

func writeSnapshotDiff(buf *bytes.Buffer, spec *artifact.SpecFrontmatter, a string, recA []artifact.Evidence, b string, recB []artifact.Evidence) {
	byACA := groupByAC(recA)
	byACB := groupByAC(recB)

	acIDs := make([]string, 0, len(spec.AcceptanceCriteria))
	seen := map[string]bool{}
	for _, ac := range spec.AcceptanceCriteria {
		acIDs = append(acIDs, ac.ID)
		seen[ac.ID] = true
	}
	for ac := range byACA {
		if !seen[ac] {
			acIDs = append(acIDs, ac)
			seen[ac] = true
		}
	}
	for ac := range byACB {
		if !seen[ac] {
			acIDs = append(acIDs, ac)
			seen[ac] = true
		}
	}

	// The two snapshot commits head their columns visually truncated
	// (.ulid ellipsis) with the full sha intact as text and title — same
	// treatment as dispositions-table ULIDs.
	buf.WriteString(`<table class="verdict-diff"><thead><tr><th>AC</th><th><code class="ulid" title="`)
	buf.WriteString(stdhtml.EscapeString(a))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(a))
	buf.WriteString(`</code></th><th><code class="ulid" title="`)
	buf.WriteString(stdhtml.EscapeString(b))
	buf.WriteString(`">`)
	buf.WriteString(stdhtml.EscapeString(b))
	buf.WriteString(`</code></th><th>diff</th></tr></thead><tbody>`)
	for _, ac := range acIDs {
		setA := toSet(byACA[ac])
		setB := toSet(byACB[ac])
		buf.WriteString("<tr><td>")
		buf.WriteString(stdhtml.EscapeString(ac))
		buf.WriteString("</td><td>")
		writeEvidenceCell(buf, byACA[ac])
		buf.WriteString("</td><td>")
		writeEvidenceCell(buf, byACB[ac])
		buf.WriteString("</td><td>")
		buf.WriteString(diffLabel(setA, setB))
		buf.WriteString("</td></tr>")
	}
	buf.WriteString("</tbody></table>")
}

func groupByAC(records []artifact.Evidence) map[string][]artifact.Evidence {
	m := map[string][]artifact.Evidence{}
	for _, r := range records {
		for _, ac := range r.EvidenceFor {
			m[ac] = append(m[ac], r)
		}
	}
	return m
}

func toSet(records []artifact.Evidence) map[evKey]bool {
	s := map[evKey]bool{}
	for _, r := range records {
		s[evKey{Kind: string(r.Kind), Verdict: string(r.Verdict), Witness: r.Witness}] = true
	}
	return s
}

func writeEvidenceCell(buf *bytes.Buffer, records []artifact.Evidence) {
	if len(records) == 0 {
		buf.WriteString("<em>no evidence</em>")
		return
	}
	buf.WriteString("<ul>")
	for _, r := range records {
		buf.WriteString("<li>")
		buf.WriteString(stdhtml.EscapeString(string(r.Kind)))
		buf.WriteString(": ")
		buf.WriteString(stdhtml.EscapeString(string(r.Verdict)))
		if r.Witness != "" {
			buf.WriteString(" (")
			buf.WriteString(stdhtml.EscapeString(r.Witness))
			buf.WriteString(")")
		}
		buf.WriteString("</li>")
	}
	buf.WriteString("</ul>")
}

// diffLabel reports whether a and b's evidence sets are identical,
// changed, or one-sided.
func diffLabel(a, b map[evKey]bool) string {
	if len(a) == 0 && len(b) == 0 {
		return "—"
	}
	if setEqual(a, b) {
		return "same"
	}
	if len(a) == 0 {
		return "added in " + "B"
	}
	if len(b) == 0 {
		return "removed in " + "B"
	}
	return "changed"
}

func setEqual(a, b map[evKey]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
