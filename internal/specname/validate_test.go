package specname

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/gitx"
	"github.com/jyang234/verdi/internal/store"
)

// TestValidateSuccessorName_Happy proves the two baseline preconditions
// pass for a fresh, well-formed name with no base ref given.
func TestValidateSuccessorName_Happy(t *testing.T) {
	root := t.TempDir()
	ref, err := ValidateSuccessorName(context.Background(), root, "lockbox-v2", "")
	if err != nil {
		t.Fatalf("ValidateSuccessorName = %v, want no error", err)
	}
	if ref.String() != "spec/lockbox-v2" {
		t.Fatalf("ref = %q, want spec/lockbox-v2", ref.String())
	}
}

// TestValidateSuccessorName_InvalidName is UAT-030's own table: every name
// shape ParseRef itself refuses (unchanged from before this fix), PLUS the
// two decorations ParseRef tolerates but a spec's own NAME must not carry —
// a "#fragment" suffix and an "@commit" pin — each refused as
// ReasonInvalidName with operator-facing wording, never blamed on the tool.
// Every case's Unwrap is non-nil (a plain "why" error, either the real
// ParseRef failure or, for the two new decoration cases, a same-shaped one
// this package synthesizes) so a caller can render every ReasonInvalidName
// refusal the same way: name the flag, then %v the unwrapped reason.
func TestValidateSuccessorName_InvalidName(t *testing.T) {
	cases := []struct{ name, why string }{
		{"", "empty"},
		{"Not_A_Valid_Name", "not kebab-case"},
		{"nested/name", "a path separator"},
		{"UPPER", "uppercase"},
		{"plainfrag#dc-1", "a #fragment suffix (UAT-030)"},
		{"pinned@abc1234", "an @commit pin (UAT-030)"},
	}
	for _, tc := range cases {
		t.Run("refuses a name that is "+tc.why, func(t *testing.T) {
			root := t.TempDir()
			_, err := ValidateSuccessorName(context.Background(), root, tc.name, "")
			var nerr *NameError
			if !errors.As(err, &nerr) {
				t.Fatalf("ValidateSuccessorName(%q) = %v, want a *NameError", tc.name, err)
			}
			if nerr.Reason != ReasonInvalidName {
				t.Fatalf("Reason = %q, want %q", nerr.Reason, ReasonInvalidName)
			}
			if nerr.Name != tc.name {
				t.Fatalf("Name = %q, want %q", nerr.Name, tc.name)
			}
			if errors.Unwrap(nerr) == nil {
				t.Fatal("NameError wraps no \"why\" error; the caller cannot report WHY the name was refused")
			}
			if got := nerr.Error(); !containsAll(got, "not a valid spec name") {
				t.Fatalf("Detail = %q, want it to name the refusal in operator-facing terms", got)
			}
			// UAT-030's own witness: the message must never blame the tool.
			if containsAll(nerr.Error(), "internal error") {
				t.Fatalf("Detail = %q, want no \"internal error\" wording for operator input", nerr.Error())
			}
		})
	}
}

// TestValidateSuccessorName_FragmentAndPinnedBothSet proves the combined
// decoration is refused too, with a message naming both. The canonical ref
// grammar (artifact.Ref.String(): "<kind>/<name>@<commit>#<object>") puts
// the pin BEFORE the fragment, so that is the only string shape that
// parses as carrying both at once ("name#frag@sha" instead parses as a
// PINNED-looking suffix that is really part of the fragment's own object
// id, and is refused earlier, as an invalid fragment id — a different
// table entry's concern, not this one's).
func TestValidateSuccessorName_FragmentAndPinnedBothSet(t *testing.T) {
	root := t.TempDir()
	_, err := ValidateSuccessorName(context.Background(), root, "combo@abc1234#dc-1", "")
	var nerr *NameError
	if !errors.As(err, &nerr) || nerr.Reason != ReasonInvalidName {
		t.Fatalf("ValidateSuccessorName = %v, want a *NameError with reason %q", err, ReasonInvalidName)
	}
	if errors.Unwrap(nerr) == nil {
		t.Fatal("NameError wraps no \"why\" error; the caller cannot report WHY the name was refused")
	}
	if !containsAll(nerr.Error(), "fragment") || !containsAll(nerr.Error(), "commit") {
		t.Fatalf("Detail = %q, want it to name both decorations", nerr.Error())
	}
}

