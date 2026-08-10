# Default-Deny Network Enforcement Design

**Status:** proposed authority; effective only after owner merge

**Planning base:** `cb7cd6fb4e6123b21469a8ced886aae1f95f4398`

**Owners:** platform-team

**Consumers:** CSE isolated execution first; CI sealed execution later

## 1. Decision

The shared execution-workspace seam gains a Linux-native, no-cgo network
backend. An absent `network` grant means **network disabled is required**. On
Linux, `Profile.Command` configures the returned command with a new user
namespace and a new network namespace by setting `syscall.SysProcAttr` with
`CLONE_NEWUSER|CLONE_NEWNET` and explicit one-id UID/GID mappings. A present
`network` grant means the policy has explicitly permitted ambient host
networking and the command receives no network namespace.

There is no weaker fallback. Unsupported platforms, a backend that cannot be
configured, and a kernel refusal at process start are operational failures.
Darwin is unsupported for this control in the first increment: the deprecated
`sandbox-exec` interface and entitlement-based App Sandbox are not adopted as
an arbitrary-child execution contract.

This decision expands SI-62 only for networking. Path-read, path-write and
resource ceilings remain could-not-apply.

## 2. Boundary

The denied boundary is creation and use of host IP-network resources. A child
in the new network namespace receives no host interfaces or routes; loopback
is left in the kernel-created down state. Unix-domain sockets and inherited
file descriptors are not network-namespace isolation. Therefore:

- `Profile.Command` creates no `ExtraFiles` and the CSE runner must not add
  inherited sockets or other pre-opened network handles;
- the trusted in-process consumer may attach ordinary stdin/stdout/stderr and
  set `Dir`, but must not replace `SysProcAttr`, `Env`, `Path`, `Args`, the
  bound context, or `ExtraFiles` after construction;
- evaluator protocol and fixture inputs must travel through files or ordinary
  pipes created for the child, not inherited network sockets;
- raw hostile mutation of an already-constructed `*exec.Cmd` by repository
  code is outside the filesystem/direct-edit threat tier, but the CSE
  integration must carry a static test proving it does not mutate protected
  launch fields.

The network grant remains payload-free. Presence is the explicit allow bit;
absence is the default-deny requirement. A lower policy layer cannot add the
grant when a parent forbids it.

## 3. Operational facts and lifecycle

The current enforcement report has one row per requested grant. That cannot
represent a mandatory control triggered by the *absence* of a grant. Add one
always-present `Network` fact to `EnforcementReport`:

```go
type NetworkMode string

const (
    NetworkDeny NetworkMode = "deny"
    NetworkAllow NetworkMode = "allow"
)

type NetworkEnforcement struct {
    Mode       NetworkMode
    Configured bool
    Reason     string
}
```

`BuildProfile` reports `allow/configured` when the explicit grant selects
ambient networking. On Linux it reports `deny/configured` only after it has
constructed a profile capable of attaching the namespace attributes. On an
unsupported platform it returns `deny/configured=false` and an operational
error. The existing per-requested-grant rows remain unchanged, except that a
present `network` grant is an applied row whose reason names explicit ambient
permission.

`Configured` is deliberately not a claim that a process ran. The kernel can
still refuse namespace creation at `Cmd.Start`. The feature-owned CSE execution
receipt may say network isolation was **applied** only after start succeeds;
start failure produces no valid execution receipt and is operational. The
shared component never rewrites a pre-start report into post-start proof.

## 4. Platform mechanism

The Linux backend owns construction of `SysProcAttr`; no consumer composes a
second value. It uses the invoking effective UID and GID in explicit mappings,
disables setgroups for the GID mapping, and combines the user and network
clone flags in the one child creation. The implementation must prove the
exact mapping with table tests and keep all platform code in `_linux.go` and
`_unsupported.go` files.

Primary mechanism sources:

- Linux network namespaces isolate devices, protocol stacks, routing tables,
  firewall rules, port numbers and `/proc/net`: [network_namespaces(7)](https://man7.org/linux/man-pages/man7/network_namespaces.7.html).
- When `CLONE_NEWUSER` is combined with other namespace flags, the user
  namespace is created first, giving the child capabilities over the new
  namespaces: [user_namespaces(7)](https://man7.org/linux/man-pages/man7/user_namespaces.7.html).
- Go exposes Linux clone flags, UID/GID mappings and setgroups control through
  `syscall.SysProcAttr`: [Go Linux process attributes](https://go.dev/src/syscall/exec_linux.go).
- `exec.Cmd.SysProcAttr` is passed to `os.StartProcess`: [os/exec Cmd](https://pkg.go.dev/os/exec#Cmd).

## 5. Failure taxonomy

| State | Result |
|---|---|
| Network grant present | ambient network explicitly allowed; report applied |
| Grant absent, supported Linux backend constructed | deny configured; command may be returned |
| Grant absent, unsupported GOOS | operational error; no command |
| Grant absent, malformed profile/backend conflict | operational error; no command |
| Kernel rejects namespace setup at `Start` | operational error; no CSE result or receipt |
| Child exits after a successful start | evaluator/child outcome, not an isolation failure |

No state silently changes `deny` into `allow`.

## 6. Implementation ownership and tests

The later runtime unit may change only `internal/execworkspace` plus its CSE
consumer integration. It must TDD:

1. absent/present grant polarity and the always-present report fact;
2. exact Linux `SysProcAttr` flags and mappings without starting a process;
3. unsupported-platform operational refusal;
4. a Linux integration probe that attempts real start and proves the child
   cannot create an outbound IP connection or observe host interfaces; when
   the runner kernel disallows user namespaces, the test must assert the
   named operational refusal rather than skip or pass silently;
5. a CSE static/behavioral witness that protected `exec.Cmd` fields and
   inherited descriptors are untouched;
6. canonical CSE receipt emission only after successful start.

Tests use no external network endpoint. The connection target is a hermetic
loopback listener in the parent network namespace; inability to reach it from
the child is the witness.

## 7. Source coverage and omissions

Coverage is **8/8** authority units: execution-workspace isolation-control,
grant enforcement, fingerprint/proof separation and implementation seam;
CSE AC-4, DC-13, CO-6 and operational-error clause. SI-12, SI-40 and SI-62
are preserved and amended, not replaced.

Intentional omissions:

- no Darwin backend;
- no path or resource-ceiling enforcement;
- no CSE runner or receipt implementation in this authority change;
- no CI sealed-execution integration;
- no external helper, container runtime, cgo, firewall mutation or privileged
  daemon.
