package ritualwitness

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalPath(t *testing.T) {
	real := t.TempDir()
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(resolved, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(resolved, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, base, in, want string
	}{
		{"an existing directory resolves its symlinks", "", real, resolved},
		{"a symlinked spelling resolves to the target", "", link, resolved},
		{"a missing tail keeps its spelling under the resolved ancestor", "", filepath.Join(link, "gone", "deeper"), filepath.Join(resolved, "gone", "deeper")},
		{"a relative path resolves against base", link, "sub", filepath.Join(resolved, "sub")},
		{"a climbing relative path resolves against base", filepath.Join(link, "sub"), "../wt", filepath.Join(resolved, "wt")},
		{"an unclean absolute path is cleaned", "", resolved + "/sub/../sub/", filepath.Join(resolved, "sub")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := canonicalPath(tt.base, tt.in); got != tt.want {
				t.Errorf("canonicalPath(%q, %q) = %q, want %q", tt.base, tt.in, got, tt.want)
			}
		})
	}
}

func TestWithin(t *testing.T) {
	tests := []struct {
		dir, path string
		want      bool
	}{
		{"/repo", "/repo", true},
		{"/repo", "/repo/a/b", true},
		{"/repo", "/repository", false},
		{"/repo", "/", false},
		{"/repo/a", "/repo", false},
		{"", "/repo", false},
	}
	for _, tt := range tests {
		if got := within(tt.dir, tt.path); got != tt.want {
			t.Errorf("within(%q, %q) = %v, want %v", tt.dir, tt.path, got, tt.want)
		}
	}
}

func TestStoreRelative(t *testing.T) {
	tests := []struct {
		prefix, in, want string
		ok               bool
	}{
		{"", "owned/x", "owned/x", true},
		{"store/", "store/owned/x", "owned/x", true},
		{"store/", "owned/x", "", false},
		{"store/", "storefront/x", "", false},
		{"store/", "store/", "", false},
	}
	for _, tt := range tests {
		got, ok := storeRelative(tt.prefix, tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("storeRelative(%q, %q) = (%q, %v), want (%q, %v)", tt.prefix, tt.in, got, ok, tt.want, tt.ok)
		}
	}
}
