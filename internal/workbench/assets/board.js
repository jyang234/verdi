// The board's entire client-side surface (05 §Workbench: "keep board JS
// minimal and in ONE file — the workbench is localhost-only; no
// framework"). Vanilla DOM + fetch, no build step, no dependency.
//
// Server contract (internal/workbench/board.go):
//   GET  /board/<key>            -> this page, with #board-state holding
//                                    the initial verdi.board/v1 JSON plus
//                                    resolved sticky bodies (window.__BOARD__)
//   POST /board/<key>/autosave   -> body: {pins,stickies,yarn} (verdi.board/v1
//                                    shape minus schema/frozen/provenance);
//                                    204 on success
//   POST /board/<key>/commit     -> body: {name, story_ref}; JSON result or
//                                    JSON {error}
(function () {
  "use strict";

  var boardKey = window.__BOARD_KEY__;
  var state = window.__BOARD__; // {pins, stickies, yarn} — mutated in place

  var canvas = document.getElementById("board-canvas");
  var statusEl = document.getElementById("autosave-status");

  function setStatus(text) {
    if (statusEl) statusEl.textContent = text;
  }

  function findPin(ref) {
    for (var i = 0; i < state.pins.length; i++) {
      if (state.pins[i].ref === ref) return state.pins[i];
    }
    return null;
  }
  function findSticky(id) {
    for (var i = 0; i < state.stickies.length; i++) {
      if (state.stickies[i].id === id) return state.stickies[i];
    }
    return null;
  }

  var saveTimer = null;
  function scheduleAutosave() {
    setStatus("saving…");
    if (saveTimer) clearTimeout(saveTimer);
    saveTimer = setTimeout(doAutosave, 250);
  }

  function doAutosave() {
    // The server's autosave payload shape is verdi.board/v1's mutable
    // subset — pins {ref,x,y}, stickies {id,x,y} ONLY, yarn {from,to,label}
    // — strict-decoded (unknown fields rejected). state.stickies carries
    // richer view fields (body/type/author/status, resolved server-side at
    // page load for rendering) that must never round-trip back into a
    // save: strip each sticky down to just what the schema accepts.
    var payload = {
      pins: state.pins,
      stickies: state.stickies.map(function (s) {
        return { id: s.id, x: s.x, y: s.y };
      }),
      yarn: state.yarn,
    };
    fetch("/board/" + encodeURIComponent(boardKey) + "/autosave", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    })
      .then(function (resp) {
        if (!resp.ok) throw new Error("autosave failed: " + resp.status);
        setStatus("saved");
      })
      .catch(function (err) {
        setStatus("autosave error: " + err.message);
      });
  }

  // -- dragging: plain pointer events, no HTML5 drag-and-drop (simpler to
  // drive from both a mouse and Playwright's synthetic mouse events) --
  var dragging = null; // {el, kind, ref, dx, dy}

  function onPointerDown(e) {
    var el = e.target.closest("[data-drag]");
    if (!el || !canvas.contains(el)) return;
    var rect = el.getBoundingClientRect();
    dragging = {
      el: el,
      kind: el.getAttribute("data-kind"),
      key: el.getAttribute("data-key"),
      dx: e.clientX - rect.left,
      dy: e.clientY - rect.top,
    };
    e.preventDefault();
  }

  function onPointerMove(e) {
    if (!dragging) return;
    var canvasRect = canvas.getBoundingClientRect();
    var x = e.clientX - canvasRect.left - dragging.dx;
    var y = e.clientY - canvasRect.top - dragging.dy;
    if (x < 0) x = 0;
    if (y < 0) y = 0;
    dragging.el.style.left = x + "px";
    dragging.el.style.top = y + "px";
  }

  function onPointerUp() {
    if (!dragging) return;
    var x = parseFloat(dragging.el.style.left) || 0;
    var y = parseFloat(dragging.el.style.top) || 0;
    if (dragging.kind === "pin") {
      var pin = findPin(dragging.key);
      if (pin) {
        pin.x = x;
        pin.y = y;
      }
    } else if (dragging.kind === "sticky") {
      var sticky = findSticky(dragging.key);
      if (sticky) {
        sticky.x = x;
        sticky.y = y;
      }
    }
    dragging = null;
    scheduleAutosave();
  }

  if (canvas) {
    canvas.addEventListener("mousedown", onPointerDown);
    document.addEventListener("mousemove", onPointerMove);
    document.addEventListener("mouseup", onPointerUp);
  }

  // -- commit-to-design --
  var commitForm = document.getElementById("commit-form");
  if (commitForm) {
    commitForm.addEventListener("submit", function (e) {
      e.preventDefault();
      var name = document.getElementById("commit-name").value;
      var storyRef = document.getElementById("commit-story-ref").value;
      var resultEl = document.getElementById("commit-result");
      resultEl.textContent = "committing…";
      fetch("/board/" + encodeURIComponent(boardKey) + "/commit", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: name, story_ref: storyRef }),
      })
        .then(function (resp) {
          return resp.json().then(function (body) {
            if (!resp.ok) throw new Error(body.error || "commit failed");
            return body;
          });
        })
        .then(function (body) {
          resultEl.textContent =
            "committed " + body.commit + ": " + body.spec_ref + " (" + body.dispositions + " sticky(s) dispositioned)";
        })
        .catch(function (err) {
          resultEl.textContent = "error: " + err.message;
        });
    });
  }
})();
