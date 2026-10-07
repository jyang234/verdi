// The wall's keyboard (spec/wall-canvas-v2 ac-6; ledger SI-350 (4),
// SI-361 (3), SI-363; BL-173 (3); lane F2c). The arrows move the selection
// on the handoff's grid — within a column (up and down) and to the
// adjacent column at the same row (left and right) — and reveal the card
// inside the bounded canvas; Enter on a focused card edits it; Delete
// removes the selected card or thread through the existing confirmation,
// and refuses a declared stub in the trash's own words; Escape clears the
// selection, last of all the layers.
//
// A new asset for the new behaviour (co-1: boardspec.js does not grow).
// Built on the selection seam (wallselect.js: window.__WALLSELECT__), the
// board's own entries (boardspec.js: window.__BOARDV2API__ editCard and
// remove — the same paths, refusals and confirmations the mouse takes)
// and the toolbar (walltoolbar.js: its Delete action is the one judgement
// of what may be deleted). It owns no data and holds no projection: the
// selection is the seam's, and every write is boardspec.js's.
//
// The layers Escape closes, innermost first, one per press (SI-363 (2)):
// a focused editor, slot or draft (their own listeners, which stop the
// key); the modal layer (boardspec.js's Escape); an open slot or draft
// whose focus has left (cancelled here through its own field); the
// branch menu (closed here: it has no backdrop, so boardspec.js's chain
// cannot see it); the reference peek or the pin tray (boardspec.js) and
// the top bar's posture popover (topbar.js, whose key is not also a
// clear); then the selection. The arrows and Delete rest while a field,
// a control, a dialog or a menu has the focus, and under a modal, so a
// key typed into an editor never reaches the card (ac-5; the Delete
// mutant). Focus follows the selection the arrows move, onto the card
// itself — the handoff's cards are focusable only when they are object
// cards, so the others take tabindex=-1 as they are reached, and again
// after a swap, so the swap's focus restore can land, selected or not —
// and the reveal is immediate: no viewport animation is introduced, so
// Wave 6 §5.2's reduced-motion rule has nothing to reduce.
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  var region = document.getElementById("boardv2-region");
  if (!state || !region) return;

  var CARDS = ".objcard, .stubcard, .refcard, .sticky";
  var REVEAL_MARGIN = 40; // px the revealed card keeps inside the canvas's visible box (the handoff's ≥ 40 px)
  // The focus a key belongs to: a field, a dialog, a menu, a control.
  var FIELDS = "input, textarea, select, [contenteditable], [role=dialog], [role=alertdialog], [role=menu]";
  var CONTROLS = FIELDS + ", button, a, summary, [role=toolbar]";
  var focusedKey = null; // the last card that had the focus, by its test id

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
  function shown(id) {
    var el = document.getElementById(id);
    return el && !el.hidden ? el : null;
  }
  // openField: the field of an open add slot or sticky draft the focus
  // has left (a focused one takes its own Escape).
  function openField() {
    var c = canvas();
    return c ? c.querySelector(".wall-slot[data-open] textarea, .sticky-draft textarea") : null;
  }
  // dialogOpen: a layer another script closes on Escape — the modal layer,
  // the reference peek and the pin tray (boardspec.js's chain), the top
  // bar's posture popover (topbar.js). While one is open, the key is its.
  function dialogOpen() {
    return modalOpen() || !!document.getElementById("ref-peek") || !!shown("pin-tray") || !!document.querySelector("details.topbar-posture[open]");
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

  // grid is the handoff's model (SI-363 (1)): the zone columns the server
  // labels, left to right (the scratch lane included; without labels,
  // every distinct card left is a column), each card in exactly one
  // column — the one nearest its centre by x, so an off-grid paper, a
  // dragged sticky or card, still has a place — and each column's cards
  // ordered by y, then by document order where two share a y.
  function grid() {
    var c = canvas();
    var cols = [];
    var labels = c.querySelectorAll(".zone-label");
    for (var i = 0; i < labels.length; i++) {
      var x = parseFloat(labels[i].style.left);
      var w = parseFloat(labels[i].style.width);
      if (isFinite(x) && isFinite(w)) cols.push({ cx: x + w / 2, cards: [] });
    }
    var all = cards(c);
    if (!cols.length) {
      var seen = {};
      for (var j = 0; j < all.length; j++) {
        var bj = box(all[j]);
        if (!seen[bj.x]) {
          seen[bj.x] = true;
          cols.push({ cx: bj.cx, cards: [] });
        }
      }
    }
    cols.sort(function (a, b) {
      return a.cx - b.cx;
    });
    for (var k = 0; k < all.length; k++) {
      var b = box(all[k]);
      var best = null;
      for (var m = 0; m < cols.length; m++) {
        var d = Math.abs(cols[m].cx - b.cx);
        if (!best || d < best.d) best = { d: d, col: cols[m] };
      }
      if (best) best.col.cards.push({ el: all[k], y: b.y, order: k });
    }
    for (var n = 0; n < cols.length; n++) {
      cols[n].cards.sort(function (a, b) {
        return a.y - b.y || a.order - b.order;
      });
    }
    return cols;
  }

  // neighbour picks the card an arrow moves to. Up and down move within
  // the column; left and right move to the adjacent column — the nearest
  // one on that side with a card in it — at the same row, clamped to that
  // column's length. At the wall's edge there is none.
  function neighbour(cur, key) {
    var cols = grid();
    for (var ci = 0; ci < cols.length; ci++) {
      var rows = cols[ci].cards;
      for (var ri = 0; ri < rows.length; ri++) {
        if (rows[ri].el !== cur) continue;
        if (key === "ArrowUp") return ri > 0 ? rows[ri - 1].el : null;
        if (key === "ArrowDown") return ri + 1 < rows.length ? rows[ri + 1].el : null;
        var step = key === "ArrowLeft" ? -1 : 1;
        for (var cj = ci + step; cj >= 0 && cj < cols.length; cj += step) {
          var next = cols[cj].cards;
          if (next.length) return next[Math.min(ri, next.length - 1)].el;
        }
        return null;
      }
    }
    return null;
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

  function focusable(el) {
    if (el && !el.hasAttribute("tabindex")) el.setAttribute("tabindex", "-1");
  }
  function focusCard(el) {
    focusable(el);
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

  // -- Escape: an unfocused slot or draft, the branch menu, then the selection ----------------

  // cancelOpen presses Escape in an open slot's or draft's own field on
  // the user's behalf: the slot closes through walltoolbar.js's handler
  // and the draft dies through boardspec.js's — one cancel path each, no
  // copy — and the focus comes back to where it was.
  function cancelOpen(field) {
    var had = document.activeElement;
    field.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, cancelable: true }));
    if (had && had !== document.body && had.isConnected && document.activeElement !== had) had.focus({ preventScroll: true });
  }

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
        if (t.closest(FIELDS) || modalOpen()) return; // the field's own, or the modal layer's (boardspec.js)
        var field = openField();
        if (field) {
          cancelOpen(field);
          e.preventDefault();
          return;
        }
        var menu = shown("branch-menu");
        if (menu) {
          menu.hidden = true;
          return;
        }
        if (dialogOpen()) return; // the peek's, the tray's (boardspec.js) or the popover's (topbar.js)
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

  // A swap replaces the cards: the selected card and the card that had
  // the focus are made focusable again before the transport restores the
  // focus by its key (boardspecasd.js applyRegion: the event fires before
  // restoreFocus), selection or not — Escape clears the selection and
  // leaves the focus on the card.
  document.addEventListener("focusin", function (e) {
    var t = e.target;
    var c = canvas();
    if (t instanceof Element && c && c.contains(t) && t.matches(CARDS)) focusedKey = t.getAttribute("data-testid");
  });
  document.addEventListener("wall-region-swapped", function () {
    focusable(selectedCard());
    var again = focusedKey && region.querySelector('[data-testid="' + focusedKey + '"]');
    if (again && again.matches(CARDS)) focusable(again);
  });
})();
