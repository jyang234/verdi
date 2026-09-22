package recovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/branchbase"
	"github.com/jyang234/verdi/internal/execworkspace"
	"github.com/jyang234/verdi/internal/filelock"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/reclaim"
	"github.com/jyang234/verdi/internal/residue"
	"github.com/jyang234/verdi/internal/store"
	"github.com/jyang234/verdi/internal/wtmanager"
)

// BranchTip is one local branch's short name and resolved tip commit.
type BranchTip struct {
	Name string
	Tip  string
}

// RitualBranch is one of a ref's four ritual branches' raw git facts
// (R-RR3-4: design/<name>, feature/<name>, close/<name>, policy/adopt).
type RitualBranch struct {
	Name   string
	Exists bool
	Tip    string
	// EmptyWitnesses lists every OTHER local branch whose tip equals or
	// descends from Tip (R-RR3-5's own ancestry predicate, computed via
	// gitx.MergeBase — the spike's documented MergeBase-equivalent to
	// IsAncestor, since IsAncestor is outside this package's own
	// command-surface allow-list). Non-empty iff the branch carries no
	// commits of its own.
	EmptyWitnesses    []string
	HasRemoteTracking bool
	RemoteChecked     bool // false when the remote-tracking read itself failed (disclosed instead)
	Ahead, Behind     int
}

// Empty reports whether b carries no commits of its own (R-RR3-5).
func (b RitualBranch) Empty() bool { return b.Exists && len(b.EmptyWitnesses) > 0 }

// LockFact pairs a lock path with its read-only Inspect result.
type LockFact struct {
	Path       string
	Inspection filelock.Inspection
	// ReadError is filelock.Inspect's own error text (a malformed,
	// complete-but-garbled lock body — R-RR3-16), "" when Inspect
	// succeeded. Inspection is the zero value whenever this is non-empty.
	ReadError string
}

// JournalFact is the draft-mutation journal's own read-only peek
// (R-RR3-11): only {schema, spec, phase} are decoded, permissively — the
// journal carries more fields than this projection needs.
type JournalFact struct {
	Path string
	// Present is true whenever a non-symlink journal file exists at Path,
	// regardless of whether its body decoded (R-RR3-16: "present but
	// undecodable" is itself a recognized fact, not silence).
	Present bool
	// Decoded is true only when the body strict-permissively decoded as
	// {schema, spec, phase}; Schema/Spec/Phase are meaningful only then.
	Decoded bool
	// DecodeError is the decode failure text, set only when Present &&
	// !Decoded.
	DecodeError string
	Schema      string
	Spec        string
	Phase       string
	Steps       []string // DraftMutationDir's own directory listing, sorted
}

// journalPeek is the permissive (non-strict) decode target for
// JournalFact: unknown fields are ignored on purpose (facts.go, Step 12).
type journalPeek struct {
	Schema string `json:"schema"`
	Spec   string `json:"spec"`
	Phase  string `json:"phase"`
}

// WorkspaceUnit is one execution-workspace id's classified sibling
// presence under execworkspace.ExecutionRoot (R-RR3-11).
type WorkspaceUnit struct {
	ID                string
	HasUnit           bool
	HasRequest        bool
	HasRequestStaging bool
	HasReleased       bool
	HasLock           bool
	LockPath          string
}

