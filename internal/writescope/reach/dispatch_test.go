package reach_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/writescope/reach"
)

func entryNames(entries []reach.Entry) []string {
	var out []string
	for _, e := range entries {
		out = append(out, e.Name)
	}
	sort.Strings(out)
	return out
}

// reachByName builds one graph over entries and returns, per entry name,
// the names of the synthetic targets it reaches.
func reachByName(t *testing.T, prog *reach.Program, entries []reach.Entry) map[string]string {
	t.Helper()
	g, err := reach.Build(prog, entries)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		out[e.Name] = strings.Join(hitNames(mustReach(t, g, e, synthTargets(t, prog))), ",")
	}
	return out
}

func testCLIEntriesDeriveVerbsFromTheDispatcher(t *testing.T, prog *reach.Program) {
	entries, err := reach.CLIEntries(prog, "example.com/synth/cli", "Run", "cli")
	if err != nil {
		t.Fatalf("CLIEntries: %v", err)
	}
	want := []string{"alias", "alias2", "direct", "ds", "mcp", "op", "op a", "op b", "qc", "qi", "serve", "sub", "sub --fast", "sub read", "sub write"}
	if got := entryNames(entries); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("CLI entries = %q, want %q", got, want)
	}
	for _, e := range entries {
		if e.Surface != "cli" {
			t.Fatalf("entry %q has surface %q, want cli", e.Name, e.Surface)
		}
	}
	got := reachByName(t, prog, entries)
	tests := []struct {
		verb string
		want string
	}{
		{"direct", "Mutate"},     // if-arm on the verb
		{"alias", ""},            // one arm of an || condition
		{"ds", "Mutate"},         // R1-A3: delegates into sub's arms, which are not its own, so they are traversed
		{"sub", ""},              // a dispatcher's own code: its arms are cut
		{"sub write", "Mutate"},  // switch on args[0] one level down
		{"sub read", ""},         // sibling arm, read-only
		{"sub --fast", "Mutate"}, // guarded if-arm: len(args) > 0 && args[0] == "--fast"
		{"op", ""},               // key held in a variable and validated by an empty-bodied switch
		{"op a", "Mutate"},       // key handed to a helper whose switch dispatches it
		{"op b", ""},             // sibling, read-only
	}
	for _, tt := range tests {
		t.Run(tt.verb, func(t *testing.T) {
			if got[tt.verb] != tt.want {
				t.Fatalf("%s reaches %q, want %q", tt.verb, got[tt.verb], tt.want)
			}
		})
	}
}

func testCLIEntriesErrors(t *testing.T, prog *reach.Program) {
	tests := []struct {
		name, pkg, fn string
	}{
		{"unknown package", "example.com/synth/nope", "Run"},
		{"unknown function", "example.com/synth/cli", "Nope"},
		{"function without a []string first parameter", "example.com/synth/cli", "Phase"},
		{"dispatcher with no arms", "example.com/synth/cli", "code"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.CLIEntries(prog, tt.pkg, tt.fn, "cli"); err == nil {
				t.Fatalf("CLIEntries(%s, %s) succeeded, want an error", tt.pkg, tt.fn)
			}
		})
	}
}

// testPreDispatchEntryReachesWhatRunsForEveryVerb pins R1-A2 (ledger
// SI-314 (3)): main, the dispatcher outside its arms, every init function
// and every package-level variable initializer that runs code, in the
// packages the binary links, form one pseudo-entry, and what it reaches is
// visible; a variable that only names a function runs nothing.
func testPreDispatchEntryReachesWhatRunsForEveryVerb(t *testing.T, prog *reach.Program) {
	tests := []struct {
		name, pkg string
		want      string
		verbs     map[string]string
	}{
		{"a clean dispatcher", "example.com/synth/cli", "", nil},
		{"a preamble, an initializer that runs code, and another package's init", "example.com/synth/precli",
			"Mutate,Other,Publish", map[string]string{"lint": "", "later": "Prune"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pre, err := reach.PreDispatchEntry(prog, tt.pkg, "Run", "cli")
			if err != nil {
				t.Fatalf("PreDispatchEntry: %v", err)
			}
			if pre.Name != reach.PreDispatch {
				t.Fatalf("pre-dispatch entry is named %q, want %q", pre.Name, reach.PreDispatch)
			}
			verbs, err := reach.CLIEntries(prog, tt.pkg, "Run", "cli")
			if err != nil {
				t.Fatalf("CLIEntries: %v", err)
			}
			got := reachByName(t, prog, append(verbs, pre))
			if got[reach.PreDispatch] != tt.want {
				t.Fatalf("pre-dispatch reaches %q, want %q", got[reach.PreDispatch], tt.want)
			}
			for verb, want := range tt.verbs {
				if got[verb] != want {
					t.Fatalf("%s reaches %q, want %q", verb, got[verb], want)
				}
			}
		})
	}
}

