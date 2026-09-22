// verdi serve (05 §MCP server, 01 §D3, PLAN.md Phase 9): the single
// writer process for a checkout. Acquires the writer lock (I-12), hosts
// the MCP endpoint on the checkout's unix socket (I-29's short-path
// scheme, pointer file at .verdi/data/serve.path), and hosts the
// localhost-only workbench HTTP skeleton (internal/workbench) alongside
// it. Runs until SIGINT/SIGTERM, then releases the lock and removes the
// socket cleanly — a crash instead leaves both behind, which I-12's
// takeover and I-29's pointer file are both designed to tolerate.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jyang234/verdi/internal/buildinfo"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/mcpserve"
	"github.com/jyang234/verdi/internal/readinessload"
	"github.com/jyang234/verdi/internal/recovery"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/workbench"
)

// defaultWorkbenchAddr is the workbench HTTP listener's default bind
// address — loopback only (05 §Workbench: "binds localhost only").
const defaultWorkbenchAddr = "127.0.0.1:4173"

type serveOptions struct {
	httpAddr           string
	contextRequestPath string
}

// parseServeOptions owns serve's closed flag grammar. The readiness flag is
// additive; the legacy --http flag keeps its last-value-wins behavior.
func parseServeOptions(args []string) (serveOptions, error) {
	options := serveOptions{httpAddr: defaultWorkbenchAddr}
	contextRequestSeen := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--http":
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return serveOptions{}, errors.New("--http requires a value")
			}
			options.httpAddr = args[i+1]
			i++
		case "--context-request":
			if contextRequestSeen {
				return serveOptions{}, errors.New("--context-request may be supplied only once")
			}
			contextRequestSeen = true
			if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "--") {
				return serveOptions{}, errors.New("--context-request requires a filesystem path")
			}
			i++
			options.contextRequestPath = args[i]
			if options.contextRequestPath == "-" {
				return serveOptions{}, errors.New("--context-request does not accept stdin ('-')")
			}
		default:
			return serveOptions{}, fmt.Errorf("unknown argument %q", args[i])
		}
	}
	return options, nil
}

// serveRunner is threaded the fully-resolved readiness loader and default
// spec (spec/readiness-recovery Task 3 exit obligation: no package-level
// hand-off) alongside the server's other startup facts — never a single
// startup-frozen snapshot (ac-2's per-request derivation).
type serveRunner func(root, httpAddr string, loader readinessload.Loader, defaultSpec string, stdout, stderr io.Writer) int

// readinessWarmBuilder is the serve command's one startup warm-up
// boundary: it runs the real judge once (JudgeRun) over a supplied
// --context-request so a later per-request JudgeCacheOnly derivation for
// the SAME request finds a cache hit, and reports that request's own
// target spec ref for ReadinessDefaultSpec. It also hands back the exact
// bytes and decoded value it validated (R-RR1-17), which cmdServeWithDeps
// carries into every per-request load. Called only when a
// --context-request path was supplied.
type readinessWarmBuilder interface {
	Build(ctx context.Context, root, requestPath string) (defaultSpec string, predecoded *readinessload.PredecodedRequest, err error)
}

// serveCommandDeps is the narrow startup-order seam. The readiness warm-up
// (when requested) is completed before run is entered; run owns every
// server effect from data-dir creation onward.
type serveCommandDeps struct {
	findRoot  func(string) (string, error)
	readiness readinessWarmBuilder
	run       serveRunner
}

// cmdServe is `verdi serve`'s real entry point, invoked by dispatch.go.
func cmdServe(args []string, stdout, stderr io.Writer) int {
	return cmdServeWithDeps(args, stdout, stderr, serveCommandDeps{
		findRoot:  store.FindRoot,
		readiness: readinessLoadBuilder{},
		run:       runServe,
	})
}

// readinessLoadBuilder is readinessWarmBuilder's one production
// implementation: internal/readinessload.Load under JudgeRun, warming the
// judge cache exactly as the predecessor startup adapter did (spec/
// readiness-recovery Task 2).
type readinessLoadBuilder struct{}

