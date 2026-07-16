// The diagram-proposal editor's entire client-side surface
// (spec/board-editor ac-1/ac-2/ac-4; JS minimal, ONE file, no framework,
// no build step — the board's own posture). The server renders the
// region and computes EVERY edit (dc-2: the op → text-edit grammar is
// server-side; this file never duplicates it); this file only
//   (a) live-renders the pane text under the one vendored mermaid asset
//       (dc-3) and PAINTS the renderer's own error when it rejects the
//       source — never a blank preview, never a stale last-good picture,
//   (b) autosaves the pane's exact bytes,
//   (c) turns preview gestures (click-click connect, drag-to-connect,
//       inline rename, delete) into op POSTs — a drag connects, it never
//       places, and no request carries a position (co-2),
//   (d) drives before-peek and reset for a derived proposal (ac-4).
//
// Server contract (internal/workbench/boarddiagram*.go):
//   GET  /board/diagram/<name>               -> the page (this script + state)
//   GET  /board/diagram/<name>/fragment      -> re-rendered editor region
//   POST /board/diagram/<name>/api/<action>  -> save/ops/peek/reset
(function () {
  "use strict";

  var state = window.__DIAGRAM__;
  if (!state || typeof mermaid === "undefined") return;

  var pane = document.getElementById("diagram-source");
  var preview = document.getElementById("diagram-preview");
  var errorBox = document.getElementById("diagram-render-error");
  var errorMsg = document.getElementById("diagram-render-error-msg");
  var statusEl = document.getElementById("autosave-status");
  var authoring = state.mode === "authoring";
  if (!pane || !preview) return;

  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme:
      window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "default",
  });

  function setStatus(text) {
    if (statusEl) statusEl.textContent = text;
  }

  // -- server round-trips --------------------------------------------------

  function api(action, body) {
    return fetch(
      "/board/diagram/" + encodeURIComponent(state.name) + "/api/" + action,
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

  // -- the live preview (ac-1) ----------------------------------------------
  //
  // Each render is sequenced: a stale async completion never paints over
  // a newer one. On a renderer rejection the previous SVG is REMOVED and
  // the error box carries the renderer's own message.

  var renderSeq = 0;

  function cleanStrays(id) {
    // mermaid can leave a temp element behind when parse/render throws.
    var stray = document.getElementById("d" + id);
    if (stray && stray.parentNode) stray.parentNode.removeChild(stray);
    stray = document.getElementById(id);
    if (stray && stray !== preview && !preview.contains(stray) && stray.parentNode) {
      stray.parentNode.removeChild(stray);
    }
  }

  function renderInto(container, text, idBase, onDone) {
    var id = idBase + "-" + ++renderSeq;
    var mine = renderSeq;
    mermaid
      .render(id, text)
      .then(function (out) {
        if (mine !== renderSeq && container === preview) return;
        container.innerHTML = out.svg;
        if (onDone) onDone(null);
      })
      .catch(function (err) {
        cleanStrays(id);
        if (mine !== renderSeq && container === preview) return;
        if (onDone) onDone(err || new Error("render failed"));
      });
  }

  function renderPreview() {
    renderInto(preview, pane.value, "live-" + state.name, function (err) {
      if (err) {
        // Paint the failure where the picture was: clear the retained
        // SVG (a stale picture would show a diagram the source no longer
        // describes) and surface the renderer's own message.
        preview.innerHTML = "";
        if (errorMsg) errorMsg.textContent = String((err && err.message) || err);
        if (errorBox) errorBox.hidden = false;
      } else {
        if (errorBox) errorBox.hidden = true;
        if (errorMsg) errorMsg.textContent = "";
        annotatePreview();
      }
    });
  }

  // -- autosave (ac-3: the pane's exact bytes) -------------------------------

  var saveTimer = null;
  var savingChain = Promise.resolve();

  function saveNow() {
    if (!authoring) return Promise.resolve();
    var text = pane.value;
    setStatus("saving…");
    savingChain = savingChain
      .then(function () {
        return api("save", { source: text });
      })
      .then(function (data) {
        applyOpsState(data);
        setStatus("saved");
      })
      .catch(function (err) {
        setStatus("error: " + err.message);
      });
    return savingChain;
  }

  pane.addEventListener("input", function () {
    renderPreview();
    if (!authoring) return;
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(saveNow, 400);
  });

  // -- ops state (ac-2) -------------------------------------------------------

  function applyOpsState(data) {
    if (!data || typeof data.opsAvailable !== "boolean") return;
    state.opsAvailable = data.opsAvailable;
    state.nodes = data.nodes || [];
    state.edges = data.edges || [];
    var addBtn = document.getElementById("add-node-btn");
    if (addBtn) addBtn.disabled = !state.opsAvailable;
    var region = document.getElementById("diagram-editor-region");
    var notice = region && region.querySelector('[data-testid="ops-unavailable"]');
    if (!state.opsAvailable && !notice && region) {
      var notices = region.querySelector(".board-notices");
      if (notices) {
        var div = document.createElement("div");
        div.className = "board-notice";
        div.setAttribute("data-testid", "ops-unavailable");
        div.setAttribute("role", "status");
        div.textContent =
          "structural operations are unavailable on this source: " +
          (data.opsUnavailable || "outside the op grammar's flowchart subset") +
          " Type in the code pane as usual.";
        notices.appendChild(div);
      }
    } else if (state.opsAvailable && notice) {
      notice.remove();
    }
  }

  // An op lands as the server's deterministic text edit: flush any
  // pending save first (the op applies to the artifact's current
  // source), then swap the pane to the server's post-op source.
  function runOp(action, body) {
    if (saveTimer) {
      clearTimeout(saveTimer);
      saveTimer = null;
      saveNow();
    }
    setStatus("saving…");
    return savingChain
      .then(function () {
        return api(action, body);
      })
      .then(function (data) {
        pane.value = data.source;
        applyOpsState(data);
        renderPreview();
        setStatus("saved");
        return data;
      })
      .catch(function (err) {
        setStatus("error: " + err.message);
        renderPreview();
        throw err;
      });
  }

  // -- preview gestures: the structural operations ---------------------------
  //
  // The server's parsed node model (state.nodes) is mapped onto the
  // rendered SVG: mermaid's flowchart nodes carry DOM ids of the form
  // "flowchart-<id>-<n>". Matching tries each known id exactly, so
  // dashed ids never mis-split.

  function nodeIdOf(el) {
    var domId = el.id || "";
    for (var i = 0; i < (state.nodes || []).length; i++) {
      var id = state.nodes[i].id;
      var re = new RegExp("^flowchart-" + id.replace(/[.*+?^${}()|[\]\\-]/g, "\\$&") + "-\\d+$");
      if (re.test(domId)) return id;
    }
    return null;
  }

  function edgeEndpointsOf(el) {
    var from = null;
    var to = null;
    el.classList.forEach(function (cls) {
      if (cls.indexOf("LS-") === 0) from = cls.slice(3);
      if (cls.indexOf("LE-") === 0) to = cls.slice(3);
    });
    if (from && to) return { from: from, to: to };
    return null;
  }

  // annotatePreview stamps data hooks on the rendered SVG (stable
  // contract for gestures and e2e): data-node-id on each recognized
  // node, data-from/data-to on each recognized edge path — plus, per
  // edge, a transparent wide-stroke HIT twin over the same geometry, so
  // a hand (or a test) can actually land on a hairline curve. The twin
  // is pure gesture surface: presentation-less, never sent anywhere.
  function annotatePreview() {
    if (!state.opsAvailable) return;
    preview.querySelectorAll("g.node").forEach(function (g) {
      var id = nodeIdOf(g);
      if (id) g.setAttribute("data-node-id", id);
    });
    preview.querySelectorAll("path.flowchart-link").forEach(function (p) {
      var ends = edgeEndpointsOf(p);
      if (!ends) return;
      p.setAttribute("data-from", ends.from);
      p.setAttribute("data-to", ends.to);
      var hit = document.createElementNS("http://www.w3.org/2000/svg", "path");
      hit.setAttribute("d", p.getAttribute("d") || "");
      hit.setAttribute("class", "diagram-edge-hit");
      hit.setAttribute("data-from", ends.from);
      hit.setAttribute("data-to", ends.to);
      hit.setAttribute("fill", "none");
      hit.setAttribute("stroke", "transparent");
      hit.setAttribute("stroke-width", "14");
      hit.style.pointerEvents = "stroke";
      p.parentNode.insertBefore(hit, p.nextSibling);
    });
    restoreSelection();
  }

  var selectedNode = null; // node id awaiting its connect partner / toolbar
  var toolbox = null;

  function clearSelection() {
    selectedNode = null;
    preview.querySelectorAll("[data-node-selected]").forEach(function (g) {
      g.removeAttribute("data-node-selected");
    });
    removeToolbox();
  }

  function restoreSelection() {
    if (!selectedNode) return;
    var g = preview.querySelector('g.node[data-node-id="' + selectedNode + '"]');
    if (g) g.setAttribute("data-node-selected", "true");
  }

  function removeToolbox() {
    if (toolbox && toolbox.parentNode) toolbox.parentNode.removeChild(toolbox);
    toolbox = null;
  }

  function labelOf(id) {
    for (var i = 0; i < (state.nodes || []).length; i++) {
      if (state.nodes[i].id === id) return state.nodes[i].label || "";
    }
    return "";
  }

  // The node toolbox: a small popover the selected node wears — Rename
  // (inline input over the node), Delete, and the connect hint.
  function showToolbox(nodeEl, id) {
    removeToolbox();
    toolbox = document.createElement("div");
    toolbox.className = "diagram-node-toolbox";
    toolbox.setAttribute("data-testid", "node-toolbox");
    toolbox.setAttribute("data-node", id);

    var hint = document.createElement("span");
    hint.className = "diagram-node-toolbox-hint";
    hint.textContent = id;
    toolbox.appendChild(hint);

    var renameBtn = document.createElement("button");
    renameBtn.type = "button";
    renameBtn.textContent = "Rename";
    renameBtn.setAttribute("data-testid", "rename-node-btn");
    renameBtn.addEventListener("click", function (ev) {
      ev.stopPropagation();
      startInlineRename(nodeEl, id);
    });
    toolbox.appendChild(renameBtn);

    var delBtn = document.createElement("button");
    delBtn.type = "button";
    delBtn.textContent = "Delete";
    delBtn.setAttribute("data-testid", "delete-node-btn");
    delBtn.addEventListener("click", function (ev) {
      ev.stopPropagation();
      clearSelection();
      runOp("delete-node", { id: id }).catch(function () {});
    });
    toolbox.appendChild(delBtn);

    positionOver(toolbox, nodeEl, 8);
  }

  // startInlineRename: the rename-inline gesture (ac-2) — an input over
  // the node's own footprint; Enter commits, Escape abandons. Only the
  // label travels; the id is immutable through the op (dc-2).
  function startInlineRename(nodeEl, id) {
    removeToolbox();
    var input = document.createElement("input");
    input.className = "diagram-rename-input";
    input.setAttribute("aria-label", "Node label");
    input.setAttribute("data-testid", "rename-input");
    input.value = labelOf(id) || id;
    positionOver(input, nodeEl, 0);
    input.focus();
    input.select();
    var done = false;
    function commit() {
      if (done) return;
      done = true;
      var label = input.value;
      input.remove();
      clearSelection();
      runOp("rename", { id: id, label: label }).catch(function () {});
    }
    function abandon() {
      if (done) return;
      done = true;
      input.remove();
      clearSelection();
    }
    input.addEventListener("keydown", function (ev) {
      if (ev.key === "Enter") commit();
      if (ev.key === "Escape") abandon();
    });
    input.addEventListener("blur", commit);
  }

  // positionOver places el (absolutely, in the stage's coordinate space)
  // over/under target. Pure presentation — nothing here is ever sent to
  // the server (co-2: no position field exists in any request).
  function positionOver(el, target, dy) {
    var wrap = preview.parentNode; // .diagram-preview-wrap (position:relative)
    var wr = wrap.getBoundingClientRect();
    var tr = target.getBoundingClientRect();
    el.style.position = "absolute";
    el.style.left = Math.max(0, tr.left - wr.left) + "px";
    el.style.top = Math.max(0, tr.bottom - wr.top + dy) + "px";
    wrap.appendChild(el);
  }

  // Click-click connect + selection, and edge selection for delete.
  preview.parentNode.addEventListener("click", function (ev) {
    if (!authoring || !state.opsAvailable) return;
    var nodeEl = ev.target.closest && ev.target.closest("g.node[data-node-id]");
    if (nodeEl && preview.contains(nodeEl)) {
      var id = nodeEl.getAttribute("data-node-id");
      if (selectedNode && selectedNode !== id) {
        // click-click: the second click connects (ac-2).
        var from = selectedNode;
        clearSelection();
        runOp("connect", { from: from, to: id }).catch(function () {});
        return;
      }
      clearSelection();
      selectedNode = id;
      nodeEl.setAttribute("data-node-selected", "true");
      showToolbox(nodeEl, id);
      return;
    }
    var edgeEl =
      ev.target.closest &&
      ev.target.closest("path.flowchart-link[data-from], path.diagram-edge-hit[data-from]");
    if (edgeEl && preview.contains(edgeEl)) {
      clearSelection();
      showEdgeToolbox(edgeEl);
      return;
    }
    clearSelection();
  });

  function showEdgeToolbox(edgeEl) {
    removeToolbox();
    var from = edgeEl.getAttribute("data-from");
    var to = edgeEl.getAttribute("data-to");
    toolbox = document.createElement("div");
    toolbox.className = "diagram-node-toolbox";
    toolbox.setAttribute("data-testid", "edge-toolbox");

    var hint = document.createElement("span");
    hint.className = "diagram-node-toolbox-hint";
    hint.textContent = from + " --> " + to;
    toolbox.appendChild(hint);

    var delBtn = document.createElement("button");
    delBtn.type = "button";
    delBtn.textContent = "Delete edge";
    delBtn.setAttribute("data-testid", "delete-edge-btn");
    delBtn.addEventListener("click", function (ev) {
      ev.stopPropagation();
      removeToolbox();
      runOp("delete-edge", { from: from, to: to }).catch(function () {});
    });
    toolbox.appendChild(delBtn);
    positionOver(toolbox, edgeEl, 8);
  }

  // Drag-to-connect (ac-2): pointerdown on a node, pointerup on another
  // connects them — an edge line, nothing spatial. A drag that connects
  // nothing produces nothing (the release simply clears). The gesture
  // suppresses the synthetic click that follows a real drag so the
  // click-click path stays independent.
  var drag = null;
  preview.parentNode.addEventListener("pointerdown", function (ev) {
    if (!authoring || !state.opsAvailable) return;
    var nodeEl = ev.target.closest && ev.target.closest("g.node[data-node-id]");
    if (!nodeEl || !preview.contains(nodeEl)) return;
    drag = {
      from: nodeEl.getAttribute("data-node-id"),
      x: ev.clientX,
      y: ev.clientY,
      moved: false,
    };
  });
  preview.parentNode.addEventListener("pointermove", function (ev) {
    if (!drag) return;
    if (Math.abs(ev.clientX - drag.x) + Math.abs(ev.clientY - drag.y) > 6) {
      drag.moved = true;
    }
  });
  preview.parentNode.addEventListener("pointerup", function (ev) {
    if (!drag) return;
    var d = drag;
    drag = null;
    if (!d.moved) return; // a plain click; the click handler owns it
    suppressNextClick();
    var over = document.elementFromPoint(ev.clientX, ev.clientY);
    var nodeEl = over && over.closest && over.closest("g.node[data-node-id]");
    if (nodeEl && preview.contains(nodeEl)) {
      var to = nodeEl.getAttribute("data-node-id");
      if (to && to !== d.from) {
        clearSelection();
        runOp("connect", { from: d.from, to: to }).catch(function () {});
        return;
      }
    }
    // connected nothing: produced nothing.
  });

  function suppressNextClick() {
    var once = function (ev) {
      ev.stopPropagation();
      ev.preventDefault();
      document.removeEventListener("click", once, true);
    };
    document.addEventListener("click", once, true);
    setTimeout(function () {
      document.removeEventListener("click", once, true);
    }, 0);
  }

  // -- add node (dialog) ------------------------------------------------------

  var backdrop = document.getElementById("modal-backdrop");
  function openDialog(el) {
    if (backdrop) backdrop.hidden = false;
    el.hidden = false;
  }
  function closeDialog(el) {
    if (backdrop) backdrop.hidden = true;
    el.hidden = true;
  }

  var addBtn = document.getElementById("add-node-btn");
  var addDialog = document.getElementById("add-node-dialog");
  if (addBtn && addDialog) {
    var addLabel = document.getElementById("add-node-label");
    addBtn.addEventListener("click", function () {
      addLabel.value = "";
      openDialog(addDialog);
      addLabel.focus();
    });
    document.getElementById("add-node-ok").addEventListener("click", function () {
      var label = addLabel.value;
      closeDialog(addDialog);
      runOp("add-node", { label: label }).catch(function () {});
    });
    document.getElementById("add-node-cancel").addEventListener("click", function () {
      closeDialog(addDialog);
    });
  }

  // -- before-peek / reset (ac-4) --------------------------------------------

  var peekBtn = document.getElementById("peek-btn");
  var peekPanel = document.getElementById("diagram-peek");
  if (peekBtn && peekPanel) {
    var peekPreview = document.getElementById("diagram-peek-preview");
    var peekFailure = document.getElementById("diagram-peek-failure");
    var peekFailureMsg = document.getElementById("diagram-peek-failure-msg");
    peekBtn.addEventListener("click", function () {
      peekPanel.hidden = false;
      peekFailure.hidden = true;
      peekPreview.innerHTML = "";
      api("peek", {})
        .then(function (data) {
          renderInto(peekPreview, data.base, "peek-" + state.name, function (err) {
            if (err) {
              peekPreview.innerHTML = "";
              peekFailureMsg.textContent = String((err && err.message) || err);
              peekFailure.hidden = false;
            }
          });
        })
        .catch(function (err) {
          // The disclosed failure (digest mismatch, unrecoverable base):
          // painted in the peek panel, nothing written anywhere.
          peekPreview.innerHTML = "";
          peekFailureMsg.textContent = err.message;
          peekFailure.hidden = false;
        });
    });
    var peekClose = document.getElementById("peek-close-btn");
    if (peekClose) {
      peekClose.addEventListener("click", function () {
        peekPanel.hidden = true;
      });
    }
  }

  var resetBtn = document.getElementById("reset-btn");
  var resetConfirm = document.getElementById("reset-confirm");
  if (resetBtn && resetConfirm) {
    resetBtn.addEventListener("click", function () {
      openDialog(resetConfirm);
    });
    document.getElementById("reset-confirm-cancel").addEventListener("click", function () {
      closeDialog(resetConfirm);
    });
    document.getElementById("reset-confirm-ok").addEventListener("click", function () {
      closeDialog(resetConfirm);
      runOp("reset", {}).catch(function (err) {
        // The disclosed mismatch failure also paints in the peek panel's
        // failure slot when present — the affordance failed visibly and
        // wrote nothing.
        var peekFailureEl = document.getElementById("diagram-peek-failure");
        var peekMsg = document.getElementById("diagram-peek-failure-msg");
        var panel = document.getElementById("diagram-peek");
        if (panel && peekFailureEl && peekMsg) {
          panel.hidden = false;
          var pv = document.getElementById("diagram-peek-preview");
          if (pv) pv.innerHTML = "";
          peekMsg.textContent = err.message;
          peekFailureEl.hidden = false;
        }
      });
    });
  }

  // -- exit affordance + Escape (spec/tool-view-exit ac-1) -------------------
  //
  // The return target is resolved once, server-side, at render (dc-2): the
  // state blob carries the exact href, honest fallback included (dc-3) —
  // this script only navigates, it never derives or guesses a target. A
  // modal dialog (the shared backdrop visible) or the inline rename editor
  // already owns Escape while either is open, so the page-level exit stands
  // down rather than discarding an in-progress gesture.
  document.addEventListener("keydown", function (ev) {
    if (ev.key !== "Escape" || !state.exitHref) return;
    var modalBackdrop = document.getElementById("modal-backdrop");
    if (modalBackdrop && !modalBackdrop.hidden) return;
    if (document.querySelector(".diagram-rename-input")) return;
    window.location.href = state.exitHref;
  });

  // first paint
  renderPreview();
})();