func testPreDispatchEntryErrors(t *testing.T, prog *reach.Program) {
	for _, tt := range []struct{ name, pkg, fn string }{
		{"unknown package", "example.com/synth/nope", "Run"},
		{"unknown dispatcher", "example.com/synth/cli", "Nope"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.PreDispatchEntry(prog, tt.pkg, tt.fn, "cli"); err == nil {
				t.Fatalf("PreDispatchEntry(%s, %s) succeeded, want an error", tt.pkg, tt.fn)
			}
		})
	}
}

func testSwitchEntriesMatchTheInventorysOneSwitch(t *testing.T, prog *reach.Program) {
	entries, err := reach.SwitchEntries(prog, "example.com/synth/tools", "mcp", []string{"write_tool", "read_tool"})
	if err != nil {
		t.Fatalf("SwitchEntries: %v", err)
	}
	if got := entryNames(entries); strings.Join(got, ",") != "read_tool,write_tool" {
		t.Fatalf("entries = %v", got)
	}
	got := reachByName(t, prog, entries)
	if got["write_tool"] != "Mutate" || got["read_tool"] != "" {
		t.Fatalf("reach = %v, want write_tool -> Mutate and read_tool -> nothing", got)
	}
}

func testSwitchEntriesErrors(t *testing.T, prog *reach.Program) {
	tests := []struct {
		name  string
		names []string
	}{
		{"no names", nil},
		{"a name no switch carries", []string{"write_tool", "missing_tool"}},
		{"a subset of a switch's cases", []string{"write_tool"}},
		{"duplicate name", []string{"write_tool", "write_tool", "read_tool"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.SwitchEntries(prog, "example.com/synth/tools", "mcp", tt.names); err == nil {
				t.Fatalf("SwitchEntries(%v) succeeded, want an error", tt.names)
			}
		})
	}
}

