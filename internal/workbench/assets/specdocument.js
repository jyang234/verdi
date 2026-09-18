// The board's Document tab (spec/spec-documents ac-4): conditional
// refresh, copy, and nothing else. The page is complete before this
// script runs — the kind switch and the download are plain links — so
// the script only keeps the rendered document current and hands the
// exact Markdown bytes to the clipboard.
//
//   - conditional refresh (Wave 6 rules, dc-2): GET <page>/snapshot every
//     2 s ONLY while the page is visible, If-None-Match with the last
//     exact revision token, 304 leaves the page untouched, pause on
//     hidden with one immediate conditional refresh on visible, plus the
//     keyboard-reachable Refresh button;
//   - the swap replaces only the document region, keeping focus (an
//     element with an id inside the region is re-found by id) and the
//     scroll position;
//   - Copy Markdown writes the hidden <pre>'s text — the same bytes the
//     snapshot's markdown field and the ?format=md download carry — and
//     announces the outcome through the one role="status" line.
//
// GET only; no route this script touches ever writes (co-2).
(function () {
  "use strict";
  var region = document.getElementById("document-region");
  if (!region) return;
  var statusEl = document.getElementById("document-status");
  var mdEl = document.getElementById("document-markdown");
  var refreshBtn = document.getElementById("document-refresh");
  var copyBtn = document.getElementById("document-copy");
  var revision = region.getAttribute("data-revision") || "";
  var snapshotHref = region.getAttribute("data-snapshot-href") || "";

  function say(text) {
    if (statusEl) statusEl.textContent = text;
  }

  function apply(snap) {
    var y = window.scrollY;
    var active = document.activeElement;
    var activeID = active && region.contains(active) ? active.id : "";
    region.innerHTML = snap.html;
    region.setAttribute("data-revision", snap.revision);
    revision = snap.revision;
    if (mdEl) mdEl.textContent = snap.markdown;
    if (activeID) {
      var el = document.getElementById(activeID);
      if (el && typeof el.focus === "function") el.focus();
    }
    window.scrollTo(0, y);
    say("Updated");
  }

  // refreshSeq is the supersession guard (boardspecasd.js's idiom): the
  // newest issued refresh owns the region; a stale in-flight response is
  // dropped rather than overwriting a newer one.
  var refreshSeq = 0;
  var pollBusy = false;

  function refresh(force) {
    var seq = ++refreshSeq;
    var headers = { "If-None-Match": '"' + revision + '"' };
    return fetch(snapshotHref, { headers: headers, credentials: "same-origin" })
      .then(function (res) {
        if (res.status === 304) {
          if (force) say("Up to date");
          return null;
        }
        if (!res.ok) {
          say("Refresh failed (" + res.status + ")");
          return null;
        }
        return res.json();
      })
      .then(function (snap) {
        if (!snap || seq !== refreshSeq) return;
        apply(snap);
      })
      .catch(function () {
        say("Refresh failed");
      });
  }

  if (refreshBtn) {
    refreshBtn.addEventListener("click", function () {
      refresh(true);
    });
  }

  if (copyBtn) {
    copyBtn.addEventListener("click", function () {
      var text = mdEl ? mdEl.textContent : "";
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(
          function () { say("Copied Markdown"); },
          function () { say("Copy failed"); }
        );
      } else {
        say("Copy unavailable");
      }
    });
  }

  // 2 s visible-only polling; a tick is skipped while the previous poll is
  // still in flight so fetches never stack; pause on hidden; one immediate
  // conditional refresh on visible.
  setInterval(function () {
    if (document.hidden || pollBusy) return;
    pollBusy = true;
    refresh(false).then(
      function () { pollBusy = false; },
      function () { pollBusy = false; }
    );
  }, 2000);
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden) refresh(false);
  });
})();
