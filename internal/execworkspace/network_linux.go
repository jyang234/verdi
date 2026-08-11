//go:build linux

package execworkspace

// Linux network mechanics (design §4, the platform mechanism section, and
// §1: "On Linux, Profile.Command configures the returned command with a
// new user namespace and a new network namespace by setting
// syscall.SysProcAttr with CLONE_NEWUSER|CLONE_NEWNET and explicit one-id
// UID/GID mappings."). This file owns the ENTIRE Linux-specific mechanism;
// network_unsupported.go is its independent, non-Linux counterpart.

import (
	"os"
	"syscall"
)

// networkDenyEnforcement is Linux's construction-time deny fact (design
// §3): Configured is unconditionally true. Building the exact SysProcAttr
// tuple below is pure Go struct/slice construction over os.Geteuid/
// os.Getegid — values the running process always has — so there is no
// runtime failure mode at CONSTRUCTION time; the only failure surface is
// the KERNEL's own refusal at Cmd.Start (design §3, §5), which this
// package never claims and never rewrites a pre-start report to cover.
func networkDenyEnforcement() NetworkEnforcement {
	return NetworkEnforcement{
		Mode:       NetworkDeny,
		Configured: true,
		Reason:     "no network grant requested: Profile.Command attaches a new user+network namespace (CLONE_NEWUSER|CLONE_NEWNET, one-id UID/GID mappings, setgroups disabled) so the child receives no host network interfaces or routes (design §§3-4, SI-75/SI-76)",
	}
}

// networkSysProcAttr is Profile.Command's ONE source of syscall.SysProcAttr
// on Linux (SI-40): nil for an explicit ambient allow (no namespace
// attached — ambient host networking, per design §§1/4); the EXACT tuple
// design §4 fixes for a configured deny (Cloneflags CLONE_NEWUSER|
// CLONE_NEWNET, one-id UID/GID mappings onto the invoking effective
// uid/gid, setgroups disabled, Credential left nil, nothing else set); and
// a fail-closed error for every other state — an unconfigured deny or a
// zero/unset mode is never started (design §5).
func networkSysProcAttr(net NetworkEnforcement) (*syscall.SysProcAttr, error) {
	switch {
	case net.Mode == NetworkAllow && net.Configured:
		return nil, nil
	case net.Mode == NetworkDeny && net.Configured:
		return &syscall.SysProcAttr{
			Cloneflags: syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
			UidMappings: []syscall.SysProcIDMap{
				{ContainerID: 0, HostID: os.Geteuid(), Size: 1},
			},
			GidMappings: []syscall.SysProcIDMap{
				{ContainerID: 0, HostID: os.Getegid(), Size: 1},
			},
			GidMappingsEnableSetgroups: false,
		}, nil
	default:
		return nil, networkNotLaunchableError(net)
	}
}
