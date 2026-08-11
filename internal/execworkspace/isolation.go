package execworkspace

// Isolation-profile construction for spec/execution-workspace
// §Isolation-control application and §Execution-grant enforcement
// (controller decisions AD-5/AD-9). This is MECHANISM ONLY, and
// CONSTRUCTION-ONLY: this package builds a Profile, and builds the launch a
// consumer then runs — it never runs the consumer's process itself (spec
// §Isolation-control application: "The component constructs the isolated
// profile ... as MECHANISM only").
//
// Profile.Command is this package's ONE launch-construction seam (ledger
// SI-40). It exists because "applied" must mean enforced: a profile that
// merely CARRIES an argv0 allowlist and a deadline enforces neither, so a
// consumer could launch any program with no deadline while the enforcement
// report read Applied — an enforcement claim with no mechanism, which the
// three-valued honesty rule forbids. Command gates argv0 against the
// granted allowlist, derives the granted deadline into the context the
// returned *exec.Cmd is bound to, and sets exactly the profile environment.
// It STILL never runs anything: it returns an unstarted *exec.Cmd, and the
// consumer decides whether, when and where to start it.
//
// AD-9 fixes v0's mechanically appliable kinds: process execution (an
// argv0 allowlist, enforced when the launch is constructed) and timeouts (a
// deadline, applied to the constructed launch's context), plus the base
// profile every grant set gets regardless of which grants it names — a
// clean environment with controlled HOME/XDG discovery. path-read,
// path-write, and resource-ceilings have no v0 mechanism.
//
// SI-75/SI-76 (network.go, network_linux.go, network_unsupported.go) give
// network its own mechanism, orthogonal to AD-9's row-based model: a
// PRESENT network grant is mechanically appliable exactly like
// process-execution/timeouts above (an applied row, explicit ambient
// permission enforced by Profile.Command attaching no namespace); an
// ABSENT grant is a MANDATORY control the row model cannot represent at
// all, since a row only exists for a grant that was requested — so
// EnforcementReport gains a separate, always-present Network fact instead
// (this file, EnforcementReport.Network) that names the default-deny
// posture whether or not network was ever requested.
//
// AD-5 makes every granted control REQUIRED: a control the component
// cannot apply is not a silent partial success. BuildProfile always
// returns its EnforcementReport (the facts survive), but when any row is
// could-not-apply — or the mandatory network deny cannot be configured
// (SI-75/SI-76, the same failure class) — it ALSO returns a non-nil error
// — reusing this package's existing *OperationalError type from
// materialize.go, which is its one shared retryable-disclosed-failure
// type. That type's Error() prefixes only "execworkspace: " and lets the
// Op name the subsystem, so this failure renders as "execworkspace:
// isolation-profile: apply-grants: ..." and is never mislabelled as a
// materialization failure. The Op and Err fields carry the specific
// content (which grant kinds could not be applied, and whether network's
// deny is among them), and errors.As reaches the typed value through any
// wrap.
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
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
)

