package experiment

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// Candidate is one registered candidate patch entry inside a Definition's
// candidates list (AC-1).
type Candidate struct {
	ID     string `yaml:"id" json:"id"`
	Patch  string `yaml:"patch" json:"patch"`
	Digest string `yaml:"digest" json:"digest"`
	Base   string `yaml:"base" json:"base"`
}

// Validate checks c's own grammar and its relationship to the enclosing
// definition's base_commit: the patch path must be exactly
// "candidates/<id>.patch", and the candidate's base must equal baseCommit
// — a differing base names both values so the mismatch is legible.
func (c Candidate) Validate(baseCommit string) error {
	if err := ValidateID(c.ID); err != nil {
		return fmt.Errorf("experiment: candidate id: %w", err)
	}
	wantPatch := "candidates/" + c.ID + ".patch"
	if c.Patch != wantPatch {
		return fmt.Errorf("experiment: candidate %q: patch %q, want %q", c.ID, c.Patch, wantPatch)
	}
	if err := ValidateDigest(c.Digest); err != nil {
		return fmt.Errorf("experiment: candidate %q: digest: %w", c.ID, err)
	}
	if err := ValidateCommit(c.Base); err != nil {
		return fmt.Errorf("experiment: candidate %q: base: %w", c.ID, err)
	}
	if c.Base != baseCommit {
		return fmt.Errorf("experiment: candidate %q: base %q does not match definition base_commit %q", c.ID, c.Base, baseCommit)
	}
	return nil
}

// sha256Digest returns data's content address in the shared "sha256:"+hex
// grammar (ValidateDigest) — the raw-bytes counterpart to
// internal/canonjson.Digest for content this package hashes directly
// (patch bytes) rather than through canonical JSON.
func sha256Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// diffGitHeaderRe, changedPathRe, and rename/copy path patterns are the
// git unified-diff header lines ValidateCandidatePatch parses to find
// every path a patch touches. Each captures at most one path per line, so
// filenames containing " b/" cannot be split ambiguously the way
// diffGitHeaderRe's two-path line can — that line is only ever used as a
// fallback for sections whose content is otherwise unchanged (a pure
// rename/copy with no diff hunks emits no "---"/"+++" lines at all).
var (
	diffGitHeaderRe = regexp.MustCompile(`^diff --git a/(.*) b/(.*)$`)
	minusPathRe     = regexp.MustCompile(`^--- a/(.*)$`)
	plusPathRe      = regexp.MustCompile(`^\+\+\+ b/(.*)$`)
	renameFromRe    = regexp.MustCompile(`^rename from (.*)$`)
	renameToRe      = regexp.MustCompile(`^rename to (.*)$`)
	copyFromRe      = regexp.MustCompile(`^copy from (.*)$`)
	copyToRe        = regexp.MustCompile(`^copy to (.*)$`)
)

// parsePatchPaths strictly parses patchBytes as a sequence of git unified
// diffs and returns the set of repo-relative paths any section touches,
// in first-seen order. It requires at least one "diff --git" header;
// content with none is unparseable and fails closed.
func parsePatchPaths(patchBytes []byte) ([]string, error) {
	seen := make(map[string]bool)
	var paths []string
	add := func(p string) {
		if p == "" || p == "/dev/null" || seen[p] {
			return
		}
		seen[p] = true
		paths = append(paths, p)
	}

	sections := 0
	for _, line := range strings.Split(string(patchBytes), "\n") {
		switch {
		case diffGitHeaderRe.MatchString(line):
			m := diffGitHeaderRe.FindStringSubmatch(line)
			sections++
			add(m[1])
			add(m[2])
		case minusPathRe.MatchString(line):
			add(minusPathRe.FindStringSubmatch(line)[1])
		case plusPathRe.MatchString(line):
			add(plusPathRe.FindStringSubmatch(line)[1])
		case renameFromRe.MatchString(line):
			add(renameFromRe.FindStringSubmatch(line)[1])
		case renameToRe.MatchString(line):
			add(renameToRe.FindStringSubmatch(line)[1])
		case copyFromRe.MatchString(line):
			add(copyFromRe.FindStringSubmatch(line)[1])
		case copyToRe.MatchString(line):
			add(copyToRe.FindStringSubmatch(line)[1])
		}
	}
	if sections == 0 {
		return nil, fmt.Errorf("experiment: patch has no parseable %q section", "diff --git")
	}
	return paths, nil
}

// pathTouchesPrefix reports whether changed is prefix itself or lies
// under prefix at a path-segment boundary — "internal/cache2" must NOT
// match protected prefix "internal/cache".
func pathTouchesPrefix(changed, prefix string) bool {
	return changed == prefix || strings.HasPrefix(changed, prefix+"/")
}

// ValidateCandidatePatch checks that patchBytes is exactly the registered
// candidate's patch — its sha256 digest matches the definition's
// registered digest for candidateID — and that none of its changed paths
// touch a protected comparison input: any def.ProtectedPaths entry, the
// evaluator's own repo-relative executable (def.Evaluator.Argv[0] with a
// leading "./" stripped), or any path under the experiment's own
// directory (experimentDir, repo-relative; pass "" when the caller has no
// meaningful directory context — the check is then skipped for that one
// input only, every other protected input still applies).
func ValidateCandidatePatch(def Definition, candidateID string, patchBytes []byte, experimentDir string) error {
	var candidate *Candidate
	for i := range def.Candidates {
		if def.Candidates[i].ID == candidateID {
			candidate = &def.Candidates[i]
			break
		}
	}
	if candidate == nil {
		return fmt.Errorf("experiment: candidate %q is not registered in this definition", candidateID)
	}

	if got := sha256Digest(patchBytes); got != candidate.Digest {
		return fmt.Errorf("experiment: candidate %q: patch digest %q does not match registered digest %q", candidateID, got, candidate.Digest)
	}

	changed, err := parsePatchPaths(patchBytes)
	if err != nil {
		return fmt.Errorf("experiment: candidate %q: %w", candidateID, err)
	}

	protected := make([]string, 0, len(def.ProtectedPaths)+2)
	protected = append(protected, def.ProtectedPaths...)
	if len(def.Evaluator.Argv) > 0 {
		trimmed := strings.TrimPrefix(def.Evaluator.Argv[0], "./")
		if ValidateRepoRelativePath(trimmed) == nil {
			protected = append(protected, trimmed)
		}
	}
	if experimentDir != "" {
		protected = append(protected, experimentDir)
	}

	for _, path := range changed {
		for _, prefix := range protected {
			if pathTouchesPrefix(path, prefix) {
				return fmt.Errorf("experiment: candidate %q: patch touches protected path %q (protected prefix %q)", candidateID, path, prefix)
			}
		}
	}
	return nil
}