func testRouteEntriesDeriveRoutesAndActions(t *testing.T, prog *reach.Program) {
	entries, err := reach.RouteEntries(prog, "example.com/synth/web", "workbench")
	if err != nil {
		t.Fatalf("RouteEntries: %v", err)
	}
	want := []string{
		"/audit",
		"/b/{branch}/thing/{name}",
		"/b/{branch}/thing/{name}/api/look",
		"/b/{branch}/thing/{name}/api/mutate",
		"/b/{branch}/thing/{name}/api/peek",
		"/b/{branch}/thing/{name}/api/push",
		"/b/{branch}/thing/{name}/api/{action}",
		"/captured/thing/{name}/api/{action}",
		"/health",
		"/iface/thing/{name}/api/{action}",
		"/ifacemod/thing/{name}/api/{action}",
		"/ifacestruct",
		"/ifield/thing/{name}/api/{action}",
		"/legacy/{key}/commit",
		"/legacy/{key}/save",
		"/legacy/{key}/{action}",
		"/quick/thing/{name}/api/{action}",
		"/static",
		"/thing/{name}",
		"/thing/{name}/api/look",
		"/thing/{name}/api/mutate",
		"/thing/{name}/api/peek",
		"/thing/{name}/api/push",
		"/thing/{name}/api/{action}",
		"/wrapped/thing/{name}/api/look",
		"/wrapped/thing/{name}/api/mutate",
		"/wrapped/thing/{name}/api/peek",
		"/wrapped/thing/{name}/api/push",
		"/wrapped/thing/{name}/api/{action}",
		"/wrappedlit/thing/{name}/api/{action}",
	}
	if got := entryNames(entries); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("route entries =\n%q\nwant\n%q", got, want)
	}
	got := reachByName(t, prog, entries)
	tests := []struct {
		route string
		want  string
	}{
		{"/thing/{name}/api/push", "Publish"},                  // switch arm in the handler closure
		{"/thing/{name}/api/mutate", "Mutate"},                 // arm of a helper the key is handed to
		{"/thing/{name}/api/look", ""},                         // read-only arm
		{"/thing/{name}/api/{action}", ""},                     // the route itself: its arms are cut
		{"/legacy/{key}/commit", "Mutate"},                     // switch directly on r.PathValue
		{"/b/{branch}/thing/{name}", "Mutate"},                 // prefix mount's own work only, from a route table
		{"/b/{branch}/thing/{name}/api/{action}", "Mutate"},    // the prefix's own work; the row's actions are cut
		{"/b/{branch}/thing/{name}/api/push", "Publish"},       // R1-A1: the row's handler, split on {action} under the prefix
		{"/b/{branch}/thing/{name}/api/mutate", "Mutate"},      // the row's helper arm under the prefix
		{"/b/{branch}/thing/{name}/api/look", ""},              // read-only arm under the prefix
		{"/audit", "Mutate"},                                   // a function-typed field Register filled, through a parameter and a second field
		{"/quick/thing/{name}/api/{action}", "Mutate,Publish"}, // R1-A3: wraps the API handler; its actions are another route's, so they are traversed
		// R1-RR1 (ledger SI-318): the API handler reached through a local
		// the registration captured (N1a), a wrapper's argument (N1b, whose
		// actions are derived under the wrapping route), and a wrapper
		// handed a literal that calls it (N1c).
		{"/captured/thing/{name}/api/{action}", "Mutate,Publish"},
		{"/wrapped/thing/{name}/api/push", "Publish"},
		{"/wrapped/thing/{name}/api/mutate", "Mutate"},
		{"/wrappedlit/thing/{name}/api/{action}", "Mutate,Publish"},
		// R1-RR5 (ledger SI-319, SI-320): the API handler held in an
		// interface value, captured (H6, and H6b through a module function
		// type) or read from an interface-typed field (Publish comes only
		// through the flow); a struct behind the same interface is
		// dispatched by class-hierarchy analysis alone.
		{"/iface/thing/{name}/api/{action}", "Mutate,Publish"},
		{"/ifacemod/thing/{name}/api/{action}", "Mutate,Publish"},
		{"/ifield/thing/{name}/api/{action}", "Mutate,Publish"},
		{"/ifacestruct", "Mutate"}, // a struct behind http.Handler: CHA dispatches every module implementation (app's handler mutates); nothing more, no error
		{"/thing/{name}", ""},      // table row's handler, read-only
		{"/health", ""},
	}
	for _, tt := range tests {
		t.Run(tt.route, func(t *testing.T) {
			if got[tt.route] != tt.want {
				t.Fatalf("%s reaches %q, want %q", tt.route, got[tt.route], tt.want)
			}
		})
	}
}

