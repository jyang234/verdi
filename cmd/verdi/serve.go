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
	"github.com/jyang234/verdi/internal/readinesspilot"
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

type serveRunner func(root, httpAddr string, readiness *readinesspilot.Snapshot, stdout, stderr io.Writer) int

// readinessSnapshotBuilder is the serve command's one startup projection
// boundary. Implementations return an immutable value; callers never retain
// a path, decoder, provider, or live fact source behind the snapshot.
type readinessSnapshotBuilder interface {
	Build(ctx context.Context, root, requestPath string) (readinesspilot.Snapshot, error)
}

// serveCommandDeps is the narrow startup-order seam. Building readiness is
// completed before run is entered; run owns every server effect from data-dir
// creation onward.
type serveCommandDeps struct {
	findRoot  func(string) (string, error)
	readiness readinessSnapshotBuilder
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

// readinessLoadBuilder is readinessSnapshotBuilder's one production
// implementation: internal/readinessload.Load under JudgeRun, warming the
// judge cache exactly as the predecessor startup adapter did (spec/
// readiness-recovery Task 2). Build also stashes the cache-only Loader
// value and the request's own target spec into the package-level
// serveReadinessLoader/serveReadinessDefaultSpec variables below, since
// runServe's signature carries only the startup snapshot — Task 3 threads
// them into the workbench/MCP deps properly; this is the narrow hand-off
// seam until then.
type readinessLoadBuilder struct{}

func (readinessLoadBuilder) Build(ctx context.Context, root, requestPath string) (readinesspilot.Snapshot, error) {
	// ContextRequestSpec reads and decodes requestPath exactly once; the
	// Predecoded bundle it returns flows into warmOpts below so Load does
	// not read the same file a second time (fix round 1, Minor 6). The
	// stashed cacheOnlyOpts deliberately does NOT carry it: every later
	// per-request Loader.Load call must read the checkout's then-current
	// request file fresh, never a startup-frozen copy.
	targetSpec, predecoded, err := readinessload.ContextRequestSpec(root, requestPath)
	if err != nil {
		return readinesspilot.Snapshot{}, err
	}
	cacheOnlyOpts := readinessload.Options{
		ContextRequestPath: requestPath,
		BoardHref:          workbench.BranchBoardHref,
		Actors:             resolveConflictActors,
		Judge:              readinessload.JudgeCacheOnly,
	}
	warmOpts := cacheOnlyOpts
	warmOpts.Judge = readinessload.JudgeRun
	warmOpts.PredecodedRequest = predecoded
	snapshot, err := readinessload.Load(ctx, root, targetSpec, warmOpts)
	if err != nil {
		return readinesspilot.Snapshot{}, err
	}
	serveReadinessLoader = readinessload.Loader{Root: root, Opts: cacheOnlyOpts}
	serveReadinessDefaultSpec = targetSpec
	return snapshot, nil
}

// serveReadinessLoader and serveReadinessDefaultSpec are runServe's hand-off
// to Task 3's consumers (workbench/MCP deps): the cache-only Loader value
// and default target spec a per-request readiness route uses, set by
// readinessLoadBuilder.Build before deps.run is ever called. Package-level
// rather than threaded through serveRunner's own signature (kept unchanged
// this task) — see readinessLoadBuilder's own doc comment.
var (
	serveReadinessLoader      readinessload.Loader
	serveReadinessDefaultSpec string
)

// cmdServeWithDeps parses the additive readiness input, captures its one
// immutable startup snapshot, and only then enters the effectful server run.
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

	var readiness *readinesspilot.Snapshot
	if options.contextRequestPath != "" {
		if deps.readiness == nil {
			fmt.Fprintln(stderr, "serve: readiness snapshot builder is nil")
			return 2
		}
		snapshot, buildErr := deps.readiness.Build(context.Background(), root, options.contextRequestPath)
		if buildErr != nil {
			fmt.Fprintln(stderr, "serve:", buildErr)
			return 2
		}
		readiness = &snapshot
	}
	if deps.run == nil {
		fmt.Fprintln(stderr, "serve: server runner is nil")
		return 2
	}
	return deps.run(root, options.httpAddr, readiness, stdout, stderr)
}

// runServe contains the existing single-writer runtime. It is entered only
// after any requested readiness snapshot has been fully built.
func runServe(root, httpAddr string, readiness *readinesspilot.Snapshot, stdout, stderr io.Writer) int {
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
	deps := workbench.Deps{Readiness: readiness}
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
	srv.Backend.Readiness = readiness // R-W3-3: get_document's readiness section, same startup snapshot the board already renders (workbench.Deps{Readiness: readiness} above)
	srv.ErrLog = os.Stderr            // spec/fail-loud dc-3: a dropped socket connection leaves a trace, matching mcp.go's stdio scrutiny
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
