// The New story dialog's own script (spec/new-story-dialog-v2 ac-1, ac-2;
// SI-369 (5)-(9)): as the name is typed it writes the branch the dialog
// will cut and the name's hint or grammar report, and it gates Create —
// enabled exactly when the name is valid and at least one criterion is
// claimed — beside a status line naming what is missing next. Everything
// it says is the server's: the name pattern (data-pattern, the server's
// own grammar, compiled as typed and never lowercased), the branch prefix
// and every sentence ride the dialog's data attributes, so no class word
// and no grammar is written here. A new asset for the new behaviour, so
// boardspec.js, which owns the dialog's opening and its submit, does not
// grow. It loads after boardspec.js and before wallnewstory.js, whose
// opener clicks #create-spec-btn and then checks a criterion without an
// event: the gate is recomputed in a microtask after that click, so the
// prefilled criterion counts. Served at /assets/newstorydialog.js.
(function () {
  "use strict";
  var dialog = document.getElementById("create-dialog");
  var name = document.getElementById("create-name");
  if (!dialog || !name) return;
  var chip = document.getElementById("create-branch-tab");
  var hint = document.getElementById("create-name-hint");
  var ok = document.getElementById("create-ok");
  var status = document.getElementById("create-status");
  var title = dialog.querySelector('[data-field="Title"]');
  var titlePlaceholder = title ? title.getAttribute("placeholder") || "" : "";
  // "(?!)" never matches: a missing pattern leaves every name invalid and
  // Create gated, never a name the server would refuse let through.
  var grammar = new RegExp(name.getAttribute("data-pattern") || "(?!)");

  // fill substitutes value for every {key} in the server's template,
  // literally: a typed "$&" is text, never a replacement pattern.
  function fill(el, attr, key, value) {
    return ((el && el.getAttribute(attr)) || "").split("{" + key + "}").join(value);
  }

  // humanize is the Title a valid name derives when Title is left blank
  // (designscaffold.HumanizeName), shown as the field's placeholder.
  function humanize(valid) {
    return valid
      .split("-")
      .map(function (part) {
        return part.charAt(0).toUpperCase() + part.slice(1);
      })
      .join(" ");
  }

  // conjoin lists the empty required fields: "Problem", "Problem and
  // Outcome", "A, B and C".
  function conjoin(labels) {
    if (labels.length < 2) return labels.join("");
    return labels.slice(0, -1).join(", ") + " and " + labels[labels.length - 1];
  }

  function refresh() {
    var typed = name.value.trim();
    var valid = grammar.test(typed);
    if (chip) chip.textContent = fill(chip, "data-cut", "name", valid ? typed : "…");
    if (hint) {
      hint.textContent = !typed
        ? hint.getAttribute("data-text-empty") || ""
        : fill(hint, valid ? "data-text-valid" : "data-text-invalid", "name", typed);
      hint.setAttribute("data-state", valid ? "valid" : typed ? "invalid" : "empty");
    }
    if (typed && !valid) name.setAttribute("aria-invalid", "true");
    else name.removeAttribute("aria-invalid");
    if (title) title.placeholder = valid ? humanize(typed) : titlePlaceholder;

    var claimed = [];
    dialog.querySelectorAll("[data-create-ac]").forEach(function (box) {
      if (box.checked) claimed.push(box.getAttribute("data-create-ac"));
    });
    var missing = [];
    dialog.querySelectorAll("[data-field][required]").forEach(function (field) {
      if (!field.value.trim()) missing.push(field.getAttribute("data-label") || field.getAttribute("data-field"));
    });
    if (ok) ok.disabled = !(valid && claimed.length > 0);
    if (!status) return;
    if (!valid) {
      status.textContent = status.getAttribute("data-name") || "";
    } else if (claimed.length === 0) {
      status.textContent = status.getAttribute("data-acs") || "";
    } else {
      var line = fill(status, "data-ready", "name", typed).split("{acs}").join(claimed.join(", "));
      if (missing.length > 0) {
        line += " · " + fill(status, missing.length > 1 ? "data-missing-many" : "data-missing-one", "fields", conjoin(missing));
      }
      status.textContent = line;
    }
  }

  function onEdit(e) {
    if (dialog.contains(e.target)) refresh();
  }
  document.addEventListener("input", onEdit);
  document.addEventListener("change", onEdit);
  document.addEventListener("click", function (e) {
    if (e.target && e.target.closest && e.target.closest("#create-spec-btn")) queueMicrotask(refresh);
  });
  refresh();
})();
