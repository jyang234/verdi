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

// diffGitPrefix opens a git unified-diff section; the rest of that line is
// the section's two path arms ("a/<p1> b/<p2>"), which resolveDiffGitArms
// splits. devNull is git's placeholder for the absent side of an added or
// deleted file and never names a repo path.
const (
	diffGitPrefix = "diff --git "
	devNull       = "/dev/null"
)

// minusPathRe, plusPathRe, and the rename/copy patterns are the
// unambiguous, single-path header lines inside a section: each captures
// exactly one path, so a filename containing " b/" cannot be mis-split the
// way the two-armed "diff --git" line can. They are authoritative for
// their section; the "diff --git" line's arms are resolved separately
// (resolveDiffGitArms) and contribute in addition to them.
var (
	minusPathRe  = regexp.MustCompile(`^--- a/(.*)$`)
	plusPathRe   = regexp.MustCompile(`^\+\+\+ b/(.*)$`)
	renameFromRe = regexp.MustCompile(`^rename from (.*)$`)
	renameToRe   = regexp.MustCompile(`^rename to (.*)$`)
	copyFromRe   = regexp.MustCompile(`^copy from (.*)$`)
	copyToRe     = regexp.MustCompile(`^copy to (.*)$`)
)

// patchSection is one "diff --git" section: the raw two-armed remainder of
// its header line, and every path its unambiguous single-path lines name,
// in file order.
type patchSection struct {
	arms     string
	explicit []string
}

// resolveDiffGitArms resolves the "a/<p1> b/<p2>" remainder of a
// "diff --git" line to the single path it names, or "" when the text alone
// cannot say which split is meant.
//
// git writes both arms as the SAME path for everything except a
// rename/copy, so the split whose two arms are equal is the intended one.
// At most one split can ever satisfy that (the two arms are equal only
// when they are also of equal length, which fixes the split point), so the
// resolution is deterministic: exactly one equal split resolves; zero
// equal splits — a rename/copy, or a genuinely ambiguous header — leaves
// the section to its unambiguous single-path lines.
func resolveDiffGitArms(arms string) string {
	body, ok := strings.CutPrefix(arms, "a/")
	if !ok {
		return ""
	}
	const sep = " b/"
	resolved := ""
	found := 0
	for i := 0; i+len(sep) <= len(body); i++ {
		if body[i:i+len(sep)] != sep {
			continue
		}
		if body[:i] == body[i+len(sep):] {
			resolved = body[:i]
			found++
		}
	}
	if found != 1 {
		return ""
	}
	return resolved
}

// splitPatchSections splits patchBytes into its "diff --git" sections,
// collecting each section's unambiguous single-path header lines. It
// requires at least one section; content with none is unparseable and
// fails closed.
func splitPatchSections(patchBytes []byte) ([]patchSection, error) {
	var sections []patchSection
	for _, line := range strings.Split(string(patchBytes), "\n") {
		if arms, ok := strings.CutPrefix(line, diffGitPrefix); ok {
			sections = append(sections, patchSection{arms: arms})
			continue
		}
		if len(sections) == 0 {
			continue
		}
		cur := &sections[len(sections)-1]
		for _, re := range []*regexp.Regexp{minusPathRe, plusPathRe, renameFromRe, renameToRe, copyFromRe, copyToRe} {
			if m := re.FindStringSubmatch(line); m != nil {
				cur.explicit = append(cur.explicit, m[1])
				break
			}
		}
	}
	if len(sections) == 0 {
		return nil, fmt.Errorf("experiment: patch has no parseable %q section", "diff --git")
	}
	return sections, nil
}

