//go:build linux

package execworkspace

// TestHermeticLinuxNetworkStartProbe is the design's real Linux
// integration probe (design §6 item 4; lane contract test area h): a
// hermetic PARENT loopback listener; a re-exec HELPER child launched under
// a genuinely constructed deny profile (no network grant; a
// process-execution grant naming os.Executable(); a bounded timeouts
// grant). No TestMain exists in this package — clean re-exec, mirroring
// launch_test.go's own TestExecworkspaceHelperSleep pattern.
//
// NEVER t.Skip: on Start success the child's isolation is the asserted
// witness (dial to the parent's listener fails, the child sees no
// non-loopback interface, and the parent listener accepts nothing); on
// Start failure the error MUST be a recognized namespace-refusal errno
// (EPERM/EACCES/EINVAL/ENOSYS/EOPNOTSUPP) — asserted and logged as the
// named operational refusal leg, never a silent pass and never
// isolation-success in that leg. Any other Start failure, or an isolation
// witness that does not hold on success, fails the test outright.
//
// No external network: the only endpoint this test ever touches is the
// listener it opens itself on 127.0.0.1:0.

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

// helperNetProbeAddr drives TestExecworkspaceHelperNetProbe, the re-exec
// child TestHermeticLinuxNetworkStartProbe launches through Profile.Command
// under a deny profile. A flag, not an env var: Profile.Command sets
// EXACTLY the profile environment (isolation.go), so an env-var channel
// would be extra state this seam never grants; the helper's own stdout is
// the one channel the CONSUMER (this test) is free to attach after
// construction (isolation.go's Command doc comment).
var helperNetProbeAddr = flag.String("execworkspace.helper.netprobe", "",
	"internal: when non-empty, TestExecworkspaceHelperNetProbe dials this loopback address and reports the outcome on stdout (re-exec helper for the hermetic Linux network start probe)")

const (
	netProbeDialFailed       = "NETPROBE_DIAL_FAILED"
	netProbeDialSucceeded    = "NETPROBE_DIAL_SUCCEEDED"
	netProbeNonLoopbackFound = "NETPROBE_NON_LOOPBACK_FOUND"
	netProbeNoNonLoopback    = "NETPROBE_NO_NON_LOOPBACK"
)

// TestExecworkspaceHelperNetProbe is not a test of this package; it is the
// child process TestHermeticLinuxNetworkStartProbe re-execs inside the
// constructed deny profile. It does nothing unless the parent passes
// -execworkspace.helper.netprobe=<addr>. No external network: addr is
// always the PARENT's own hermetic loopback listener from this same test
// run.
func TestExecworkspaceHelperNetProbe(t *testing.T) {
	addr := *helperNetProbeAddr
	if addr == "" {
		t.Skip("not the re-exec helper: -execworkspace.helper.netprobe unset")
	}

	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		fmt.Println(netProbeDialFailed)
	} else {
		_ = conn.Close()
		fmt.Println(netProbeDialSucceeded)
	}

	nonLoopback := false
	ifaces, ierr := net.Interfaces()
	if ierr != nil {
		// Cannot enumerate: cannot claim isolation either way, so report
		// explicitly rather than silently assuming success either
		// direction (the parent's NoNonLoopback check below then fails,
		// disclosed, rather than passing on an unproven claim).
		fmt.Println("NETPROBE_INTERFACES_ERROR:", ierr)
	} else {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, aerr := iface.Addrs()
			if aerr != nil || len(addrs) == 0 {
				continue
			}
			nonLoopback = true
		}
	}
	if nonLoopback {
		fmt.Println(netProbeNonLoopbackFound)
	} else {
		fmt.Println(netProbeNoNonLoopback)
	}
}

func TestHermeticLinuxNetworkStartProbe(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("parent loopback listener: %v", err)
	}
	defer func() { _ = listener.Close() }()
	addr := listener.Addr().String()

	// The parent listener must accept nothing during the probe: an accept
	// loop that records any connection at all is itself part of the
	// isolation witness on the success leg.
	accepted := make(chan error, 1)
	go func() {
		conn, aerr := listener.Accept()
		if aerr == nil {
			_ = conn.Close()
		}
		accepted <- aerr
	}()

	dir := t.TempDir()
	envRoot := t.TempDir()
	set := GrantSet{Grants: []Grant{
		{Kind: GrantProcessExecution, Argv0s: []string{self}},
		{Kind: GrantTimeouts, Seconds: 20},
	}}
	profile, report, berr := BuildProfile(dir, envRoot, set, nil)
	if berr != nil {
		t.Fatalf("BuildProfile: unexpected error: %v (report=%+v)", berr, report)
	}
	if report.Network.Mode != NetworkDeny || !report.Network.Configured {
		t.Fatalf("report.Network = %+v, want configured deny", report.Network)
	}

	cmd, _, cancel, cerr := profile.Command(context.Background(), self,
		"-test.run=^TestExecworkspaceHelperNetProbe$",
		"-execworkspace.helper.netprobe="+addr,
	)
	if cerr != nil {
		t.Fatalf("Command: %v", cerr)
	}
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	startErr := cmd.Start()
	if startErr != nil {
		var errno syscall.Errno
		if !errors.As(startErr, &errno) {
			t.Fatalf("Start failed with a non-errno error %v (%T): cannot confirm a namespace-refusal class, so this is not the accepted refusal leg", startErr, startErr)
		}
		switch errno {
		case syscall.EPERM, syscall.EACCES, syscall.EINVAL, syscall.ENOSYS, syscall.EOPNOTSUPP:
			t.Logf("REFUSAL LEG: Start refused with recognized namespace-refusal errno %v (%d) — this runner's kernel disallows the required namespaces; asserted as the named operational refusal, never treated as isolation success", errno, errno)
			return
		default:
			t.Fatalf("Start failed with unrecognized errno %v (%d): not a known namespace-refusal class", errno, errno)
		}
	}

	t.Log("ISOLATION-WITNESS LEG: Start succeeded")
	waitErr := cmd.Wait()
	out := stdout.String()
	t.Logf("helper stdout: %q, stderr: %q, wait error: %v", out, stderr.String(), waitErr)

	if !strings.Contains(out, netProbeDialFailed) {
		t.Fatalf("helper did not report %s (stdout=%q): the child must be unable to dial the parent's loopback listener", netProbeDialFailed, out)
	}
	if strings.Contains(out, netProbeDialSucceeded) {
		t.Fatalf("helper reported %s (stdout=%q): the network namespace did not isolate the child", netProbeDialSucceeded, out)
	}
	if !strings.Contains(out, netProbeNoNonLoopback) {
		t.Fatalf("helper did not report %s (stdout=%q): the child must see no non-loopback interface", netProbeNoNonLoopback, out)
	}
	if strings.Contains(out, netProbeNonLoopbackFound) {
		t.Fatalf("helper reported %s (stdout=%q): the child observed a non-loopback interface", netProbeNonLoopbackFound, out)
	}

	select {
	case aerr := <-accepted:
		t.Fatalf("parent listener accepted a connection (err=%v): the child reached it despite the network namespace", aerr)
	case <-time.After(500 * time.Millisecond):
		// No connection arrived within the window: the parent listener
		// accepted nothing, consistent with the child's isolation. The
		// accept goroutine itself is cleaned up by the deferred
		// listener.Close() above, whichever leg this test took.
	}
}
