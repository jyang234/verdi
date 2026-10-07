// The wall's keyboard (spec/wall-canvas-v2 ac-6; ledger SI-350 (4),
// SI-361 (3); BL-173 (3); lane F2c). The arrows move the selection within
// a column (up and down) and across columns (left and right, to the
// nearest card by position) and reveal the card inside the bounded
// canvas; Enter on a focused card edits it; Delete removes the selected
// card or thread through the existing confirmation, and refuses a declared
// stub in the trash's own words; Escape clears the selection, last of all
// the layers.
//
// A new asset for the new behaviour (co-1: boardspec.js does not grow).
// Built on the selection seam (wallselect.js: window.__WALLSELECT__), the
// board's own entries (boardspec.js: window.__BOARDV2API__ editCard and
// remove — the same paths, refusals and confirmations the mouse takes)
// and the toolbar (walltoolbar.js: its Delete action is the one judgement
// of what may be deleted). It owns no data and holds no projection: the
// selection is the seam's, and every write is boardspec.js's.
//
// The layers Escape closes, innermost first, one per press: an inline
// editor or an open slot (their own listeners, which stop the key), then
// the modal layer or one open dialog (boardspec.js's Escape, which closes
// one layer per press), then the selection (here). The arrows and Delete
// rest while a field, a dialog or a menu has the focus, and under a
// modal, so a key typed into an editor never reaches the card (ac-5; the
// Delete mutant). Focus follows the selection the arrows move, onto the
// card itself — the handoff's cards are focusable only when they are
// object cards, so the others take tabindex=-1 as they are reached, and
// again after a swap, so the swap's focus restore can land — and the
// reveal is immediate: no viewport animation is introduced, so Wave 6
// §5.2's reduced-motion rule has nothing to reduce.
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  var region = document.getElementById("boardv2-region");
  if (!state || !region) return;

  var CARDS = ".objcard, .stubcard, .refcard, .sticky";
  var REVEAL_MARGIN = 40; // px the revealed card keeps inside the canvas's visible box (the handoff's ≥ 40 px)
  var COLUMN_SLACK = 100; // px of horizontal distance within which cards count as one column (half a card)
  // The focus a key belongs to: a field, a dialog, a menu, a control.
  var FIELDS = "input, textarea, select, [contenteditable], [role=dialog], [role=alertdialog], [role=menu]";
  var CONTROLS = FIELDS + ", button, a, [role=toolbar]";

  function canvas() {
    return document.getElementById("board-canvas");
  }
  function api() {
    return window.__BOARDV2API__;
  }
  function seam() {
    return window.__WALLSELECT__;
  }

  // -- what is open -------------------------------------------------------------

  // modalOpen: a layer with a scrim is up (the board dialogs' shared
  // backdrop, the expand dialog's, the badge drawer's).
  function modalOpen() {
    var bd = document.getElementById("modal-backdrop");
    if (bd && !bd.hidden) return true;
    return !!(document.getElementById("expand-dialog") || document.getElementById("drawer-backdrop"));
  }
  // dialogOpen mirrors boardspec.js's Escape chain: the modal layer, the
  // reference peek, the pin tray. While any is open, Escape is that
  // chain's, not the selection's.
  function dialogOpen() {
    if (modalOpen() || document.getElementById("ref-peek")) return true;
    var tray = document.getElementById("pin-tray");
    return !!(tray && !tray.hidden);
  }

  // -- the cards and their geometry ----------------------------------------------------

  function cards(c) {
    var els = c.querySelectorAll(CARDS);
    var out = [];
    for (var i = 0; i < els.length; i++) {
      if (!els[i].classList.contains("sticky-draft")) out.push(els[i]);
    }
    return out;
  }
  function box(el) {
    return {
      x: el.offsetLeft,
      y: el.offsetTop,
      w: el.offsetWidth,
      h: el.offsetHeight,
      cx: el.offsetLeft + el.offsetWidth / 2,
      cy: el.offsetTop + el.offsetHeight / 2,
    };
  }

  function selectedCard() {
    var s = seam() ? seam().selection() : null;
    return s && s.kind === "card" ? seam().elementOf(s) : null;
  }

  // origin is the card an arrow moves from: the selected card, else the
  // focused card (a Tab stop reached before anything was selected).
  function origin(t) {
    var sel = selectedCard();
    if (sel) return sel;
    var c = canvas();
    var card = t.closest(CARDS);
    return card && c && c.contains(card) && !card.classList.contains("sticky-draft") ? card : null;
  }

  // neighbour picks the card an arrow moves to. Up and down stay in the
  // column: among the cards whose footprint shares the current card's
  // horizontal span, the nearest above or below. Left and right cross to
  // the nearest column on that side — the smallest horizontal distance
  // between centres, with COLUMN_SLACK gathering that column's cards —
  // and within it to the card nearest by vertical position. At the wall's
  // edge there is none.
  function neighbour(cur, key) {
    var all = cards(canvas());
    var r = box(cur);
    var vertical = key === "ArrowUp" || key === "ArrowDown";
    var cands = [];
    for (var i = 0; i < all.length; i++) {
      if (all[i] === cur) continue;
      var b = box(all[i]);
      if (vertical) {
        if (b.x >= r.x + r.w || b.x + b.w <= r.x) continue;
        var dy = key === "ArrowUp" ? r.cy - b.cy : b.cy - r.cy;
        if (dy > 0) cands.push({ el: all[i], d: dy, dy: 0 });
      } else {
        var dx = key === "ArrowLeft" ? r.cx - b.cx : b.cx - r.cx;
        if (dx > 0) cands.push({ el: all[i], d: dx, dy: Math.abs(b.cy - r.cy) });
      }
    }
    if (!cands.length) return null;
    var nearest = Infinity;
    for (var j = 0; j < cands.length; j++) nearest = Math.min(nearest, cands[j].d);
    var best = null;
    for (var k = 0; k < cands.length; k++) {
      var cd = cands[k];
      if (cd.d > nearest + COLUMN_SLACK) continue;
      if (!best || cd.dy < best.dy || (cd.dy === best.dy && cd.d < best.d)) best = cd;
    }
    return best.el;
  }

  // reveal scrolls the bounded canvas so the card sits REVEAL_MARGIN inside
  // its visible box, where the canvas has the room, then lets the page
  // bring the wall frame into view — the frame fits the viewport
  // (wallselect.js measure), so its row, with the toolbar and the
  // minimap, comes with the canvas — and the card last, in case the
  // frame does not fit. The move is immediate.
  function reveal(el) {
    var c = canvas();
    if (!c) return;
    var b = box(el);
    var m = REVEAL_MARGIN;
    var sl = c.scrollLeft;
    var st = c.scrollTop;
    if (b.x - m < sl) sl = b.x - m;
    else if (b.x + b.w + m > sl + c.clientWidth) sl = b.x + b.w + m - c.clientWidth;
    if (b.y - m < st) st = b.y - m;
    else if (b.y + b.h + m > st + c.clientHeight) st = b.y + b.h + m - c.clientHeight;
    c.scrollLeft = Math.max(0, sl);
    c.scrollTop = Math.max(0, st);
    var f = region.querySelector(".wall-frame");
    if (f) f.scrollIntoView({ block: "nearest", inline: "nearest" });
    el.scrollIntoView({ block: "nearest", inline: "nearest" });
  }

  function focusCard(el) {
    if (!el.hasAttribute("tabindex")) el.setAttribute("tabindex", "-1");
    el.focus({ preventScroll: true });
  }

  function move(key, t) {
    var cur = origin(t);
    if (!cur || !seam()) return false;
    var next = neighbour(cur, key) || cur;
    seam().selectElement(next);
    reveal(next);
    focusCard(next);
    return true;
  }

  // -- Enter on a focused card ---------------------------------------------------------

  // Enter on a focused object card — the card itself, never a control
  // inside it, whose Enter is its own — selects it and opens the card
  // editor (boardspec.js's one editor entry, with its domain gate); Enter
  // with the focus elsewhere is walltoolbar.js's, on the selection.
  function enterOnCard(t) {
    var c = canvas();
    var card = t.classList.contains("objcard") ? t : null;
    if (!card || !c || !c.contains(card) || !api() || !seam()) return false;
    if (selectedCard() !== card) seam().selectElement(card);
    api().editCard(card);
    return true;
  }

  // -- Delete -------------------------------------------------------------------

  // del removes the selection as the toolbar would: a card takes the
  // trash's own routing (boardspec.js removeElement — the domain refusal,
  // the stub's plain refusal, a held reference's, and the confirmations,
  // the same words from the same source), in authoring mode where the
  // trash exists; a thread is deleted only where the toolbar offers its
  // Delete, which is the one legality judgement.
  function del() {
    var s = seam() ? seam().selection() : null;
    var el = s ? seam().elementOf(s) : null;
    if (!el || !api()) return false;
    if (s.kind === "card") {
      if (state.mode !== "authoring") return false;
      api().remove(el);
      return true;
    }
    var button = region.querySelector('.wall-toolbar [data-wall-action="delete"]');
    if (!button) return false;
    button.click();
    return true;
  }

  // -- Escape clears the selection, last ---------------------------------------------------

  // clear clears the selection and keeps the focus where it was; a focus
  // the toolbar's refill could not keep (its action is gone with the
  // selection) goes to the toolbar's first action.
  function clear() {
    var toolbar = region.querySelector(".wall-toolbar");
    var inToolbar = toolbar && toolbar.contains(document.activeElement);
    seam().clear();
    if (inToolbar && !toolbar.contains(document.activeElement)) {
      var first = toolbar.querySelector("button, a");
      if (first) first.focus({ preventScroll: true });
    }
  }

  document.addEventListener(
    "keydown",
    function (e) {
      var t = e.target;
      if (!(t instanceof Element)) t = document.body;
      if (e.altKey || e.ctrlKey || e.metaKey) return;
      var key = e.key;
      if (key === "Escape") {
        if (t.closest(FIELDS) || dialogOpen()) return; // the field's, or boardspec.js's chain
        if (!seam() || !seam().selection()) return;
        clear();
        return;
      }
      if (key === "ArrowUp" || key === "ArrowDown" || key === "ArrowLeft" || key === "ArrowRight") {
        if (e.shiftKey || t.closest(CONTROLS) || modalOpen()) return;
        if (move(key, t)) e.preventDefault();
        return;
      }
      if (key === "Delete" || key === "Backspace") {
        if (e.shiftKey || t.closest(CONTROLS) || modalOpen()) return;
        if (del()) e.preventDefault();
        return;
      }
      if (key === "Enter" && !e.shiftKey && !e.isComposing) {
        if (t.closest(FIELDS) || modalOpen()) return;
        if (enterOnCard(t)) e.preventDefault();
      }
    },
    true
  );

  // A swap replaces the cards: the selected card is made focusable again
  // before the transport restores the focus to it by its key
  // (boardspecasd.js applyRegion: the event fires before restoreFocus).
  document.addEventListener("wall-region-swapped", function () {
    var sel = selectedCard();
    if (sel && !sel.hasAttribute("tabindex")) sel.setAttribute("tabindex", "-1");
  });
})();
