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
    if (key === "spec") return null; // document-level edges: chip only
    return (
      c.querySelector('.objcard[data-id="' + esc(key) + '"]') ||
      c.querySelector('.refcard[data-ref="' + esc(key) + '"]')
    );
  }

  function centerOf(el) {
    return {
      x: el.offsetLeft + el.offsetWidth / 2,
      y: el.offsetTop + el.offsetHeight / 2,
    };
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

  // layoutYarn draws every chip's thread and sits the chip on the
  // thread's midpoint. Chips with an unresolvable endpoint stack in the
  // canvas corner — visible, never dropped.
  function layoutYarn() {
    var c = canvas();
    if (!c) return;
    var svg = ensureYarnSvg();
    while (svg.firstChild) svg.removeChild(svg.firstChild);

    var chips = c.querySelectorAll(".yarn-chip");
    var orphanRow = 0;
    for (var i = 0; i < chips.length; i++) {
      var chip = chips[i];
      var fromEl = endpointElement(chip.getAttribute("data-from"));
      var toEl = endpointElement(chip.getAttribute("data-to"));
      if (!fromEl || !toEl) {
        chip.style.left = "16px";
        chip.style.top = 16 + orphanRow * 34 + "px";
        orphanRow++;
        continue;
      }
      var a = centerOf(fromEl);
      var b = centerOf(toEl);
      var dx = b.x - a.x;
      var dy = b.y - a.y;
      var sag = 14 + Math.sqrt(dx * dx + dy * dy) * 0.1;
      var cx = (a.x + b.x) / 2;
      var cy = (a.y + b.y) / 2 + sag;

      var path = document.createElementNS(svgNS, "path");
      path.setAttribute(
        "class",
        "yarn-thread yarn-thread--" + chip.getAttribute("data-layer")
      );
      path.setAttribute(
        "d",
        "M " + a.x + " " + a.y + " Q " + cx + " " + cy + " " + b.x + " " + b.y
      );
      svg.appendChild(path);
      var ends = [a, b];
      for (var j = 0; j < 2; j++) {
        var knot = document.createElementNS(svgNS, "circle");
        knot.setAttribute("class", "yarn-knot");
        knot.setAttribute("cx", ends[j].x);
        knot.setAttribute("cy", ends[j].y);
        knot.setAttribute("r", 3);
        svg.appendChild(knot);
      }

      // The chip rides the curve's own midpoint (t=0.5).
      var mx = 0.25 * a.x + 0.5 * cx + 0.25 * b.x;
      var my = 0.25 * a.y + 0.5 * cy + 0.25 * b.y;
      chip.style.left = mx - chip.offsetWidth / 2 + "px";
      chip.style.top = my - chip.offsetHeight / 2 + "px";
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

  // openPicker fills the context-sensitive menu: ONLY the pair's legal
  // types (each with its one-line consequence), plus the scratch tier's
  // untyped thread — always available (05 §Workbench).
  function openPicker(p) {
    pending = p;
    var items = document.getElementById("edge-picker-items");
    var pair = document.getElementById("edge-picker-pair");
    if (!items) return;
    items.innerHTML = "";
    if (pair) pair.textContent = p.from + " → " + p.to;

    var types = (state.legal[p.fromKind + "|" + p.toKind] || []).slice();
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

  var gesture = null; // {kind: "card"|"sticky"|"yarn", ...}

  function onMouseDown(e) {
    if (!authoring) return;
    var c = canvas();
    if (!c) return;

    var handle = e.target.closest(".yarn-handle");
    if (handle && c.contains(handle)) {
      var card = handle.closest(".objcard");
      gesture = {
        kind: "yarn",
        fromEl: card,
        from: card.getAttribute("data-id"),
        fromKind: card.getAttribute("data-object-kind"),
      };
      e.preventDefault();
      return;
    }

    if (e.target.closest("button, textarea, input, .review-sticky")) return;

    var el = e.target.closest(".objcard, .sticky");
    if (!el || !c.contains(el)) return;
    var rect = el.getBoundingClientRect();
    gesture = {
      kind: el.classList.contains("objcard") ? "card" : "sticky",
      el: el,
      dx: e.clientX - rect.left,
      dy: e.clientY - rect.top,
      moved: false,
    };
    el.classList.add("dragging");
    e.preventDefault();
  }

  function onMouseMove(e) {
    if (!gesture) return;
    var c = canvas();
    var canvasRect = c.getBoundingClientRect();

    if (gesture.kind === "yarn") {
      var svg = ensureYarnSvg();
      var old = svg.querySelector(".yarn-draft");
      if (old) svg.removeChild(old);
      var a = centerOf(gesture.fromEl);
      var bx = e.clientX - canvasRect.left + c.scrollLeft;
      var by = e.clientY - canvasRect.top + c.scrollTop;
      var line = document.createElementNS(svgNS, "line");
      line.setAttribute("class", "yarn-thread yarn-draft");
      line.setAttribute("x1", a.x);
      line.setAttribute("y1", a.y);
      line.setAttribute("x2", bx);
      line.setAttribute("y2", by);
      svg.appendChild(line);
      return;
    }

    var x = e.clientX - canvasRect.left - gesture.dx + c.scrollLeft;
    var y = e.clientY - canvasRect.top - gesture.dy + c.scrollTop;
    if (x < 0) x = 0;
    if (y < 0) y = 0;
    gesture.el.style.left = x + "px";
    gesture.el.style.top = y + "px";
    gesture.moved = true;
    layoutYarn();
  }

  function onMouseUp(e) {
    if (!gesture) return;
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

  function startStickyEditor() {
    var c = canvas();
    if (!c || c.querySelector(".sticky-draft")) return;
    var draft = document.createElement("div");
    draft.className = "sticky sticky--question sticky-draft";
    draft.style.left = "16px";
    draft.style.top = "16px";
    var editor = document.createElement("textarea");
    editor.setAttribute("aria-label", "Sticky text");
    editor.className = "sticky-editor";
    draft.appendChild(editor);
    c.appendChild(draft);
    editor.focus();
    editor.addEventListener("blur", function () {
      var text = editor.value.trim();
      draft.remove();
      if (text) mutate("sticky", { text: text });
    });
  }

  // -- graduate menus ---------------------------------------------------------

  var pendingSticky = null;

  function openGraduateMenu(anchorEl, stickyID) {
    pendingSticky = stickyID;
    var menu = document.getElementById("graduate-menu");
    if (!menu) return;
    var rect = anchorEl.getBoundingClientRect();
    menu.style.left = rect.left + "px";
    menu.style.top = rect.bottom + 4 + "px";
    show("graduate-menu");
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
      case "edge-confirm-cancel":
        pending = null;
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

  document.addEventListener("mousedown", onMouseDown);
  document.addEventListener("mousemove", onMouseMove);
  document.addEventListener("mouseup", onMouseUp);
  document.addEventListener("dblclick", onDblClick);
  document.addEventListener("click", onClick);
  document.addEventListener("keydown", onKeyDown);

  layoutYarn();
})();
