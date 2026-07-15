package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// provisionStore builds a scratch store at storeRoot: examples/showcase's
// committed zone (.verdi/{specs,adr,diagrams,attestations,waivers,
// conflicts,verdi.yaml}) as one real git commit (a fresh, throwaway repo —
// not fixturegit's golden-SHA-pinned build, since nothing here asserts a
// specific commit hash; it only needs REAL git history for gitx's
// RevParse/CommitDate/AddAll/CreateCommit, which `verdi serve`'s backend
// and commit-to-design both exercise), plus examples/showcase's mutable/ and
// derived/ trees copied in UNTRACKED (VL-013: never git-add those).
//
// verdi.yaml is committed at examples/showcase/.verdi/verdi.yaml (not
// load-bearing for anything this suite drives, since
// storyresolve/evidence/commitdesign none of them require the manifest —
// kept for parity with a real store, and so a human poking at the scratch
// store with other verdi verbs sees a legible one) and arrives with the
// rest of the committed zone via the copyTree call below — this function no
// longer writes it itself.
func provisionStore(moduleRoot, storeRoot string) error {
	if err := os.MkdirAll(storeRoot, 0o755); err != nil {
		return err
	}

	corpusDir := filepath.Join(moduleRoot, "examples", "showcase")
	if err := copyTree(filepath.Join(corpusDir, ".verdi"), filepath.Join(storeRoot, ".verdi")); err != nil {
		return fmt.Errorf("copying committed zone: %w", err)
	}

	// examples/showcase's own "loansvc" service root: stale-decline/spec.md
	// declares `impacts: { ref: svc/loansvc/boundary-contract }`, which
	// needs a real, discoverable service root to resolve (VL-003) — carried
	// alongside the committed zone (not layers.txt-tracked: service
	// discovery reads the filesystem directly, never git, 01 §notes) so a
	// provisioned checkout of this store is lint-clean on this link, not
	// only the Go test suite's own synthetic fixture
	// (internal/lint/harness_test.go's writeLoansvcFixture).
	if err := copyTree(filepath.Join(corpusDir, "loansvc"), filepath.Join(storeRoot, "loansvc")); err != nil {
		return fmt.Errorf("copying loansvc service: %w", err)
	}

	// Fold in testdata/svcfix as a real service root (it carries a
	// .flowmap.yaml plus a .flowmap/boundary-contract.json), so the built
	// dex site gains a by-service axis and a boundary-contract permalink
	// whose JSON is pretty-printed through chroma — the one e2e page with a
	// highlighted code block, which the dark-mode syntax-highlighting check
	// (05-dex.spec) needs something real to assert against.
	if err := copyTree(filepath.Join(moduleRoot, "testdata", "svcfix"), filepath.Join(storeRoot, "svcfix")); err != nil {
		return fmt.Errorf("copying svcfix service: %w", err)
	}

	// The spec-stale living report for borrower-update-mobile and the
	// round-four archived quartet (formerly testdata/dexoverlay, folded
	// into examples/showcase/.verdi/ as layers.txt layer 4 — see
	// examples/showcase/OVERLAY-NOTES.md) are now part of the committed
	// zone copied above, so the dex site's story-page ladder badges and
	// by-story axis have their fixtures on MAIN without a separate copy
	// (the dex is main-only; the open-MR half of the ladder is seeded
	// through the fake forge in main.go, never written into the store).

	// A component spec whose markdown body carries a fenced ```mermaid block.
	// examples/showcase already provisions a diagram-KIND artifact
	// (diagrams/loansvc-topology.mermaid, copied whole above), so the e2e
	// store exercises both mermaid surfaces: the diagram kind and an inline
	// fence inside ordinary markdown. This one lives only in the throwaway
	// scratch store (not examples/showcase) so it perturbs no golden-SHA fixture
	// the Go tests pin.
	mermaidDemoDir := filepath.Join(storeRoot, ".verdi", "specs", "active", "mermaid-demo")
	if err := os.MkdirAll(mermaidDemoDir, 0o755); err != nil {
		return fmt.Errorf("creating mermaid-demo spec dir: %w", err)
	}
	if err := os.WriteFile(filepath.Join(mermaidDemoDir, "spec.md"), []byte(mermaidDemoSpec), 0o644); err != nil {
		return fmt.Errorf("writing mermaid-demo spec: %w", err)
	}

	// A class: proposal diagram (spec/illustrative-class ac-3's second
	// tier): its pages must carry the extractor-computed tier marker and
	// NEVER the illustrative badge, so the e2e store holds both tiers —
	// this proposal beside the corpus's incumbent loansvc-topology.mermaid
	// (illustrative by class). Scratch-store-only for the same golden-SHA
	// reason as mermaidDemoSpec above.
	if err := os.WriteFile(filepath.Join(storeRoot, ".verdi", "diagrams", "decline-flow-future.mermaid"), []byte(proposalDiagram), 0o644); err != nil {
		return fmt.Errorf("writing proposal diagram fixture: %w", err)
	}

	// The draft-boards same-spec fixture's LANDED half (spec/draft-boards
	// ac-3): a spec landed on main whose name also exists as a DRAFT
	// edition on its own design branch (provision_draftboards.go). Landed
	// here — before the corpus commit — so it is on main and on every
	// branch cut from main, including the serving checkout's. Scratch-only
	// like mermaid-demo (a name reused across main and a design branch
	// cannot live in examples/showcase without perturbing other suites).
	ledgerDir := filepath.Join(storeRoot, ".verdi", "specs", "active", dbSameSpecName)
	if err := os.MkdirAll(ledgerDir, 0o755); err != nil {
		return fmt.Errorf("creating %s spec dir: %w", dbSameSpecName, err)
	}
	if err := os.WriteFile(filepath.Join(ledgerDir, "spec.md"), []byte(dbSameSpecLanded), 0o644); err != nil {
		return fmt.Errorf("writing %s spec: %w", dbSameSpecName, err)
	}

	// verdi.yaml is no longer written here — it is now committed at
	// examples/showcase/.verdi/verdi.yaml (layers.txt layer 1) and arrives
	// via the copyTree call above.
	if err := os.WriteFile(filepath.Join(storeRoot, ".verdi", ".gitignore"), []byte("data/\n"), 0o644); err != nil {
		return err
	}
	// .gitattributes is likewise no longer synthesized here — it is a real,
	// committed file at examples/showcase/.gitattributes (task-1.8's own
	// lint-clean sweep: VL-012 found the repo-root plumbing file this
	// harness had always synthesized was never actually present in the
	// showcase tree itself, so a from-disk-only construction — no harness
	// step to paper over the gap — could never lint clean on VL-012).
	if err := copyFile(filepath.Join(corpusDir, ".gitattributes"), filepath.Join(storeRoot, ".gitattributes")); err != nil {
		return fmt.Errorf("copying .gitattributes: %w", err)
	}

	if err := gitInitAndCommit(storeRoot); err != nil {
		return fmt.Errorf("git init/commit: %w", err)
	}

	if err := copyTree(filepath.Join(corpusDir, "mutable"), filepath.Join(storeRoot, ".verdi", "data", "mutable")); err != nil {
		return fmt.Errorf("copying mutable zone: %w", err)
	}
	if err := copyTree(filepath.Join(corpusDir, "derived"), filepath.Join(storeRoot, ".verdi", "data", "derived")); err != nil {
		return fmt.Errorf("copying derived zone: %w", err)
	}

	return nil
}

