package refindex

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/store"
)

// specsActiveZone and specsArchiveZone are the two default-branch zones
// dc-4 walks — the same two directories internal/index/walk.go's
// working-tree corpus walk covers, ref-scoped here instead.
const (
	specsActiveZone  = "active"
	specsArchiveZone = "archive"
)

// ComputeIndex returns the directory index spec/workbench-directory ac-2
// needs: one Entry per default-branch spec (dc-4) and one Entry per
// UNMERGED design branch's draft (ac-1, dc-5) — local and remote-tracking
// design refs alike (ac-2) — as a pure function of root's ref state. It
// never execs a checkout-mutating git command: every read goes through deps
// (dc-2's git-runner port), whose method set exposes none (ac-5).
//
// The returned slice is sorted by Ref, so two calls against identical ref
// state return byte-identical output (ac-1, ac-2, ac-3's determinism
// requirement) — never an incidental artifact of map iteration order.
func ComputeIndex(ctx context.Context, root string, deps GitRunner) ([]Entry, error) {
	var entries []Entry

	defaultEntries, err := computeDefaultBranchEntries(ctx, root, deps)
	if err != nil {
		return nil, fmt.Errorf("refindex: default-branch walk: %w", err)
	}
	entries = append(entries, defaultEntries...)

	designEntries, err := computeDesignBranchEntries(ctx, root, deps)
	if err != nil {
		return nil, fmt.Errorf("refindex: design-branch walk: %w", err)
	}
	entries = append(entries, designEntries...)

	sort.Slice(entries, func(i, j int) bool { return entries[i].Ref < entries[j].Ref })
	return entries, nil
}

// computeDefaultBranchEntries walks the default branch's own tree (dc-4) —
// never the working tree (co-1) — under .verdi/specs/active/ and
// .verdi/specs/archive/, reading each spec.md's frontmatter status through
// the same internal/artifact strict-decode seam every other spec read in
// this store uses.
func computeDefaultBranchEntries(ctx context.Context, root string, deps GitRunner) ([]Entry, error) {
	defaultBranch, err := deps.DefaultBranch(ctx, root)
	if err != nil {
		return nil, err
	}
	if defaultBranch == "" {
		// Unconfigured (e.g. no "origin" remote at all): gitx.DefaultBranch's
		// own contract treats this as "can't prove it", not an operational
		// failure (I-14's local-otherwise-warns posture) — there is no
		// default-branch ref to walk, so this walk honestly contributes no
		// entries rather than fabricating one.
		return nil, nil
	}

	var entries []Entry
	for _, zone := range []string{specsActiveZone, specsArchiveZone} {
		prefix := ".verdi/specs/" + zone
		paths, err := deps.ListTree(ctx, root, defaultBranch, prefix)
		if err != nil {
			return nil, err
		}
		for _, p := range paths {
			name, ok := specNameFromPath(p, prefix)
			if !ok {
				continue // not a spec.md file (e.g. board.json, deviation-report.md)
			}
			content, err := deps.Show(ctx, root, defaultBranch, p)
			if err != nil {
				return nil, err
			}
			fm, _, err := artifact.SplitFrontmatter(content)
			if err != nil {
				return nil, fmt.Errorf("%s at %s: %w", p, defaultBranch, err)
			}
			spec, err := artifact.DecodeSpec(fm)
			if err != nil {
				return nil, fmt.Errorf("%s at %s: %w", p, defaultBranch, err)
			}
			group, err := mapStatusGroup(spec.Status)
			if err != nil {
				return nil, fmt.Errorf("%s at %s: %w", p, defaultBranch, err)
			}
			entries = append(entries, Entry{
				Ref:         "spec/" + name,
				Source:      SourceDefault,
				StatusGroup: group,
				SpecStatus:  string(spec.Status),
				// Zone is WHERE this iteration of the two-zone loop above
				// found the entry (spec/home-status-glance dc-2) — zone and
				// specsActiveZone/specsArchiveZone share the exact "active"/
				// "archive" string values by construction, so this is a
				// direct cast, never a second vocabulary.
				Zone: Zone(zone),
			})
		}
	}
	return entries, nil
}

// specNameFromPath extracts <name> from "<prefix>/<name>/spec.md", the only
// shape counted as a spec entry — any other file under a spec's own
// directory (board.json, rollup.json, deviation-report.md) or nested more
// deeply is not a spec.md and is skipped, mirroring
// internal/index/walk.go's own non-artifact-file skip posture.
func specNameFromPath(path, prefix string) (name string, ok bool) {
	rest := strings.TrimPrefix(path, prefix+"/")
	if rest == path {
		return "", false // path was not actually under prefix
	}
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[1] != "spec.md" {
		return "", false
	}
	return parts[0], true
}

