package workbench

import (
	"embed"
	"net/http"

	"github.com/jyang234/verdi/internal/dex"
)

//go:embed assets/index.js assets/board.js assets/boardspec.js assets/boardspecasd.js assets/boarddiagram.js assets/readiness.js assets/specimport.js assets/specdocument.js assets/documentpage.js assets/topbar.js assets/wallselect.js assets/walltoolbar.js assets/wallkeys.js assets/wallminimap.js assets/wallnewstory.js assets/wallstrip.js assets/walldrawer.js
//go:embed assets/newstorydialog.js
var embeddedAssets embed.FS

// mermaidHandler serves dex's vendored mermaid.min.js (05 §Workbench:
// "mermaid client-side reusing the dex's vendored asset") — the same
// bytes, not a second copy.
func mermaidHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := dex.MermaidJS()
		if err != nil {
			http.Error(w, "mermaid.min.js unavailable: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(data) // response body write; post-header error is unactionable
	}
}

// styleCSSHandler serves internal/dex's composed stylesheet, chroma
// light/dark palettes and all (dex.StyleCSS) — the docs site's copy is the
// same composition with the workbench-only blocks stripped (SI-322), so
// these bytes keep those blocks — so the workbench's shared class-based
// code rendering is coloured and equally dark-mode-correct without owning
// a second stylesheet (the same one-copy-two-surfaces pattern as the
// vendored mermaid.min.js).
func styleCSSHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := dex.StyleCSS()
		if err != nil {
			http.Error(w, "style.css unavailable: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = w.Write(data) // response body write; post-header error is unactionable
	}
}

// boardJSHandler serves the v0 board page's one JS file (05 §Workbench:
// "keep board JS minimal and in ONE file").
func boardJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/board.js")
}

// boardSpecJSHandler serves the v1 board's one JS file — same minimal,
// dependency-free posture as the v0 board's.
func boardSpecJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/boardspec.js")
}

// boardSpecASDJSHandler serves the ASD workbench's route-scoped asset
// (Wave 6 Task 2): conditional refresh, typed mutation transport, and
// panel wiring — dependency-free, and structurally capped at 64 KiB
// uncompressed (SI-168; TestBoardSpecASDAssetBudget).
func boardSpecASDJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/boardspecasd.js")
}

// specDocumentJSHandler serves the board Document tab's one JS file
// (spec/spec-documents): conditional refresh and copy only, dependency-
// free, and structurally capped at 64 KiB uncompressed like the ASD
// asset (TestBoardDocumentAssetBudget).
func specDocumentJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/specdocument.js")
}

// documentPageJSHandler serves the Document page's chrome script
// (spec/document-page-v2): the refreshed time, the id chips, and the
// chrome's refresh on a poll — a new asset for the new behaviour
// (spec/workbench-redesign co-1), dependency-free, and structurally
// capped at 64 KiB uncompressed (TestDocumentPageAssetBudget).
func documentPageJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/documentpage.js")
}

// boardDiagramJSHandler serves the diagram editor's one JS file
// (spec/board-editor) — same minimal, dependency-free posture.
func boardDiagramJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/boarddiagram.js")
}

// readinessJSHandler serves the readiness pilot cockpit's one JS file —
// page-memory instrumentation only, same minimal, dependency-free
// posture as the board scripts.
func readinessJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/readiness.js")
}

// topBarJSHandler serves the top bar's one JS file
// (spec/chrome-and-tokens-v2), referenced from every workbench page —
// dependency-free, and structurally capped at 64 KiB uncompressed
// (spec/workbench-redesign co-1; TestTopBarAsset_ServedWithinBudget).
func topBarJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/topbar.js")
}

// wallSelectJSHandler serves the wall's selection script
// (spec/wall-canvas-v2 ac-1, ac-2, co-2): selecting a card or a thread,
// the emphasis and the recede, the yarn overlay above the cards, the
// status pill, and arrival on `#obj-<id>` — a new asset for the new
// behaviour (co-1: boardspec.js does not grow), dependency-free, and
// structurally capped at 64 KiB uncompressed
// (TestWallSelectAsset_ServedWithinBudget).
func wallSelectJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/wallselect.js")
}

