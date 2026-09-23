// Package gotestjson reads the `go test -json` event stream of one package,
// as the Go 1.25 toolchain writes it, into per-test outcomes. It is the one
// test2json reader shared by the per-test evidence producer (cmd/verdi) and
// the public-execution release checker (internal/publicrelease); each applies
// its own policy to the Result it returns.
//
// The event model follows Go 1.25.5's own documentation and source:
//   - cmd/test2json's package documentation: the TestEvent shape (Time,
//     Action, Package, Test, Elapsed, Output, FailedBuild) and the actions
//     start, run, pause, cont, pass, bench, fail, output, and skip;
//   - cmd/internal/test2json's event struct and Converter: the Key and Value
//     fields and the "attr" action (testing's "=== ATTR" line, T.Attr), and
//     the package's own terminal event, which Close always writes once
//     cmd/go has reported the test binary's exit (Converter.Exited);
//   - `go help buildjson` and cmd/go/internal/load's JSONPrinter: the
//     BuildEvent shape (ImportPath, Action, Output) and the "build-output"
//     and "build-fail" actions, interleaved with TestEvents and told apart by
//     Action;
//   - cmd/go/internal/test: every TestEvent's Package is the package's import
//     path, or, when the go command cannot load the package at all, the
//     package argument exactly as given on the command line; the package's
//     fail event carries FailedBuild when its build or setup failed.
//
// Decoding posture: strict, as CLAUDE.md requires of all JSON
// (DisallowUnknownFields plus trailing-data rejection; unknown enum values
// fail closed). The toolchain's event stream is not a ratified open contract
// the way forge-transport ac-1/dc-1 make provider responses one, so no
// relaxation applies. Every field decodes into its declared JSON type (a
// type mismatch fails), a field Event does not declare fails, anything after
// the last event fails, and an unknown Action fails closed; each such error
// names the event that carried it.
package gotestjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Event is one line of a `go test -json` stream: a TestEvent or a BuildEvent.
// Its fields are exactly the union of the two shapes Go 1.25.5 writes: the
// test2json event (src/cmd/internal/test2json/test2json.go, type event: Time,
// Action, Package, Test, Elapsed, Output, FailedBuild, Key, Value) and the
// build event (src/cmd/go/internal/load/printer.go, type jsonBuildEvent:
// ImportPath, Action, Output). ReadPackage decodes it with
// DisallowUnknownFields, so a toolchain upgrade that adds a field fails
// closed — an error, never a silently dropped field — until Verdi learns the
// field here.
type Event struct {
	Time        string `json:",omitempty"`
	Action      string
	Package     string  `json:",omitempty"`
	Test        string  `json:",omitempty"`
	Elapsed     float64 `json:",omitempty"`
	Output      string  `json:",omitempty"`
	FailedBuild string  `json:",omitempty"`
	Key         string  `json:",omitempty"`
	Value       string  `json:",omitempty"`
	ImportPath  string  `json:",omitempty"`
}

// The Go 1.25 actions. Any other Action is an error.
const (
	ActionStart       = "start"
	ActionRun         = "run"
	ActionPause       = "pause"
	ActionCont        = "cont"
	ActionPass        = "pass"
	ActionBench       = "bench"
	ActionFail        = "fail"
	ActionOutput      = "output"
	ActionSkip        = "skip"
	ActionAttr        = "attr"
	ActionBuildOutput = "build-output"
	ActionBuildFail   = "build-fail"
)

// Target names the one package a single `go test -json` invocation was asked
// to test.
type Target struct {
	// ImportPath is the package's import path: the Package the go command
	// reports on every TestEvent once the package loads. Required.
	ImportPath string
	// Arg is the package argument exactly as passed to go test (for example
	// "./internal/forge"): the Package the go command reports when it cannot
	// load the package at all (a missing directory, no Go files, a directory
	// in another module). Such a stream carries no test. Empty accepts only
	// ImportPath.
	Arg string
}

// Result is one package's stream, read to its end.
type Result struct {
	// Package is the Package every TestEvent carried: Target.ImportPath or
	// Target.Arg.
	Package string
	// Loaded reports whether the go command loaded the package (Package is
	// Target.ImportPath).
	Loaded bool
	// Outcome is the package's own terminal action: pass, fail, or skip.
	Outcome string
	// FailedBuild is the package fail event's FailedBuild: the package whose
	// build or setup failure caused this package's failure.
	FailedBuild string
	// BuildOutput and BuildFailed report whether any build-output or
	// build-fail event appeared.
	BuildOutput bool
	BuildFailed bool
	// Tests maps every test, top-level or subtest, whose own terminal event
	// appeared to that action: pass, fail, skip, or bench.
	Tests map[string]string
}

