// The Document page's chrome script (spec/document-page-v2; ledger
// SI-340 (5), (8), (10)–(12); SI-343 (2)). The page is complete before
// this script runs — the temporal stamp, the identity card, and the
// contents rail are server-rendered (ac-4) — so it adds only what needs
// a clock or a wall it can reach:
//
//   - the refreshed time (dc-2): computed here, never by the server, from
//     the page's own load and from every completed check the document
//     script reports — the verdi:document-refresh event specdocument.js
//     dispatches on each poll or Refresh with its HTTP status, 200 and
//     304 alike being completed checks. It sits outside the live region;
//     data-refreshed-at holds the last check's time, so a test can see
//     it move.
//   - the id chips (dc-1): one link per eligible anchor the page's facts
//     list, to <board href>#obj-<id>, in the object kind's colour word,
//     drawn right at the body's own anchor. They are drawn only here, so
//     a script-less reader, an archived spec, and the docs site never
//     carry a chip that cannot work.
//   - the chrome refresh (SI-340 (8)): when a poll brings a new revision,
//     the stamp, the identity card, and the rail are redrawn from that
//     snapshot's facts once the body has been swapped to that revision,
//     so the chrome never shows another revision than the body; the chips
//     are redrawn at the new body's anchors, and the focus the swap moved
//     is put back where the reader had it.
//
// This script never fetches: it reads what specdocument.js fetched.
(function () {
  "use strict";
  var region = document.getElementById("document-region");
  var stamp = document.getElementById("document-stamp");
  var refreshed = document.getElementById("document-refreshed");
  var identity = document.getElementById("document-identity");
  var rail = document.getElementById("document-contents-list");
  if (!region || !stamp || !refreshed) return;
  var boardHref = region.getAttribute("data-board-href") || "";
  var chips = [];
  var chipsEl = document.getElementById("document-chips");
  try {
    chips = JSON.parse(chipsEl ? chipsEl.textContent : "[]") || [];
  } catch (e) {
    chips = [];
  }

  // ---- the refreshed time ----
  var lastCheck = 0;
  function ago(ms) {
    var s = Math.max(0, Math.floor(ms / 1000));
    if (s < 60) return s + " s ago";
    var m = Math.floor(s / 60);
    if (m < 60) return m + " min ago";
    return Math.floor(m / 60) + " h ago";
  }
  function tick() {
    if (!lastCheck) return;
    refreshed.textContent = "refreshed " + ago(Date.now() - lastCheck);
  }
  function checked() {
    lastCheck = Date.now();
    refreshed.setAttribute("data-refreshed-at", new Date(lastCheck).toISOString());
    tick();
  }
  setInterval(tick, 1000);

  // ---- the id chips ----
  function shortCommit(c) {
    return c && c.length > 8 ? c.slice(0, 8) : c || "";
  }
  function clearChips() {
    var old = region.querySelectorAll(".document-chip");
    for (var i = 0; i < old.length; i++) old[i].parentNode.removeChild(old[i]);
  }
  function anchorFor(id) {
    var all = region.querySelectorAll("a[id]");
    for (var i = 0; i < all.length; i++) if (all[i].id === id) return all[i];
    return null;
  }
  function focusChip(id) {
    var all = region.querySelectorAll(".document-chip");
    for (var i = 0; i < all.length; i++) {
      if (all[i].getAttribute("data-id") === id) {
        all[i].focus({ preventScroll: true });
        return;
      }
    }
  }
  // drawChips redraws every chip from list; a chip the reader has
  // focused is focused again by its id, so a redraw never drops focus.
  function drawChips(list) {
    var active = document.activeElement;
    var keep = active && active.classList && active.classList.contains("document-chip") ? active.getAttribute("data-id") : null;
    clearChips();
    for (var i = 0; boardHref && i < list.length; i++) {
      var c = list[i];
      if (!c || !c.id || !c.kind) continue;
      var anchor = anchorFor(c.id);
      if (!anchor) continue;
      var chip = document.createElement("a");
      chip.className = "document-chip document-chip--" + c.kind;
      chip.setAttribute("data-object-kind", c.kind);
      chip.setAttribute("data-id", c.id);
      chip.setAttribute("data-testid", "document-chip-" + c.id);
      chip.href = boardHref + "#obj-" + c.id;
      chip.title = "Show " + c.id + " on the wall";
      chip.textContent = c.id;
      anchor.parentNode.insertBefore(chip, anchor);
    }
    if (keep) focusChip(keep);
  }

  // ---- the chrome, redrawn from a snapshot's facts ----
  function cell(slug) {
    return identity ? identity.querySelector('[data-testid="document-identity-' + slug + '"]') : null;
  }
  function applyStamp(s) {
    stamp.className = "document-stamp document-stamp--" + (s.state || "");
    stamp.setAttribute("data-state", s.state || "");
    stamp.setAttribute("data-commit", s.commit || "");
    var words = stamp.querySelector(".document-stamp-words");
    if (words) words.textContent = s.words || "";
    var commit = stamp.querySelector('[data-testid="document-stamp-commit"]');
    if (commit) {
      commit.textContent = shortCommit(s.commit);
      commit.title = s.commit || "";
    }
  }
  function applyBranch(el, b, detached) {
    el.textContent = "";
    el.removeAttribute("data-detached");
    el.removeAttribute("title");
    if (b.unproven) {
      el.setAttribute("data-state", "unproven");
      el.title = b.unproven;
      el.appendChild(document.createTextNode(b.text || ""));
      var why = document.createElement("span");
      why.className = "document-identity-why";
      why.textContent = b.unproven;
      el.appendChild(why);
      return;
    }
    el.setAttribute("data-state", "proven");
    if (detached) {
      el.setAttribute("data-detached", "true");
      el.textContent = "detached HEAD";
      return;
    }
    el.textContent = b.text || "";
  }
  function applyIdentity(id) {
    var ref = cell("ref");
    if (ref) {
      ref.textContent = "";
      var code = document.createElement("code");
      code.textContent = id.ref || "";
      ref.appendChild(code);
    }
    var cls = cell("class");
    if (cls) {
      if (id.classLabel) {
        cls.removeAttribute("data-state");
        cls.textContent = id.classLabel;
      } else {
        cls.setAttribute("data-state", "none");
        cls.textContent = "not declared";
      }
    }
    var branch = cell("branch");
    if (branch) applyBranch(branch, id.branch || {}, !!id.detached);
    var owners = cell("owners");
    if (owners) {
      if (id.owners && id.owners.length) {
        owners.removeAttribute("data-state");
        owners.textContent = id.owners.join(", ");
      } else {
        owners.setAttribute("data-state", "none");
        owners.textContent = "none declared";
      }
    }
    var files = cell("files");
    if (files) {
      files.textContent = "";
      (id.files || []).forEach(function (f) {
        var code = document.createElement("code");
        code.textContent = f;
        files.appendChild(code);
      });
    }
  }
  // applyRail rebuilds the rail from entries; a rail link the reader has
  // focused is focused again by its target.
  function applyRail(entries) {
    if (!rail) return;
    var active = document.activeElement;
    var keep = active && rail.contains(active) && active.getAttribute ? active.getAttribute("href") : null;
    rail.textContent = "";
    entries.forEach(function (e) {
      var li = document.createElement("li");
      li.setAttribute("data-testid", "document-contents-" + e.id);
      var a = document.createElement("a");
      a.href = "#" + e.id;
      var text = document.createElement("span");
      text.className = "document-contents-text";
      text.textContent = e.text || "";
      a.appendChild(text);
      if (e.count !== undefined && e.count !== null) {
        li.setAttribute("data-count", String(e.count));
        var n = document.createElement("span");
        n.className = "document-contents-count";
        n.setAttribute("data-testid", "document-contents-" + e.id + "-count");
        n.textContent = String(e.count);
        a.appendChild(n);
      }
      li.appendChild(a);
      rail.appendChild(li);
    });
    if (keep) {
      var links = rail.querySelectorAll("a[href]");
      for (var i = 0; i < links.length; i++) {
        if (links[i].getAttribute("href") === keep) {
          links[i].focus({ preventScroll: true });
          break;
        }
      }
    }
  }
  function applyFacts(f) {
    applyStamp(f.stamp || {});
    applyIdentity(f.identity || {});
    applyRail(f.rail || []);
    chips = f.chips || [];
  }

  // ---- focus across a swap ----
  // specdocument.js restores focus by index among the region's links and
  // buttons, a count the chips it does not know about would skew; the
  // record here is taken before the swap, chip-free — a chip by its id,
  // anything else by its index among the body's own controls — and
  // applied after the chips are back.
  var focusRecord = null;
  function bodyControls() {
    var all = region.querySelectorAll("a[href],button");
    var out = [];
    for (var i = 0; i < all.length; i++) if (!all[i].classList.contains("document-chip")) out.push(all[i]);
    return out;
  }
  function recordFocus() {
    var active = document.activeElement;
    focusRecord = null;
    if (!active || !region.contains(active)) return;
    if (active.classList.contains("document-chip")) {
      focusRecord = { chip: active.getAttribute("data-id") };
      return;
    }
    focusRecord = { index: bodyControls().indexOf(active) };
  }
  function restoreFocus() {
    var r = focusRecord;
    if (!r) return;
    focusRecord = null;
    if (r.chip) {
      focusChip(r.chip);
      return;
    }
    var el = r.index >= 0 ? bodyControls()[r.index] || null : null;
    if (el && typeof el.focus === "function") el.focus({ preventScroll: true });
  }

  // ---- wiring ----
  // pending holds a snapshot's facts by revision until the body carries
  // that revision: whichever lands first, the facts or the swap, the
  // chrome is redrawn once the two agree, and never for a revision the
  // body does not show.
  var pending = {};
  function applyPending() {
    var rev = region.getAttribute("data-revision") || "";
    var f = pending[rev];
    if (!f) return false;
    pending = {};
    applyFacts(f);
    drawChips(chips);
    restoreFocus();
    return true;
  }
  region.addEventListener("verdi:document-refresh", function (e) {
    var d = e.detail || {};
    if (d.status === 200) {
      recordFocus();
      if (d.snapshot && typeof d.snapshot.then === "function") {
        d.snapshot.then(
          function (snap) {
            if (snap && snap.revision && snap.facts) {
              pending[snap.revision] = snap.facts;
              applyPending();
            }
          },
          function () {}
        );
      }
    }
    if (d.status === 200 || d.status === 304) checked();
  });
  new MutationObserver(function () {
    if (!applyPending()) {
      drawChips(chips);
      restoreFocus();
    }
  }).observe(region, { attributes: true, attributeFilter: ["data-revision"] });

  drawChips(chips);
  checked();
})();
