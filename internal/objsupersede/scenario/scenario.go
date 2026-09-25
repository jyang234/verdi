// Package scenario loads and builds the closed-spec object supersession
// scenario stores committed under testdata/objsupersede (design §8's
// records, one separable store per scenario). It is a test helper, like
// internal/fixturegit: Load and Files are plain functions any provisioner
// can use, and Build turns one scenario into a real git repository for a
// Go test.
//
// A scenario starts from the manifest's base layers, committed on main by
// fixturegit.Build at fixturegit's fixed date (2024-01-01), and then runs
// its steps in order. A step either writes layers onto its branch as one
// commit, or merges another branch into its branch with --no-ff. Each step
// carries its own UTC date, so acceptance and closure dates are distinct
// and deterministic.
package scenario

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// Manifest is testdata/objsupersede/scenarios.json.
type Manifest struct {
	// Base names the layers every scenario starts from, one fixturegit
	// commit each, on main.
	Base []string `json:"base"`
	// Layers maps a layer name to the files it writes: repo path to a
	// record file relative to the fixture's records/ directory.
	Layers map[string]map[string]string `json:"layers"`
	// Scenarios maps a scenario name to its steps.
	Scenarios map[string]Scenario `json:"scenarios"`
}

// Scenario is one store: its steps, and the branch left checked out.
type Scenario struct {
	Checkout string `json:"checkout"`
	Steps    []Step `json:"steps"`
}

// Step is one commit on Branch (created from the current HEAD when it does
// not exist yet): Layers written as one commit, or Merge merged --no-ff.
type Step struct {
	Branch  string   `json:"branch"`
	Date    string   `json:"date"`
	Message string   `json:"message"`
	Layers  []string `json:"layers,omitempty"`
	Merge   string   `json:"merge,omitempty"`
}

// Dir returns the committed fixture directory, testdata/objsupersede at
// the module root, located from this source file.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "objsupersede")
}

// Load strict-decodes dir's scenarios.json and checks that every layer,
// branch, date, and record file a scenario names exists.
func Load(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, "scenarios.json"))
	if err != nil {
		return nil, fmt.Errorf("scenario: %w", err)
	}
	var m Manifest
	if err := artifact.DecodeExactJSON(data, &m); err != nil {
		return nil, fmt.Errorf("scenario: scenarios.json: %w", err)
	}
	if err := m.validate(dir); err != nil {
		return nil, fmt.Errorf("scenario: scenarios.json: %w", err)
	}
	return &m, nil
}

func (m *Manifest) validate(dir string) error {
	if len(m.Base) == 0 {
		return fmt.Errorf("no base layer")
	}
	for _, b := range m.Base {
		if _, ok := m.Layers[b]; !ok {
			return fmt.Errorf("base layer %q is not defined", b)
		}
	}
	for name, files := range m.Layers {
		for repoPath, src := range files {
			if path.Clean(repoPath) != repoPath || strings.HasPrefix(repoPath, "/") || strings.HasPrefix(repoPath, "..") {
				return fmt.Errorf("layer %q: repo path %q is not a clean relative path", name, repoPath)
			}
			if _, err := os.Stat(filepath.Join(dir, "records", filepath.FromSlash(src))); err != nil {
				return fmt.Errorf("layer %q: %w", name, err)
			}
		}
	}
	for name, sc := range m.Scenarios {
		if sc.Checkout == "" || len(sc.Steps) == 0 {
			return fmt.Errorf("scenario %q needs a checkout branch and at least one step", name)
		}
		for i, st := range sc.Steps {
			if err := m.validateStep(st); err != nil {
				return fmt.Errorf("scenario %q step %d: %w", name, i, err)
			}
		}
	}
	return nil
}

func (m *Manifest) validateStep(st Step) error {
	if st.Branch == "" || st.Message == "" {
		return fmt.Errorf("a step needs a branch and a message")
	}
	if _, err := stepTime(st.Date); err != nil {
		return err
	}
	if (len(st.Layers) == 0) == (st.Merge == "") {
		return fmt.Errorf("a step either writes layers or merges a branch, not both or neither")
	}
	for _, l := range st.Layers {
		if _, ok := m.Layers[l]; !ok {
			return fmt.Errorf("layer %q is not defined", l)
		}
	}
	return nil
}

// stepTime parses a step's RFC 3339 date and requires it in UTC.
func stepTime(date string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return time.Time{}, fmt.Errorf("date %q: %w", date, err)
	}
	if _, off := t.Zone(); off != 0 {
		return time.Time{}, fmt.Errorf("date %q is not UTC", date)
	}
	return t, nil
}

// Files returns the repo files the named layers write, later layers
// overriding earlier ones: repo path to content.
func (m *Manifest) Files(dir string, layers ...string) (map[string]string, error) {
	out := map[string]string{}
	for _, l := range layers {
		files, ok := m.Layers[l]
		if !ok {
			return nil, fmt.Errorf("scenario: layer %q is not defined", l)
		}
		paths := make([]string, 0, len(files))
		for p := range files {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			data, err := os.ReadFile(filepath.Join(dir, "records", filepath.FromSlash(files[p])))
			if err != nil {
				return nil, fmt.Errorf("scenario: layer %q: %w", l, err)
			}
			out[p] = string(data)
		}
	}
	return out, nil
}
