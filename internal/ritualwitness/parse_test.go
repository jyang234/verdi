package ritualwitness

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

const (
	oidA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	oidB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	oidC = "cccccccccccccccccccccccccccccccccccccccc"
)

func TestIsObjectID(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{oidA, true},
		{strings.Repeat("0", 64), true},
		{"", false},
		{strings.Repeat("a", 39), false},
		{strings.Repeat("A", 40), false},
		{strings.Repeat("g", 40), false},
		{strings.Repeat("a", 41), false},
	}
	for _, tt := range tests {
		if got := isObjectID(tt.in); got != tt.want {
			t.Errorf("isObjectID(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestParseRefs(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    map[string]Ref
		wantErr bool
	}{
		{"empty output is no refs", "", map[string]Ref{}, false},
		{"plain and symbolic refs", oidA + "\x00refs/heads/main\x00\n" + oidA + "\x00refs/remotes/origin/HEAD\x00refs/remotes/origin/main\n",
			map[string]Ref{"refs/heads/main": {Object: oidA}, "refs/remotes/origin/HEAD": {Object: oidA, Symref: "refs/remotes/origin/main"}}, false},
		{"a line with too few fields", oidA + "\x00refs/heads/main\n", nil, true},
		{"a line with too many fields", oidA + "\x00refs/heads/main\x00\x00x\n", nil, true},
		{"a malformed object id", "xyz\x00refs/heads/main\x00\n", nil, true},
		{"a refname outside refs/", oidA + "\x00HEAD\x00\n", nil, true},
		{"a duplicate refname", oidA + "\x00refs/heads/main\x00\n" + oidB + "\x00refs/heads/main\x00\n", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseRefs([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseRefs err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseRefs = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseIndex(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []IndexEntry
		wantErr bool
	}{
		{"empty index", "", nil, false},
		{"entries sorted by path then stage, NUL paths kept verbatim, flag tags kept",
			"H 100644 " + oidB + " 0\tz.txt\x00h 100755 " + oidA + " 0\tcaf\u00e9 \"q\".txt\x00S 100644 " + oidC + " 0\tk\x00",
			[]IndexEntry{
				{Tag: "h", Mode: "100755", Object: oidA, Stage: 0, Path: "caf\u00e9 \"q\".txt"},
				{Tag: "S", Mode: "100644", Object: oidC, Stage: 0, Path: "k"},
				{Tag: "H", Mode: "100644", Object: oidB, Stage: 0, Path: "z.txt"},
			}, false},
		{"no tab", "H 100644 " + oidA + " 0 x\x00", nil, true},
		{"no tag (ls-files -s alone)", "100644 " + oidA + " 0\tx\x00", nil, true},
		{"an unknown tag", "Q 100644 " + oidA + " 0\tx\x00", nil, true},
		{"a non-numeric stage", "H 100644 " + oidA + " s\tx\x00", nil, true},
		{"a stage above 3", "H 100644 " + oidA + " 4\tx\x00", nil, true},
		{"a malformed object id", "H 100644 nothex 0\tx\x00", nil, true},
		{"an empty path", "H 100644 " + oidA + " 0\t\x00", nil, true},
		{"output not NUL-terminated", "H 100644 " + oidA + " 0\tx", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIndex([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseIndex err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIndex = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseReflogStart(t *testing.T) {
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{"a worktree add", strings.Repeat("0", 40) + " " + oidA + " F <f@x> 1 +0000\n" + oidA + " " + oidB + " F <f@x> 2 +0000\tcommit\n", oidA, false},
		{"one entry without a newline", strings.Repeat("0", 40) + " " + oidB + " F <f@x> 1 +0000", oidB, false},
		{"empty", "", "", true},
		{"a malformed new id", strings.Repeat("0", 40) + " nothex F <f@x> 1 +0000\n", "", true},
		{"too few fields", oidA + "\n", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReflogStart([]byte(tt.in))
			if (err != nil) != tt.wantErr || got != tt.want {
				t.Fatalf("parseReflogStart = (%q, %v), want (%q, err %v)", got, err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestParseTree(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    map[string]TreeEntry
		wantErr bool
	}{
		{"empty tree", "", map[string]TreeEntry{}, false},
		{"a blob and a gitlink, paths verbatim", "100644 blob " + oidA + "\tcaf\u00e9.txt\x00160000 commit " + oidB + "\tsub\x00",
			map[string]TreeEntry{"caf\u00e9.txt": {Mode: "100644", Object: oidA}, "sub": {Mode: "160000", Object: oidB}}, false},
		{"no tab", "100644 blob " + oidA + " x\x00", nil, true},
		{"a tree entry (ls-tree without -r)", "040000 tree " + oidA + "\tdir\x00", nil, true},
		{"a malformed object id", "100644 blob nothex\tx\x00", nil, true},
		{"a path listed twice", "100644 blob " + oidA + "\tx\x00100644 blob " + oidB + "\tx\x00", nil, true},
		{"not NUL-terminated", "100644 blob " + oidA + "\tx", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTree([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseTree err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTree = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseStatus(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []StatusEntry
		wantErr bool
	}{
		{"clean tree", "", nil, false},
		{"modified, added, untracked, sorted",
			"?? u.txt\x00 M t.txt\x00A  f.txt\x00",
			[]StatusEntry{{X: 'A', Y: ' ', Path: "f.txt"}, {X: ' ', Y: 'M', Path: "t.txt"}, {X: '?', Y: '?', Path: "u.txt"}}, false},
		{"a rename carries its source", "R  new.txt\x00old.txt\x00",
			[]StatusEntry{{X: 'R', Y: ' ', Path: "new.txt", OrigPath: "old.txt"}}, false},
		{"a rename with no source field", "R  new.txt\x00", nil, true},
		{"too short an entry", "M\x00", nil, true},
		{"no separator after the columns", "MMx.txt\x00", nil, true},
		{"output not NUL-terminated", " M t.txt", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStatus([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseStatus err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseStatus = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParsePaths(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"empty", "", nil, false},
		{"NUL-terminated paths, kept verbatim and deduplicated in order", "b\x00a b\x00b\x00café\x00", []string{"b", "a b", "café"}, false},
		{"an empty field", "a\x00\x00b\x00", nil, true},
		{"not NUL-terminated", "a\x00b", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePaths([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePaths err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePaths = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseCommitObjects(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    []string
		wantErr bool
	}{
		{"no objects", "", nil, false},
		{"only commits are kept", oidA + " commit\n" + oidB + " blob\n" + oidC + " tree\n", []string{oidA}, false},
		{"a line with one field", oidA + "\n", nil, true},
		{"a malformed object id", "nothex commit\n", nil, true},
		{"an unknown object type", oidA + " widget\n", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCommitObjects([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseCommitObjects err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCommitObjects = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseParents(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    map[string][]string
		wantErr bool
	}{
		{"no commits", "", map[string][]string{}, false},
		{"a root, a child, a merge", oidA + "\n" + oidB + " " + oidA + "\n" + oidC + " " + oidB + " " + oidA + "\n",
			map[string][]string{oidA: nil, oidB: {oidA}, oidC: {oidB, oidA}}, false},
		{"a malformed commit id", "nothex\n", nil, true},
		{"a malformed parent id", oidA + " nothex\n", nil, true},
		{"a commit listed twice", oidA + "\n" + oidA + "\n", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseParents([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseParents err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseParents = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    map[string][]string
		wantErr bool
	}{
		{"empty config", "", map[string][]string{}, false},
		{"values, a multi-valued key, and a value-less key",
			"core.bare\nfalse\x00remote.origin.fetch\na\x00remote.origin.fetch\nb\x00core.flag\x00",
			map[string][]string{"core.bare": {"=false"}, "remote.origin.fetch": {"=a", "=b"}, "core.flag": {"(no value)"}}, false},
		{"a value containing a newline", "x.y\nline1\nline2\x00", map[string][]string{"x.y": {"=line1\nline2"}}, false},
		{"an empty key", "\nv\x00", nil, true},
		{"not NUL-terminated", "x.y\nv", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfig([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseConfig err = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseConfig = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestParseHeadFile(t *testing.T) {
	tests := []struct {
		name             string
		in               string
		wantRef, wantOID string
		wantErr          bool
	}{
		{"attached", "ref: refs/heads/main\n", "refs/heads/main", "", false},
		{"detached", oidA + "\n", "", oidA, false},
		{"a symbolic ref outside refs/", "ref: HEAD\n", "", "", true},
		{"garbage", "hello\n", "", "", true},
		{"empty", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, oid, err := parseHeadFile([]byte(tt.in))
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseHeadFile err = %v, wantErr %v", err, tt.wantErr)
			}
			if ref != tt.wantRef || oid != tt.wantOID {
				t.Errorf("parseHeadFile = (%q, %q), want (%q, %q)", ref, oid, tt.wantRef, tt.wantOID)
			}
		})
	}
}

// exitError returns a real *exec.ExitError with the given code, from a
// shell that exits with it (hermetic: no network, no repository).
func exitError(t *testing.T, code string) error {
	t.Helper()
	err := exec.CommandContext(context.Background(), "sh", "-c", "exit "+code).Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("sh -c 'exit %s' gave %v, want an *exec.ExitError", code, err)
	}
	return err
}

func TestHeadFromSymbolicRef(t *testing.T) {
	tests := []struct {
		name         string
		out          string
		err          error
		wantRef      string
		wantDetached bool
		wantErr      bool
	}{
		{"attached", "refs/heads/main\n", nil, "refs/heads/main", false, false},
		{"exit 1 means detached", "", exitError(t, "1"), "", true, false},
		{"exit 128 is operational, never detached", "", exitError(t, "128"), "", false, true},
		{"a non-exit error is operational", "", errors.New("exec: not found"), "", false, true},
		{"an attached HEAD outside refs/", "HEAD\n", nil, "", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, detached, err := headFromSymbolicRef([]byte(tt.out), tt.err)
			if (err != nil) != tt.wantErr {
				t.Fatalf("headFromSymbolicRef err = %v, wantErr %v", err, tt.wantErr)
			}
			if ref != tt.wantRef || detached != tt.wantDetached {
				t.Errorf("headFromSymbolicRef = (%q, %v), want (%q, %v)", ref, detached, tt.wantRef, tt.wantDetached)
			}
		})
	}
}
