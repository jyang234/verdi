package policyartifact

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// Phase names the three lifecycle phases a constitution scope may narrow
// to (spec/context-integrity-v2 AC-2 names design, authoritative build,
// and independent review as the phase universe; the compiler that
// consumes them is Wave 3 work — this package owns only the closed
// vocabulary). The set is Verdi-owned and closed: an unknown phase fails
// decode (co-2), it is never a registration surface.
const (
	PhaseDesign = "design"
	PhaseBuild  = "build"
	PhaseReview = "review"
)

var knownPhases = map[string]bool{
	PhaseDesign: true,
	PhaseBuild:  true,
	PhaseReview: true,
}

// kebabRe matches the kebab-case identifier grammar policy names, claim
// ids, subjects, environments, and adapter ids share (02 §Identity's
// name shape, reused rather than re-invented).
var kebabRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// Scope is the applicability boundary every constitution artifact and
// claim carries (AC-1's kernel scope field). Each dimension is a semantic
// set; the EXPLICIT empty set means unconstrained on that dimension —
// omission is invalid (SI-18's explicit-presence posture: rigor may be
// zero but never unstated). Scope comparison semantics (overlap, subset,
// unknown) beyond the narrow-only refinement check are Wave 3 work.
type Scope struct {
	Phases       []string `yaml:"phases" json:"phases"`
	Environments []string `yaml:"environments" json:"environments"`
	Paths        []string `yaml:"paths" json:"paths"`
	Refs         []string `yaml:"refs" json:"refs"`
}

// scopeDoc is Scope's strict decode target: pointers distinguish an
// omitted dimension (invalid) from an explicitly empty one (universal).
type scopeDoc struct {
	Phases       *[]string `yaml:"phases"`
	Environments *[]string `yaml:"environments"`
	Paths        *[]string `yaml:"paths"`
	Refs         *[]string `yaml:"refs"`
}

func (d scopeDoc) toScope(field string) (Scope, error) {
	missing := func(dim string) error {
		return fmt.Errorf("policyartifact: %s.%s is missing: every scope dimension is mandatory (an explicitly empty set is [])", field, dim)
	}
	switch {
	case d.Phases == nil:
		return Scope{}, missing("phases")
	case d.Environments == nil:
		return Scope{}, missing("environments")
	case d.Paths == nil:
		return Scope{}, missing("paths")
	case d.Refs == nil:
		return Scope{}, missing("refs")
	}
	return Scope{
		Phases:       emptyIfNil(*d.Phases),
		Environments: emptyIfNil(*d.Environments),
		Paths:        emptyIfNil(*d.Paths),
		Refs:         emptyIfNil(*d.Refs),
	}, nil
}

// emptyIfNil normalizes a present-but-nil YAML sequence to the explicit
// empty set so equivalent artifacts share one canonical encoding.
func emptyIfNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// Validate checks every dimension's presence and member grammar.
func (s Scope) Validate() error {
	if s.Phases == nil {
		return fmt.Errorf("policyartifact: scope phases dimension is missing")
	}
	if s.Environments == nil {
		return fmt.Errorf("policyartifact: scope environments dimension is missing")
	}
	if s.Paths == nil {
		return fmt.Errorf("policyartifact: scope paths dimension is missing")
	}
	if s.Refs == nil {
		return fmt.Errorf("policyartifact: scope refs dimension is missing")
	}
	if err := uniqueSet("scope.phases", s.Phases, func(p string) error {
		if !knownPhases[p] {
			return fmt.Errorf("unknown phase %q (known: design, build, review)", p)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := uniqueSet("scope.environments", s.Environments, func(e string) error {
		if e == "" {
			return fmt.Errorf("empty environment")
		}
		if !kebabRe.MatchString(e) {
			return fmt.Errorf("environment %q must be kebab-case", e)
		}
		return nil
	}); err != nil {
		return err
	}
	if err := uniqueSet("scope.paths", s.Paths, validateRelPath); err != nil {
		return err
	}
	return uniqueSet("scope.refs", s.Refs, func(r string) error {
		if _, err := artifact.ParseRef(r); err != nil {
			return fmt.Errorf("invalid artifact ref: %w", err)
		}
		return nil
	})
}

// normalizeScope sorts every dimension: each is a semantic set, never
// ordered evidence.
func normalizeScope(s *Scope) {
	sort.Strings(s.Phases)
	sort.Strings(s.Environments)
	sort.Strings(s.Paths)
	sort.Strings(s.Refs)
}

// validateRelPath enforces the repo-relative path grammar shared by scope
// paths, claim path operands, and adapter projection paths: relative,
// forward-slash, no escape, no backslash (co-3's no-local-absolute-paths
// posture and the store's own path-escape discipline).
func validateRelPath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("path %q is absolute; only repo-relative paths are permitted", p)
	}
	if strings.Contains(p, `\`) {
		return fmt.Errorf("path %q contains a backslash; use forward slashes", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return fmt.Errorf("path %q escapes the repository root (.. segment)", p)
		}
	}
	return nil
}

// uniqueSet validates each member of a semantic set and rejects
// duplicates.
func uniqueSet(field string, members []string, check func(string) error) error {
	seen := make(map[string]bool, len(members))
	for _, m := range members {
		if err := check(m); err != nil {
			return fmt.Errorf("policyartifact: %s: %w", field, err)
		}
		if seen[m] {
			return fmt.Errorf("policyartifact: %s: duplicate entry %q", field, m)
		}
		seen[m] = true
	}
	return nil
}