// TestValidateSuccessorName_ActiveExists proves the active-zone collision —
// a directory or, historically, an existing plain FILE at that path —
// refuses as ReasonSuccessorExists, naming the colliding path.
func TestValidateSuccessorName_ActiveExists(t *testing.T) {
	t.Run("existing directory", func(t *testing.T) {
		root := t.TempDir()
		dir := store.ActiveSpecDir(root, "lockbox-v2")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		_, err := ValidateSuccessorName(context.Background(), root, "lockbox-v2", "")
		var nerr *NameError
		if !errors.As(err, &nerr) {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError", err)
		}
		if nerr.Reason != ReasonSuccessorExists {
			t.Fatalf("Reason = %q, want %q", nerr.Reason, ReasonSuccessorExists)
		}
		if nerr.Path != dir {
			t.Fatalf("Path = %q, want %q", nerr.Path, dir)
		}
	})

	t.Run("existing plain file", func(t *testing.T) {
		root := t.TempDir()
		dir := store.ActiveSpecDir(root, "lockbox-v2")
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(dir, []byte("not a directory"), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		_, err := ValidateSuccessorName(context.Background(), root, "lockbox-v2", "")
		var nerr *NameError
		if !errors.As(err, &nerr) || nerr.Reason != ReasonSuccessorExists {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError with reason %q", err, ReasonSuccessorExists)
		}
	})
}

// TestValidateSuccessorName_ArchivedExists is UAT-032's own witness: a name
// already used by an ARCHIVED spec refuses even though the active zone is
// clear, naming the guide-6.1 rule.
func TestValidateSuccessorName_ArchivedExists(t *testing.T) {
	root := t.TempDir()
	dir := store.ArchiveSpecDir(root, "retired")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	_, err := ValidateSuccessorName(context.Background(), root, "retired", "")
	var nerr *NameError
	if !errors.As(err, &nerr) {
		t.Fatalf("ValidateSuccessorName = %v, want a *NameError", err)
	}
	if nerr.Reason != ReasonArchivedExists {
		t.Fatalf("Reason = %q, want %q", nerr.Reason, ReasonArchivedExists)
	}
	if nerr.Path != dir {
		t.Fatalf("Path = %q, want %q", nerr.Path, dir)
	}
	if !containsAll(nerr.Error(), "guide 6.1") {
		t.Fatalf("Detail = %q, want it to cite guide 6.1 (names unique across active and archived specs)", nerr.Error())
	}
}

// TestValidateSuccessorName_ActiveWinsOverArchive pins the check ORDER when
// a name (pathologically) collides in both zones at once: the active-zone
// reason surfaces, matching the order the working-tree checks have always
// run in (active before archive).
func TestValidateSuccessorName_ActiveWinsOverArchive(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(store.ActiveSpecDir(root, "dup"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(store.ArchiveSpecDir(root, "dup"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := ValidateSuccessorName(context.Background(), root, "dup", "")
	var nerr *NameError
	if !errors.As(err, &nerr) || nerr.Reason != ReasonSuccessorExists {
		t.Fatalf("ValidateSuccessorName = %v, want ReasonSuccessorExists (active wins)", err)
	}
}

// buildBehindCheckoutFixture builds a fixturegit repo with two branches:
// "main" carries activeName/archiveName (whichever is non-empty) landed on
// a SECOND commit; "behind" was cut BEFORE that second commit and is left
// checked out — the UAT-031 reproduction shape (a serving checkout behind
// the resolved default branch). Returns the repo positioned on "behind".
func buildBehindCheckoutFixture(t *testing.T, activeName, archiveName string) *fixturegit.Repo {
	t.Helper()
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files:   map[string]string{".verdi/verdi.yaml": "schema: verdi.layout/v1\n"},
		Message: "seed store",
	}})
	if err := gitx.CheckoutNewBranch(ctx, repo.Dir, "behind"); err != nil {
		t.Fatalf("CheckoutNewBranch(behind): %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "main"); err != nil {
		t.Fatalf("CheckoutExisting(main): %v", err)
	}
	files := map[string]string{}
	if activeName != "" {
		files[store.ActiveSpecRelPath(activeName)] = "landed on main only\n"
	}
	if archiveName != "" {
		files[store.SpecRelPath(store.ZoneArchive, archiveName)] = "landed on main only\n"
	}
	for rel, content := range files {
		full := filepath.Join(repo.Dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := gitx.AddAll(ctx, repo.Dir); err != nil {
		t.Fatalf("AddAll: %v", err)
	}
	if _, err := gitx.CreateCommit(ctx, repo.Dir, "land on main only"); err != nil {
		t.Fatalf("CreateCommit: %v", err)
	}
	if err := gitx.CheckoutExisting(ctx, repo.Dir, "behind"); err != nil {
		t.Fatalf("CheckoutExisting(behind): %v", err)
	}
	return repo
}

// TestValidateSuccessorName_ExistsOnBase is UAT-031's own witness: a name
// present on the resolved base ref but absent from the CURRENT checkout's
// working tree (a checkout behind that ref) still refuses — proven for
// both the active and the archive zone landing on the base.
func TestValidateSuccessorName_ExistsOnBase(t *testing.T) {
	t.Run("active zone on base", func(t *testing.T) {
		repo := buildBehindCheckoutFixture(t, "taken-on-main", "")
		if _, err := os.Stat(store.ActiveSpecDir(repo.Dir, "taken-on-main")); !os.IsNotExist(err) {
			t.Fatalf("test setup: %s must be ABSENT from the behind checkout's working tree", store.ActiveSpecDir(repo.Dir, "taken-on-main"))
		}
		_, err := ValidateSuccessorName(context.Background(), repo.Dir, "taken-on-main", "main")
		var nerr *NameError
		if !errors.As(err, &nerr) {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError", err)
		}
		if nerr.Reason != ReasonExistsOnBase {
			t.Fatalf("Reason = %q, want %q", nerr.Reason, ReasonExistsOnBase)
		}
		if !containsAll(nerr.Error(), "main") || !containsAll(nerr.Error(), "behind") {
			t.Fatalf("Detail = %q, want it to name the base ref and say the checkout is behind it", nerr.Error())
		}
	})

	t.Run("archive zone on base", func(t *testing.T) {
		repo := buildBehindCheckoutFixture(t, "", "retired-on-main")
		_, err := ValidateSuccessorName(context.Background(), repo.Dir, "retired-on-main", "main")
		var nerr *NameError
		if !errors.As(err, &nerr) || nerr.Reason != ReasonExistsOnBase {
			t.Fatalf("ValidateSuccessorName = %v, want a *NameError with reason %q", err, ReasonExistsOnBase)
		}
	})

	t.Run("absent from base too: proceeds", func(t *testing.T) {
		repo := buildBehindCheckoutFixture(t, "unrelated-name", "")
		ref, err := ValidateSuccessorName(context.Background(), repo.Dir, "genuinely-fresh", "main")
		if err != nil {
			t.Fatalf("ValidateSuccessorName = %v, want no error for a name absent everywhere", err)
		}
		if ref.String() != "spec/genuinely-fresh" {
			t.Fatalf("ref = %q, want spec/genuinely-fresh", ref.String())
		}
	})

	t.Run("empty baseRef skips the check even when the name would collide on main", func(t *testing.T) {
		repo := buildBehindCheckoutFixture(t, "taken-on-main", "")
		ref, err := ValidateSuccessorName(context.Background(), repo.Dir, "taken-on-main", "")
		if err != nil {
			t.Fatalf("ValidateSuccessorName with baseRef=\"\" = %v, want no error (base-ref check opted out)", err)
		}
		if ref.String() != "spec/taken-on-main" {
			t.Fatalf("ref = %q, want spec/taken-on-main", ref.String())
		}
	})
}

// TestExistsOnBaseDetail is F1's own table (the wave-3 review's fix
// round on this lane): both wordings ExistsOnBaseDetail can render,
// side by side. The non-HEAD case is asserted byte-for-byte identical to
// the wording every caller used before F1 (the review's own instruction:
// "leave the non-HEAD text byte-identical").
func TestExistsOnBaseDetail(t *testing.T) {
	cases := []struct {
		name, baseRef, want string
	}{
		{
			name: "taken", baseRef: "main",
			want: `spec/taken already exists on main — this checkout is behind main; fetch/pull before starting a new spec of this name`,
		},
		{
			name: "taken", baseRef: "origin/main",
			want: `spec/taken already exists on origin/main — this checkout is behind origin/main; fetch/pull before starting a new spec of this name`,
		},
		{
			// dc-7's disclosed no-origin fallback: baseRef IS the calling
			// checkout's own current HEAD, so "behind HEAD" is backwards
			// (the checkout is AT HEAD by definition) and there is no
			// remote to fetch/pull from.
			name: "taken", baseRef: "HEAD",
			want: `spec/taken already exists at this branch's base (HEAD) though it is absent from the working tree; commit or restore it before reusing this name`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.baseRef, func(t *testing.T) {
			got := ExistsOnBaseDetail(tc.name, tc.baseRef)
			if got != tc.want {
				t.Fatalf("ExistsOnBaseDetail(%q, %q) = %q, want %q", tc.name, tc.baseRef, got, tc.want)
			}
			if tc.baseRef != "HEAD" && (containsAll(got, "HEAD") || containsAll(got, "restore")) {
				t.Fatalf("ExistsOnBaseDetail(%q, %q) = %q leaks HEAD-fallback wording into a real-ref case", tc.name, tc.baseRef, got)
			}
		})
	}
}

// TestValidateSuccessorName_ExistsOnBase_HeadFallback is F1's end-to-end
// witness, reproducing the reviewer's exact proof scenario: a fresh,
// remote-less store (dc-7's disclosed HEAD fallback — no "origin" remote
// configured at all) where a spec is committed, then its working-tree
// directory is deleted WITHOUT committing that deletion. The two
// working-tree checks (store.Active/ArchiveSpecDir stats) find nothing —
// the name looks free — but it is still exactly as taken as it was, one
// commit ago, at this branch's own base (HEAD). Before F1 this refused
// with "already exists on HEAD — this checkout is behind HEAD; fetch/pull
// …" — true nowhere (the checkout IS at HEAD; there is no remote at all)
// and actively misleading (there is nothing to fetch or pull).
func TestValidateSuccessorName_ExistsOnBase_HeadFallback(t *testing.T) {
	ctx := context.Background()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			store.ActiveSpecRelPath("deleted-locally"): "committed, then deleted from the working tree\n",
			".verdi/verdi.yaml":                        "schema: verdi.layout/v1\n",
		},
		Message: "seed store with deleted-locally",
	}})
	// No "origin" remote at all — dc-7's own precondition for baseRef to
	// resolve to the literal string "HEAD" (resolveDesignStartBase /
	// stubinstantiate.ResolveDesignBranchBase's own disclosed fallback);
	// this test drives ValidateSuccessorName directly with that same
	// literal, exactly as every one of the four callers would pass it.
	if err := os.RemoveAll(filepath.Join(repo.Dir, ".verdi", "specs", "active", "deleted-locally")); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo.Dir, ".verdi", "specs", "active", "deleted-locally")); !os.IsNotExist(err) {
		t.Fatalf("test setup: deleted-locally must be ABSENT from the working tree")
	}

	_, err := ValidateSuccessorName(ctx, repo.Dir, "deleted-locally", "HEAD")
	var nerr *NameError
	if !errors.As(err, &nerr) || nerr.Reason != ReasonExistsOnBase {
		t.Fatalf("ValidateSuccessorName = %v, want a *NameError with reason %q", err, ReasonExistsOnBase)
	}
	want := "specname: spec/deleted-locally already exists at this branch's base (HEAD) though it is absent from the working tree; commit or restore it before reusing this name"
	if got := nerr.Error(); got != want {
		t.Fatalf("Detail = %q, want %q", got, want)
	}
	if containsAll(nerr.Error(), "behind HEAD") || containsAll(nerr.Error(), "fetch") {
		t.Fatalf("Detail = %q, still carries the wrong-advice wording F1 closes", nerr.Error())
	}
}

