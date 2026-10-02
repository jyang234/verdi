// verdi dex — search.js
//
// This file carries two of dex's three budgeted client-side JS behaviors
// (05 §Verdi-dex mechanics: "client-side JavaScript budget is exactly
// three items"):
//
//   1. Search over the build-emitted JSON inverted index
//      (/search-index.json) with a small vanilla lookup, active on any
//      page carrying a #search-box / #search-results pair.
//   2. The copy-reference button's clipboard behavior, via a single
//      delegated click listener for every [data-copy-ref] element on the
//      page. The button itself is emitted per PLAN.md Phase 12's own
//      instruction: "keep it clipboard-API inline attribute or make it
//      part of the search JS file" — dex chose the latter, so every page's
//      copy-reference button stays plain, inert HTML unless this one
//      sitewide script is present, and the whole clipboard interaction
//      lives in one place rather than duplicated per page as inline
//      attributes.
//
// No build step, no bundler, no third-party library: this is the "small
// vanilla lookup" the spec calls for.
(function () {
  "use strict";

  // -- 2. copy-reference button (delegated, sitewide) --------------------
  document.addEventListener("click", function (ev) {
    var el = ev.target.closest && ev.target.closest("[data-copy-ref]");
    if (!el) return;
    var ref = el.getAttribute("data-copy-ref");
    if (!ref || !navigator.clipboard) return;
    navigator.clipboard.writeText(ref).then(function () {
      // The clipboard got the FULL pinned form (ref); the button's visible
      // label may be sha-shortened, so restore its markup, not just text.
      var original = el.innerHTML;
      el.textContent = "Copied full reference";
      setTimeout(function () {
        el.innerHTML = original;
      }, 1500);
    });
  });

  // -- 1. search ----------------------------------------------------------
  var box = document.getElementById("search-box");
  var results = document.getElementById("search-results");
  if (!box || !results) return;

  var indexPromise = fetch("/search-index.json").then(function (r) {
    return r.json();
  });

  // tokenize mirrors internal/index's server-side tokenizer exactly:
  // lowercase, split on maximal runs of [a-z0-9].
  function tokenize(s) {
    var m = s.toLowerCase().match(/[a-z0-9]+/g);
    return m || [];
  }

  function search(index, query) {
    var qtokens = tokenize(query);
    var seen = {};
    var scores = {};
    for (var i = 0; i < qtokens.length; i++) {
      var t = qtokens[i];
      if (seen[t]) continue;
      seen[t] = true;
      var postings = index.tokens[t] || [];
      for (var j = 0; j < postings.length; j++) {
        var p = postings[j];
        scores[p.ref] = (scores[p.ref] || 0) + p.score;
      }
    }
    var refs = Object.keys(scores);
    refs.sort(function (a, b) {
      if (scores[a] !== scores[b]) return scores[b] - scores[a];
      return a < b ? -1 : a > b ? 1 : 0;
    });
    return refs;
  }

  function render(index, refs) {
    results.innerHTML = "";
    if (refs.length === 0) {
      var li = document.createElement("li");
      li.textContent = "No results.";
      results.appendChild(li);
      return;
    }
    for (var i = 0; i < refs.length; i++) {
      var ref = refs[i];
      var meta = index.refs[ref] || {};
      var li = document.createElement("li");
      var a = document.createElement("a");
      a.href = "/a/" + ref + "/";
      a.textContent = (meta.title || ref) + " (" + ref + ")";
      li.appendChild(a);
      results.appendChild(li);
    }
  }

  indexPromise.then(function (index) {
    box.addEventListener("input", function () {
      var q = box.value.trim();
      if (!q) {
        results.innerHTML = "";
        return;
      }
      render(index, search(index, q));
    });
  });
})();
