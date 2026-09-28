package objsupersede

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/objsupersede/scenario"
)

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// hermetic keeps an ambient CI_DEFAULT_BRANCH from choosing the branch.
func hermetic(t *testing.T) { t.Setenv("CI_DEFAULT_BRANCH", "") }

func obj(spec, id string) artifact.Ref {
	return artifact.Ref{Kind: artifact.KindSpec, Name: spec, Object: id}
}

// TestHistory_Acceptance pins SI-270: the earliest first-parent commit whose
// tree holds the spec in either zone, dated in UTC; a later in-place edit
// does not move it; a missing default branch or shallow history is unproven.
func TestHistory_Acceptance(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	chain := scenario.Build(t, "chain")
	proposed := scenario.Build(t, "proposed")
	unresolved := scenario.Build(t, "accepted")
	gitIn(t, unresolved.Dir, "update-ref", "-d", "refs/remotes/origin/main")
	shallow := fixturegit.ShallowClone(t, &fixturegit.Repo{Dir: chain.Dir}, 2)
	tests := []struct {
		name, dir, spec string
		want            Fact
	}{
		{"accepted by merge, dated by its committer date; the in-place edit does not move it", chain.Dir, "successor", Fact{State: FactProven, Commit: chain.Steps[1], Date: "2024-02-15"}},
		{"a revision", chain.Dir, "successor-v2", Fact{State: FactProven, Commit: chain.Steps[4], Date: "2024-03-15"}},
		{"accepted in the active zone; the later archive move does not move it", chain.Dir, "closed-feature", Fact{State: FactProven, Commit: chain.Base[1], Date: "2024-01-01"}},
		{"only on a design branch", proposed.Dir, "successor", Fact{State: FactAbsent}},
		{"never written", chain.Dir, "nowhere", Fact{State: FactAbsent}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewHistory(ctx, tc.dir).Acceptance(ctx, tc.spec); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
	for name, dir := range map[string]string{"no default branch": unresolved.Dir, "shallow history": shallow} {
		t.Run(name, func(t *testing.T) {
			got := NewHistory(ctx, dir).Acceptance(ctx, "successor")
			if got.State != FactUnproven || got.Witness == "" || got.Date != "" {
				t.Fatalf("got %+v, want unproven with a witness and no date", got)
			}
		})
	}
}

func TestUTCDay(t *testing.T) {
	tests := []struct{ iso, want string }{
		{"2024-02-15T09:00:00+00:00", "2024-02-15"},
		{"2024-02-15T23:30:00-05:00", "2024-02-16"},
		{"2024-02-16T01:00:00+09:00", "2024-02-15"},
	}
	for _, tc := range tests {
		if got, err := utcDay(tc.iso); err != nil || got != tc.want {
			t.Errorf("utcDay(%q) = %q, %v; want %q", tc.iso, got, err, tc.want)
		}
	}
	if got, err := utcDay("yesterday"); err == nil {
		t.Errorf("utcDay accepted a non-date: %q", got)
	}
}

// TestHistory_Closed pins SI-270's closed date: the committer date of the
// landing of specstate's Closed baseline, never invented from a shallow
// history (review a X-1) or an unresolved default branch.
func TestHistory_Closed(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	repo := scenario.Build(t, "accepted")
	recs := mustRead(t, CommitTree{Root: repo.Dir, Commit: "main"})
	shallow := fixturegit.ShallowClone(t, &fixturegit.Repo{Dir: repo.Dir}, 1)
	unresolved := scenario.Build(t, "accepted")
	gitIn(t, unresolved.Dir, "update-ref", "-d", "refs/remotes/origin/main")
	tests := []struct {
		name, dir, spec string
		want            Fact
		witness         string // unproven: the witness's required substring
	}{
		{"closed feature: the archive move, not its acceptance", repo.Dir, "closed-feature", Fact{State: FactProven, Commit: repo.Base[2], Date: "2024-01-10"}, ""},
		{"closed story: the archive move, by its committer date", repo.Dir, "closed-story", Fact{State: FactProven, Commit: repo.Base[2], Date: "2024-01-10"}, ""},
		{"an accepted, unclosed spec", repo.Dir, "other-feature", Fact{State: FactAbsent}, ""},
		{"shallow history", shallow, "closed-feature", Fact{State: FactUnproven}, "shallow history"},
		{"no default branch", unresolved.Dir, "closed-feature", Fact{State: FactUnproven}, "no default branch"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewHistory(ctx, tc.dir).Closed(ctx, recs.Specs[tc.spec])
			witnessOK := strings.Contains(got.Witness, tc.witness) && (tc.witness == "") == (got.Witness == "")
			if got.State != tc.want.State || got.Commit != tc.want.Commit || got.Date != tc.want.Date || !witnessOK {
				t.Fatalf("got %+v, want %+v with witness containing %q", got, tc.want, tc.witness)
			}
		})
	}
}

// TestHistory_Establishment pins SI-270's in-force rule: the new-replacement
// match evaluated on the acceptance commit's tree.
func TestHistory_Establishment(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	accepted := scenario.Build(t, "accepted")
	noBranchRepo := func(t *testing.T, dir string) {
		gitIn(t, dir, "update-ref", "-d", "refs/remotes/origin/main")
	}
	// unreadable drops one record blob of the acceptance commit from the
	// object store, so reading that commit's tree fails.
	unreadable := func(t *testing.T, dir string) {
		oid := gitOut(t, dir, "rev-parse", "main:.verdi/conflicts/successor-closed-story.md")
		if err := os.Remove(filepath.Join(dir, ".git", "objects", oid[:2], oid[2:])); err != nil {
			t.Fatal(err)
		}
	}
	// acceptBroken and acceptOddName accept design/successor together with
	// one more record: an undecodable spec (review b I-1's witness), or a
	// conflict under a name git quotes whose id disagrees with it (review a
	// I-1's witness).
	acceptBroken := func(t *testing.T, dir string) {
		acceptRecord(t, dir, ".verdi/specs/active/zz-broken/spec.md", "---\nid: [\n---\n")
	}
	acceptOddName := func(t *testing.T, dir string) {
		raw, err := os.ReadFile(filepath.Join(dir, ".verdi/conflicts/successor-closed-feature.md"))
		if err != nil {
			t.Fatal(err)
		}
		acceptRecord(t, dir, ".verdi/conflicts/successor-closed-feature-2é.md", string(raw))
	}
	tests := []struct {
		name, scenario, successor string
		prep                      func(*testing.T, string)

		object artifact.Ref
		want   Establishment // Detail: its required prefix; {main} names main's commit, where each reason is evaluated
	}{
		{"in force", "", "successor", nil, obj("closed-feature", "dc-1"), Establishment{Commit: accepted.Steps[1], Date: "2024-02-15"}},
		{"in force, criterion", "", "successor", nil, obj("closed-story", "ac-1"), Establishment{Commit: accepted.Steps[1], Date: "2024-02-15"}},
		{"no edge to the object at acceptance", "", "successor", nil, obj("closed-feature", "ac-1"), Establishment{Reason: ReasonEstablisherNotInForce, Detail: "as of commit {main}, spec/successor carries no edge to spec/closed-feature#ac-1"}},
		{"not accepted", "proposed", "successor", nil, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonEstablisherNotAccepted}},
		{"records did not match at acceptance", "unrelated-accepted", "unrelated", nil, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonEstablisherNotInForce, Detail: "as of commit {main}, the object spec/closed-feature#dc-1 is already superseded by spec/successor (conflict/successor-closed-feature)"}},
		{"acceptance unproven: no default branch", "accepted", "successor", noBranchRepo, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonAcceptanceUnproven, Detail: noBranch}},
		{"acceptance unproven: a record fails decode at acceptance", "proposed", "successor", acceptBroken, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonAcceptanceUnproven, Detail: "records do not decode at commit {main}: .verdi/specs/active/zz-broken/spec.md: "}},
		{"acceptance unproven: a record under a quoted name fails at acceptance", "proposed", "successor", acceptOddName, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonAcceptanceUnproven, Detail: "records do not decode at commit {main}: .verdi/conflicts/successor-closed-feature-2é.md: id conflict/successor-closed-feature disagrees"}},
		{"acceptance unproven: an unreadable acceptance commit", "accepted", "successor", unreadable, obj("closed-feature", "dc-1"), Establishment{Reason: ReasonAcceptanceUnproven, Detail: "the records at commit {main} cannot be read: objsupersede: reading .verdi/conflicts/successor-closed-story.md: "}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := accepted.Dir
			if tc.scenario != "" {
				dir = scenario.Build(t, tc.scenario).Dir
			}
			if tc.prep != nil {
				tc.prep(t, dir)
			}
			tc.want.Detail = strings.ReplaceAll(tc.want.Detail, "{main}", shortCommit(gitOut(t, dir, "rev-parse", "main")))
			got := NewHistory(ctx, dir).Establishment(ctx, tc.successor, tc.object)
			if got.Reason != tc.want.Reason || got.Commit != tc.want.Commit || got.Date != tc.want.Date ||
				!strings.HasPrefix(got.Detail, tc.want.Detail) || (tc.want.Detail == "") != (got.Detail == "") {
				t.Fatalf("got %+v, want %+v (Detail as a prefix)", got, tc.want)
			}
		})
	}
}

