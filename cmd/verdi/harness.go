// verdi harness render|check [--host claude|codex|all] [-o <repo root>]
// (spec/spec-documents ac-7): renders the four verdi skills for one or
// both hosts from the templates embedded in this binary, or checks the
// rendered copies for drift. Both read nothing from a store: -o names an
// existing directory (no ancestor search); without it the store root is
// found from the working directory (R-W3-7). Kept in its own file per the
// context_project.go convention so dispatch.go's diff is one arm.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/skillpack"
	"github.com/jyang234/verdi/internal/store"
)

// vocab:identity — CLI usage/flag grammar (identity)
const harnessUsage = "usage: verdi harness render|check [--host claude|codex|all] [-o <repo root>]"

func cmdHarness(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, harnessUsage)
		return 2
	}
	sub := args[0]
	if sub != "render" && sub != "check" {
		fmt.Fprintf(stderr, "harness: unknown subcommand %q\n%s\n", sub, harnessUsage)
		return 2
	}
	hostFlag, out, err := parseHarnessFlags(args[1:])
	if err != nil {
		fmt.Fprintf(stderr, "harness %s: %v\n%s\n", sub, err, harnessUsage)
		return 2
	}
	hosts, err := skillpack.ParseHosts(hostFlag)
	if err != nil {
		fmt.Fprintf(stderr, "harness %s: %v\n%s\n", sub, err, harnessUsage)
		return 2
	}
	root, err := harnessRoot(out)
	if err != nil {
		fmt.Fprintf(stderr, "harness %s: %v\n", sub, err)
		return 2
	}
	switch sub {
	case "render":
		res, err := skillpack.Write(context.Background(), root, hosts)
		if err != nil {
			fmt.Fprintf(stderr, "harness render: %v\n", err)
			return 2
		}
		// res.Files is sorted by Path (skillpack.Write's own contract), but
		// the printed line leads with the digest ("<digest>  <path>", proven
		// by the HasPrefix(l, "sha256:") check every caller relies on), so a
		// Path-major order does not imply the PRINTED LINES are themselves
		// in sorted (byte-comparable) order — the digest varies
		// independently of path. Sort the formatted lines directly so two
		// runs' stdout stays byte-for-byte diffable in the one order the
		// line's own leading field actually determines.
		lines := make([]string, 0, len(res.Files))
		for _, f := range res.Files {
			lines = append(lines, fmt.Sprintf("%s  %s", f.Digest, f.Path))
		}
		sort.Strings(lines)
		var b strings.Builder
		for _, l := range lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
		if _, err := io.WriteString(stdout, b.String()); err != nil {
			fmt.Fprintf(stderr, "harness render: writing output: %v\n", err)
			return 2
		}
		fmt.Fprintf(stderr, "harness render: wrote %d skill files under %s (render commit %s)\n", len(res.Files), root, res.RenderCommit)
		return 0
	default:
		rep, err := skillpack.Check(root, hosts)
		if err != nil {
			fmt.Fprintf(stderr, "harness check: %v\n", err)
			return 2
		}
		var b strings.Builder
		for _, f := range rep.Findings {
			fmt.Fprintf(&b, "%s  %s\n", f.Code, f.Path)
		}
		if _, err := io.WriteString(stdout, b.String()); err != nil {
			fmt.Fprintf(stderr, "harness check: writing output: %v\n", err)
			return 2
		}
		if !rep.Clean() {
			fmt.Fprintf(stderr, "harness check: %d of %d skill files drifted or are missing under %s; run `verdi harness render` to regenerate\n", len(rep.Findings), rep.Checked, root)
			return 1
		}
		fmt.Fprintf(stderr, "harness check: %d skill files match this binary under %s\n", rep.Checked, root)
		return 0
	}
}

// parseHarnessFlags accepts --host <v> | --host=<v> and -o <dir> | -o=<dir>,
// each at most once, no positionals.
func parseHarnessFlags(args []string) (host, out string, err error) {
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, value, inline := a, "", false
		if k, v, ok := strings.Cut(a, "="); ok && (k == "--host" || k == "-o") {
			name, value, inline = k, v, true
		}
		switch name {
		case "--host", "-o":
			if seen[name] {
				return "", "", fmt.Errorf("%s given twice", name)
			}
			seen[name] = true
			if !inline {
				if i+1 >= len(args) {
					return "", "", fmt.Errorf("%s requires a value", name)
				}
				i++
				value = args[i]
			}
			if strings.TrimSpace(value) == "" {
				return "", "", fmt.Errorf("%s requires a value", name)
			}
			if name == "--host" {
				host = value
			} else {
				out = value
			}
		default:
			return "", "", fmt.Errorf("unexpected argument %q", a)
		}
	}
	return host, out, nil
}

// harnessRoot resolves -o (an existing directory, no store required) or,
// absent, the store root found from the working directory.
func harnessRoot(out string) (string, error) {
	if out != "" {
		info, err := os.Stat(out)
		if err != nil {
			return "", fmt.Errorf("-o %q: %w", out, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("-o %q is not a directory", out)
		}
		return out, nil
	}
	return store.FindRoot(".")
}
