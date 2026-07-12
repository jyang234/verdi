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
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  if (!state) return;

  var region = document.getElementById("boardv2-region");
  var statusEl = document.getElementById("autosave-status");
  var authoring = state.mode === "authoring";

  function canvas() {
    return document.getElementById("board-canvas");
  }
  function setStatus(text) {
    if (statusEl) statusEl.textContent = text;
  }
  function esc(s) {
    return window.CSS && CSS.escape ? CSS.escape(s) : s.replace(/["\\]/g, "\\$&");
  }

  // -- server round-trips --------------------------------------------------

  function api(action, body) {
    return fetch(
      "/board/spec/" + encodeURIComponent(state.spec) + "/api/" + action,
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

  function refreshFragment() {
    return fetch(
      "/board/spec/" + encodeURIComponent(state.spec) + "/fragment"
    ).then(function (resp) {
      if (!resp.ok) throw new Error("fragment: HTTP " + resp.status);
      return resp.text().then(function (html) {
        region.innerHTML = html;
        layoutYarn();
      });
    });
  }

  // mutate: POST, remember dirtiness, swap the fresh projection in, and
  // only then report "saved" — the signal the autosave contract binds.
  function mutate(action, body) {
    setStatus("saving…");
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
    return (
      c.querySelector('.objcard[data-id="' + esc(key) + '"]') ||
      c.querySelector('.refcard[data-ref="' + esc(key) + '"]')
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
    var papers = c.querySelectorAll(".objcard, .refcard");
    var offboardCount = 0; // bow alternation + margin slots
    var offboardTies = {}; // fan-out of several document edges on one card
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

      var path = document.createElementNS(svgNS, "path");
      path.setAttribute(
        "class",
        "yarn-thread yarn-thread--" + chip.getAttribute("data-layer") +
          (offboard ? " yarn-thread--offboard" : "")
      );
      path.setAttribute(
        "d",
        "M " + a.x + " " + a.y + " Q " + cx + " " + cy + " " + b.x + " " + b.y
      );
      svg.appendChild(path);
      for (var j = 0; j < knots.length; j++) {
        var knot = document.createElementNS(svgNS, "circle");
        knot.setAttribute("class", "yarn-knot");
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
      var clearOfCards = function (x, y) {
        for (var k = 0; k < papers.length; k++) {
          var r = rectOf(papers[k]);
          if (x < r.x + r.w && r.x < x + w && y < r.y + r.h && r.y < y + h) {
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
      for (var m = 0; m < ts.length; m++) {
        var p = pointAt(ts[m]);
        if (clearOfCards(p.x - w / 2, p.y - h / 2)) {
          spot = p;
          break;
        }
      }
      var top = spot.y - h / 2;
      // A short off-board thread's midpoint can sit above the canvas;
      // the chip stays fully on the board (the thread is what exits).
      if (offboard && top < 4) top = 4;
      chip.style.left = spot.x - w / 2 + "px";
      chip.style.top = top + "px";
    }
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

    var legal = state.legal[p.fromKind + "|" + p.toKind] || [];
    if (legal.length === 0) {
      var note = document.createElement("p");
      note.className = "ritual-note picker-empty-note";
      note.setAttribute("data-testid", "picker-no-typed-edge");
      note.textContent =
        "No typed edge exists between " + pairPhrase(p.fromKind, p.toKind) +
        (p.annotationId
          ? " — this stays a scratch thread. To type a connection, draw it from a decision card."
          : " — this can only be a scratch thread.");
      items.appendChild(note);
    }

    var types = legal.slice();
    if (!p.annotationId) types.push("relates");
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

  function pickEdgeType(t) {
    hide("edge-picker");
    if (t === "relates") {
      hideAllDialogs();
      mutate("relates", { from: pending.from, to: pending.to });
      return;
    }
    if (state.gate.indexOf(t) >= 0) {
      pending.type = t;
      var confirmEl = document.getElementById("edge-confirm");
      confirmEl.setAttribute("aria-label", "Confirm " + t);
      document.getElementById("edge-confirm-title").textContent = "Confirm " + t;
      document.getElementById("edge-confirm-consequence").textContent =
        state.consequences[t] || "";
      var reasonField = document.getElementById("edge-confirm-reason-field");
      var reason = document.getElementById("edge-confirm-reason");
      reason.value = "";
      reasonField.hidden = t !== "exempts";
      show("edge-confirm");
      return;
    }
    commitEdge(t, "");
  }

  function commitEdge(t, note) {
    hideAllDialogs();
    if (pending.annotationId) {
      mutate("relates-graduate", { id: pending.annotationId, type: t, note: note });
    } else {
      mutate("edge", { from: pending.from, to: pending.to, type: t, note: note });
    }
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

  var gesture = null; // {kind: "card"|"sticky"|"yarn", pointerId, ...}

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

    if (e.target.closest("button, textarea, input, .review-sticky")) return;

    var el = e.target.closest(".objcard, .sticky");
    if (!el || !c.contains(el)) return;
    if (!authoring) {
      refuseDrag();
      return;
    }
    var rect = el.getBoundingClientRect();
    gesture = {
      kind: el.classList.contains("objcard") ? "card" : "sticky",
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
    layoutYarn();
  }

  function finishGesture(e) {
    var g = gesture;
    gesture = null;

    if (g.kind === "yarn") {
      var svg = ensureYarnSvg();
      var draft = svg.querySelector(".yarn-draft");
      if (draft) svg.removeChild(draft);
      var hit = document.elementFromPoint(e.clientX, e.clientY);
      var target = hit && hit.closest(".objcard, .refcard");
      if (!target || target === g.fromEl) return;
      openPicker({
        from: g.from,
        fromKind: g.fromKind,
        to: keyOfElement(target),
        toKind: kindOfElement(target),
      });
      return;
    }

    g.el.classList.remove("dragging");
    if (!g.moved) return; // a plain click (or half a dblclick), not a drag
    var x = parseFloat(g.el.style.left) || 0;
    var y = parseFloat(g.el.style.top) || 0;
    if (g.kind === "card") {
      mutate("position", { id: g.el.getAttribute("data-id"), x: x, y: y });
    } else {
      mutate("sticky-position", { id: g.el.getAttribute("data-id"), x: x, y: y });
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
    if (g.kind === "yarn") {
      var svg = ensureYarnSvg();
      var draft = svg.querySelector(".yarn-draft");
      if (draft) svg.removeChild(draft);
      return;
    }
    g.el.classList.remove("dragging");
    g.el.style.left = g.startLeft;
    g.el.style.top = g.startTop;
    layoutYarn();
  }

  // -- inline card editor (authoring is bidirectional) ----------------------

  var editing = false;

  function onDblClick(e) {
    if (!authoring || editing) return;
    var card = e.target.closest(".objcard");
    if (!card) return;
    var textEl = card.querySelector(".card-text");
    if (!textEl) return;
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

    // Commit when focus truly leaves the draft (a type-button click moves
    // focus WITHIN it and must not count as leaving).
    draft.addEventListener("focusout", function (e) {
      if (e.relatedTarget && draft.contains(e.relatedTarget)) return;
      var text = editor.value.trim();
      if (!text) {
        draft.remove();
        return;
      }
      if (!chosen) {
        needType();
        return;
      }
      draft.remove();
      mutate("sticky", { text: text, type: chosen });
    });
    draft.addEventListener("keydown", function (e) {
      if (e.key === "Escape") {
        e.stopPropagation(); // the draft dies; open dialogs are not its business
        draft.remove();
      }
    });
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

    var choice = t.closest("[data-edge-choice]");
    if (choice) {
      pickEdgeType(choice.getAttribute("data-edge-choice"));
      return;
    }

    switch (t.id) {
      case "edge-confirm-ok":
        commitEdge(pending.type, document.getElementById("edge-confirm-reason").value.trim());
        return;
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

    var grad = t.closest(".graduate-btn");
    if (grad) {
      if (grad.getAttribute("data-graduate") === "sticky") {
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
      hideAllDialogs();
    }
  }

  document.addEventListener("pointerdown", onPointerDown);
  document.addEventListener("pointermove", onPointerMove);
  document.addEventListener("pointerup", onPointerUp);
  document.addEventListener("pointercancel", onPointerCancel);
  document.addEventListener("dblclick", onDblClick);
  document.addEventListener("click", onClick);
  document.addEventListener("keydown", onKeyDown);

  layoutYarn();
})();
