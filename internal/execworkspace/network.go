package execworkspace

// Default-deny network enforcement types for spec/execution-workspace
// §Isolation-control application and §Execution-grant enforcement, per
// docs/superpowers/specs/2026-08-10-default-deny-network-enforcement-design.md
// §§2-5 (ledger SI-75/SI-76). An absent `network` grant is the MANDATORY
// default-deny requirement; a present, payload-free grant is the explicit
// ambient-allow bit (design §2: "The network grant remains payload-free.
// Presence is the explicit allow bit; absence is the default-deny
// requirement.").
//
// NetworkMode/NetworkEnforcement's shape is fixed authority — design §3's
// Go type literal, reproduced here verbatim. EnforcementReport's Network
// field (isolation.go) is always present on every BuildProfile call that
// reaches it, independent of whether a network grant was ever requested —
// the one fact this package can never omit, because absence itself is
// exactly what triggers the mandatory control (design §3: "That cannot
// represent a mandatory control triggered by the ABSENCE of a grant").
//
// Platform mechanics live ONLY in network_linux.go and
// network_unsupported.go (design §6, "Platform code in _linux.go and
// _unsupported.go files"): each independently implements
// networkDenyEnforcement (this file's computeNetworkEnforcement calls it
// for the absent-grant leg) and networkSysProcAttr (Profile.Command's sole
// source of syscall.SysProcAttr, isolation.go's SI-40 seam). The two
// platform files are fully self-contained duplicates of each other's
// function signatures rather than halves of one shared dispatcher, so
// neither depends on the other's symbols and each platform's mechanism
// stays independently readable and testable.

import "fmt"

// NetworkMode is default-deny network enforcement's two-value mode (design
// §3): NetworkDeny is the mandatory default; NetworkAllow is the explicit,
// policy-granted ambient exception. There is no third value and no weaker
// fallback (design §1).
type NetworkMode string

// NetworkDeny and NetworkAllow are NetworkMode's only two members (design
// §3's Go type literal, exact spelling and values).
const (
	NetworkDeny  NetworkMode = "deny"
	NetworkAllow NetworkMode = "allow"
)

// NetworkEnforcement is EnforcementReport's always-present network fact
// (design §3's Go type literal, exact shape): Mode is deny or allow.
// Configured is PRE-START CONSTRUCTION TRUTH — "deliberately not a claim
// that a process ran" (design §3) — never applied/post-start proof; this
// package never rewrites a pre-start report into post-start proof. Reason
// names why, in either state.
type NetworkEnforcement struct {
	Mode       NetworkMode
	Configured bool
	Reason     string
}

// networkAllowReason is the row/report Reason recorded whenever an
// explicit network grant is present: the one and only condition that ever
// produces NetworkAllow (design §2: presence is the explicit allow bit).
const networkAllowReason = "network grant present: explicit ambient host networking permitted (design §2); Profile.Command attaches no network namespace for this launch (SI-75/SI-76)"

// computeNetworkEnforcement is BuildProfile's construction-time network
// fact (design §3): a present network grant is unconditionally
// allow/configured on every platform — ambient networking needs no
// platform mechanism at all, so it can never fail to configure. An absent
// grant is the mandatory deny, whose Configured truth is platform-owned
// (networkDenyEnforcement, network_linux.go / network_unsupported.go):
// true only after a profile capable of attaching the namespace attributes
// has been constructed (Linux); false, with an operational error, on every
// other platform (design §3, §5).
func computeNetworkEnforcement(grantPresent bool) NetworkEnforcement {
	if grantPresent {
		return NetworkEnforcement{Mode: NetworkAllow, Configured: true, Reason: networkAllowReason}
	}
	return networkDenyEnforcement()
}

// networkNotLaunchableError renders the ONE message every platform's
// networkSysProcAttr fail-closed default branch uses, kept as a shared
// helper so identical malformed/zero-mode input produces an identical
// message regardless of which platform file is compiled in (design §5:
// "No state silently changes deny into allow").
func networkNotLaunchableError(net NetworkEnforcement) error {
	return fmt.Errorf(
		"network mode %q (configured=%v) has no launchable mechanism: an unconfigured deny or a zero/unset network mode is never started (design §5, SI-75/SI-76)",
		net.Mode, net.Configured,
	)
}
