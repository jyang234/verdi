// wallstrip.js: the wall's case-file strip, Commit and push's changes
// popover, the readiness pill and the branch menu (spec/wall-strip-and-
// drawer-v2 ac-1 to ac-4; ledger SI-368 (2), (7), (8), (13); lane F3a). A
// new asset for the new behaviour (parent co-1: boardspec.js does not
// grow), built on the board's seams — the transport's mutate
// (boardspecasd.js: window.__verdiASD) and the hold contract
// (boardspec.js: window.__BOARDV2API__) — and owning no data: every
// fragment it swaps is the server's, and every write is one typed
// operation through the shared mutation core.
//
// The strip (ac-1; SI-368 (13)): on a live authoring wall a click or
// Enter on a half's text opens the editor in place — a textarea with the
// statement, Apply, Cancel and the hint — and Enter applies ONE
// set-problem or set-outcome with the server's anchor, Escape cancels
// with nothing written. The open editor holds the projection swap
// ([data-holds-projection], which interactionLive reads), so a refresh
// never yanks the statement out from under the hand; leaving the editor
// with the statement unchanged closes it, with changes keeps it. The
// "full case file" control and the chips are left to boardspec.js (the
// expand dialog, the derivation drawer); in review and read-only a click
// still expands, since no half there names an operation.
//
// The bar (SI-368 (2), (7), (8)): each snapshot's Commit and push
// fragment, pill facts and branch list arrive through apply(), called
// by the transport after every applied projection — the fragment is
// swapped with its open state and focus kept, the pill's words follow
// the server's own rule (readinessPillWords), and the menu is refilled
// only when the list changed and the hand is not in it. The popover is a
// native <details>: it opens on focus or a click, never on hover alone;
// Escape and an outside press close it. The branch menu is placed under
// the bar's switcher when boardspec.js opens it, closed on an outside
// press, and its switcher says whether it is expanded.
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  var region = document.getElementById("boardv2-region");
  if (!state || !region) return;

  function api() {
    return window.__BOARDV2API__ || null;
  }
  function transport() {
    return window.__verdiASD || null;
  }
  function node(tag, cls, text) {
    var el = document.createElement(tag);
    if (cls) el.className = cls;
    if (text) el.textContent = text;
    return el;
  }

  // -- the strip's in-place editor (ac-1) ---------------------------------------

  var HALF = ".case-strip .placard[data-strip-op]";

  function halfOf(t) {
    return t && t.closest ? t.closest(HALF) : null;
  }
  function whichOf(half) {
    return half.getAttribute("data-strip-op") === "set-outcome" ? "outcome" : "problem";
  }
  function editingHalf() {
    return region.querySelector(".case-strip .placard[data-editing]");
  }

  function openEditor(half) {
    if (!half || half.hasAttribute("data-editing")) return;
    var open = editingHalf();
    if (open && open !== half) cancelEditor(open);
    var op = half.getAttribute("data-strip-op");
    var which = whichOf(half);
    var text = half.querySelector(".placard-text");
    var original = text ? text.textContent : "";
    half.setAttribute("data-editing", "true");
    var form = node("div", "case-strip-editor");
    form.setAttribute("data-holds-projection", "true");
    form.setAttribute("data-testid", "case-strip-editor-" + which);
    form.setAttribute("data-original", original);
    var area = document.createElement("textarea");
    area.className = "case-strip-editor-text";
    area.setAttribute("aria-label", which === "outcome" ? "Outcome statement" : "Problem statement");
    area.setAttribute("data-testid", "case-strip-text-" + which);
    area.value = original;
    var actions = node("div", "case-strip-editor-actions");
    var apply = node("button", "btn-primary case-strip-apply", "Apply");
    apply.type = "button";
    apply.setAttribute("data-testid", "case-strip-apply-" + which);
    var cancel = node("button", "case-strip-cancel", "Cancel");
    cancel.type = "button";
    cancel.setAttribute("data-testid", "case-strip-cancel-" + which);
    var hint = node("span", "case-strip-editor-hint", "one typed operation · " + op + " · ⏎ applies, Esc cancels");
    actions.appendChild(apply);
    actions.appendChild(cancel);
    actions.appendChild(hint);
    form.appendChild(area);
    form.appendChild(actions);
    half.appendChild(form);
    area.focus();
    area.setSelectionRange(area.value.length, area.value.length);
    area.addEventListener("keydown", function (e) {
      if (e.key === "Escape") {
        e.stopPropagation(); // the editor closes; no other layer reads this press
        e.preventDefault();
        cancelEditor(half);
        return;
      }
      if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
        e.stopPropagation();
        e.preventDefault();
        applyEditor(half);
      }
    });
  }

  // closeEditor removes the half's editor and resumes a refresh it held.
  function closeEditor(half) {
    var form = half.querySelector(".case-strip-editor");
    if (form) form.remove();
    half.removeAttribute("data-editing");
    if (api()) api().resumeHeldRefresh();
  }

  function cancelEditor(half) {
    closeEditor(half);
    var text = half.querySelector(".placard-text");
    if (text && text.focus) text.focus({ preventScroll: true });
  }

  // applyEditor sends the one typed operation, exactly as the typed-
  // operation dialog does (the same op, the server's anchor), then closes.
  // An empty statement is not an operation: nothing is sent and the
  // editor stays, as the dialog's own rule has it.
  function applyEditor(half) {
    var area = half.querySelector(".case-strip-editor-text");
    var value = area ? area.value.trim() : "";
    var t = transport();
    if (!value || !t || !t.mutate) {
      if (area) area.focus();
      return;
    }
    var op = half.getAttribute("data-strip-op");
    var anchor = half.getAttribute("data-strip-anchor") || "#" + whichOf(half);
    closeEditor(half);
    t.mutate([{ op: op, text: value, anchor: anchor }], {});
  }

  // A click on an editable half's face opens the editor, before any other
  // listener reads it as an expand (the capture phase); the half's own
  // "full case file" control and chips keep their own click. Inside an
  // open editor, Apply and Cancel are handled here and nothing below may
  // read the click.
  document.addEventListener(
    "click",
    function (e) {
      var t = e.target;
      if (!(t instanceof Element)) return;
      var half = halfOf(t);
      if (!half) return;
      if (t.closest(".case-strip-editor")) {
        e.stopPropagation();
        if (t.closest(".case-strip-apply")) applyEditor(half);
        else if (t.closest(".case-strip-cancel")) cancelEditor(half);
        return;
      }
      if (t.closest(".placard-more, .case-stamp, .case-chip, a, button")) return;
      e.stopPropagation();
      e.preventDefault();
      openEditor(half);
    },
    true
  );

  // Enter or Space on a focused headline opens its editor (the keyboard
  // path), before the wall's keys read the press.
  document.addEventListener(
    "keydown",
    function (e) {
      var t = e.target;
      if (!(t instanceof Element) || (e.key !== "Enter" && e.key !== " ")) return;
      if (!t.matches(HALF + " > .placard-text")) return;
      e.stopPropagation();
      e.preventDefault();
      openEditor(halfOf(t));
    },
    true
  );

  // Leaving an editor whose statement is unchanged closes it (nothing to
  // keep, nothing written); one with changes stays open, so nothing typed
  // is silently lost and nothing is silently written.
  document.addEventListener("focusout", function (e) {
    var t = e.target;
    var half = t instanceof Element ? t.closest(".case-strip .placard[data-editing]") : null;
    if (!half) return;
    setTimeout(function () {
      if (!half.isConnected || !half.hasAttribute("data-editing") || half.contains(document.activeElement)) return;
      var form = half.querySelector(".case-strip-editor");
      var area = half.querySelector(".case-strip-editor-text");
      if (form && area && area.value.trim() === (form.getAttribute("data-original") || "").trim()) closeEditor(half);
    }, 0);
  });

  // -- the bar's fragments on each applied projection (SI-368 (2), (7), (8)) ----

  var quietFocus = false; // a programmatic focus the popover must not open on

  function focusQuietly(el) {
    if (!el || !el.focus) return;
    quietFocus = true;
    el.focus({ preventScroll: true });
    quietFocus = false;
  }

  function applyCommit(html) {
    var cur = document.querySelector('[data-testid="wall-commit"]');
    if (!cur || typeof html !== "string" || !html) return;
    var wasOpen = !!cur.querySelector("details[open]");
    var active = document.activeElement;
    var focused = active && cur.contains(active) ? active.getAttribute("data-testid") : null;
    var tpl = document.createElement("template");
    tpl.innerHTML = html;
    var next = tpl.content.firstElementChild;
    if (!next) return;
    cur.replaceWith(next);
    if (wasOpen) {
      var d = next.querySelector("details");
      if (d) d.open = true;
    }
    if (focused) focusQuietly(next.querySelector('[data-testid="' + focused + '"]'));
  }

  // pillWords mirrors the server's readinessPillWords (wallbarrender.go):
  // the same words for the same facts — a lead the bar may fold away on a
  // narrow row, and the rest.
  function pillWords(pill) {
    if (pill.unavailable) return { lead: "readiness ", rest: "unavailable", state: "unavailable" };
    if (!pill.step) return { lead: "", rest: "Ready", state: "ready" };
    return { lead: "Step " + pill.step + " \u00b7 ", rest: (pill.unresolved || 0) + " to resolve", state: "step" };
  }

  function applyPill(pill) {
    var el = document.querySelector('[data-testid="readiness-pill"]');
    if (!el || !pill) return;
    var w = pillWords(pill);
    el.setAttribute("data-state", w.state);
    el.textContent = "";
    if (w.lead) el.appendChild(node("span", "readiness-pill-lead", w.lead));
    el.appendChild(document.createTextNode(w.rest));
    if (w.state === "unavailable") {
      el.setAttribute("title", w.lead + w.rest + ": " + pill.unavailable);
      el.removeAttribute("data-step");
      el.removeAttribute("data-unresolved");
      el.appendChild(node("span", "topbar-sr", ": " + pill.unavailable));
      return;
    }
    el.setAttribute("title", w.lead + w.rest);
    el.setAttribute("data-step", String(pill.step || 0));
    el.setAttribute("data-unresolved", String(pill.unresolved || 0));
  }

  function applyBranches(list) {
    var menu = document.getElementById("branch-menu");
    if (!menu || !Array.isArray(list)) return;
    var have = [];
    var items = menu.querySelectorAll("[data-branch]");
    for (var i = 0; i < items.length; i++) have.push(items[i].getAttribute("data-branch"));
    if (have.join("\n") === list.join("\n")) return;
    if (menu.contains(document.activeElement)) return; // never under the hand; the next refresh refills
    menu.textContent = "";
    for (var j = 0; j < list.length; j++) {
      var b = node("button", "", list[j]);
      b.type = "button";
      b.setAttribute("role", "menuitem");
      b.setAttribute("data-branch", list[j]);
      menu.appendChild(b);
    }
  }

  window.__WALLSTRIP__ = {
    apply: function (p) {
      if (!p) return;
      if (typeof p.uncommitted === "string" && p.uncommitted) applyCommit(p.uncommitted);
      if (p.pill) applyPill(p.pill);
      if (p.branches) applyBranches(p.branches);
    },
    openEditor: openEditor,
  };

  // -- Commit and push's popover: focus or click opens, Escape or an outside press closes ----

  function commitDetails() {
    return document.querySelector('[data-testid="wall-commit"] details');
  }

  document.addEventListener("focusin", function (e) {
    if (quietFocus) return;
    var d = commitDetails();
    var t = e.target;
    if (!d || d.open || !(t instanceof Element) || !d.contains(t) || !t.closest("summary")) return;
    d.open = true;
  });

  document.addEventListener("pointerdown", function (e) {
    var d = commitDetails();
    var t = e.target;
    if (!d || !(t instanceof Element)) return;
    if (d.contains(t)) {
      // A press on the closed summary: the click opens it; the focus the
      // press would move would open it first and the click close it again.
      if (!d.open && t.closest("summary")) e.preventDefault();
      return;
    }
    if (d.open) d.open = false;
  });

  // The focus leaving the popover closes it (it opened on focus, so a Tab
  // passing through the count never leaves it standing open). Focus moving
  // to another element closes it at once, with the key that moved it: a
  // timer would run behind the keys that follow, and an Escape among them
  // would close this popover, a layer the user has left, instead of the
  // next (SI-368 (17)). Focus going nowhere is settled once it lands.
  document.addEventListener("focusout", function (e) {
    var d = commitDetails();
    if (!d || !d.open) return;
    var to = e.relatedTarget;
    if (to instanceof Element) {
      if (!d.contains(to)) d.open = false;
      return;
    }
    setTimeout(function () {
      if (d.isConnected && d.open && !d.contains(document.activeElement)) d.open = false;
    }, 0);
  });

  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    var d = commitDetails();
    if (!d || !d.open) return;
    e.preventDefault();
    d.open = false;
    // The focus returns to the count only when it was in the popover or
    // nowhere; a focus elsewhere (a card) is never taken from it.
    var active = document.activeElement;
    if (!active || active === document.body || d.contains(active)) focusQuietly(d.querySelector("summary"));
  });

  // -- the branch menu's placement under the bar's switcher (SI-368 (7)) --------

  function switcher() {
    return document.querySelector('[data-testid="branch-switcher"]');
  }

  function placeMenu(menu, sw) {
    var r = sw.getBoundingClientRect();
    var room = document.documentElement.clientWidth;
    var left = r.left;
    if (left + menu.offsetWidth > room - 8) left = Math.max(8, room - menu.offsetWidth - 8);
    menu.style.position = "fixed";
    menu.style.top = Math.round(r.bottom + 4) + "px";
    menu.style.left = Math.round(left) + "px";
  }

  var menu = document.getElementById("branch-menu");
  if (menu) {
    // boardspec.js toggles the menu and wallkeys.js closes it on Escape;
    // whatever hid or showed it, the switcher says so and an open menu is
    // placed under it.
    if (window.MutationObserver) {
      new MutationObserver(function () {
        var sw = switcher();
        if (!sw) return;
        sw.setAttribute("aria-expanded", menu.hidden ? "false" : "true");
        if (!menu.hidden) placeMenu(menu, sw);
      }).observe(menu, { attributes: true, attributeFilter: ["hidden"] });
    }
    document.addEventListener("click", function (e) {
      var t = e.target;
      if (menu.hidden || !(t instanceof Element)) return;
      if (t.closest("#branch-menu") || t.closest('[data-testid="branch-switcher"]')) return;
      menu.hidden = true;
    });
    // The keyboard's way in: the menu sits at the body level, so Tab from
    // the switcher enters its first item, Shift+Tab from the first and Tab
    // from the last return to the switcher (the last closes the menu), and
    // the focus leaving both closes it.
    document.addEventListener("keydown", function (e) {
      if (e.key !== "Tab" || menu.hidden) return;
      var sw = switcher();
      var items = menu.querySelectorAll('[role="menuitem"]');
      var t = e.target;
      if (!items.length || !sw) return;
      if (t === sw && !e.shiftKey) {
        e.preventDefault();
        items[0].focus();
      } else if (t === items[0] && e.shiftKey) {
        e.preventDefault();
        sw.focus();
      } else if (t === items[items.length - 1] && !e.shiftKey) {
        e.preventDefault();
        menu.hidden = true;
        sw.focus();
      }
    });
    document.addEventListener("focusout", function (e) {
      if (menu.hidden) return;
      var to = e.relatedTarget;
      if (to instanceof Element && (menu.contains(to) || to.closest('[data-testid="branch-switcher"]'))) return;
      setTimeout(function () {
        var a = document.activeElement;
        if (!menu.hidden && a && a !== document.body && !menu.contains(a) && !a.closest('[data-testid="branch-switcher"]')) menu.hidden = true;
      }, 0);
    });
  }
})();
