package repositoryfacts

import (
	"context"
	"fmt"
	"strings"
)

// CIRefFact is the CI provider's ref for this run, accepted only as a
// validated branch (SI-251, SI-257; plan R-PB-3). It rides on Snapshot
// beside Facts, never inside it: Facts's wire shape is pinned by the
// journey record and the conflict report, and Facts keeps recording the
// physical checkout (a detached HEAD stays detached there).
//
// Known is true only when exactly one provider's run was detected, its ref
// names a branch that is valid under git's check-ref-format rules, and the
// exact remote-tracking ref refs/remotes/origin/<Name> resolves to HEAD.
// Provider and Name are set only when Known. Reason is one of the closed
// CIRefReason* codes when a CI run was detected but the ref could not be
// accepted, and "" outside CI (and whenever Known).
type CIRefFact struct {
	Known    bool   `json:"known"`
	Provider string `json:"provider,omitempty"` // "github" | "gitlab" when Known
	Name     string `json:"name,omitempty"`     // the branch name when Known
	Reason   string `json:"reason,omitempty"`   // closed code when not Known and a CI run was detected; "" outside CI
}

// The closed CIRefFact.Provider values.
const (
	CIProviderGitHub = "github"
	CIProviderGitLab = "gitlab"
)

var validCIProvider = map[string]bool{
	CIProviderGitHub: true,
	CIProviderGitLab: true,
}

// The closed CIRefFact.Reason codes: one per cause a detected CI run's ref
// is not accepted as the validated branch.
const (
	// CIRefReasonProvidersAmbiguous: both GITHUB_ACTIONS and GITLAB_CI
	// are "true", so no single provider's ref can be trusted.
	CIRefReasonProvidersAmbiguous = "ci-ref-providers-ambiguous"
	// CIRefReasonRefTypeNotBranch: a GitHub run whose GITHUB_REF_TYPE is
	// not "branch" (a tag run, or the variable is absent).
	CIRefReasonRefTypeNotBranch = "ci-ref-ref-type-not-branch"
	// CIRefReasonTagPipeline: a GitLab pipeline with CI_COMMIT_TAG set.
	CIRefReasonTagPipeline = "ci-ref-tag-pipeline"
	// CIRefReasonNameMissing: the provider's ref-name variable
	// (GITHUB_REF_NAME, GitLab CI_COMMIT_REF_NAME) is empty.
	CIRefReasonNameMissing = "ci-ref-name-missing"
	// CIRefReasonBranchMismatch: a GitLab pipeline whose
	// CI_COMMIT_REF_NAME differs from CI_COMMIT_BRANCH (including an
	// absent CI_COMMIT_BRANCH, as in a merge-request pipeline).
	CIRefReasonBranchMismatch = "ci-ref-branch-mismatch"
	// CIRefReasonNameInvalid: the ref name is not a valid branch name
	// under git's check-ref-format rules; it never reaches rev-parse.
	CIRefReasonNameInvalid = "ci-ref-name-invalid"
	// CIRefReasonHeadUnresolved: HEAD itself could not be resolved, so
	// there is nothing to compare the remote-tracking head against.
	CIRefReasonHeadUnresolved = "ci-ref-head-unresolved"
	// CIRefReasonRemoteTrackingUnresolved: refs/remotes/origin/<name>
	// does not resolve (absent from this checkout, or a git error).
	CIRefReasonRemoteTrackingUnresolved = "ci-ref-remote-tracking-unresolved"
	// CIRefReasonRemoteTrackingNotHead: refs/remotes/origin/<name>
	// resolves, but to a commit other than HEAD.
	CIRefReasonRemoteTrackingNotHead = "ci-ref-remote-tracking-not-head"
)

var validCIRefReason = map[string]bool{
	CIRefReasonProvidersAmbiguous:       true,
	CIRefReasonRefTypeNotBranch:         true,
	CIRefReasonTagPipeline:              true,
	CIRefReasonNameMissing:              true,
	CIRefReasonBranchMismatch:           true,
	CIRefReasonNameInvalid:              true,
	CIRefReasonHeadUnresolved:           true,
	CIRefReasonRemoteTrackingUnresolved: true,
	CIRefReasonRemoteTrackingNotHead:    true,
}

// Validate reports whether f is internally consistent: a known fact names
// a closed provider and a valid branch and carries no reason; an unknown
// fact carries no provider or name, and its reason is "" or closed.
func (f CIRefFact) Validate() error {
	if f.Known {
		if !validCIProvider[f.Provider] {
			return fmt.Errorf("repositoryfacts: known CI ref has unknown provider %q", f.Provider)
		}
		if !validBranchName(f.Name) {
			return fmt.Errorf("repositoryfacts: known CI ref name %q is not a valid branch name", f.Name)
		}
		if f.Reason != "" {
			return fmt.Errorf("repositoryfacts: known CI ref carries reason %q", f.Reason)
		}
		return nil
	}
	if f.Provider != "" || f.Name != "" {
		return fmt.Errorf("repositoryfacts: unknown CI ref carries provider %q or name %q", f.Provider, f.Name)
	}
	if f.Reason != "" && !validCIRefReason[f.Reason] {
		return fmt.Errorf("repositoryfacts: unknown CI ref reason %q", f.Reason)
	}
	return nil
}