// wallToolbarJSHandler serves the wall's contextual toolbar script
// (spec/wall-canvas-v2 ac-3, ac-4, ac-5, co-2): the toolbar built on the
// selection, drag-to-thread's target highlight and picker placement, the
// add-in-place slots, and the card editor's Enter and Escape — a new
// asset for the new behaviour (co-1: boardspec.js does not grow),
// dependency-free, and structurally capped at 64 KiB uncompressed
// (TestWallToolbarAsset_ServedWithinBudget).
func wallToolbarJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/walltoolbar.js")
}

// wallKeysJSHandler serves the wall's keyboard script (spec/wall-canvas-v2
// ac-6): the arrows that move the selection and reveal the card, Enter on
// a focused card, Delete through the existing confirmation and the stub
// refusal, and the Escape that clears the selection last — a new asset for
// the new behaviour (co-1: boardspec.js does not grow), dependency-free,
// and structurally capped at 64 KiB uncompressed
// (TestWallKeysAssets_ServedWithinBudget).
func wallKeysJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/wallkeys.js")
}

// wallMinimapJSHandler serves the wall's minimap script (spec/wall-canvas-v2
// ac-6): every card and the canvas's viewport drawn into the status row's
// host, and the viewport moved by dragging the frame — a new asset for the
// new behaviour (co-1), dependency-free, and structurally capped at 64 KiB
// uncompressed (TestWallKeysAssets_ServedWithinBudget).
func wallMinimapJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/wallminimap.js")
}

// wallStripJSHandler serves the wall's strip script
// (spec/wall-strip-and-drawer-v2 ac-1 to ac-4; lane F3a): the case-file
// strip, Commit and push's changes popover, and the branch menu — a new
// asset for the new behaviour (parent co-1: boardspec.js does not grow),
// dependency-free, and structurally capped at 64 KiB uncompressed
// (TestWallStripDrawerAssets_ServedWithinBudget).
func wallStripJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/wallstrip.js")
}

// wallDrawerJSHandler serves the wall's record drawer script
// (spec/wall-strip-and-drawer-v2 ac-4, ac-5; lane F3b): the drawer, its
// tabs, and the menu that opens them — a new asset for the new behaviour
// (parent co-1), dependency-free, and structurally capped at 64 KiB
// uncompressed (TestWallStripDrawerAssets_ServedWithinBudget).
func wallDrawerJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/walldrawer.js")
}

func embeddedJSHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := embeddedAssets.ReadFile(name)
		if err != nil {
			http.Error(w, name+" unavailable: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write(data) // response body write; post-header error is unactionable
	}
}

// indexJSHandler serves the index's one JS file (spec/index-v2 ac-3): the
// filter row over the server-rendered cards — a new asset for the new
// behaviour (spec/workbench-redesign co-1), dependency-free, and
// structurally capped at 64 KiB uncompressed
// (TestIndexAsset_ServedWithinBudget).
func indexJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/index.js")
}

// wallNewStoryJSHandler serves the wall's opener for the index's call to
// action (spec/index-v2 ac-4; SI-366 (11)): on `?new-story=<ac>` it opens
// the existing create dialog through its button and checks the named
// criterion — a small asset for the new behaviour (co-1: boardspec.js
// does not grow), dependency-free, and structurally capped at 2 KiB
// (TestWallNewStoryAsset_ServedWithinBudget).
func wallNewStoryJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/wallnewstory.js")
}

// newStoryDialogJSHandler serves the New story dialog's own script
// (spec/new-story-dialog-v2; SI-369 (9)): the branch preview, the inline
// grammar report, the gated Create and the criterion rows — a new asset
// for the new behaviour (parent co-1: boardspec.js does not grow),
// dependency-free, and structurally capped at 64 KiB uncompressed
// (TestNewStoryDialogAsset_ServedWithinBudget).
func newStoryDialogJSHandler() http.HandlerFunc {
	return embeddedJSHandler("assets/newstorydialog.js")
}
