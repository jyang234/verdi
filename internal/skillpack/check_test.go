package skillpack

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckFindings(t *testing.T) {
	root := t.TempDir()
	if _, err := Write(context.Background(), root, Hosts()); err != nil {
		t.Fatal(err)
	}
	// drift: an edit anywhere in the body
	p := filepath.Join(root, filepath.FromSlash(Path(HostClaude, "plan")))
	b, _ := os.ReadFile(p)
	if err := os.WriteFile(p, append(b, []byte("\nhand edit\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	// missing: a deleted codex skill
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(Path(HostCodex, "tasks")))); err != nil {
		t.Fatal(err)
	}
	// a changed render-commit line alone is NOT drift (R-W3-2)
	q := filepath.Join(root, filepath.FromSlash(Path(HostCodex, "specify")))
	c, _ := os.ReadFile(q)
	c = bytes.Replace(c, []byte("<!-- verdi:render-commit none -->"), []byte("<!-- verdi:render-commit 0123456789abcdef0123456789abcdef01234567 -->"), 1)
	if err := os.WriteFile(q, c, 0o644); err != nil {
		t.Fatal(err)
	}
	// truncated file counts as drift
	r := filepath.Join(root, filepath.FromSlash(Path(HostClaude, "tasks")))
	d, _ := os.ReadFile(r)
	if err := os.WriteFile(r, d[:len(d)/2], 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := Check(root, Hosts())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, f := range rep.Findings {
		got[f.Path] = f.Code
	}
	want := map[string]string{
		Path(HostClaude, "plan"):  "drift",
		Path(HostCodex, "tasks"):  "missing",
		Path(HostClaude, "tasks"): "drift",
	}
	if len(got) != len(want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	for p, code := range want {
		if got[p] != code {
			t.Fatalf("finding for %s = %q, want %q (all: %v)", p, got[p], code, got)
		}
	}
	for _, f := range rep.Findings {
		if f.Code == "drift" && (f.Expected == "" || f.Actual == "" || f.Expected == f.Actual) {
			t.Fatalf("drift finding must carry distinct expected/actual digests: %+v", f)
		}
	}
	// Findings are sorted by path.
	for i := 1; i < len(rep.Findings); i++ {
		if rep.Findings[i-1].Path >= rep.Findings[i].Path {
			t.Fatalf("findings not sorted: %v", rep.Findings)
		}
	}
	// Scoped to one host: the codex findings alone.
	rep2, _ := Check(root, []Host{HostCodex})
	if len(rep2.Findings) != 1 || rep2.Findings[0].Code != "missing" || rep2.Checked != 4 {
		t.Fatalf("Check(codex) = %+v", rep2)
	}
}
