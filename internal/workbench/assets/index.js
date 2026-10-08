// The index's script (spec/index-v2 ac-3; SI-366 (6), (19)): the filter
// row's pills over the one server-rendered DOM. Served at /assets/index.js
// as its own asset of at most 64 KiB (spec/workbench-redesign co-1). The
// page is complete before this runs — every card shown, every fold a
// native <details> — and this script only hides cards.
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
(function () {
  "use strict";
  var dir = document.querySelector(".home-directory");
  if (!dir) return;
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

  function apply(filter) {
    dir.setAttribute("data-filter", filter);
    var cards = dir.querySelectorAll(".dir-entry");
    for (var i = 0; i < cards.length; i++) {
      if (selects(filter, cards[i])) cards[i].removeAttribute("hidden");
      else cards[i].setAttribute("hidden", "");
    }
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
