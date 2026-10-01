package writescope_test

import (
	"bytes"
	"context"
	"encoding/json"
	"go/types"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/mcpserve"
	ws "github.com/jyang234/verdi/internal/writescope"
	"github.com/jyang234/verdi/internal/writescope/reach"
)

// The anchors: where each inventory lives. They name where entry points
// are defined, never the entry points themselves, which are derived from
// the code at each anchor (story dc-2).
const (
	cliPackage       = "cmd/verdi"
	cliDispatcher    = "run"       // dispatch.go: one arm per verb
	cliInventory     = "verbPhase" // dispatch.go: the CLI-verb inventory
	mcpPackage       = "internal/mcpserve"
	workbenchPackage = "internal/workbench"
	gitxPackage      = "internal/gitx"
	gitDirLocator    = "internal/gitx.CommonDir"
)

// TestRegistry_CoversEveryMutatingVerb is the static witness of
// spec/write-scope-registry ac-1 (obligation
// write-scope-registry--ac-1--static): every declaration validates; every
// exported gitx function and every function outside gitx that writes under
// a git directory is classified and every classified name exists; every
// CLI verb, workbench route or action, and MCP tool that reaches a
// mutating function is named by exactly one declaration; no declaration
// names a verb that reaches none; and the awaiting-fix list is reported
// with its count.
func TestRegistry_CoversEveryMutatingVerb(t *testing.T) {
	start := time.Now()
	facts := analyzeModule(t, filepath.Join("..", ".."))

	findings := ws.Check(ws.Registry(), ws.AwaitingFixes(), ws.Classification(), facts)
	for _, f := range findings {
		t.Error(f)
	}

	fixes := ws.AwaitingFixes()
	t.Logf("awaiting fix (%d), declared scoped until spec/ritual-effect-witness fixes them:", len(fixes))
	for _, a := range fixes {
		t.Logf("  %s [%s]: %s", a.Path, a.Ritual, a.Defect)
	}
	mutating := 0
	for _, hits := range facts.Verbs {
		if len(hits) > 0 {
			mutating++
		}
	}
	t.Logf("%d of %d verbs reach a mutating function; %d declarations; %d gitx exports; %d git-directory writers; union of %d targets; %v",
		mutating, len(facts.Verbs), len(ws.Registry()), len(facts.GitxExports), len(facts.GitDirWriters),
		len(reach.Targets()), time.Since(start).Round(time.Millisecond))

	// The falsifiers, on these same facts: each mutation of the registry,
	// its awaiting-fix list, or the classification turns the witness red.
	for _, m := range liveMutants() {
		t.Run(m.name, func(t *testing.T) {
			decls, awaiting, classes := ws.Registry(), ws.AwaitingFixes(), ws.Classification()
			m.mutate(&decls, &awaiting, &classes)
			got := strings.Join(ws.Check(decls, awaiting, classes, facts), "\n")
			if !strings.Contains(got, m.want) {
				t.Fatalf("the witness stayed green under %q (findings:\n%s\n); want a finding mentioning %q", m.name, got, m.want)
			}
		})
	}
}

// liveMutant is one mutation the witness must catch on the module's own
// facts.
type liveMutant struct {
	name   string
	mutate func(*[]ws.Declaration, *[]ws.AwaitingFix, *[]ws.Classified)
	want   string
}

// branchBoardCommit is the board's Commit and push under the /b/{branch}
// managed-worktree mount (ledger SI-314 (1)).
const branchBoardCommit = "/b/{branch}/board/spec/{name}/api/git-commit"

func liveMutants() []liveMutant {
	return []liveMutant{
		{"undeclaring the /b/ git-commit action (R1-B1)", func(d *[]ws.Declaration, _ *[]ws.AwaitingFix, _ *[]ws.Classified) {
			dropVerb(*d, ws.Workbench(branchBoardCommit))
		}, "verb workbench:" + branchBoardCommit + " reaches internal/gitx.AddAll"},
	}
}

// dropVerb removes v from whichever declaration names it.
func dropVerb(decls []ws.Declaration, v ws.Verb) {
	for i := range decls {
		var kept []ws.Verb
		for _, have := range decls[i].Verbs {
			if have != v {
				kept = append(kept, have)
			}
		}
		decls[i].Verbs = kept
	}
}