// Facts is everything Gather observes for one ref, read-only. Derive is a
// pure function of Facts alone (no I/O).
type Facts struct {
	Root string
	Name string // the ref's own spec name
	Ref  artifact.Ref

	CurrentBranch string
	Head          string

	LocalBranches []BranchTip

	DefaultBranch         branchbase.Resolution
	DefaultBranchResolved bool

	Design      RitualBranch
	Feature     RitualBranch
	Close       RitualBranch
	PolicyAdopt RitualBranch

	// StagedPaths and WorktreeChangedPaths are the two explicit listings
	// every clean-tree question in this package is answered from, and
	// their *Observed flags are the answer's own validity (DC-13): a
	// listing whose read FAILED leaves the slice nil and its flag false,
	// which is not the same fact as "the listing was empty". Nothing
	// consuming these as proof may read one for the other. The failure's
	// own text is not duplicated here — Disclosures already carries it.
	StagedPaths             []string
	StagedPathsObserved     bool
	WorktreeChangedPaths    []string
	WorktreeChangedObserved bool

	// RepoPrefix is the STORE root's own path inside the repository
	// (gitx.RepoPrefix: "" when they coincide, "product/" when the store
	// sits below the git root), resolved ONCE here because every
	// recognizer that compares a git listing against a store-relative
	// zone prefix needs the same answer. RepoPrefixObserved is false when
	// the read itself failed — the two vocabularies are then unrelatable,
	// which every consumer treats as "withhold", never as "they
	// coincide".
	RepoPrefix         string
	RepoPrefixObserved bool

	ActiveSpecOnDisk  bool
	ArchiveSpecOnDisk bool
	ActiveSpecAtHead  bool
	ArchiveSpecAtHead bool

	WriterLock  LockFact
	RitualLocks []LockFact // one per existing ritual branch, sorted by path

	Journal JournalFact

	WorkspaceUnits []WorkspaceUnit // sorted by id
	WorkspaceLocks []LockFact      // sorted by path
	// WorkspaceUnclassified names every execution-workspace directory
	// entry ClassifyEntry did not recognize (grammar-external, R-RR3-16),
	// sorted.
	WorkspaceUnclassified []string

	ResidueScanned bool
	Residue        *residue.Result
	ReclaimRows    []reclaim.Row // this ref's own units only (design/close/feature branches), in Plan order

	Disclosures []string // co-6: facts that could not be gathered, sorted
}

// Gatherer holds no state; its methods are seams tests can call against a
// real store.Config.
type Gatherer struct{}

// NewGatherer returns a ready-to-use Gatherer.
func NewGatherer() Gatherer { return Gatherer{} }

