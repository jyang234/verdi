// The wall's opener for the index's call to action (spec/index-v2 ac-4;
// SI-366 (11)): on /board/spec/<feature>?new-story=<ac> it opens the
// existing create dialog the way its own button does and checks the named
// criterion. It reads only the two hooks that contract names —
// #create-spec-btn and [data-create-ac] — and nothing else of the dialog,
// which stays boardspec.js's; without the query it does nothing. A
// criterion this wall does not declare is said so in the dialog's own
// error line, never checked as something else. Served at
// /assets/wallnewstory.js, within 2 KiB.
(function () {
  "use strict";
  var ac = new URLSearchParams(window.location.search).get("new-story");
  if (!ac) return;
  var btn = document.getElementById("create-spec-btn");
  if (!btn) return;
  btn.click();
  var boxes = document.querySelectorAll("#create-dialog [data-create-ac]");
  for (var i = 0; i < boxes.length; i++) {
    if (boxes[i].getAttribute("data-create-ac") === ac) {
      boxes[i].checked = true;
      return;
    }
  }
  var err = document.getElementById("create-error");
  if (!err) return;
  err.textContent = "Criterion " + ac + " is not one this wall declares; choose one below.";
  err.hidden = false;
})();
