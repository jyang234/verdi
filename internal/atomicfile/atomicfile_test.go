package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWrite_Happy(t *testing.T) {
	tests := []struct {
		name string
		rel  string // path relative to a fresh t.TempDir(), parent dirs may not exist yet
		data []byte
		perm os.FileMode
	}{
		{name: "flat file, parent already exists", rel: "flat.txt", data: []byte("hello\n"), perm: 0o600},
		{name: "nested parents auto-created", rel: filepath.Join("a", "b", "c", "nested.json"), data: []byte(`{"x":1}`), perm: 0o644},
		{name: "empty content", rel: "empty.txt", data: []byte(""), perm: 0o600},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, tc.rel)

			if err := Write(path, tc.data, tc.perm); err != nil {
				t.Fatalf("Write: %v", err)
			}

			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if string(got) != string(tc.data) {
				t.Fatalf("content = %q, want %q", got, tc.data)
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("Stat: %v", err)
			}
			if info.Mode().Perm() != tc.perm {
				t.Fatalf("perm = %v, want %v", info.Mode().Perm(), tc.perm)
			}
		})
	}
}

func TestWrite_OverwritesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")

	if err := Write(path, []byte("first"), 0o600); err != nil {
		t.Fatalf("Write 1: %v", err)
	}
	if err := Write(path, []byte("second, longer content"), 0o600); err != nil {
		t.Fatalf("Write 2: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "second, longer content" {
		t.Fatalf("content = %q, want the second write's content", got)
	}

	// No temp litter: the directory holds exactly the target file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "file.txt" {
		t.Fatalf("dir entries = %v, want exactly [file.txt]", entries)
	}
}

// TestWrite_Negative_UnwritableParent proves an unwritable destination
// directory is refused with a wrapped error, and that no temp file is left
// behind under the unwritable parent.
func TestWrite_Negative_UnwritableParent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("DISCLOSURE: running as root — os.Chmod(0o555) does not restrict root's own writes, so this permission-based negative test cannot exercise the unwritable-parent path under this user")
	}

	base := t.TempDir()
	blocked := filepath.Join(base, "blocked")
	if err := os.Mkdir(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) }) // let TempDir cleanup remove it

	path := filepath.Join(blocked, "file.txt")
	err := Write(path, []byte("nope"), 0o600)
	if err == nil {
		t.Fatal("expected an error writing under an unwritable directory")
	}
	if !strings.Contains(err.Error(), "atomicfile:") {
		t.Fatalf("error = %q, want it wrapped with an atomicfile: prefix", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("target file must not exist after a refused write; stat err = %v", statErr)
	}

	entries, err := os.ReadDir(blocked)
	if err != nil {
		// Directory itself may be unreadable depending on platform; that's fine,
		// the important assertion (no file created) already holds.
		return
	}
	for _, e := range entries {
		t.Fatalf("leftover entry under unwritable parent: %s", e.Name())
	}
}

// TestWrite_Negative_NoTempLeftOnFailure exercises a failure path AFTER the
// temp file has been created (the destination MkdirAll succeeds, but the
// final Rename target's directory disappears between temp-creation and
// rename), proving Write cleans up its temp file rather than leaving it
// behind. Since Write itself controls that whole sequence with no window
// for external interference in a single-threaded test, we instead prove
// the invariant the other tests already assert transitively: the failure
// case with an unwritable parent (above) never gets far enough to create a
// temp file at all, so there is nothing to leave behind. This test proves
// the complementary case — a destination that IS a directory, not a file,
// so CreateTemp succeeds (it's created under the parent, not overwriting
// path) but Rename fails because you cannot rename a file onto a
// non-empty... actually onto a directory.
func TestWrite_Negative_DestinationIsDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "iamadir")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	err := Write(path, []byte("data"), 0o600)
	if err == nil {
		t.Fatal("expected an error when the destination path is an existing directory")
	}
	if !strings.Contains(err.Error(), "atomicfile:") {
		t.Fatalf("error = %q, want it wrapped with an atomicfile: prefix", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "iamadir" {
			t.Fatalf("leftover temp file after failed rename: %s", e.Name())
		}
	}
}