// Gather reads ref's whole fact set from root, read-only throughout: it
// never checks out a branch, writes a file, or mutates git state (the AST
// gate in commandsurface_test.go proves this package calls no mutating
// gitx primitive). A failure gathering any single fact becomes a
// disclosure and the rest proceeds (co-6); only ref resolution itself
// (an unparseable ref, a non-spec ref, or a non-feature spec class) is an
// operational error, since without it there is no ref-scoped inventory to
// gather at all.
func (g Gatherer) Gather(ctx context.Context, cfg *store.Config, refStr string) (Facts, error) {
	ref, err := artifact.ParseRef(refStr)
	if err != nil {
		return Facts{}, fmt.Errorf("recovery: gather: %w", err)
	}
	if ref.Kind != artifact.KindSpec {
		return Facts{}, fmt.Errorf("recovery: gather: %q names a %s, not a spec; recovery only inspects spec refs", refStr, ref.Kind)
	}

	root := cfg.Root

	// The default-branch base is resolved up front (R-RR3-15): specClassAt's
	// own fallback chain needs it, and Facts needs it regardless, so it is
	// computed exactly once.
	f := Facts{Root: root, Name: ref.Name, Ref: ref}
	var disclosures []string

	res, defaultBranchErr := branchbase.Resolve(ctx, root)
	if defaultBranchErr != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not resolve the default-branch base: %v", defaultBranchErr))
	} else {
		f.DefaultBranch = res
		f.DefaultBranchResolved = res.Kind == branchbase.ResolvedDefault
		switch res.Kind {
		case branchbase.HeadFallback:
			disclosures = append(disclosures, "the default branch is unresolved (no origin remote); facts about it are based on the current HEAD, disclosed, not a default-branch base")
		case branchbase.Unresolvable:
			disclosures = append(disclosures, "the default branch could not be resolved (origin is configured but no default branch resolves)")
		}
	}

	defaultBranchRefForClass := ""
	if f.DefaultBranchResolved {
		defaultBranchRefForClass = f.DefaultBranch.Ref
	}
	class, err := specClassAt(ctx, root, ref.Name, defaultBranchRefForClass)
	if err != nil {
		return Facts{}, fmt.Errorf("recovery: gather: %w", err)
	}
	if class != artifact.ClassFeature {
		classWord := cfg.Model.DisplayClass(string(class))
		return Facts{}, fmt.Errorf("recovery: gather: %s is %s; recovery only inspects a feature spec's own ritual branches", refStr, model.Indefinite(classWord))
	}

	branch, err := gitx.CurrentBranch(ctx, root)
	if err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not determine the current branch: %v", err))
	} else {
		f.CurrentBranch = branch
	}
	head, err := gitx.RevParse(ctx, root, "HEAD")
	if err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not resolve HEAD: %v", err))
	} else {
		f.Head = head
	}

	tips, tipDisclosures := gatherLocalBranchTips(ctx, root)
	f.LocalBranches = tips
	disclosures = append(disclosures, tipDisclosures...)

	var branchDisclosures []string
	f.Design, branchDisclosures = gatherRitualBranch(ctx, root, "design/"+ref.Name, tips)
	disclosures = append(disclosures, branchDisclosures...)
	f.Feature, branchDisclosures = gatherRitualBranch(ctx, root, "feature/"+ref.Name, tips)
	disclosures = append(disclosures, branchDisclosures...)
	f.Close, branchDisclosures = gatherRitualBranch(ctx, root, "close/"+ref.Name, tips)
	disclosures = append(disclosures, branchDisclosures...)
	f.PolicyAdopt, branchDisclosures = gatherRitualBranch(ctx, root, "policy/adopt", tips)
	disclosures = append(disclosures, branchDisclosures...)

	// The two listings below are the whole of this projection's
	// working-tree evidence. gitx.StatusDirty's single bool is
	// deliberately NOT gathered beside them: it runs a plain `git status
	// --porcelain`, which honors status.showUntrackedFiles, so an
	// ordinary display setting makes it answer "clean" over untracked
	// work that WorktreeChangedPaths (--untracked-files=all, which
	// overrides the setting) names outright. One configuration-
	// independent answer, read from the explicit listings, is the only
	// one anything here proves a precondition from (owner risk review
	// F1).
	if staged, err := gitx.StagedPaths(ctx, root); err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not list staged paths: %v", err))
	} else {
		f.StagedPaths = staged
		f.StagedPathsObserved = true
	}
	if changed, err := gitx.WorktreeChangedPaths(ctx, root); err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not list working-tree changed paths: %v", err))
	} else {
		f.WorktreeChangedPaths = changed
		f.WorktreeChangedObserved = true
	}
	if prefix, err := gitx.RepoPrefix(ctx, root); err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not resolve the store root's own path inside the repository: %v", err))
	} else {
		f.RepoPrefix = prefix
		f.RepoPrefixObserved = true
	}

	f.ActiveSpecOnDisk = pathExists(store.ActiveSpecPath(root, ref.Name))
	f.ArchiveSpecOnDisk = pathExists(store.ArchiveSpecPath(root, ref.Name))
	if entries, err := gitx.LsTree(ctx, root, "HEAD", store.SpecDirRelPath(store.ZoneActive, ref.Name)); err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not read HEAD's active-zone tree for spec/%s: %v", ref.Name, err))
	} else {
		f.ActiveSpecAtHead = len(entries) > 0
	}
	if entries, err := gitx.LsTree(ctx, root, "HEAD", store.SpecDirRelPath(store.ZoneArchive, ref.Name)); err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not read HEAD's archive-zone tree for spec/%s: %v", ref.Name, err))
	} else {
		f.ArchiveSpecAtHead = len(entries) > 0
	}

	f.WriterLock = LockFact{Path: store.WriterLockPath(root)}
	if insp, err := filelock.Inspect(f.WriterLock.Path); err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not inspect the writer lock %s: %v", f.WriterLock.Path, err))
		f.WriterLock.ReadError = err.Error()
	} else {
		f.WriterLock.Inspection = insp
	}

	for _, rb := range []RitualBranch{f.Design, f.Feature, f.Close, f.PolicyAdopt} {
		if !rb.Exists {
			continue
		}
		path := wtmanager.WorktreePath(root, rb.Name) + ".lock"
		lf := LockFact{Path: path}
		if insp, err := filelock.Inspect(path); err != nil {
			disclosures = append(disclosures, fmt.Sprintf("could not inspect lock %s: %v", path, err))
			lf.ReadError = err.Error()
		} else {
			lf.Inspection = insp
		}
		f.RitualLocks = append(f.RitualLocks, lf)
	}
	sort.Slice(f.RitualLocks, func(i, j int) bool { return f.RitualLocks[i].Path < f.RitualLocks[j].Path })

	journal, journalDisclosures := gatherJournal(root, ref.Name)
	f.Journal = journal
	disclosures = append(disclosures, journalDisclosures...)

	units, workspaceLocks, workspaceUnclassified, workspaceDisclosures := gatherWorkspace(root)
	f.WorkspaceUnits = units
	f.WorkspaceLocks = workspaceLocks
	f.WorkspaceUnclassified = workspaceUnclassified
	disclosures = append(disclosures, workspaceDisclosures...)

	if f.DefaultBranchResolved {
		res, err := residue.Scan(ctx, root, f.DefaultBranch.Ref)
		if err != nil {
			disclosures = append(disclosures, fmt.Sprintf("could not scan for stranded residue: %v", err))
		} else {
			f.ResidueScanned = true
			f.Residue = res
			if len(res.UnprovenSpecs) > 0 {
				// R-RR3-14: mirrors gc's own refusal verbatim — no
				// reclaim plan is computed over an incomplete scan.
				disclosures = append(disclosures, gcUnprovenSpecsRefusal)
				for _, u := range res.UnprovenSpecs {
					disclosures = append(disclosures, fmt.Sprintf("spec/%s: %s", u.Name, strings.Join(u.Disclosures, "; ")))
				}
			} else {
				plan := reclaim.Compute(res, root, f.CurrentBranch, f.DefaultBranch.BranchName)
				f.ReclaimRows = filterReclaimRowsForRef(plan.DryRunRows(), ref.Name)
			}
		}
	} else {
		disclosures = append(disclosures, "stranded-residue scan skipped: the default branch did not resolve")
	}

	sort.Strings(disclosures)
	f.Disclosures = dedupSorted(disclosures)
	return f, nil
}

