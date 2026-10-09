// The New story dialog's own script (spec/new-story-dialog-v2; SI-369
// (9)): the branch preview, the inline grammar report, the gated Create,
// the status line and the criterion rows, drawn from the server's data
// attributes — a new asset for the new behaviour, so boardspec.js does
// not grow. It loads after boardspec.js, which owns the dialog and its
// button, and before wallnewstory.js, so a criterion that opener checks
// is seen. Served at /assets/newstorydialog.js, within 64 KiB. A
// placeholder until the dialog's presentation lane (F6a) fills it.
(function () {
  "use strict";
})();