// parsePatchPaths strictly parses patchBytes as a sequence of git unified
// diffs and returns the set of repo-relative paths any section touches, in
// first-seen order.
//
// It fails closed on anything it cannot read as one exact set of paths:
// content with no "diff --git" section; a section that names no path at
// all (an unresolvable header with no unambiguous single-path lines); and
// any path that is not already in canonical repo-relative form
// (ValidateRepoRelativePath). A patch whose paths would have to be
// normalized before they could be compared to a protected input is not
// canonical, and is rejected outright rather than cleaned and accepted —
// otherwise the same file would have several accepted spellings and only
// one of them would be checked.
func parsePatchPaths(patchBytes []byte) ([]string, error) {
	sections, err := splitPatchSections(patchBytes)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var paths []string
	add := func(p string) error {
		if !namesRepoPath(p) || seen[p] {
			return nil
		}
		if err := ValidateRepoRelativePath(p); err != nil {
			return fmt.Errorf("experiment: patch names a non-canonical path: %w", err)
		}
		seen[p] = true
		paths = append(paths, p)
		return nil
	}

	for _, s := range sections {
		resolved := resolveDiffGitArms(s.arms)
		named := namesRepoPath(resolved)
		for _, p := range s.explicit {
			if namesRepoPath(p) {
				named = true
			}
		}
		if !named {
			return nil, fmt.Errorf("experiment: patch section %q names no unambiguous path", diffGitPrefix+s.arms)
		}
		if err := add(resolved); err != nil {
			return nil, err
		}
		for _, p := range s.explicit {
			if err := add(p); err != nil {
				return nil, err
			}
		}
	}
	return paths, nil
}

// namesRepoPath reports whether p is a candidate repo path at all, as
// opposed to an unresolved arm ("") or git's /dev/null placeholder.
func namesRepoPath(p string) bool { return p != "" && p != devNull }

// pathTouchesPrefix reports whether changed is prefix itself or lies
// under prefix at a path-segment boundary — "internal/cache2" must NOT
// match protected prefix "internal/cache". Both arguments are already in
// canonical repo-relative form (ValidateRepoRelativePath), so the literal
// comparison here cannot be evaded by an equivalent spelling.
func pathTouchesPrefix(changed, prefix string) bool {
	return changed == prefix || strings.HasPrefix(changed, prefix+"/")
}

// ValidateCandidatePatch checks that patchBytes is exactly the registered
// candidate's patch — its sha256 digest matches the definition's
// registered digest for candidateID — and that none of its changed paths
// touch a protected comparison input: any def.ProtectedPaths entry, the
// evaluator's own executable when it lives in this repository
// (EvaluatorRepoPath — an absolute or PATH-resolved argv[0] names no repo
// path and is therefore not a protected input), or any path under the
// experiment's own directory.
//
// experimentDir is REQUIRED and is the experiment directory's canonical
// repo-relative path — the same coordinate system protected_paths and the
// changed paths use. An empty or non-repo-relative experimentDir is a hard
// error: a caller with no directory context cannot be served by silently
// dropping one protected input, because the resulting "clean" verdict
// would be indistinguishable from one that actually checked it.
func ValidateCandidatePatch(def Definition, candidateID string, patchBytes []byte, experimentDir string) error {
	if err := ValidateRepoRelativePath(experimentDir); err != nil {
		return fmt.Errorf("experiment: candidate %q: experiment directory: %w", candidateID, err)
	}

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
		// A validated definition can only carry an absolute, bare-command,
		// or canonical repo-relative executable, so this never rejects a
		// registration Validate already accepted — but an unvalidated def
		// must fail here rather than quietly lose one protected input.
		evaluatorPath, err := EvaluatorRepoPath(def.Evaluator.Argv[0])
		if err != nil {
			return fmt.Errorf("experiment: candidate %q: %w", candidateID, err)
		}
		if evaluatorPath != "" {
			protected = append(protected, evaluatorPath)
		}
	}
	protected = append(protected, experimentDir)

	for _, path := range changed {
		for _, prefix := range protected {
			if pathTouchesPrefix(path, prefix) {
				return fmt.Errorf("experiment: candidate %q: patch touches protected path %q (protected prefix %q)", candidateID, path, prefix)
			}
		}
	}
	return nil
}
