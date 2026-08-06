// Package experiment implements the comparative-spike-experiments (CSE)
// artifact schemas: strict decode and validation for every versioned CSE
// artifact (spec/comparative-spike-experiments AC-1, AC-2, DC-2..DC-5,
// DC-12..DC-14, CO-1..CO-4, CO-7), canonical normalization and definition
// identity, derived lifecycle state, and candidate/observation integrity
// checks. It owns schema shape only: no decision engine, rendering,
// execution, CLI, or MCP surface lives here.
//
// Every YAML artifact decodes through internal/artifact.DecodeStrict and
// every JSON artifact through internal/artifact.DecodeStrictJSON — the
// module's only strict-decode seams — so unknown fields, unknown schema
// versions, and the restricted YAML dialect (no anchors, aliases, or
// custom tags) fail closed everywhere in this package. Two package-local
// guards are LAYERED over that seam without replacing it (strictdecode.go):
// repeated JSON object keys and trailing YAML documents, both of which the
// shared seam currently lets through. Canonical JSON and content digests go
// exclusively through internal/canonjson.
package experiment

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode"
)

// digestRe is the shared sha256 content-digest grammar every CSE artifact
// digest field uses: "sha256:" followed by 64 lowercase hex characters
// (internal/canonjson.Digest's own output shape).
var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// idRe is the shared CSE identifier grammar: one or more lowercase
// alphanumeric segments joined by single hyphens, matching experiment,
// candidate, guard, metric, workload, fixture, contract, and observer ids,
// run identity, and retention policy id.
var idRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// commitRe is the shared Git commit grammar for base_commit and candidate
// base: exactly 40 lowercase hex characters.
var commitRe = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ValidateDigest checks s against the shared sha256 digest grammar
// (spec/comparative-spike-experiments "Digest" grammar). Unknown or
// malformed digests fail closed.
func ValidateDigest(s string) error {
	if !digestRe.MatchString(s) {
		return fmt.Errorf("experiment: invalid digest %q: want sha256:<64 lowercase hex>", s)
	}
	return nil
}

// ValidateID checks id against the shared CSE identifier grammar.
func ValidateID(id string) error {
	if !idRe.MatchString(id) {
		return fmt.Errorf("experiment: invalid id %q: want lowercase alphanumeric segments joined by single hyphens", id)
	}
	return nil
}

// ValidateCommit checks s against the shared Git commit grammar: exactly
// 40 lowercase hex characters.
func ValidateCommit(s string) error {
	if !commitRe.MatchString(s) {
		return fmt.Errorf("experiment: invalid commit %q: want exactly 40 lowercase hex characters", s)
	}
	return nil
}

// ValidateUnit checks unit is nonempty and contains no whitespace or
// control characters.
func ValidateUnit(unit string) error {
	if unit == "" {
		return fmt.Errorf("experiment: unit must be nonempty")
	}
	for _, r := range unit {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("experiment: invalid unit %q: must contain no whitespace or control characters", unit)
		}
	}
	return nil
}

// ValidateRepoRelativePath checks p is a repo-relative path in canonical
// form: nonempty, no leading "/", and no empty, ".", or ".." segment.
//
// It is the ONE grammar both sides of the protected-input comparison pass
// through, which is what makes that comparison's literal matching sound:
// on the protected side, every protected_paths entry, the experiment
// directory, and a repo-relative evaluator executable (EvaluatorRepoPath,
// which rejects a non-canonical spelling at registration rather than
// dropping the input); on the other side, every path a candidate patch
// names. Since neither side can hold a non-canonical spelling, no path can
// name a protected input in a form the match does not recognize.
//
// An evaluator executable that is absolute or a bare PATH-resolved command
// names no path in this repository at all, and is deliberately outside
// both sides of the comparison rather than an unrecognized spelling.
//
// A path needing normalization is rejected rather than normalized: "." and
// ".." segments are refused outright, so the same file never has two
// accepted spellings.
func ValidateRepoRelativePath(p string) error {
	if p == "" {
		return fmt.Errorf("experiment: repo-relative path must be nonempty")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("experiment: repo-relative path %q must not have a leading '/'", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" {
			return fmt.Errorf("experiment: repo-relative path %q has an empty segment", p)
		}
		if seg == "." || seg == ".." {
			return fmt.Errorf("experiment: repo-relative path %q must not contain a %q segment", p, seg)
		}
	}
	return nil
}

// validateFinite checks v is a real number: neither NaN nor an infinity of
// either sign, naming field in any error. Every numeric bound in this
// package goes through it before any ordering comparison, because NaN
// silently answers false to every such comparison ("v <= 0" does not
// reject NaN) and an infinity satisfies "strictly positive" while making
// the threshold it states unreachable — a registered decision contract
// that can never be met is not a contract.
//
// The live path is YAML, whose float grammar spells these values (".nan",
// ".inf", "-.inf"). JSON has no literal for either, so the JSON-side calls
// are defense in depth: a numeric literal large enough to overflow is
// already rejected by json.Number.Float64's range error.
func validateFinite(field string, v float64) error {
	if math.IsNaN(v) {
		return fmt.Errorf("experiment: %s must be a finite number, got NaN", field)
	}
	if math.IsInf(v, 0) {
		return fmt.Errorf("experiment: %s must be a finite number, got %v", field, v)
	}
	return nil
}

// nonemptyString checks that field carries a nonempty value, returning an
// error naming field when it does not. It is the shared helper for every
// "nonempty string" grammar rule in this package that is not itself an ID,
// digest, commit, or unit.
func nonemptyString(field, value string) error {
	if value == "" {
		return fmt.Errorf("experiment: %s must be nonempty", field)
	}
	return nil
}
