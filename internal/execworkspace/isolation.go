package execworkspace

// Isolation-profile construction for spec/execution-workspace
// §Isolation-control application and §Execution-grant enforcement
// (controller decisions AD-5/AD-9). This is MECHANISM ONLY, and
// construction-only: this package builds a Profile a consumer applies to
// its own process launch — it never runs the consumer's process itself
// (spec §Isolation-control application: "The component constructs the
// isolated profile ... as MECHANISM only").
//
// AD-9 fixes v0's mechanically appliable kinds: process execution (an
// argv0 allowlist recorded on the profile) and timeouts (a deadline
// recorded on the profile, applied by the consumer to its own context),
// plus the base profile every grant set gets regardless of which grants it
// names — a clean environment with controlled HOME/XDG discovery. network,
// path-read, path-write, and resource-ceilings have no v0 mechanism.
//
// AD-5 makes every granted control REQUIRED: a control the component
// cannot apply is not a silent partial success. BuildProfile always
// returns its EnforcementReport (the facts survive), but when any row is
// could-not-apply it ALSO returns a non-nil error — reusing this package's
// existing *OperationalError type from materialize.go, which is why the
// rendered message below carries that type's own fixed
// "execworkspace: materialize: ..." prefix even though this failure has
// nothing to do with materialization: OperationalError is this package's
// one shared retryable-disclosed-failure type, and this commit's write set
// does not touch materialize.go to give it a second, more general
// constructor. The Op and Err fields still carry the correct, specific
// content (which grant kinds could not be applied), and errors.As reaches
// the typed value through any wrap.
//
// Grounding for the fail-closed posture (spec §Isolation-control
// application, quoted there): CI dc-10 — "authoritative launch fails when
// isolation cannot be proven; Verdi may offer a visibly new advisory run,
// but no adapter or harness may silently reinterpret the failed launch as
// authoritative" — and CSE's operational-error clause — "An evaluator
// crash, malformed response, protected-input mismatch, missing round,
// environment mismatch, or unavailable required isolation control
// invalidates the run and returns an operational error." A missing
// required control must never be silently reinterpreted as authoritative
// execution.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// profileOwnedEnvKeys are the four environment keys BuildProfile itself
// sets from workspacePath. A declaredEnv pair naming one of these is
// rejected outright — never a silent override — so a caller can never
// smuggle a value into a key this package's own clean-environment contract
// owns.
var profileOwnedEnvKeys = map[string]bool{
	"HOME":            true,
	"XDG_CONFIG_HOME": true,
	"XDG_CACHE_HOME":  true,
	"TMPDIR":          true,
}

// Profile is a constructed, ready-to-use isolation profile. A consumer
// applies Env() and Timeout to its own process launch and its own context;
// this package never launches anything itself. The zero Profile has an
// empty (but non-panicking) Env() and a zero Timeout/AllowedArgv0s — only
// BuildProfile constructs a Profile actually backed by a workspace.
type Profile struct {
	env           map[string]string
	Timeout       time.Duration
	AllowedArgv0s []string
}

// Env renders the profile's environment as deterministic, sorted
// "KEY=VALUE" entries (the shape exec.Cmd.Env expects). NO inherited
// process environment ever appears here: every entry originates either
// from the four profile-owned keys or from the caller's own declaredEnv.
func (p Profile) Env() []string {
	entries := make([]string, 0, len(p.env))
	for k, v := range p.env {
		entries = append(entries, k+"="+v)
	}
	sort.Strings(entries)
	return entries
}

// EnforcementReportRow is one requested grant's applied/could-not-apply
// operational fact (spec §Execution-grant enforcement: "the component
// reports which grants it could and could not apply as operational
// facts").
type EnforcementReportRow struct {
	Kind    GrantKind
	Applied bool
	Reason  string
}

// EnforcementReport is BuildProfile's per-grant fact report: exactly one
// row per GrantKind present in the requested GrantSet, in deterministic
// GrantKind declaration order — never the input grant list's own order —
// so two GrantSets differing only in JSON array order produce
// byte-identical row ordering.
type EnforcementReport struct {
	Rows []EnforcementReportRow
}

// appliedReasons names the v0 mechanism actually used for each
// mechanically appliable kind (AD-9).
var appliedReasons = map[GrantKind]string{
	GrantProcessExecution: "applied: argv0 allowlist recorded on the profile (AD-9)",
	GrantTimeouts:         "applied: deadline recorded on the profile; the consumer applies it to its own context (AD-9)",
}

// couldNotApplyReasons names the missing v0 mechanism for each kind AD-9
// leaves unimplemented, cited in every could-not-apply row.
var couldNotApplyReasons = map[GrantKind]string{
	GrantNetwork:          "no v0 mechanism to enforce network policy (AD-9)",
	GrantPathRead:         "no v0 mechanism to enforce path-read scoping (AD-9)",
	GrantPathWrite:        "no v0 mechanism to enforce path-write scoping (AD-9)",
	GrantResourceCeilings: "no v0 mechanism to enforce resource ceilings (AD-9)",
}

