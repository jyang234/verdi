package dex

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// readFile reads outDir/relPath as a string, failing the test if it does
// not exist.
func readFile(t *testing.T, outDir, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(relPath)))
	if err != nil {
		t.Fatalf("reading %s: %v", relPath, err)
	}
	return string(data)
}

func fileExists(outDir, relPath string) bool {
	_, err := os.Stat(filepath.Join(outDir, filepath.FromSlash(relPath)))
	return err == nil
}

func TestBuild_Happy(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	outDir := t.TempDir()

	ctx := context.Background()
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: outDir}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	t.Run("home page exists", func(t *testing.T) {
		if !fileExists(outDir, "index.html") {
			t.Fatal("index.html was not written")
		}
	})

	t.Run("frozen temporal banner", func(t *testing.T) {
		// spec/stale-decline is frozen at layer-1's commit (c5e360a9...).
		page := readFile(t, outDir, "a/spec/stale-decline/index.html")
		want := "point-in-time record · frozen 2026-05-14 @ c5e360a"
		if !strings.Contains(page, want) {
			t.Fatalf("spec/stale-decline page missing frozen banner %q; got:\n%s", want, page)
		}
	})

	t.Run("authored-living temporal banner", func(t *testing.T) {
		// spec/store-layout-notes is an active component spec, never frozen.
		page := readFile(t, outDir, "a/spec/store-layout-notes/index.html")
		if !strings.Contains(page, "last-modified") {
			t.Fatalf("spec/store-layout-notes page missing 'last-modified' banner; got:\n%s", page)
		}
	})

	t.Run("living-gated temporal banner on external ref", func(t *testing.T) {
		page := readFile(t, outDir, "a/svc/svcfix/boundary-contract/index.html")
		if !strings.Contains(page, "main @ ") {
			t.Fatalf("svc/svcfix/boundary-contract page missing 'main @ ' living-gated banner; got:\n%s", page)
		}
	})

	t.Run("living-gated banner on listing pages too", func(t *testing.T) {
		page := readFile(t, outDir, "by-kind/index.html")
		if !strings.Contains(page, "main @ ") {
			t.Fatalf("by-kind hub page missing living-gated banner; got:\n%s", page)
		}
	})

	t.Run("backlink on a linked-to page", func(t *testing.T) {
		// adr/0002-outbox-events carries `{ type: supersedes, ref: adr/0001-outbox-events }`;
		// adr/0001's page must show the computed inverse "superseded-by".
		page := readFile(t, outDir, "a/adr/0001-outbox-events/index.html")
		if !strings.Contains(page, "superseded-by") {
			t.Fatalf("adr/0001-outbox-events page missing a 'superseded-by' backlink; got:\n%s", page)
		}
		if !strings.Contains(page, "adr/0002-outbox-events") {
			t.Fatalf("adr/0001-outbox-events page missing the backlink source ref adr/0002-outbox-events; got:\n%s", page)
		}
	})

	t.Run("permalink pages exist for verdi and svc refs", func(t *testing.T) {
		for _, rel := range []string{
			"a/spec/stale-decline/index.html",
			"a/adr/0001-outbox-events/index.html",
			"a/diagram/loansvc-topology/index.html",
			"a/svc/svcfix/boundary-contract/index.html",
			"a/svc/svcfix/api/index.html",
			"a/svc/svcfix/obligations/audit-before-publish/index.html",
		} {
			if !fileExists(outDir, rel) {
				t.Errorf("expected permalink page %s to exist", rel)
			}
		}
	})

	t.Run("openapi.json written alongside the API permalink page", func(t *testing.T) {
		if !fileExists(outDir, "a/svc/svcfix/api/openapi.json") {
			t.Fatal("a/svc/svcfix/api/openapi.json was not written")
		}
		var doc map[string]interface{}
		if err := json.Unmarshal([]byte(readFile(t, outDir, "a/svc/svcfix/api/openapi.json")), &doc); err != nil {
			t.Fatalf("openapi.json is not valid JSON: %v", err)
		}
		if doc["openapi"] == nil {
			t.Fatalf("openapi.json missing top-level 'openapi' key: %v", doc)
		}
	})

	t.Run("search index contains a known term and its posting", func(t *testing.T) {
		raw := readFile(t, outDir, "search-index.json")
		var doc searchIndexDoc
		if err := json.Unmarshal([]byte(raw), &doc); err != nil {
			t.Fatalf("search-index.json is not valid JSON: %v", err)
		}
		postings, ok := doc.Tokens["outbox"]
		if !ok || len(postings) == 0 {
			t.Fatalf(`search-index.json missing postings for "outbox"; tokens present: %d`, len(doc.Tokens))
		}
		found := false
		for _, p := range postings {
			if p.Ref == "adr/0001-outbox-events" {
				found = true
			}
		}
		if !found {
			t.Fatalf(`"outbox" postings %+v do not include adr/0001-outbox-events`, postings)
		}
	})

	t.Run("changelog lists the fixture's commits", func(t *testing.T) {
		page := readFile(t, outDir, "changelog/index.html")
		for _, msg := range []string{"layer 1", "layer 2", "layer 3"} {
			if !strings.Contains(page, msg) {
				t.Errorf("changelog missing commit message %q; got:\n%s", msg, page)
			}
		}
		// "add svcfix service" only touched svcfix/, not .verdi/, so the
		// .verdi/-scoped changelog must NOT list it — proving the log is
		// actually path-scoped, not just "every commit".
		if strings.Contains(page, "add svcfix service") {
			t.Error("changelog must not list a commit that never touched .verdi/")
		}
	})

	t.Run("dispositions table rendered on the feature-spec page", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/stale-decline/index.html")
		if !strings.Contains(page, "dispositions-table") {
			t.Fatal("spec/stale-decline page missing the I-5 dispositions table")
		}
		if !strings.Contains(page, "a-01J8Z0K3AAAAAAAAAAAAAAAAAA") {
			t.Fatal("dispositions table missing an expected sticky id")
		}
		if !strings.Contains(page, "incorporated") || !strings.Contains(page, "contradicted") || !strings.Contains(page, "open-question") {
			t.Fatal("dispositions table missing one of the three disposition values")
		}
	})

	t.Run("dispositions table absent on a non-feature-spec page", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/store-layout-notes/index.html")
		if strings.Contains(page, "dispositions-table") {
			t.Fatal("a component spec page must not render a dispositions table")
		}
	})

	t.Run("JS file count is exactly 3", func(t *testing.T) {
		count := 0
		var found []string
		err := filepath.Walk(outDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && strings.HasSuffix(path, ".js") {
				count++
				rel, _ := filepath.Rel(outDir, path)
				found = append(found, rel)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking outDir: %v", err)
		}
		if count != 3 {
			t.Fatalf("found %d .js files, want exactly 3: %v", count, found)
		}
	})

	t.Run("dependency mini-map renders on a service page", func(t *testing.T) {
		// spec/stale-decline declares `{ from: loansvc, to: notification-svc, via: events }`,
		// which does not name svcfix, so svcfix's own dependency map is
		// legitimately empty — assert the section still renders (never
		// silently omitted).
		page := readFile(t, outDir, "by-service/svcfix/index.html")
		if !strings.Contains(page, "Dependency mini-map") {
			t.Fatal("by-service page missing the Dependency mini-map section")
		}
	})

	t.Run("copy-reference button carries the pinned form", func(t *testing.T) {
		page := readFile(t, outDir, "a/spec/stale-decline/index.html")
		if !strings.Contains(page, `data-copy-ref="spec/stale-decline@c5e360a9ee5e9eb6089e54b772fa16959ada4662"`) {
			t.Fatalf("copy-reference button missing the expected pinned form; got:\n%s", page)
		}
	})
}