// computeDesignBranchEntries enumerates every UNMERGED design branch's
// draft (ac-1, dc-5), local refs/heads/design/* and remote-tracking
// refs/remotes/origin/design/* alike (ac-2), through the one shared
// merge-by-name path below — never two independently-maintained loops.
func computeDesignBranchEntries(ctx context.Context, root string, deps GitRunner) ([]Entry, error) {
	local, err := deps.LocalDesignBranches(ctx, root)
	if err != nil {
		return nil, err
	}
	remote, err := deps.RemoteDesignBranches(ctx, root)
	if err != nil {
		return nil, err
	}

	sources := mergeDesignSources(local, remote)

	defaultBranch, err := deps.DefaultBranch(ctx, root)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)

	var entries []Entry
	for _, name := range names {
		src := sources[name]
		// The revision used to read this branch's content and test its
		// merge state: the local branch short name when a local ref exists
		// (SourceLocal or SourceBoth — the local branch is the authoring
		// branch dc-5's parent feature treats as primary, "only a local
		// design branch opens as an authoring wall"), else the
		// remote-tracking short name ("origin/"+name) for a remote-only
		// entry. Either form is a valid git revision, resolved read-only.
		revision := name
		if src == SourceRemote {
			revision = "origin/" + name
		}

		slug := strings.TrimPrefix(name, designPrefix)
		ref := "spec/" + slug
		specPath := store.ActiveSpecRelPath(slug)

		// The existence probe (ListTree, not Show) comes BEFORE dc-5's
		// merged-branch check, deliberately: dc-5's exclusion exists solely
		// to stop a REAL spec from being double-counted (once from this
		// walk, once from the default-branch walk) — its own stated
		// rationale is "re-enumerating the SAME SPEC a second time...
		// fabricate[s] a duplicate entry". A branch with no spec.md at all
		// has no content to duplicate, so the exclusion's rationale never
		// applies to it — and it MUST not apply, because gitx.IsAncestor
		// treats a commit as its own ancestor (by design, matching git's
		// semantics), so a design branch freshly cut off the default branch
		// (dc-1's own "branch-cut-before-scaffold-commit window", ac-4) has
		// a tip IDENTICAL to the default branch's tip before its first
		// commit lands — trivially "merged" by that inclusive test, despite
		// never having been merged in any meaningful sense. Checking for
		// content first, and reserving the merged-exclusion for the case
		// where real content was actually found, is what lets a fresh,
		// still-empty branch still yield ac-4's required disclosed entry
		// instead of being silently dropped by dc-5's check.
		found, err := deps.ListTree(ctx, root, revision, specPath)
		if err != nil {
			return nil, err
		}
		if len(found) == 0 {
			d := disclosure.New(
				"refindex:no-draft-spec",
				ref,
				fmt.Sprintf("design branch %q resolves but has no spec.md at %s yet", name, specPath),
			)
			entries = append(entries, Entry{
				Ref:         ref,
				Source:      src,
				StatusGroup: StatusGroupDraftsInProgress,
				Disclosed:   &d,
				// Unconditional, exactly like StatusGroup above: a design
				// branch's spec (had it existed) is only ever read from the
				// active zone (specPath, above) — never derived from
				// content that was never there to read.
				Zone: ZoneActive,
			})
			continue
		}

		if defaultBranch != "" {
			merged, err := deps.IsAncestor(ctx, root, revision, defaultBranch)
			if err != nil {
				return nil, err
			}
			if merged {
				// dc-5: already counted once, from the default-branch walk —
				// re-including it here would double-count the same spec.
				continue
			}
		}

		entry, err := computeOrdinaryDesignEntry(ctx, root, deps, ref, src, revision, specPath)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// mergeDesignSources is the one shared code path (ac-2's static obligation)
// that folds local and remote-tracking design branch names into a single
// name->Source map: a name present on both sides becomes SourceBoth (one
// entry, never two).
func mergeDesignSources(local, remote []string) map[string]Source {
	sources := make(map[string]Source, len(local)+len(remote))
	for _, name := range local {
		sources[name] = SourceLocal
	}
	for _, name := range remote {
		if sources[name] == SourceLocal {
			sources[name] = SourceBoth
		} else {
			sources[name] = SourceRemote
		}
	}
	return sources
}

// computeOrdinaryDesignEntry builds one ordinary (non-degraded) design-branch
// Entry for a branch whose caller has already confirmed specPath exists at
// revision (computeDesignBranchEntries's ListTree probe) — reading its
// frontmatter status through the same internal/artifact strict-decode seam
// every other spec read in this store uses.
func computeOrdinaryDesignEntry(ctx context.Context, root string, deps GitRunner, ref string, src Source, revision, specPath string) (Entry, error) {
	content, err := deps.Show(ctx, root, revision, specPath)
	if err != nil {
		return Entry{}, err
	}
	fm, _, err := artifact.SplitFrontmatter(content)
	if err != nil {
		return Entry{}, fmt.Errorf("%s at %s: %w", specPath, revision, err)
	}
	spec, err := artifact.DecodeSpec(fm)
	if err != nil {
		return Entry{}, fmt.Errorf("%s at %s: %w", specPath, revision, err)
	}

	return Entry{
		Ref:    ref,
		Source: src,
		// Unconditional per ac-3: a design-branch entry's StatusGroup is
		// never derived from its own content, readable or not.
		StatusGroup: StatusGroupDraftsInProgress,
		SpecStatus:  string(spec.Status),
		// Unconditional per the Zone type's own doc comment: specPath
		// (the caller's existence probe, above) is always under the
		// active zone for a design-branch entry.
		Zone: ZoneActive,
	}, nil
}
