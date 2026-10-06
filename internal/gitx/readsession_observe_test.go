package gitx

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// sessionEvents records a context's processes and its read sessions'
// events, in order: each entry is the event name ("exec" for a process)
// and the argv.
type sessionEvents struct {
	mu     sync.Mutex
	events []string
}

func (s *sessionEvents) Observe(_ string, args []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, "exec "+strings.Join(args, " "))
}

func (s *sessionEvents) ObserveSession(_ string, event SessionEvent, args []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, strings.TrimSpace(string(event)+" "+strings.Join(args, " ")))
}

func (s *sessionEvents) list() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.events...)
}

// TestSessionObserver_SeesWhatTheSessionAnswers (ledger SI-356; Wave 6
// §5.3): an observer implementing SessionObserver is told of the session
// opening (once, not again for a nested call), of every read the session
// replays, and of every object name it sends its batch process — the
// reads a process-only observer never sees.
func TestSessionObserver_SeesWhatTheSessionAnswers(t *testing.T) {
	repo := sessionRepo(t)
	obs := &sessionEvents{}
	ctx, release := WithReadSession(WithObserver(context.Background(), obs), repo.Dir)
	nested, nestedRelease := WithReadSession(ctx, repo.Dir)
	nestedRelease()
	for range 2 {
		if _, err := RevParse(nested, repo.Dir, "HEAD"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Show(ctx, repo.Dir, "HEAD", "plain.txt"); err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"opened",
		"exec rev-parse --verify HEAD",
		"replayed rev-parse --verify HEAD",
		"exec cat-file --batch",
		"batched HEAD:plain.txt",
		"exec rev-parse --verify HEAD", // after release: its own process, no event
	}
	if got := obs.list(); !reflect.DeepEqual(got, want) {
		t.Fatalf("events = %q\nwant %q", got, want)
	}
}

// TestSessionObserver_ProcessOnlyObserverUnchanged: an observer without
// ObserveSession sees exactly the processes it always saw.
func TestSessionObserver_ProcessOnlyObserverUnchanged(t *testing.T) {
	repo := sessionRepo(t)
	obs := newCountingObserver()
	ctx, release := WithReadSession(WithObserver(context.Background(), obs), repo.Dir)
	defer release()
	for range 2 {
		if _, err := RevParse(ctx, repo.Dir, "HEAD"); err != nil {
			t.Fatal(err)
		}
		if _, err := Show(ctx, repo.Dir, "HEAD", "plain.txt"); err != nil {
			t.Fatal(err)
		}
	}
	want := [][]string{{"rev-parse", "--verify", "HEAD"}, {"cat-file", "--batch"}}
	if !reflect.DeepEqual(obs.argv, want) {
		t.Fatalf("a process-only observer saw %q, want %q", obs.argv, want)
	}
}

// TestReadSession_TreeListingAndPrefixRunOnce: within a session, a full
// id's whole-tree listing and the directory's prefix each run once and
// are replayed byte for byte, error for error; a listing of a ref name,
// which resolves the ref, runs every time.
func TestReadSession_TreeListingAndPrefixRunOnce(t *testing.T) {
	repo := sessionRepo(t)
	full := gitT(t, repo.Dir, "rev-parse", "HEAD")
	missing := strings.Repeat("0", 40)
	reads := func(ctx context.Context) string {
		var b strings.Builder
		for range 2 {
			for _, rev := range []string{full, missing, "HEAD"} {
				entries, err := LsTreeEntries(ctx, repo.Dir, rev)
				b.WriteString(rev + ":" + errText(err) + "|")
				for _, e := range entries {
					b.WriteString(e.Mode + " " + e.Object + " " + e.Path + ";")
				}
			}
			prefix, err := RepoPrefix(ctx, repo.Dir)
			b.WriteString("prefix " + prefix + errText(err) + "|")
		}
		return b.String()
	}
	want := reads(context.Background())
	obs := newCountingObserver()
	ctx, release := WithReadSession(WithObserver(context.Background(), obs), repo.Dir)
	defer release()
	if got := reads(ctx); got != want {
		t.Fatalf("listings in a session = %q\nwant %q", got, want)
	}
	// Once each for the full and the missing id, twice for HEAD, once for
	// the prefix.
	if obs.count("ls-tree") != 4 || obs.count("rev-parse") != 1 || obs.total() != 5 {
		t.Fatalf("two rounds launched %q, want 4 listings (HEAD's twice) and one prefix read", obs.argv)
	}
}

// TestMemoizable_Shapes pins exactly which argv a session may replay.
func TestMemoizable_Shapes(t *testing.T) {
	full := strings.Repeat("a", 40)
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"rev-parse", "--show-prefix"}, true},
		{[]string{"ls-tree", "-rz", "--full-tree", full}, true},
		{[]string{"ls-tree", "-rz", "--full-tree", strings.Repeat("b", 64)}, true},
		{[]string{"ls-tree", "-rz", "--full-tree", "HEAD"}, false},
		{[]string{"ls-tree", "-rz", "--full-tree", full[:39]}, false},
		{[]string{"ls-tree", "-rtz", "--full-tree", full}, false},
		{[]string{"ls-tree", "-r", "--name-only", full, "--", ".verdi/specs"}, false},
		{[]string{"rev-parse", "--show-toplevel", "--show-prefix"}, false},
		{[]string{"rev-parse", "--verify", "HEAD"}, true},
		{[]string{"symbolic-ref", "HEAD", "refs/heads/x"}, false},
	} {
		if got := memoizable(tc.args); got != tc.want {
			t.Errorf("memoizable(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}