// gcUnprovenSpecsRefusal is `verdi gc --reclaim-unmanaged`'s own refusal
// sentence (cmd/verdi/gc.go), reused verbatim (R-RR3-14: "the projection
// discloses the unproven specs with gc's own sentence") — copied, not
// imported, since cmd/verdi is package main.
const gcUnprovenSpecsRefusal = "gc: --reclaim-unmanaged: one or more active-zone specs have an unproven effective lifecycle state; refusing to compute or apply a reclamation plan over an incomplete scan"

// specClassAt resolves name's spec class (R-RR3-15), tried in order: the
// on-disk active zone, the on-disk archive zone, `design/<name>`'s own
// tree (gitx.Show, active-zone path — the ritual's own scaffold commit),
// `close/<name>`'s own tree (same path — a close ritual that has not yet
// committed its archive move still shows the active-zone path there),
// and finally the resolved default-branch base's own tree (both zones,
// for a spec already closed and merged). gitx.Show simply errors when a
// branch does not exist or the path is absent from its tree — exactly
// the "not found here, try the next location" signal every step already
// tolerates, so no separate existence pre-check is needed. Only when
// every location fails is Gather an operational error naming each one
// tried (never a guess).
func specClassAt(ctx context.Context, root, name, defaultBranchRef string) (artifact.SpecClass, error) {
	activeRel := store.ActiveSpecRelPath(name)
	archiveRel := store.SpecRelPath(store.ZoneArchive, name)

	locations := []string{
		"the on-disk active zone",
		"the on-disk archive zone",
		"design/" + name,
		"close/" + name,
	}
	var lastDecodeErr error
	// tryDecode reads the class out of ONE candidate document's FRONT
	// MATTER (R-RR3-22). The split is not optional: a spec's Markdown body
	// is prose, not YAML — an 80-column paragraph whose continuation line
	// carries a ": ", or a fenced code block, is refused by the YAML
	// scanner — so handing the whole document to the strict decoder turns
	// ordinary authoring into a fabricated operational error about the
	// artifact (ac-8's exit 2 is reserved for a genuine operational
	// failure; co-6 forbids blaming the artifact for the reader's
	// defect). This is the shape every other caller of artifact.DecodeSpec
	// in the repository uses — internal/journey/facts.go's
	// decodeTargetSpec is the one copied here.
	tryDecode := func(data []byte, readErr error) (artifact.SpecClass, bool) {
		if readErr != nil {
			return "", false
		}
		frontmatter, _, serr := artifact.SplitFrontmatter(data)
		if serr != nil {
			lastDecodeErr = serr
			return "", false
		}
		fm, ferr := artifact.DecodeSpec(frontmatter)
		if ferr != nil {
			lastDecodeErr = ferr
			return "", false
		}
		return fm.Class, true
	}

	data, err := os.ReadFile(store.ActiveSpecPath(root, name))
	if class, ok := tryDecode(data, err); ok {
		return class, nil
	}
	data, err = os.ReadFile(store.ArchiveSpecPath(root, name))
	if class, ok := tryDecode(data, err); ok {
		return class, nil
	}
	for _, ritualBranch := range []string{"design/" + name, "close/" + name} {
		data, err := gitx.Show(ctx, root, ritualBranch, activeRel)
		if class, ok := tryDecode(data, err); ok {
			return class, nil
		}
	}
	if defaultBranchRef != "" {
		locations = append(locations, "the resolved default-branch base ("+defaultBranchRef+")")
		for _, rel := range []string{activeRel, archiveRel} {
			data, err := gitx.Show(ctx, root, defaultBranchRef, rel)
			if class, ok := tryDecode(data, err); ok {
				return class, nil
			}
		}
	}

	if lastDecodeErr != nil {
		return "", fmt.Errorf("spec/%s's spec.md could not be decoded (last attempt: %w)", name, lastDecodeErr)
	}
	return "", fmt.Errorf("could not find spec/%s's spec.md at any of: %s", name, strings.Join(locations, "; "))
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// gatherLocalBranchTips resolves every local branch's tip once, up front,
// so both ritual-branch existence and R-RR3-5's ancestry predicate reuse
// the same read.
func gatherLocalBranchTips(ctx context.Context, root string) ([]BranchTip, []string) {
	names, err := gitx.LocalBranches(ctx, root)
	if err != nil {
		return nil, []string{fmt.Sprintf("could not list local branches: %v", err)}
	}
	var tips []BranchTip
	var disclosures []string
	for _, name := range names {
		tip, err := gitx.RevParse(ctx, root, name)
		if err != nil {
			disclosures = append(disclosures, fmt.Sprintf("could not resolve branch %s's tip: %v", name, err))
			continue
		}
		tips = append(tips, BranchTip{Name: name, Tip: tip})
	}
	return tips, disclosures
}

// gatherRitualBranch resolves name's existence, tip, and R-RR3-5's
// ancestry predicate against tips (every other local branch): name
// carries no commits of its own iff some OTHER branch's tip equals or
// descends from name's own tip, tested via gitx.MergeBase (the spike's
// documented equivalent to gitx.IsAncestor, which is outside this
// package's command-surface allow-list): tip is an ancestor of (or equal
// to) other iff MergeBase(tip, other) == tip.
func gatherRitualBranch(ctx context.Context, root, name string, tips []BranchTip) (RitualBranch, []string) {
	rb := RitualBranch{Name: name}
	var tip string
	for _, t := range tips {
		if t.Name == name {
			rb.Exists = true
			tip = t.Tip
			rb.Tip = tip
			break
		}
	}
	if !rb.Exists {
		return rb, nil
	}

	var disclosures []string
	for _, other := range tips {
		if other.Name == name {
			continue
		}
		mb, err := gitx.MergeBase(ctx, root, tip, other.Tip)
		if err != nil {
			// Disjoint history (no common ancestor at all, e.g. an
			// unrelated orphan branch): not a witness, not an error worth
			// disclosing on its own — a genuinely empty ritual branch has
			// at least one related candidate that resolves cleanly.
			continue
		}
		if mb == tip {
			rb.EmptyWitnesses = append(rb.EmptyWitnesses, other.Name)
		}
	}
	sort.Strings(rb.EmptyWitnesses)

	hasRemote, err := gitx.HasRemoteTrackingBranch(ctx, root, "origin", name)
	if err != nil {
		disclosures = append(disclosures, fmt.Sprintf("could not determine whether %s has a remote-tracking branch: %v", name, err))
		return rb, disclosures
	}
	rb.HasRemoteTracking = hasRemote
	rb.RemoteChecked = true
	if hasRemote {
		ahead, behind, err := gitx.AheadBehind(ctx, root, name, "origin/"+name)
		if err != nil {
			disclosures = append(disclosures, fmt.Sprintf("could not compute ahead/behind for %s against its remote-tracking branch: %v", name, err))
		} else {
			rb.Ahead, rb.Behind = ahead, behind
		}
	}
	return rb, disclosures
}

// gatherJournal peeks the draft-mutation journal for name, permissively
// decoding only {schema, spec, phase} (R-RR3-11). A symlinked journal
// path is disclosed, never followed.
func gatherJournal(root, name string) (JournalFact, []string) {
	path := store.DraftMutationJournalPath(root, name)
	jf := JournalFact{Path: path}

	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return jf, nil
		}
		// vocab:identity — "draft-mutation" names the internal/draftmutation package/artifact identity, not a lifecycle status
		return jf, []string{fmt.Sprintf("could not stat draft-mutation journal %s: %v", path, err)}
	}
	if info.Mode()&os.ModeSymlink != 0 {
		// vocab:identity — "draft-mutation" names the internal/draftmutation package/artifact identity, not a lifecycle status
		return jf, []string{fmt.Sprintf("draft-mutation journal %s is a symlink; not followed", path)}
	}
	// Present from here on regardless of whether the body itself decodes
	// (R-RR3-16): "present but undecodable" is itself a recognized fact.
	jf.Present = true

	data, err := os.ReadFile(path)
	if err != nil {
		jf.DecodeError = err.Error()
		// vocab:identity — "draft-mutation" names the internal/draftmutation package/artifact identity, not a lifecycle status
		return jf, []string{fmt.Sprintf("could not read draft-mutation journal %s: %v", path, err)}
	}
	var peek journalPeek
	if err := json.Unmarshal(data, &peek); err != nil {
		jf.DecodeError = err.Error()
		// vocab:identity — "draft-mutation" names the internal/draftmutation package/artifact identity, not a lifecycle status
		return jf, []string{fmt.Sprintf("could not decode draft-mutation journal %s: %v", path, err)}
	}
	jf.Decoded = true
	jf.Schema, jf.Spec, jf.Phase = peek.Schema, peek.Spec, peek.Phase

	if entries, err := os.ReadDir(store.DraftMutationDir(root, name)); err == nil {
		for _, e := range entries {
			jf.Steps = append(jf.Steps, e.Name())
		}
		sort.Strings(jf.Steps)
	}
	return jf, nil
}