// The provider environment variables gatherCIRef reads. GITHUB_HEAD_REF
// (a pull-request run's head branch) is deliberately absent: SI-257 never
// consults it.
const (
	envGitHubActions   = "GITHUB_ACTIONS"
	envGitHubRefName   = "GITHUB_REF_NAME"
	envGitHubRefType   = "GITHUB_REF_TYPE"
	envGitLabCI        = "GITLAB_CI"
	envGitLabRefName   = "CI_COMMIT_REF_NAME"
	envGitLabBranch    = "CI_COMMIT_BRANCH"
	envGitLabCommitTag = "CI_COMMIT_TAG"
)

// remoteTrackingPrefix is the one remote-tracking namespace a CI ref name
// is resolved in: the full refname refs/remotes/origin/<name>, which git's
// rev-parse matches as that exact ref before any other lookup rule.
const remoteTrackingPrefix = "refs/remotes/origin/"

// gatherCIRef computes the CI-ref fact (SI-257). A run is a CI run for
// this purpose only when exactly one of GITHUB_ACTIONS and GITLAB_CI is
// "true"; neither is the outside-CI zero value, and both is refused. The
// provider's ref must name a branch (GitHub: GITHUB_REF_TYPE is "branch";
// GitLab: no CI_COMMIT_TAG, and CI_COMMIT_REF_NAME equals
// CI_COMMIT_BRANCH), the name must pass validBranchName before anything
// reaches git, and refs/remotes/origin/<name> must resolve to head. Every
// failure is Known == false with its own closed reason; a git failure
// resolving the remote-tracking ref is such a failure, never an error.
func gatherCIRef(ctx context.Context, git GitReader, getenv EnvReader, root string, head StringFact) CIRefFact {
	github := getenv(envGitHubActions) == "true"
	gitlab := getenv(envGitLabCI) == "true"
	var provider, name string
	switch {
	case github && gitlab:
		return CIRefFact{Reason: CIRefReasonProvidersAmbiguous}
	case github:
		provider = CIProviderGitHub
		if getenv(envGitHubRefType) != "branch" {
			return CIRefFact{Reason: CIRefReasonRefTypeNotBranch}
		}
		if name = getenv(envGitHubRefName); name == "" {
			return CIRefFact{Reason: CIRefReasonNameMissing}
		}
	case gitlab:
		provider = CIProviderGitLab
		if getenv(envGitLabCommitTag) != "" {
			return CIRefFact{Reason: CIRefReasonTagPipeline}
		}
		if name = getenv(envGitLabRefName); name == "" {
			return CIRefFact{Reason: CIRefReasonNameMissing}
		}
		if name != getenv(envGitLabBranch) {
			return CIRefFact{Reason: CIRefReasonBranchMismatch}
		}
	default:
		return CIRefFact{}
	}
	if !validBranchName(name) {
		return CIRefFact{Reason: CIRefReasonNameInvalid}
	}
	if !head.Known {
		return CIRefFact{Reason: CIRefReasonHeadUnresolved}
	}
	tracking, err := git.RevParse(ctx, root, remoteTrackingPrefix+name)
	if err != nil {
		return CIRefFact{Reason: CIRefReasonRemoteTrackingUnresolved}
	}
	if tracking != head.Value {
		return CIRefFact{Reason: CIRefReasonRemoteTrackingNotHead}
	}
	return CIRefFact{Known: true, Provider: provider, Name: name}
}

// BranchBeingClosed is SI-257's one reading of the branch a lifecycle
// operation on this checkout acts for: the checked-out branch when there
// is one; otherwise — only when the checkout is detached — the validated
// CI ref; otherwise unknown. A branch that could not be resolved for any
// reason other than a detached HEAD stays unknown even with a known CI
// ref: the CI ref stands in for a detached checkout's missing name, never
// for a failed read. A CI ref that does not validate is never used.
func (s Snapshot) BranchBeingClosed() StringFact {
	if s.Facts.Branch.Known {
		return s.Facts.Branch
	}
	if !s.hasDisclosure(DisclosureBranchDetached) || !s.CIRef.Known || s.CIRef.Validate() != nil {
		return StringFact{}
	}
	return StringFact{Known: true, Value: s.CIRef.Name}
}

func (s Snapshot) hasDisclosure(code DisclosureCode) bool {
	for _, c := range s.Disclosures {
		if c == code {
			return true
		}
	}
	return false
}

// validBranchName reports whether name is a valid branch name under git's
// check-ref-format rules for refs/heads/<name> (git-check-ref-format(1),
// plus --branch's refusal of a leading "-" and of "HEAD"): no component
// begins with "." or ends with ".lock"; no ".."; no control character,
// space, "~", "^", ":", "?", "*", "[" or "\"; no leading or trailing "/"
// and no empty component; no trailing "."; no "@{"; and not "@". It is a
// pure function of name: nothing here consults a repository, so a name
// such as "main~1" is refused before any rev-parse could expand it.
func validBranchName(name string) bool {
	if name == "" || name == "@" || name == "HEAD" {
		return false
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".") {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "@{") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
		switch r {
		case ' ', '~', '^', ':', '?', '*', '[', '\\':
			return false
		}
	}
	for _, component := range strings.Split(name, "/") {
		if component == "" || strings.HasPrefix(component, ".") || strings.HasSuffix(component, ".lock") {
			return false
		}
	}
	return true
}
