package writescope

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Surface is where a verb is reached from (parent dc-6).
type Surface string

// The three surfaces a verb can be reached from.
const (
	SurfaceCLI       Surface = "cli"
	SurfaceWorkbench Surface = "workbench"
	SurfaceMCP       Surface = "mcp"
)

// Verb is one entry point that can mutate a repository: a CLI verb named
// by its words down to the subcommand that dispatches it ("design start",
// "design start --from-stub"), a workbench route or board action named by
// its route pattern ("/board/spec/{name}/api/git-commit"), or an MCP tool
// ("import_apply").
type Verb struct {
	Surface Surface
	Name    string
}

// CLI returns the CLI verb name.
func CLI(name string) Verb { return Verb{Surface: SurfaceCLI, Name: name} }

// Workbench returns the workbench route or action pattern.
func Workbench(pattern string) Verb { return Verb{Surface: SurfaceWorkbench, Name: pattern} }

// MCP returns the MCP tool name.
func MCP(tool string) Verb { return Verb{Surface: SurfaceMCP, Name: tool} }

// String renders the verb as surface:name.
func (v Verb) String() string { return string(v.Surface) + ":" + v.Name }

// IndexCarry is how a ritual treats index entries it did not stage
// (parent dc-3): refused before any mutation, scoped out of its commits,
// carried into its commits, or moot because it never commits. Where more
// than one state could describe a ritual, the declared value is the first
// that holds in the ritual's execution order.
type IndexCarry string

// The four index-carry states, in parent dc-3's precedence order.
const (
	CarryRefused  IndexCarry = "refused"
	CarryNoCommit IndexCarry = "no_commit"
	CarryScoped   IndexCarry = "scoped"
	CarryCarried  IndexCarry = "carried"
)

// RefPattern names local branches a ritual may create, move, or delete:
// an exact ref ("refs/heads/policy/adopt"), every ref under a namespace
// ("refs/heads/design/*", the final segment "*" matching one or more
// segments), any local branch ("refs/heads/*"), or the role RefCheckedOut.
// A remote-tracking ref a push creates or moves is not one: it belongs to
// MayPush (Declaration).
type RefPattern string

// RefCheckedOut is the branch checked out in the checkout the ritual acts
// on: a commit on it moves it (parent dc-7), and so does a fast-forward of
// that checkout (the sealed execution's hand-back, whose runway is the
// checkout its store root is derived from). It may appear only among the
// refs a ritual moves.
const RefCheckedOut RefPattern = "@checked-out"

// WorktreePattern names where a ritual may add or remove linked
// worktrees: a store-relative directory whose immediate children are the
// worktrees (".verdi/data/worktrees/*"), or a role. A worktree's
// administrative entry under the git directory, and every effect inside a
// worktree the ritual itself added, belong to that worktree (parent dc-7).
type WorktreePattern string

// The worktree roles.
const (
	// WorktreeTemp is a fresh temporary directory outside the repository.
	WorktreeTemp WorktreePattern = "@temp"
	// WorktreeRegistered is any linked worktree already registered with
	// the repository, wherever it lies (reclamation removes them).
	WorktreeRegistered WorktreePattern = "@registered"
)

// PathPattern names store-relative paths a ritual may stage: a file
// (".verdi/policy/constitution.md"), a directory and everything under it
// (trailing "/"), a segment wildcard ("*" standing for exactly one whole
// segment), or PathWholeTree. Untracked files inside a declared path
// belong to that path (parent dc-7).
type PathPattern string

// PathWholeTree is the whole working tree (git add -A).
const PathWholeTree PathPattern = "**"

// Declaration is one ritual's write scope: the verbs it covers and the
// nine fields of parent ac-1. Every field is typed; Validate rejects an
// unknown value and an internally inconsistent declaration.
type Declaration struct {
	// Ritual is the declaration's stable snake_case identifier.
	Ritual string
	// Verbs are the entry points the ritual is reached from.
	Verbs []Verb

	RefsCreate        []RefPattern
	RefsMove          []RefPattern
	RefsDelete        []RefPattern
	HeadSwitch        bool
	Worktrees         []WorktreePattern
	StagePaths        []PathPattern
	IndexCarry        IndexCarry
	UntrackedMayEnter bool
	// MayPush is whether the ritual may push. A push with --set-upstream
	// (gitx.Push) also creates or moves the remote-tracking ref
	// refs/remotes/origin/<branch> and writes the branch's upstream
	// configuration (branch.<branch>.*); both belong to MayPush (parent
	// dc-7, ledger SI-314 (4)), never to the refs fields, which name local
	// branches only.
	MayPush bool
}