// profileOwnedEnvKeys are the four environment keys BuildProfile itself
// sets from envRoot. A declaredEnv pair naming one of these is
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
// constructs its launch through Command, which is the seam that ENFORCES
// AllowedArgv0s and Timeout; this package never starts a process itself.
// The zero Profile has an empty (but non-panicking) Env(), a zero Timeout
// and a nil AllowedArgv0s, so Command refuses it outright — only
// BuildProfile constructs a Profile actually backed by a workspace.
type Profile struct {
	env           map[string]string
	network       NetworkEnforcement
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

// Command constructs the one launch this profile authorizes for argv0 and
// args: the package's SINGLE launch-construction seam (ledger SI-40), and
// the mechanism the enforcement report's two Applied rows now name. It
// returns an UNSTARTED *exec.Cmd, the context that Cmd is bound to, that
// context's cancel func, and an error — this package still never runs the
// consumer's process (spec §Isolation-control application: MECHANISM only).
//
// FAIL-CLOSED, in this order:
//
//   - A nil ctx is an operational error, never a silently substituted
//     Background: the caller's cancellation scope is the caller's own
//     choice, and context.WithTimeout would panic on it.
//   - Constructing a launch REQUIRES a recorded process-execution
//     allowance. A nil AllowedArgv0s means no GrantProcessExecution grant
//     was requested at all, and that is an operational error — never an
//     unconstrained Cmd. (BuildProfile leaves the field nil in exactly that
//     case, and a grant always carries a non-empty allowlist, so nil and
//     "allowed nothing" never get confused.)
//   - argv0 must be an EXACT member of AllowedArgv0s — byte equality, no
//     prefix, base-name, symlink or path-normalization matching, since any
//     looser rule would silently widen a ratified allowlist. A miss is an
//     operational error naming argv0 and the whole allowlist.
//   - The profile's network enforcement fact (p.network, set by
//     BuildProfile — design §3, ledger SI-75/SI-76) must have a launchable
//     mechanism: networkSysProcAttr (network_linux.go/network_
//     unsupported.go, the platform seam) maps an explicit ambient allow to
//     a nil SysProcAttr and a configured deny to the platform's exact
//     namespace attrs, and refuses (operational error, no Cmd) every other
//     state — an unconfigured deny or a zero/unset mode is never started
//     (design §5). This check runs LAST, after the two checks above, so
//     the pre-existing execution-allowance and allowlist refusals keep
//     their exact original failure reasons on a zero-value or
//     network-less Profile; only a Profile that already cleared both earlier
//     gates ever reaches it.
//
// On success: when Timeout > 0 (a GrantTimeouts grant), the returned
// context is context.WithTimeout(ctx, Timeout) and the Cmd is built with
// exec.CommandContext against it, so the granted deadline actually kills an
// overrunning child rather than merely being recorded. When Timeout is
// zero, NO deadline is added — the returned context IS the caller's ctx
// (the Cmd is still bound to it, so the caller's own cancellation works)
// and the cancel func is a no-op. The cancel func is NEVER nil on success,
// so `defer cancel()` is always correct.
//
// cmd.Env is set to exactly Env(): the profile environment and nothing
// inherited. cmd.Dir is deliberately NOT set — the working directory is the
// consumer's choice (it may want the unit path, a subdirectory of it, or
// somewhere else entirely), and this seam never guesses it. cmd.SysProcAttr
// is set to networkSysProcAttr's result and NOTHING ELSE composes a second
// value onto it (design §4: "no consumer composes a second value").
// cmd.ExtraFiles is never set (design §2: "Profile.Command creates no
// ExtraFiles"), so it is always nil on the returned Cmd.
//
// DISCLOSED LIMIT: os/exec resolves an argv0 containing no path separator
// through LookPath against the CALLING PROCESS's PATH, not cmd.Env, so for
// such an entry the allowlist constrains the NAME while the ambient PATH
// picks the file. An allowlist entry that is an absolute path is
// resolution-free and has no such gap; this seam neither rewrites the
// caller's entries nor imposes absoluteness, since the allowlist's contents
// are the grant author's ratified choice.
func (p Profile) Command(ctx context.Context, argv0 string, args ...string) (*exec.Cmd, context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, nil, operationalError("isolation-profile: launch", fmt.Errorf(
			"nil context: the launch's cancellation scope is the caller's own choice and is never silently substituted (SI-40)"))
	}
	if p.AllowedArgv0s == nil {
		return nil, nil, nil, operationalError("isolation-profile: launch", fmt.Errorf(
			"no process-execution grant is recorded on this profile, so no launch may be constructed for %q: an execution allowance is required, never assumed (AD-5, AD-9, SI-40; CI dc-10)",
			argv0))
	}
	if !containsExact(p.AllowedArgv0s, argv0) {
		return nil, nil, nil, operationalError("isolation-profile: launch", fmt.Errorf(
			"argv0 %q is not in the granted argv0 allowlist [%s]: membership is exact, never a prefix, base name, or resolved path (AD-9, SI-40)",
			argv0, strings.Join(p.AllowedArgv0s, ", ")))
	}

	sysProcAttr, npErr := networkSysProcAttr(p.network)
	if npErr != nil {
		return nil, nil, nil, operationalError("isolation-profile: launch", fmt.Errorf(
			"network enforcement not launchable for %q: %w (SI-75/SI-76)", argv0, npErr))
	}

	runCtx := ctx
	cancel := context.CancelFunc(func() {})
	if p.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, p.Timeout)
	}

	cmd := exec.CommandContext(runCtx, argv0, args...)
	cmd.Env = p.Env()
	cmd.SysProcAttr = sysProcAttr
	return cmd, runCtx, cancel, nil
}

