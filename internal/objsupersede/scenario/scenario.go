// Package scenario loads and builds the closed-spec object supersession
// scenario stores committed under testdata/objsupersede (design §8's
// records, one separable store per scenario). It is a test helper, like
// internal/fixturegit: Load, Files, and BaseFiles are plain functions any
// provisioner can use, and Build turns one scenario into a real git
// repository for a Go test.
//
// The manifest records every input a commit's SHA depends on (lane L3
// review a M-7): the commit identity and initial branch (Commit), and each
// step's branch, committer date, optional author date, message, and
// operation. Materialize replays it; a materializer that follows these
// rules reproduces every SHA, and so every frozen stamp, which names the
// root commit (re-review a m-3):
//
//   - Environment of every git invocation: no inherited GIT_* variable,
//     GIT_CONFIG_NOSYSTEM=1, GIT_CONFIG_GLOBAL=/dev/null, TZ=UTC, the
//     author and committer name and email from Commit, and the options
//     -c merge.log=false -c i18n.commitEncoding=UTF-8 -c core.autocrlf=false
//     -c core.hooksPath=/dev/null -c commit.gpgsign=false.
//   - `git init --object-format=sha1 --initial-branch=<initial branch>`.
//   - The base steps, in order, then the scenario's steps. Before a step,
//     check out its branch, creating it from HEAD when it does not exist.
//   - A layers step writes each layer's files in order (later layers win),
//     mode 0644, creating directories, then `git add -A` and
//     `git commit -q --no-verify -m <message>`.
//   - A moves step runs `git mv <from> <to>` for each move, creating the
//     destination's parent, then commits the same way.
//   - A merge step runs `git merge -q --no-ff --no-verify -m <message>
//     <branch>`.
//   - A commit or merge carries GIT_AUTHOR_DATE (the author date, else the
//     date) and GIT_COMMITTER_DATE (the date), each as "<unix seconds>
//     +0000".
//   - Finally, point refs/remotes/origin/<initial branch> at the initial
//     branch and check out the scenario's checkout.
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
	// Commit is every fixture commit's identity, and the initial branch.
	Commit Identity `json:"commit"`
	// Base holds the steps every scenario starts from, on the initial
	// branch; the first is the store's root commit.
	Base []Step `json:"base"`
	// Layers maps a layer name to the files it writes: repo path to a
	// record file relative to the fixture's records/ directory.
	Layers map[string]map[string]string `json:"layers"`
	// Scenarios maps a scenario name to its steps.
	Scenarios map[string]Scenario `json:"scenarios"`
}

// Identity is the author and committer of every fixture commit, and the
// branch `git init` starts on.
type Identity struct {
	Name          string `json:"name"`
	Email         string `json:"email"`
	InitialBranch string `json:"initial_branch"`
}

// Scenario is one store: its steps, and the branch left checked out.
type Scenario struct {
	Checkout string `json:"checkout"`
	Steps    []Step `json:"steps"`
}

// Step is one commit on Branch (created from the current HEAD when it does
// not exist yet): Layers written as one commit, Moves applied as one
// commit, or Merge merged --no-ff. Date is the committer date, and the
// author date unless AuthorDate is set; both are RFC 3339 in UTC.
type Step struct {
	Branch     string   `json:"branch"`
	Date       string   `json:"date"`
	AuthorDate string   `json:"author_date,omitempty"`
	Message    string   `json:"message"`
	Layers     []string `json:"layers,omitempty"`
	Moves      []Move   `json:"moves,omitempty"`
	Merge      string   `json:"merge,omitempty"`
}

// Move renames a repo path (a file or a directory) to another.
type Move struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Dir returns the committed fixture directory, testdata/objsupersede at
// the module root, located from this source file.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "objsupersede")
}

