package skillpack

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderStampsAndGolden(t *testing.T) {
	update := os.Getenv("SKILLPACK_UPDATE_GOLDEN") == "1"
	for _, h := range Hosts() {
		for _, s := range Skills() {
			r, err := Render(h, s, "0123456789abcdef0123456789abcdef01234567")
			if err != nil {
				t.Fatalf("Render(%s,%s): %v", h, s, err)
			}
			if r.Path != Path(h, s) || r.Digest != contentDigest(r.Content) {
				t.Fatalf("Render(%s,%s) path/digest mismatch", h, s)
			}
			body := string(r.Content)
			for _, want := range []string{
				"---\nname: verdi-" + s + "\n",
				"<!-- verdi:generated-skill host=" + string(h) + " skill=" + s + " -->\n",
				"<!-- verdi:engine-digest " + EngineDigest() + " -->\n",
				"<!-- verdi:render-commit 0123456789abcdef0123456789abcdef01234567 -->\n",
				"<!-- verdi: this is a generated skill;",
				"```verdi-sequence\n",
			} {
				if !strings.Contains(body, want) {
					t.Fatalf("Render(%s,%s) lacks %q:\n%s", h, s, want, body)
				}
			}
			td, _ := TemplateDigest(s)
			if !strings.Contains(body, "<!-- verdi:template-digest "+td+" -->\n") {
				t.Fatalf("Render(%s,%s) lacks its template digest", h, s)
			}
			golden := filepath.Join("testdata", "golden", string(h), "verdi-"+s+".md")
			if update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, r.Content, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("golden %s: %v (run with SKILLPACK_UPDATE_GOLDEN=1 to create)", golden, err)
			}
			if !bytes.Equal(want, r.Content) {
				t.Fatalf("Render(%s,%s) differs from golden %s", h, s, golden)
			}
		}
	}
}

func TestRenderHostsDifferOnlyInMarker(t *testing.T) {
	for _, s := range Skills() {
		a, _ := Render(HostClaude, s, "none")
		b, _ := Render(HostCodex, s, "none")
		na := strings.Replace(string(a.Content), "host=claude", "host=X", 1)
		nb := strings.Replace(string(b.Content), "host=codex", "host=X", 1)
		if na != nb {
			t.Fatalf("skill %s: hosts differ beyond the marker", s)
		}
	}
}

func TestRenderRefusesUnknown(t *testing.T) {
	if _, err := Render(HostClaude, "deploy", "none"); err == nil {
		t.Fatal("unknown skill must refuse")
	}
	if _, err := Render(Host("cursor"), "specify", "none"); err == nil {
		t.Fatal("unknown host must refuse")
	}
	if _, err := Render(HostClaude, "specify", "HEAD"); err == nil {
		t.Fatal("render commit must be 40 hex or none")
	}
}

func TestRenderCommit(t *testing.T) {
	plain := t.TempDir()
	if got := RenderCommit(context.Background(), plain); got != "none" {
		t.Fatalf("RenderCommit outside git = %q, want none", got)
	}
	repo := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "x"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if got := RenderCommit(context.Background(), repo); len(got) != 40 {
		t.Fatalf("RenderCommit inside git = %q, want 40 hex", got)
	}
}