// containsExact reports whether s is a byte-exact member of list. It is the
// allowlist membership rule Command enforces, kept as its own named
// predicate so the "exact, never fuzzy" property is stated in one place.
func containsExact(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
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
// byte-identical row ordering. Network is a SEPARATE, always-present fact
// (design §3, ledger SI-75/SI-76): unlike every row above, it is populated
// on EVERY BuildProfile call that reaches it regardless of whether a
// network grant was ever requested, because network's ABSENCE is itself
// what triggers the mandatory default-deny control — a row keyed to
// requested-grant presence could never represent that.
type EnforcementReport struct {
	Rows    []EnforcementReportRow
	Network NetworkEnforcement
}

// appliedReasons names the mechanism actually used for each mechanically
// appliable kind. Process-execution and timeouts are AD-9's v0 set;
// network joined them under SI-75/SI-76 — a PRESENT network grant is
// always applied (explicit ambient permission needs no platform
// mechanism), never could-not-apply.
var appliedReasons = map[GrantKind]string{
	GrantNetwork:          "applied: " + networkAllowReason,
	GrantProcessExecution: "applied: argv0 allowlist enforced by Profile.Command, the package's one launch-construction seam, which refuses any argv0 outside the allowlist and refuses to construct a launch at all without this grant (AD-9, SI-40)",
	GrantTimeouts:         "applied: deadline derived by Profile.Command into the context the constructed *exec.Cmd is bound to, so an overrunning child is killed (AD-9, SI-40)",
}

// couldNotApplyReasons names the missing v0 mechanism for each kind AD-9
// leaves unimplemented, cited in every could-not-apply row. GrantNetwork
// is NOT a member: SI-75/SI-76 gave it a mechanism (a present grant is
// always applied), so it moved to appliedReasons above.
var couldNotApplyReasons = map[GrantKind]string{
	GrantPathRead:         "no v0 mechanism to enforce path-read scoping (AD-9)",
	GrantPathWrite:        "no v0 mechanism to enforce path-write scoping (AD-9)",
	GrantResourceCeilings: "no v0 mechanism to enforce resource ceilings (AD-9)",
}

// BuildProfile constructs the isolation Profile for workspacePath from
// envRoot, grants and declaredEnv (spec §Isolation-control application,
// §Execution-grant enforcement; AD-5/AD-7/AD-9/AD-13).
//
// envRoot is the CALLER-CHOSEN parent of the four profile-owned directories.
// It is REQUIRED: an empty envRoot is a fail-closed error, never silently
// defaulted to workspacePath (controller decision AD-13, closing whole-wave
// finding F2). Both envRoot and workspacePath must additionally be ABSOLUTE:
// a relative root resolves against the calling process's working directory,
// which this package neither chose nor controls, so it would place the
// profile-owned directories wherever that process happens to be standing and
// emit cwd-relative env values that change meaning on any chdir. A
// non-absolute root is the same fail-closed error as an empty one (AD-13).
//
// COMPOSITION PROPERTY, disclosed — this is the reason envRoot is a separate
// parameter rather than derived from workspacePath. The directories below are
// created and then WRITTEN INTO by the launched process. If envRoot is inside
// the unit path, that content is untracked content in the unit's worktree, so
// `gitx.StatusDirty` reports the unit DIRTY and the execution gc slice keeps
// it at rank 3 (spec §GC slice) on every invocation — permanently, since
// nothing ever cleans it. That keep is CORRECT and FAIL-CLOSED, not a defect:
// real process state inside the workspace is exactly what rank 3 exists to
// protect. It does mean gc never converges for that unit ONCE ANYTHING HAS
// BEEN WRITTEN under those directories: git ignores empty directories, so at
// construction time alone the four freshly created dirs leave the unit clean
// and still reclaimable — the non-convergence begins with the first write,
// which for a launched process is a practical certainty. A consumer that
// wants a RECLAIMABLE workspace therefore places envRoot OUTSIDE the unit
// path, in its own lifecycle territory, and disposes of it itself; a consumer
// that deliberately wants the environment to live and die with the workspace
// passes the unit path and accepts the permanent keep-dirty. This component
// makes the choice explicit and never makes it silently.
//
// Env(): NO inherited process environment. Exactly HOME=<envRoot>/.home,
// XDG_CONFIG_HOME=<home>/.config, XDG_CACHE_HOME=<home>/.cache,
// TMPDIR=<envRoot>/.tmp, plus every declaredEnv pair (PATH included
// only if declaredEnv declares it — this package sets no default). The
// four .home/.config/.cache/.tmp directories are created under envRoot.
// A declaredEnv key that is empty, contains '=', contains NUL,
// or collides with one of the four profile-owned keys is a fail-closed
// error — never a silent override. A declaredEnv VALUE containing a NUL is
// likewise a fail-closed error, since the OS truncates an environment entry
// at its first NUL and the launched process would then see a different
// environment than the Profile reports.
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
// Network (design §3, ledger SI-75/SI-76) is a SEPARATE, always-present
// report fact, never a row: a PRESENT network grant is unconditionally
// allow/configured (an applied row too — GrantNetwork joined AD-9's
// mechanically-appliable set); an ABSENT grant is the mandatory deny, whose
// Configured truth is platform-owned — true on Linux after constructing a
// profile capable of attaching the namespace attributes, false everywhere
// else. A deny that cannot be configured is folded into the SAME combined
// operational error as any could-not-apply row, naming everything in one
// deterministic message — never two separate errors, never a silently
// dropped fact.
//
// No wall clock, no randomness: BuildProfile's output depends only on its
// four inputs.
func BuildProfile(workspacePath, envRoot string, grants GrantSet, declaredEnv map[string]string) (Profile, *EnforcementReport, error) {
	if workspacePath == "" {
		return Profile{}, nil, fmt.Errorf("execworkspace: build profile: workspace path is empty")
	}
	if !filepath.IsAbs(workspacePath) {
		return Profile{}, nil, fmt.Errorf(
			"execworkspace: build profile: workspace path %q is not absolute: a relative root resolves against the calling process's working directory, which this package neither chose nor controls, so the profile-owned values it feeds would change meaning on any chdir; both roots are required absolute caller choices (AD-13)",
			workspacePath)
	}
	if envRoot == "" {
		return Profile{}, nil, fmt.Errorf(
			"execworkspace: build profile: env root is empty: the parent of the profile-owned HOME/XDG/TMPDIR directories is a required caller choice, never silently defaulted to the workspace path (AD-13)")
	}
	if !filepath.IsAbs(envRoot) {
		return Profile{}, nil, fmt.Errorf(
			"execworkspace: build profile: env root %q is not absolute: a relative env root would create the profile-owned HOME/XDG/TMPDIR directories under the calling process's working directory and emit cwd-relative values for them, so it fails closed exactly as an empty env root does (AD-13)",
			envRoot)
	}
	if err := grants.Validate(); err != nil {
		return Profile{}, nil, fmt.Errorf("execworkspace: build profile: %w", err)
	}
	for key, value := range declaredEnv {
		if err := validateEnvKey(key); err != nil {
			return Profile{}, nil, fmt.Errorf("execworkspace: build profile: declared env: %w", err)
		}
		if err := validateEnvValue(key, value); err != nil {
			return Profile{}, nil, fmt.Errorf("execworkspace: build profile: declared env: %w", err)
		}
		if profileOwnedEnvKeys[key] {
			return Profile{}, nil, fmt.Errorf(
				"execworkspace: build profile: declared env key %q collides with a profile-owned key, never a silent override",
				key,
			)
		}
	}

	home := filepath.Join(envRoot, ".home")
	env := map[string]string{
		"HOME":            home,
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"TMPDIR":          filepath.Join(envRoot, ".tmp"),
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
	_, networkGranted := grants.Get(GrantNetwork)
	profile.network = computeNetworkEnforcement(networkGranted)

	report, unapplied := buildEnforcementReport(grants)
	report.Network = profile.network

	// AD-5's could-not-apply failure and design §3's deny-unconfigured
	// failure are the SAME failure class (a required control this call
	// could not provide), so a combined problem set names BOTH in ONE
	// deterministic operational error rather than two: the network problem
	// (when present) is always listed first, then the requested-grant
	// kinds in their fixed declaration order — never map iteration, never
	// randomness.
	var problems []string
	if profile.network.Mode == NetworkDeny && !profile.network.Configured {
		problems = append(problems, "network (deny unconfigurable on this platform)")
	}
	problems = append(problems, unapplied...)
	if len(problems) > 0 {
		return profile, report, operationalError(
			"isolation-profile: apply-grants",
			fmt.Errorf(
				"required execution grant(s) could not be applied: %s (CI dc-10: authoritative launch fails when isolation cannot be proven; CSE operational-error clause: unavailable required isolation control invalidates the run)",
				strings.Join(problems, ", "),
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

// Validate reports whether r is well-formed, fail-closed: Network.Mode must
// be one of the closed two values NetworkDeny/NetworkAllow; Network.Reason
// must be non-empty; every row's Kind must be a member of the closed
// six-kind GrantKind vocabulary (grants.go); and every row's Reason must be
// non-empty. An empty Rows slice always validates — "no requested grants"
// is a legitimate report, never an error. EncodeEnforcementReport calls
// this first (mirroring EncodeGrantSet's own validate-then-encode gate,
// grants.go), so a malformed report is never serialized.
func (r EnforcementReport) Validate() error {
	switch r.Network.Mode {
	case NetworkDeny, NetworkAllow:
	default:
		return fmt.Errorf("execworkspace: enforcement report: unknown network mode %q", r.Network.Mode)
	}
	if r.Network.Reason == "" {
		return fmt.Errorf("execworkspace: enforcement report: network reason is empty")
	}
	for i, row := range r.Rows {
		if row.Kind < 0 || row.Kind >= numGrantKinds {
			return fmt.Errorf("execworkspace: enforcement report: rows[%d]: unknown grant kind %d", i, row.Kind)
		}
		if row.Reason == "" {
			return fmt.Errorf("execworkspace: enforcement report: rows[%d]: reason is empty", i)
		}
	}
	return nil
}

// networkEnforcementDoc, enforcementReportRowDoc and enforcementReportDoc
// are EncodeEnforcementReport's on-disk JSON shape: SCHEMA-LESS, following
// CollectFingerprint's own AD-8 precedent (fingerprint.go's package doc
// comment: "COLLECTION IS SHARED; SCHEMAS ARE NOT") — this package states
// only the shared fields, never a feature-owned outer schema tag.
type networkEnforcementDoc struct {
	Configured bool   `json:"configured"`
	Mode       string `json:"mode"`
	Reason     string `json:"reason"`
}

type enforcementReportRowDoc struct {
	Applied bool   `json:"applied"`
	Kind    string `json:"kind"`
	Reason  string `json:"reason"`
}

type enforcementReportDoc struct {
	Network networkEnforcementDoc     `json:"network"`
	Rows    []enforcementReportRowDoc `json:"rows"`
}

// enforcementReportDocFor projects a validated EnforcementReport onto its
// wire shape. Rows is built via make(..., 0, ...) rather than left nil, so
// a report with no rows still encodes "rows":[] rather than "rows":null —
// the same non-nil-slice discipline grantSetDocFor already uses for
// GrantSet (grants.go).
func enforcementReportDocFor(r EnforcementReport) enforcementReportDoc {
	rows := make([]enforcementReportRowDoc, len(r.Rows))
	for i, row := range r.Rows {
		rows[i] = enforcementReportRowDoc{Applied: row.Applied, Kind: row.Kind.String(), Reason: row.Reason}
	}
	return enforcementReportDoc{
		Network: networkEnforcementDoc{
			Configured: r.Network.Configured,
			Mode:       string(r.Network.Mode),
			Reason:     r.Network.Reason,
		},
		Rows: rows,
	}
}

// EncodeEnforcementReport renders r as canonical JSON bytes
// (internal/canonjson: sorted keys, no HTML escaping, a trailing newline):
// {"network":{"configured":…,"mode":…,"reason":…},"rows":[{"applied":…,
// "kind":…,"reason":…}]}. It validates r first (r.Validate) and fails
// closed on any invalid report — a malformed report is never serialized
// (mirrors EncodeGrantSet's validate-then-encode gate, grants.go).
func EncodeEnforcementReport(r EnforcementReport) ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, fmt.Errorf("execworkspace: encoding enforcement report: %w", err)
	}
	data, err := canonjson.Marshal(enforcementReportDocFor(r))
	if err != nil {
		return nil, fmt.Errorf("execworkspace: encoding enforcement report: %w", err)
	}
	return data, nil
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

// validateEnvValue enforces the value half of BuildProfile's declared-env
// rules: no NUL byte. Env() renders "KEY=VALUE" entries destined for
// exec.Cmd.Env, and the OS terminates each entry at its first NUL — so a
// value carrying one would ship a SILENTLY DIFFERENT environment to the
// launched process than the Profile reports and than CollectFingerprint
// records. An empty value is legitimate ("declared, and empty") and is not
// rejected; '=' is legitimate too, since only the first '=' in an entry
// separates key from value.
func validateEnvValue(key, value string) error {
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("declared env value for key %q contains a NUL byte", key)
	}
	return nil
}
