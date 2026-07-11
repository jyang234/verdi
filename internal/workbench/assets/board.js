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

  // -- yarn: real thread, drawn as an SVG overlay -------------------------
  // Each yarn entry {from, to, label} names endpoints as "pin:<ref>" /
  // "sticky:<id>". The overlay redraws from the elements' live positions,
  // so threads follow their pins and stickies through a drag. Unresolvable
  // endpoints simply draw nothing — the yarn ledger list still shows them.
  var svgNS = "http://www.w3.org/2000/svg";
  var yarnSvg = null;

  function ensureYarnSvg() {
    if (yarnSvg || !canvas) return yarnSvg;
    yarnSvg = document.createElementNS(svgNS, "svg");
    yarnSvg.setAttribute("class", "yarn-svg");
    yarnSvg.setAttribute("aria-hidden", "true");
    canvas.insertBefore(yarnSvg, canvas.firstChild);
    return yarnSvg;
  }

  function endpointElement(endpoint) {
    var sep = endpoint.indexOf(":");
    if (sep < 0) return null;
    var kind = endpoint.slice(0, sep);
    var key = endpoint.slice(sep + 1);
    var els = canvas.querySelectorAll('[data-drag][data-kind="' + kind + '"]');
    for (var i = 0; i < els.length; i++) {
      var k = els[i].getAttribute("data-key");
      // A pin endpoint may name the unpinned ref while the card carries the
      // pinned form ("spec/x" vs "spec/x@<sha>") — match either.
      if (k === key || (kind === "pin" && k.indexOf(key + "@") === 0)) {
        return els[i];
      }
    }
    return null;
  }

  function centerOf(el) {
    return {
      x: el.offsetLeft + el.offsetWidth / 2,
      y: el.offsetTop + el.offsetHeight / 2,
    };
  }

  function drawYarn() {
    var svg = ensureYarnSvg();
    if (!svg) return;
    while (svg.firstChild) svg.removeChild(svg.firstChild);
    for (var i = 0; i < state.yarn.length; i++) {
      var y = state.yarn[i];
      var fromEl = endpointElement(y.from);
      var toEl = endpointElement(y.to);
      if (!fromEl || !toEl) continue;
      var a = centerOf(fromEl);
      var b = centerOf(toEl);
      // A hung thread: quadratic curve whose control point sags below the
      // midpoint, deeper for longer spans.
      var dx = b.x - a.x;
      var dy = b.y - a.y;
      var sag = 18 + Math.sqrt(dx * dx + dy * dy) * 0.12;
      var cx = (a.x + b.x) / 2;
      var cy = (a.y + b.y) / 2 + sag;

      var path = document.createElementNS(svgNS, "path");
      path.setAttribute("class", "yarn-thread");
      path.setAttribute(
        "d",
        "M " + a.x + " " + a.y + " Q " + cx + " " + cy + " " + b.x + " " + b.y
      );
      svg.appendChild(path);

      var ends = [a, b];
      for (var j = 0; j < ends.length; j++) {
        var knot = document.createElementNS(svgNS, "circle");
        knot.setAttribute("class", "yarn-knot");
        knot.setAttribute("cx", ends[j].x);
        knot.setAttribute("cy", ends[j].y);
        knot.setAttribute("r", 3);
        svg.appendChild(knot);
      }

      if (y.label) {
        // The curve's own midpoint (t=0.5 on the quadratic).
        var mx = 0.25 * a.x + 0.5 * cx + 0.25 * b.x;
        var my = 0.25 * a.y + 0.5 * cy + 0.25 * b.y;
        var text = document.createElementNS(svgNS, "text");
        text.setAttribute("class", "yarn-thread-label");
        text.setAttribute("x", mx);
        text.setAttribute("y", my - 6);
        text.setAttribute("text-anchor", "middle");
        text.textContent = y.label;
        svg.appendChild(text);
      }
    }
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
    el.classList.add("dragging");
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
    drawYarn(); // threads follow their pins and stickies live
  }

  function onPointerUp() {
    if (!dragging) return;
    dragging.el.classList.remove("dragging");
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
    drawYarn();
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
