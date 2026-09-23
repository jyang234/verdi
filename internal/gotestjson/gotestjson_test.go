package gotestjson

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const (
	pkg    = "example.com/m/p"
	pkgArg = "./p"
)

var target = Target{ImportPath: pkg, Arg: pkgArg}

// ev renders one TestEvent line for pkg; extra holds additional
// "Key":value JSON members appended verbatim.
func ev(action, test string, extra ...string) string {
	return evFor(pkg, action, test, extra...)
}

func evFor(pkgName, action, test string, extra ...string) string {
	fields := []string{`"Action":` + quote(action), `"Package":` + quote(pkgName)}
	if test != "" {
		fields = append(fields, `"Test":`+quote(test))
	}
	fields = append(fields, extra...)
	return "{" + strings.Join(fields, ",") + "}"
}

// build renders one BuildEvent line (`go help buildjson`).
func build(action, importPath string, extra ...string) string {
	fields := append([]string{`"ImportPath":` + quote(importPath), `"Action":` + quote(action)}, extra...)
	return "{" + strings.Join(fields, ",") + "}"
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func stream(lines ...string) string { return strings.Join(lines, "\n") + "\n" }

// Each want is checked field by field; nil Tests means "no test outcome".
type want struct {
	pkg         string
	loaded      bool
	built       bool
	outcome     string
	failedBuild string
	buildOutput bool
	buildFailed bool
	tests       map[string]string
}

func TestReadPackage(t *testing.T) {
	passed := want{pkg: pkg, loaded: true, built: true, outcome: ActionPass}
	withTests := func(w want, tests map[string]string) want { w.tests = tests; return w }

	cases := []struct {
		name   string
		in     string
		target Target
		want   *want // nil: an error is required
	}{
		// --- the toolchain's own shapes (go1.25.5, Time fields dropped) ---
		{"pass", stream(ev("start", ""), ev("run", "TestA"), ev("output", "TestA", `"Output":"=== RUN   TestA\n"`), ev("pass", "TestA", `"Elapsed":0`), ev("output", "", `"Output":"PASS\n"`), ev("pass", "", `"Elapsed":0.1`)), target,
			ptr(withTests(passed, map[string]string{"TestA": ActionPass}))},
		{"fail", stream(ev("start", ""), ev("run", "TestA"), ev("fail", "TestA"), ev("fail", "")), target,
			&want{pkg: pkg, loaded: true, built: true, outcome: ActionFail, tests: map[string]string{"TestA": ActionFail}}},
		{"skip", stream(ev("start", ""), ev("run", "TestA"), ev("skip", "TestA"), ev("pass", "")), target,
			ptr(withTests(passed, map[string]string{"TestA": ActionSkip}))},
		{"named test absent", stream(ev("start", ""), ev("run", "TestB"), ev("pass", "TestB"), ev("pass", "")), target,
			ptr(withTests(passed, map[string]string{"TestB": ActionPass}))},
		{"parent fails after its subtest passes", stream(ev("start", ""), ev("run", "TestP"), ev("run", "TestP/sub"), ev("pass", "TestP/sub"), ev("output", "TestP", `"Output":"--- FAIL: TestP\n"`), ev("fail", "TestP"), ev("fail", "")), target,
			&want{pkg: pkg, loaded: true, built: true, outcome: ActionFail, tests: map[string]string{"TestP": ActionFail, "TestP/sub": ActionPass}}},
		{"parallel subtest pauses and continues", stream(ev("start", ""), ev("run", "TestC"), ev("run", "TestC/s"), ev("pause", "TestC/s"), ev("cont", "TestC/s"), ev("pass", "TestC/s"), ev("pass", "TestC"), ev("pass", "")), target,
			ptr(withTests(passed, map[string]string{"TestC": ActionPass, "TestC/s": ActionPass}))},
		{"interleaved output", stream(ev("start", ""), ev("run", "TestA"), ev("run", "TestB"), ev("output", "TestA", `"Output":"a\n"`), ev("output", "TestB", `"Output":"b\n"`), ev("pass", "TestB"), ev("pass", "TestA"), ev("pass", "")), target,
			ptr(withTests(passed, map[string]string{"TestA": ActionPass, "TestB": ActionPass}))},
		{"attr event (T.Attr)", stream(ev("start", ""), ev("run", "TestAttr"), ev("attr", "TestAttr", `"Key":"k"`, `"Value":"v"`), ev("pass", "TestAttr"), ev("pass", "")), target,
			ptr(withTests(passed, map[string]string{"TestAttr": ActionPass}))},
		{"benchmark terminal", stream(ev("start", ""), ev("run", "BenchmarkX"), ev("bench", "BenchmarkX"), ev("pass", "")), target,
			ptr(withTests(passed, map[string]string{"BenchmarkX": ActionBench}))},
		{"no test files", stream(ev("start", ""), ev("output", "", `"Output":"?   \texample.com/m/p\t[no test files]\n"`), ev("skip", "")), target,
			&want{pkg: pkg, loaded: true, built: true, outcome: ActionSkip}},
		{"build failure", stream(
			build("build-output", pkg+" ["+pkg+".test]", `"Output":"# example.com/m/p\n"`),
			build("build-output", pkg+" ["+pkg+".test]", `"Output":"p/x_test.go:5:2: undefined: nope\n"`),
			build("build-fail", pkg+" ["+pkg+".test]"),
			ev("start", ""), ev("output", "", `"Output":"FAIL\texample.com/m/p [build failed]\n"`), ev("fail", "", `"Elapsed":0`, `"FailedBuild":"example.com/m/p [example.com/m/p.test]"`)), target,
			&want{pkg: pkg, loaded: true, built: false, outcome: ActionFail, failedBuild: pkg + " [" + pkg + ".test]", buildOutput: true, buildFailed: true}},
		{"unloaded package directory", stream(
			build("build-output", pkgArg, `"Output":"# ./p\n"`),
			build("build-output", pkgArg, `"Output":"stat /m/p: directory not found\n"`),
			build("build-fail", pkgArg),
			evFor(pkgArg, "start", ""), evFor(pkgArg, "output", "", `"Output":"FAIL\t./p [setup failed]\n"`), evFor(pkgArg, "fail", "", `"FailedBuild":"./p"`)), target,
			&want{pkg: pkgArg, loaded: false, built: false, outcome: ActionFail, failedBuild: pkgArg, buildOutput: true, buildFailed: true}},
		{"package fail carrying FailedBuild without a build-fail event", stream(ev("start", ""), ev("output", "", `"Output":"FAIL\texample.com/m/p [build failed]\n"`), ev("fail", "", `"FailedBuild":"example.com/m/p [example.com/m/p.test]"`)), target,
			&want{pkg: pkg, loaded: true, built: false, outcome: ActionFail, failedBuild: pkg + " [" + pkg + ".test]"}},
		{"unloaded package, GODEBUG gotestjsonbuildtext=1 (no build events, no FailedBuild)", stream(evFor(pkgArg, "start", ""), evFor(pkgArg, "output", "", `"Output":"FAIL\t./p [setup failed]\n"`), evFor(pkgArg, "fail", "")), target,
			&want{pkg: pkgArg, loaded: false, built: false, outcome: ActionFail}},
		{"additive unknown fields are ignored", stream(ev("start", "", `"Surprise":{"x":[1]}`), ev("run", "TestA", `"Source":"new"`), ev("pass", "TestA", `"OutputType":"frame"`), ev("pass", "")), target,
			ptr(withTests(passed, map[string]string{"TestA": ActionPass}))},
		{"build output without failure", stream(build("build-output", pkg, `"Output":"# cgo warning\n"`), ev("start", ""), ev("run", "TestA"), ev("pass", "TestA"), ev("pass", "")), target,
			&want{pkg: pkg, loaded: true, built: true, outcome: ActionPass, buildOutput: true, tests: map[string]string{"TestA": ActionPass}}},

		// --- streams that cannot be the toolchain's account of one package ---
		{"known field of the wrong type", stream(ev("start", ""), `{"Action":"run","Package":"example.com/m/p","Test":5}`, ev("pass", "")), target, nil},
		{"malformed line", stream(ev("start", ""), "not json"), target, nil},
		{"truncated before the package's terminal event", stream(ev("start", ""), ev("run", "TestA"), ev("pass", "TestA")), target, nil},
		{"empty stream", "", target, nil},
		{"unknown action", stream(ev("start", ""), ev("invented", ""), ev("pass", "")), target, nil},
		{"null event", stream(ev("start", ""), "null", ev("pass", "")), target, nil},
		{"supplied success", stream(`{"pass":true}`), target, nil},
		{"another package", stream(ev("start", ""), evFor("example.com/m/q", "run", "TestA"), ev("pass", "")), target, nil},
		{"relative package path instead of the import path", stream(evFor("p", "start", ""), evFor("p", "pass", "")), target, nil},
		{"identity switches from import path to argument", stream(ev("start", ""), evFor(pkgArg, "pass", "")), target, nil},
		{"argument identity without Target.Arg", stream(evFor(pkgArg, "start", ""), evFor(pkgArg, "fail", "")), Target{ImportPath: pkg}, nil},
		{"unloaded package runs a test", stream(evFor(pkgArg, "start", ""), evFor(pkgArg, "run", "TestA"), evFor(pkgArg, "fail", "TestA"), evFor(pkgArg, "fail", "")), target, nil},
		{"unloaded package passes", stream(evFor(pkgArg, "start", ""), evFor(pkgArg, "pass", "")), target, nil},
		{"test event without a package", stream(ev("start", ""), `{"Action":"output","Output":"x"}`, ev("pass", "")), target, nil},
		{"build event carrying a Package", stream(`{"ImportPath":"example.com/m/p","Action":"build-fail","Package":"example.com/m/p"}`, ev("start", ""), ev("fail", "")), target, nil},
		{"duplicate start", stream(ev("start", ""), ev("start", ""), ev("pass", "")), target, nil},
		{"test-scoped start", stream(ev("start", "TestA"), ev("pass", "")), target, nil},
		{"output before start", stream(ev("output", "", `"Output":"x\n"`), ev("start", ""), ev("pass", "")), target, nil},
		{"attr before start", stream(ev("attr", "TestA", `"Key":"k"`), ev("start", ""), ev("pass", "")), target, nil},
		{"run before start", stream(ev("run", "TestA"), ev("start", ""), ev("pass", "")), target, nil},
		{"run without a test name", stream(ev("start", ""), ev("run", ""), ev("pass", "")), target, nil},
		{"terminal event without a run", stream(ev("start", ""), ev("pass", "TestA"), ev("pass", "")), target, nil},
		{"duplicate terminal event", stream(ev("start", ""), ev("run", "TestA"), ev("pass", "TestA"), ev("fail", "TestA"), ev("fail", "")), target, nil},
		{"run again after its terminal event", stream(ev("start", ""), ev("run", "TestA"), ev("pass", "TestA"), ev("run", "TestA"), ev("pass", "TestA"), ev("pass", "")), target, nil},
		{"duplicate run while running", stream(ev("start", ""), ev("run", "TestA"), ev("run", "TestA"), ev("pass", "TestA"), ev("pass", "")), target, nil},
		{"package passes with a test still running", stream(ev("start", ""), ev("run", "TestA"), ev("pass", "")), target, nil},
		{"package fails with a test still running (test called os.Exit(1))", stream(ev("start", ""), ev("run", "TestExits"), ev("output", "TestExits", `"Output":"=== RUN   TestExits\n"`), ev("output", "", `"Output":"FAIL\texample.com/m/p\t0.6s\n"`), ev("fail", "")), target, nil},
		{"event after the package's terminal event", stream(ev("start", ""), ev("pass", ""), ev("output", "", `"Output":"x\n"`)), target, nil},
		{"terminal event after the package's terminal event", stream(ev("start", ""), ev("run", "TestA"), ev("pass", "TestA"), ev("pass", ""), ev("pass", "")), target, nil},
		{"package-level bench", stream(ev("start", ""), ev("bench", "")), target, nil},
		{"FailedBuild on a package pass", stream(ev("start", ""), ev("pass", "", `"FailedBuild":"x"`)), target, nil},
		{"a test ran despite a build failure", stream(build("build-fail", pkg), ev("start", ""), ev("run", "TestA"), ev("fail", "TestA"), ev("fail", "")), target, nil},
		{"target without an import path", stream(ev("start", ""), ev("pass", "")), Target{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ReadPackage(strings.NewReader(c.in), c.target)
			if c.want == nil {
				if err == nil {
					t.Fatalf("ReadPackage = %+v, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadPackage: %v", err)
			}
			w := *c.want
			if w.tests == nil {
				w.tests = map[string]string{}
			}
			if got.Package != w.pkg || got.Loaded != w.loaded || got.Built() != w.built || got.Outcome != w.outcome ||
				got.FailedBuild != w.failedBuild || got.BuildOutput != w.buildOutput || got.BuildFailed != w.buildFailed {
				t.Errorf("ReadPackage = {Package:%q Loaded:%v Built:%v Outcome:%q FailedBuild:%q BuildOutput:%v BuildFailed:%v}, want %+v",
					got.Package, got.Loaded, got.Built(), got.Outcome, got.FailedBuild, got.BuildOutput, got.BuildFailed, w)
			}
			if !reflect.DeepEqual(got.Tests, w.tests) {
				t.Errorf("Tests = %v, want %v", got.Tests, w.tests)
			}
		})
	}
}

func ptr(w want) *want { return &w }

// TestEventRoundTrip proves Event carries every Go 1.25 field under its
// toolchain name: a TestEvent with Key/Value and a BuildEvent with
// ImportPath decode into the documented fields and re-encode losslessly.
func TestEventRoundTrip(t *testing.T) {
	for _, line := range []string{
		`{"Time":"2026-09-22T22:41:53.758675-04:00","Action":"attr","Package":"example.com/m/p","Test":"TestAttr","Key":"k","Value":"v"}`,
		`{"Time":"2026-09-22T22:41:53.759085-04:00","Action":"pass","Package":"example.com/m/p","Elapsed":0.962}`,
		`{"Action":"fail","Package":"example.com/m/p","FailedBuild":"example.com/m/p [example.com/m/p.test]"}`,
		`{"Action":"build-output","Output":"# example.com/m/p\n","ImportPath":"example.com/m/p [example.com/m/p.test]"}`,
	} {
		var e Event
		dec := json.NewDecoder(strings.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			t.Fatalf("decoding %s: %v", line, err)
		}
		var original, again map[string]any
		if err := json.Unmarshal([]byte(line), &original); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &again); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(original, again) {
			t.Errorf("round trip of %s = %s", line, b)
		}
	}
}
