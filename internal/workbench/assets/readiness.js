// Wave 3.5 readiness pilot instrumentation — page-memory only.
//
// Closed event vocabulary:
//   readiness-opened, area-inspected, concern-inspected,
//   board-link-followed, cli-fallback-copied, stale-notice-inspected.
//
// Every event is {sequence, event, area_id, concern_id}. The final 200
// events live in window.__verdiReadinessPilotEvents and each one is also
// dispatched as a same-page "verdi:readiness-pilot" CustomEvent. Nothing
// here leaves the browser, persists anywhere, or influences rendering:
// this file registers listeners and appends to one in-memory array.
(function () {
  "use strict";

  var MAX_EVENTS = 200;
  var events = [];
  var sequence = 0;
  window.__verdiReadinessPilotEvents = events;

  function record(name, areaID, concernID) {
    sequence += 1;
    var event = {
      sequence: sequence,
      event: name,
      area_id: areaID || "",
      concern_id: concernID || ""
    };
    events.push(event);
    if (events.length > MAX_EVENTS) {
      events.splice(0, events.length - MAX_EVENTS);
    }
    try {
      document.dispatchEvent(new CustomEvent("verdi:readiness-pilot", { detail: event }));
    } catch (err) {
      // Dispatch failure must never affect the page.
    }
  }

  function closestAttr(el, selector, attr) {
    var hit = el && el.closest ? el.closest(selector) : null;
    return hit ? hit.getAttribute(attr) || "" : "";
  }

  function areaOf(el) {
    return closestAttr(el, "[data-area-id]", "data-area-id");
  }

  function concernOf(el) {
    return closestAttr(el, "[data-concern-id]", "data-concern-id");
  }

  document.addEventListener("click", function (e) {
    var target = e.target instanceof Element ? e.target : null;
    if (!target) {
      return;
    }
    var boardLink = target.closest("a.readiness-board-link");
    if (boardLink) {
      // Appended synchronously in the click phase, BEFORE the browser's
      // default _blank navigation opens the separate tab; this source
      // tab keeps the sequence. Never preventDefault here.
      record("board-link-followed", areaOf(boardLink), concernOf(boardLink));
      return;
    }
    if (target.closest("[data-readiness-stale]")) {
      record("stale-notice-inspected", "", "");
      return;
    }
    var concern = target.closest("[data-concern-id]");
    if (concern) {
      record("concern-inspected", areaOf(concern), concernOf(concern));
      return;
    }
    var area = target.closest("[data-area-id]");
    if (area) {
      record("area-inspected", areaOf(area), "");
    }
  });

  // Keyboard path for the (non-activatable) stale notice: it carries
  // tabindex="0", so focusing it is its inspection.
  document.addEventListener("focusin", function (e) {
    var target = e.target instanceof Element ? e.target : null;
    if (target && target.matches("[data-readiness-stale]")) {
      record("stale-notice-inspected", "", "");
    }
  });

  // Copying from a CLI fallback vector is the fallback actually being
  // taken; the browser's own copy behavior is untouched.
  document.addEventListener("copy", function () {
    var selection = document.getSelection();
    var node = selection ? selection.anchorNode : null;
    if (!node) {
      return;
    }
    var el = node.nodeType === 1 ? node : node.parentElement;
    var cli = el && el.closest ? el.closest("[data-readiness-cli]") : null;
    if (cli) {
      record("cli-fallback-copied", areaOf(cli), concernOf(cli));
    }
  });

  record("readiness-opened", "", "");
})();
