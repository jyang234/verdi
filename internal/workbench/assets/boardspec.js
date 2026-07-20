// The v1 board's entire client-side surface (05 §Workbench: the one
// deliberately fat page; JS minimal, ONE file, no framework, no build
// step). The server renders the board region; this file only (a) lays
// yarn threads and chips over the server-rendered cards, (b) drives the
// authoring gestures — drag, inline edit, sticky creation, the
// context-sensitive type picker with its gate-bearing confirmation, the
// git affordance — and (c) swaps the server-rendered fragment back in
// after every mutation, so the DOM is always the projection.
//
// Server contract (internal/workbench/boardspec*.go):
//   GET  /board/spec/<name>              -> the page (this script + state)
//   GET  /board/spec/<name>/fragment     -> re-rendered board region
//   POST /board/spec/<name>/api/<action> -> mutations; JSON {dirty} or {error}
// The same routes also serve beneath a /b/<branch-escaped>/ prefix
// (spec/draft-boards): every request below is issued relative to the
// page's own mount (mountPrefix), so one client works at both addresses.
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  if (!state) return;

  var region = document.getElementById("boardv2-region");
  var statusEl = document.getElementById("autosave-status");
  var authoring = state.mode === "authoring";

  // The board's own mount prefix (spec/draft-boards dc-1): the same page
  // serves at the unprefixed /board/spec/<name> and beneath a
  // /b/<branch-escaped>/ prefix, so every request this file issues is
  // addressed relative to the page's own mount — derived from
  // location.pathname (which keeps the branch segment's %2F escaped),
  // never a hardcoded root path. Unprefixed pages derive "".
  var mountPrefix = window.location.pathname.replace(/\/board\/spec\/[^/]+$/, "");
  function boardURL(rest) {
    return mountPrefix + "/board/spec/" + encodeURIComponent(state.spec) + rest;
  }

  function canvas() {
    return document.getElementById("board-canvas");
  }
  function setStatus(text) {
    if (statusEl) statusEl.textContent = text;
  }
  function esc(s) {
    return window.CSS && CSS.escape ? CSS.escape(s) : s.replace(/["\\]/g, "\\$&");
  }

  // -- class display words (spec/vocabulary-surfaces) ------------------------
  //
  // state.words carries the model's class-word renames (server-resolved,
  // present only for ids that actually rename — a no-rename store embeds
  // no words key at all), so THIS file's own display prose — dialog and
  // refusal copy, the sticky menu's story/spike labels — speaks the same
  // resolved vocabulary the server-rendered wall does. Fallback is the
  // bare id. Identity values never resolve here: state.class gates,
  // data-annotation-type / data-* reads, and every api() payload keep
  // bare ids (internal/workbench/vocabulary.go's enumeration rule).
  function classWord(id) {
    return (state.words && state.words[id]) || id;
  }
  function classWordCap(id) {
    var w = classWord(id);
    return w.charAt(0).toUpperCase() + w.slice(1);
  }
  // classArticle mirrors internal/model's Article — the one a/an rule
  // (judged-article-agreement-approximation-undisclosed, L-M13a(4)):
  // vowel-INITIAL display word takes "an" ("an Initiative"), else "a" —
  // duplicated here as a rule rather than a payload field exactly like
  // classWordCap mirrors model.Capitalize (the smaller diff; the Go
  // helper stays the rule's documented home, spelling-based best effort).
  function classArticle(id) {
    return /^[aeiou]/i.test(classWord(id)) ? "an" : "a";
  }
  function classArticleCap(id) {
    var a = classArticle(id);
    return a.charAt(0).toUpperCase() + a.slice(1);
  }

  // -- client-side mermaid over injected fragments ---------------------------
  //
  // The board's spec-body surfaces (the placard body dialog and the
  // reference peek) inject SERVER-rendered HTML that may carry the badged
  // <figure><pre class="mermaid"> from internal/render's one seam
  // (spec/illustrative-class dc-1 — the badge markup is server-emitted;
  // this script computes NOTHING about the tier). All the client does is
  // what the dex page's inline init does for static pages: hand the pre
  // to the one vendored mermaid asset (/assets/mermaid.min.js, the
  // dex-embedded copy the workbench re-serves — same renderer bytes on
  // every surface, no CDN, spec/illustrative-class co-2) so the source
  // becomes an SVG. Loaded lazily, once, and only when a fragment
  // actually carries a diagram — most walls never pay the asset's cost.

  var mermaidLoad = null;

  function ensureMermaid() {
    if (!mermaidLoad) {
      mermaidLoad = new Promise(function (resolve, reject) {
        var s = document.createElement("script");
        s.src = "/assets/mermaid.min.js";
        s.onload = function () {
          window.mermaid.initialize({
            startOnLoad: false,
            securityLevel: "strict",
            theme:
              window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches
                ? "dark"
                : "default",
          });
          resolve();
        };
        s.onerror = function () {
          reject(new Error("mermaid.min.js failed to load"));
        };
        document.head.appendChild(s);
      });
    }
    return mermaidLoad;
  }

  function renderMermaidIn(container) {
    var nodes = container.querySelectorAll("pre.mermaid");
    if (!nodes.length) return;
    ensureMermaid()
      .then(function () {
        return window.mermaid.run({ nodes: nodes });
      })
      .catch(function () {
        // The un-rendered <pre> stays visible — the diagram source is
        // legible text, never a dead hole; nothing here is load-bearing.
      });
  }

  // -- server round-trips --------------------------------------------------

  function api(action, body) {
    return fetch(
      boardURL("/api/" + action),
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body || {}),
      }
    ).then(function (resp) {
      return resp
        .json()
        .catch(function () {
          return {};
        })
        .then(function (data) {
          if (!resp.ok) throw new Error(data.error || "HTTP " + resp.status);
          return data;
        });
    });
  }

  // -- refresh discipline (owner bug 2026-07-19: "jerky … the refresh
  // would delay and then the screen would reset") --------------------------
  //
  // The DOM stays the server's projection, but the swap must respect the
  // hand: (a) responses carry a sequence, so a slow fragment can never
  // roll the wall back over a newer one; (b) while a gesture, an inline
  // card edit, or a sticky draft is LIVE, an arriving fragment is not
  // applied — it is held and resumed (as a fresh fetch, so nothing stale
  // is ever painted) when the interaction ends; (c) the swap replaces
  // the scrollable canvas element, so its scroll offsets are carried
  // across — a save must never snap the wall back to the origin.

  var fragmentSeq = 0; // the newest refreshFragment call owns the wall
  var heldRefresh = false; // a refresh arrived mid-interaction; resume after

  function interactionLive() {
    if (gesture || editing) return true;
    var c = canvas();
    return !!(c && c.querySelector(".sticky-draft"));
  }

  function applyFragment(html) {
    // An open derivation drawer's element lives at the body while
    // open; put it back before the region it came from is replaced,
    // so the swap never strands it.
    closeBadgeDrawer();
    var c = canvas();
    var sx = c ? c.scrollLeft : 0;
    var sy = c ? c.scrollTop : 0;
    region.innerHTML = html;
    var swapped = canvas();
    if (swapped) {
      swapped.scrollLeft = sx;
      swapped.scrollTop = sy;
    }
    layoutYarn();
    markClamped();
  }

  function refreshFragment() {
    var seq = ++fragmentSeq;
    return fetch(
      boardURL("/fragment")
    ).then(function (resp) {
      if (!resp.ok) throw new Error("fragment: HTTP " + resp.status);
      return resp.text().then(function (html) {
        if (seq !== fragmentSeq) return; // superseded by a newer refresh
        if (interactionLive()) {
          heldRefresh = true; // never yank the wall out from under the hand
          return;
        }
        applyFragment(html);
      });
    });
  }

  // resumeHeldRefresh runs at every interaction end: when a refresh was
  // held mid-gesture, re-fetch (never re-apply the held bytes — a fresh
  // fetch cannot be stale). A gesture that ended in its own mutation
  // already cleared the flag: that mutation's refresh supersedes.
  function resumeHeldRefresh() {
    if (!heldRefresh || interactionLive()) return;
    heldRefresh = false;
    refreshFragment().catch(function () {
      // The wall keeps its current render; the next mutation refreshes.
    });
  }

  // mutate: POST, remember dirtiness, swap the fresh projection in, and
  // only then report "saved" — the signal the autosave contract binds.
  function mutate(action, body) {
    setStatus("saving…");
    heldRefresh = false; // this mutation's own refresh supersedes a held one
    return api(action, body)
      .then(function (data) {
        if (typeof data.dirty === "boolean") state.git.dirty = data.dirty;
        return refreshFragment();
      })
      .then(function () {
        setStatus("saved");
      })
      .catch(function (err) {
        setStatus("error: " + err.message);
        // Reconcile: the wall must never keep showing a state the server
        // refused (a dragged card at a refused position is a lie).
        refreshFragment().catch(function () {
          // The error status above stays; the next mutation refreshes.
        });
      });
  }

  // -- yarn: SVG threads under HTML chips ----------------------------------

  var svgNS = "http://www.w3.org/2000/svg";

  function endpointElement(key) {
    var c = canvas();
    if (!c) return null;
    // "spec" is the document itself — not a card; it lives above the
    // canvas (the placards header), so its edges hang off-board.
    if (key === "spec") return null;
    // A scoping edge's From is the stub card's own "stub:<slug>" key
    // (the same key its stored position lives under): the coverage/
    // resolution thread ties to the stub's kraft paper.
    if (key.indexOf("stub:") === 0) {
      return c.querySelector('.stubcard[data-stub="' + esc(key.slice(5)) + '"]');
    }
    // A relates endpoint may name a live sticky by annotation id (round
    // 5.4): the attribution thread ties to the proto-sticky's paper.
    return (
      c.querySelector('.objcard[data-id="' + esc(key) + '"]') ||
      c.querySelector('.refcard[data-ref="' + esc(key) + '"]') ||
      c.querySelector('.sticky[data-id="' + esc(key) + '"]')
    );
  }

  function rectOf(el) {
    return {
      x: el.offsetLeft,
      y: el.offsetTop,
      w: el.offsetWidth,
      h: el.offsetHeight,
    };
  }

  function centerOf(el) {
    var r = rectOf(el);
    return { x: r.x + r.w / 2, y: r.y + r.h / 2 };
  }

  // edgeAnchor: the point where the ray from a card's center toward
  // `toward` crosses the card's border — threads tie to card EDGES, so
  // yarn never runs through a card's text (it reads as strikethrough).
  function edgeAnchor(el, toward) {
    var r = rectOf(el);
    var cx = r.x + r.w / 2;
    var cy = r.y + r.h / 2;
    var dx = toward.x - cx;
    var dy = toward.y - cy;
    if (dx === 0 && dy === 0) return { x: cx, y: cy };
    var sx = dx !== 0 ? r.w / 2 / Math.abs(dx) : Infinity;
    var sy = dy !== 0 ? r.h / 2 / Math.abs(dy) : Infinity;
    var t = Math.min(sx, sy);
    return { x: cx + dx * t, y: cy + dy * t };
  }

  function ensureYarnSvg() {
    var c = canvas();
    if (!c) return null;
    var svg = c.querySelector("svg.yarn-svg");
    if (!svg) {
      svg = document.createElementNS(svgNS, "svg");
      svg.setAttribute("class", "yarn-svg");
      svg.setAttribute("aria-hidden", "true");
      c.insertBefore(svg, c.firstChild);
    }
    return svg;
  }

  // layoutYarn draws every chip's thread — anchored edge-to-edge, sagging
  // like real yarn, layered UNDER the papers so an in-between card is
  // passed behind, never struck through — and sits the chip on the sag's
  // midpoint (open canvas between the cards). An edge with an off-board
  // endpoint (From:"spec": the document itself, hanging above the canvas
  // as the placards header) gets an off-board thread instead — entering
  // past the canvas's top edge, where the SVG clips it, so the yarn reads
  // as running up to the document — tied to its one on-board endpoint.
  // Every edge stays visible, never dropped, and never a bare chip.
  var OFFBOARD_Y = -24; // thread origin above the canvas (clipped at 0)

  function layoutYarn() {
    var c = canvas();
    if (!c) return;
    var svg = ensureYarnSvg();
    while (svg.firstChild) svg.removeChild(svg.firstChild);

    var chips = c.querySelectorAll(".yarn-chip");
    var papers = c.querySelectorAll(".objcard, .refcard, .stubcard");
    var offboardCount = 0; // bow alternation + margin slots
    var offboardTies = {}; // fan-out of several document edges on one card
    var placedChips = []; // chips seated earlier this pass: labels never bury labels
    for (var i = 0; i < chips.length; i++) {
      var chip = chips[i];
      var fromEl = endpointElement(chip.getAttribute("data-from"));
      var toEl = endpointElement(chip.getAttribute("data-to"));
      var offboard = !fromEl || !toEl;
      var a, b, cx, cy, knots;
      if (!offboard) {
        a = edgeAnchor(fromEl, centerOf(toEl));
        b = edgeAnchor(toEl, centerOf(fromEl));
        var dx = b.x - a.x;
        var dy = b.y - a.y;
        var sag = 8 + Math.sqrt(dx * dx + dy * dy) * 0.06;
        cx = (a.x + b.x) / 2;
        cy = (a.y + b.y) / 2 + sag;
        knots = [a, b];
      } else {
        var anchorEl = fromEl || toEl;
        if (anchorEl) {
          // One on-board endpoint: the thread hangs from above it and
          // ties to its top edge. Several document edges on one card fan
          // out along that edge instead of overlapping.
          var tieKey = keyOfElement(anchorEl);
          var tie = offboardTies[tieKey] || 0;
          offboardTies[tieKey] = tie + 1;
          a = { x: centerOf(anchorEl).x + tie * 26, y: OFFBOARD_Y };
          b = edgeAnchor(anchorEl, a);
          knots = [b];
        } else {
          // No on-board endpoint at all — impossible by construction (the
          // projection renders a reference card for every non-document
          // endpoint), but the visible-never-dropped guarantee survives
          // as the same designed state: a short thread hung from the top
          // margin, the chip riding its end.
          a = { x: 40 + offboardCount * 180, y: OFFBOARD_Y };
          b = { x: a.x, y: 56 };
          knots = [];
        }
        // A gentle lateral bow — hung yarn, not a plumb line.
        var bow = (offboardCount % 2 === 0 ? 1 : -1) * Math.min(24, 6 + (b.y - a.y) * 0.05);
        cx = (a.x + b.x) / 2 + bow;
        cy = (a.y + b.y) / 2;
        offboardCount++;
      }

      // The thread carries its TYPE as well as its layer: yarn color is
      // the relationship's meaning (the rail's yarn key teaches it), so
      // the thread and its knots must be paintable per edge type.
      var edgeType = chip.getAttribute("data-edge-type");
      var path = document.createElementNS(svgNS, "path");
      path.setAttribute(
        "class",
        "yarn-thread yarn-thread--" + chip.getAttribute("data-layer") +
          " yarn-thread--type-" + edgeType +
          (offboard ? " yarn-thread--offboard" : "")
      );
      path.setAttribute(
        "d",
        "M " + a.x + " " + a.y + " Q " + cx + " " + cy + " " + b.x + " " + b.y
      );
      svg.appendChild(path);
      for (var j = 0; j < knots.length; j++) {
        var knot = document.createElementNS(svgNS, "circle");
        // The knot carries the layer too: a scoping "resolves" pin must
        // never wear the spec layer's committed resolves ink.
        knot.setAttribute(
          "class",
          "yarn-knot yarn-knot--" + chip.getAttribute("data-layer") +
            " yarn-knot--type-" + edgeType
        );
        knot.setAttribute("cx", knots[j].x);
        knot.setAttribute("cy", knots[j].y);
        knot.setAttribute("r", 3.2);
        svg.appendChild(knot);
      }

      // The chip rides the curve — at the midpoint when that's open
      // canvas, otherwise slid along the thread to the first spot clear
      // of every card's interior (the chip sits ON the yarn, never over
      // anyone's text).
      var w = chip.offsetWidth;
      var h = chip.offsetHeight;
      // Blocked space for the chip: every card's interior AND its
      // pushpin (the yarn handle protrudes above the card's top edge,
      // and chips paint over cards — a chip parked on a pin makes yarn
      // undrawable from that card) AND every chip already seated this
      // pass (two threads sharing a corridor — e.g. a scoping resolves
      // beside a decision's exempts — must not stack their labels;
      // DOM order seats them, so the layout stays deterministic).
      var clearOfCards = function (x, y) {
        for (var k = 0; k < papers.length; k++) {
          var r = rectOf(papers[k]);
          if (x < r.x + r.w && r.x < x + w && y < r.y + r.h && r.y < y + h) {
            return false;
          }
          var pinX = r.x + r.w / 2 - 14;
          var pinY = r.y - 18;
          if (x < pinX + 28 && pinX < x + w && y < pinY + 22 && pinY < y + h) {
            return false;
          }
        }
        for (var q = 0; q < placedChips.length; q++) {
          var pc = placedChips[q];
          if (x < pc.x + pc.w && pc.x < x + w && y < pc.y + pc.h && pc.y < y + h) {
            return false;
          }
        }
        return true;
      };
      var pointAt = function (t) {
        var u = 1 - t;
        return {
          x: u * u * a.x + 2 * u * t * cx + t * t * b.x,
          y: u * u * a.y + 2 * u * t * cy + t * t * b.y,
        };
      };
      var ts = [0.5, 0.42, 0.58, 0.34, 0.66, 0.26, 0.74, 0.18, 0.82];
      var spot = pointAt(0.5);
      var seated = false;
      for (var m = 0; m < ts.length; m++) {
        var p = pointAt(ts[m]);
        if (clearOfCards(p.x - w / 2, p.y - h / 2)) {
          spot = p;
          seated = true;
          break;
        }
      }
      // A corridor narrower than the chip — adjacent columns, e.g. a
      // spike stub filed right beside the open question it claims — has
      // no clear on-thread seat. Rather than lie over a card's text (or
      // another chip's label), the chip slides DOWN the sag, hanging
      // beneath the papers like a tag on the yarn. Fixed candidate
      // order: the layout stays deterministic.
      if (!seated) {
        for (var dy = 18; dy <= 126 && !seated; dy += 18) {
          for (var m2 = 0; m2 < ts.length; m2++) {
            var p2 = pointAt(ts[m2]);
            if (clearOfCards(p2.x - w / 2, p2.y + dy - h / 2)) {
              spot = { x: p2.x, y: p2.y + dy };
              seated = true;
              break;
            }
          }
        }
      }
      var top = spot.y - h / 2;
      // A short off-board thread's midpoint can sit above the canvas;
      // the chip stays fully on the board (the thread is what exits).
      if (offboard && top < 4) top = 4;
      chip.style.left = spot.x - w / 2 + "px";
      chip.style.top = top + "px";
      placedChips.push({ x: spot.x - w / 2, y: top, w: w, h: h });
    }
  }

  // -- click-to-expand: truncation made visible (owner directive) -----------
  //
  // The wall clamps text to keep every paper a bounded footprint (a
  // case-file placard to three lines, an object card and a stub to their
  // index-card size). When a clamp actually cuts text, that must be
  // visible, not silent: the element gets `.is-clamped` (a fade on its
  // last line + a zoom-in cursor) and a quiet "⋯" mark in its corner, and
  // a click opens the read-only expand dialog. The affordance appears
  // ONLY when the text measurably overflows — a short placard stays crisp
  // and inert. Measured on the SERVER-RENDERED text (the DOM always holds
  // the full string; the clamp only hides it), so it re-runs after every
  // fragment swap, on load (web fonts change wrapping), and on resize.
  // The mark lives in the element's parent (never inside the clamped box,
  // where it would perturb -webkit-line-clamp or leak into the full text
  // the dialog reads back).

  var EXPANDABLE_SELECTOR =
    ".placard > p, .objcard .card-text, .stubcard .stub-title, .sticky:not(.sticky-draft) .sticky-body";

  function setClampMark(el, on) {
    var parent = el.parentNode;
    if (!parent) return;
    var mark = null;
    for (var i = 0; i < parent.children.length; i++) {
      if (parent.children[i].classList.contains("clamp-more")) {
        mark = parent.children[i];
        break;
      }
    }
    if (on && !mark) {
      mark = document.createElement("span");
      mark.className = "clamp-more";
      mark.setAttribute("aria-hidden", "true");
      parent.appendChild(mark);
    } else if (!on && mark) {
      mark.remove();
    }
  }

  function markClamped() {
    if (!region) return;
    var els = region.querySelectorAll(EXPANDABLE_SELECTOR);
    for (var i = 0; i < els.length; i++) {
      var el = els[i];
      // A vertical clamp overflows in height; the ref/nowrap cases carry
      // their own ellipsis and (for a reference card) the peek, so only
      // the multi-line papers are measured here.
      var truncated = el.scrollHeight - el.clientHeight > 1;
      el.classList.toggle("is-clamped", truncated);
      if (truncated) el.setAttribute("data-expandable", "");
      else el.removeAttribute("data-expandable");
      setClampMark(el, truncated);
    }
    markPlacards();
  }

  // markPlacards gives the case-file placards their ALWAYS-PRESENT expand
  // affordance — the quiet dog-ear (setPlacardMore) that says "there is a
  // fuller file to open" — independent of whether the headline currently
  // clamps. That is the whole point of the board-polish pass: a placard is
  // expandable when there is anything more to read than the three lines on
  // its face, i.e. it carries a server-rendered body section OR its headline
  // is actually clamped. The one degenerate case — no body section AND a
  // headline that fits — has nothing more to show, so it gets no affordance
  // and stays inert (the object-card/stub clamp affordance, by contrast, is
  // untouched: still purely clamp-triggered, above). Runs inside the same
  // measure pass, after the headline's own is-clamped state is set.
  function markPlacards() {
    var placards = region.querySelectorAll(".placard");
    for (var i = 0; i < placards.length; i++) {
      var pl = placards[i];
      var headline = pl.querySelector(".placard-text");
      var hasBody = !!pl.querySelector(".placard-full");
      var clamps = !!(headline && headline.classList.contains("is-clamped"));
      var expandable = hasBody || clamps;
      pl.classList.toggle("placard--expandable", expandable);
      setPlacardMore(pl, expandable);
    }
  }

  // setPlacardMore adds or removes a placard's dog-ear button. It is a real
  // button (keyboard-focusable, screen-reader labelled with what it opens),
  // its fold drawn in CSS; the whole placard is also click-to-open, so the
  // dog-ear is the visible promise, not the only way in.
  function setPlacardMore(pl, on) {
    var btn = pl.querySelector(".placard-more");
    if (on && !btn) {
      btn = document.createElement("button");
      btn.type = "button";
      btn.className = "placard-more";
      var which = pl.classList.contains("placard--outcome") ? "outcome" : "problem";
      btn.setAttribute("aria-label", "Read the full " + which);
      btn.setAttribute("aria-haspopup", "dialog");
      pl.appendChild(btn);
    } else if (!on && btn) {
      btn.remove();
    }
  }

  // expandHeaderFor names the element in its own eyebrow voice — the
  // dialog's header (owner directive: "the element's kind + id"). A
  // placard is its tag; an object card its id; a stub its slug.
  function expandHeaderFor(el) {
    var placard = el.closest(".placard");
    if (placard) {
      return placard.classList.contains("placard--outcome") ? "OUTCOME" : "PROBLEM";
    }
    var card = el.closest(".objcard");
    if (card) {
      var kind = (card.getAttribute("data-object-kind") || "").replace(/-/g, " ");
      var id = card.getAttribute("data-id") || "";
      return kind ? kind + " · " + id : id;
    }
    var stub = el.closest(".stubcard");
    if (stub) return stub.getAttribute("data-stub") || "stub";
    var sticky = el.closest(".sticky");
    if (sticky) return sticky.getAttribute("data-annotation-type") || "sticky";
    return "";
  }

  // A scratch element (a sticky) reads back in the hand it was written in;
  // everything else is the spec register's serif.
  function expandIsHand(el) {
    return !!el.closest(".sticky");
  }

  function closeExpandDialog() {
    var d = document.getElementById("expand-dialog");
    var b = document.getElementById("expand-backdrop");
    if (d) d.remove();
    if (b) b.remove();
  }

  // buildExpandDialog frames the read-only dialog (the shared board-dialog
  // chrome: × / scrim / Escape) and returns its empty body container for
  // the caller to fill — with plain text (a card/stub/sticky, or a placard
  // with no body section) or with rendered prose (a placard's server-side
  // HTML body). `rich` swaps the plain pre-wrap body for prose that lays
  // out paragraphs and lists; `hand` keeps a scratch element in its own
  // handwriting.
  function buildExpandDialog(header, rich, hand) {
    closeExpandDialog();
    var backdrop = document.createElement("div");
    backdrop.className = "modal-backdrop expand-backdrop";
    backdrop.id = "expand-backdrop";

    var dlg = document.createElement("div");
    dlg.id = "expand-dialog";
    dlg.className =
      "board-dialog expand-dialog" +
      (hand ? " expand-dialog--hand" : "") +
      (rich ? " expand-dialog--rich" : "");
    dlg.setAttribute("data-testid", "expand-dialog");
    dlg.setAttribute("role", "dialog");
    dlg.setAttribute("aria-modal", "true");
    dlg.setAttribute("aria-label", header);
    dlg.setAttribute("aria-labelledby", "expand-dialog-kind");

    var close = document.createElement("button");
    close.type = "button";
    close.id = "expand-close";
    close.className = "expand-close";
    close.setAttribute("aria-label", "Close");
    close.textContent = "×"; // ×

    var kind = document.createElement("h2");
    kind.id = "expand-dialog-kind";
    kind.className = "expand-kind";
    kind.textContent = header;

    var body = document.createElement("div");
    body.className = "expand-text" + (rich ? " expand-text--rich" : "");
    body.setAttribute("data-testid", "expand-text");

    dlg.appendChild(close);
    dlg.appendChild(kind);
    dlg.appendChild(body);
    document.body.appendChild(backdrop);
    document.body.appendChild(dlg);
    close.focus();
    return body;
  }

  function openExpandDialog(el) {
    var body = buildExpandDialog(expandHeaderFor(el), false, expandIsHand(el));
    // The DOM holds the full string; the clamp only hid it. Reading
    // textContent (not innerHTML) keeps this strictly read-only text.
    body.textContent = el.textContent;
  }

  // openPlacardExpand reads a case-file placard's FULL prose. When the spec
  // carried a "## Problem"/"## Outcome" body section, the server rendered it
  // (already escaped, through the same markdown path the corpus page uses)
  // into a hidden `.placard-full` sibling of the headline — the dialog shows
  // that body as laid-out prose. When there is no body section, there is
  // nothing fuller to reveal than the headline itself (which the wall may
  // have clamped), so the dialog falls back to the headline text.
  function openPlacardExpand(placard) {
    var header = expandHeaderFor(placard);
    var full = placard.querySelector(".placard-full");
    if (full) {
      var richBody = buildExpandDialog(header, true, false);
      // The body is trusted, server-rendered HTML (goldmark output from the
      // committed spec, never user input) — the same bytes the corpus page
      // injects. Surfacing it as markup is the point of the seam.
      richBody.innerHTML = full.innerHTML;
      // A body section may carry a badged mermaid figure (the fenced
      // illustrative register) — same pinned renderer as every surface.
      renderMermaidIn(richBody);
      return;
    }
    var headline = placard.querySelector(".placard-text");
    var body = buildExpandDialog(header, false, false);
    body.textContent = headline ? headline.textContent : "";
  }

  // A single click opens the dialog, but a card is also double-click-to-
  // edit in authoring: the open is deferred a beat and CANCELLED by the
  // dblclick handler, so the second click never lands on an expand dialog
  // and editing wins. A drag (past the slop) is a drag — its click tail is
  // guarded out below.
  var EXPAND_DELAY = 250;
  var expandTimer = null;
  function scheduleExpand(el) {
    if (expandTimer) clearTimeout(expandTimer);
    expandTimer = setTimeout(function () {
      expandTimer = null;
      openExpandDialog(el);
    }, EXPAND_DELAY);
  }
  function cancelExpand() {
    if (expandTimer) {
      clearTimeout(expandTimer);
      expandTimer = null;
    }
  }

  // -- derivation drawer: open, position, close — NOTHING else --------------
  //
  // Every wall badge is a button whose drawer body the server already
  // rendered as its hidden sibling (spec/derivation-drawer dc-1 — the
  // writePlacardFull idiom). This code never reads or templates the
  // derivation data: opening moves the server's own element to the body
  // (so no card clipping can cut a receipt off) behind a scrim, closing
  // puts the untouched element back where the server rendered it, and a
  // comment marker remembers that exact slot. Reading a receipt is never
  // a write, so this works identically in every board mode (dc-4).
  var openDrawer = null; // {el, marker, opener}

  function openBadgeDrawer(btn) {
    closeBadgeDrawer();
    var drawer = btn.nextElementSibling;
    if (!drawer || !drawer.classList || !drawer.classList.contains("badge-drawer")) return;
    var backdrop = document.createElement("div");
    backdrop.className = "modal-backdrop drawer-backdrop";
    backdrop.id = "drawer-backdrop";
    var marker = document.createComment("badge-drawer-home");
    drawer.parentNode.insertBefore(marker, drawer);
    document.body.appendChild(backdrop);
    document.body.appendChild(drawer);
    drawer.hidden = false;
    openDrawer = { el: drawer, marker: marker, opener: btn };
    // Focus moves into the dialog on open and back to the opener on
    // close (dc-4) — the close control is the drawer's one affordance.
    var close = drawer.querySelector(".drawer-close");
    if (close) close.focus();
  }

  function closeBadgeDrawer() {
    if (!openDrawer) return;
    var st = openDrawer;
    openDrawer = null;
    st.el.hidden = true;
    if (st.marker && st.marker.parentNode) {
      st.marker.parentNode.insertBefore(st.el, st.marker);
      st.marker.parentNode.removeChild(st.marker);
    }
    var bd = document.getElementById("drawer-backdrop");
    if (bd) bd.remove();
    if (st.opener && document.contains(st.opener)) st.opener.focus();
  }

  // -- dialogs ---------------------------------------------------------------

  function show(id) {
    var el = document.getElementById(id);
    if (el) el.hidden = false;
    var bd = document.getElementById("modal-backdrop");
    if (bd) bd.hidden = false;
  }
  function hide(id) {
    var el = document.getElementById(id);
    if (el) el.hidden = true;
  }
  function hideAllDialogs() {
    ["edge-picker", "edge-confirm", "commit-dialog", "branch-guard", "graduate-menu", "branch-menu"].forEach(hide);
    var bd = document.getElementById("modal-backdrop");
    if (bd) bd.hidden = true;
  }

  // pending carries the gesture the picker/confirm is deciding about.
  var pending = null; // {from, fromKind, to, toKind, annotationId, type}

  function kindOfElement(el) {
    if (el.hasAttribute("data-object-kind")) return el.getAttribute("data-object-kind");
    if (el.hasAttribute("data-ref-kind")) return el.getAttribute("data-ref-kind");
    return "unknown";
  }
  function keyOfElement(el) {
    return el.getAttribute("data-id") || el.getAttribute("data-ref");
  }

  // The picker speaks human: kind names as a PM reads them — an article
  // form and a plural form, for the no-typed-edge explanation.
  var kindNames = {
    "acceptance-criterion": ["an acceptance criterion", "acceptance criteria"],
    constraint: ["a constraint", "constraints"],
    decision: ["a decision", "decisions"],
    "open-question": ["an open question", "open questions"],
    adr: ["an ADR", "ADRs"],
    spec: ["a spec", "specs"],
    "spec-fragment": ["a spec fragment", "spec fragments"],
    diagram: ["a diagram", "diagrams"],
  };
  function pairPhrase(a, b) {
    var an = kindNames[a] || ["a " + a, a + "s"];
    var bn = kindNames[b] || ["a " + b, b + "s"];
    return a === b ? "two " + an[1] : an[0] + " and " + bn[0];
  }

  // openPicker fills the context-sensitive menu: ONLY the pair's legal
  // types (each with its one-line consequence), plus the scratch tier's
  // untyped thread. When NO typed edge is legal for the pair, the picker
  // says so in plain language instead of presenting a menu of nothing
  // (owner UAT round 6, item 1).
  function openPicker(p) {
    pending = p;
    var items = document.getElementById("edge-picker-items");
    var pair = document.getElementById("edge-picker-pair");
    if (!items) return;
    items.innerHTML = "";
    if (pair) pair.textContent = p.from + " → " + p.to;

    var legal = (state.legal[p.fromKind + "|" + p.toKind] || []).slice();
    if (p.retype) {
      // In-place retype (owner directive): offer the OTHER legal types.
      legal = legal.filter(function (t) {
        return t !== p.retype;
      });
      if (pair) pair.textContent += " · currently " + p.retype;
    }
    if (legal.length === 0) {
      var note = document.createElement("p");
      note.className = "ritual-note picker-empty-note";
      note.setAttribute("data-testid", "picker-no-typed-edge");
      note.textContent = p.retype
        ? "No other typed edge is legal between " + pairPhrase(p.fromKind, p.toKind) + " — remove the edge (×) and draw a new connection instead."
        : "No typed edge exists between " + pairPhrase(p.fromKind, p.toKind) +
          (p.annotationId
            ? " — this stays a scratch thread. To type a connection, draw it from a decision card."
            : " — this can only be a scratch thread.");
      items.appendChild(note);
    }

    var types = legal;
    if (!p.annotationId && !p.retype) types.push("relates");
    types.forEach(function (t) {
      var row = document.createElement("div");
      row.className = "picker-item";
      var btn = document.createElement("button");
      btn.type = "button";
      btn.setAttribute("role", "menuitem");
      btn.setAttribute("data-edge-choice", t);
      btn.textContent = t === "relates" ? "relates (scratch)" : t;
      row.appendChild(btn);
      var why = document.createElement("span");
      why.className = "consequence";
      why.setAttribute("data-testid", "consequence-" + t);
      why.textContent = state.consequences[t] || "";
      row.appendChild(why);
      items.appendChild(row);
    });
    show("edge-picker");
  }

  // openConfirm arms the gate-bearing confirmation dialog — the same
  // ritual for creating, retyping, and removing (a menu misclick must
  // not summon an org-wide supersession flow, 05 §Workbench).
  function openConfirm(title, consequence, withReason) {
    var confirmEl = document.getElementById("edge-confirm");
    confirmEl.setAttribute("aria-label", title);
    document.getElementById("edge-confirm-title").textContent = title;
    document.getElementById("edge-confirm-consequence").textContent = consequence;
    document.getElementById("edge-confirm-ok").hidden = false; // a refusal may have hidden it
    var reasonField = document.getElementById("edge-confirm-reason-field");
    document.getElementById("edge-confirm-reason").value = "";
    reasonField.hidden = !withReason;
    show("edge-confirm");
  }

  function pickEdgeType(t) {
    hide("edge-picker");
    if (t === "relates") {
      hideAllDialogs();
      mutate("relates", { from: pending.from, to: pending.to });
      return;
    }
    if (state.gate.indexOf(t) >= 0) {
      pending.type = t;
      // The reason field feeds a NEW exempts edge's note; a retype keeps
      // the existing note verbatim.
      openConfirm("Confirm " + t, state.consequences[t] || "", t === "exempts" && !pending.retype);
      return;
    }
    commitEdge(t, "");
  }

  function commitEdge(t, note) {
    hideAllDialogs();
    if (pending.retype) {
      mutate("edge-retype", { from: pending.from, to: pending.to, type: pending.retype, newType: t });
    } else if (pending.annotationId) {
      mutate("relates-graduate", { id: pending.annotationId, type: t, note: note });
    } else {
      mutate("edge", { from: pending.from, to: pending.to, type: t, note: note });
    }
  }

  // -- proto-sticky attribution yarn (the scoping canvas, dc-5) --------------
  //
  // A story sticky's thread to an acceptance criterion is the coverage
  // claim; a spike sticky's thread to an open question is the resolution
  // attribution. Each pair has exactly ONE reading, so there is no picker
  // ceremony: a legal drop confirms what the thread means and mints the
  // untyped relates record directly; an illegal pair gets the picker's
  // plain-language refusal (the endpoint pair IS the claim).

  function protoRefusal(fromLabel, toLabel, message) {
    pending = null;
    var items = document.getElementById("edge-picker-items");
    var pair = document.getElementById("edge-picker-pair");
    if (!items) return;
    items.innerHTML = "";
    if (pair) pair.textContent = fromLabel + " → " + toLabel;
    var note = document.createElement("p");
    note.className = "ritual-note picker-empty-note";
    note.setAttribute("data-testid", "proto-yarn-refusal");
    note.textContent = message;
    items.appendChild(note);
    show("edge-picker");
  }

  function routeProtoYarn(g, target) {
    var toKind = kindOfElement(target);
    var to = keyOfElement(target);
    var story = g.proto === "story";
    var wantKind = story ? "acceptance-criterion" : "open-question";

    // The dialog copy's story/spike words are display prose and resolve
    // (classWord); g.proto — the sticky's TYPE id — stays the bare enum
    // value everywhere it addresses (data reads, the api() payloads).
    if (toKind !== wantKind) {
      var refusal;
      if (story && toKind === "open-question") {
        refusal =
          classArticleCap("story") + " " + classWord("story") + " sticky's thread claims coverage — it ties only to acceptance criteria. " +
          "If this thought answers open questions, it wants to be " + classArticle("spike") + " " + classWord("spike") + " sticky instead.";
      } else if (!story && toKind === "acceptance-criterion") {
        refusal =
          classArticleCap("spike") + " " + classWord("spike") + " sticky's thread claims an answer — it ties only to open questions. " +
          "If this thought delivers an acceptance criterion, it wants to be " + classArticle("story") + " " + classWord("story") + " sticky instead.";
      } else {
        refusal =
          classArticleCap(g.proto) + " " + classWord(g.proto) + " sticky's thread has one meaning: " +
          (story
            ? "coverage of an acceptance criterion."
            : "resolution of an open question.") +
          " It has nothing to say to " + pairPhrase(toKind, toKind).replace(/^two /, "") + ".";
      }
      protoRefusal(classWord(g.proto) + " sticky", to, refusal);
      return;
    }

    pending = { protoRelate: { from: g.from, to: to } };
    if (story) {
      openConfirm(
        "Claim coverage of " + to,
        "Ties this " + classWord("story") + " sticky to " + to + ". The thread is the coverage claim: " +
          "when the sticky graduates into a stub, " + to + " joins its declared acceptance criteria.",
        false
      );
    } else {
      openConfirm(
        "Claim resolution of " + to,
        "Ties this " + classWord("spike") + " sticky to " + to + ". The thread is the attribution: " +
          "when the sticky graduates into " + classArticle("spike") + " " + classWord("spike") + " stub, " + to + " joins the questions it resolves.",
        false
      );
    }
  }

  // -- obligation authoring (spec/obligation-artifact ac-3) ------------------
  //
  // The story wall's counterpart to the feature wall's scoping canvas: a
  // scratch sticky's yarn dropped on a story acceptance criterion authors
  // that AC's evidence obligation. The endpoint must be an AC card
  // (obligations attach to STORY acceptance criteria only, 03 §The feature
  // fold); any other target gets the picker's plain-language refusal. A
  // legal drop opens the evidence-kind picker — the one choice the
  // (sticky, AC) pair leaves open — and choosing a kind graduates the
  // sticky into the obligation, seeding its verifies edge (→ the whole
  // story spec) and its for_kind.

  var FOR_KINDS = [
    ["static", "a code-fact the store can prove"],
    ["behavioral", "a test the suite runs"],
    ["runtime", "a probe against a live surface"],
    ["attestation", "a human sign-off on file"],
  ];

  function routeObligationYarn(g, target) {
    var toKind = kindOfElement(target);
    var to = keyOfElement(target);
    if (toKind !== "acceptance-criterion") {
      protoRefusal(
        "obligation",
        to,
        "An obligation binds to " + classArticle("story") + " " + classWord("story") + " acceptance criterion — this thread lands on " +
          to + ", which is not one. Draw it to an AC card instead."
      );
      return;
    }
    openForKindPicker(g.from, to);
  }

  // openForKindPicker offers the four evidence kinds an obligation can be
  // FOR (spec/obligation-artifact DC-1), reusing the yarn picker's own
  // dialog: choosing is part of authoring, nothing defaults silently
  // (the same posture "Add sticky" wears).
  function openForKindPicker(stickyID, acID) {
    pending = { obligationGraduate: { sticky: stickyID, ac: acID } };
    var items = document.getElementById("edge-picker-items");
    var pair = document.getElementById("edge-picker-pair");
    if (!items) return;
    items.innerHTML = "";
    if (pair) pair.textContent = "obligation → " + acID;
    FOR_KINDS.forEach(function (fk) {
      var row = document.createElement("div");
      row.className = "picker-item";
      var btn = document.createElement("button");
      btn.type = "button";
      btn.setAttribute("role", "menuitem");
      btn.setAttribute("data-forkind", fk[0]);
      btn.textContent = fk[0];
      row.appendChild(btn);
      var why = document.createElement("span");
      why.className = "consequence";
      why.setAttribute("data-testid", "for-kind-" + fk[0]);
      why.textContent = fk[1];
      row.appendChild(why);
      items.appendChild(row);
    });
    show("edge-picker");
  }

  // pickForKind graduates the sticky into its obligation. The server's
  // refusals — a missing AC, a non-story wall, an obligation that already
  // exists — come back in plain language and surface in the confirm
  // dialog's own voice (the same posture stub-graduate wears), never a raw
  // toast.
  function pickForKind(forKind) {
    hide("edge-picker");
    if (!pending || !pending.obligationGraduate) return;
    var og = pending.obligationGraduate;
    pending = null;
    setStatus("saving…");
    api("sticky-graduate", { id: og.sticky, ref: og.ac, kind: "obligation:" + forKind })
      .then(function (data) {
        if (typeof data.dirty === "boolean") state.git.dirty = data.dirty;
        return refreshFragment().then(function () {
          setStatus("saved");
        });
      })
      .catch(function (err) {
        setStatus("");
        openConfirm("Not yet an obligation", err.message, false);
        document.getElementById("edge-confirm-ok").hidden = true;
      });
  }

  // -- pointer gestures: drag, yarn draw ------------------------------------
  //
  // Pointer Events, not mouse events: the e2e suite's synthetic mouse
  // stream masked three real-input gaps — (a) touch/pen drags never fire
  // mouse events, so the board was undraggable on any touch device;
  // (b) without pointer capture a release outside the window (or on a
  // native scrollbar) is lost, leaving a stuck gesture chasing a
  // button-up cursor and then committing a phantom position; (c) real
  // hands jitter a pixel or two inside a double-click, which the old
  // zero-threshold code turned into a drag-write that re-rendered the
  // fragment between the two clicks. Hence: setPointerCapture on the
  // dragged element, an e.buttons===0 guard that lands the drop where
  // the drag actually was, pointercancel reverting the gesture, and a
  // small slop threshold before a press becomes a drag. Selection and
  // scroll hijack are handled declaratively (style.css: user-select and
  // touch-action on the drag surfaces), so pointerdown is never
  // preventDefault-ed and click/dblclick semantics stay native.

  var DRAG_SLOP = 4; // px of pointer travel before a press is a drag

  var gesture = null; // {kind: "card"|"sticky"|"refcard"|"stub"|"chip"|"yarn", pointerId, ...}

  // A completed drag still fires a click on its element (press and
  // release land on the captured element); the reference card's click
  // affordance (the peek) must not fire off the tail of a drag.
  var justDragged = null;

  // A calm, visible refusal for a drag attempt on a board that will not
  // move (05 §Workbench: read-only is a document, review is a mirror) —
  // never a dead-silent immovable element. Spoken through the board's
  // existing notice channel, and transient: it names why, then leaves.
  var refusalTimer = null;
  function refuseDrag() {
    var container = region.querySelector(".board-notices");
    if (!container) {
      container = document.createElement("div");
      container.className = "board-notices";
      region.insertBefore(container, region.firstChild);
    }
    var note = container.querySelector(".board-notice--refusal");
    if (!note) {
      note = document.createElement("div");
      note.className = "board-notice board-notice--refusal";
      note.setAttribute("data-testid", "drag-refusal");
      note.setAttribute("role", "status");
      container.appendChild(note);
    }
    note.textContent =
      state.mode === "review"
        ? "this board mirrors the merge request under review — nothing moves here; reply on the MR or wait for the branch"
        : "positions are frozen with the accepted spec — change means supersession (the amendment ladder)";
    if (refusalTimer) clearTimeout(refusalTimer);
    refusalTimer = setTimeout(function () {
      note.remove();
    }, 6000);
  }

  function capturePointer(el, e) {
    if (el.setPointerCapture) {
      try {
        el.setPointerCapture(e.pointerId);
      } catch (err) {
        /* a vanished pointer: the move/up guards cover it */
      }
    }
  }

  function onPointerDown(e) {
    if (e.pointerType === "mouse" && e.button !== 0) return;
    if (gesture) return; // one gesture at a time
    var c = canvas();
    if (!c) return;

    var handle = e.target.closest(".yarn-handle");
    if (handle && c.contains(handle)) {
      if (!authoring) return;
      // The pin on a story/spike proto-sticky draws the ATTRIBUTION
      // thread (dc-5): same gesture, but the drop resolves against the
      // endpoint-pair table instead of the type picker.
      var protoSticky = handle.closest(".sticky");
      if (protoSticky) {
        gesture = {
          kind: "yarn",
          proto: protoSticky.getAttribute("data-annotation-type"),
          pointerId: e.pointerId,
          fromEl: protoSticky,
          from: protoSticky.getAttribute("data-id"),
          fromKind: "sticky",
        };
        capturePointer(handle, e);
        return;
      }
      var card = handle.closest(".objcard");
      gesture = {
        kind: "yarn",
        pointerId: e.pointerId,
        fromEl: card,
        from: card.getAttribute("data-id"),
        fromKind: card.getAttribute("data-object-kind"),
      };
      capturePointer(handle, e);
      return;
    }

    // A pin buried under a floating chip stays grabbable: between two
    // close cards a chip can be wider than the thread's whole span, so
    // it may legitimately park over a card's pushpin — and the grab,
    // like the drop, must resolve GEOMETRICALLY so a decoration never
    // deadens the affordance beneath it. The gesture is PROVISIONAL
    // (viaCover): no pointer capture and nothing happens until the
    // press travels past the drag slop, so an unmoved press stays the
    // chip's own click (its buttons keep working), while a real drag
    // from the buried pin becomes the yarn draw.
    var overChip = e.target.closest(".yarn-chip");
    if (authoring && overChip) {
      var canvasRect0 = c.getBoundingClientRect();
      var gx = e.clientX - canvasRect0.left + c.scrollLeft;
      var gy = e.clientY - canvasRect0.top + c.scrollTop;
      // Pin owners: object cards AND proto-stickies (their pins draw
      // attribution yarn) — any paper whose pushpin a chip may bury.
      var cards0 = c.querySelectorAll(".objcard, .sticky");
      for (var ci = 0; ci < cards0.length; ci++) {
        if (!cards0[ci].querySelector(".yarn-handle")) continue;
        var cr = rectOf(cards0[ci]);
        var pinCX = cr.x + cr.w / 2;
        if (gx >= pinCX - 8 && gx <= pinCX + 8 && gy >= cr.y - 8 && gy <= cr.y + 8) {
          var isSticky0 = cards0[ci].classList.contains("sticky");
          gesture = {
            kind: "yarn",
            proto: isSticky0 ? cards0[ci].getAttribute("data-annotation-type") : undefined,
            viaCover: true,
            moved: false,
            downX: e.clientX,
            downY: e.clientY,
            pointerId: e.pointerId,
            fromEl: cards0[ci],
            from: cards0[ci].getAttribute("data-id"),
            fromKind: isSticky0 ? "sticky" : cards0[ci].getAttribute("data-object-kind"),
          };
          return;
        }
      }
    }

    // A real link inside a card (the proposal-diagram ref card's
    // "open in editor" doorway, spec/board-editor dc-1) stays a link:
    // capturing the pointer here would retarget its click and swallow
    // the navigation.
    if (e.target.closest("a, button, textarea, input, .review-sticky")) return;

    var el = e.target.closest(".objcard, .sticky, .refcard, .stubcard, .yarn-chip--annotation");
    if (!el || !c.contains(el)) return;
    if (!authoring) {
      // A reference card outside authoring is a peek affordance, not a
      // frozen drag — no refusal theater on it. Chips likewise. A stub
      // card is a positioned paper like an object card: its refusal
      // speaks (round 5.5 dc-6 — stubs are movable, so a frozen wall
      // must say why this one is not).
      if (
        el.classList.contains("objcard") ||
        el.classList.contains("sticky") ||
        el.classList.contains("stubcard")
      ) {
        refuseDrag();
      }
      return;
    }
    var kind = "card";
    if (el.classList.contains("sticky")) kind = "sticky";
    else if (el.classList.contains("refcard")) kind = "refcard";
    else if (el.classList.contains("stubcard")) kind = "stub";
    else if (el.classList.contains("yarn-chip")) kind = "chip";
    var rect = el.getBoundingClientRect();
    gesture = {
      kind: kind,
      pointerId: e.pointerId,
      el: el,
      dx: e.clientX - rect.left,
      dy: e.clientY - rect.top,
      downX: e.clientX,
      downY: e.clientY,
      startLeft: el.style.left,
      startTop: el.style.top,
      moved: false,
    };
    capturePointer(el, e);
  }

  function onPointerMove(e) {
    if (!gesture || e.pointerId !== gesture.pointerId) return;
    // A mouse with no button down mid-gesture means the release happened
    // where the page could not see it: land the drop at the last dragged
    // position instead of chasing a button-up cursor.
    if (e.pointerType === "mouse" && e.buttons === 0) {
      finishGesture(e);
      return;
    }
    var c = canvas();
    if (!c) return;
    var canvasRect = c.getBoundingClientRect();

    if (gesture.kind === "yarn") {
      var svg = ensureYarnSvg();
      var old = svg.querySelector(".yarn-draft");
      if (old) svg.removeChild(old);
      var bx = e.clientX - canvasRect.left + c.scrollLeft;
      var by = e.clientY - canvasRect.top + c.scrollTop;
      var a = edgeAnchor(gesture.fromEl, { x: bx, y: by });
      var line = document.createElementNS(svgNS, "line");
      line.setAttribute("class", "yarn-thread yarn-draft");
      line.setAttribute("x1", a.x);
      line.setAttribute("y1", a.y);
      line.setAttribute("x2", bx);
      line.setAttribute("y2", by);
      svg.appendChild(line);
      return;
    }

    if (!gesture.moved) {
      var tx = e.clientX - gesture.downX;
      var ty = e.clientY - gesture.downY;
      if (tx * tx + ty * ty < DRAG_SLOP * DRAG_SLOP) return; // jitter, not a drag
      gesture.moved = true;
      gesture.el.classList.add("dragging");
    }
    var x = e.clientX - canvasRect.left - gesture.dx + c.scrollLeft;
    var y = e.clientY - canvasRect.top - gesture.dy + c.scrollTop;
    if (x < 0) x = 0;
    if (y < 0) y = 0;
    gesture.el.style.left = x + "px";
    gesture.el.style.top = y + "px";
    // A dragged chip rides the hand alone (layoutYarn would seat every
    // chip back on its thread, the dragged one included).
    if (gesture.kind !== "chip") layoutYarn();
    updateTrashState(e);
  }

  // -- the trash target (owner directive) ------------------------------------
  //
  // During any wall-element drag, nearing the screen's lower-right
  // raises the trash target; hovering it makes it unmistakably hot;
  // releasing there removes the element per tier (scratch dies without
  // ceremony; record removals confirm first, naming their edges). The
  // target never takes pointer events — the gesture code measures its
  // box — so it can never eat a drop meant for the wall.

  var TRASH_NEAR = 300; // px from the lower-right corner where it rises

  function trashEl() {
    return document.getElementById("board-trash");
  }
  function overTrash(e) {
    var tr = trashEl();
    if (!tr) return false;
    var r = tr.getBoundingClientRect();
    var pad = 14; // a forgiving halo: near-misses on a 4rem disc count
    return (
      e.clientX >= r.left - pad && e.clientX <= r.right + pad &&
      e.clientY >= r.top - pad && e.clientY <= r.bottom + pad
    );
  }
  function updateTrashState(e) {
    var tr = trashEl();
    if (!tr) return;
    var dx = window.innerWidth - e.clientX;
    var dy = window.innerHeight - e.clientY;
    var near = dx * dx + dy * dy < TRASH_NEAR * TRASH_NEAR;
    tr.classList.toggle("is-armed", near);
    tr.classList.toggle("is-hot", near && overTrash(e));
  }
  function hideTrash() {
    var tr = trashEl();
    if (tr) tr.classList.remove("is-armed", "is-hot");
  }

  // specEdgeChipsFor collects the spec-layer chips touching a ref — the
  // edges a trash confirmation must name.
  function specEdgeChipsFor(key) {
    var out = [];
    var chips = region.querySelectorAll('.yarn-chip[data-layer="spec"]');
    for (var i = 0; i < chips.length; i++) {
      var from = chips[i].getAttribute("data-from");
      var to = chips[i].getAttribute("data-to");
      if (from === key || to === key) out.push({ from: from, to: to, type: chips[i].getAttribute("data-edge-type") });
    }
    return out;
  }

  // gateRitualCopy appends the removal consequence for every gate-bearing
  // type among the named edges — the same ritual voice removal wears at
  // the chip's own × (a trash drop must not be a quieter path).
  function gateRitualCopy(edges) {
    var seen = {};
    var copy = "";
    for (var i = 0; i < edges.length; i++) {
      var t = edges[i].type;
      if (state.gate.indexOf(t) >= 0 && !seen[t]) {
        seen[t] = true;
        copy += " Removing " + t + ": " + (state.removals[t] || "") + ".";
      }
    }
    return copy;
  }

  // trashDrop routes a trash release per tier. Scratch dies without
  // ceremony; anything that would edit the spec document confirms first,
  // in plain language, naming what goes.
  function trashDrop(g) {
    if (g.kind === "stub") {
      // A declared stub is spec content — the stubs block in the
      // document's own frontmatter — and removing a stub from the wall
      // is not built yet. A designed refusal in plain language (the
      // paper already snapped home; nothing is written).
      var slug = g.el.getAttribute("data-stub");
      pending = null;
      openConfirm(
        "This stub stays",
        slug + " is a declared stub — spec content, in the document's own stubs block. " +
          "Removing a stub from the wall is not built yet; to retire it, edit the spec document itself.",
        false
      );
      document.getElementById("edge-confirm-ok").hidden = true;
      return;
    }
    if (g.kind === "sticky") {
      // Scratch dies without ceremony — and visibly NOW: the element
      // hides the moment the delete is posted, so the stale window
      // between POST and refresh never shows a dead-but-clickable ghost
      // (the owner-witnessed double-delete "no annotation …" race). A
      // refused delete reconciles it back via mutate's error refetch.
      g.el.style.visibility = "hidden";
      mutate("annotation-delete", { id: g.el.getAttribute("data-id") });
      return;
    }
    if (g.kind === "chip") {
      g.el.style.visibility = "hidden"; // same immediate acknowledgment
      mutate("annotation-delete", { id: g.el.getAttribute("data-annotation-id") });
      return;
    }
    if (g.kind === "refcard") {
      var ref = g.el.getAttribute("data-ref");
      var edges = specEdgeChipsFor(ref);
      var docHeld = edges.some(function (c) {
        return c.from === "spec";
      });
      if (docHeld) {
        // The card is held by the document's own links: block, which the
        // board cannot edit — a designed refusal, not a doomed confirm.
        openConfirm(
          "This card stays",
          ref + " is held by the spec document's own links: block (its implements/resolves edges), which the board cannot edit.",
          false
        );
        document.getElementById("edge-confirm-ok").hidden = true;
        return;
      }
      if (edges.length === 0) {
        // A pure pin (or a card held only by scratch threads): the
        // scratch tier's "or they die" — no ceremony.
        mutate("ref-trash", { ref: ref });
        return;
      }
      var names = edges
        .map(function (c) {
          return c.from + " " + c.type;
        })
        .join(", ");
      var msg =
        "Removes the " +
        (edges.length === 1 ? "typed relationship" : edges.length + " typed relationships") +
        " holding it here (" + names + ") from the spec document." +
        gateRitualCopy(edges);
      if (g.el.hasAttribute("data-pin-id")) {
        msg += " Its pin and scratch threads go with it.";
      }
      pending = { trashRef: ref };
      openConfirm("Take " + ref + " off the wall", msg, false);
      return;
    }
    // A declared object card: removing it removes its declaration from
    // the spec document plus every edge touching it — prose stays.
    var id = g.el.getAttribute("data-id");
    var edges2 = specEdgeChipsFor(id);
    var msg2 = "Removes " + id + " from the spec document";
    if (edges2.length > 0) {
      var names2 = edges2
        .map(function (c) {
          return (c.from === id ? "its " + c.type + " to " + c.to : c.from + " " + c.type);
        })
        .join(", ");
      msg2 += ", and the " + (edges2.length === 1 ? "edge" : edges2.length + " edges") + " touching it (" + names2 + ")";
    }
    msg2 += ". Its body prose stays in the document — the board never deletes prose." + gateRitualCopy(edges2);
    pending = { trashObject: id };
    openConfirm("Remove " + id + " from the spec", msg2, false);
  }

  // finishGesture ends the gesture and THEN resumes any refresh that was
  // held while it was live. Order matters: the branches below that end in
  // a mutation clear the held flag themselves (their own refresh
  // supersedes), so the resume only fires for gesture ends that wrote
  // nothing (a cancelled drop, a yarn draw opening its picker, a snap
  // home) — where the held projection would otherwise never arrive.
  function finishGesture(e) {
    try {
      finishGestureNow(e);
    } finally {
      resumeHeldRefresh();
    }
  }

  function finishGestureNow(e) {
    var g = gesture;
    gesture = null;

    if (g.kind === "yarn") {
      var svg = ensureYarnSvg();
      var draft = svg.querySelector(".yarn-draft");
      if (draft) svg.removeChild(draft);
      // The drop target is resolved GEOMETRICALLY against the card
      // rects, not via elementFromPoint: yarn chips and stickies float
      // above the papers, and a drop must never die because a floating
      // element happens to cover the aimed-at card. Cards are
      // collision-free by construction (R4-I-35), so at most one
      // contains the point.
      var c2 = canvas();
      if (!c2) return;
      var canvasRect2 = c2.getBoundingClientRect();
      var px2 = e.clientX - canvasRect2.left + c2.scrollLeft;
      var py2 = e.clientY - canvasRect2.top + c2.scrollTop;
      var target = null;
      var papers2 = c2.querySelectorAll(".objcard, .refcard");
      for (var pi = 0; pi < papers2.length; pi++) {
        var pr = rectOf(papers2[pi]);
        if (px2 >= pr.x && px2 <= pr.x + pr.w && py2 >= pr.y && py2 <= pr.y + pr.h) {
          target = papers2[pi];
          break;
        }
      }
      if (!target || target === g.fromEl) return;
      if (g.proto) {
        // A sticky's yarn. On a STORY wall it authors an evidence
        // obligation on the AC it lands on (spec/obligation-artifact ac-3);
        // on a FEATURE wall it is a proto-sticky's stub attribution (dc-5).
        // The two never mix: proto-stickies are feature-class only.
        if (state.class === "story") {
          routeObligationYarn(g, target);
        } else {
          routeProtoYarn(g, target);
        }
        return;
      }
      openPicker({
        from: g.from,
        fromKind: g.fromKind,
        to: keyOfElement(target),
        toKind: kindOfElement(target),
      });
      return;
    }

    g.el.classList.remove("dragging");
    var onTrash = g.moved && overTrash(e);
    hideTrash();
    if (!g.moved) return; // a plain click (or half a dblclick), not a drag
    justDragged = g.el;

    if (onTrash) {
      // The paper snaps back first (nothing is written by the drop
      // itself); removal then happens per tier — immediately for
      // scratch, behind the confirmation for record edits, so a
      // cancelled confirm leaves the wall exactly as it was.
      g.el.style.left = g.startLeft;
      g.el.style.top = g.startTop;
      layoutYarn();
      trashDrop(g);
      return;
    }

    if (g.kind === "card") {
      var x = parseFloat(g.el.style.left) || 0;
      var y = parseFloat(g.el.style.top) || 0;
      mutate("position", { id: g.el.getAttribute("data-id"), x: x, y: y });
    } else if (g.kind === "stub") {
      // A stub drags exactly like an object card — same position action,
      // same collision-free drop resolution server-side — keyed in its
      // own "stub:<slug>" namespace (round 5.5 dc-6).
      mutate("position", {
        id: "stub:" + g.el.getAttribute("data-stub"),
        x: parseFloat(g.el.style.left) || 0,
        y: parseFloat(g.el.style.top) || 0,
      });
    } else if (g.kind === "sticky") {
      mutate("sticky-position", { id: g.el.getAttribute("data-id"), x: parseFloat(g.el.style.left) || 0, y: parseFloat(g.el.style.top) || 0 });
    } else if (g.kind === "refcard" && g.el.hasAttribute("data-pin-id")) {
      // Pins drag like stickies: the position lives in the pin record.
      mutate("sticky-position", { id: g.el.getAttribute("data-pin-id"), x: parseFloat(g.el.style.left) || 0, y: parseFloat(g.el.style.top) || 0 });
    } else {
      // An edge-derived reference card (or a chip) has no stored
      // position: away from the trash it snaps home, exactly as today.
      g.el.style.left = g.startLeft;
      g.el.style.top = g.startTop;
      layoutYarn();
    }
  }

  function onPointerUp(e) {
    if (!gesture || e.pointerId !== gesture.pointerId) return;
    finishGesture(e);
  }

  // The platform took the pointer away (system gesture, palm rejection,
  // window switch): revert — the card goes back where it was, nothing is
  // written.
  function onPointerCancel(e) {
    if (!gesture || e.pointerId !== gesture.pointerId) return;
    var g = gesture;
    gesture = null;
    hideTrash();
    if (g.kind === "yarn") {
      var svg = ensureYarnSvg();
      var draft = svg.querySelector(".yarn-draft");
      if (draft) svg.removeChild(draft);
    } else {
      g.el.classList.remove("dragging");
      g.el.style.left = g.startLeft;
      g.el.style.top = g.startTop;
      layoutYarn();
    }
    resumeHeldRefresh(); // a cancel writes nothing; a held refresh may land now
  }

  // -- inline card editor (authoring is bidirectional) ----------------------

  var editing = false;

  function onDblClick(e) {
    if (!authoring || editing) return;
    var card = e.target.closest(".objcard");
    if (!card) return;
    var textEl = card.querySelector(".card-text");
    if (!textEl) return;
    cancelExpand(); // editing a card wins over the click-to-expand it shares
    editing = true;
    var original = textEl.textContent;
    var editor = document.createElement("textarea");
    editor.className = "card-editor";
    editor.setAttribute("aria-label", "Card text");
    editor.value = original;
    textEl.hidden = true;
    textEl.parentNode.insertBefore(editor, textEl);
    editor.focus();
    editor.addEventListener("blur", function () {
      var next = editor.value.trim();
      editor.remove();
      textEl.hidden = false;
      editing = false;
      if (next && next !== original) {
        mutate("edit-text", { id: card.getAttribute("data-id"), text: next });
      }
      resumeHeldRefresh(); // an unchanged edit ends the hold with no mutation
    });
  }

  // -- sticky creation -------------------------------------------------------
  //
  // The draft starts NEUTRAL: the author picks the sticky's type from an
  // inline segmented control as part of creating it (owner UAT round 6,
  // item 2 — no silent question-by-default, no second modal). Leaving
  // the draft commits it once text AND type exist; with text but no type
  // it stays, showing a hint (never a silent default, never silent
  // loss); Escape discards it.

  var STICKY_TYPES = [
    ["comment", "Comment"],
    ["question", "Question"],
    ["decision-needed", "Decision needed"],
    ["agent-task", "Agent task"],
  ];
  // Story/spike proto-stickies are the feature wall's scoping surface
  // (dc-5): the server refuses them anywhere else, so the control only
  // offers them there — the menu never offers what the server refuses.
  // The menu LABEL is the class display word (a proto-sticky names what
  // it becomes); the first element — the type VALUE sent to the server —
  // stays the bare enum id, like state.class's own gate here.
  if (state.class === "feature") {
    STICKY_TYPES.push(["story", classWordCap("story")], ["spike", classWordCap("spike")]);
  }

  function startStickyEditor() {
    var c = canvas();
    if (!c || c.querySelector(".sticky-draft")) return;
    var draft = document.createElement("div");
    draft.className = "sticky sticky-draft";
    draft.style.left = "16px";
    draft.style.top = "16px";

    var chosen = "";
    var picker = document.createElement("div");
    picker.className = "sticky-type-picker";
    picker.setAttribute("role", "group");
    picker.setAttribute("aria-label", "Sticky type");
    STICKY_TYPES.forEach(function (t) {
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = "sticky-type-choice";
      btn.setAttribute("data-sticky-type", t[0]);
      btn.setAttribute("aria-pressed", "false");
      btn.textContent = t[1];
      // Choosing a type must never steal focus from the editor: Safari
      // blurs the focused element to body on a button's mousedown
      // (relatedTarget null), which read as "leaving the draft" and
      // closed it before the author typed a word (live-UAT finding).
      btn.addEventListener("pointerdown", function (ev) {
        ev.preventDefault();
      });
      btn.addEventListener("mousedown", function (ev) {
        ev.preventDefault();
      });
      btn.addEventListener("click", function () {
        chosen = t[0];
        draft.className = "sticky sticky--" + t[0] + " sticky-draft";
        var all = picker.querySelectorAll(".sticky-type-choice");
        for (var i = 0; i < all.length; i++) {
          all[i].setAttribute("aria-pressed", all[i] === btn ? "true" : "false");
        }
        var hint = draft.querySelector(".sticky-type-hint");
        if (hint) hint.remove();
      });
      picker.appendChild(btn);
    });
    draft.appendChild(picker);

    var editor = document.createElement("textarea");
    editor.setAttribute("aria-label", "Sticky text");
    editor.className = "sticky-editor";
    draft.appendChild(editor);
    c.appendChild(draft);
    editor.focus();

    function needType() {
      if (draft.querySelector(".sticky-type-hint")) return;
      var hint = document.createElement("p");
      hint.className = "sticky-type-hint";
      hint.setAttribute("data-testid", "sticky-type-hint");
      hint.textContent = "Pick a type to pin this sticky — or press Escape to discard it.";
      draft.appendChild(hint);
    }

    // commitDraft is the one commit path (focus-out and the Enter key
    // share it): with text AND a chosen type the sticky is minted; with
    // text but no type the hint appears — never a silent default; with
    // no text there is nothing to commit and the draft stays.
    function commitDraft() {
      var text = editor.value.trim();
      if (!text) return;
      if (!chosen) {
        needType();
        return;
      }
      draft.remove();
      mutate("sticky", { text: text, type: chosen });
    }

    // Commit when focus truly leaves the draft. Decided one tick later
    // against document.activeElement, not focusout's relatedTarget —
    // Safari reports null relatedTarget on blurs it routes to body, and
    // the type buttons refuse focus anyway (their pointerdown is
    // prevented), so activeElement is the only honest signal.
    draft.addEventListener("focusout", function () {
      setTimeout(function () {
        if (!draft.isConnected) return;
        if (draft.contains(document.activeElement)) return;
        if (!editor.value.trim()) {
          draft.remove(); // leaving an empty draft discards it
          resumeHeldRefresh();
          return;
        }
        commitDraft();
      }, 0);
    });
    // Enter commits the sticky; Shift+Enter stays the textarea's native
    // newline (owner request 2026-07-19). An Enter that belongs to an
    // IME composition is the IME's, never a commit.
    editor.addEventListener("keydown", function (e) {
      if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
        e.preventDefault();
        commitDraft();
      }
    });
    draft.addEventListener("keydown", function (e) {
      if (e.key === "Escape") {
        e.stopPropagation(); // the draft dies; open dialogs are not its business
        draft.remove();
        resumeHeldRefresh();
      }
    });
  }

  // -- reference peek ---------------------------------------------------------
  //
  // Clicking a reference card opens an in-board peek of the referenced
  // artifact (owner UAT round 6, item 4): the server fragment carries
  // title, kind, status, rendered body, and the full-page link. Read-only
  // information, so it works in EVERY board mode; an unresolvable ref
  // renders the fragment's disclosed explanation — never a dead click.
  // Closes via ×, Escape, or clicking anywhere outside it.

  function closeRefPeek() {
    var p = document.getElementById("ref-peek");
    if (p) p.remove();
  }

  function openRefPeek(ref) {
    closeRefPeek();
    var panel = document.createElement("aside");
    panel.id = "ref-peek";
    panel.className = "ref-peek";
    panel.setAttribute("data-testid", "ref-peek");
    panel.setAttribute("role", "complementary");
    panel.setAttribute("aria-label", "Reference peek");

    var bar = document.createElement("div");
    bar.className = "ref-peek-bar";
    var label = document.createElement("span");
    label.className = "ref-peek-ref";
    label.textContent = ref;
    label.title = ref;
    var close = document.createElement("button");
    close.type = "button";
    close.id = "ref-peek-close";
    close.setAttribute("aria-label", "Close peek");
    close.textContent = "×";
    bar.appendChild(label);
    bar.appendChild(close);
    panel.appendChild(bar);

    var content = document.createElement("div");
    content.className = "ref-peek-content";
    content.textContent = "loading…";
    panel.appendChild(content);
    document.body.appendChild(panel);

    fetch(
      boardURL("/peek?ref=" + encodeURIComponent(ref))
    )
      .then(function (resp) {
        if (!resp.ok) throw new Error("HTTP " + resp.status);
        return resp.text();
      })
      .then(function (html) {
        content.innerHTML = html;
        // A peeked artifact body may carry a badged mermaid figure (a
        // diagram-kind target, or a fenced block in spec prose) — same
        // pinned renderer as every surface.
        renderMermaidIn(content);
      })
      .catch(function (err) {
        content.textContent = "peek failed: " + err.message;
      });
  }

  // -- the supply toolbox (import/pin) ----------------------------------------
  //
  // The wall's box of pins (owner directive): a quiet tab at the
  // screen's lower-left; one click opens the tray — a search picker over
  // the corpus index, server-rendered rows — and choosing a row pins the
  // artifact to the wall. Escape, the tab, or any outside click closes
  // it without residue.

  var pinFetchSeq = 0;

  function pinTray() {
    return document.getElementById("pin-tray");
  }

  function fetchPinResults(q) {
    var results = document.getElementById("pin-results");
    if (!results) return;
    var seq = ++pinFetchSeq;
    fetch(
      boardURL("/pinsearch?q=" + encodeURIComponent(q))
    )
      .then(function (resp) {
        if (!resp.ok) throw new Error("HTTP " + resp.status);
        return resp.text();
      })
      .then(function (html) {
        if (seq !== pinFetchSeq) return; // a newer query already answered
        results.innerHTML = html;
      })
      .catch(function (err) {
        if (seq !== pinFetchSeq) return;
        results.textContent = "search failed: " + err.message;
      });
  }

  function openPinTray() {
    var tray = pinTray();
    var tab = document.getElementById("pin-toolbox-tab");
    if (!tray || !tray.hidden) return;
    tray.hidden = false;
    if (tab) tab.setAttribute("aria-expanded", "true");
    var input = document.getElementById("pin-search");
    if (input) {
      input.value = "";
      input.focus();
    }
    fetchPinResults("");
  }

  function closePinTray() {
    var tray = pinTray();
    var tab = document.getElementById("pin-toolbox-tab");
    if (!tray || tray.hidden) return;
    tray.hidden = true;
    if (tab) tab.setAttribute("aria-expanded", "false");
  }

  function onInput(e) {
    if (e.target && e.target.id === "pin-search") fetchPinResults(e.target.value);
  }

  // -- graduate menus ---------------------------------------------------------

  var pendingSticky = null;

  function openGraduateMenu(anchorEl, stickyID) {
    pendingSticky = stickyID;
    var menu = document.getElementById("graduate-menu");
    if (!menu) return;
    var rect = anchorEl.getBoundingClientRect();
    show("graduate-menu"); // visible first: a hidden menu measures 0×0
    // Clamped to the viewport: the menu is position:fixed, so a cut-off
    // menu can never be scrolled to — a sticky near the bottom edge
    // opens its menu upward instead.
    var left = rect.left;
    var top = rect.bottom + 4;
    if (top + menu.offsetHeight > window.innerHeight - 8) {
      top = Math.max(8, rect.top - menu.offsetHeight - 4);
    }
    if (left + menu.offsetWidth > window.innerWidth - 8) {
      left = Math.max(8, window.innerWidth - menu.offsetWidth - 8);
    }
    menu.style.left = left + "px";
    menu.style.top = top + "px";
  }

  // -- click routing -----------------------------------------------------------

  function onClick(e) {
    var t = e.target;
    var dragGhost = justDragged;
    justDragged = null;

    // The peek dismisses on any click outside it (a reference card click
    // replaces it with the new ref's peek instead).
    var peekEl = document.getElementById("ref-peek");
    if (peekEl && !peekEl.contains(t) && !t.closest(".refcard")) {
      closeRefPeek();
    }
    if (t.id === "ref-peek-close") {
      closeRefPeek();
      return;
    }

    // The expand dialog closes from its ×, the backdrop, or Escape — the
    // same three exits every board dialog offers.
    if (t.id === "expand-close" || t.id === "expand-backdrop") {
      closeExpandDialog();
      return;
    }

    // The derivation drawer (spec/derivation-drawer dc-4): activating a
    // badge button — pointer here, and Enter/Space through the button's
    // own native click synthesis — opens its server-rendered sibling;
    // its ×, the scrim, or Escape closes it. Available in every mode:
    // reading a receipt is never a write.
    if (t.closest(".drawer-close") || t.id === "drawer-backdrop") {
      closeBadgeDrawer();
      return;
    }
    var badgeBtn = t.closest(".badge-chip, .case-stamp");
    if (badgeBtn) {
      openBadgeDrawer(badgeBtn);
      return;
    }

    // The supply toolbox: the tab toggles the tray; a result row pins;
    // any click outside closes the tray without residue.
    if (t.closest("#pin-toolbox-tab")) {
      var trayEl = pinTray();
      if (trayEl && trayEl.hidden) openPinTray();
      else closePinTray();
      return;
    }
    var pinResult = t.closest(".pin-result");
    if (pinResult) {
      closePinTray();
      mutate("pin", { ref: pinResult.getAttribute("data-ref") });
      return;
    }
    var openTray = pinTray();
    if (openTray && !openTray.hidden && !t.closest("#pin-toolbox")) {
      closePinTray();
    }

    var refcard = t.closest(".refcard");
    if (refcard) {
      if (refcard === dragGhost) return; // a drag's tail, not a peek click
      if (t.closest("a")) return; // the card's editor doorway is a real link, not a peek click
      openRefPeek(refcard.getAttribute("data-ref"));
      return;
    }

    // Case-file placards are ALWAYS expandable — a click anywhere on one
    // (its face, its clamped headline, or its dog-ear button) reads the full
    // case file: the rendered body prose when the spec carried one, else the
    // headline. This is independent of clamping (the width-dependence fix),
    // so it is handled before the generic clamp path below. A degenerate
    // placard never wears `.placard--expandable`, so a click on it is inert.
    var expandablePlacard = t.closest(".placard--expandable");
    if (expandablePlacard) {
      openPlacardExpand(expandablePlacard);
      return;
    }

    // Click-to-expand: a clamped placard / card text / stub title opens
    // its read-only dialog. Only truncated text carries `.is-clamped`, so
    // a short one is inert. A reference card is excluded above (its own
    // click is the peek, which already shows the whole artifact).
    var clampEl = t.closest(".is-clamped[data-expandable]");
    if (!clampEl && !t.closest("button, textarea, input, .review-sticky")) {
      // A draggable paper captures the pointer on press, so the click's
      // target is the paper itself, not the clamped text child underneath
      // it (a placard, uncaptured, resolves directly above). Recover the
      // paper's own clamped text so a click anywhere on a truncated card
      // or stub still expands it.
      var paper = t.closest(".objcard, .stubcard, .sticky");
      if (paper) clampEl = paper.querySelector(".is-clamped[data-expandable]");
    }
    // The drag-tail guard: a completed drag's click fires on the dragged
    // paper (dragGhost) — its clamped child must not be read as an expand.
    if (clampEl && !(dragGhost && dragGhost.contains(clampEl))) {
      scheduleExpand(clampEl);
      return;
    }

    var choice = t.closest("[data-edge-choice]");
    if (choice) {
      pickEdgeType(choice.getAttribute("data-edge-choice"));
      return;
    }

    // The obligation's evidence-kind pick (spec/obligation-artifact ac-3):
    // choosing a for_kind graduates the dropped sticky into its obligation.
    var forKind = t.closest("[data-forkind]");
    if (forKind) {
      pickForKind(forKind.getAttribute("data-forkind"));
      return;
    }

    switch (t.id) {
      case "edge-confirm-ok": {
        // The proto-sticky attribution thread: confirmed meaning, minted
        // directly as an untyped relates record (dc-5 — the endpoint
        // pair carries the semantics, no picker ceremony).
        if (pending && pending.protoRelate) {
          var rel = pending.protoRelate;
          pending = null;
          hideAllDialogs();
          mutate("relates", { from: rel.from, to: rel.to });
          return;
        }
        // Stub graduation (dc-6's register ceremony). The server's
        // refusals — zero yarn, slug collision — come back in plain
        // language; surface them in the same dialog, never a raw toast.
        if (pending && pending.stubGraduate) {
          var gradID = pending.stubGraduate;
          pending = null;
          hideAllDialogs();
          setStatus("saving…");
          api("stub-graduate", { id: gradID })
            .then(function (data) {
              if (typeof data.dirty === "boolean") state.git.dirty = data.dirty;
              return refreshFragment().then(function () {
                setStatus("saved");
              });
            })
            .catch(function (err) {
              setStatus("");
              openConfirm("Not yet a stub", err.message, false);
              document.getElementById("edge-confirm-ok").hidden = true;
            });
          return;
        }
        // Instantiate (ac-6): the sealed wall's one live affordance.
        // Success is a receipt — the branch name and the tracker-ref
        // placeholder the operator must fill; failure surfaces plain.
        if (pending && pending.instantiate) {
          var slug = pending.instantiate;
          pending = null;
          hideAllDialogs();
          setStatus("cutting branch…");
          api("stub-instantiate", { id: slug })
            .then(function () {
              setStatus("");
              // The receipt's class words are display prose (classWord);
              // "story:" names the frontmatter FIELD and the branch/spec
              // refs are identity — all stay bare.
              openConfirm(
                classWordCap("story") + " instantiated",
                "Branch design/" + slug + " now carries spec/" + slug +
                  ", scaffolded from this stub. Its story: tracker ref is the placeholder " +
                  "todo:REPLACE-ME — fill it in on the branch before the " + classWord("story") + " is real. " +
                  "This wall (the serving checkout) has not moved.",
                false
              );
              document.getElementById("edge-confirm-ok").hidden = true;
            })
            .catch(function (err) {
              setStatus("");
              openConfirm("Could not instantiate", err.message, false);
              document.getElementById("edge-confirm-ok").hidden = true;
            });
          return;
        }
        if (pending && pending.remove) {
          var removal = pending;
          pending = null;
          hideAllDialogs();
          mutate("edge-delete", { from: removal.from, to: removal.to, type: removal.type });
          return;
        }
        // The trash confirmations (owner directive): removal happens
        // ONLY here — cancel or Escape leaves everything standing.
        if (pending && pending.trashRef) {
          var deadRef = pending.trashRef;
          pending = null;
          hideAllDialogs();
          mutate("ref-trash", { ref: deadRef });
          return;
        }
        if (pending && pending.trashObject) {
          var deadObject = pending.trashObject;
          pending = null;
          hideAllDialogs();
          mutate("object-trash", { id: deadObject });
          return;
        }
        commitEdge(pending.type, document.getElementById("edge-confirm-reason").value.trim());
        return;
      }
      // Every dialog closes from a visible affordance, the backdrop, or
      // Escape (owner UAT round 6: never a modal you can't get out of).
      case "modal-backdrop":
      case "edge-confirm-cancel":
      case "edge-picker-cancel":
      case "graduate-menu-cancel":
        pending = null;
        pendingSticky = null;
        hideAllDialogs();
        return;
      case "commit-push-btn":
        document.getElementById("commit-message").value = "";
        show("commit-dialog");
        document.getElementById("commit-message").focus();
        return;
      case "commit-dialog-ok": {
        var msg = document.getElementById("commit-message").value.trim();
        if (!msg) {
          document.getElementById("commit-message").focus();
          return;
        }
        hideAllDialogs();
        mutate("git-commit", { message: msg });
        return;
      }
      case "commit-dialog-cancel":
      case "branch-guard-stay":
        hideAllDialogs();
        return;
      case "add-sticky-btn":
        startStickyEditor();
        return;
    }

    // Deletion affordances (owner UAT round 6, item 3): scratch records
    // die immediately (mutable stream only); a spec-layer edge mirrors
    // creation — gate-bearing types restate their removal consequence
    // and confirm first, others remove on the spot.
    var del = t.closest(".delete-btn");
    if (del) {
      var what = del.getAttribute("data-delete");
      if (what === "sticky") {
        // Immediate acknowledgment (same as the trash drop): the sticky
        // hides the moment its delete is posted — no stale ghost to
        // double-delete; a refusal reconciles it back via the refetch.
        var deadSticky = del.closest(".sticky");
        deadSticky.style.visibility = "hidden";
        mutate("annotation-delete", { id: deadSticky.getAttribute("data-id") });
      } else if (what === "thread") {
        var deadChip = del.closest(".yarn-chip");
        deadChip.style.visibility = "hidden";
        mutate("annotation-delete", { id: deadChip.getAttribute("data-annotation-id") });
      } else {
        var edgeChip = del.closest(".yarn-chip");
        var edge = {
          from: edgeChip.getAttribute("data-from"),
          to: edgeChip.getAttribute("data-to"),
          type: edgeChip.getAttribute("data-edge-type"),
        };
        if (state.gate.indexOf(edge.type) >= 0) {
          pending = { remove: true, from: edge.from, to: edge.to, type: edge.type };
          openConfirm("Remove " + edge.type, state.removals[edge.type] || "", false);
        } else {
          mutate("edge-delete", edge);
        }
      }
      return;
    }

    // In-place retype (owner directive): the chip's type label reopens
    // the context-sensitive picker over the same pair.
    var retypeBtn = t.closest("[data-retype]");
    if (retypeBtn) {
      var retypeChip = retypeBtn.closest(".yarn-chip");
      var rFrom = endpointElement(retypeChip.getAttribute("data-from"));
      var rTo = endpointElement(retypeChip.getAttribute("data-to"));
      openPicker({
        from: retypeChip.getAttribute("data-from"),
        fromKind: rFrom ? kindOfElement(rFrom) : "unknown",
        to: retypeChip.getAttribute("data-to"),
        toKind: rTo ? kindOfElement(rTo) : "unknown",
        retype: retypeChip.getAttribute("data-edge-type"),
      });
      return;
    }

    // Instantiate (sealed accepted feature wall): consequence-labeled
    // before it fires — a branch cut is not a hover-and-hope click.
    var inst = t.closest("[data-instantiate]");
    if (inst) {
      var instSlug = inst.getAttribute("data-instantiate");
      var isSpike = !!inst.closest(".stubcard--spike");
      var instWord = classWord(isSpike ? "spike" : "story");
      pending = { instantiate: instSlug };
      openConfirm(
        "Instantiate " + instWord + " “" + instSlug + "”",
        "Cuts branch design/" + instSlug + " carrying a scaffolded " +
          instWord + " spec bound to this stub by slug. " +
          "The serving checkout never moves — nothing on this wall changes until that branch merges.",
        false
      );
      return;
    }

    var grad = t.closest(".graduate-btn");
    if (grad) {
      if (grad.getAttribute("data-graduate") === "stub") {
        // The proto-sticky's graduation: the kind is already the
        // sticky's type, so there is no menu — one confirmation naming
        // the ceremony (dc-6: the band stays, the voice changes).
        var protoEl = grad.closest(".sticky");
        pending = { stubGraduate: protoEl.getAttribute("data-id") };
        openConfirm(
          "Graduate into a stub",
          "Typesets this sticky in place: its text becomes the stub's slug, its yarn " +
            "the coverage it claims — an ordinary spec edit into the stubs registry. " +
            "The handwriting becomes the record.",
          false
        );
      } else if (grad.getAttribute("data-graduate") === "sticky") {
        openGraduateMenu(grad, grad.closest(".sticky").getAttribute("data-id"));
      } else {
        var chip = grad.closest(".yarn-chip");
        var fromEl = endpointElement(chip.getAttribute("data-from"));
        var toEl = endpointElement(chip.getAttribute("data-to"));
        openPicker({
          from: chip.getAttribute("data-from"),
          fromKind: fromEl ? kindOfElement(fromEl) : "unknown",
          to: chip.getAttribute("data-to"),
          toKind: toEl ? kindOfElement(toEl) : "unknown",
          annotationId: chip.getAttribute("data-annotation-id"),
        });
      }
      return;
    }

    var gradItem = t.closest("#graduate-menu [data-object-kind]");
    if (gradItem) {
      var kind = gradItem.getAttribute("data-object-kind");
      hideAllDialogs();
      mutate("sticky-graduate", { id: pendingSticky, kind: kind });
      pendingSticky = null;
      return;
    }

    if (t.closest('[data-testid="branch-switcher"]')) {
      var menu = document.getElementById("branch-menu");
      if (menu) menu.hidden = !menu.hidden;
      return;
    }

    var branchItem = t.closest("#branch-menu [data-branch]");
    if (branchItem) {
      var branch = branchItem.getAttribute("data-branch");
      hide("branch-menu");
      if (branch === state.git.branch) return;
      if (state.git.dirty) {
        // The branch-switch guard: interrupt instead of switching (05
        // §Workbench — silent loss is the failure mode this forbids).
        show("branch-guard");
        return;
      }
      setStatus("switching…");
      api("git-switch", { branch: branch })
        .then(function () {
          window.location.reload();
        })
        .catch(function (err) {
          setStatus("error: " + err.message);
        });
      return;
    }
  }

  function onKeyDown(e) {
    if (e.key === "Escape") {
      pending = null;
      cancelExpand();
      hideAllDialogs();
      closeRefPeek();
      closeExpandDialog();
      closeBadgeDrawer();
      closePinTray();
    }
  }

  document.addEventListener("pointerdown", onPointerDown);
  document.addEventListener("pointermove", onPointerMove);
  document.addEventListener("pointerup", onPointerUp);
  document.addEventListener("pointercancel", onPointerCancel);
  document.addEventListener("dblclick", onDblClick);
  document.addEventListener("click", onClick);
  document.addEventListener("keydown", onKeyDown);
  document.addEventListener("input", onInput);

  // Re-measure clamps when wrapping can change: after web fonts load and
  // on resize (the placards are fluid). requestAnimationFrame coalesces a
  // resize storm into one measure.
  var clampRAF = null;
  window.addEventListener("resize", function () {
    if (clampRAF) cancelAnimationFrame(clampRAF);
    clampRAF = requestAnimationFrame(markClamped);
  });
  window.addEventListener("load", markClamped);

  layoutYarn();
  markClamped();
})();
