// verdi dex — openapi-renderer.js
//
// The third of dex's three budgeted client-side JS items (05 §Verdi-dex
// mechanics): "an OpenAPI renderer (script tag per API page; the committed
// spec file is the source of truth, discovered by convention at
// <service-root>/api/openapi.{yaml,yml,json} ...)".
//
// dex build transcodes the discovered OpenAPI document (YAML or JSON) to a
// canonical JSON file at build time (internal/dex/openapi.go) so this
// script never needs a YAML parser in the browser. It reads its own
// <script data-openapi-json="..."> attribute, fetches that JSON, and
// renders a small, dependency-free table of paths/methods/summaries — not
// a full Swagger-UI-class renderer, which would pull in a large
// third-party bundle and blow the three-item budget.
(function () {
  "use strict";

  var script = document.currentScript;
  if (!script) return;
  var src = script.getAttribute("data-openapi-json");
  var root = document.getElementById("openapi-root");
  if (!src || !root) return;

  fetch(src)
    .then(function (r) {
      return r.json();
    })
    .then(function (doc) {
      render(doc);
    })
    .catch(function (err) {
      root.textContent = "Failed to load OpenAPI document: " + err;
    });

  function render(doc) {
    var info = doc.info || {};
    var header = document.createElement("p");
    header.textContent =
      (info.title || "API") + (info.version ? " · v" + info.version : "");
    root.appendChild(header);

    var paths = doc.paths || {};
    var pathKeys = Object.keys(paths).sort();
    if (pathKeys.length === 0) {
      var empty = document.createElement("p");
      empty.textContent = "No paths declared.";
      root.appendChild(empty);
      return;
    }

    var table = document.createElement("table");
    var thead = document.createElement("thead");
    thead.innerHTML = "<tr><th>Method</th><th>Path</th><th>Summary</th></tr>";
    table.appendChild(thead);
    var tbody = document.createElement("tbody");

    var methods = ["get", "put", "post", "delete", "options", "head", "patch", "trace"];
    for (var i = 0; i < pathKeys.length; i++) {
      var p = pathKeys[i];
      var ops = paths[p] || {};
      for (var m = 0; m < methods.length; m++) {
        var method = methods[m];
        if (!ops[method]) continue;
        var tr = document.createElement("tr");
        var tdMethod = document.createElement("td");
        tdMethod.textContent = method.toUpperCase();
        var tdPath = document.createElement("td");
        tdPath.textContent = p;
        var tdSummary = document.createElement("td");
        tdSummary.textContent = ops[method].summary || "";
        tr.appendChild(tdMethod);
        tr.appendChild(tdPath);
        tr.appendChild(tdSummary);
        tbody.appendChild(tr);
      }
    }
    table.appendChild(tbody);
    root.appendChild(table);
  }
})();