// Load strict-decodes dir's scenarios.json and checks it, reporting the
// first problem in sorted order: the identity is complete; every layer's
// repo paths are clean and its record files exist; every step names a
// branch, a message, UTC dates, and exactly one operation over defined
// layers or clean paths; base steps stay on the initial branch and never
// merge; and every merged branch and checkout exists by then (the initial
// branch, or one an earlier step committed to).
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
	if m.Commit.Name == "" || m.Commit.Email == "" || m.Commit.InitialBranch == "" {
		return fmt.Errorf("commit needs a name, an email, and an initial branch")
	}
	for _, name := range sortedKeys(m.Layers) {
		files := m.Layers[name]
		for _, repoPath := range sortedKeys(files) {
			if !cleanRel(repoPath) {
				return fmt.Errorf("layer %q: repo path %q is not a clean relative path", name, repoPath)
			}
			if _, err := os.Stat(filepath.Join(dir, "records", filepath.FromSlash(files[repoPath]))); err != nil {
				return fmt.Errorf("layer %q: %w", name, err)
			}
		}
	}
	if len(m.Base) == 0 {
		return fmt.Errorf("no base step")
	}
	for i, st := range m.Base {
		if st.Branch != m.Commit.InitialBranch || st.Merge != "" {
			return fmt.Errorf("base step %d must write layers or move paths on %s", i, m.Commit.InitialBranch)
		}
		if err := m.validateStep(st); err != nil {
			return fmt.Errorf("base step %d: %w", i, err)
		}
	}
	for _, name := range sortedKeys(m.Scenarios) {
		sc := m.Scenarios[name]
		if sc.Checkout == "" || len(sc.Steps) == 0 {
			return fmt.Errorf("scenario %q needs a checkout branch and at least one step", name)
		}
		branches := map[string]bool{m.Commit.InitialBranch: true}
		for i, st := range sc.Steps {
			if err := m.validateStep(st); err != nil {
				return fmt.Errorf("scenario %q step %d: %w", name, i, err)
			}
			if st.Merge != "" && !branches[st.Merge] {
				return fmt.Errorf("scenario %q step %d merges %q before any step commits to it", name, i, st.Merge)
			}
			branches[st.Branch] = true
		}
		if !branches[sc.Checkout] {
			return fmt.Errorf("scenario %q checks out %q, which no step commits to", name, sc.Checkout)
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
	if st.AuthorDate != "" {
		if _, err := stepTime(st.AuthorDate); err != nil {
			return err
		}
	}
	ops := 0
	for _, has := range []bool{len(st.Layers) > 0, len(st.Moves) > 0, st.Merge != ""} {
		if has {
			ops++
		}
	}
	if ops != 1 {
		return fmt.Errorf("a step writes layers, moves paths, or merges a branch: exactly one")
	}
	for _, l := range st.Layers {
		if _, ok := m.Layers[l]; !ok {
			return fmt.Errorf("layer %q is not defined", l)
		}
	}
	for _, mv := range st.Moves {
		if !cleanRel(mv.From) || !cleanRel(mv.To) || mv.From == mv.To {
			return fmt.Errorf("move %q to %q needs two distinct clean relative paths", mv.From, mv.To)
		}
	}
	return nil
}

// cleanRel reports whether p is a clean, relative repo path inside the
// repository.
func cleanRel(p string) bool {
	return p != "" && path.Clean(p) == p && !strings.HasPrefix(p, "/") && p != ".." && !strings.HasPrefix(p, "../")
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
		for _, p := range sortedKeys(files) {
			data, err := os.ReadFile(filepath.Join(dir, "records", filepath.FromSlash(files[p])))
			if err != nil {
				return nil, fmt.Errorf("scenario: layer %q: %w", l, err)
			}
			out[p] = string(data)
		}
	}
	return out, nil
}

// BaseFiles returns the repo files the base steps leave: each step's
// layers written and its moves applied, in order.
func (m *Manifest) BaseFiles(dir string) (map[string]string, error) {
	out := map[string]string{}
	for _, st := range m.Base {
		files, err := m.Files(dir, st.Layers...)
		if err != nil {
			return nil, err
		}
		for p, c := range files {
			out[p] = c
		}
		for _, mv := range st.Moves {
			for _, p := range sortedKeys(out) {
				if p == mv.From || strings.HasPrefix(p, mv.From+"/") {
					out[mv.To+strings.TrimPrefix(p, mv.From)] = out[p]
					delete(out, p)
				}
			}
		}
	}
	return out, nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
