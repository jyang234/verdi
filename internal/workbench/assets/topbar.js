// The workbench top bar's script (spec/chrome-and-tokens-v2): every
// workbench page's one bar, served at /assets/topbar.js so the shared
// layout and the four page shells all reference the same asset. New
// behavior ships in a new asset of at most 64 KiB (spec/workbench-redesign
// co-1; TestTopBarAsset_ServedWithinBudget) rather than growing
// boardspec.js. The bar is complete before this script runs; the script
// only enhances it.
//
// What it enhances (dc-2; ledger SI-331): the posture disclosure — a
// native <details>, open before any script runs — becomes the handoff's
// popover. Escape closes an open popover and returns focus to its summary.
// A pointer press outside the posture group closes it; a press on the
// group's own controls (its Refresh) is inside, so an open popover
// survives a refresh the user asks for, as it survives a snapshot refresh
// (boardspecasd.js's applyPosture keeps the open state and the focused
// control across the swap, SI-323 (3)). On an outside press, focus
// returns to the summary only when the press leaves focus nowhere — a
// press on a focusable control keeps that control's focus — and the page
// never scrolls for it. Nothing here holds an element across events: the
// wall's refresh replaces the posture group underneath this script.
(function () {
  "use strict";
  var bar = document.querySelector('[data-testid="topbar"]');
  if (!bar) return;

  function shown() {
    return bar.querySelector("details.topbar-posture[open]");
  }
  function refocus() {
    var s = bar.querySelector(".topbar-posture > summary");
    if (s) s.focus({ preventScroll: true });
  }

  // preventDefault marks the keystroke as taken, so a page-level Escape
  // handler registered before this one (the diagram editor's exit) stands
  // down whichever of the two listeners runs first.
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    var d = shown();
    if (!d) return;
    e.preventDefault();
    d.removeAttribute("open");
    refocus();
  });

  document.addEventListener("pointerdown", function (e) {
    var d = shown();
    if (!d) return;
    var group = d.closest("#asd-posture") || d;
    if (group.contains(e.target)) return;
    d.removeAttribute("open");
    setTimeout(function () {
      if (!document.activeElement || document.activeElement === document.body) refocus();
    }, 0);
  });
})();