// mermaidDemoSpec is a minimal component spec whose markdown body carries a
// fenced ```mermaid block — the e2e fixture for "an inline mermaid fence in
// ordinary markdown still renders client-side" (the diagram KIND is covered
// by the corpus's own loansvc-topology.mermaid).
const mermaidDemoSpec = "---\n" +
	"id: spec/mermaid-demo\n" +
	"kind: spec\n" +
	"class: component\n" +
	"title: \"Mermaid demo (e2e fixture)\"\n" +
	"status: active\n" +
	"owners: [platform-team]\n" +
	"---\n" +
	"# Mermaid demo\n\n" +
	"A fenced mermaid block inside a markdown body must still render:\n\n" +
	"```mermaid\n" +
	"graph TD\n" +
	"  a --> b\n" +
	"  b --> c\n" +
	"```\n"

// proposalDiagram is a class: proposal diagram fixture
// (spec/illustrative-class ac-3): a from-scratch proposal whose mermaid
// body sits entirely inside the verification extractor's declared grammar,
// so its rendered surfaces carry data-diagram-tier="full" (the
// extractor-computed vocabulary) and must never wear the illustrative
// badge (ac-2's negative case). Nothing here runs flowmap — the tier is
// grammar coverage, a pure function of these bytes.
const proposalDiagram = "---\n" +
	"id: diagram/decline-flow-future\n" +
	"kind: diagram\n" +
	"class: proposal\n" +
	"title: \"Decline flow, future state (e2e fixture)\"\n" +
	"status: proposed\n" +
	"owners: [platform-team]\n" +
	"---\n" +
	"graph TD\n" +
	"  decline --> audit\n" +
	"  audit --> notify\n"

func gitInitAndCommit(dir string) error {
	if err := runGit(dir, nil, "init", "--quiet", "--initial-branch=main"); err != nil {
		return err
	}
	if err := runGit(dir, nil, "add", "-A"); err != nil {
		return err
	}
	return runGit(dir, nil, "commit", "--quiet", "--no-verify", "-m", "e2e scratch store: seeded from examples/showcase")
}

// copyTree recursively copies every regular file under src to dst. A
// missing src is tolerated (some callers pass optional overlay trees); any
// other stat failure (e.g. permission denied) is a real error and returns
// wrapped, not silently swallowed alongside the absent case.
func copyTree(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", src, err)
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", target, err)
			}
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }() // read-only source; close error is unactionable
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	// Propagate the write-path Close: a swallowed close on a written file
	// can hide a short/truncated copy (previously deferred and dropped).
	return out.Close()
}