// BuildProfile constructs the isolation Profile for workspacePath from
// grants and declaredEnv (spec §Isolation-control application,
// §Execution-grant enforcement; AD-5/AD-7/AD-9).
//
// Env(): NO inherited process environment. Exactly HOME=<workspacePath>/
// .home, XDG_CONFIG_HOME=<home>/.config, XDG_CACHE_HOME=<home>/.cache,
// TMPDIR=<workspacePath>/.tmp, plus every declaredEnv pair (PATH included
// only if declaredEnv declares it — this package sets no default). The
// four .home/.config/.cache/.tmp directories are created under the
// workspace. A declaredEnv key that is empty, contains '=', contains NUL,
// or collides with one of the four profile-owned keys is a fail-closed
// error — never a silent override.
//
// Timeout is set from a GrantTimeouts grant, if present, else remains the
// zero Duration. AllowedArgv0s is a sorted copy of a GrantProcessExecution
// grant's Argv0s, if present, else remains nil (a fact — no execution
// allowance recorded — never an enforced empty allowlist).
//
// The returned EnforcementReport always has exactly one row per grant kind
// present in grants, deterministically kind-ordered. Per AD-5: if any row
// is could-not-apply, BuildProfile ALSO returns a non-nil error (see this
// file's own doc comment for why that error's concrete type is the
// package's existing *OperationalError) naming every unapplied kind — the
// Profile and EnforcementReport are still returned alongside it, since the
// facts survive the error and a caller inspecting the report should never
// need to special-case the error path to see them.
//
// No wall clock, no randomness: BuildProfile's output depends only on its
// three inputs.
func BuildProfile(workspacePath string, grants GrantSet, declaredEnv map[string]string) (Profile, *EnforcementReport, error) {
	if workspacePath == "" {
		return Profile{}, nil, fmt.Errorf("execworkspace: build profile: workspace path is empty")
	}
	if err := grants.Validate(); err != nil {
		return Profile{}, nil, fmt.Errorf("execworkspace: build profile: %w", err)
	}
	for key := range declaredEnv {
		if err := validateEnvKey(key); err != nil {
			return Profile{}, nil, fmt.Errorf("execworkspace: build profile: declared env: %w", err)
		}
		if profileOwnedEnvKeys[key] {
			return Profile{}, nil, fmt.Errorf(
				"execworkspace: build profile: declared env key %q collides with a profile-owned key, never a silent override",
				key,
			)
		}
	}

	home := filepath.Join(workspacePath, ".home")
	env := map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"TMPDIR":          filepath.Join(workspacePath, ".tmp"),
	}
	for key, value := range declaredEnv {
		env[key] = value
	}

	for _, dir := range []string{env["HOME"], env["XDG_CONFIG_HOME"], env["XDG_CACHE_HOME"], env["TMPDIR"]} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return Profile{}, nil, fmt.Errorf("execworkspace: build profile: creating %q: %w", dir, err)
		}
	}

	profile := Profile{env: env}

	if g, ok := grants.Get(GrantTimeouts); ok {
		profile.Timeout = time.Duration(g.Seconds) * time.Second
	}
	if g, ok := grants.Get(GrantProcessExecution); ok {
		argv0s := append([]string(nil), g.Argv0s...)
		sort.Strings(argv0s)
		profile.AllowedArgv0s = argv0s
	}

	report, unapplied := buildEnforcementReport(grants)
	if len(unapplied) > 0 {
		return profile, report, operationalError(
			"isolation-profile: apply-grants",
			fmt.Errorf(
				"required execution grant(s) could not be applied: %s (CI dc-10: authoritative launch fails when isolation cannot be proven; CSE operational-error clause: unavailable required isolation control invalidates the run)",
				strings.Join(unapplied, ", "),
			),
		)
	}
	return profile, report, nil
}

// buildEnforcementReport builds the deterministic, kind-ordered
// EnforcementReport for grants, and separately returns the String() labels
// of every could-not-apply kind (for the AD-5 operational-error message).
func buildEnforcementReport(grants GrantSet) (*EnforcementReport, []string) {
	present := make(map[GrantKind]bool, len(grants.Grants))
	for _, g := range grants.Grants {
		present[g.Kind] = true
	}

	rows := []EnforcementReportRow{}
	var unapplied []string
	for k := GrantKind(0); k < numGrantKinds; k++ {
		if !present[k] {
			continue
		}
		if reason, ok := appliedReasons[k]; ok {
			rows = append(rows, EnforcementReportRow{Kind: k, Applied: true, Reason: reason})
			continue
		}
		rows = append(rows, EnforcementReportRow{Kind: k, Applied: false, Reason: couldNotApplyReasons[k]})
		unapplied = append(unapplied, k.String())
	}
	return &EnforcementReport{Rows: rows}, unapplied
}

// validateEnvKey enforces BuildProfile's declared-env key rules: non-empty,
// no '=', no NUL byte.
func validateEnvKey(key string) error {
	if key == "" {
		return fmt.Errorf("declared env key is empty")
	}
	if strings.ContainsRune(key, '=') {
		return fmt.Errorf("declared env key %q contains '='", key)
	}
	if strings.ContainsRune(key, 0) {
		return fmt.Errorf("declared env key %q contains a NUL byte", key)
	}
	return nil
}