func (readinessLoadBuilder) Build(ctx context.Context, root, requestPath string) (string, *readinessload.PredecodedRequest, error) {
	// ContextRequestSpec reads, path-validates and decodes requestPath
	// exactly once. The Predecoded bundle it returns flows into warmOpts
	// below so the warm-up Load does not read the same file a second time,
	// and is handed back to cmdServeWithDeps so every later per-request
	// load carries it too (R-RR1-17). An earlier comment here required the
	// opposite — a fresh read of the request file on every per-request
	// load; R-RR1-17 supersedes it, because a request file deleted, moved
	// or edited mid-run then turned EVERY spec's derivation through this
	// one server-wide loader into an operational failure for as long as
	// the server ran. The startup-validated bytes are the ones this server
	// was started with, so they are the honest thing to keep deriving
	// against; the store itself is still re-read on every request (ac-2).
	targetSpec, predecoded, err := readinessload.ContextRequestSpec(root, requestPath)
	if err != nil {
		return "", nil, err
	}
	warmOpts := readinessload.Options{
		ContextRequestPath: requestPath,
		BoardHref:          workbench.BranchBoardHref,
		Actors:             resolveConflictActors,
		Judge:              readinessload.JudgeRun,
		PredecodedRequest:  predecoded,
		// R-RRF-3 (SI-214): the WARM-UP is the only caller that still
		// refuses a request whose `expected` branch/HEAD does not describe
		// this checkout — a request already stale at startup is a
		// misconfiguration, and serve exits 2 on it. The per-request
		// loaderOpts cmdServeWithDeps builds below deliberately leaves the
		// option false: those loads run against this same retained bundle
		// for the life of the server, so an ordinary commit must disclose
		// the stale context, not blank the whole readiness page.
		RequireExpectedMatch: true,
	}
	if _, err := readinessload.Load(ctx, root, targetSpec, warmOpts); err != nil {
		return "", nil, err
	}
	return targetSpec, predecoded, nil
}

// startupRequestTarget renders the warm-up's own spec ref for R-RR1-16's
// stdout line. ContextRequestSpec returns the request's `spec` field,
// which is already a whole spec ref ("spec/<name>") in every request the
// decoder accepts; the prefix is added only for a value that somehow does
// not carry it, and never doubled for one that does.
func startupRequestTarget(ref string) string {
	// vocab:identity — "spec/" is the artifact-ref grammar's own kind prefix (02 §Refs), not a display word
	const specRefPrefix = "spec/"
	if strings.HasPrefix(ref, specRefPrefix) {
		return ref
	}
	return specRefPrefix + ref
}

// cmdServeWithDeps parses the additive readiness input, runs the optional
// startup warm-up, and only then enters the effectful server run — always
// with a working readiness loader (spec/readiness-recovery ac-2: general
// per-ref derivation needs no request at all), bound to the supplied
// --context-request only when one was given.
func cmdServeWithDeps(args []string, stdout, stderr io.Writer, deps serveCommandDeps) int {
	options, err := parseServeOptions(args)
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}
	if deps.findRoot == nil {
		fmt.Fprintln(stderr, "serve: store root resolver is nil")
		return 2
	}

	root, err := deps.findRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}

	loaderOpts := readinessload.Options{
		BoardHref: workbench.BranchBoardHref,
		Actors:    resolveConflictActors,
	}
	defaultSpec := ""
	if options.contextRequestPath != "" {
		if deps.readiness == nil {
			fmt.Fprintln(stderr, "serve: readiness warm-up builder is nil")
			return 2
		}
		spec, predecoded, buildErr := deps.readiness.Build(context.Background(), root, options.contextRequestPath)
		if buildErr != nil {
			fmt.Fprintln(stderr, "serve:", buildErr)
			return 2
		}
		defaultSpec = spec
		// ContextRequestPath stays set: Load's contextFallback vector
		// names it as the context-conflict verb's own --request argument,
		// and the path is still this server's request identity.
		loaderOpts.ContextRequestPath = options.contextRequestPath
		// R-RR1-17: the startup-validated request bundle, carried into
		// every per-request load so none of them re-reads the file.
		loaderOpts.PredecodedRequest = predecoded
		// R-RR1-16: the fact that THIS server was started with a context
		// request bound to one spec is disclosed exactly once, here, on
		// the same stdout stream runServe logs its socket/workbench lines
		// to. It deliberately does not travel in any derived document's
		// bytes: a request-specific witness inside a document would make
		// the request's own spec read differently through this loader than
		// through a CLI carrying no request, which is the ac-4 parity
		// divergence R-RR1-15 closed. Every other spec this server derives
		// still derives as if no request had been supplied, which is what
		// the second clause tells the operator.
		fmt.Fprintf(stdout, "readiness: the startup context request targets %s; every other spec derives without a request\n", startupRequestTarget(defaultSpec))
	}
	loader := readinessload.Loader{Root: root, Opts: loaderOpts}

	if deps.run == nil {
		fmt.Fprintln(stderr, "serve: server runner is nil")
		return 2
	}
	return deps.run(root, options.httpAddr, loader, defaultSpec, stdout, stderr)
}