// Built reports whether the package built and loaded, so its tests could
// run: it loaded, and no build failure was reported.
func (r Result) Built() bool {
	return r.Loaded && !r.BuildFailed && r.FailedBuild == ""
}

// ReadPackage reads one package's `go test -json` stream to its end. It
// returns an error, and no Result, for a stream that cannot be the
// toolchain's account of that one package: undecodable JSON or trailing
// data, a field Event does not declare, an unknown action, a TestEvent
// naming another package, a BuildEvent carrying a TestEvent's Package or
// Test, a duplicate start, an event before start or after the package's
// terminal event, a run or terminal event out of sequence, a duplicate
// terminal event, the package completing while a test is still running, a
// test running despite a build failure or an unloaded package, or a stream
// that ends before the package's terminal event.
func ReadPackage(r io.Reader, want Target) (Result, error) {
	if want.ImportPath == "" {
		return Result{}, errors.New("gotestjson: target has no import path")
	}
	fail := func(format string, args ...any) (Result, error) {
		return Result{}, fmt.Errorf("gotestjson: %s: "+format, append([]any{want.ImportPath}, args...)...)
	}

	res := Result{Tests: map[string]string{}}
	running := map[string]bool{}
	started, finished, ran := false, false, false
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	for n := 1; ; n++ {
		var e Event
		if err := dec.Decode(&e); err == io.EOF {
			break
		} else if err != nil {
			return fail("event %d: %v", n, err)
		}
		if finished {
			return fail("event %d (%s) after the package's terminal event", n, e.Action)
		}

		switch e.Action {
		case ActionBuildOutput, ActionBuildFail:
			if e.Package != "" || e.Test != "" {
				return fail("event %d: build event carries a test event's Package or Test", n)
			}
			if e.Action == ActionBuildFail {
				res.BuildFailed = true
			} else {
				res.BuildOutput = true
			}
			continue
		case ActionStart, ActionRun, ActionPause, ActionCont, ActionPass, ActionBench, ActionFail, ActionOutput, ActionSkip, ActionAttr:
		default:
			return fail("event %d: unknown action %q", n, e.Action)
		}

		switch {
		case e.Package == "":
			return fail("event %d (%s) names no package", n, e.Action)
		case res.Package == "" && (e.Package == want.ImportPath || (want.Arg != "" && e.Package == want.Arg)):
			res.Package = e.Package
		case e.Package != res.Package:
			return fail("event %d names unexpected package %q", n, e.Package)
		}

		switch e.Action {
		case ActionStart:
			if started || e.Test != "" {
				return fail("event %d: duplicate or test-scoped start", n)
			}
			started = true
		case ActionOutput, ActionPause, ActionCont, ActionAttr:
			if !started {
				return fail("event %d (%s) before the package's start", n, e.Action)
			}
		case ActionRun:
			if !started || e.Test == "" || running[e.Test] {
				return fail("event %d: invalid run of %q", n, e.Test)
			}
			if _, done := res.Tests[e.Test]; done {
				return fail("event %d: %q ran again after its terminal event", n, e.Test)
			}
			running[e.Test] = true
			ran = true
		default: // pass, fail, skip, bench
			if !started {
				return fail("event %d (%s) before the package's start", n, e.Action)
			}
			if e.Test != "" {
				if _, dup := res.Tests[e.Test]; dup {
					return fail("event %d: duplicate terminal event for %q", n, e.Test)
				}
				if !running[e.Test] {
					return fail("event %d: terminal event for %q without a run", n, e.Test)
				}
				delete(running, e.Test)
				res.Tests[e.Test] = e.Action
				continue
			}
			if e.Action == ActionBench {
				return fail("event %d: package-level bench", n)
			}
			if len(running) > 0 {
				return fail("event %d: package completed with %d test(s) still running", n, len(running))
			}
			if e.FailedBuild != "" && e.Action != ActionFail {
				return fail("event %d: FailedBuild on a package %s", n, e.Action)
			}
			res.Outcome = e.Action
			res.FailedBuild = e.FailedBuild
			finished = true
		}
	}

	if !finished {
		return fail("stream ended before the package's terminal event (truncated)")
	}
	res.Loaded = res.Package == want.ImportPath
	if !res.Loaded && (ran || res.Outcome != ActionFail) {
		return fail("unloaded package %q reports test execution or a %s outcome", res.Package, res.Outcome)
	}
	if (res.BuildFailed || res.FailedBuild != "") && ran {
		return fail("tests ran despite a build failure")
	}
	return res, nil
}