// testRouteEntriesFailClosedOnUnresolvedFieldCalls pins R1-A1's fail-closed
// half: inside a route's reach, a call through a route table's
// function-typed field its registration did not bind, or through a field
// holding a value no static evaluation can follow, is an error, never a
// route that reaches nothing.
func testRouteEntriesFailClosedOnUnresolvedFieldCalls(t *testing.T, prog *reach.Program) {
	entries, err := reach.RouteEntries(prog, "example.com/synth/strictweb", "workbench")
	if err != nil {
		t.Fatalf("RouteEntries: %v", err)
	}
	if got := entryNames(entries); strings.Join(got, "|") != "/opaque|/p/x|/stray" {
		t.Fatalf("route entries = %q", got)
	}
	g, err := reach.Build(prog, entries)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	tests := []struct {
		route   string
		wantErr string // "" for no error
	}{
		{"/p/x", ""},
		{"/stray", "handler"},
		{"/opaque", "hook"},
	}
	byName := map[string]reach.Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	for _, tt := range tests {
		t.Run(tt.route, func(t *testing.T) {
			hits, err := g.Reach(byName[tt.route], synthTargets(t, prog))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("Reach(%s) = %v, want no error", tt.route, err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("Reach(%s) = %v, nil; want an error naming %q (fail closed)", tt.route, hitNames(hits), tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("Reach(%s) = %v, want it to name %q", tt.route, err, tt.wantErr)
			}
		})
	}
}

// testRouteEntriesFailClosedOnUnfollowableFunctionValues pins SI-318's
// fail-closed shapes: a route that calls a function value the analysis
// cannot follow is an error naming the site, never a route that reaches
// nothing; a pointer loaded from a package-level variable is followed.
func testRouteEntriesFailClosedOnUnfollowableFunctionValues(t *testing.T, prog *reach.Program) {
	entries, err := reach.RouteEntries(prog, "example.com/synth/holes", "workbench")
	if err != nil {
		t.Fatalf("RouteEntries: %v", err)
	}
	g, err := reach.Build(prog, entries)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	byName := map[string]reach.Entry{}
	for _, e := range entries {
		byName[e.Name] = e
	}
	tests := []struct {
		route   string
		wantErr string // "" for no error
	}{
		{"/hole/argument", "direct"},    // a captured function value passed on as an argument
		{"/hole/receive", "<-s.ch"},     // a channel receive yielding a function value
		{"/hole/assert", "anyHook"},     // a type assertion to a function type
		{"/hole/deref", "*p"},           // a dereference of a pointer loaded from a local map
		{"/hole/element", "s.handlers"}, // an element of a field-held container
		{"/hole/switch", "switches"},    // R1-RR6: a type switch to a function type
		{"/hole/ifacechan", "<-s.hch"},  // SI-319: a channel of http.Handler
		{"/hole/ifaceslice", "s.hs"},    // SI-319: a field-held slice of http.Handler
		// SI-320: a function laundered through the empty interface or a
		// type parameter fails closed where it surfaces.
		{"/launder/assert", "anyH.(http.Handler)"},    // (i) a captured any asserted to http.Handler and called
		{"/launder/map", `m["k"].(http.HandlerFunc)`}, // (ii) a captured map[string]any's element asserted to a function type
		{"/launder/helper", "v.(http.Handler)"},       // (iii) a captured any handed to a helper that asserts it
		{"/launder/typeparam", "captured"},            // (iv) a captured ~func type parameter passed on as an argument
		{"/control/packagederef", ""},                 // a pointer loaded from a package-level variable
		{"/control/benign", ""},                       // a field-held container's length, keys, and nil comparison
		{"/control/capturednil", ""},                  // a captured function compared to nil, then called (resolved)
	}
	for _, tt := range tests {
		t.Run(tt.route, func(t *testing.T) {
			e, ok := byName[tt.route]
			if !ok {
				t.Fatalf("no route %s; got %v", tt.route, entryNames(entries))
			}
			hits, err := g.Reach(e, synthTargets(t, prog))
			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("Reach(%s) = %v, want no error", tt.route, err)
			case tt.route == "/control/capturednil" && strings.Join(hitNames(hits), ",") != "Mutate":
				t.Fatalf("Reach(%s) = %v, want [Mutate]: the captured call resolves through the flow", tt.route, hitNames(hits))
			case tt.wantErr != "" && err == nil:
				t.Fatalf("Reach(%s) = %v, nil; want a fail-closed error naming %q", tt.route, hitNames(hits), tt.wantErr)
			case tt.wantErr != "" && (!strings.Contains(err.Error(), tt.wantErr) || !strings.Contains(err.Error(), "fail closed")):
				t.Fatalf("Reach(%s) = %v, want a fail-closed error naming %q", tt.route, err, tt.wantErr)
			}
		})
	}
}

func testRouteEntriesFailClosedOnAnUnresolvableRegistration(t *testing.T, prog *reach.Program) {
	tests := []struct {
		name, pkg string
	}{
		{"computed pattern", "example.com/synth/badweb"},
		{"package without registrations", "example.com/synth/app"},
		{"unknown package", "example.com/synth/nope"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.RouteEntries(prog, tt.pkg, "workbench"); err == nil {
				t.Fatalf("RouteEntries(%s) succeeded, want an error", tt.pkg)
			}
		})
	}
}

func testStringKeyedMap(t *testing.T, prog *reach.Program) {
	got, err := reach.StringKeyedMap(prog, "example.com/synth/cli", "verbs")
	if err != nil {
		t.Fatalf("StringKeyedMap: %v", err)
	}
	want := map[string]string{"direct": "1", "sub": "2", "op": "3", "gone": "0"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("got[%q] = %q, want %q", k, got[k], v)
		}
	}
	for _, tt := range []struct{ name, pkg, v string }{
		{"not a map literal", "example.com/synth/cli", "notAMap"},
		{"unknown variable", "example.com/synth/cli", "nope"},
		{"unknown package", "example.com/synth/nope", "verbs"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := reach.StringKeyedMap(prog, tt.pkg, tt.v); err == nil {
				t.Fatalf("StringKeyedMap(%s, %s) succeeded, want an error", tt.pkg, tt.v)
			}
		})
	}
}