// runServe contains the existing single-writer runtime. It is entered only
// after any requested readiness warm-up has fully completed.
func runServe(root, httpAddr string, readinessLoader readinessload.Loader, readinessDefaultSpec string, stdout, stderr io.Writer) int {
	dataDir := filepath.Join(root, ".verdi", "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}

	lockPath := filepath.Join(dataDir, "writer.lock")
	lockFile, err := filelock.Acquire(lockPath)
	if err != nil {
		var held *filelock.ErrHeld
		if errors.As(err, &held) {
			fmt.Fprintf(stderr, "serve: %v — another verdi serve is already the writer for this checkout; use `verdi mcp` to reach it\n", err)
		} else {
			fmt.Fprintln(stderr, "serve:", err)
		}
		return 2
	}
	defer func() { _ = filelock.Release(lockFile, lockPath) }()

	sockPath, err := mcpserve.SocketPath(root)
	if err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}
	if err := os.MkdirAll(filepath.Dir(sockPath), 0o755); err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}
	_ = os.Remove(sockPath) // best-effort: a prior crash may have left a stale socket inode
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		fmt.Fprintln(stderr, "serve: binding MCP socket:", err)
		return 2
	}
	defer func() {
		_ = ln.Close()
		_ = os.Remove(sockPath)
	}()

	if err := mcpserve.WritePointerFile(root, sockPath); err != nil {
		fmt.Fprintln(stderr, "serve:", err)
		return 2
	}

	// Review mode's comment feed (05 §Review stickies). The real source is
	// the forge adapter over workbench.CommentFeed (reviewfeed.go), built
	// from the same best-effort forge that feeds mcpserve's review-sticky
	// mirror below — a single forge construction, reused. When no real
	// forge is configured/reachable (the hermetic e2e harness, any offline
	// checkout) it falls back to the canned-file feed the harness injects
	// (VERDI_REVIEW_FEED — a strict-decoded local JSON file, no network).
	// Real forge config takes precedence when both are present; with
	// neither, no spec is ever under review and the board keys purely off
	// branch state.
	forgePort, configuredKind := forgeBestEffort(context.Background(), root)
	deps := workbench.Deps{ReadinessLoader: readinessLoader, ReadinessDefaultSpec: readinessDefaultSpec}
	// The ASD design bridge (Wave 6 Task 2): the one application core the
	// CLI and MCP already use, injected behind the workbench's port.
	deps.Design = newServeDesignBridge()
	switch {
	case forgePort != nil:
		deps.CommentFeed = newForgeCommentFeed(forgePort, root)
		// The pending-supersession wall badge's forge access (spec/badge-
		// computes ac-3): the same best-effort forge construction above,
		// reused — no second construction. nil (no forge configured/
		// reachable) leaves Deps.SupersessionCandidates at its zero value,
		// so every pending-supersession outcome renders as a disclosed-
		// unproven notice rather than a badge (never silently unflagged).
		deps.SupersessionCandidates = newForgeSupersessionLoader(forgePort, root)
	case os.Getenv("VERDI_REVIEW_FEED") != "":
		feed, ferr := workbench.LoadCannedCommentFeed(os.Getenv("VERDI_REVIEW_FEED"))
		if ferr != nil {
			fmt.Fprintln(stderr, "serve:", ferr)
			return 2
		}
		deps.CommentFeed = feed
	case configuredKind != "":
		// A forge is named in verdi.yaml but no live adapter could be built
		// (no credentials): disclose on the board rather than render as
		// silently not-under-review (I-1(b)).
		deps.ReviewUnavailable = reviewUnavailableReason(configuredKind)
	}
	if forgePort == nil && configuredKind != "" {
		// The /disclosures page's process-context input
		// (spec/disclosures-panel ac-1): the same structured seam value
		// behind reviewUnavailableReason, under the same condition
		// mcpserve's ReviewUnavailable uses below — deliberately NOT the
		// board switch's narrower case, because a canned harness feed
		// (VERDI_REVIEW_FEED) substituting for review comments does not
		// make the live forge any more reachable; the checkout's
		// disclosed context holds either way.
		deps.Disclosures = append(deps.Disclosures, reviewUnavailableDisclosure(configuredKind))
	}

	// The directory home's in-review consultation (spec/directory-home
	// dc-4), wired in the same precedence order as the review feed above:
	// the live forge, else the hermetic harness feed (VERDI_OPENMR_FEED, a
	// loopback URL), else — when a forge IS configured but unreachable —
	// the always-erroring lister whose disclosed reason the home page
	// renders as its "MR status unavailable" notice (I-1(b)). With none of
	// the three, no forge is configured and the chips are silently,
	// legitimately absent (home.OpenMRs nil).
	home := workbench.HomeDeps{}
	switch {
	case forgePort != nil:
		home.OpenMRs = newForgeOpenMRs(forgePort, root)
	case os.Getenv("VERDI_OPENMR_FEED") != "":
		home.OpenMRs = httpOpenMRFeed{url: os.Getenv("VERDI_OPENMR_FEED")}
	case configuredKind != "":
		home.OpenMRs = unavailableOpenMRs{reason: reviewUnavailableReason(configuredKind)}
	}

	// The diagram editor's verification rail (spec/board-editor dc-4): the
	// canned-file verifier is the hermetic e2e harness's injection
	// (VERDI_DIAGRAM_VERIFICATION — a strict-decoded local JSON file, no
	// network), mirroring VERDI_REVIEW_FEED above. With nothing wired the
	// rail renders its disclosed verification-unavailable state — the
	// verification-extractor story's live adapter arrives through this
	// same Deps seam when its wiring lands, keeping the two stories
	// buildable in either order.
	if p := os.Getenv("VERDI_DIAGRAM_VERIFICATION"); p != "" {
		verifier, verr := workbench.LoadCannedDiagramVerifier(p)
		if verr != nil {
			fmt.Fprintln(stderr, "serve:", verr)
			return 2
		}
		deps.DiagramVerifier = verifier
	}

	httpLn, err := net.Listen("tcp", httpAddr)
	if err != nil {
		fmt.Fprintln(stderr, "serve: binding workbench HTTP:", err)
		return 2
	}
	httpSrv := &http.Server{Handler: workbench.NewHandlerWithHome(root, deps, home)}
	go func() {
		_ = httpSrv.Serve(httpLn)
	}()
	defer httpSrv.Close()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		s := <-sigc
		fmt.Fprintf(stdout, "serve: signal %v received, shutting down\n", s)
		httpSrv.Close()
		ln.Close()
	}()

	// spec/uat-round-1 ac-1 (closes UAT-003): identify the exact build
	// serving this checkout, once, on the same stream (stdout) serve
	// already logs its socket/workbench lines to — the same
	// internal/buildinfo.Line() `verdi version`/`--version` print
	// (version.go) and the workbench footer renders (a separate Fable
	// lane), so no two of the three can ever disagree about which build
	// produced a result.
	fmt.Fprintln(stdout, buildinfo.Line())
	fmt.Fprintf(stdout, "serve: MCP socket at %s (pointer: %s)\n", sockPath, filepath.Join(root, ".verdi", "data", "serve.path"))
	fmt.Fprintf(stdout, "serve: workbench at http://%s\n", httpLn.Addr())

	srv := mcpserve.NewServer(root)
	srv.Backend.ReadinessLoader = readinessLoader            // spec/readiness-recovery ac-4: get_document's readiness section, the SAME loader the board renders through (workbench.Deps{ReadinessLoader: readinessLoader} above)
	srv.Backend.RecoveryLoader = recovery.Loader{Root: root} // spec/readiness-recovery-v2 ac-8, ac-10's MCP half: get_recovery, the same production loader mcp.go's standalone path wires
	srv.ErrLog = os.Stderr                                   // spec/fail-loud dc-3: a dropped socket connection leaves a trace, matching mcp.go's stdio scrutiny
	// Best-effort (V1-P7): see mcp.go's identical comment — a
	// missing/unreachable forge never blocks `verdi serve` from starting;
	// list_annotations' review-sticky mirrored population (05 §MCP
	// server) just degrades to "no review population" (Backend.Forge nil
	// is a fully valid zero value). Same instance the workbench comment
	// feed above uses — one construction per serve.
	srv.Backend.Forge = forgePort
	if forgePort == nil && configuredKind != "" {
		// Same disclosed-unavailable state on the machine read surface:
		// list_annotations returns a disclosure field rather than silently
		// omitting review population (I-1(b)).
		srv.Backend.ReviewUnavailable = reviewUnavailableReason(configuredKind)
	}
	// Serve blocks until ln errors — the expected path is ln.Close() from
	// the signal handler above, a clean shutdown rather than a failure.
	_ = srv.Serve(context.Background(), ln)
	return 0
}
