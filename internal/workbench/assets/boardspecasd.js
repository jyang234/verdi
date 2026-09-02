// boardspecasd.js — the ASD workbench's route-scoped transport (Wave 6
// Task 2; SI-163/165/167/168). Dependency-free, ≤64KiB, no frameworks, no
// websocket/SSE, no storage authority: this file owns
//
//   - the conditional refresh loop: GET <page>/snapshot every 2s ONLY
//     while the page is visible, If-None-Match with the last exact
//     revision token, 304 leaves the page untouched, pause on hidden with
//     one immediate conditional refresh on visible, plus the
//     keyboard-reachable manual Refresh control;
//   - state preservation across refresh: unsaved dialog inputs live
//     OUTSIDE the swapped region, in-region interactions hold the swap
//     (boardspec.js's interaction contract), focus and expanded
//     disclosures are restored into the fresh region, and the last action
//     result lives outside the region;
//   - the typed mutation transport: ONE mutate_draft envelope per domain
//     gesture (built by boardspec.js from server-rendered facts), the
//     exact base digest/bytes and expected identity riding every request
//     so a stale action is refused by the application core with zero
//     mutation and answered with a fresh projection;
//   - the on-demand application panels (provenance / semantic review /
//     design context) — one explicit projection each, fetched only when
//     opened, never authority; and
//   - the typed-operation forms (set-problem / set-outcome / add-object /
//     stub correction) with field-level slug grammar validation from the
//     server's own pattern.
//
// No polling result ever writes; nothing here derives semantic state; the
// DOM is always the server's own projection.
(function () {
  "use strict";
  var boot = window.__BOARDV2__ || {};
  var asdState = boot.asd || null;
  if (!asdState) return; // no ASD state embedded: nothing to run

  var specName = boot.spec;
  var base = "/board/spec/" + specName;
  var prefix = window.location.pathname.indexOf("/b/") === 0
    ? window.location.pathname.slice(0, window.location.pathname.indexOf("/board/spec/"))
    : "";
  function url(rest) {
    return prefix + base + rest;
  }

  function api2() {
    return window.__BOARDV2API__ || null;
  }
  function setStatus(text) {
    var a = api2();
    if (a) a.setStatus(text);
  }

  // -- bounded live region --------------------------------------------------
  // Announce only CHANGES; an unchanged poll (304) never re-announces.
  var liveEl = document.getElementById("asd-live");
  function announce(text) {
    if (liveEl && liveEl.textContent !== text) liveEl.textContent = text;
  }

  // -- last action result (outside the region: survives every swap) --------
  var lastResultEl = document.getElementById("asd-last-result");
  function showResult(kind, text) {
    if (!lastResultEl) return;
    lastResultEl.setAttribute("data-result-kind", kind);
    lastResultEl.textContent = text;
  }

  // -- snapshot state -------------------------------------------------------
  var revision = asdState.revision;
  var baseDigest = asdState.baseDigest;
  var baseSpecB64 = asdState.baseSpecB64;
  var expected = asdState.expected;

  function adoptProjection(p) {
    if (typeof p.revision === "string" && p.revision) revision = p.revision;
    if (typeof p.base_digest === "string" && p.base_digest) baseDigest = p.base_digest;
    if (typeof p.base_spec_b64 === "string" && p.base_spec_b64) baseSpecB64 = p.base_spec_b64;
    if (p.expected && p.expected.head) expected = p.expected;
    if (typeof p.dirty === "boolean") {
      var a = api2();
      if (a) a.setDirty(p.dirty);
    }
  }

  // -- region swap with state preservation ----------------------------------
  function openDisclosureKeys() {
    var keys = [];
    var open = document.querySelectorAll("#boardv2-region details[open]");
    for (var i = 0; i < open.length; i++) {
      var key = open[i].getAttribute("data-testid") || open[i].id || open[i].className;
      if (key) keys.push(key);
    }
    return keys;
  }
  function reopenDisclosures(keys) {
    for (var i = 0; i < keys.length; i++) {
      var el =
        document.querySelector('#boardv2-region details[data-testid="' + keys[i] + '"]') ||
        document.getElementById(keys[i]);
      if (el && el.tagName === "DETAILS") el.setAttribute("open", "");
    }
  }
  function focusKey() {
    var el = document.activeElement;
    if (!el || el === document.body) return null;
    if (!el.closest || !el.closest("#boardv2-region")) return null;
    return el.getAttribute("data-testid") || el.id || null;
  }
  function restoreFocus(key) {
    if (!key) return;
    var el =
      document.querySelector('#boardv2-region [data-testid="' + key + '"]') ||
      document.getElementById(key);
    if (el && el.focus) el.focus({ preventScroll: true });
  }

  function applyRegion(html) {
    var a = api2();
    var keys = openDisclosureKeys();
    var focused = focusKey();
    if (a) {
      a.applyFragment(html);
    } else {
      var region = document.getElementById("boardv2-region");
      if (region) region.innerHTML = html;
    }
    reopenDisclosures(keys);
    restoreFocus(focused);
  }

  function applyProjection(p, announceText) {
    var a = api2();
    if (a && a.interactionLive()) {
      // Never yank the wall out from under the hand — AND never adopt the
      // fresh base under an open editor: the in-flight edit keeps the
      // stale base it was made against, so its save is refused by the
      // kernel's stale precondition and the conflict is VISIBLE (AC-2:
      // "keeps the stale base and forces a visible conflict"), never a
      // silent overwrite of what changed underneath. The interaction's
      // end re-fetches a fresh projection (resumeHeldRefresh routes back
      // through refresh()).
      a.noteHeld();
      return;
    }
    adoptProjection(p);
    if (typeof p.html === "string" && p.html) applyRegion(p.html);
    if (announceText) announce(announceText);
  }

  // -- conditional refresh --------------------------------------------------
  // refreshSeq is the supersession guard (the fragmentSeq contract carried
  // into the snapshot transport — the owner-witnessed 2026-07-19 rollback):
  // the NEWEST issued refresh owns the wall, a stale in-flight response is
  // discarded on arrival, and a mutation's own applied projection
  // invalidates every refresh issued before it. A superseded caller's
  // promise settles WITH the superseding run, so a mutation's "saved"
  // never fires before a projection at least as fresh as its own write
  // has actually applied.
  var refreshSeq = 0;
  var mutationSeq = 0;
  var latestRefresh = Promise.resolve();
  // force skips the conditional token: a refused mutation must reconcile
  // the wall to server truth even when the SERVER state never changed
  // (the refusal left only optimistic client-side divergence, which a 304
  // cannot repair) — and its application must not be displaced by a
  // concurrent conditional poll's 304 (which applies nothing). Only a
  // NEWER MUTATION application outranks a forced reconcile.
  function refresh(force) {
    var seq = ++refreshSeq;
    var mutationsAtStart = mutationSeq;
    var headers = force ? {} : { "If-None-Match": '"' + revision + '"' };
    var run;
    run = fetch(url("/snapshot"), { headers: headers })
      .then(function (resp) {
        if (resp.status === 304) return null; // unchanged: page untouched
        if (!resp.ok) throw new Error("snapshot: HTTP " + resp.status);
        return resp.json();
      })
      .then(function (snap) {
        if (force && snap && mutationSeq === mutationsAtStart) {
          applyProjection({ html: snap.html, revision: snap.revision, base_digest: snap.base_digest, base_spec_b64: snap.base_spec_b64, expected: snap.expected, dirty: snap.git ? snap.git.dirty : undefined });
          return undefined;
        }
        if (seq !== refreshSeq) {
          // Superseded. By a newer RUN: settle with it, so awaiters see a
          // projection at least as fresh as their own write. By a
          // mutation's own applied projection (no newer run): that
          // application is already fresher than this fetch — settle now.
          return latestRefresh === run ? undefined : latestRefresh;
        }
        if (snap) {
          var changed = snap.revision !== revision;
          applyProjection(
            { html: snap.html, revision: snap.revision, base_digest: snap.base_digest, base_spec_b64: snap.base_spec_b64, expected: snap.expected, dirty: snap.git ? snap.git.dirty : undefined },
            changed ? "Board updated" : null
          );
        }
        return undefined;
      })
      .catch(function () {
        // A failed poll leaves the page as it stands; the next tick or a
        // manual Refresh tries again. Never a favorable substitution.
      });
    latestRefresh = run;
    return run;
  }

  // 2s visible-only polling; pause on hidden; one immediate conditional
  // refresh on visible (SI-165's fixed browser behavior). The interval
  // never STACKS fetches: a tick is skipped while the newest refresh is
  // still in flight (mutation-driven refreshes still start immediately).
  var pollBusy = false;
  setInterval(function () {
    if (document.hidden || pollBusy) return;
    pollBusy = true;
    refresh().then(function () { pollBusy = false; }, function () { pollBusy = false; });
  }, 2000);
  document.addEventListener("visibilitychange", function () {
    if (!document.hidden) refresh();
  });
  document.addEventListener("click", function (e) {
    if (e.target.closest && e.target.closest("#asd-refresh")) refresh();
  });

  // -- typed mutation transport ---------------------------------------------
  function mutate(ops, extras) {
    setStatus("saving…");
    var request = {
      schema: "verdi.draftmutation/v1",
      spec: "spec/" + specName,
      base_digest: baseDigest,
      base_spec_b64: baseSpecB64,
      expected: expected,
      operations: ops,
    };
    var envelope = { request: request };
    if (extras && extras.graduate && extras.graduate.length) envelope.graduate_annotations = extras.graduate;
    if (extras && extras.del && extras.del.length) envelope.delete_annotations = extras.del;
    return fetch(url("/api/mutate_draft"), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(envelope),
    })
      .then(function (resp) {
        return resp.json().catch(function () { return {}; }).then(function (data) {
          return { status: resp.status, data: data };
        });
      })
      .then(function (r) {
        var d = r.data;
        if (d.projection) {
          refreshSeq++; // this response supersedes every in-flight refresh
          mutationSeq++;
          applyProjection(d.projection);
        }
        if (d.result) {
          var changed = (d.result.changes || []).map(function (c) { return c.target + " " + c.change; }).join(", ");
          if (d.projection_failure) {
            // The transaction LANDED but the fresh projection could not
            // be rendered (the server's typed operational disclosure —
            // §4.3: no partial action effect may be hidden). Never
            // present the landed write as a failure, never claim a
            // refreshed wall: the durable facts and the refresh failure
            // are both named, and the ordinary conditional refresh (or a
            // reload) reconciles the view.
            var followUp = d.post_transaction_error ? " A follow-up also failed and is disclosed: " + d.post_transaction_error + "." : "";
            showResult("applied-unrefreshed", "Saved — the edit landed" + (changed ? " (changed: " + changed + ")" : "") +
              ", but the fresh view could not be rendered: " + d.projection_failure.detail + "." + followUp +
              " The wall may be stale; use Refresh or reload the page.");
            announce("Saved; the view could not be refreshed");
            setStatus("saved — view refresh failed");
            return d;
          }
          if (d.post_transaction_error) {
            showResult("partial", "Saved, but a follow-up failed and is disclosed: " + d.post_transaction_error);
            announce("Saved with a disclosed follow-up failure");
          } else {
            showResult("clean", "Saved. " + (changed ? "Changed: " + changed + "." : ""));
            announce("Saved");
          }
          setStatus("saved");
          return d;
        }
        if (d.stale) {
          var changedT = (d.stale.changed_targets || []).join(", ");
          // Claim a refreshed board only when a fresh projection actually
          // rode the refusal; a projection failure beside it is disclosed
          // instead (never a favorable substitution).
          var reconcile = d.projection
            ? "The board has been refreshed; redo the edit on the current text."
            : "The board could NOT be refreshed (" + (d.projection_failure ? d.projection_failure.detail : "projection unavailable") + "); use Refresh, then redo the edit.";
          showResult("stale", "Not applied: the draft changed underneath this edit (stale base). " +
            (changedT ? "Changed since your view: " + changedT + ". " : "") +
            reconcile + " Your input was: " +
            JSON.stringify(ops));
          announce(d.projection ? "Edit refused: stale base — board refreshed" : "Edit refused: stale base — refresh manually");
          setStatus("stale — redo on the refreshed board");
          return d;
        }
        if (d.failure) {
          var f = d.failure;
          showResult(f.classification === "verdict" ? "verdict" : "operational",
            (f.classification === "verdict" ? "Refused: " : "Failed: ") + f.code + " — " + f.detail);
          announce(f.classification === "verdict" ? "Edit refused: " + f.code : "Operational failure: " + f.code);
          setStatus(f.classification === "verdict" ? "refused: " + f.code : "error: " + f.code);
          return d;
        }
        if (d.error) {
          showResult("operational", "Request refused before the application ran: " + d.error);
          setStatus("error: " + d.error);
          return d;
        }
        setStatus("error: unexpected response");
        return d;
      })
      .catch(function (err) {
        showResult("operational", "Transport failure: " + err.message + ". Nothing favorable is assumed; refresh and retry.");
        setStatus("error: " + err.message);
        throw err;
      });
  }

  // -- on-demand application panels ----------------------------------------
  function renderPanelJSON(bodyEl, jsonText) {
    bodyEl.textContent = "";
    var pre = document.createElement("pre");
    pre.className = "asd-panel-json";
    try {
      pre.textContent = JSON.stringify(JSON.parse(jsonText), null, 2);
    } catch (e) {
      pre.textContent = jsonText;
    }
    bodyEl.appendChild(pre);
  }
  document.addEventListener("toggle", function (e) {
    var panel = e.target;
    if (!panel.getAttribute || !panel.getAttribute("data-asd-panel")) return;
    if (!panel.open || panel.getAttribute("data-asd-loaded") === "1") return;
    var op = panel.getAttribute("data-asd-panel");
    var bodyEl = panel.querySelector("[data-asd-panel-body]");
    if (!bodyEl) return;
    bodyEl.textContent = "deriving…";
    fetch(url("/api/" + op), { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" })
      .then(function (resp) { return resp.text().then(function (t) { return { status: resp.status, text: t }; }); })
      .then(function (r) {
        panel.setAttribute("data-asd-loaded", "1");
        renderPanelJSON(bodyEl, r.text);
      })
      .catch(function (err) {
        bodyEl.textContent = "Could not derive: " + err.message;
      });
  }, true);

  // -- typed-operation forms ------------------------------------------------
  var opDialog = document.getElementById("asd-op-dialog");
  var opState = null;
  function show(el) {
    var backdrop = document.getElementById("modal-backdrop");
    if (backdrop) backdrop.hidden = false;
    el.hidden = false;
  }
  function hideDialogs() {
    var backdrop = document.getElementById("modal-backdrop");
    if (backdrop) backdrop.hidden = true;
    if (opDialog) opDialog.hidden = true;
    var stub = document.getElementById("asd-stub-dialog");
    if (stub) stub.hidden = true;
    // The dialog's end is an interaction end (Codex closure round 1): a
    // projection held while it was open resumes now — as a fresh fetch,
    // never a re-apply of held bytes (resumeHeldRefresh's own contract).
    // On the Apply path the mutation POST still carries the held base by
    // construction (mutate() reads it synchronously after this call),
    // and the mutation's own applied projection supersedes the resumed
    // fetch through the refreshSeq contract.
    var a = api2();
    if (a) a.resumeHeldRefresh();
  }
  function slugPattern() {
    return asdState.slugPattern || "^[a-z0-9]+(?:-[a-z0-9]+)*$";
  }
  function validSlug(slug) {
    try {
      return new RegExp(slugPattern()).test(slug);
    } catch (e) {
      return /^[a-z0-9]+(?:-[a-z0-9]+)*$/.test(slug);
    }
  }

  function openOpDialog(kind) {
    if (!opDialog) return;
    opState = { kind: kind };
    var title = document.getElementById("asd-op-title");
    var note = document.getElementById("asd-op-note");
    var kindField = document.getElementById("asd-op-kind-field");
    var text = document.getElementById("asd-op-text");
    var err = document.getElementById("asd-op-error");
    err.hidden = true;
    text.value = "";
    if (kind === "set-problem" || kind === "set-outcome") {
      title.textContent = kind === "set-problem" ? "Set problem" : "Set outcome";
      note.textContent = "One typed operation (" + kind + ") replaces the statement in the spec document.";
      kindField.hidden = true;
      var placard = document.querySelector(kind === "set-problem" ? '[data-testid="placard-problem"] .placard-text' : '[data-testid="placard-outcome"] .placard-text');
      if (placard) text.value = placard.textContent;
    } else {
      title.textContent = "Add object";
      note.textContent = "One typed operation declares a new object in the spec document, with its own anchor heading.";
      kindField.hidden = false;
      updateIDPreview();
    }
    show(opDialog);
    text.focus();
  }
  function nextIDFor(op) {
    var prefixes = { "add-ac": "ac", "add-constraint": "co", "add-decision": "dc", "add-question": "oq" };
    var c = document.getElementById("board-canvas");
    return c ? c.getAttribute("data-next-id-" + prefixes[op]) : "";
  }
  function updateIDPreview() {
    var sel = document.getElementById("asd-op-kind");
    var preview = document.getElementById("asd-op-id-preview");
    if (sel && preview) preview.textContent = "will be declared as " + nextIDFor(sel.value);
  }
  document.addEventListener("change", function (e) {
    if (e.target && e.target.id === "asd-op-kind") updateIDPreview();
  });

  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!t.closest) return;
    var opBtn = t.closest("[data-asd-op]");
    if (opBtn) {
      openOpDialog(opBtn.getAttribute("data-asd-op"));
      return;
    }
    if (t.closest("#asd-op-cancel")) {
      hideDialogs();
      return;
    }
    if (t.closest("#asd-op-ok")) {
      var text = document.getElementById("asd-op-text").value.trim();
      var err = document.getElementById("asd-op-error");
      if (!text) {
        err.textContent = "Text is required — an empty statement is not an operation.";
        err.hidden = false;
        return;
      }
      var ops;
      if (opState.kind === "set-problem" || opState.kind === "set-outcome") {
        var anchorAttr = document.querySelector('[data-asd-op="' + opState.kind + '"]');
        var anchor = anchorAttr && anchorAttr.getAttribute("data-anchor") ? anchorAttr.getAttribute("data-anchor") : (opState.kind === "set-problem" ? "#problem" : "#outcome");
        ops = [{ op: opState.kind, text: text, anchor: anchor }];
      } else {
        var sel = document.getElementById("asd-op-kind").value;
        var id = nextIDFor(sel);
        var op = { op: sel, id: id, text: text, anchor: "#" + id };
        if (sel === "add-ac") op.evidence = ["attestation"];
        ops = [op];
      }
      hideDialogs();
      mutate(ops, {});
      return;
    }
    // In-place stub correction (F-06): the same typed transaction — an
    // edit-stub for binding changes, an atomic [remove-stub, add-stub]
    // for a slug rename.
    var correct = t.closest("[data-asd-correct-stub]");
    if (correct) {
      openStubDialog(correct.closest(".stubcard"));
      return;
    }
    if (t.closest("#asd-stub-cancel")) {
      hideDialogs();
      return;
    }
    if (t.closest("#asd-stub-ok")) {
      applyStubDialog();
      return;
    }
  });

  var stubState = null;
  // rebuildStubChoices re-derives the dialog's AC/question checkboxes
  // from the CURRENT server-rendered region (Codex correction round 1,
  // finding 3): the dialog lives OUTSIDE the snapshot-replaced region, so
  // its server-rendered choices are the page-load inventory only — a
  // criterion added since load must be offered, a removed object must
  // not. One source of truth: the region's own object cards, in the
  // projection's order. Runs ONLY at open — an open, actively edited
  // dialog is never rebuilt (the applyProjection interaction-guard
  // precedent: in-flight input belongs to the author).
  function rebuildStubChoices(dialog) {
    function fill(fieldsetID, kind, attr) {
      var fs = dialog.querySelector("#" + fieldsetID);
      if (!fs) return;
      var old = fs.querySelectorAll("label");
      for (var i = 0; i < old.length; i++) fs.removeChild(old[i]);
      var cards = document.querySelectorAll('#boardv2-region .objcard[data-object-kind="' + kind + '"]');
      for (var j = 0; j < cards.length; j++) {
        var id = cards[j].getAttribute("data-id");
        if (!id) continue;
        var label = document.createElement("label");
        var box = document.createElement("input");
        box.type = "checkbox";
        box.setAttribute(attr, id);
        label.appendChild(box);
        label.appendChild(document.createTextNode(" " + id));
        fs.appendChild(label);
      }
    }
    fill("asd-stub-acs", "acceptance-criterion", "data-asd-stub-ac");
    fill("asd-stub-oqs", "open-question", "data-asd-stub-oq");
  }
  function openStubDialog(stubCard) {
    var dialog = document.getElementById("asd-stub-dialog");
    if (!dialog || !stubCard) return;
    rebuildStubChoices(dialog);
    var slug = stubCard.getAttribute("data-stub");
    stubState = {
      slug: slug,
      spike: stubCard.getAttribute("data-spike") === "true",
      acs: (stubCard.getAttribute("data-acs") || "").split(",").filter(Boolean),
      oqs: (stubCard.getAttribute("data-resolves") || "").split(",").filter(Boolean),
    };
    document.getElementById("asd-stub-slug").value = slug;
    document.getElementById("asd-stub-spike").checked = stubState.spike;
    var boxes = dialog.querySelectorAll("[data-asd-stub-ac]");
    for (var i = 0; i < boxes.length; i++) {
      boxes[i].checked = stubState.acs.indexOf(boxes[i].getAttribute("data-asd-stub-ac")) >= 0;
    }
    var oqBoxes = dialog.querySelectorAll("[data-asd-stub-oq]");
    for (var j = 0; j < oqBoxes.length; j++) {
      oqBoxes[j].checked = stubState.oqs.indexOf(oqBoxes[j].getAttribute("data-asd-stub-oq")) >= 0;
    }
    document.getElementById("asd-stub-slug-error").hidden = true;
    show(dialog);
    document.getElementById("asd-stub-slug").focus();
  }
  // Field-level slug grammar validation BEFORE authoring (F-06): the
  // rejected bytes and the corrective grammar at the field itself.
  document.addEventListener("input", function (e) {
    if (!e.target || e.target.id !== "asd-stub-slug") return;
    var err = document.getElementById("asd-stub-slug-error");
    var v = e.target.value;
    if (v && !validSlug(v)) {
      err.textContent = JSON.stringify(v) + " is not kebab-case; the spec name grammar is " + slugPattern() + ".";
      err.hidden = false;
    } else {
      err.hidden = true;
    }
  });
  function applyStubDialog() {
    if (!stubState) return;
    var dialog = document.getElementById("asd-stub-dialog");
    var slug = document.getElementById("asd-stub-slug").value.trim();
    var err = document.getElementById("asd-stub-slug-error");
    if (!validSlug(slug)) {
      err.textContent = JSON.stringify(slug) + " is not kebab-case; the spec name grammar is " + slugPattern() + ".";
      err.hidden = false;
      return;
    }
    var spike = document.getElementById("asd-stub-spike").checked;
    var acs = [];
    var boxes = dialog.querySelectorAll("[data-asd-stub-ac]");
    for (var i = 0; i < boxes.length; i++) {
      if (boxes[i].checked) acs.push(boxes[i].getAttribute("data-asd-stub-ac"));
    }
    var oqs = [];
    var oqBoxes = dialog.querySelectorAll("[data-asd-stub-oq]");
    for (var j = 0; j < oqBoxes.length; j++) {
      if (oqBoxes[j].checked) oqs.push(oqBoxes[j].getAttribute("data-asd-stub-oq"));
    }
    var ops;
    if (slug !== stubState.slug) {
      // A rename replaces the stub atomically in ONE ordered batch.
      var added = { op: "add-stub", slug: slug };
      if (spike) {
        added.spike = true;
        if (oqs.length) added.resolves = oqs;
      } else if (acs.length) {
        added.acceptance_criteria = acs;
      }
      ops = [{ op: "remove-stub", slug: stubState.slug }, added];
    } else {
      var edited = { op: "edit-stub", slug: slug };
      if (spike) {
        edited.spike = true;
        if (oqs.length) edited.resolves = oqs;
      } else if (acs.length) {
        edited.acceptance_criteria = acs;
      }
      ops = [edited];
    }
    hideDialogs();
    stubState = null;
    mutate(ops, {});
  }

  // The published transport surface boardspec.js consumes.
  window.__verdiASD = {
    mutate: mutate,
    refresh: refresh,
    slugPattern: slugPattern,
    state: function () {
      return { revision: revision, baseDigest: baseDigest, expected: expected };
    },
  };
})();