// TestValidateSuccessorName_BaseRefOperationalErrorFailsClosed (F4,
// optional half, the wave-3 review's own fix round on this lane) proves
// the base-ref probe's OTHER failure shape: gitx.BlobAt returning a real
// operational error (as opposed to found=true/false) propagates as a
// plain wrapped Go error, never silently swallowed or misreported as a
// *NameError refusal. root is a plain t.TempDir() with no git repository
// at all — the simplest reproduction of BlobAt's own "gitx: BlobAt(...)"
// wrapped failure (a real `git ls-tree` invocation against a directory
// that is not a repository), reached only after the two working-tree
// checks (both vacuously pass: nothing exists in an empty temp dir).
func TestValidateSuccessorName_BaseRefOperationalErrorFailsClosed(t *testing.T) {
	root := t.TempDir()
	_, err := ValidateSuccessorName(context.Background(), root, "fresh-name", "main")
	if err == nil {
		t.Fatal("ValidateSuccessorName = nil error, want BlobAt's operational failure propagated")
	}
	var nerr *NameError
	if errors.As(err, &nerr) {
		t.Fatalf("ValidateSuccessorName = %v (*NameError, reason %q), want a plain operational error — a git-plumbing failure is not a naming refusal", err, nerr.Reason)
	}
	if !containsAll(err.Error(), "specname:") || !containsAll(err.Error(), "main") {
		t.Fatalf("error = %q, want it to name the base ref it was checking", err.Error())
	}
	if !containsAll(err.Error(), "gitx: BlobAt") {
		t.Fatalf("error = %q, want gitx.BlobAt's own wrapped context preserved (%%w)", err.Error())
	}
}

// containsAll is strings.Contains under this file's own established name
// (mirrors cmd/verdi's own test-local "contains" helper convention).
func containsAll(s, substr string) bool {
	return strings.Contains(s, substr)
}
