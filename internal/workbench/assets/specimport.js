// specimport.js — the spec importer's browser script (spec-import-contract
// "Errors and browser behavior"; plan Task 4 UI; the owner-approved
// usability correction — main's usability-ui-adjudication over the FABLE
// D1-D6 proposal). Dependency-free, no frameworks, no storage authority.
// It owns
//
//   - file reading: each chosen file's exact bytes, base64-encoded without
//     any text decode, labeled by the file NAME (never a path), with a
//     mechanical source id derived from that name;
//   - mapping by selection (spec/uat-round-1 ac-8, UAT-006): each source's
//     selected slice is rendered read-only beneath its row, decoded as
//     UTF-8 for display only; a selection inside ONE source, mapped to a
//     target picked from the statements, the known object ids or a new
//     object of any of the four kinds, becomes a source-backed mapping
//     whose start/end are UTF-8 BYTE offsets computed by walking the
//     source's own byte array (never JS string indices) and cross-checked
//     against the selected text; an empty, outside, foreign or
//     two-source selection is refused with a visible message; the mapping
//     list shows the covered text beside each byte range;
//   - request composition: the exact strict Request JSON the server's ONE
//     decoder accepts — sources, format, target, the two choices, explicit
//     mappings and declared links — nothing inferred, nothing invented;
//   - the preview fetch and its rendering, in reading order: a plain-
//     language readiness line; the blockers, each with guidance keyed by
//     finding CODE and TARGET SHAPE (never by message text) and a jump to
//     the card it concerns or into the advanced controls; the statements
//     (present cards, or placeholders offering the direct "Write it" and
//     pair-TODO actions); the evidence helper the page serves BEFORE the
//     cards; the object cards under an origin-neutral count, each with its
//     exact provenance, its status, a copy of its own findings beside the
//     controls that resolve them, and its evidence controls; the rest of
//     the source with its explicit keep action; and the byte accounting
//     under a collapsed technical region;
//   - invalidation: EVERY edit (file, range, primary, format, target,
//     choice, mapping, evidence, link, written text) bumps an edit epoch,
//     marks the shown preview stale — replacing the current-sounding
//     readiness line, marking every earlier finding as such and restating
//     each card's status from the page's own choices as NOT yet previewed
//     — and clears and disables the confirmation; a preview response for
//     an older epoch — or an older request — is discarded on arrival, so a
//     stale response can never reinstate a confirmation;
//   - apply: only the exact request bytes previewed, with that preview's
//     digest in X-Verdi-Import-Preview, and only while confirmed.
//
// Nothing is chosen for the user: no evidence kind, no retention, no
// deferral. The page's own actions (keep the remaining text, leave both
// statements as TODOs, apply one card's kinds to every criterion) are
// explicit clicks that set the form's own controls; the request flags and
// mapping shapes are unchanged. Everything the server or a source supplies
// is rendered with textContent; no source byte becomes markup. No polling,
// no AI, no provider, no client-side acceptance judgment.
(function () {
  "use strict";
  var form = document.getElementById("import-form");
  if (!form) return;

  function $(id) {
    return document.getElementById(id);
  }
  var filesInput = $("import-files");
  var sourceList = $("import-source-list");
  var mappingList = $("import-mapping-list");
  var linkList = $("import-link-list");
  var nextAction = $("import-next-action");
  var errorEl = $("import-error");
  var resultEl = $("import-result");
  var staleNote = $("import-stale-note");
  var readyEl = $("import-ready");
  var digestEl = $("import-digest");
  var findingsHeading = $("import-findings-heading");
  var findingsEl = $("import-findings");
  var statementsEl = $("import-statements");
  var guideEl = $("import-evidence-guide");
  var fieldsEl = $("import-fields");
  var remainingEl = $("import-remaining");
  var coverageBody = $("import-coverage").querySelector("tbody");
  var sourcesEl = $("import-sources");
  var confirmEl = $("import-confirm");
  var applyBtn = $("import-apply-btn");
  var createdEl = $("import-created");
  var retryEl = $("import-retry");
  var retryNote = $("import-retry-note");
  var retryBtn = $("import-retry-btn");
  var advancedEl = $("import-advanced");
  var mappingCountEl = $("import-mapping-count");
  var linkCountEl = $("import-link-count");
  var storyNote = $("import-story-note");
  var classSelect = $("import-class");
  var formatSelect = $("import-format");
  var retainEl = $("import-retain");
  var deferEl = $("import-defer");

  var EVIDENCE_KINDS = ["static", "behavioral", "runtime", "attestation"];
  // Concrete words with the technical kind in parentheses (the evidence
  // model's four kinds; meanings in the served helper). The enum values on
  // the wire and in data-kind stay bare.
  var KIND_LABELS = {
    static: "Code fact (static)",
    behavioral: "Suite test (behavioral)",
    runtime: "Live probe (runtime)",
    attestation: "Human sign-off (attestation)",
  };
  var TRANSFORMS = ["identity", "trim-blank-lines", "collapse-whitespace", "list-item"];
  var LINK_TYPES = ["implements", "resolves", "supersedes", "exempts", "verifies", "derived-from", "annotates", "depends-on", "story", "impacts", "challenges"];
  var MAX_SOURCES = 32;
  var STATEMENT_NAMES = { problem: "Problem statement", outcome: "Outcome statement" };
  var OBJECT_GROUPS = [
    ["ac-", "acceptance criterion", "acceptance criteria"],
    ["co-", "constraint", "constraints"],
    ["dc-", "decision", "decisions"],
    ["oq-", "open question", "open questions"],
  ];
  // Sources above this size are rendered collapsed (still selectable once
  // expanded); smaller ones show their text at once.
  var LARGE_SOURCE_BYTES = 262144;
  // The mapping list's excerpt shows at most this many characters of the
  // covered text's first line.
  var EXCERPT_CHARS = 140;
  // The value prefix of the picker's "new <kind>" choices.
  var NEW_TARGET = "new:";

  // Finding code -> a plain-language title. The code itself stays visible
  // beside it; the server's message is always rendered verbatim.
  var FINDING_TITLES = {
    "missing-statement": "Statement missing from the source",
    "empty-field": "Field resolved to nothing",
    "ambiguous-field": "Field labeled more than once",
    "multiple-targets": "More than one document selected",
    "unsupported-structure": "Cannot be composed as selected",
    "missing-evidence": "Evidence kinds not chosen",
    "unresolved-coverage": "Leftover source text has no choice yet",
    "source-id-requires-mapping": "Source id needs an explicit mapping",
    "invalid-candidate": "Fails the store's validation",
    "existing-corpus-finding": "Pre-existing store finding",
    "statements-deferred": "TODO placeholder in place",
  };
  // Finding code -> the next corrective action, in plain words. Guidance
  // is keyed by code (and, for unsupported-structure, by the target's
  // shape), never by matching message text.
  var GUIDANCE = {
    "missing-statement": "Write it on its card below, add a labeled Problem or Outcome section to the source, or leave both statements as TODOs for now (an explicit choice).",
    "empty-field": "The labeled section is empty: add its text to the source or map the field explicitly in Advanced.",
    "ambiguous-field": "The source labels this field more than once; write the text to use on its card, or add an explicit mapping naming it in Advanced. The importer never picks one.",
    "multiple-targets": "The source holds more than one top-level document; narrow the line range to one.",
    "missing-evidence": "Choose the evidence kinds on this criterion's card (your choice, never inferred); the helper above the cards explains them.",
    "unresolved-coverage": "Choose 'Keep the remaining source text as reference material' (in step 4, or below under the rest of the source), or map the remaining bytes in Advanced.",
    "source-id-requires-mapping": "The item carries its own id; add an explicit mapping in Advanced that preserves or resolves it.",
    "invalid-candidate": "The composed spec fails the store's validation; the message names the rule. Correct the named field, target, link or tracker reference.",
    "existing-corpus-finding": "A pre-existing store finding, disclosed separately; it was not caused by this import.",
    "statements-deferred": "Disclosure only: a TODO placeholder stands in for this statement until it is replaced on the board.",
  };
  // unsupported-structure is one closed code for several causes. Only the
  // primary-target finding is the heading fault the Markdown reader
  // reports; every other target (template, target.class, a field id, or
  // none) gets neutral, cause-aware guidance that repeats no assumption
  // about the source or the template being sound.
  var GUIDANCE_PRIMARY_STRUCTURE =
    "The primary file has no Markdown heading this profile can read: add a title heading, choose another reading profile, or map the fields explicitly in Advanced.";
  var GUIDANCE_STRUCTURE_NEUTRAL =
    "The importer could not compose the candidate as selected; the message names what it could not express (a template, model, class or field). That is not a claim that the source or the template is otherwise sound: correct what the message names, then preview again.";
  var ERROR_GUIDANCE = {
    "dirty-context": "Commit or remove the named paths in the serving checkout, then preview again; nothing was reset.",
    "stale-preview": "The server recomputed a different digest; preview again and confirm the fresh preview.",
    "target-exists": "That spec name is already taken on this store; choose a different name. The existing branch was left untouched, never renamed or republished.",
    "unresolved": "The preview still has blocking findings; correct them and preview again.",
    "policy-forbidden": "The store's adopted policy refuses this write.",
    "actor-forbidden": "This write's actor was refused.",
    "invalid-request": "Correct the named input and preview again.",
    "invalid-source": "The named source cannot be honored as selected (bytes, range or encoding).",
    "unsupported-format": "Choose one of the listed reading profiles; the F13 profile only accepts its pinned primary bytes.",
  };

  var state = {
    sources: [], // {id, label, data, bytes, size, startLine, endLine}
    primary: "",
    mappings: [], // {target, sourceId, start, end, transform, text, evidence}
    links: [], // {type, ref}
    epoch: 0,
    previewSeq: 0,
    preview: null, // {result, request, epoch}
    // created: the last publication outcome {result, epoch}; it stays
    // visible (marked stale) across later edits — a publication is never
    // silently forgotten on this page.
    created: null,
    // applyAttempt: the one creation request in flight or whose response
    // was lost {epoch, digest, request, status: "in-flight"|"lost"}.
    applyAttempt: null,
  };

  // -- helpers --------------------------------------------------------------
  function el(tag, attrs, text) {
    var node = document.createElement(tag);
    if (attrs) {
      for (var k in attrs) {
        if (Object.prototype.hasOwnProperty.call(attrs, k)) node.setAttribute(k, attrs[k]);
      }
    }
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }
  function clear(node) {
    while (node.firstChild) node.removeChild(node.firstChild);
  }
  function sourceById(sourceId) {
    for (var i = 0; i < state.sources.length; i++) {
      if (state.sources[i].id === sourceId) return state.sources[i];
    }
    return null;
  }
  function labelOf(sourceId) {
    var src = sourceById(sourceId);
    return src ? src.label : sourceId;
  }
  function isAC(target) {
    return typeof target === "string" && target.indexOf("ac-") === 0;
  }
  function isStatement(target) {
    return target === "problem" || target === "outcome";
  }
  function slugOf(specRef) {
    return String(specRef || "").replace(/^spec\//, "");
  }
  function kindLabel(kind) {
    return KIND_LABELS[kind] || kind;
  }
  function plural(n, one, many) {
    return n + " " + (n === 1 ? one : many);
  }
  function sameKinds(a, b) {
    var x = (a || []).slice().sort().join(",");
    var y = (b || []).slice().sort().join(",");
    return x === y;
  }
  function isEditable() {
    return formatSelect.value !== "native";
  }
  // tick sets one of the form's own checkboxes from a page action and lets
  // the form's ordinary change path (sync, invalidate) run — the same
  // effect as the user ticking it in step 4.
  function tick(box) {
    if (!box || box.checked) return;
    box.checked = true;
    box.dispatchEvent(new Event("change", { bubbles: true }));
  }
  function openAdvanced(focusId) {
    if (advancedEl) advancedEl.open = true;
    var target = focusId ? $(focusId) : null;
    if (target && typeof target.focus === "function") target.focus();
  }
  function syncAdvancedSummary() {
    if (mappingCountEl) mappingCountEl.textContent = String(state.mappings.length);
    if (linkCountEl) linkCountEl.textContent = String(state.links.length);
  }
  // syncClassNotes shows the class-specific notes the page serves for both
  // classes: the story's link requirement in the target step, and the
  // evidence helper's floor sentence for the chosen class.
  function syncClassNotes() {
    var cls = classSelect.value;
    if (storyNote) storyNote.hidden = cls !== "story";
    var notes = document.querySelectorAll("[data-import-class]");
    for (var i = 0; i < notes.length; i++) {
      notes[i].hidden = notes[i].getAttribute("data-import-class") !== cls;
    }
  }

  // The mechanical source id: the file NAME lowercased, every
  // non-alphanumeric run collapsed to one hyphen, trimmed, at most 64
  // characters ([a-z0-9][a-z0-9-]{0,63}); de-duplicated with a numeric
  // suffix.
  function sourceIdFor(name) {
    var id = String(name).toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "").slice(0, 64);
    if (!id) id = "source";
    var base = id;
    var n = 2;
    while (hasSource(id)) {
      id = base.slice(0, 60) + "-" + n;
      n++;
    }
    return id;
  }
  function hasSource(id) {
    for (var i = 0; i < state.sources.length; i++) {
      if (state.sources[i].id === id) return true;
    }
    return false;
  }

  // Exact bytes -> base64, chunked over the byte array; no text decode, no
  // normalization, no BOM or newline changes.
  function base64Of(bytes) {
    var s = "";
    for (var i = 0; i < bytes.length; i += 0x8000) {
      s += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
    }
    return btoa(s);
  }
  function readFile(file) {
    return file.arrayBuffer().then(function (buf) {
      var bytes = new Uint8Array(buf);
      return { label: file.name, data: base64Of(bytes), bytes: bytes, size: buf.byteLength };
    });
  }

  // -- bytes, slices and selections -------------------------------------------------
  // selectedSlice mirrors the server's selectLineRange over a source's
  // exact bytes: both lines zero is the whole file; otherwise inclusive,
  // 1-based physical LF-terminated lines with an unterminated final line
  // counted, CRLF preserved. Every mapping offset is relative to THIS
  // slice, so the rendered text is this slice and nothing else.
  function selectedSlice(src) {
    var data = src.bytes;
    var s = src.startLine;
    var e = src.endLine;
    if (!s && !e) return { bytes: data };
    if (s <= 0 || e <= 0) return { error: "start line and end line must both be set" };
    if (s > e) return { error: "start line " + s + " is after end line " + e };
    var offsets = [0];
    for (var i = 0; i < data.length; i++) {
      if (data[i] === 10) offsets.push(i + 1);
    }
    if (data.length && data[data.length - 1] !== 10) offsets.push(data.length);
    var total = offsets.length - 1;
    if (e > total) return { error: "end line " + e + " exceeds the file's " + total + " lines" };
    return { bytes: data.subarray(offsets[s - 1], offsets[e]) };
  }
  // decodeUTF8 decodes exact bytes for DISPLAY only, strictly (invalid
  // UTF-8 yields null, never a substituted character that would break the
  // byte correspondence) and with a leading BOM kept as text for the same
  // reason. Nothing decoded is ever sent.
  function decodeUTF8(bytes) {
    try {
      return new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(bytes);
    } catch (e) {
      return null;
    }
  }
  function runeBoundary(bytes, i) {
    if (i === bytes.length) return true;
    if (i < 0 || i > bytes.length) return false;
    return (bytes[i] & 0xc0) !== 0x80;
  }
  // byteOffsetsForUnits walks the byte array rune by rune, counting the
  // UTF-16 code units each rune occupies in the decoded text (two for a
  // four-byte sequence, one otherwise), and returns the byte offsets at
  // which the given code-unit offsets fall — null if either falls inside a
  // rune. Offsets are thus computed from the bytes, never from string
  // indices.
  function byteOffsetsForUnits(bytes, startUnits, endUnits) {
    var units = 0;
    var i = 0;
    var start = -1;
    var end = -1;
    for (;;) {
      if (units === startUnits && start < 0) start = i;
      if (units === endUnits) {
        end = i;
        break;
      }
      if (i >= bytes.length || units > endUnits) break;
      var b = bytes[i];
      var len = b < 0x80 ? 1 : b >= 0xf0 ? 4 : b >= 0xe0 ? 3 : b >= 0xc0 ? 2 : 1;
      units += len === 4 ? 2 : 1;
      i += len;
    }
    if (start < 0 || end < 0) return null;
    return { start: start, end: end };
  }
  function sourceTextOf(node) {
    var elNode = node && node.nodeType === 3 ? node.parentNode : node;
    return elNode && elNode.closest ? elNode.closest(".import-source-text") : null;
  }
  // selectionIn resolves the browser's live selection against one source's
  // rendered text: refused (with the reason a reader sees) when empty,
  // outside every source, inside another source, or spanning two; else the
  // exact byte range of the selected text within the source's selected
  // slice, verified by decoding those bytes back and comparing them with
  // the selected text itself.
  function selectionIn(pre, src) {
    var sel = window.getSelection();
    if (!sel || sel.rangeCount === 0 || sel.isCollapsed) {
      return { error: "Nothing is selected. Select a passage in the text above (click and drag over it), then choose Map selection." };
    }
    var range = sel.getRangeAt(0);
    var startPre = sourceTextOf(range.startContainer);
    var endPre = sourceTextOf(range.endContainer);
    if (!startPre && !endPre) {
      return { error: "The selection is not inside a source's text. Select a passage in the text above, then choose Map selection." };
    }
    if (startPre !== endPre) {
      return { error: "The selection spans two sources or runs outside one; a mapping covers a passage in one source only. Select within this source's text." };
    }
    if (startPre !== pre) {
      return { error: "The selection is in " + labelOf(startPre.getAttribute("data-source-id")) + "; use that source's own Map selection." };
    }
    var text = range.toString();
    if (!text.trim()) return { error: "The selection contains only whitespace; select the passage itself." };
    var prefix = document.createRange();
    prefix.selectNodeContents(pre);
    prefix.setEnd(range.startContainer, range.startOffset);
    var startUnits = prefix.toString().length;
    var slice = selectedSlice(src);
    if (slice.error) return { error: "The line range for this source is not valid (" + slice.error + "), so nothing can be mapped from it." };
    var offsets = byteOffsetsForUnits(slice.bytes, startUnits, startUnits + text.length);
    if (!offsets || decodeUTF8(slice.bytes.subarray(offsets.start, offsets.end)) !== text) {
      return { error: "The selection does not fall on character boundaries of the source's bytes; select the passage again." };
    }
    return { start: offsets.start, end: offsets.end, text: text };
  }
  // knownTargets lists the object ids the page knows per kind prefix: the
  // last preview's fields (stale or not — they return on the next
  // preview) and the current mappings' targets, sorted by number.
  function knownTargets() {
    var known = {};
    OBJECT_GROUPS.forEach(function (g) {
      known[g[0]] = [];
    });
    function add(target) {
      OBJECT_GROUPS.forEach(function (g) {
        if (typeof target === "string" && target.indexOf(g[0]) === 0 && known[g[0]].indexOf(target) < 0) known[g[0]].push(target);
      });
    }
    if (state.preview) {
      (state.preview.result.fields || []).forEach(function (f) {
        add(f.target);
      });
    }
    state.mappings.forEach(function (m) {
      add(m.target);
    });
    OBJECT_GROUPS.forEach(function (g) {
      known[g[0]].sort(function (a, b) {
        var na = parseInt(a.slice(g[0].length), 10);
        var nb = parseInt(b.slice(g[0].length), 10);
        if (isNaN(na) || isNaN(nb) || na === nb) return a < b ? -1 : a > b ? 1 : 0;
        return na - nb;
      });
    });
    return known;
  }
  // nextIdFor follows the importer's own numbering (1-based ordinals per
  // kind): one past the highest numbered id the page knows for that kind.
  function nextIdFor(prefix) {
    var max = 0;
    (knownTargets()[prefix] || []).forEach(function (id) {
      var n = parseInt(id.slice(prefix.length), 10);
      if (!isNaN(n) && n > max) max = n;
    });
    return prefix + (max + 1);
  }
  function fillMapPicker(select) {
    var prev = select.value;
    clear(select);
    var statements = el("optgroup", { label: "Statements" });
    statements.appendChild(el("option", { value: "problem" }, STATEMENT_NAMES.problem));
    statements.appendChild(el("option", { value: "outcome" }, STATEMENT_NAMES.outcome));
    select.appendChild(statements);
    var known = knownTargets();
    var existing = el("optgroup", { label: "Existing objects" });
    OBJECT_GROUPS.forEach(function (g) {
      known[g[0]].forEach(function (id) {
        existing.appendChild(el("option", { value: id }, id + " (" + g[1] + ")"));
      });
    });
    if (existing.firstChild) select.appendChild(existing);
    // Novelty is only asserted when a CURRENT preview proves the id absent
    // from the document's automatic recognition; before any preview, or
    // after an edit, the id is merely the next one, and mapping it replaces
    // whatever recognition may already own under that id (the contract's
    // explicit override), which the label says instead of "New".
    var verified = previewCurrent();
    var fresh = el("optgroup", { label: verified ? "New object" : "Next id (no current preview)" });
    OBJECT_GROUPS.forEach(function (g) {
      var id = nextIdFor(g[0]);
      var label = verified
        ? "New " + g[1] + " (" + id + ")"
        : g[1] + " " + id + " (next id; no current preview — if recognition owns " + id + " this mapping replaces it)";
      fresh.appendChild(el("option", { value: NEW_TARGET + g[0] }, label));
    });
    select.appendChild(fresh);
    if (prev && select.querySelector('option[value="' + prev + '"]')) select.value = prev;
  }
  function refreshMapPickers() {
    var pickers = sourceList.querySelectorAll(".import-map-target");
    for (var i = 0; i < pickers.length; i++) fillMapPicker(pickers[i]);
  }
  // setMapNote states one source's last mapping outcome and keeps it on
  // the source so a re-render of the list (another file added or removed)
  // restores it.
  function setMapNote(src, note, refused, text) {
    src.mapNote = { refused: refused, text: text };
    note.setAttribute("data-refused", refused ? "true" : "false");
    note.textContent = text;
  }
  function targetName(target) {
    return isStatement(target) ? target + " (" + STATEMENT_NAMES[target] + ")" : target;
  }

  // -- invalidation -------------------------------------------------------------
  // Every edit lands here: the epoch moves, the confirmation is cleared and
  // disabled, the shown preview (if any) is marked stale, and the created
  // panel is retired.
  function invalidate() {
    state.epoch++;
    confirmEl.checked = false;
    confirmEl.disabled = true;
    applyBtn.disabled = true;
    if (state.created) {
      // A publication already made is kept on the page, marked stale
      // relative to the edited inputs — never hidden.
      createdEl.setAttribute("data-stale", "true");
    }
    if (state.applyAttempt && state.applyAttempt.status === "lost") {
      // The same request can no longer be retried as-is; the unknown
      // outcome is still named, never dropped.
      retryBtn.disabled = true;
      retryNote.textContent = lostNote(state.applyAttempt) +
        " The inputs changed since that attempt, so it cannot be retried as-is and its outcome is still unknown: preview again — creating under that name answers already-created if it was published, or target-exists.";
    }
    if (state.preview) {
      markStale("Not previewed since your last edit: the findings and statuses below are earlier results. Preview again to see the current state.");
    }
    // The pickers' "new" ids are unverified again until the next preview.
    refreshMapPickers();
    setNextAction();
  }
  // markStale turns the shown preview into an explicitly EARLIER result:
  // the current-sounding readiness line is replaced, every finding (in the
  // list and on the cards) is marked as an earlier result, and each card's
  // status is restated from the page's own choices as not yet previewed.
  // The confirmation is never re-enabled here.
  function markStale(text) {
    resultEl.setAttribute("data-stale", "true");
    staleNote.hidden = false;
    readyEl.setAttribute("data-stale", "true");
    readyEl.textContent = text;
    findingsHeading.textContent = "Earlier results (from the last preview)";
    var items = resultEl.querySelectorAll("#import-findings li[data-code], .import-field-findings li[data-code]");
    for (var i = 0; i < items.length; i++) {
      if (items[i].hasAttribute("data-earlier")) continue;
      items[i].setAttribute("data-earlier", "true");
      items[i].appendChild(el("span", { class: "import-finding-earlier" }, " (earlier result)"));
    }
    // The remaining-source summary describes the EARLIER preview's inputs
    // until the next preview; it is never a current or persisted fact.
    if (remainingEl.firstChild && !remainingEl.hasAttribute("data-earlier")) {
      remainingEl.setAttribute("data-earlier", "true");
      remainingEl.insertBefore(el("p", { class: "import-finding-earlier", "data-testid": "import-remaining-earlier" }, "Earlier preview result: preview again to see what the current inputs would keep."), remainingEl.firstChild);
    }
    refreshCardStatuses();
  }
  function previewCurrent() {
    return state.preview && state.preview.epoch === state.epoch;
  }
  function blockingCount(result) {
    var n = 0;
    for (var i = 0; i < (result.findings || []).length; i++) {
      if (result.findings[i].blocking) n++;
    }
    return n;
  }
  function specRefOf(attemptOrRequest) {
    try {
      var req = JSON.parse(attemptOrRequest.request);
      return "spec/" + req.target.slug;
    } catch (e) {
      return "the spec";
    }
  }
  function lostNote(attempt) {
    return "The response to the creation request for " + specRefOf(attempt) + " was lost after the request was sent" +
      (attempt.error ? " (" + attempt.error + ")" : "") + ", so its outcome is unknown: the server may already have created it.";
  }
  function setNextAction(text) {
    if (text) {
      nextAction.textContent = text;
      return;
    }
    var attempt = state.applyAttempt;
    if (attempt && attempt.status === "in-flight") {
      nextAction.textContent = "Creating… an edit now does not cancel the request; its outcome will be reported here.";
    } else if (attempt && attempt.status === "lost" && attempt.epoch === state.epoch) {
      nextAction.textContent = "The creation request's outcome is unknown. Retry the same request: an already-created answer means it was published; nothing is duplicated.";
    } else if (attempt && attempt.status === "lost") {
      nextAction.textContent = "An earlier creation request's outcome is unknown and the inputs changed since — preview again; a same-name creation answers already-created or target-exists.";
    } else if (state.created && state.created.epoch === state.epoch) {
      nextAction.textContent = "Created. Open the board to continue, or the source record to inspect the original copied content.";
    } else if (state.created) {
      var res = state.created.result;
      nextAction.textContent = "An earlier request created " + res.spec_ref + " on " + res.branch + "; your current edits are not part of it. To create another proposal, use a new name and preview again.";
    } else if (state.sources.length === 0) {
      nextAction.textContent = "Add at least one source file, then preview.";
    } else if (!state.preview) {
      nextAction.textContent = "Preview to check the mapping before anything is created.";
    } else if (!previewCurrent()) {
      nextAction.textContent = "Inputs changed since the last preview — preview again before creating.";
    } else if (!state.preview.result.ready) {
      var n = blockingCount(state.preview.result);
      nextAction.textContent = "Resolve the " + n + " blocking finding" + (n === 1 ? "" : "s") + " above, then preview again.";
    } else if (!confirmEl.checked) {
      nextAction.textContent = "Review the preview, then confirm it to create the proposal.";
    } else {
      nextAction.textContent = "Create the proposal from exactly this confirmed preview.";
    }
  }

  // -- sources ---------------------------------------------------------------------
  filesInput.addEventListener("change", function () {
    var files = Array.prototype.slice.call(filesInput.files || []);
    filesInput.value = "";
    if (!files.length) return;
    Promise.all(files.map(readFile)).then(function (read) {
      for (var i = 0; i < read.length; i++) {
        if (state.sources.length >= MAX_SOURCES) {
          showError({ code: "invalid-request", error: "at most " + MAX_SOURCES + " sources per import; the remaining files were not added" });
          break;
        }
        var src = { id: sourceIdFor(read[i].label), label: read[i].label, data: read[i].data, bytes: read[i].bytes, size: read[i].size, startLine: 0, endLine: 0 };
        state.sources.push(src);
        if (!state.primary) state.primary = src.id;
      }
      renderSources();
      invalidate();
    }, function (err) {
      showError({ code: "invalid-source", error: "could not read a chosen file: " + err.message });
    });
  });

  function renderSources() {
    clear(sourceList);
    state.sources.forEach(function (src) {
      var li = el("li", { "data-testid": "import-source-" + src.id, "data-source-id": src.id });
      li.appendChild(el("span", { class: "import-source-label" }, src.label));
      li.appendChild(document.createTextNode(" "));
      li.appendChild(el("span", { class: "import-source-size" }, src.size + " bytes, id " + src.id));
      var line = el("div", { class: "import-inline" });
      var primaryLabel = el("label");
      var radio = el("input", { type: "radio", name: "import-primary", value: src.id, "data-testid": "import-primary-" + src.id });
      radio.checked = state.primary === src.id;
      primaryLabel.appendChild(radio);
      primaryLabel.appendChild(document.createTextNode(" Primary"));
      line.appendChild(primaryLabel);
      var startLabel = el("label");
      startLabel.appendChild(document.createTextNode("Start line "));
      var start = el("input", { type: "number", min: "1", class: "import-source-start", "data-testid": "import-start-" + src.id });
      if (src.startLine) start.value = String(src.startLine);
      startLabel.appendChild(start);
      line.appendChild(startLabel);
      var endLabel = el("label");
      endLabel.appendChild(document.createTextNode("End line "));
      var end = el("input", { type: "number", min: "1", class: "import-source-end", "data-testid": "import-end-" + src.id });
      if (src.endLine) end.value = String(src.endLine);
      endLabel.appendChild(end);
      line.appendChild(endLabel);
      line.appendChild(el("button", { type: "button", class: "import-source-remove", "data-testid": "import-remove-" + src.id }, "Remove"));
      li.appendChild(line);
      li.appendChild(renderSourceView(src));
      sourceList.appendChild(li);
      renderSourceText(li, src);
    });
  }
  // renderSourceView builds one source's read-only text block (filled by
  // renderSourceText) and the controls that map a selection in it: the
  // target picker and the Map selection action, with a status line for
  // the outcome or the refusal. Large sources start collapsed.
  function renderSourceView(src) {
    var view = el("details", { class: "import-source-view", "data-testid": "import-source-view-" + src.id });
    if (src.size <= LARGE_SOURCE_BYTES) view.setAttribute("open", "");
    view.appendChild(el("summary", null, "Text of " + src.label + " — select a passage, pick where it belongs, then Map selection"));
    view.appendChild(el("p", { class: "import-hint import-source-view-note", "data-testid": "import-source-view-note-" + src.id }));
    view.appendChild(el("pre", { class: "import-source-text", "data-testid": "import-source-text-" + src.id, "data-source-id": src.id, tabindex: "0", "aria-label": "Text of " + src.label + ", read-only" }));
    var controls = el("div", { class: "import-map-controls", "data-source-id": src.id });
    var pickLabel = el("label");
    pickLabel.appendChild(document.createTextNode("Map the selection to "));
    var picker = el("select", { class: "import-map-target", "data-testid": "import-map-target-" + src.id });
    fillMapPicker(picker);
    pickLabel.appendChild(picker);
    controls.appendChild(pickLabel);
    controls.appendChild(el("button", { type: "button", class: "import-map-selection", "data-testid": "import-map-selection-" + src.id }, "Map selection"));
    view.appendChild(controls);
    var note = el("p", { class: "import-map-note", "data-testid": "import-map-note-" + src.id, role: "status", "aria-live": "polite", "data-refused": "false" });
    if (src.mapNote) {
      note.setAttribute("data-refused", src.mapNote.refused ? "true" : "false");
      note.textContent = src.mapNote.text;
    }
    view.appendChild(note);
    return view;
  }
  // renderSourceText fills one source's block with its selected slice
  // decoded as text (or says why it cannot be shown). Called on add and
  // whenever the source's line range changes.
  function renderSourceText(li, src) {
    var pre = li.querySelector(".import-source-text");
    var note = li.querySelector(".import-source-view-note");
    if (!pre || !note) return;
    var slice = selectedSlice(src);
    if (slice.error) {
      pre.hidden = true;
      pre.textContent = "";
      note.textContent = "No text to show: the line range is not valid (" + slice.error + "); the preview refuses it too.";
      return;
    }
    var text = decodeUTF8(slice.bytes);
    if (text === null) {
      pre.hidden = true;
      pre.textContent = "";
      note.textContent = "This source is not valid UTF-8, so its text cannot be shown or mapped; the importer refuses it as well.";
      return;
    }
    pre.hidden = false;
    pre.textContent = text;
    var ranged = src.startLine || src.endLine ? "Lines " + src.startLine + "–" + src.endLine + ": " : "";
    note.textContent = ranged + slice.bytes.length + " bytes shown exactly as read; a mapped range counts bytes from the start of this text.";
  }
  // mapSelection turns the live selection inside one source's text into a
  // source-backed identity mapping for the picked target (a new id follows
  // nextIdFor), replacing that target's earlier span or text but keeping
  // its evidence kinds; every outcome is stated in the source's note.
  function mapSelection(controls) {
    var li = controls.closest("li");
    var src = sourceById(controls.getAttribute("data-source-id"));
    var pre = li ? li.querySelector(".import-source-text") : null;
    var note = li ? li.querySelector(".import-map-note") : null;
    var picker = li ? li.querySelector(".import-map-target") : null;
    if (!src || !pre || !note || !picker) return;
    var sel = selectionIn(pre, src);
    if (sel.error) {
      setMapNote(src, note, true, sel.error);
      return;
    }
    var picked = picker.value;
    var created = picked.indexOf(NEW_TARGET) === 0;
    var target = created ? nextIdFor(picked.slice(NEW_TARGET.length)) : picked;
    // Novelty is known only while the preview is current (see
    // fillMapPicker); decided before this edit invalidates it.
    var verified = created && previewCurrent();
    var m = ensureMapping(target);
    var replaced = !!(m.sourceId || m.text);
    m.sourceId = src.id;
    m.start = sel.start;
    m.end = sel.end;
    m.transform = "identity";
    m.text = "";
    renderMappings();
    invalidate();
    refreshMapPickers();
    var novelty = "";
    if (created) {
      novelty = verified ? " (new)" : " (next id, unverified until preview; replaces any automatically recognized " + target + ")";
    }
    setMapNote(src, note, false, "Mapped " + (sel.end - sel.start) + " bytes [" + sel.start + "," + sel.end + ") of " + src.label + " to " + targetName(target) +
      novelty + (replaced ? ", replacing its earlier mapping" : "") + ". It is listed under Advanced with the text it covers; preview to check it.");
  }

  function syncSourceRow(li) {
    var id = li.getAttribute("data-source-id");
    for (var i = 0; i < state.sources.length; i++) {
      if (state.sources[i].id !== id) continue;
      var start = li.querySelector(".import-source-start");
      var end = li.querySelector(".import-source-end");
      state.sources[i].startLine = start && start.value ? parseInt(start.value, 10) || 0 : 0;
      state.sources[i].endLine = end && end.value ? parseInt(end.value, 10) || 0 : 0;
    }
  }
  function removeSource(id) {
    state.sources = state.sources.filter(function (s) {
      return s.id !== id;
    });
    if (state.primary === id) state.primary = state.sources.length ? state.sources[0].id : "";
    renderSources();
  }

  // -- explicit mappings ------------------------------------------------------
  function findMapping(target) {
    for (var i = 0; i < state.mappings.length; i++) {
      if (state.mappings[i].target === target) return state.mappings[i];
    }
    return null;
  }
  function newMapping(target) {
    return { target: target || "", sourceId: "", start: 0, end: 0, transform: "", text: "", evidence: [] };
  }
  function ensureMapping(target) {
    var m = findMapping(target);
    if (!m) {
      m = newMapping(target);
      state.mappings.push(m);
    }
    return m;
  }
  function dropEmptyMapping(target) {
    var m = findMapping(target);
    if (m && !m.sourceId && !m.text && m.evidence.length === 0) {
      state.mappings = state.mappings.filter(function (x) {
        return x !== m;
      });
    }
  }
  // setEvidence records the user's selection for one criterion: an
  // evidence-only mapping when nothing else is mapped for it, or the
  // evidence half of an existing explicit mapping.
  function setEvidence(target, kinds) {
    var m = ensureMapping(target);
    m.evidence = kinds.slice();
    dropEmptyMapping(target);
  }
  // setText records a user edit of one field's text: source-backed over
  // the field's own single span (origin user-edited-source when it
  // differs), else user-added text.
  function setText(field, text) {
    var m = ensureMapping(field.target);
    if (field.spans && field.spans.length === 1) {
      m.sourceId = field.spans[0].source_id;
      m.start = field.spans[0].start;
      m.end = field.spans[0].end;
      m.transform = field.spans[0].transform || "identity";
    }
    m.text = text;
  }
  // setWritten records a statement written on a placeholder card: a
  // user-added mapping (no source span) for a target no field resolved.
  function setWritten(target, text) {
    var m = ensureMapping(target);
    m.text = text;
    dropEmptyMapping(target);
  }

  function renderMappings() {
    clear(mappingList);
    state.mappings.forEach(function (m, index) {
      var li = el("li", { "data-testid": "import-mapping-" + index, "data-mapping-index": String(index), "data-mapping-target": m.target });
      var head = el("div", { class: "import-inline" });
      var targetLabel = el("label");
      targetLabel.appendChild(document.createTextNode("Target "));
      var target = el("input", { class: "import-mapping-target", "data-testid": "import-mapping-target-" + index, spellcheck: "false", placeholder: "problem, outcome, ac-1, co-1, dc-1 or oq-1" });
      target.value = m.target;
      targetLabel.appendChild(target);
      head.appendChild(targetLabel);
      var sourceLabel = el("label");
      sourceLabel.appendChild(document.createTextNode("Source "));
      var source = el("select", { class: "import-mapping-source" });
      source.appendChild(el("option", { value: "" }, "(none — your text only)"));
      state.sources.forEach(function (s) {
        source.appendChild(el("option", { value: s.id }, s.label + " (" + s.id + ")"));
      });
      source.value = m.sourceId;
      sourceLabel.appendChild(source);
      head.appendChild(sourceLabel);
      var startLabel = el("label");
      startLabel.appendChild(document.createTextNode("Start byte "));
      var start = el("input", { type: "number", min: "0", class: "import-mapping-start" });
      start.value = m.sourceId ? String(m.start) : "";
      startLabel.appendChild(start);
      head.appendChild(startLabel);
      var endLabel = el("label");
      endLabel.appendChild(document.createTextNode("End byte "));
      var end = el("input", { type: "number", min: "0", class: "import-mapping-end" });
      end.value = m.sourceId ? String(m.end) : "";
      endLabel.appendChild(end);
      head.appendChild(endLabel);
      var transformLabel = el("label");
      transformLabel.appendChild(document.createTextNode("Transform "));
      var transform = el("select", { class: "import-mapping-transform" });
      transform.appendChild(el("option", { value: "" }, "(none)"));
      TRANSFORMS.forEach(function (t) {
        transform.appendChild(el("option", { value: t }, t));
      });
      transform.value = m.transform;
      transformLabel.appendChild(transform);
      head.appendChild(transformLabel);
      head.appendChild(el("button", { type: "button", class: "import-mapping-remove", "data-testid": "import-mapping-remove-" + index }, "Remove"));
      li.appendChild(head);
      li.appendChild(el("p", { class: "import-mapping-excerpt", "data-testid": "import-mapping-excerpt-" + index }));
      renderExcerpt(li, m);
      var textLabel = el("label");
      textLabel.appendChild(document.createTextNode("Text (your wording; with a source range it marks the field user-edited-source)"));
      var text = el("textarea", { class: "import-mapping-text", "data-testid": "import-mapping-text-" + index });
      text.value = m.text;
      textLabel.appendChild(text);
      li.appendChild(textLabel);
      var ev = el("fieldset", { class: "import-evidence import-mapping-evidence" });
      ev.appendChild(el("legend", null, "Evidence kinds (acceptance criteria only)"));
      EVIDENCE_KINDS.forEach(function (kind) {
        var lab = el("label");
        var box = el("input", { type: "checkbox", "data-kind": kind });
        box.checked = m.evidence.indexOf(kind) >= 0;
        lab.appendChild(box);
        lab.appendChild(document.createTextNode(" " + kindLabel(kind)));
        ev.appendChild(lab);
      });
      li.appendChild(ev);
      mappingList.appendChild(li);
    });
    syncAdvancedSummary();
    refreshMapPickers();
  }
  // renderExcerpt shows, beside a source-backed mapping's byte range, the
  // first line of the text those bytes cover (from the source's own bytes,
  // never from the server), so a reader can verify the range — or says
  // why the range covers nothing showable. Hidden for text-only mappings.
  function renderExcerpt(li, m) {
    var p = li.querySelector(".import-mapping-excerpt");
    if (!p) return;
    clear(p);
    if (!m.sourceId) {
      p.hidden = true;
      return;
    }
    p.hidden = false;
    var src = sourceById(m.sourceId);
    var range = "Bytes [" + m.start + "," + m.end + ") of " + labelOf(m.sourceId) + ": ";
    if (!src) {
      p.textContent = range + "that source is no longer selected.";
      return;
    }
    var slice = selectedSlice(src);
    if (slice.error) {
      p.textContent = range + "the source's line range is not valid, so the covered text cannot be shown.";
      return;
    }
    var b = slice.bytes;
    if (m.start < 0 || m.end > b.length || m.end <= m.start) {
      p.textContent = range + "not a range within its " + b.length + " selected bytes; the preview refuses it.";
      return;
    }
    if (!runeBoundary(b, m.start) || !runeBoundary(b, m.end)) {
      p.textContent = range + "the range cuts through a character; the preview refuses it.";
      return;
    }
    var text = decodeUTF8(b.subarray(m.start, m.end));
    if (text === null) {
      p.textContent = range + "not valid UTF-8.";
      return;
    }
    var nl = text.indexOf("\n");
    var first = nl >= 0 ? text.slice(0, nl) : text;
    var more = nl >= 0 || first.length > EXCERPT_CHARS;
    if (first.length > EXCERPT_CHARS) first = first.slice(0, EXCERPT_CHARS);
    p.appendChild(document.createTextNode(range));
    p.appendChild(el("q", null, first));
    if (more) p.appendChild(document.createTextNode(" … (first line shown)"));
  }
  function refreshExcerpts() {
    var rows = mappingList.querySelectorAll("li[data-mapping-index]");
    for (var i = 0; i < rows.length; i++) {
      var m = state.mappings[parseInt(rows[i].getAttribute("data-mapping-index"), 10)];
      if (m) renderExcerpt(rows[i], m);
    }
  }
  function syncMappingRow(li) {
    var index = parseInt(li.getAttribute("data-mapping-index"), 10);
    var m = state.mappings[index];
    if (!m) return;
    m.target = li.querySelector(".import-mapping-target").value.trim();
    li.setAttribute("data-mapping-target", m.target);
    m.sourceId = li.querySelector(".import-mapping-source").value;
    var start = li.querySelector(".import-mapping-start").value;
    var end = li.querySelector(".import-mapping-end").value;
    m.start = start === "" ? 0 : parseInt(start, 10) || 0;
    m.end = end === "" ? 0 : parseInt(end, 10) || 0;
    m.transform = li.querySelector(".import-mapping-transform").value;
    m.text = li.querySelector(".import-mapping-text").value;
    m.evidence = [];
    var boxes = li.querySelectorAll(".import-mapping-evidence input[data-kind]");
    for (var i = 0; i < boxes.length; i++) {
      if (boxes[i].checked) m.evidence.push(boxes[i].getAttribute("data-kind"));
    }
    renderExcerpt(li, m);
    refreshMapPickers();
  }

  // -- declared links ---------------------------------------------------------
  function renderLinks() {
    clear(linkList);
    state.links.forEach(function (l, index) {
      var li = el("li", { "data-testid": "import-link-" + index, "data-link-index": String(index) });
      var typeLabel = el("label");
      typeLabel.appendChild(document.createTextNode("Type "));
      var type = el("select", { class: "import-link-type" });
      LINK_TYPES.forEach(function (t) {
        type.appendChild(el("option", { value: t }, t));
      });
      type.value = l.type;
      typeLabel.appendChild(type);
      li.appendChild(typeLabel);
      li.appendChild(document.createTextNode(" "));
      var refLabel = el("label");
      refLabel.appendChild(document.createTextNode("Ref "));
      var ref = el("input", { class: "import-link-ref", spellcheck: "false", placeholder: "spec/<name>#ac-1" });
      ref.value = l.ref;
      refLabel.appendChild(ref);
      li.appendChild(refLabel);
      li.appendChild(document.createTextNode(" "));
      li.appendChild(el("button", { type: "button", class: "import-link-remove" }, "Remove"));
      linkList.appendChild(li);
    });
    syncAdvancedSummary();
  }
  function syncLinkRow(li) {
    var index = parseInt(li.getAttribute("data-link-index"), 10);
    var l = state.links[index];
    if (!l) return;
    l.type = li.querySelector(".import-link-type").value;
    l.ref = li.querySelector(".import-link-ref").value.trim();
  }

  // -- form events: every edit invalidates --------------------------------------
  function onFormEdit(e) {
    var t = e.target;
    if (!t || !t.closest) return;
    if (t === filesInput) return; // handled by the change reader above
    // The selection picker chooses where a NEXT mapping goes; it is not an
    // edit of the request and never invalidates the preview.
    if (t.closest(".import-map-controls")) return;
    if (t === classSelect) syncClassNotes();
    var sourceRow = t.closest("#import-source-list li");
    if (sourceRow) {
      if (t.name === "import-primary") state.primary = t.value;
      syncSourceRow(sourceRow);
      var edited = sourceById(sourceRow.getAttribute("data-source-id"));
      if (edited && (t.classList.contains("import-source-start") || t.classList.contains("import-source-end"))) {
        renderSourceText(sourceRow, edited);
        refreshExcerpts();
      }
    }
    var mappingRow = t.closest("#import-mapping-list li");
    if (mappingRow) syncMappingRow(mappingRow);
    var linkRow = t.closest("#import-link-list li");
    if (linkRow) syncLinkRow(linkRow);
    invalidate();
  }
  form.addEventListener("input", onFormEdit);
  form.addEventListener("change", onFormEdit);
  form.addEventListener("submit", function (e) {
    e.preventDefault();
  });
  // Pressing Map selection must not disturb the very selection it maps:
  // the button takes no focus on mousedown (keyboard activation is
  // unaffected).
  form.addEventListener("mousedown", function (e) {
    var t = e.target;
    if (t && t.closest && t.closest(".import-map-selection")) e.preventDefault();
  });
  form.addEventListener("click", function (e) {
    var t = e.target;
    if (!t || !t.closest) return;
    var opener = t.closest(".import-open-advanced");
    if (opener) {
      openAdvanced(opener.getAttribute("data-focus"));
      return; // the anchor's own navigation to #import-advanced proceeds
    }
    var mapBtn = t.closest(".import-map-selection");
    if (mapBtn) {
      mapSelection(mapBtn.closest(".import-map-controls"));
      return;
    }
    var removeSrc = t.closest(".import-source-remove");
    if (removeSrc) {
      removeSource(removeSrc.closest("li").getAttribute("data-source-id"));
      invalidate();
      return;
    }
    if (t.closest("#import-add-mapping")) {
      state.mappings.push(newMapping(""));
      renderMappings();
      invalidate();
      return;
    }
    var removeMap = t.closest(".import-mapping-remove");
    if (removeMap) {
      var mi = parseInt(removeMap.closest("li").getAttribute("data-mapping-index"), 10);
      state.mappings.splice(mi, 1);
      renderMappings();
      invalidate();
      return;
    }
    if (t.closest("#import-add-link")) {
      state.links.push({ type: LINK_TYPES[0], ref: "" });
      renderLinks();
      invalidate();
      return;
    }
    var removeLink = t.closest(".import-link-remove");
    if (removeLink) {
      var li2 = parseInt(removeLink.closest("li").getAttribute("data-link-index"), 10);
      state.links.splice(li2, 1);
      renderLinks();
      invalidate();
      return;
    }
    if (t.closest("#import-preview-btn")) {
      doPreview();
    }
  });

  // -- request composition ------------------------------------------------------
  function mappingWire(m) {
    var w = { target: m.target };
    if (m.sourceId) {
      w.source_id = m.sourceId;
      w.start = m.start;
      w.end = m.end;
      w.transform = m.transform || "identity";
    }
    if (m.text !== "" && m.text !== null && m.text !== undefined) w.text = m.text;
    if (m.evidence && m.evidence.length) w.evidence = m.evidence.slice();
    return w;
  }
  function buildRequest() {
    var req = {
      schema: "verdi.spec-import-request/v1",
      target: { slug: $("import-slug").value.trim(), class: classSelect.value, title: $("import-title").value.trim() },
      format: formatSelect.value,
      primary: state.primary,
      sources: state.sources.map(function (s) {
        var o = { id: s.id, label: s.label, data: s.data };
        if (s.startLine) o.start_line = s.startLine;
        if (s.endLine) o.end_line = s.endLine;
        return o;
      }),
      defer_statements: deferEl.checked,
      retain_unmapped: retainEl.checked,
    };
    var story = $("import-story").value.trim();
    if (story) req.target.story = story;
    var mappings = state.mappings
      .filter(function (m) {
        return m.target !== "";
      })
      .map(mappingWire);
    if (mappings.length) req.mappings = mappings;
    var links = state.links.filter(function (l) {
      return l.ref !== "";
    });
    if (links.length) req.links = links;
    return req;
  }

  // -- errors -------------------------------------------------------------------
  function hideError() {
    errorEl.hidden = true;
    errorEl.textContent = "";
    errorEl.removeAttribute("data-code");
  }
  // slugOfRequest reads the target slug out of the exact request JSON a
  // refusal answers — never the form's current value, which may have
  // changed since that request was sent.
  function slugOfRequest(request) {
    try {
      var req = JSON.parse(request);
      return (req && req.target && typeof req.target.slug === "string") ? req.target.slug : "";
    } catch (e) {
      return "";
    }
  }
  // showError renders one refusal. request is the exact request JSON the
  // refusal answers (the previewed or applied bytes); late marks an answer
  // to a creation request sent before the current inputs. Everything the
  // refusal names — the spec, the existing-board link — comes from that
  // request, so a refusal about spec/A never points at a slug edited since.
  function showError(failure, request, late) {
    var code = (failure && failure.code) || "transport";
    var detail = (failure && (failure.error || failure.message)) || "no detail";
    var slug = request ? slugOfRequest(request) : "";
    var about = slug ? "spec/" + slug : "";
    errorEl.setAttribute("data-code", code);
    if (about) {
      errorEl.setAttribute("data-request-spec", about);
    } else {
      errorEl.removeAttribute("data-request-spec");
    }
    var lead = late && about ? "This answers the earlier creation request for " + about + ", sent before your latest edits. " : "";
    errorEl.textContent = lead + "Refused (" + code + ")" + (about && !late ? " for " + about : "") + ": " + detail + (ERROR_GUIDANCE[code] ? " — " + ERROR_GUIDANCE[code] : "");
    if (code === "target-exists" && slug) {
      // The existing proposal under the REFUSED request's name is one
      // click away — the discoverable path when an earlier attempt on this
      // page may have been the one that created it.
      errorEl.appendChild(document.createTextNode(" "));
      errorEl.appendChild(el("a", { href: "/b/" + encodeURIComponent("design/" + slug) + "/board/spec/" + encodeURIComponent(slug), "data-testid": "import-existing-board-link" }, "Open the existing board for " + about + " (the name in the refused request)"));
    }
    errorEl.hidden = false;
  }
  function parseJSON(text) {
    try {
      return JSON.parse(text);
    } catch (e) {
      return { code: "transport", error: "unreadable response: " + text.slice(0, 200) };
    }
  }
  function post(path, body, headers) {
    var h = { "Content-Type": "application/json" };
    for (var k in headers || {}) {
      if (Object.prototype.hasOwnProperty.call(headers, k)) h[k] = headers[k];
    }
    return fetch(path, { method: "POST", headers: h, body: body, credentials: "same-origin" }).then(function (resp) {
      return resp.text().then(function (text) {
        return { status: resp.status, data: parseJSON(text) };
      });
    });
  }

  // -- preview ------------------------------------------------------------------
  function doPreview() {
    var epoch = state.epoch;
    var seq = ++state.previewSeq;
    var body = JSON.stringify(buildRequest());
    hideError();
    setNextAction("Previewing…");
    post("/design/import/preview", body)
      .then(function (r) {
        // Discard a response for an older request or an older epoch: an
        // edit after this request was issued must never be answered by it.
        if (seq !== state.previewSeq || epoch !== state.epoch) {
          setNextAction();
          return;
        }
        if (r.status !== 200) {
          showError(r.data, body, false);
          setNextAction();
          return;
        }
        state.preview = { result: r.data, request: body, epoch: epoch };
        renderPreview(r.data);
        setNextAction();
      })
      .catch(function (err) {
        if (epoch !== state.epoch) return;
        showError({ code: "transport", error: err.message }, body, false);
        setNextAction();
      });
  }

  // readinessSummary states the server's readiness in plain words: the
  // counts come from the finding CODES the server returned, never from a
  // judgment of the page's own.
  function readinessSummary(result) {
    if (result.ready) return "Ready: no blocking findings. Confirm below to create exactly this proposal.";
    var counts = {};
    var blocking = 0;
    (result.findings || []).forEach(function (f) {
      if (!f.blocking) return;
      blocking++;
      counts[f.code] = (counts[f.code] || 0) + 1;
    });
    var parts = [];
    var named = 0;
    if (counts["missing-evidence"]) {
      parts.push(plural(counts["missing-evidence"], "acceptance criterion needs", "acceptance criteria need") + " evidence kinds");
      named += counts["missing-evidence"];
    }
    if (counts["missing-statement"]) {
      parts.push(plural(counts["missing-statement"], "statement is", "statements are") + " missing");
      named += counts["missing-statement"];
    }
    if (counts["unresolved-coverage"]) {
      parts.push(plural(counts["unresolved-coverage"], "source has", "sources have") + " leftover text with no choice yet");
      named += counts["unresolved-coverage"];
    }
    if (blocking - named > 0) parts.push(plural(blocking - named, "other blocking finding", "other blocking findings"));
    return "Not ready: " + parts.join("; ") + ". Correct them below, then preview again.";
  }
  function findingTitle(f) {
    if (f.code === "unsupported-structure" && f.target === "primary") return "No heading structure found in the primary";
    return FINDING_TITLES[f.code] || f.code;
  }
  function guidanceFor(f) {
    if (f.code === "unsupported-structure") return f.target === "primary" ? GUIDANCE_PRIMARY_STRUCTURE : GUIDANCE_STRUCTURE_NEUTRAL;
    return GUIDANCE[f.code] || "";
  }
  function groupFindings(result) {
    var byTarget = {};
    (result.findings || []).forEach(function (f) {
      if (!f.target) return;
      if (!byTarget[f.target]) byTarget[f.target] = [];
      byTarget[f.target].push(f);
    });
    return byTarget;
  }
  // cardIdFor names the card (or statement placeholder) a finding's target
  // concerns, once the cards are rendered; "" for targets that are not a
  // field (template, target.class, a path, nothing).
  function cardIdFor(target) {
    if (!target) return "";
    if ($("import-field-" + target)) return "import-field-" + target;
    if ($("import-missing-" + target)) return "import-missing-" + target;
    return "";
  }

  function renderPreview(result) {
    resultEl.hidden = false;
    resultEl.setAttribute("data-stale", "false");
    resultEl.setAttribute("data-ready", String(!!result.ready));
    staleNote.hidden = true;
    readyEl.setAttribute("data-ready", String(!!result.ready));
    readyEl.setAttribute("data-stale", "false");
    readyEl.textContent = readinessSummary(result);
    digestEl.textContent = result.digest || "";

    var editable = isEditable();
    var findingsByTarget = groupFindings(result);
    guideEl.hidden = !editable;
    renderStatements(result, editable, findingsByTarget);
    renderFields(result, editable, findingsByTarget);
    renderFindings(result);
    renderRemaining(result, editable, primaryOfRequest(state.preview ? state.preview.request : ""));

    clear(coverageBody);
    (result.coverage || []).forEach(function (c) {
      var tr = el("tr", {
        "data-testid": "import-coverage-" + c.source_id,
        "data-total": String(c.total_bytes),
        "data-mapped": String(c.mapped_bytes),
        "data-retained": String(c.retained_bytes),
        "data-unresolved": String(c.unresolved_bytes),
      });
      tr.appendChild(el("td", null, labelOf(c.source_id) + " (" + c.source_id + ")"));
      tr.appendChild(el("td", null, c.total_bytes));
      tr.appendChild(el("td", null, c.mapped_bytes));
      tr.appendChild(el("td", null, c.retained_bytes));
      tr.appendChild(el("td", null, c.unresolved_bytes));
      coverageBody.appendChild(tr);
    });

    clear(sourcesEl);
    (result.sources || []).forEach(function (s) {
      var li = el("li", { "data-testid": "import-source-summary-" + s.id });
      var lines = s.start_line || s.end_line ? "lines " + s.start_line + "–" + s.end_line : "whole file";
      li.appendChild(el("span", { class: "import-source-label" }, s.label));
      li.appendChild(document.createTextNode(" (" + s.id + ", " + lines + ") selected digest "));
      li.appendChild(el("code", null, s.digest));
      sourcesEl.appendChild(li);
    });

    confirmEl.checked = false;
    confirmEl.disabled = !result.ready;
    applyBtn.disabled = true;
    // The preview's fields are now known object ids for the pickers.
    refreshMapPickers();
  }

  // renderFindings lists every finding the server returned: a plain title,
  // the code, the target, the verbatim message, the next action and a jump
  // to the card it concerns — or, for a finding that names no card, into
  // the advanced controls that can change the candidate.
  function renderFindings(result) {
    clear(findingsEl);
    findingsHeading.textContent = result.ready ? "Findings (none blocking)" : "What still blocks creation";
    (result.findings || []).forEach(function (f) {
      var li = el("li", { "data-code": f.code, "data-target": f.target || "", "data-blocking": String(!!f.blocking) });
      li.appendChild(el("strong", { class: "import-finding-title" }, findingTitle(f) + (f.blocking ? " (blocking)" : " (disclosure)")));
      li.appendChild(el("code", { class: "import-finding-code" }, f.code));
      li.appendChild(document.createTextNode(" "));
      if (f.target) li.appendChild(el("span", { class: "import-finding-target" }, f.target + ": "));
      li.appendChild(el("span", { class: "import-finding-message" }, f.message));
      var next = guidanceFor(f);
      if (next) li.appendChild(el("span", { class: "import-finding-next" }, "Next: " + next));
      var jumps = el("span", { class: "import-finding-jumps" });
      var cardId = cardIdFor(f.target);
      if (cardId) {
        jumps.appendChild(el("a", { class: "import-finding-jump", href: "#" + cardId }, "Go to " + f.target));
      } else if (f.code === "unresolved-coverage") {
        jumps.appendChild(el("a", { class: "import-finding-jump", href: "#import-remaining" }, "Go to the rest of the source"));
      } else if (f.blocking) {
        jumps.appendChild(el("a", { class: "import-finding-jump import-open-advanced", href: "#import-advanced", "data-focus": "import-add-mapping" }, "Open Advanced (mappings and links)"));
      }
      if (jumps.firstChild) li.appendChild(jumps);
      findingsEl.appendChild(li);
    });
    if (!(result.findings || []).length) findingsEl.appendChild(el("li", { class: "empty" }, "None."));
  }

  // renderStatements: the Problem and Outcome cards when resolved, else a
  // placeholder per missing statement carrying the direct actions. Native
  // has neither (the candidate is the file).
  function renderStatements(result, editable, findingsByTarget) {
    clear(statementsEl);
    if (!editable) return;
    var byTarget = {};
    (result.fields || []).forEach(function (f) {
      byTarget[f.target] = f;
    });
    ["problem", "outcome"].forEach(function (t) {
      if (byTarget[t]) statementsEl.appendChild(renderField(byTarget[t], editable, findingsByTarget[t] || [], 0));
      else statementsEl.appendChild(renderMissingStatement(t, findingsByTarget[t] || []));
    });
  }
  function renderMissingStatement(target, fs) {
    var name = STATEMENT_NAMES[target] || target;
    var box = el("section", { class: "import-field import-field-missing", id: "import-missing-" + target, "data-testid": "import-missing-" + target, "data-target": target });
    box.appendChild(el("h5", null, name));
    box.appendChild(el("p", { class: "import-field-source" }, "No " + name + " was resolved from the source. Write one here, or leave both statements as TODOs for now; the importer never invents one."));
    appendMirroredFindings(box, target, fs);
    var actions = el("div", { class: "import-field-actions" });
    actions.appendChild(el("button", { type: "button", class: "import-write", "data-write-target": target, "data-testid": "import-write-" + target }, "Write it"));
    actions.appendChild(el("button", { type: "button", class: "import-defer-action", "data-testid": "import-defer-action-" + target }, "Leave both statements as TODOs for now"));
    box.appendChild(actions);
    var existing = findMapping(target);
    var ta = el("textarea", { class: "import-write-text", "data-write-target": target, "data-testid": "import-write-text-" + target, "aria-label": "Your " + name });
    ta.value = existing && existing.text ? existing.text : "";
    ta.hidden = !(existing && existing.text);
    box.appendChild(ta);
    box.appendChild(el("p", { class: "import-hint" }, "Written text is recorded as your wording (user-added), never as source text. The TODO choice is the same as in step 4: it replaces both statements, one written here included."));
    return box;
  }

  // renderFields: the object cards grouped by kind under an origin-neutral
  // count (each card names its own source), or the native/empty notes.
  function renderFields(result, editable, findingsByTarget) {
    clear(fieldsEl);
    if (!editable) {
      fieldsEl.appendChild(el("p", { class: "empty" }, "Native import: the candidate is the primary source byte for byte, so there are no per-field mappings to show or edit."));
      return;
    }
    var objects = (result.fields || []).filter(function (f) {
      return !isStatement(f.target);
    });
    if (!objects.length) {
      var none = el("p", { class: "empty" });
      none.appendChild(document.createTextNode("No acceptance criteria or other objects were resolved from the source. Add them as manual field mappings in Advanced; each acceptance criterion also needs its evidence kinds. "));
      none.appendChild(el("a", { class: "import-open-advanced", href: "#import-advanced", "data-focus": "import-add-mapping" }, "Open Advanced"));
      fieldsEl.appendChild(none);
      return;
    }
    var placed = {};
    OBJECT_GROUPS.forEach(function (g) {
      var members = objects.filter(function (f) {
        return f.target.indexOf(g[0]) === 0;
      });
      if (!members.length) return;
      var testid = g[0] === "ac-" ? "import-criteria-count" : "import-group-count-" + g[0].replace("-", "");
      fieldsEl.appendChild(el("h4", { class: "import-group-heading", "data-testid": testid }, plural(members.length, g[1], g[2])));
      members.forEach(function (f) {
        placed[f.target] = true;
        fieldsEl.appendChild(renderField(f, editable, findingsByTarget[f.target] || [], members.length));
      });
    });
    objects.forEach(function (f) {
      if (!placed[f.target]) fieldsEl.appendChild(renderField(f, editable, findingsByTarget[f.target] || [], 0));
    });
  }

  // originSentence states one field's exact provenance in plain words:
  // copied from a named source, the user's wording replacing a named
  // source's text, the user's wording with no source text, or a generated
  // placeholder. Never "from the primary" by assumption.
  function originSentence(f) {
    var labels = [];
    (f.spans || []).forEach(function (sp) {
      var l = labelOf(sp.source_id);
      if (labels.indexOf(l) < 0) labels.push(l);
    });
    var from = labels.length ? labels.join(", ") : "the source";
    switch (f.origin) {
      case "copied-source":
        return "Copied from " + from + ".";
      case "user-edited-source":
        return "Your wording, replacing text copied from " + from + " (the original selection stays in the record).";
      case "user-added":
        return "Your wording; no source text was selected for it.";
      case "generated-deferral":
        return "Generated TODO placeholder, not source text; it stands in until the real statement is written on the board.";
      default:
        return "Origin: " + f.origin + ".";
    }
  }
  function spansSentence(f) {
    if (f.origin === "generated-deferral") return "Generated TODO placeholder — visibly incomplete, not source text.";
    if (f.spans && f.spans.length) {
      return f.spans
        .map(function (sp) {
          return "from " + labelOf(sp.source_id) + " bytes [" + sp.start + "," + sp.end + ")" + (sp.transform ? " transform " + sp.transform : "");
        })
        .join("; ");
    }
    return "User-supplied text; no source span.";
  }
  function appendMirroredFindings(node, target, fs) {
    if (!fs || !fs.length) return;
    var ul = el("ul", { class: "import-field-findings", "data-testid": "import-field-findings-" + target });
    fs.forEach(function (f) {
      var li = el("li", { "data-code": f.code, "data-blocking": String(!!f.blocking) });
      li.appendChild(el("strong", null, findingTitle(f) + (f.blocking ? " (blocking): " : " (disclosure): ")));
      li.appendChild(document.createTextNode(f.message));
      ul.appendChild(li);
    });
    node.appendChild(ul);
  }

  function renderField(f, editable, fs, acCount) {
    var art = el("article", { class: "import-field", id: "import-field-" + f.target, "data-testid": "import-field-" + f.target, "data-target": f.target, "data-origin": f.origin });
    art.appendChild(el("h5", null, STATEMENT_NAMES[f.target] || f.target));
    var source = el("p", { class: "import-field-source", "data-testid": "import-field-source-" + f.target });
    source.appendChild(document.createTextNode(originSentence(f)));
    source.appendChild(el("code", { class: "import-origin", "data-testid": "import-field-origin-" + f.target }, f.origin));
    art.appendChild(source);
    art.appendChild(el("pre", { class: "import-field-text", "data-testid": "import-field-text-" + f.target }, f.text));

    if (isAC(f.target)) {
      var status = el("p", { class: "import-field-status", "data-testid": "import-field-status-" + f.target });
      art.appendChild(status);
      var ev = el("fieldset", { class: "import-evidence" });
      ev.appendChild(el("legend", null, "Evidence kinds for " + f.target + " (your choice)"));
      EVIDENCE_KINDS.forEach(function (kind) {
        var lab = el("label");
        var box = el("input", { type: "checkbox", "data-evidence-target": f.target, "data-kind": kind, "data-testid": "import-evidence-" + f.target + "-" + kind });
        box.checked = (f.evidence || []).indexOf(kind) >= 0;
        lab.appendChild(box);
        lab.appendChild(document.createTextNode(" " + kindLabel(kind)));
        ev.appendChild(lab);
      });
      if (acCount > 1) {
        ev.appendChild(el("button", { type: "button", class: "import-evidence-all", "data-evidence-target": f.target, "data-testid": "import-evidence-all-" + f.target },
          "Apply " + f.target + "'s kinds to all " + plural(acCount, "acceptance criterion", "acceptance criteria")));
      }
      art.appendChild(ev);
    }
    appendMirroredFindings(art, f.target, fs);
    if (editable) {
      art.appendChild(el("button", { type: "button", class: "import-edit", "data-edit-target": f.target, "data-testid": "import-edit-" + f.target }, "Edit text"));
      var ta = el("textarea", { class: "import-edit-text", "data-edit-target": f.target, "data-testid": "import-edit-text-" + f.target, "aria-label": "Edited text for " + f.target });
      ta.value = f.text;
      ta.hidden = true;
      art.appendChild(ta);
    }
    var tech = el("details", { class: "import-tech" });
    tech.appendChild(el("summary", null, "Exact source bytes"));
    tech.appendChild(el("p", { class: "import-field-spans", "data-testid": "import-field-spans-" + f.target }, spansSentence(f)));
    art.appendChild(tech);
    art._field = f;
    if (isAC(f.target)) renderStatus(art);
    return art;
  }

  // renderStatus restates one criterion card's evidence status: the kinds
  // the last preview validated while it is current; after any edit, the
  // page's own choice for that card, marked as changed (not yet previewed)
  // when it differs from what the server last saw, else as an earlier
  // result. The server's readiness is never inferred from this.
  function renderStatus(art) {
    var f = art._field;
    var node = art.querySelector(".import-field-status");
    if (!f || !node) return;
    var server = f.evidence || [];
    var m = findMapping(f.target);
    var local = m ? m.evidence || [] : [];
    var fresh = previewCurrent();
    var chosen = fresh ? server : local;
    var text = chosen.length ? "Evidence kinds chosen: " + chosen.map(kindLabel).join(", ") : "Evidence kinds: none chosen";
    var status = "previewed";
    if (!fresh) {
      if (!sameKinds(local, server)) {
        status = "changed";
        text += " (changed since the last preview; preview again to check it)";
      } else {
        status = "earlier";
        text += " (from the last preview)";
      }
    }
    node.textContent = text;
    node.setAttribute("data-status", status);
  }
  function refreshCardStatuses() {
    var arts = fieldsEl.querySelectorAll("article[data-target]");
    for (var i = 0; i < arts.length; i++) {
      if (arts[i]._field && isAC(arts[i]._field.target)) renderStatus(arts[i]);
    }
  }

  // primaryOfRequest reads the primary source id out of the exact request
  // JSON a preview answered — the previewed identity, never the form's
  // current radio, which may have changed since.
  function primaryOfRequest(request) {
    try {
      var req = JSON.parse(request);
      return req && typeof req.primary === "string" ? req.primary : "";
    } catch (e) {
      return "";
    }
  }

  // renderRemaining: one plain, prospective sentence per source about what
  // the preview would do with the bytes that did not become fields, and —
  // while any have no choice yet — the explicit keep action, which ticks
  // the form's own checkbox. In native mode only the previewed request's
  // primary becomes the candidate; every other source is an ordinary
  // support (unresolved until kept, then reference material), never spec
  // content. Nothing here is a persisted fact: no import record exists
  // until creation.
  function renderRemaining(result, editable, primary) {
    clear(remainingEl);
    remainingEl.removeAttribute("data-earlier");
    var anyUnresolved = false;
    (result.coverage || []).forEach(function (c) {
      var label = labelOf(c.source_id);
      var text;
      var stateName;
      if (!editable && c.source_id === primary) {
        stateName = "candidate";
        text = label + " (the primary): the whole selection (" + c.total_bytes + " bytes) becomes the candidate spec itself, byte for byte.";
      } else if (c.unresolved_bytes > 0) {
        anyUnresolved = true;
        stateName = "unresolved";
        text = label + ": " + c.unresolved_bytes + " bytes did not become a field and have no choice yet; this blocks creation." + (c.mapped_bytes ? " " + c.mapped_bytes + " bytes became fields." : "");
      } else if (c.mapped_bytes === 0) {
        stateName = "kept";
        text = label + ": will be kept whole with the import as reference material (" + c.retained_bytes + " bytes); no field is taken from it and it does not become spec content.";
      } else if (c.retained_bytes > 0) {
        stateName = "kept";
        text = label + ": " + c.mapped_bytes + " bytes became fields; " + c.retained_bytes + " bytes will be kept with the import as reference material, not as spec fields.";
      } else {
        stateName = "mapped";
        text = label + ": every selected byte became a field.";
      }
      remainingEl.appendChild(el("p", { "data-testid": "import-remaining-" + c.source_id, "data-state": stateName }, text));
    });
    if (anyUnresolved) {
      var actions = el("div", { class: "import-field-actions" });
      actions.appendChild(el("button", { type: "button", class: "import-keep-remaining", id: "import-keep-remaining", "data-testid": "import-keep-remaining" }, "Keep the remaining source text as reference material"));
      actions.appendChild(el("span", { class: "import-hint" }, "The same choice as in step 4: the text will be kept with the import as reference material, never promoted into fields, never a substitute for a missing or ambiguous field."));
      remainingEl.appendChild(actions);
    }
    if (!(result.coverage || []).length) remainingEl.appendChild(el("p", { class: "empty" }, "No source coverage was reported."));
  }

  function checkedKinds(target) {
    var kinds = [];
    var boxes = fieldsEl.querySelectorAll('input[data-evidence-target="' + target + '"][data-kind]');
    for (var i = 0; i < boxes.length; i++) {
      if (boxes[i].checked) kinds.push(boxes[i].getAttribute("data-kind"));
    }
    return kinds;
  }
  function fieldOf(target) {
    var art = resultEl.querySelector('article[data-target="' + target + '"]');
    return art ? art._field : null;
  }

  // Preview-side controls (delegated over the whole preview: statements,
  // cards, the rest of the source): evidence selection, apply-to-all, text
  // edits, written statements, the pair-TODO and keep actions, and the
  // jumps into Advanced. Each choice is an explicit click or keystroke.
  resultEl.addEventListener("change", function (e) {
    var t = e.target;
    if (!t || !t.getAttribute || !t.getAttribute("data-evidence-target") || t.tagName !== "INPUT") return;
    var target = t.getAttribute("data-evidence-target");
    setEvidence(target, checkedKinds(target));
    renderMappings();
    invalidate();
  });
  resultEl.addEventListener("input", function (e) {
    var t = e.target;
    if (!t || !t.classList) return;
    if (t.classList.contains("import-edit-text")) {
      var field = fieldOf(t.getAttribute("data-edit-target"));
      if (!field) return;
      setText(field, t.value);
      renderMappings();
      invalidate();
      return;
    }
    if (t.classList.contains("import-write-text")) {
      setWritten(t.getAttribute("data-write-target"), t.value);
      renderMappings();
      invalidate();
    }
  });
  resultEl.addEventListener("click", function (e) {
    var t = e.target;
    if (!t || !t.closest) return;
    var opener = t.closest(".import-open-advanced");
    if (opener) {
      openAdvanced(opener.getAttribute("data-focus"));
      return; // the anchor's own navigation to #import-advanced proceeds
    }
    var all = t.closest(".import-evidence-all");
    if (all) {
      var from = all.getAttribute("data-evidence-target");
      var kinds = checkedKinds(from);
      var arts = fieldsEl.querySelectorAll("article[data-target]");
      for (var i = 0; i < arts.length; i++) {
        var target = arts[i].getAttribute("data-target");
        if (!isAC(target)) continue;
        var boxes = arts[i].querySelectorAll("input[data-kind]");
        for (var j = 0; j < boxes.length; j++) {
          boxes[j].checked = kinds.indexOf(boxes[j].getAttribute("data-kind")) >= 0;
        }
        setEvidence(target, kinds);
      }
      renderMappings();
      invalidate();
      return;
    }
    var edit = t.closest(".import-edit");
    if (edit) {
      var ta = resultEl.querySelector('textarea[data-edit-target="' + edit.getAttribute("data-edit-target") + '"]');
      if (ta) {
        ta.hidden = false;
        ta.focus();
      }
      return;
    }
    var write = t.closest(".import-write");
    if (write) {
      var wt = resultEl.querySelector('textarea.import-write-text[data-write-target="' + write.getAttribute("data-write-target") + '"]');
      if (wt) {
        wt.hidden = false;
        wt.focus();
      }
      return;
    }
    if (t.closest(".import-defer-action")) {
      tick(deferEl);
      deferEl.focus();
      return;
    }
    if (t.closest(".import-keep-remaining")) {
      tick(retainEl);
      retainEl.focus();
    }
  });

  // -- confirmation and apply ------------------------------------------------------
  confirmEl.addEventListener("change", function () {
    applyBtn.disabled = !(confirmEl.checked && previewCurrent() && state.preview.result.ready);
    setNextAction();
  });
  // sendApply posts one creation attempt: the exact previewed bytes and
  // digest, never rebuilt. A response that arrives after an edit is still
  // reported (marked late); a lost response leaves the attempt retryable
  // while the inputs are unchanged.
  function sendApply(attempt) {
    hideError();
    retryEl.hidden = true;
    applyBtn.disabled = true;
    attempt.status = "in-flight";
    attempt.error = "";
    state.applyAttempt = attempt;
    setNextAction();
    post("/design/import/apply", attempt.request, { "X-Verdi-Import-Preview": attempt.digest })
      .then(function (r) {
        if (state.applyAttempt !== attempt) return;
        state.applyAttempt = null;
        var late = attempt.epoch !== state.epoch;
        if (r.status === 200) {
          state.created = { result: r.data, epoch: attempt.epoch };
          renderCreated(r.data, late);
          if (!late) {
            confirmEl.disabled = true;
            applyBtn.disabled = true;
          }
          setNextAction();
          return;
        }
        showError(r.data, attempt.request, late);
        if (!late) {
          if (r.data && (r.data.code === "stale-preview" || r.data.code === "dirty-context" || r.data.code === "unresolved")) {
            markStale("The server refused this preview as no longer current (" + r.data.code + "): the results below are earlier results. Preview again before creating anything.");
            confirmEl.checked = false;
            confirmEl.disabled = true;
          } else {
            applyBtn.disabled = !confirmEl.checked;
          }
        }
        setNextAction();
      })
      .catch(function (err) {
        if (state.applyAttempt !== attempt) return;
        // The request may have reached the server: its outcome is unknown,
        // and it is retryable as the SAME request while nothing changed.
        attempt.status = "lost";
        attempt.error = err.message;
        retryNote.textContent = lostNote(attempt) + " Retry the same confirmed preview: an already-created answer means it did, and nothing is duplicated.";
        retryBtn.disabled = attempt.epoch !== state.epoch;
        retryEl.hidden = false;
        setNextAction();
      });
  }
  applyBtn.addEventListener("click", function () {
    if (!previewCurrent() || !state.preview.result.ready || !confirmEl.checked) return;
    if (state.applyAttempt && state.applyAttempt.status === "in-flight") return;
    sendApply({ epoch: state.epoch, digest: state.preview.result.digest, request: state.preview.request, status: "in-flight", error: "" });
  });
  retryBtn.addEventListener("click", function () {
    var attempt = state.applyAttempt;
    if (!attempt || attempt.status !== "lost" || attempt.epoch !== state.epoch) return;
    sendApply(attempt);
  });

  function renderCreated(res, late) {
    createdEl.hidden = false;
    createdEl.setAttribute("data-status", res.status || "");
    createdEl.setAttribute("data-late", late ? "true" : "false");
    createdEl.removeAttribute("data-stale");
    var typed = "spec/" + $("import-slug").value.trim();
    var summary = (res.status === "already-created" ? "Already created earlier: " : "Created: ") + res.spec_ref + " on branch " + res.branch + " at commit " + res.commit + ", from preview " + res.preview_digest + ".";
    if (late) {
      summary = "Created earlier, for the inputs previewed before your latest edits: " + res.spec_ref + " on branch " + res.branch + " at commit " + res.commit + ", from preview " + res.preview_digest + ". Your current edits are not part of that proposal.";
    } else if (res.spec_ref !== typed) {
      summary += " Note: the published name " + res.spec_ref + " differs from the name typed here (" + typed + ").";
    } else {
      summary += " The name is exactly the one you confirmed; nothing was renamed.";
    }
    if (res.statements_deferred) {
      summary += " Both statements were left as TODO placeholders: they stand in until they are replaced on the board.";
    }
    $("import-created-summary").textContent = summary;
    $("import-board-link").setAttribute("href", res.board_path || "#");
    $("import-record-link").setAttribute("href", "/design/import/record?branch=" + encodeURIComponent(res.branch || "") + "&spec=" + encodeURIComponent(slugOf(res.spec_ref)));
    var list = $("import-disclosures");
    clear(list);
    (res.disclosures || []).forEach(function (d) {
      var li = el("li", { "data-code": d.code, "data-target": d.target || "" });
      li.appendChild(el("strong", null, d.code));
      li.appendChild(document.createTextNode(" " + (d.target ? d.target + ": " : "") + d.message));
      list.appendChild(li);
    });
    if (!(res.disclosures || []).length) list.appendChild(el("li", { class: "empty" }, "None."));
  }

  // The published transport surface (the boardspecasd.js precedent): the
  // exact request bytes and digest of the current preview, for a retry
  // driven from outside the page.
  window.__verdiImport = {
    state: function () {
      return {
        request: state.preview ? state.preview.request : "",
        digest: state.preview ? state.preview.result.digest : "",
        epoch: state.epoch,
        current: !!previewCurrent(),
      };
    },
  };

  syncClassNotes();
  syncAdvancedSummary();
  setNextAction();
})();
