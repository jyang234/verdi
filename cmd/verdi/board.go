// verdi board commit <board-key> --name <spec-name> [--story-ref <scheme:key>]
// (05 §Workbench's commit-to-design ritual, PLAN.md ledger I-20): the CLI
// entry point for the mechanical half of commit-to-design. This is ONE of
// the two entry points the ritual's logic has (internal/commitdesign's doc
// comment) — the workbench's `POST /board/<key>/commit` HTTP action calls
// the exact same internal/commitdesign.Run function in-process; neither
// entry point shells out to the other. Kept in its own file per the
// lint.go/sync.go/matrix.go/dex.go convention.
package main

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/OWNER/verdi/internal/commitdesign"
	"github.com/OWNER/verdi/internal/store"
)

// runBoardVerb dispatches `verdi board <subcommand>`. v0 has exactly one
// subcommand, "commit".
func runBoardVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "commit" {
		fmt.Fprintln(stderr, "usage: verdi board commit <board-key> --name <spec-name> [--story-ref <scheme:key>]")
		return 2
	}
	return cmdBoardCommit(args[1:], stdout, stderr)
}

// cmdBoardCommit is `verdi board commit`'s real entry point.
func cmdBoardCommit(args []string, stdout, stderr io.Writer) int {
	name, storyRef, rest, err := extractBoardCommitFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, "board commit:", err)
		return 2
	}
	if name == "" {
		fmt.Fprintln(stderr, "board commit: --name is required (I-10: no magic, no tracker-derived naming)")
		return 2
	}
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "board commit: usage: verdi board commit <board-key> --name <spec-name> [--story-ref <scheme:key>]")
		return 2
	}
	boardKey := rest[0]

	root, err := store.FindRoot(".")
	if err != nil {
		fmt.Fprintln(stderr, "board commit:", err)
		return 2
	}

	res, err := commitdesign.Run(context.Background(), commitdesign.Input{
		Root: root, BoardKey: boardKey, SpecName: name, StoryRef: storyRef,
	})
	if err != nil {
		fmt.Fprintln(stderr, "board commit:", err)
		return 2
	}

	fmt.Fprintf(stdout, "board commit: wrote %s\n", res.SpecRelPath)
	fmt.Fprintf(stdout, "board commit: wrote %s\n", res.BoardRelPath)
	fmt.Fprintf(stdout, "board commit: dispositioned %d sticky(s) as open-question\n", len(res.Dispositions))
	fmt.Fprintf(stdout, "board commit: committed %s\n", res.Commit)
	return 0
}

// extractBoardCommitFlags pulls --name/-name and --story-ref/-story-ref
// out of args in whatever position they appear (mirroring design.go's
// extractNameFlag, since 05 §CLI's own `design start <story-ref> --name
// <name>` ordering is the pattern this verb follows too), returning every
// remaining positional argument in order.
func extractBoardCommitFlags(args []string) (name, storyRef string, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--name" || a == "-name":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("%s requires a value", a)
			}
			if name != "" {
				return "", "", nil, fmt.Errorf("--name given more than once")
			}
			name = args[i+1]
			i++
		case strings.HasPrefix(a, "--name=") || strings.HasPrefix(a, "-name="):
			if name != "" {
				return "", "", nil, fmt.Errorf("--name given more than once")
			}
			_, name, _ = strings.Cut(a, "=")
		case a == "--story-ref" || a == "-story-ref":
			if i+1 >= len(args) {
				return "", "", nil, fmt.Errorf("%s requires a value", a)
			}
			if storyRef != "" {
				return "", "", nil, fmt.Errorf("--story-ref given more than once")
			}
			storyRef = args[i+1]
			i++
		case strings.HasPrefix(a, "--story-ref=") || strings.HasPrefix(a, "-story-ref="):
			if storyRef != "" {
				return "", "", nil, fmt.Errorf("--story-ref given more than once")
			}
			_, storyRef, _ = strings.Cut(a, "=")
		default:
			rest = append(rest, a)
		}
	}
	return name, storyRef, rest, nil
}