// analyzeModule computes the facts for the module at root, as the union
// over every release target, so a platform-specific file is never missed.
func analyzeModule(t *testing.T, root string) ws.Facts {
	t.Helper()
	ctx := context.Background()
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatalf("resolving %s: %v", root, err)
	}
	tools := liveMCPTools(t, root)
	facts := ws.Facts{GitxPackage: gitxPackage, Functions: map[string]bool{}, Verbs: map[ws.Verb][]ws.Hit{}}
	exports := map[string]bool{}
	writers := map[string]ws.Writer{}
	for _, target := range reach.Targets() {
		prog, err := reach.Load(ctx, root, target, "./"+cliPackage)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		mod := prog.Module + "/"

		gitxExports, err := reach.ExportedFuncs(prog, mod+gitxPackage)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		for _, e := range gitxExports {
			exports[e] = true
		}
		locator := prog.FuncByName(gitDirLocator)
		if locator == nil {
			t.Fatalf("%s: the git-directory locator %s no longer exists", target, gitDirLocator)
		}
		found, err := reach.GitDirWriters(prog, []*types.Func{locator}, []string{mod + gitxPackage})
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		for _, w := range found {
			name := prog.FuncName(w.Func)
			if _, seen := writers[name]; !seen {
				at := w.At
				if rel, err := filepath.Rel(absRoot, w.At); err == nil {
					at = filepath.ToSlash(rel)
				}
				writers[name] = ws.Writer{Func: name, At: at}
			}
		}

		entries := deriveEntries(t, prog, target, tools)
		g, err := reach.Build(prog, entries)
		if err != nil {
			t.Fatalf("%s: %v", target, err)
		}
		targets := map[*types.Func]bool{}
		for _, name := range ws.MutatingFuncs(ws.Classification()) {
			if fn := prog.FuncByName(name); fn != nil {
				targets[fn] = true
			}
		}
		for _, e := range entries {
			v := ws.Verb{Surface: ws.Surface(e.Surface), Name: e.Name}
			merged := facts.Verbs[v]
			have := map[string]bool{}
			for _, h := range merged {
				have[h.Func] = true
			}
			hits, err := g.Reach(e, targets)
			if err != nil {
				t.Fatalf("%s: %v", target, err)
			}
			for _, h := range hits {
				if name := prog.FuncName(h.Func); !have[name] {
					merged = append(merged, ws.Hit{Func: name, Path: h.Path})
					have[name] = true
				}
			}
			sort.Slice(merged, func(i, j int) bool { return merged[i].Func < merged[j].Func })
			facts.Verbs[v] = merged
		}
		for _, pkg := range prog.Packages() {
			for _, name := range declaredFuncs(prog, pkg) {
				facts.Functions[name] = true
			}
		}
	}
	for e := range exports {
		facts.GitxExports = append(facts.GitxExports, e)
	}
	sort.Strings(facts.GitxExports)
	for _, w := range writers {
		facts.GitDirWriters = append(facts.GitDirWriters, w)
	}
	sort.Slice(facts.GitDirWriters, func(i, j int) bool { return facts.GitDirWriters[i].Func < facts.GitDirWriters[j].Func })
	return facts
}

// deriveEntries derives every verb from the three inventories, checking
// each derivation against the inventory it came from.
func deriveEntries(t *testing.T, prog *reach.Program, target reach.Target, tools []string) []reach.Entry {
	t.Helper()
	mod := prog.Module + "/"
	cli, err := reach.CLIEntries(prog, mod+cliPackage, cliDispatcher, string(ws.SurfaceCLI))
	if err != nil {
		t.Fatalf("%s: %v", target, err)
	}
	inventory, err := reach.StringKeyedMap(prog, mod+cliPackage, cliInventory)
	if err != nil {
		t.Fatalf("%s: reading the CLI-verb inventory: %v", target, err)
	}
	dispatched := map[string]bool{}
	for _, e := range cli {
		dispatched[e.Name] = true
	}
	for verb, phase := range inventory {
		if phase != "0" && !dispatched[verb] {
			t.Fatalf("%s: the CLI-verb inventory names %q (phase %s), but no dispatcher arm for it was derived; the witness cannot see its code", target, verb, phase)
		}
	}
	mcp, err := reach.SwitchEntries(prog, mod+mcpPackage, string(ws.SurfaceMCP), tools)
	if err != nil {
		t.Fatalf("%s: matching the MCP-tool inventory to its dispatch: %v", target, err)
	}
	wb, err := reach.RouteEntries(prog, mod+workbenchPackage, string(ws.SurfaceWorkbench))
	if err != nil {
		t.Fatalf("%s: %v", target, err)
	}
	return append(append(cli, mcp...), wb...)
}

// liveMCPTools reads the MCP-tool inventory from the live server's
// tools/list, over the wire framing a client uses.
func liveMCPTools(t *testing.T, root string) []string {
	t.Helper()
	srv := mcpserve.NewServer(root)
	var out bytes.Buffer
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	if err := mcpserve.ServeConn(context.Background(), strings.NewReader(req), &out, srv); err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decoding tools/list: %v", err)
	}
	var names []string
	for _, tool := range resp.Result.Tools {
		names = append(names, tool.Name)
	}
	if len(names) == 0 {
		t.Fatal("tools/list returned no tools")
	}
	return names
}

// declaredFuncs returns the FuncName of every function pkg declares.
func declaredFuncs(prog *reach.Program, pkg *reach.Package) []string {
	var out []string
	for _, obj := range pkg.Info.Defs {
		if fn, ok := obj.(*types.Func); ok {
			if sig, ok := fn.Type().(*types.Signature); ok && sig.Recv() != nil && types.IsInterface(sig.Recv().Type()) {
				continue
			}
			out = append(out, prog.FuncName(fn))
		}
	}
	return out
}