// acceptRecord commits one more record on design/successor and accepts
// that branch into main with a --no-ff merge.
func acceptRecord(t *testing.T, dir, path, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "--no-verify", "-m", "Add "+path)
	gitIn(t, dir, "checkout", "-q", "main")
	gitIn(t, dir, "merge", "-q", "--no-ff", "--no-verify", "-m", "Accept spec/successor", "design/successor")
	gitIn(t, dir, "update-ref", "refs/remotes/origin/main", "main")
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return strings.TrimSpace(string(out))
}

// TestEvaluate_EveryScenario builds every scenario of the committed manifest
// and pins, on its checked-out branch with the real history, each edge's
// intended outcome and reason, in result order ("-" marks a completeness
// result): the fixture's promise to lanes L4 and L5, machine-checked (lane
// L3 re-review a m-6).
func TestEvaluate_EveryScenario(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	const (
		newE     = "resolved-new"
		carried  = "resolved-carried"
		happy    = "dc-1 " + newE + ", dc-2 " + newE
		refusal1 = ", dc-2 " + newE // the other edge still resolves
	)
	want := map[string]struct{ spec, results string }{
		"accepted":                     {"successor", happy},
		"already-superseded":           {"successor", "dc-1 unresolved/already-superseded" + refusal1},
		"chain":                        {"successor-v3", "dc-1 " + carried + ", dc-2 " + carried},
		"chain-drop":                   {"successor-v2", "dc-2 " + carried},
		"chain-not-in-force":           {"successor-v2", "dc-1 unresolved/establisher-not-in-force, dc-2 " + carried + ", dc-3 unresolved/no-conflict"},
		"conflict-dismissed":           {"successor", "dc-1 unresolved/conflict-not-superseded" + refusal1},
		"conflict-open":                {"successor", "dc-1 unresolved/conflict-not-superseded" + refusal1},
		"conflict-spans-specs":         {"successor", "dc-1 unresolved/conflict-spans-specs, dc-2 unresolved/conflict-spans-specs"},
		"constraint-target":            {"successor", "dc-1 unresolved/object-not-criterion-or-decision" + refusal1},
		"feature-criterion":            {"successor", happy + ", dc-3 " + newE},
		"feature-fragment-link":        {"successor", happy},
		"ff-close-in-pr":               {"successor", happy},
		"ff-close-in-pr-then-conflict": {"successor", happy},
		"ff-landing":                   {"successor", happy},
		"ff-widening-series":           {"successor", happy + ", dc-3 " + newE},
		"late-close":                   {"successor", happy},
		"late-close-rival":             {"unrelated", "dc-1 unresolved/already-superseded"},
		"late-close-then-conflict":     {"successor", happy},
		"late-close-tie":               {"successor", "dc-1 unresolved/acceptance-unproven" + refusal1},
		"no-conflict":                  {"successor", "dc-1 unresolved/no-conflict" + refusal1},
		"proposed":                     {"successor", happy},
		"rebase-landing":               {"successor", happy},
		"resolved-by-other":            {"successor", "dc-1 unresolved/resolved-by-other" + refusal1},
		"same-commit-tie":              {"successor", "dc-1 unresolved/acceptance-unproven" + refusal1},
		"skeleton-landing":             {"successor", happy},
		"stale-base":                   {"unrelated", "dc-1 " + newE},
		"target-not-closed":            {"successor", "dc-1 unresolved/target-not-closed" + refusal1},
		"top-level-supersedes":         {"successor", "dc-2 " + newE},
		"undecodable-before-point":     {"successor", happy},
		"undeclared-object":            {"successor", "dc-1 unresolved/object-not-declared" + refusal1},
		"unmatched-challenge":          {"successor", happy + ", - unresolved/unmatched-challenge"},
		"unrelated":                    {"unrelated", "dc-1 unresolved/already-superseded"},
		"unrelated-accepted":           {"unrelated", "dc-1 unresolved/already-superseded"},
	}
	m, err := scenario.Load(scenario.Dir())
	if err != nil {
		t.Fatal(err)
	}
	for name := range m.Scenarios {
		if _, ok := want[name]; !ok {
			t.Errorf("scenario %q has no intended outcome here", name)
		}
	}
	for name, w := range want {
		t.Run(name, func(t *testing.T) {
			dir := scenario.Build(t, name).Dir
			res, err := Evaluate(ctx, mustRead(t, WorkTree{Root: dir}), w.spec, NewHistory(ctx, dir))
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range res {
				d, o := r.Decision, string(r.Outcome)
				if d == "" {
					d = "-"
				}
				if r.Reason != "" {
					o += "/" + string(r.Reason)
				}
				got = append(got, d+" "+o)
			}
			if strings.Join(got, ", ") != w.results {
				t.Fatalf("spec/%s: %s\nwant %s", w.spec, strings.Join(got, ", "), w.results)
			}
		})
	}
}