// gatherWorkspace classifies every entry under execworkspace.ExecutionRoot
// into its owning workspace id's sibling presence, plus the lock siblings'
// own Inspect results (R-RR3-11). A missing execution root is normal (no
// execution-workspace history at all), not a disclosure.
func gatherWorkspace(root string) (units []WorkspaceUnit, locks []LockFact, unclassified []string, disclosures []string) {
	dir := execworkspace.ExecutionRoot(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil, nil
		}
		return nil, nil, nil, []string{fmt.Sprintf("could not list %s: %v", dir, err)}
	}

	byID := map[string]*WorkspaceUnit{}
	var order []string
	unit := func(id string) *WorkspaceUnit {
		if u, ok := byID[id]; ok {
			return u
		}
		u := &WorkspaceUnit{ID: id}
		byID[id] = u
		order = append(order, id)
		return u
	}

	for _, e := range entries {
		classified, ok := execworkspace.ClassifyEntry(e.Name())
		if !ok {
			// grammar-external (R-RR3-16): not silently dropped — it is
			// itself a recognized "outside the inventory" observation.
			unclassified = append(unclassified, e.Name())
			continue
		}
		u := unit(classified.WorkspaceID)
		switch classified.Form {
		case execworkspace.FormUnit:
			u.HasUnit = true
		case execworkspace.FormRequest:
			u.HasRequest = true
		case execworkspace.FormRequestStaging:
			u.HasRequestStaging = true
		case execworkspace.FormReleased:
			u.HasReleased = true
		case execworkspace.FormLock:
			u.HasLock = true
			u.LockPath = execworkspace.LockPath(root, classified.WorkspaceID)
			lf := LockFact{Path: u.LockPath}
			if insp, err := filelock.Inspect(u.LockPath); err != nil {
				disclosures = append(disclosures, fmt.Sprintf("could not inspect lock %s: %v", u.LockPath, err))
				lf.ReadError = err.Error()
			} else {
				lf.Inspection = insp
			}
			locks = append(locks, lf)
		}
	}

	sort.Strings(order)
	units = make([]WorkspaceUnit, 0, len(order))
	for _, id := range order {
		units = append(units, *byID[id])
	}
	sort.Strings(unclassified)
	sort.Slice(locks, func(i, j int) bool { return locks[i].Path < locks[j].Path })
	return units, locks, unclassified, disclosures
}

// filterReclaimRowsForRef keeps only rows whose unit branch is one of
// name's own ritual branches (design/close/feature), in the plan's own
// deterministic order.
func filterReclaimRowsForRef(rows []reclaim.Row, name string) []reclaim.Row {
	wanted := map[string]bool{
		"design/" + name:  true,
		"feature/" + name: true,
		"close/" + name:   true,
	}
	var out []reclaim.Row
	for _, r := range rows {
		if wanted[r.Unit.Branch] {
			out = append(out, r)
		}
	}
	return out
}

// dedupSorted removes adjacent duplicates from an already-sorted slice.
func dedupSorted(ss []string) []string {
	if len(ss) == 0 {
		return ss
	}
	out := ss[:1]
	for _, s := range ss[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}
