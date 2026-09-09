// verdi design context <spec-ref> [--child-story <ref>]... (AC-8's
// get_design_context, CLI equivalent): returns only material relevant to
// design assistance (AC-5) — the current draft, an implements-linked
// parent feature, any explicitly named child stories, applicable
// ratified decisions, the spec's declared pinned context references,
// Verdi-go-derived findings, and the context/policy digests — over
// designapp.Service.GetDesignContext. Provenance is deliberately absent;
// use `verdi design provenance` on explicit request.
package main

import (
	"context"
	"fmt"
	"io"

	"github.com/jyang234/verdi/internal/designapp"
)

// extractDesignContextFlags pulls every "--child-story"/"-child-story"
// occurrence (repeatable, mirroring get_context_bundle's own "refs" list
// shape) out of args, returning the one required positional spec ref and
// every child-story value in the order given.
func extractDesignContextFlags(args []string) (spec string, childStories []string, rest []string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--child-story", "-child-story":
			if i+1 >= len(args) {
				// vocab:identity — CLI usage/flag grammar (--child-story flag name, identity)
				return "", nil, nil, fmt.Errorf("--child-story requires a value")
			}
			childStories = append(childStories, args[i+1])
			i++
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) != 1 {
		// vocab:identity — CLI usage/flag grammar (--child-story flag name, identity)
		return "", nil, nil, fmt.Errorf("usage: verdi design context <spec-ref> [--child-story <ref>] (repeatable)")
	}
	return rest[0], childStories, rest, nil
}

func cmdDesignContext(args []string, stdout, stderr io.Writer) int {
	spec, childStories, _, err := extractDesignContextFlags(args)
	if err != nil {
		fmt.Fprintln(stderr, "design context:", err)
		return 2
	}
	svc := designapp.NewService()
	result, appErr := svc.GetDesignContext(context.Background(), ".", designapp.GetDesignContextRequest{Spec: spec, ChildStories: childStories})
	if appErr != nil {
		return renderDesignAppResult(stdout, stderr, "design context", nil, appErr)
	}
	return renderDesignAppResult(stdout, stderr, "design context", result, nil)
}