// TestHistory_EstablishmentLandings is the whole-wave review's F-4
// witness, rebuilt: a successor whose pull request lands without a merge
// commit (a fast-forward, or a rebase onto a main that moved) is in force
// from the earliest first-parent commit at which it is present AND §3's
// match for (S_k, T) holds, dated by that commit's committer date (SI-270
// as amended), never refused at the commit that first holds its spec. The
// point is per (S_k, T), so an object S_k has no edge to is not in force.
func TestHistory_EstablishmentLandings(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	ff := scenario.Build(t, "ff-landing")
	rb := scenario.Build(t, "rebase-landing")
	wide := scenario.Build(t, "ff-widening-series")
	tests := []struct {
		name   string
		repo   *scenario.Repo
		object artifact.Ref
		want   Establishment
	}{
		{"fast-forward: the decision, at the series' second commit", ff, obj("closed-feature", "dc-1"), Establishment{Commit: ff.Steps[1], Date: "2024-02-10"}},
		{"fast-forward: the criterion, at the series' second commit", ff, obj("closed-story", "ac-1"), Establishment{Commit: ff.Steps[1], Date: "2024-02-10"}},
		{"rebase: the decision, at the replayed second commit", rb, obj("closed-feature", "dc-1"), Establishment{Commit: rb.Steps[3], Date: "2024-02-15"}},
		{"rebase: the criterion, at the replayed second commit", rb, obj("closed-story", "ac-1"), Establishment{Commit: rb.Steps[3], Date: "2024-02-15"}},
		{"fast-forward: an object the successor has no edge to, named at the point", ff, obj("closed-feature", "ac-1"), Establishment{Reason: ReasonEstablisherNotInForce, Detail: "as of commit " + shortCommit(ff.Steps[1]) + ", spec/successor carries no edge to spec/closed-feature#ac-1"}},
		{"a widening series: in force at its first commit, where the match first holds", wide, obj("closed-feature", "dc-1"), Establishment{Commit: wide.Steps[0], Date: "2024-02-01"}},
		// BL-94's residual, named at the point so it never reads as a
		// statement about the head, which carries the edge (SI-281).
		{"a widening series: the later edge's reason names the point", wide, obj("closed-feature", "ac-1"), Establishment{Reason: ReasonEstablisherNotInForce, Detail: "as of commit " + shortCommit(wide.Steps[0]) + ", spec/successor carries no edge to spec/closed-feature#ac-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewHistory(ctx, tc.repo.Dir).Establishment(ctx, "successor", tc.object); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
	// The accepted fact is still the first-parent commit that first holds
	// the spec: acceptance and the in-force point differ here.
	if got := NewHistory(ctx, ff.Dir).Acceptance(ctx, "successor"); got != (Fact{State: FactProven, Commit: ff.Steps[0], Date: "2024-02-01"}) {
		t.Fatalf("acceptance %+v, want the series' first commit", got)
	}
}

// TestHistory_EstablishmentCorrective pins SI-270 as amended on a later
// corrective commit: a successor whose records did not match when it
// landed takes effect at the first-parent commit where they first match,
// dated by it; the walk visits only commits touching its spec paths or
// .verdi/conflicts/, from its first presence.
func TestHistory_EstablishmentCorrective(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	repo := scenario.Build(t, "chain-not-in-force")
	gitIn(t, repo.Dir, "checkout", "-q", "main")
	// A default-branch commit outside the walk: it moves no fact.
	commitAt(t, repo.Dir, "2024-02-18T09:00:00Z", "README.md", "unrelated\n")
	// The corrective commit: the successor's conflict for the closed
	// feature now also challenges #ac-1, which its dc-3 supersedes.
	unmatched, err := os.ReadFile(filepath.Join(scenario.Dir(), "records", "conflicts", "successor-closed-feature-unmatched.md"))
	if err != nil {
		t.Fatal(err)
	}
	corrective := commitAt(t, repo.Dir, "2024-02-20T09:00:00Z", ".verdi/conflicts/successor-closed-feature.md", string(unmatched))
	h := NewHistory(ctx, repo.Dir)
	for _, o := range []artifact.Ref{obj("closed-feature", "dc-1"), obj("closed-feature", "ac-1")} {
		if got := h.Establishment(ctx, "successor", o); got != (Establishment{Commit: corrective, Date: "2024-02-20"}) {
			t.Errorf("%s: got %+v, want in force at the corrective commit since 2024-02-20", o, got)
		}
	}
	// The closed story's match held at the merge: its point does not move.
	if got := h.Establishment(ctx, "successor", obj("closed-story", "ac-1")); got != (Establishment{Commit: repo.Steps[1], Date: "2024-02-15"}) {
		t.Errorf("closed-story#ac-1: got %+v, want in force at the merge since 2024-02-15", got)
	}
}

// TestHistory_EstablishmentStaleBase is the whole-wave review's F-1
// witness, rebuilt: in a conforming stale-base merge the successor's
// supersession of the closed feature is never in force, and under SI-272
// as amended it never refuses a later successor, which is in force once
// accepted; the closed story's supersession, whose records matched, is.
func TestHistory_EstablishmentStaleBase(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	repo := scenario.Build(t, "stale-base")
	// gap is the successor's reason, named at the latest commit its walk
	// evaluated (SI-281): the merge, or a later commit the walk visits.
	gap := func(at string) Establishment {
		return Establishment{Reason: ReasonEstablisherNotInForce, Detail: "as of commit " + shortCommit(at) + ", conflict/successor-closed-feature challenges spec/closed-feature#ac-1, but spec/successor carries no matching edge"}
	}
	story := Establishment{Commit: repo.Steps[3], Date: "2024-02-15"}
	before := NewHistory(ctx, repo.Dir)
	for _, tc := range []struct {
		successor string
		object    artifact.Ref
		want      Establishment
	}{
		{"successor", obj("closed-feature", "dc-1"), gap(repo.Steps[3])},
		{"successor", obj("closed-story", "ac-1"), story},
		{"unrelated", obj("closed-feature", "dc-1"), Establishment{Reason: ReasonEstablisherNotAccepted}},
	} {
		if got := before.Establishment(ctx, tc.successor, tc.object); got != tc.want {
			t.Errorf("before spec/unrelated is accepted: spec/%s, %s: got %+v, want %+v", tc.successor, tc.object, got, tc.want)
		}
	}
	// Accept spec/unrelated with a merge commit.
	gitIn(t, repo.Dir, "checkout", "-q", "main")
	cmd := exec.Command("git", "-C", repo.Dir, "merge", "-q", "--no-ff", "--no-verify", "-m", "Accept spec/unrelated", "design/unrelated")
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2024-03-15T09:00:00Z", "GIT_COMMITTER_DATE=2024-03-15T09:00:00Z")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git merge: %v\n%s", err, out)
	}
	gitIn(t, repo.Dir, "update-ref", "refs/remotes/origin/main", "main")
	accepted := gitOut(t, repo.Dir, "rev-parse", "main")
	after := NewHistory(ctx, repo.Dir)
	for _, tc := range []struct {
		successor string
		object    artifact.Ref
		want      Establishment
	}{
		{"unrelated", obj("closed-feature", "dc-1"), Establishment{Commit: accepted, Date: "2024-03-15"}},
		{"successor", obj("closed-feature", "dc-1"), gap(accepted)},
		{"successor", obj("closed-story", "ac-1"), story},
	} {
		if got := after.Establishment(ctx, tc.successor, tc.object); got != tc.want {
			t.Errorf("after spec/unrelated is accepted: spec/%s, %s: got %+v, want %+v", tc.successor, tc.object, got, tc.want)
		}
	}
}

// TestHistory_EstablishmentLateClose is the lane L6 review's I-1 witness,
// rebuilt (SI-281's walk set): the walk for (S, T) visits T's spec paths
// in both zones, so a target closed after the successor landed is seen at
// its archive commit and dated by it, never at a later commit the walk
// happens to visit; and a rival's walk in the tie check uses its own (X, T)
// set, so two successors that land before the close first match at the
// archive commit together (SI-281(2)).
func TestHistory_EstablishmentLateClose(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	late := scenario.Build(t, "late-close")
	lateThen := scenario.Build(t, "late-close-then-conflict")
	ff := scenario.Build(t, "ff-close-in-pr")
	ffThen := scenario.Build(t, "ff-close-in-pr-then-conflict")
	rival := scenario.Build(t, "late-close-rival")
	tie := scenario.Build(t, "late-close-tie")
	other, story := obj("other-feature", "dc-1"), obj("closed-story", "ac-1")
	tied := func(a, b string) Establishment {
		return Establishment{Reason: ReasonAcceptanceUnproven, Detail: "spec/" + a + " and spec/" + b + " first match for spec/other-feature#dc-1 at the same commit " + tie.Steps[4] + ", so neither takes effect before the other"}
	}
	tests := []struct {
		name      string
		repo      *scenario.Repo
		successor string
		object    artifact.Ref
		want      Establishment
	}{
		{"closed after the merge: in force at the archive commit", late, "successor", other, Establishment{Commit: late.Steps[2], Date: "2024-03-01"}},
		{"closed after the merge: the closed story's point does not move", late, "successor", story, Establishment{Commit: late.Steps[1], Date: "2024-02-15"}},
		{"a later conflict filing: dated by the archive commit, never by the filing", lateThen, "successor", other, Establishment{Commit: lateThen.Steps[2], Date: "2024-03-01"}},
		{"closed by the pull request's last commit, landed by fast-forward", ff, "successor", other, Establishment{Commit: ff.Steps[1], Date: "2024-02-10"}},
		{"closed in the pull request: the closed story at the series' first commit", ff, "successor", story, Establishment{Commit: ff.Steps[0], Date: "2024-02-01"}},
		{"closed in the pull request, then a later conflict filing", ffThen, "successor", other, Establishment{Commit: ffThen.Steps[1], Date: "2024-02-10"}},
		{"a rival after the late close: the successor stays in force, no tie", rival, "successor", other, Establishment{Commit: rival.Steps[2], Date: "2024-03-01"}},
		{"a rival after the late close is already superseded", rival, "unrelated", other, Establishment{Reason: ReasonEstablisherNotInForce, Detail: "as of commit " + shortCommit(rival.Steps[4]) + ", the object spec/other-feature#dc-1 is already superseded by spec/successor (conflict/successor-other-feature)"}},
		{"two successors landed before the close tie at the archive commit", tie, "successor", other, tied("successor", "unrelated")},
		{"two successors landed before the close: asked of the rival", tie, "unrelated", other, tied("unrelated", "successor")},
		{"two successors landed before the close: the closed story is not tied", tie, "successor", story, Establishment{Commit: tie.Steps[1], Date: "2024-02-15"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := NewHistory(ctx, tc.repo.Dir).Establishment(ctx, tc.successor, tc.object); got != tc.want {
				t.Fatalf("got  %+v\nwant %+v", got, tc.want)
			}
		})
	}
}

// atSteps replaces each "{step N}" in s with repo's step N commit as a
// reason names it (shortCommit).
func atSteps(repo *scenario.Repo, s string) string {
	for i, c := range repo.Steps {
		s = strings.ReplaceAll(s, fmt.Sprintf("{step %d}", i), shortCommit(c))
	}
	return s
}

// commitAt writes path on the checked-out branch, commits it with author
// and committer date date, points origin/main at main, and returns the
// commit.
func commitAt(t *testing.T, dir, date, path, content string) string {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "add", "-A")
	cmd := exec.Command("git", "-C", dir, "commit", "-q", "--no-verify", "-m", "Commit "+path)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	gitIn(t, dir, "update-ref", "refs/remotes/origin/main", "main")
	return gitOut(t, dir, "rev-parse", "HEAD")
}

