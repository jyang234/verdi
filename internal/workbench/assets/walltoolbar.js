// The wall's contextual toolbar, drag-to-thread, and add-in-place slots
// (spec/wall-canvas-v2 ac-3, ac-4, ac-5, co-2; ledger SI-350 (5), (6), (7),
// (8), (9), (13), (15), (16), SI-352 (2); lane F2b). Built on the
// selection seam (wallselect.js: window.__WALLSELECT__ and the
// `wall-selection` event) and the board's own entries (boardspec.js:
// window.__BOARDV2API__; boardspecasd.js: window.__verdiASD), it offers
// exactly the actions legal for the selection and the mode, and it reaches
// every action through the path that existed before: the inline sticky
// draft, the add-object dialog, the pin tray, the card editor, the
// Correct stub dialog, the graduate menu and the stub graduation's
// confirmation, the type picker, and the trash drop's own removal routing.
//
// A new asset for the new behaviour (co-1: boardspec.js does not grow).
// It owns no data: what it offers is read from the server's markup — a
// chip's or sticky's data-can-* attributes name the actions the kernel
// accepts for it, a slot's data-slot-* its typed operation, the canvas's
// data-next-id-* the server's next ids — and the selection is the seam's.
// The toolbar is drawn into the host the server renders in the wall
// frame's row (boardspecrender.go), refilled after every region swap and
// every selection change; with nothing to say it leaves the host empty.
//
// Review and read-only modes offer the yarn key and the read actions only
// (ac-3): the Document link and the thread count. Under an authoring
// wall's domain refusal the typed writes are not offered either — the
// server renders no pin, no data-can-graduate and no slot there — and the
// scratch tier's actions remain.
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  var region = document.getElementById("boardv2-region");
  if (!state || !region) return;

  var authoring = state.mode === "authoring";
  var domainLive = authoring && !state.domainRefusal;

  function canvas() {
    return document.getElementById("board-canvas");
  }
  function api() {
    return window.__BOARDV2API__;
  }
  function seam() {
    return window.__WALLSELECT__;
  }
  function host() {
    return region.querySelector(".wall-toolbar");
  }
  function pinTray() {
    return document.getElementById("pin-tray");
  }

  // -- reading the wall -------------------------------------------------------

  function kindWords(kind) {
    return (kind || "").replace(/-/g, " ");
  }

  // keyOfCard mirrors wallselect.js's card key: the grammar the chips'
  // endpoints use.
  function keyOfCard(el) {
    if (el.classList.contains("stubcard")) return "stub:" + el.getAttribute("data-stub");
    if (el.classList.contains("refcard")) return el.getAttribute("data-ref");
    return el.getAttribute("data-id");
  }

  // labelOf names an endpoint the way the status pill does.
  function labelOf(key) {
    if (key === "spec") return "this spec";
    if (key.indexOf("stub:") === 0) return key.slice(5);
    var c = canvas();
    var sticky = c && c.querySelector('.sticky[data-id="' + cssEsc(key) + '"]');
    return sticky ? "sticky" : key;
  }

  function cssEsc(s) {
    return window.CSS && CSS.escape ? CSS.escape(s) : s.replace(/["\\]/g, "\\$&");
  }

  function threadsTouching(key) {
    var c = canvas();
    if (!c) return 0;
    var chips = c.querySelectorAll(".yarn-chip");
    var n = 0;
    for (var i = 0; i < chips.length; i++) {
      if (chips[i].getAttribute("data-from") === key || chips[i].getAttribute("data-to") === key) n++;
    }
    return n;
  }

  // specEdgesHolding mirrors boardspec.js's trash routing for a reference
  // card: the spec-layer chips touching the ref, and whether any is the
  // document's own (which the board cannot edit).
  function refDeletable(card) {
    var key = card.getAttribute("data-ref");
    var c = canvas();
    var chips = c ? c.querySelectorAll('.yarn-chip[data-layer="spec"]') : [];
    var held = 0;
    var docHeld = false;
    for (var i = 0; i < chips.length; i++) {
      var from = chips[i].getAttribute("data-from");
      if (from !== key && chips[i].getAttribute("data-to") !== key) continue;
      held++;
      if (from === "spec") docHeld = true;
    }
    if (held === 0) {
      // A pure pin, or a card held only by scratch threads: the scratch
      // tier's "or they die".
      return card.hasAttribute("data-pin-id") || threadsTouching(key) > 0;
    }
    return !docHeld && domainLive;
  }

  function documentHref() {
    var a = document.querySelector('[data-testid="board-tab-document"]');
    return a ? a.getAttribute("href") : "";
  }

  // -- building the toolbar -------------------------------------------------------

  function node(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text) n.textContent = text;
    return n;
  }
  function button(action, label, kbd) {
    var b = node("button", "wall-toolbar-btn", label);
    b.type = "button";
    b.setAttribute("data-wall-action", action);
    if (kbd) {
      var k = node("kbd", "", kbd);
      k.setAttribute("aria-hidden", "true");
      b.appendChild(document.createTextNode(" "));
      b.appendChild(k);
    }
    return b;
  }
  function divider() {
    var d = node("span", "wall-toolbar-divider");
    d.setAttribute("aria-hidden", "true");
    return d;
  }
  function pushpin() {
    var p = node("span", "wall-toolbar-pushpin");
    p.setAttribute("aria-hidden", "true");
    return p;
  }
  function lockup(kind, kindLabel, id) {
    var l = node("span", "wall-toolbar-lockup");
    l.setAttribute("data-object-kind", kind);
    l.appendChild(node("span", "wall-toolbar-kind", kindLabel));
    if (id) l.appendChild(node("span", "wall-toolbar-id", id));
    return l;
  }

  // yarnKeyAction opens the existing yarn key (SI-350 (9)): the rail's
  // section, until wall-strip-and-drawer-v2 moves it into the drawer.
  function yarnKeyAction(h) {
    if (!document.querySelector(".yarn-key")) return;
    if (h.childElementCount) h.appendChild(divider());
    var b = button("yarn-key", "Yarn key");
    var key = node("span", "wall-toolbar-key");
    key.setAttribute("aria-hidden", "true");
    for (var i = 0; i < 5; i++) key.appendChild(node("i"));
    b.insertBefore(key, b.firstChild);
    h.appendChild(b);
  }

  function renderIdle(h) {
    if (authoring) {
      // The scratch tier is live on every authoring wall, the domain's
      // dialogs and the pin tray only where the server rendered them.
      h.appendChild(button("sticky", "Sticky"));
      if (document.getElementById("asd-add-object")) {
        var card = button("card", "Card", "\u25be");
        card.setAttribute("aria-haspopup", "dialog");
        h.appendChild(card);
      }
      var tray = pinTray();
      if (tray) {
        var pin = button("pin", "Pin an artifact");
        pin.insertBefore(pushpin(), pin.firstChild);
        pin.setAttribute("aria-controls", "pin-tray");
        pin.setAttribute("aria-expanded", tray.hidden ? "false" : "true");
        h.appendChild(pin);
      }
    }
    yarnKeyAction(h);
    if (authoring) h.appendChild(node("span", "wall-toolbar-hint", "click a card to see what you can do with it"));
  }

  function renderCard(h, card) {
    var key = keyOfCard(card);
    var objcard = card.classList.contains("objcard");
    var stub = card.classList.contains("stubcard");
    var sticky = card.classList.contains("sticky");
    var ref = card.classList.contains("refcard");

    if (objcard) {
      h.appendChild(lockup(card.getAttribute("data-object-kind"), kindWords(card.getAttribute("data-object-kind")), key));
    } else if (stub) {
      var kindEl = card.querySelector(".card-kind-label");
      h.appendChild(lockup("stub", kindEl ? kindEl.textContent : "stub", card.getAttribute("data-stub")));
    } else if (ref) {
      h.appendChild(lockup("reference", card.hasAttribute("data-pin-id") ? "pinned reference" : "reference", key));
    } else if (sticky) {
      var typeEl = card.querySelector(".sticky-type");
      h.appendChild(lockup("sticky", (typeEl ? typeEl.textContent.trim() + " " : "") + "sticky", ""));
    }

    // Edit: an object card in place (the card editor), a stub through its
    // Correct stub dialog; a sticky has no update path (SI-352 (2)) and a
    // reference card nothing to edit.
    if (objcard && domainLive) {
      h.appendChild(button("edit", "Edit", "\u23ce"));
    } else if (stub && card.querySelector("[data-asd-correct-stub]")) {
      var correct = button("edit", "Edit");
      correct.setAttribute("aria-haspopup", "dialog");
      correct.title = "Correct stub";
      h.appendChild(correct);
    }
    // The thread hint, where the card carries a pin to drag.
    if (card.querySelector(".yarn-handle")) {
      var hint = node("span", "wall-toolbar-hint", "Thread \u2014 drag the pin");
      hint.insertBefore(pushpin(), hint.firstChild);
      h.appendChild(hint);
    }
    // Graduate, for a sticky the kernel lets graduate.
    if (sticky && card.hasAttribute("data-can-graduate")) {
      var grad = button("graduate", "Graduate", "\u25be");
      grad.setAttribute("aria-haspopup", card.getAttribute("data-can-graduate") === "stub" ? "dialog" : "menu");
      h.appendChild(grad);
    }
    // Read in document: an object card at its anchor; a stub at the Plan
    // section, where stubs are listed (SI-350 (7)).
    var doc = documentHref();
    if (doc && (objcard || stub)) {
      var read = node("a", "wall-toolbar-link", "Read in document \u2197");
      read.setAttribute("data-wall-action", "read");
      read.href = doc + (objcard ? "#" + key : "#plan");
      h.appendChild(read);
    }
    // The thread count.
    var n = threadsTouching(key);
    h.appendChild(node("span", "wall-toolbar-count", n === 0 ? "no threads yet" : n === 1 ? "1 thread" : n + " threads"));
    // Delete, through the existing confirmation: an object card where the
    // domain is live, a sticky where the server says so, a reference card
    // where the trash would take it; never a stub (its refusal is the
    // trash's and Delete's, F2c).
    var deletable = (objcard && domainLive) || (sticky && card.hasAttribute("data-can-delete")) || (ref && authoring && refDeletable(card));
    if (deletable) {
      h.appendChild(divider());
      var del = button("delete", "Delete");
      del.classList.add("wall-toolbar-btn--delete");
      h.appendChild(del);
    }
    yarnKeyAction(h);
  }

  function renderThread(h, chip) {
    var type = chip.getAttribute("data-edge-type");
    var l = lockup("", type, "");
    l.setAttribute("data-edge-type", type);
    var sw = node("span", "wall-toolbar-swatch");
    sw.setAttribute("data-edge-type", type);
    sw.setAttribute("aria-hidden", "true");
    l.insertBefore(sw, l.firstChild);
    h.appendChild(l);
    h.appendChild(node("span", "wall-toolbar-pair", labelOf(chip.getAttribute("data-from")) + " \u2192 " + labelOf(chip.getAttribute("data-to"))));
    if (chip.hasAttribute("data-can-retype")) {
      var re = button("retype", "Retype", "\u25be");
      re.setAttribute("aria-haspopup", "dialog");
      h.appendChild(re);
    }
    if (chip.hasAttribute("data-can-graduate")) {
      var grad = button("graduate", "Graduate to a typed edge");
      grad.setAttribute("aria-haspopup", "dialog");
      h.appendChild(grad);
    }
    var can = chip.getAttribute("data-can-delete");
    if (can) {
      h.appendChild(divider());
      var del = button("delete", can === "edge" ? "Remove " + type + " edge" : "Delete thread");
      del.classList.add("wall-toolbar-btn--delete");
      h.appendChild(del);
    }
    yarnKeyAction(h);
  }

  function render() {
    var h = host();
    if (!h) return;
    var row = h.parentNode;
    var before = row ? row.offsetHeight : 0;
    while (h.firstChild) h.removeChild(h.firstChild);
    var s = seam() ? seam().selection() : null;
    var target = s && seam().elementOf(s);
    if (!target) {
      h.removeAttribute("data-wall-context");
      renderIdle(h);
    } else if (s.kind === "thread") {
      h.setAttribute("data-wall-context", "thread");
      renderThread(h, target);
    } else {
      h.setAttribute("data-wall-context", "card");
      renderCard(h, target);
    }
    // The row's height feeds the canvas's bound (wallselect.js measure):
    // a toolbar that wrapped differently re-measures it.
    if (row && row.offsetHeight !== before) window.dispatchEvent(new Event("resize"));
  }

  // -- acting ---------------------------------------------------------------------

  var anchorPoint = null; // where the picker opens next: a drop point or a chip
  var pickerPair = null; // { from, to } the open picker decides about
  var pendingSelect = null; // { from, to, type, until }: the thread to select once written

  function selected() {
    var s = seam() ? seam().selection() : null;
    return s ? seam().elementOf(s) : null;
  }

  function editElement(card) {
    if (!card || !api()) return;
    if (card.classList.contains("objcard")) {
      api().editCard(card);
      return;
    }
    var correct = card.querySelector("[data-asd-correct-stub]");
    if (correct) correct.click();
  }

  function revealYarnKey() {
    var key = document.querySelector(".yarn-key");
    if (!key) return;
    if (!key.hasAttribute("tabindex")) key.setAttribute("tabindex", "-1");
    key.setAttribute("data-revealed", "true");
    key.scrollIntoView({ block: "nearest" });
    key.focus({ preventScroll: true });
    key.addEventListener(
      "blur",
      function () {
        key.removeAttribute("data-revealed");
      },
      { once: true }
    );
  }

  function pickerAtChip(chip) {
    var r = chip.getBoundingClientRect();
    anchorPoint = { x: r.left, y: r.bottom + 4 };
    pickerPair = { from: chip.getAttribute("data-from"), to: chip.getAttribute("data-to") };
  }

  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!(t instanceof Element)) return;
    var actor = t.closest("[data-wall-action]");
    var h = host();
    if (!actor || !h || !h.contains(actor)) return;
    var action = actor.getAttribute("data-wall-action");
    var target = selected();
    switch (action) {
      case "sticky":
        if (api()) api().addSticky();
        return;
      case "card": {
        var add = document.getElementById("asd-add-object");
        if (add) add.click();
        return;
      }
      case "pin":
        return; // boardspec.js toggles the tray from the button's aria-controls
      case "yarn-key":
        revealYarnKey();
        return;
      case "edit":
        editElement(target);
        return;
      case "graduate":
        if (target && api()) {
          if (target.classList.contains("yarn-chip")) pickerAtChip(target);
          api().graduate(target, actor);
        }
        return;
      case "retype":
        if (target && api()) {
          pickerAtChip(target);
          api().retype(target);
        }
        return;
      case "delete":
        if (target && api()) api().remove(target);
        return;
    }
  });

  // -- drag-to-thread (ac-4; SI-350 (8)) ---------------------------------------------
  //
  // boardspec.js draws the draft line and resolves the drop; this file
  // lights the paper under the pointer while the draft is drawn, records
  // the drop point and the pair, places the picker at the drop point, and
  // selects the thread the choice writes once the projection carries it.

  var dropTarget = null;

  function rectOf(el) {
    return { x: el.offsetLeft, y: el.offsetTop, w: el.offsetWidth, h: el.offsetHeight };
  }
  function contains(r, x, y) {
    return x >= r.x && x <= r.x + r.w && y >= r.y && y <= r.y + r.h;
  }
  // sourceOf finds the paper the draft line starts on: its anchor is a
  // point on the source's edge.
  function sourceOf(c, x, y) {
    var papers = c.querySelectorAll(".objcard, .sticky");
    for (var i = 0; i < papers.length; i++) {
      var r = rectOf(papers[i]);
      if (x >= r.x - 1 && x <= r.x + r.w + 1 && y >= r.y - 1 && y <= r.y + r.h + 1) return papers[i];
    }
    return null;
  }
  // paperAt resolves the drop target as boardspec.js does: geometrically,
  // among the object and reference cards, the source excluded.
  function paperAt(c, x, y, source) {
    var papers = c.querySelectorAll(".objcard, .refcard");
    for (var i = 0; i < papers.length; i++) {
      if (papers[i] === source) continue;
      if (contains(rectOf(papers[i]), x, y)) return papers[i];
    }
    return null;
  }
  function clearDrop() {
    if (dropTarget) dropTarget.removeAttribute("data-drop-target");
    dropTarget = null;
  }
  function canvasPoint(c, e) {
    var cr = c.getBoundingClientRect();
    return { x: e.clientX - cr.left + c.scrollLeft, y: e.clientY - cr.top + c.scrollTop };
  }

  document.addEventListener(
    "pointermove",
    function (e) {
      var c = canvas();
      var draft = c && c.querySelector("svg.yarn-svg .yarn-draft");
      if (!draft) {
        if (dropTarget) clearDrop();
        return;
      }
      var p = canvasPoint(c, e);
      var source = sourceOf(c, parseFloat(draft.getAttribute("x1")), parseFloat(draft.getAttribute("y1")));
      var over = paperAt(c, p.x, p.y, source);
      if (over !== dropTarget) {
        clearDrop();
        if (over) {
          over.setAttribute("data-drop-target", "true");
          dropTarget = over;
        }
      }
    },
    true
  );

  // The release, before boardspec.js resolves it: the picker will open at
  // this point, over this pair.
  document.addEventListener(
    "pointerup",
    function (e) {
      var c = canvas();
      var draft = c && c.querySelector("svg.yarn-svg .yarn-draft");
      if (!draft) {
        clearDrop();
        return;
      }
      var p = canvasPoint(c, e);
      var source = sourceOf(c, parseFloat(draft.getAttribute("x1")), parseFloat(draft.getAttribute("y1")));
      var over = paperAt(c, p.x, p.y, source);
      clearDrop();
      if (!over || !source) return;
      anchorPoint = { x: e.clientX, y: e.clientY };
      pickerPair = { from: source.getAttribute("data-id"), to: keyOfCard(over) };
    },
    true
  );
  document.addEventListener("pointercancel", clearDrop, true);

  // The picker opens where the drop (or the chip) was, clamped inside
  // the viewport; hidden again, it returns to the shared dialog's place.
  var picker = document.getElementById("edge-picker");
  function placePicker() {
    if (!picker) return;
    if (picker.hidden) {
      picker.removeAttribute("data-at-point");
      picker.style.left = "";
      picker.style.top = "";
      return;
    }
    if (!anchorPoint) return;
    picker.setAttribute("data-at-point", "true");
    var w = picker.offsetWidth;
    var h = picker.offsetHeight;
    picker.style.left = Math.max(8, Math.min(anchorPoint.x, window.innerWidth - w - 8)) + "px";
    picker.style.top = Math.max(8, Math.min(anchorPoint.y, window.innerHeight - h - 8)) + "px";
    anchorPoint = null;
  }
  if (picker) {
    new MutationObserver(placePicker).observe(picker, { attributes: true, attributeFilter: ["hidden"] });
  }

  // A choice in the picker names the thread to select once it is written;
  // a cancel, the backdrop or Escape withdraws it.
  document.addEventListener(
    "click",
    function (e) {
      var t = e.target;
      if (!(t instanceof Element)) return;
      var choice = t.closest("[data-edge-choice]");
      if (choice && pickerPair) {
        pendingSelect = { from: pickerPair.from, to: pickerPair.to, type: choice.getAttribute("data-edge-choice"), until: Date.now() + 15000 };
        return;
      }
      if (t.closest("#edge-picker-cancel, #edge-confirm-cancel, #modal-backdrop")) {
        pendingSelect = null;
        pickerPair = null;
      }
    },
    true
  );
  document.addEventListener(
    "keydown",
    function (e) {
      if (e.key !== "Escape") return;
      var confirm = document.getElementById("edge-confirm");
      if ((picker && !picker.hidden) || (confirm && !confirm.hidden)) {
        pendingSelect = null; // Escape closes the picker or the confirmation (boardspec.js)
        pickerPair = null;
      }
    },
    true
  );

  function selectPending() {
    if (!pendingSelect) return;
    if (Date.now() > pendingSelect.until) {
      pendingSelect = null;
      return;
    }
    var c = canvas();
    if (!c || !seam()) return;
    var chip = c.querySelector(
      '.yarn-chip[data-from="' + cssEsc(pendingSelect.from) + '"][data-to="' + cssEsc(pendingSelect.to) + '"][data-edge-type="' + cssEsc(pendingSelect.type) + '"]'
    );
    if (!chip) return;
    pendingSelect = null;
    pickerPair = null;
    seam().selectElement(chip);
  }

  // -- the add-in-place slots (ac-5; SI-350 (15)) -----------------------------------
  //
  // A slot opens into its form; Enter declares the object as one typed
  // operation with the server's next id (declareOp reads the canvas's
  // data-next-id-* at that moment), Escape cancels with nothing written.
  // An open slot holds the projection (co-2, boardspec.js interactionLive
  // reads the data-open mark); its close resumes a held refresh.

  function openSlot(slot) {
    var c = canvas();
    if (!slot || !c || slot.hasAttribute("data-open")) return;
    var op = slot.getAttribute("data-slot-op");
    var prefix = slot.getAttribute("data-slot-prefix");
    var words = kindWords(slot.getAttribute("data-slot-kind"));
    var id = c.getAttribute("data-next-id-" + prefix) || "";
    slot.setAttribute("data-open", "true");
    var form = node("div", "wall-slot-form");
    var line = node("p", "wall-slot-line", words + " \u00b7 will be declared as " + id);
    line.setAttribute("data-testid", "slot-line-" + prefix);
    var text = document.createElement("textarea");
    text.className = "wall-slot-text";
    text.setAttribute("aria-label", "New " + words + " text");
    text.setAttribute("data-testid", "slot-text-" + prefix);
    var hint = node("p", "wall-slot-hint", "\u23ce declares it \u00b7 Esc cancels");
    form.appendChild(line);
    form.appendChild(text);
    form.appendChild(hint);
    slot.appendChild(form);
    text.focus();

    text.addEventListener("keydown", function (ev) {
      if (ev.key === "Escape") {
        ev.stopPropagation(); // the slot closes; open dialogs are not its business
        closeSlot(slot);
        var open = slot.querySelector(".wall-slot-open");
        if (open) open.focus();
        return;
      }
      if (ev.key === "Enter" && !ev.shiftKey && !ev.isComposing) {
        ev.preventDefault();
        var value = text.value.trim();
        var asd = window.__verdiASD;
        if (!value || !asd || !asd.declareOp) return;
        var declared = asd.declareOp(op, value);
        closeSlot(slot);
        asd.mutate([declared], {});
      }
    });
  }

  // closeSlot closes a slot's current form, whichever open made it, and
  // resumes a refresh the open slot held.
  function closeSlot(slot) {
    if (!slot.hasAttribute("data-open")) return;
    slot.removeAttribute("data-open");
    var form = slot.querySelector(".wall-slot-form");
    if (form) form.remove();
    if (api()) api().resumeHeldRefresh();
  }

  // Leaving an empty slot closes it; a slot with text stays open, so
  // nothing typed is silently lost and nothing is silently written. One
  // listener serves every slot and every open: a listener added per open
  // outlived its form, and its close took a later open's hold with it,
  // so the next swap destroyed the text (F2BR-1).
  document.addEventListener("focusout", function (e) {
    var t = e.target;
    var slot = t instanceof Element ? t.closest(".wall-slot[data-open]") : null;
    if (!slot) return;
    setTimeout(function () {
      if (!slot.isConnected || !slot.hasAttribute("data-open") || slot.contains(document.activeElement)) return;
      var text = slot.querySelector(".wall-slot-text");
      if (!text || !text.value.trim()) closeSlot(slot);
    }, 0);
  });

  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!(t instanceof Element)) return;
    var open = t.closest(".wall-slot-open");
    if (open) openSlot(open.closest(".wall-slot"));
  });

  // -- keys: the card editor's Enter and Escape, Enter on the selection, a chip's activation ----

  document.addEventListener(
    "keydown",
    function (e) {
      var t = e.target;
      if (!(t instanceof Element)) return;
      if (t.classList.contains("card-editor")) {
        // The editor applies on blur (boardspec.js): Enter leaves it, so
        // the edit applies; Escape restores the server's text first, so
        // nothing is written (ac-5).
        if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
          e.preventDefault();
          t.blur();
        } else if (e.key === "Escape") {
          e.stopPropagation();
          var card = t.closest(".objcard");
          var original = card && card.querySelector(".card-text");
          if (original) t.value = original.textContent;
          t.blur();
          if (card) card.focus();
        }
        return;
      }
      if ((e.key === "Enter" || e.key === " ") && t.classList.contains("yarn-chip")) {
        e.preventDefault();
        t.click(); // the chip is the button that selects its thread
        return;
      }
      if (e.key !== "Enter" || e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
      // Enter edits the selected card (ac-5) when the key is not another
      // control's: a focused object card is boardspec.js's own.
      if (t.closest("input, textarea, select, button, a, [contenteditable], [role=dialog], [role=alertdialog], [role=menu], .objcard, .yarn-chip")) return;
      var confirm = document.getElementById("edge-confirm");
      if ((picker && !picker.hidden) || (confirm && !confirm.hidden)) return;
      var sel = selected();
      if (!sel || !(sel.classList.contains("objcard") || sel.classList.contains("stubcard"))) return;
      e.preventDefault();
      editElement(sel);
    },
    true
  );

  // -- swaps and the selection -------------------------------------------------------

  document.addEventListener("wall-selection", render);
  new MutationObserver(function () {
    var h = host();
    if (h && !h.childElementCount) render(); // a swap replaced the host
    selectPending();
  }).observe(region, { childList: true, subtree: true });

  render();
})();
