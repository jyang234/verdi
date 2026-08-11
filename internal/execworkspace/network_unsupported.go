//go:build !linux

package execworkspace

// Unsupported-platform network mechanics (design §1: "Darwin is
// unsupported for this control in the first increment: the deprecated
// sandbox-exec interface and entitlement-based App Sandbox are not adopted
// as an arbitrary-child execution contract ... There is no weaker
// fallback."). Every non-Linux GOOS builds this file; it offers no
// default-deny mechanism at all, by design — never a probe result, a
// structural fact of this build.

import (
	"fmt"
	"runtime"
	"syscall"
)

// networkDenyEnforcement is every unsupported platform's construction-time
// deny fact: Configured is unconditionally false, since no default-deny
// mechanism exists here at all.
func networkDenyEnforcement() NetworkEnforcement {
	return NetworkEnforcement{
		Mode:       NetworkDeny,
		Configured: false,
		Reason: fmt.Sprintf(
			"no network grant requested and GOOS=%s has no default-deny network backend: unsupported platforms have no weaker fallback (design §1, SI-75/SI-76)",
			runtime.GOOS,
		),
	}
}

// networkSysProcAttr is this platform's ONE source of syscall.SysProcAttr
// (SI-40): nil for an explicit ambient allow, which needs no platform
// mechanism at all and so is available even here; a fail-closed error for
// every other state, since a configured deny can never occur on an
// unsupported platform (networkDenyEnforcement above always reports
// Configured=false) and there is no cgo, helper binary, or weaker fallback
// to construct one (design §1).
func networkSysProcAttr(net NetworkEnforcement) (*syscall.SysProcAttr, error) {
	if net.Mode == NetworkAllow && net.Configured {
		return nil, nil
	}
	return nil, networkNotLaunchableError(net)
}
