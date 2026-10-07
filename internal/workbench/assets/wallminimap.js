// The wall's minimap (spec/wall-canvas-v2 ac-6; ledger SI-358 (4); lane
// F2c). It shows every card as a block at the canvas's scale and the
// bounded canvas's viewport as a frame, and dragging the frame moves the
// viewport: the canvas scrolls so the point under the pointer is its
// centre, as the handoff draws it.
//
// A new asset for the new behaviour (co-1: boardspec.js does not grow).
// It owns no data: the blocks are the cards' own boxes, read from the
// server-rendered DOM, and the frame is the canvas's scroll state. It
// draws into the host the server renders at the status row's end
// (boardspecrender.go), so it covers no paper, label or control (SI-358
// (4)) and departs from the handoff's corner over the cork. The host is
// hidden from assistive technology and is never a tab stop: a pointer
// aid, whose keyboard path is the arrows (wallkeys.js), which reveal
// every card (Wave 6 §5.2). The frame moves with the scroll and the
// scroll with the frame at once — no transition, nothing for reduced
// motion to reduce.
//
// Every region swap and every yarn redraw (a drag moves a card) rebuilds
// the blocks, coalesced into one frame; the canvas's scroll, which does
// not bubble, is caught in the capture phase on the region, so a swapped-in
// canvas is followed without rebinding.
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  var region = document.getElementById("boardv2-region");
  if (!state || !region) return;

  var PAD = 3; // px of the host's inner box kept clear around the drawing
  var CARDS = ".objcard, .stubcard, .refcard, .sticky";
  var scale = 0; // canvas px to minimap px
  var raf = null;
  var dragging = false;

  function canvas() {
    return document.getElementById("board-canvas");
  }
  function host() {
    return region.querySelector(".wall-minimap");
  }

  function kindOf(el) {
    if (el.classList.contains("objcard")) return el.getAttribute("data-object-kind") || "object";
    if (el.classList.contains("stubcard")) return "stub";
    if (el.classList.contains("refcard")) return "reference";
    return "sticky";
  }

  // build redraws the blocks — one per card, the card's canvas box scaled
  // into the host's inner box — and the viewport frame over them. The
  // scale fits the canvas's whole scroll extent, so every card is shown.
  function build() {
    raf = null;
    var h = host();
    var c = canvas();
    if (!h || !c) return;
    var w = h.clientWidth - 2 * PAD;
    var hh = h.clientHeight - 2 * PAD;
    if (w <= 0 || hh <= 0) return;
    scale = Math.min(w / Math.max(c.scrollWidth, 1), hh / Math.max(c.scrollHeight, 1));
    while (h.firstChild) h.removeChild(h.firstChild);
    var els = c.querySelectorAll(CARDS);
    for (var i = 0; i < els.length; i++) {
      var el = els[i];
      if (el.classList.contains("sticky-draft")) continue;
      var b = document.createElement("div");
      b.className = "wall-minimap-card";
      b.setAttribute("data-kind", kindOf(el));
      var type = el.getAttribute("data-annotation-type");
      if (type && el.classList.contains("sticky")) b.setAttribute("data-sticky-type", type);
      b.style.left = PAD + el.offsetLeft * scale + "px";
      b.style.top = PAD + el.offsetTop * scale + "px";
      b.style.width = Math.max(2, el.offsetWidth * scale) + "px";
      b.style.height = Math.max(2, el.offsetHeight * scale) + "px";
      h.appendChild(b);
    }
    var view = document.createElement("div");
    view.className = "wall-minimap-view";
    h.appendChild(view);
    sync();
  }

  // sync places the viewport frame from the canvas's scroll state.
  function sync() {
    var h = host();
    var c = canvas();
    var view = h && h.querySelector(".wall-minimap-view");
    if (!view || !c || !scale) return;
    view.style.left = PAD + c.scrollLeft * scale + "px";
    view.style.top = PAD + c.scrollTop * scale + "px";
    view.style.width = c.clientWidth * scale + "px";
    view.style.height = c.clientHeight * scale + "px";
  }

  function schedule() {
    if (raf === null) raf = requestAnimationFrame(build);
  }

  // -- the drag --------------------------------------------------------------------

  // moveTo scrolls the canvas so the canvas point under the pointer is the
  // centre of its viewport (the handoff: scrollLeft = x / scale − width / 2).
  // A zoomed body scales the host's client box, so the pointer is read in
  // the host's own scale.
  function moveTo(h, c, e) {
    var r = h.getBoundingClientRect();
    var zoom = h.offsetWidth ? r.width / h.offsetWidth : 1;
    var x = (e.clientX - r.left) / zoom - h.clientLeft - PAD;
    var y = (e.clientY - r.top) / zoom - h.clientTop - PAD;
    c.scrollLeft = x / scale - c.clientWidth / 2;
    c.scrollTop = y / scale - c.clientHeight / 2;
  }

  function end(e) {
    if (!dragging) return;
    dragging = false;
    var h = host();
    if (!h) return;
    h.removeAttribute("data-dragging");
    if (h.hasPointerCapture && h.hasPointerCapture(e.pointerId)) h.releasePointerCapture(e.pointerId);
  }

  region.addEventListener("pointerdown", function (e) {
    var h = host();
    var c = canvas();
    if (!h || !c || !scale || !(e.target instanceof Element) || !h.contains(e.target)) return;
    if (e.button !== 0 && e.pointerType === "mouse") return;
    e.preventDefault(); // no text selection, and the focus stays where it is
    dragging = true;
    h.setAttribute("data-dragging", "true");
    if (h.setPointerCapture) h.setPointerCapture(e.pointerId);
    moveTo(h, c, e);
  });
  region.addEventListener("pointermove", function (e) {
    if (!dragging) return;
    var h = host();
    var c = canvas();
    // A swap replaced the host mid-drag: the capture is gone with it.
    if (!h || !c || !h.hasAttribute("data-dragging")) {
      dragging = false;
      return;
    }
    moveTo(h, c, e);
  });
  region.addEventListener("pointerup", end);
  region.addEventListener("pointercancel", end);

  // -- following the canvas -------------------------------------------------------------

  region.addEventListener("scroll", sync, true);
  window.addEventListener("resize", schedule);
  window.addEventListener("load", schedule);
  // Mutations inside the host are this file's own drawing; any other —
  // a swap, a redraw, a moved card — schedules a rebuild.
  var observer = new MutationObserver(function (records) {
    var h = host();
    for (var i = 0; i < records.length; i++) {
      if (!h || !h.contains(records[i].target)) {
        schedule();
        return;
      }
    }
  });
  observer.observe(region, { childList: true, subtree: true, attributes: true, attributeFilter: ["style"] });
  build();
})();
