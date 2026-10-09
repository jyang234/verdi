// The index's script (spec/index-v2 ac-3, ac-6; SI-366 (6), (7), (14),
// (19)): the filter row's pills, the bar's view toggle and the keyboard,
// over the one server-rendered DOM. Served at /assets/index.js as its
// own asset of at most 64 KiB (spec/workbench-redesign co-1). The page is
// complete before this runs — every card shown, every fold a native
// <details>, the view the query chose drawn, every link a real address —
// and this script only hides cards, switches the directory's data-view,
// and moves focus.
//
// A pill selects by the facts each card already carries as attributes
// (directory.go's li open tag): quiet reads data-quiet, in review reads
// data-review, disclosed reads data-disclosed; everything shows all. A
// card a filter does not select gets the hidden attribute; nothing else
// changes — the column counts above the cards stay the totals the server
// wrote (19), and the pills' own counts were computed server-side from the
// same facts (indexfilters.go). A disabled pill (the in-review pill when
// the forge was unconfigured or unavailable) never selects: what it would
// select is unknown.
//
// The view toggle's links are real addresses (/ and /?view=list) the
// server honours without script; here a plain click on one switches the
// view in place and records the address, so a reload keeps it. A
// modified click (a new tab) is left to the browser.
//
// The keyboard (SI-366 (14)): the arrows move focus among the cards'
// primary links — the board link where the routing serves one, else the
// corpus link — in document order through every card a reader can see
// (not hidden by a filter, not inside a closed fold), in either view:
// Down and Right to the next, Up and Left to the previous, no wrap and
// no trap (Tab leaves as it always did; the ends simply stop). A card
// with no link at all (the no-draft entry) is made focusable with
// tabindex -1 so the arrows reach it; Enter follows a focused link
// natively and does nothing on such a card — this script never handles
// Enter.
(function () {
  "use strict";
  var dir = document.querySelector(".home-directory");
  if (!dir) return;

  var toggle = document.querySelector(".topbar-view");
  function setView(view) {
    dir.setAttribute("data-view", view);
    if (!toggle) return;
    var links = toggle.querySelectorAll("a[data-view]");
    for (var i = 0; i < links.length; i++) {
      if (links[i].getAttribute("data-view") === view) links[i].setAttribute("aria-current", "page");
      else links[i].removeAttribute("aria-current");
    }
  }
  if (toggle) {
    toggle.addEventListener("click", function (e) {
      var link = e.target.closest ? e.target.closest("a[data-view]") : null;
      if (!link || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
      e.preventDefault();
      setView(link.getAttribute("data-view"));
      if (window.history && window.history.replaceState) {
        window.history.replaceState(null, "", link.getAttribute("href"));
      }
    });
  }

  // --- the keyboard ---
  function primary(card) {
    return card.querySelector("a.dir-board") || card.querySelector("a.dir-title") || card;
  }
  function reachable(card) {
    if (card.hasAttribute("hidden")) return false;
    for (var el = card.parentNode; el && el !== dir; el = el.parentNode) {
      if (el.tagName === "DETAILS" && !el.open) return false;
    }
    return true;
  }
  function targets() {
    var out = [];
    var cards = dir.querySelectorAll(".dir-entry");
    for (var i = 0; i < cards.length; i++) {
      if (reachable(cards[i])) out.push(primary(cards[i]));
    }
    return out;
  }
  var all = dir.querySelectorAll(".dir-entry");
  for (var c = 0; c < all.length; c++) {
    if (primary(all[c]) === all[c]) all[c].setAttribute("tabindex", "-1");
  }
  dir.addEventListener("keydown", function (e) {
    if (e.defaultPrevented || e.altKey || e.ctrlKey || e.metaKey || e.shiftKey) return;
    var step = 0;
    if (e.key === "ArrowDown" || e.key === "ArrowRight") step = 1;
    else if (e.key === "ArrowUp" || e.key === "ArrowLeft") step = -1;
    if (!step) return;
    var card = e.target.closest ? e.target.closest(".dir-entry") : null;
    if (!card) return;
    var list = targets();
    var at = list.indexOf(primary(card));
    if (at < 0) return;
    var next = list[at + step];
    if (!next) return;
    e.preventDefault();
    next.focus();
  });

  // --- the filters ---
  var pills = Array.prototype.slice.call(dir.querySelectorAll(".dir-filter[data-filter]"));
  if (!pills.length) return;

  function selects(filter, card) {
    switch (filter) {
      case "quiet":
        return card.getAttribute("data-quiet") === "true";
      case "in-review":
        return card.getAttribute("data-review") === "open";
      case "disclosed":
        return card.getAttribute("data-disclosed") === "true";
      default:
        return true;
    }
  }

  // A filter's matches inside a closed archived fold would otherwise be
  // hidden twice over, so the fold's summary says how many of its cards
  // the pressed filter selects (F7BR-1); with everything pressed the
  // mark is empty and hidden again, the count beside it being the whole.
  function markFolds(filter) {
    var folds = dir.querySelectorAll("details.dir-archived");
    for (var i = 0; i < folds.length; i++) {
      var mark = folds[i].querySelector("summary .dir-archived-matched");
      if (!mark) continue;
      if (filter === "everything") {
        mark.textContent = "";
        mark.hidden = true;
        continue;
      }
      var n = folds[i].querySelectorAll(".dir-entry:not([hidden])").length;
      mark.textContent = "· " + n + " match";
      mark.hidden = false;
    }
  }

  function apply(filter) {
    dir.setAttribute("data-filter", filter);
    var cards = dir.querySelectorAll(".dir-entry");
    for (var i = 0; i < cards.length; i++) {
      if (selects(filter, cards[i])) cards[i].removeAttribute("hidden");
      else cards[i].setAttribute("hidden", "");
    }
    markFolds(filter);
    for (var j = 0; j < pills.length; j++) {
      var on = pills[j].getAttribute("data-filter") === filter;
      pills[j].setAttribute("aria-pressed", on ? "true" : "false");
    }
  }

  for (var k = 0; k < pills.length; k++) {
    pills[k].addEventListener("click", function (e) {
      var pill = e.currentTarget;
      if (pill.disabled) return;
      apply(pill.getAttribute("data-filter"));
    });
  }
})();