// TestHistory_EstablishmentBelowGitRoot pins that a store below the git
// root reads its acceptance commit's own records (lane L3 re-review a I-A),
// never "not in its acceptance commit's tree".
func TestHistory_EstablishmentBelowGitRoot(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	root := underSubdir(t, scenario.Build(t, "accepted"))
	got := NewHistory(ctx, root).Establishment(ctx, "successor", obj("closed-feature", "dc-1"))
	if got.Reason != "" || got.Date != "2024-02-15" || got.Commit == "" {
		t.Fatalf("got %+v, want in force since 2024-02-15", got)
	}
}

// TestEvaluate_Scenarios drives Evaluate over built scenario stores with the
// real history: §8's proposed, carried, and unrelated-reuse paths.
func TestEvaluate_Scenarios(t *testing.T) {
	hermetic(t)
	ctx := context.Background()
	chain := scenario.Build(t, "chain")
	tests := []struct {
		name, scenario, branch, spec, dc string
		want                             string
	}{
		{"proposed successor", "proposed", "", "successor", "dc-1", "records match; takes effect when spec/successor is accepted"},
		{"S2 carries S1's replacement", "chain", "design/successor-v2", "successor-v2", "dc-1", "carries the replacement established by spec/successor (conflict/successor-closed-feature, since 2024-02-15)"},
		{"S3 amends and still carries it", "chain", "design/successor-v3", "successor-v3", "dc-1", "carries the replacement established by spec/successor (conflict/successor-closed-feature, since 2024-02-15)"},
		{"S3 amends the closed criterion's replacement and still carries it", "chain", "design/successor-v3", "successor-v3", "dc-2", "carries the replacement established by spec/successor (conflict/successor-closed-story, since 2024-02-15)"},
		{"S1 was not in force for T at acceptance: a sibling edge had no conflict", "chain-not-in-force", "", "successor-v2", "dc-1", "spec/successor's supersession was not in force at its acceptance: as of commit {step 1}, no conflict challenges spec/closed-feature#ac-1"},
		{"the same S1 was in force for another closed spec", "chain-not-in-force", "", "successor-v2", "dc-2", "carries the replacement established by spec/successor (conflict/successor-closed-story, since 2024-02-15)"},
		{"unrelated reuse", "unrelated", "", "unrelated", "dc-1", "the object spec/closed-feature#dc-1 is already superseded by spec/successor (conflict/successor-closed-feature)"},
		{"stale base: the successor's records match at its own head", "stale-base", "design/successor", "successor", "dc-1", "records match; takes effect when spec/successor is accepted"},
		{"stale base: a successor not in force never refuses a later one (F-1)", "stale-base", "", "unrelated", "dc-1", "records match; takes effect when spec/unrelated is accepted"},
		{"a rival proposed after a late close is refused (I-1)", "late-close-rival", "design/unrelated", "unrelated", "dc-1", "the object spec/other-feature#dc-1 is already superseded by spec/successor (conflict/successor-other-feature)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := chain
			if tc.scenario != "chain" {
				repo = scenario.Build(t, tc.scenario)
			}
			dir := repo.Dir
			if tc.branch != "" {
				gitIn(t, dir, "checkout", "-q", tc.branch)
			}
			res, err := Evaluate(ctx, mustRead(t, WorkTree{Root: dir}), tc.spec, NewHistory(ctx, dir))
			if err != nil {
				t.Fatal(err)
			}
			want := atSteps(repo, tc.want)
			got, err := result(t, res, tc.dc).Text()
			if err != nil || got != want {
				t.Fatalf("got %q, %v; want %q", got, err, want)
			}
		})
	}
}
