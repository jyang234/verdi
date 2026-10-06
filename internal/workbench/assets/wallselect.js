// The wall's selection model (spec/wall-canvas-v2 ac-1, ac-2, co-2; ledger
// SI-350 (4), (5), (8), (14), SI-340 (11)'s arrival carry-in, and the
// review rulings in SI-358). Clicking a card selects it: the card and every
// card threaded to it are emphasized and everything else recedes; clicking
// the wall or the same card clears the selection; clicking a thread's chip
// selects the thread; a double click on the selected card edits it and
// keeps it selected. The selection's threads are drawn again in an overlay
// layer above the cards, over the same curves the base layer draws under
// them, and a status pill names the selected card and its threads, or says
// it has none. Arriving on `#obj-<id>` selects and reveals that card, and
// so does a hashchange.
//
// A new asset for the new behaviour (co-1: boardspec.js does not grow).
// It owns no data: the selection is a key into the server-rendered DOM
// (an object card's id, a stub's "stub:<slug>", a reference's ref, a
// sticky's annotation id — the chips' own data-from/-to grammar), so a
// region swap (boardspecasd.js applyRegion, boardspec.js applyFragment)
// never loses it: every swap and every yarn redraw re-applies the marks,
// and a selection whose card or thread is gone clears itself. Selection
// never holds the projection (co-2's hold contract is untouched). With
// nothing selected this file leaves no element of its own in the region,
// so the region's markup is exactly the server's.
//
// The pill is drawn in the wall frame's reserved row, below the canvas
// in normal flow (SI-358 (4): it covers nothing), and is hidden from
// assistive technology; its words go to the live region rendered once
// outside the swapped region (boardspecrender.go), rewritten only when
// they change — a clear is held long enough for a slow double click to
// bring the same words back unspoken — so an unchanged selection is
// never re-announced (Wave 6 §5.2). The canvas is bounded to the viewport
// alone — what it leaves below the row and a margin, never less than one
// card — so the frame fits the viewport wherever it sits on the page: the
// page scrolls the frame into view, the canvas scrolls inside itself, and
// the row is in view whenever the frame fills the viewport. The bound is
// hosted on #boardv2-region, which a swap never replaces, so a swapped-in
// canvas is bounded the moment it is inserted and the swap's restored
// scroll offsets survive (§5.1).
//
// Marks (read by the stylesheet, by F5c's Document chips, and by the
// lanes that follow): data-selected="true" on the selected card or chip,
// data-linked="<edge type>" (+ data-linked-layer) on every card the
// selection's threads reach, data-hot="true" on those threads' chips, and
// data-selection="card"|"thread" on the canvas while anything is selected.
// The `wall-selection` event fires on document after every change, a
// clear-on-gone included.
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  var region = document.getElementById("boardv2-region");
  if (!state || !region) return;

  var live = document.getElementById("wall-status-live");
  // The pill mentions dragging a pin only where the wall offers a pin to
  // drag: authoring mode with its domain live (SI-350 (14) as amended),
  // and then only on a card that carries one.
  var pinsLive = state.mode === "authoring" && !state.domainRefusal;
  var SVG = "http://www.w3.org/2000/svg";
  var DRAG_SLOP = 4; // px of travel between press and release that makes a click a drag's tail
  // A second click on the selected card inside this window is a double
  // click, which edits the card and keeps it selected (ac-5; SI-358 (5)):
  // the clear a repeated click means waits it out.
  var CLEAR_DELAY = 300;
  var CONTROLS = "a, button, textarea, input, select, label, .review-sticky, .sticky-draft, .card-editor, .badge-drawer";
  var CARDS = ".objcard, .stubcard, .refcard, .sticky";

  // The live region's clear waits this long: a slow double click (two
  // clicks apart by more than CLEAR_DELAY but inside the OS interval)
  // clears and re-selects, and the same words must not be spoken again.
  var LIVE_HOLD = 600;
  var VIEW_MARGIN = 16; // CSS px left below the frame when it fills the viewport
  var CANVAS_FLOOR = 160; // the least the bounded canvas shrinks to: one card (140) plus a 10 px margin above and below
  // The row's layout (data-wall-row on the region; the stylesheet reads
  // it). The authoring wall's fixed pin-toolbox tab holds the viewport's
  // lower left, and the row keeps the pill clear of it to the right (the
  // stylesheet's band). Where the frame is narrower than ROW_STACK_WIDTH
  // that leaves the pill too little room to read, so the pill takes a
  // line of its own above the band ("stacked") — safe only while the
  // viewport is tall enough for the frame to fit with that taller row,
  // since a band reserved below the pill protects it only when the
  // frame's foot can reach the viewport's. Below ROW_FIT_HEIGHT (a short
  // window at 200 % zoom) not even the floor canvas and a wrapped row fit
  // together, so the row leads the canvas ("leading"): the pill stays in
  // view with the canvas's top, keeps the band's margin, and the canvas's
  // foot is reached by scrolling the page.
  var ROW_STACK_WIDTH = 480; // CSS px of frame width below which the pill takes its own line
  var ROW_FIT_HEIGHT = 320; // CSS px of viewport height below which the row leads the canvas

  var selection = null; // { kind: "card" | "thread", key: string } | null
  var spoken = ""; // the live region's current words, so unchanged words are never rewritten
  var liveClear = null; // the held clear of the live region
  var down = null; // the last press's point, for the drag-tail guard
  var pendingClear = null; // the deferred clear of a repeated click

  function canvas() {
    return document.getElementById("board-canvas");
  }
  function frame() {
    return region.querySelector(".wall-frame");
  }

  // measure bounds the canvas to the viewport (SI-358 (4); Wave 6 §5.2):
  // its height is the least of its content-sized min-height (the server's
  // inline style) and what the viewport leaves below the row and
  // VIEW_MARGIN, never under CANVAS_FLOOR. Where the frame sits on the
  // page does not enter it: at 320 px the layout stacks and the frame
  // starts thousands of pixels down, which must not shrink the canvas to
  // its floor. A zoomed body scales the frame's box, so lengths are read
  // in its scale. The row's layout is chosen first, so its height is the
  // one the bound subtracts. The bound lives on the region, which a swap
  // never replaces (§5.1: the swap's restored scroll offsets survive).
  function measure() {
    var f = frame();
    var c = canvas();
    var row = f && f.querySelector(".wall-status-row");
    if (!f || !c || !row) return;
    var content = parseFloat(c.style.minHeight);
    if (!(content > 0)) return;
    var scale = f.offsetWidth ? f.getBoundingClientRect().width / f.offsetWidth : 1;
    if (!(scale > 0)) scale = 1;
    var view = document.documentElement.clientHeight / scale;
    if (view < ROW_FIT_HEIGHT) {
      region.setAttribute("data-wall-row", "leading");
    } else if (f.offsetWidth < ROW_STACK_WIDTH) {
      region.setAttribute("data-wall-row", "stacked");
    } else {
      region.removeAttribute("data-wall-row");
    }
    var room = view - row.getBoundingClientRect().height / scale - VIEW_MARGIN;
    var height = Math.max(CANVAS_FLOOR, Math.min(content, room));
    region.style.setProperty("--wall-canvas-height", Math.round(height) + "px");
    region.setAttribute("data-wall-measured", "true");
  }
  function esc(s) {
    return window.CSS && CSS.escape ? CSS.escape(s) : s.replace(/["\\]/g, "\\$&");
  }

  // -- keys -----------------------------------------------------------------

  function cardKey(el) {
    if (el.classList.contains("stubcard")) return "stub:" + el.getAttribute("data-stub");
    if (el.classList.contains("refcard")) return el.getAttribute("data-ref");
    return el.getAttribute("data-id");
  }

  // cardByKey mirrors boardspec.js's endpointElement: the one grammar the
  // chips' endpoints and this file share. "spec" is the document itself,
  // not a card.
  function cardByKey(key) {
    var c = canvas();
    if (!c || !key || key === "spec") return null;
    if (key.indexOf("stub:") === 0) {
      return c.querySelector('.stubcard[data-stub="' + esc(key.slice(5)) + '"]');
    }
    return (
      c.querySelector('.objcard[data-id="' + esc(key) + '"]') ||
      c.querySelector('.refcard[data-ref="' + esc(key) + '"]') ||
      c.querySelector('.sticky[data-id="' + esc(key) + '"]')
    );
  }

  // A thread's key is its chip's identity across swaps: layer, type, both
  // endpoints, and the annotation id when it has one.
  function chipKey(chip) {
    return [
      chip.getAttribute("data-layer"),
      chip.getAttribute("data-edge-type"),
      chip.getAttribute("data-from"),
      chip.getAttribute("data-to"),
      chip.getAttribute("data-annotation-id") || "",
    ].join("\u001f");
  }

  function chips() {
    var c = canvas();
    return c ? Array.prototype.slice.call(c.querySelectorAll(".yarn-chip")) : [];
  }

  // -- names: how the pill speaks of an endpoint ------------------------------

  function labelOf(key) {
    if (key === "spec") return "this spec";
    if (key.indexOf("stub:") === 0) return key.slice(5);
    var el = cardByKey(key);
    if (el && el.classList.contains("sticky")) return "sticky";
    return key;
  }

  function cardLabel(el) {
    if (el.classList.contains("sticky")) {
      var type = el.querySelector(".sticky-type");
      return (type ? type.textContent.trim() + " " : "") + "sticky";
    }
    return labelOf(cardKey(el));
  }

  // -- the base layer's threads, paired with their chips ----------------------
  //
  // layoutYarn (boardspec.js) appends one path per chip, in chip order,
  // each followed by its knots, so the k-th thread is the k-th chip's.
  function baseThreads() {
    var c = canvas();
    var svg = c && c.querySelector("svg.yarn-svg");
    var out = [];
    if (!svg) return out;
    var cur = null;
    for (var n = svg.firstElementChild; n; n = n.nextElementSibling) {
      if (n.tagName === "path" && n.classList.contains("yarn-thread")) {
        cur = { path: n, knots: [] };
        out.push(cur);
      } else if (n.tagName === "circle" && cur) {
        cur.knots.push(n);
      }
    }
    return out;
  }

  // drawOverlay traces the selection's threads above the cards: a halo in
  // the wall's own colour under a heavier stroke in the thread's colour,
  // and larger knots — the same curve the base layer drew, so the two
  // layers can never disagree. The overlay exists only while there is a
  // thread to trace: with nothing selected the region's markup is exactly
  // the server's (spec 38 pins a drawer's open-and-close as a no-op).
  function drawOverlay(c, hot) {
    var svg = c.querySelector("svg.yarn-overlay");
    if (!hot.length) {
      if (svg) svg.remove();
      return;
    }
    if (!svg) {
      svg = document.createElementNS(SVG, "svg");
      svg.setAttribute("class", "yarn-overlay");
      svg.setAttribute("aria-hidden", "true");
      c.appendChild(svg);
    }
    while (svg.firstChild) svg.removeChild(svg.firstChild);
    var threads = baseThreads();
    for (var i = 0; i < hot.length; i++) {
      var t = threads[hot[i].index];
      if (!t) continue;
      var d = t.path.getAttribute("d");
      var halo = document.createElementNS(SVG, "path");
      halo.setAttribute("class", "yarn-overlay-halo");
      halo.setAttribute("d", d);
      svg.appendChild(halo);
      var p = document.createElementNS(SVG, "path");
      p.setAttribute("class", t.path.getAttribute("class") + " yarn-overlay-thread");
      p.setAttribute("d", d);
      svg.appendChild(p);
      for (var j = 0; j < t.knots.length; j++) {
        var k = document.createElementNS(SVG, "circle");
        k.setAttribute("class", t.knots[j].getAttribute("class") + " yarn-overlay-knot");
        k.setAttribute("cx", t.knots[j].getAttribute("cx"));
        k.setAttribute("cy", t.knots[j].getAttribute("cy"));
        k.setAttribute("r", 4);
        svg.appendChild(k);
      }
    }
  }

  // -- the status pill --------------------------------------------------------

  // words derives the pill's two parts from the selection: the card or
  // the thread's type, and the threads or the pair.
  function words(selEl, hot) {
    var id = "";
    var summary = "";
    if (selection && selection.kind === "card" && selEl) {
      id = cardLabel(selEl);
      var parts = [];
      for (var i = 0; i < hot.length; i++) {
        var chip = hot[i].chip;
        var from = chip.getAttribute("data-from");
        var type = chip.getAttribute("data-edge-type");
        parts.push(
          from === selection.key
            ? type + " \u2192 " + labelOf(chip.getAttribute("data-to"))
            : type + " \u2190 " + labelOf(from)
        );
      }
      if (parts.length) {
        summary = parts.join(" \u00b7 ");
      } else if (pinsLive && selEl.querySelector(".yarn-handle")) {
        summary = "no threads yet \u2014 drag the pin to string one";
      } else {
        summary = "no threads yet";
      }
    } else if (selection && selection.kind === "thread" && hot.length) {
      var sel = hot[0].chip;
      id = sel.getAttribute("data-edge-type");
      summary = labelOf(sel.getAttribute("data-from")) + " \u2192 " + labelOf(sel.getAttribute("data-to"));
    }
    return { id: id, summary: summary };
  }

  // drawPill keeps the visual pill in the frame's reserved row while there
  // is a selection to name, and removes it otherwise.
  function drawPill(w) {
    var row = region.querySelector(".wall-status-row");
    if (!row) return;
    var el = row.querySelector(".wall-status");
    if (!w.id) {
      if (el) el.remove();
      return;
    }
    if (!el) {
      el = document.createElement("div");
      el.className = "wall-status";
      el.setAttribute("data-testid", "wall-status");
      el.setAttribute("aria-hidden", "true");
      var idEl = document.createElement("span");
      idEl.className = "wall-status-id";
      var sumEl = document.createElement("span");
      sumEl.className = "wall-status-summary";
      el.appendChild(idEl);
      el.appendChild(sumEl);
      row.appendChild(el);
    }
    if (el.children[0].textContent !== w.id) el.children[0].textContent = w.id;
    if (el.children[1].textContent !== w.summary) el.children[1].textContent = w.summary;
  }

  // speak gives the live region the pill's words, only when they change.
  // A clear is held for LIVE_HOLD: if the same words come back inside it
  // (a slow double click), nothing is rewritten; different words cancel
  // the hold and are spoken at once.
  function speak(w) {
    if (!live) return;
    var text = w.id ? w.id + " " + w.summary : "";
    if (text) {
      if (liveClear) {
        clearTimeout(liveClear);
        liveClear = null;
      }
      if (text === spoken) return;
      spoken = text;
      live.textContent = text;
      return;
    }
    if (!spoken || liveClear) return;
    liveClear = setTimeout(function () {
      liveClear = null;
      spoken = "";
      live.textContent = "";
    }, LIVE_HOLD);
  }

  // -- applying the selection to the DOM --------------------------------------

  function clearMarks(c) {
    var marked = c.querySelectorAll("[data-selected], [data-linked], [data-hot]");
    for (var i = 0; i < marked.length; i++) {
      marked[i].removeAttribute("data-selected");
      marked[i].removeAttribute("data-linked");
      marked[i].removeAttribute("data-linked-layer");
      marked[i].removeAttribute("data-hot");
    }
  }

  function link(el, chip) {
    if (!el) return;
    el.setAttribute("data-linked", chip.getAttribute("data-edge-type"));
    el.setAttribute("data-linked-layer", chip.getAttribute("data-layer"));
  }

  // apply re-derives every mark from the selection and the current DOM:
  // idempotent, so it runs after every click, swap, and yarn redraw. It
  // reports whether it cleared a selection whose card or thread is gone.
  function apply() {
    var c = canvas();
    if (!c) return false;
    var had = selection;
    clearMarks(c);
    var all = chips();
    var hot = []; // { chip, index }: the threads touching the selection
    var selEl = null;
    if (selection && selection.kind === "card") {
      selEl = cardByKey(selection.key);
      if (!selEl) {
        selection = null;
      } else {
        selEl.setAttribute("data-selected", "true");
        for (var i = 0; i < all.length; i++) {
          var from = all[i].getAttribute("data-from");
          var to = all[i].getAttribute("data-to");
          if (from !== selection.key && to !== selection.key) continue;
          hot.push({ chip: all[i], index: i });
          var other = cardByKey(from === selection.key ? to : from);
          if (other && other !== selEl) link(other, all[i]);
        }
      }
    } else if (selection && selection.kind === "thread") {
      var found = -1;
      for (var j = 0; j < all.length; j++) {
        if (chipKey(all[j]) === selection.key) {
          found = j;
          break;
        }
      }
      if (found < 0) {
        selection = null;
      } else {
        var chip = all[found];
        chip.setAttribute("data-selected", "true");
        hot.push({ chip: chip, index: found });
        link(cardByKey(chip.getAttribute("data-from")), chip);
        link(cardByKey(chip.getAttribute("data-to")), chip);
      }
    }
    for (var k = 0; k < hot.length; k++) hot[k].chip.setAttribute("data-hot", "true");
    if (selection) c.setAttribute("data-selection", selection.kind);
    else c.removeAttribute("data-selection");
    drawOverlay(c, hot);
    var w = words(selEl, hot);
    drawPill(w);
    speak(w);
    measure();
    // The overlay's and the pill's own rebuilds are the mutations this
    // file makes that the observer would see; drop them so apply never
    // re-triggers itself.
    observer.takeRecords();
    return !!had && !selection;
  }

  function announce() {
    document.dispatchEvent(new CustomEvent("wall-selection", { detail: { selection: selection } }));
  }

  function same(a, b) {
    return !!a && !!b && a.kind === b.kind && a.key === b.key;
  }

  function cancelPendingClear() {
    if (pendingClear) {
      clearTimeout(pendingClear);
      pendingClear = null;
    }
  }

  function select(next) {
    cancelPendingClear();
    selection = next;
    apply();
    announce();
  }

  // toggle selects what was clicked, or, for the selection itself, clears
  // it once the double-click window has passed without a double click.
  function toggle(next) {
    if (!same(selection, next)) {
      select(next);
      return;
    }
    cancelPendingClear();
    pendingClear = setTimeout(function () {
      pendingClear = null;
      if (same(selection, next)) select(null);
    }, CLEAR_DELAY);
  }

  // -- clicks -----------------------------------------------------------------

  document.addEventListener(
    "pointerdown",
    function (e) {
      down = { x: e.clientX, y: e.clientY };
    },
    true
  );

  document.addEventListener("click", function (e) {
    var t = e.target;
    var c = canvas();
    if (!c || !(t instanceof Element) || !c.contains(t)) return; // only the wall selects or clears
    if (e.detail > 1) return; // the second click of a double click: editing wins
    if (down && Math.hypot(e.clientX - down.x, e.clientY - down.y) > DRAG_SLOP) return; // a drag's tail
    // Controls keep their own meaning: a chip's inner buttons, a card's
    // affordances, its ⋯ control and links, editors and drafts, an
    // anchored review sticky.
    if (t.closest(CONTROLS)) return;
    var chip = t.closest(".yarn-chip");
    if (chip) {
      toggle({ kind: "thread", key: chipKey(chip) });
      return;
    }
    var card = t.closest(CARDS);
    if (card) {
      toggle({ kind: "card", key: cardKey(card) });
      return;
    }
    select(null); // the wall itself
  });

  // A double click edits the card it lands on (boardspec.js) and keeps it
  // selected: the repeated click's deferred clear is withdrawn, and a card
  // not yet selected becomes so.
  document.addEventListener("dblclick", function (e) {
    cancelPendingClear();
    var t = e.target;
    var c = canvas();
    if (!c || !(t instanceof Element) || !c.contains(t) || t.closest(CONTROLS)) return;
    var card = t.closest(CARDS);
    if (!card) return;
    var next = { kind: "card", key: cardKey(card) };
    if (!same(selection, next)) select(next);
  });

  // -- swaps and redraws --------------------------------------------------------
  //
  // Every region swap and every yarn redraw (layoutYarn clears and refills
  // the base layer, on each drag frame too) re-applies the marks and
  // retraces the overlay; attribute changes do not fire it. A selection
  // the swap took away is announced as cleared.
  var observer = new MutationObserver(function () {
    if (apply()) announce();
  });
  observer.observe(region, { childList: true, subtree: true });

  // The seam the following lanes read: the toolbar acts on the selection,
  // the keyboard moves it. Set before arrival, so a page that arrives on a
  // card already has it.
  window.__WALLSELECT__ = {
    selection: function () {
      return selection;
    },
    select: select,
    clear: function () {
      select(null);
    },
  };

  // -- arrival (SI-340 (11)) -----------------------------------------------------

  function arrive() {
    var m = /^#obj-(.+)$/.exec(window.location.hash || "");
    if (!m) return;
    var id;
    try {
      id = decodeURIComponent(m[1]);
    } catch (err) {
      return; // a malformed hash names no card
    }
    var c = canvas();
    var card = c && c.querySelector('.objcard[data-id="' + esc(id) + '"]');
    if (!card) return;
    select({ kind: "card", key: id });
    card.scrollIntoView({ block: "nearest", inline: "nearest" });
  }
  window.addEventListener("hashchange", arrive);
  window.addEventListener("resize", measure);
  window.addEventListener("load", measure);
  measure();
  arrive();
})();
