package skillpack

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// Finding is one skill that is absent or differs from what the running
// binary renders. Code is "missing" or "drift". Expected and Actual are
// content digests of the render-commit-masked bytes (empty for missing).
type Finding struct {
	Code     string
	Path     string
	Expected string
	Actual   string
}

// Report is Check's fail-closed result: Checked counts every (host,
// skill) pair examined; Findings is sorted by Path.
type Report struct {
	Findings []Finding
	Checked  int
}

// Clean reports zero findings.
func (r Report) Clean() bool { return len(r.Findings) == 0 }

var renderCommitLine = regexp.MustCompile(`(?m)^<!-- verdi:render-commit [^>]*-->\n`)

// maskRenderCommit removes the render-commit stamp line (R-W3-2).
func maskRenderCommit(b []byte) []byte { return renderCommitLine.ReplaceAll(b, nil) }

// Check recomputes every skill for hosts from the running binary and
// compares the on-disk bytes under root with the render-commit line
// masked on both sides.
func Check(root string, hosts []Host) (Report, error) {
	var rep Report
	for _, h := range hosts {
		for _, s := range Skills() {
			rep.Checked++
			want, err := Render(h, s, noRenderCommit)
			if err != nil {
				return Report{}, err
			}
			wantMasked := maskRenderCommit(want.Content)
			full := filepath.Join(root, filepath.FromSlash(want.Path))
			got, err := os.ReadFile(full)
			switch {
			case errors.Is(err, fs.ErrNotExist):
				rep.Findings = append(rep.Findings, Finding{Code: "missing", Path: want.Path})
				continue
			case err != nil:
				return Report{}, fmt.Errorf("skillpack: %s: %w", want.Path, err)
			}
			gotMasked := maskRenderCommit(got)
			if !bytes.Equal(wantMasked, gotMasked) {
				rep.Findings = append(rep.Findings, Finding{Code: "drift", Path: want.Path, Expected: contentDigest(wantMasked), Actual: contentDigest(gotMasked)})
			}
		}
	}
	sort.Slice(rep.Findings, func(i, j int) bool { return rep.Findings[i].Path < rep.Findings[j].Path })
	return rep, nil
}