func TestBuild_Negative_EmptyRoot(t *testing.T) {
	if err := Build(context.Background(), Options{OutDir: t.TempDir()}); err == nil {
		t.Fatal("Build: expected an error for an empty Root, got nil")
	}
}

func TestBuild_Negative_EmptyOutDir(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	if err := Build(context.Background(), Options{Root: repo.Dir}); err == nil {
		t.Fatal("Build: expected an error for an empty OutDir, got nil")
	}
}

func TestBuild_Negative_UnknownCommit(t *testing.T) {
	repo := buildDexFixtureRepo(t)
	err := Build(context.Background(), Options{Root: repo.Dir, OutDir: t.TempDir(), Commit: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"})
	if err == nil {
		t.Fatal("Build: expected an error for an unresolvable commit, got nil")
	}
}

func TestBuild_Negative_RootNotAStore(t *testing.T) {
	// A directory with no .verdi/ at all: index.Build should fail
	// (there is nothing to walk), and Build must surface that error rather
	// than silently emitting an empty site.
	if err := Build(context.Background(), Options{Root: t.TempDir(), OutDir: t.TempDir()}); err == nil {
		t.Fatal("Build: expected an error for a root with no .verdi/, got nil")
	}
}

// TestBuild_ByteIdenticalRebuild proves Phase 12's central determinism
// requirement (constitution 1 / PLAN.md test strategy: "build twice,
// assert byte-identical output") — the site is a pure function of the
// tree at a given commit, never time.Now() or map-iteration order.
func TestBuild_ByteIdenticalRebuild(t *testing.T) {
	repo := buildDexFixtureRepo(t)

	out1 := t.TempDir()
	out2 := t.TempDir()

	ctx := context.Background()
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: out1}); err != nil {
		t.Fatalf("first Build: %v", err)
	}
	if err := Build(ctx, Options{Root: repo.Dir, OutDir: out2}); err != nil {
		t.Fatalf("second Build: %v", err)
	}

	h1, n1, err := hashTree(out1)
	if err != nil {
		t.Fatalf("hashing out1: %v", err)
	}
	h2, n2, err := hashTree(out2)
	if err != nil {
		t.Fatalf("hashing out2: %v", err)
	}
	if n1 == 0 {
		t.Fatal("hashTree walked zero files — test would be vacuous")
	}
	if n1 != n2 {
		t.Fatalf("file count differs across rebuilds: %d vs %d", n1, n2)
	}
	if h1 != h2 {
		t.Fatalf("rebuild is not byte-identical: %s vs %s", h1, h2)
	}
}

// hashTree walks dir and returns a single sha256 digest over every file's
// (relative path, content) pair, sorted by path — a whole-tree content
// hash independent of filesystem walk order.
func hashTree(dir string) (digest string, fileCount int, err error) {
	type entry struct {
		path string
		data []byte
	}
	var entries []entry
	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{path: filepath.ToSlash(rel), data: data})
		return nil
	})
	if walkErr != nil {
		return "", 0, walkErr
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	h := sha256.New()
	for _, e := range entries {
		_, _ = fmt.Fprintf(h, "%s\x00", e.path) // hash.Hash.Write never fails
		h.Write(e.data)
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil)), len(entries), nil
}