var (
	ritualRE  = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	cliRE     = regexp.MustCompile(`^[a-z][a-z0-9-]*( (--)?[a-z][a-z0-9-]*)*$`)
	mcpRE     = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
	workRE    = regexp.MustCompile(`^(/[A-Za-z0-9._{}-]+)+$`)
	refSegRE  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	pathSegRE = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]*$|^\.[A-Za-z0-9_][A-Za-z0-9._-]*$`)
)

// Validate reports the first problem with d. Besides each value's own
// grammar, it enforces these consistency rules, read from parent dc-3 and
// dc-7:
//
//   - no_commit stages no path and lets no untracked file in: there is no
//     commit for either to enter.
//   - scoped lets no untracked file outside its declared paths in, and does
//     not declare the whole tree: a scoped commit records only declared
//     paths, and a declared whole tree would make it a carried commit.
//   - only carried declares the whole tree.
//   - scoped and carried commit, so they declare at least one stage path.
//   - a declaration that stages a path commits it, whatever its carry, so
//     it declares at least one branch the commit lands on (created or
//     moved): refused stages only after its refusal passes, and still
//     commits.
//   - untracked files may enter only a declaration that stages a path:
//     without one there is no commit for them to enter.
//   - RefCheckedOut is only ever moved: it exists, and the checkout
//     holding it cannot delete it.
//   - a declaration declares at least one effect: it exists because its
//     verbs reach a mutating function (parent dc-6).
func (d Declaration) Validate() error {
	if !ritualRE.MatchString(d.Ritual) {
		return fmt.Errorf("writescope: ritual %q is not a snake_case identifier", d.Ritual)
	}
	if err := d.validateVerbs(); err != nil {
		return err
	}
	switch d.IndexCarry {
	case CarryRefused, CarryNoCommit, CarryScoped, CarryCarried:
	default:
		return fmt.Errorf("writescope: %s: unknown index carry %q (want refused, no_commit, scoped, or carried)", d.Ritual, d.IndexCarry)
	}
	for _, f := range []struct {
		field string
		refs  []RefPattern
	}{{"refs_create", d.RefsCreate}, {"refs_move", d.RefsMove}, {"refs_delete", d.RefsDelete}} {
		if err := d.validateRefs(f.field, f.refs); err != nil {
			return err
		}
	}
	if err := d.validateWorktrees(); err != nil {
		return err
	}
	if err := d.validateStagePaths(); err != nil {
		return err
	}
	return d.validateConsistency()
}

func (d Declaration) validateVerbs() error {
	if len(d.Verbs) == 0 {
		return fmt.Errorf("writescope: %s names no verb", d.Ritual)
	}
	seen := map[Verb]bool{}
	for _, v := range d.Verbs {
		var ok bool
		switch v.Surface {
		case SurfaceCLI:
			ok = cliRE.MatchString(v.Name)
		case SurfaceWorkbench:
			ok = workRE.MatchString(v.Name)
		case SurfaceMCP:
			ok = mcpRE.MatchString(v.Name)
		default:
			return fmt.Errorf("writescope: %s: unknown surface %q (want cli, workbench, or mcp)", d.Ritual, v.Surface)
		}
		if !ok {
			return fmt.Errorf("writescope: %s: malformed %s verb %q", d.Ritual, v.Surface, v.Name)
		}
		if seen[v] {
			return fmt.Errorf("writescope: %s names verb %s twice", d.Ritual, v)
		}
		seen[v] = true
	}
	return nil
}

func (d Declaration) validateRefs(field string, refs []RefPattern) error {
	seen := map[RefPattern]bool{}
	for _, r := range refs {
		if seen[r] {
			return fmt.Errorf("writescope: %s: %s names ref %q twice", d.Ritual, field, r)
		}
		seen[r] = true
		if r == RefCheckedOut {
			if field != "refs_move" {
				return fmt.Errorf("writescope: %s: %s cannot name %s: the checked-out branch is only ever moved", d.Ritual, field, r)
			}
			continue
		}
		if err := validRef(string(r)); err != nil {
			return fmt.Errorf("writescope: %s: %s: %w", d.Ritual, field, err)
		}
	}
	return nil
}

func validRef(r string) error {
	rest, ok := strings.CutPrefix(r, "refs/heads/")
	if !ok || rest == "" {
		return fmt.Errorf("ref %q is neither %s nor a refs/heads/ pattern", r, RefCheckedOut)
	}
	segs := strings.Split(rest, "/")
	for i, s := range segs {
		if s == "*" && i == len(segs)-1 {
			continue
		}
		if !refSegRE.MatchString(s) || strings.Contains(s, "..") || strings.HasSuffix(s, ".lock") {
			return fmt.Errorf("ref %q has a malformed segment %q (a wildcard may only be the whole final segment)", r, s)
		}
	}
	return nil
}

func (d Declaration) validateWorktrees() error {
	seen := map[WorktreePattern]bool{}
	for _, w := range d.Worktrees {
		if seen[w] {
			return fmt.Errorf("writescope: %s: worktrees names %q twice", d.Ritual, w)
		}
		seen[w] = true
		switch w {
		case WorktreeTemp, WorktreeRegistered:
			continue
		}
		dir, ok := strings.CutSuffix(string(w), "/*")
		if !ok || strings.HasPrefix(string(w), "@") {
			return fmt.Errorf("writescope: %s: worktree pattern %q is neither a role (%s, %s) nor a store-relative directory ending in /*", d.Ritual, w, WorktreeTemp, WorktreeRegistered)
		}
		if err := validRelPath(dir, false); err != nil {
			return fmt.Errorf("writescope: %s: worktree pattern %q: %w", d.Ritual, w, err)
		}
	}
	return nil
}

func (d Declaration) validateStagePaths() error {
	seen := map[PathPattern]bool{}
	for _, p := range d.StagePaths {
		if seen[p] {
			return fmt.Errorf("writescope: %s: stage_paths names %q twice", d.Ritual, p)
		}
		seen[p] = true
		if p == PathWholeTree {
			continue
		}
		if err := validRelPath(strings.TrimSuffix(string(p), "/"), true); err != nil {
			return fmt.Errorf("writescope: %s: stage path %q: %w", d.Ritual, p, err)
		}
	}
	return nil
}

// validRelPath checks a store-relative slash path; wildcard allows "*" as a
// whole segment.
func validRelPath(p string, wildcard bool) error {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, `\`) {
		return errors.New("path must be a non-empty, relative, slash-separated path")
	}
	for _, s := range strings.Split(p, "/") {
		if wildcard && s == "*" {
			continue
		}
		if s == "" || s == "." || s == ".." || !pathSegRE.MatchString(s) {
			return fmt.Errorf("path has a malformed segment %q (no empty, ., or .. segments; a wildcard is a whole segment)", s)
		}
	}
	return nil
}

func (d Declaration) validateConsistency() error {
	whole := false
	for _, p := range d.StagePaths {
		if p == PathWholeTree {
			whole = true
		}
	}
	switch d.IndexCarry {
	case CarryNoCommit:
		if len(d.StagePaths) > 0 || d.UntrackedMayEnter {
			return fmt.Errorf("writescope: %s: no_commit declares stage paths or untracked files that no commit would record", d.Ritual)
		}
	case CarryScoped:
		if d.UntrackedMayEnter {
			return fmt.Errorf("writescope: %s: a scoped commit records only its declared paths, so no untracked file outside them may enter", d.Ritual)
		}
	}
	if whole && d.IndexCarry != CarryCarried {
		return fmt.Errorf("writescope: %s: only a carried declaration may stage the whole tree", d.Ritual)
	}
	if (d.IndexCarry == CarryScoped || d.IndexCarry == CarryCarried) && len(d.StagePaths) == 0 {
		return fmt.Errorf("writescope: %s: a %s commit records declared paths, but no stage path is declared", d.Ritual, d.IndexCarry)
	}
	if len(d.StagePaths) > 0 && len(d.RefsCreate)+len(d.RefsMove) == 0 {
		return fmt.Errorf("writescope: %s: it stages paths, so it commits, and a commit lands on a branch, but no branch is created or moved", d.Ritual)
	}
	if d.UntrackedMayEnter && len(d.StagePaths) == 0 {
		return fmt.Errorf("writescope: %s: untracked files may enter only a commit, but no stage path is declared", d.Ritual)
	}
	if len(d.RefsCreate)+len(d.RefsMove)+len(d.RefsDelete)+len(d.Worktrees)+len(d.StagePaths) == 0 && !d.HeadSwitch && !d.MayPush {
		return fmt.Errorf("writescope: %s declares no effect", d.Ritual)
	}
	return nil
}
