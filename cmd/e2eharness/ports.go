package main

// The e2e harness's port derivation (D6-28): before VERDI_E2E_PORT_BASE
// existed, cmd/e2eharness always bound 127.0.0.1:4173 (workbench),
// :4174 (dex static site), and :4177 (control server, control.go) — fine
// for one run, but two concurrent `make verify` invocations in sibling git
// worktrees collide on those same three ports and one loses with "address
// already in use", paying a retry tax during parallel implementation
// waves. resolvePorts is the single knob that fixes this: set
// VERDI_E2E_PORT_BASE and every port below derives from it as base,
// base+1, base+2; e2e/ports.ts (imported by e2e/playwright.config.ts and
// e2e/tests/fixtures.ts) mirrors this exact derivation so the Playwright
// runner's URLs move in lockstep with the harness's listeners. Unset (or
// unparsable): the historical hard-coded ports below, byte-for-byte —
// zero behavior change.

import (
	"fmt"
	"log"
	"strconv"
)

// Historical hard-coded ports — the defaults when portBaseEnvVar is unset.
const (
	defaultWorkbenchPort = 4173
	defaultDexPort       = 4174
	defaultControlPort   = 4177
)

// portBaseEnvVar is the one env knob that shifts all three harness ports in
// lockstep.
const portBaseEnvVar = "VERDI_E2E_PORT_BASE"

// minPortBase/maxPortBase bound an acceptable base: low enough to leave
// base+2 a valid TCP port, and above 0 (port 0 means "OS picks" for
// net.Listen, which is not what a fixed, playwright-addressable base
// means here).
const (
	minPortBase = 1
	maxPortBase = 65533
)

// ports is the resolved trio of loopback addresses one harness run binds.
type ports struct {
	workbench string
	dex       string
	control   string
}

// resolvePorts reads portBaseEnvVar through getenv (os.Getenv in
// production; a stub in tests) and derives the trio. Any missing,
// non-numeric, or out-of-range value fails CLOSED to the historical
// defaults — printing a notice via log — rather than silently binding a
// half-derived or invalid port set.
func resolvePorts(getenv func(string) string) ports {
	defaults := ports{
		workbench: loopback(defaultWorkbenchPort),
		dex:       loopback(defaultDexPort),
		control:   loopback(defaultControlPort),
	}

	raw := getenv(portBaseEnvVar)
	if raw == "" {
		return defaults
	}

	base, err := strconv.Atoi(raw)
	if err != nil || base < minPortBase || base > maxPortBase {
		log.Printf(
			"e2eharness: %s=%q is not a usable port base (want an integer in %d..%d) — falling back to default ports %s/%s/%s",
			portBaseEnvVar, raw, minPortBase, maxPortBase, defaults.workbench, defaults.dex, defaults.control,
		)
		return defaults
	}

	return ports{
		workbench: loopback(base),
		dex:       loopback(base + 1),
		control:   loopback(base + 2),
	}
}

// loopback formats a port as a 127.0.0.1 address string.
func loopback(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}
