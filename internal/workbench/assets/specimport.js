// specimport.js — the spec importer's browser script (spec-import-contract
// "Errors and browser behavior"; plan Task 4 UI). Dependency-free, no
// frameworks, no storage authority. It owns
//
//   - file reading: each chosen file's exact bytes, base64-encoded without
//     any text decode, labeled by the file NAME (never a path), with a
//     mechanical source id derived from that name;
//   - request composition: the exact strict Request JSON the server's ONE
//     decoder accepts — sources, format, target, dispositions, explicit
//     mappings and declared links — nothing inferred, nothing invented;
//   - the preview fetch and its rendering: findings with their next
//     corrective action, fields with their source origins, evidence and
//     edit controls, byte coverage, sources;
//   - invalidation: EVERY edit (file, range, primary, format, target,
//     disposition, mapping, evidence, link) bumps an edit epoch, marks the
//     shown preview stale, and clears and disables the confirmation; a
//     preview response for an older epoch — or an older request — is
//     discarded on arrival, so a stale response can never reinstate a
//     confirmation;
//   - apply: only the exact request bytes previewed, with that preview's
//     digest in X-Verdi-Import-Preview, and only while confirmed.
//
// Everything the server or a source supplies is rendered with textContent;
// no source byte becomes markup. No polling, no AI, no provider.
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
  var findingsEl = $("import-findings");
  var fieldsEl = $("import-fields");
  var coverageBody = $("import-coverage").querySelector("tbody");
  var sourcesEl = $("import-sources");
  var confirmEl = $("import-confirm");
  var applyBtn = $("import-apply-btn");
  var createdEl = $("import-created");

  var EVIDENCE_KINDS = ["static", "behavioral", "runtime", "attestation"];
  var TRANSFORMS = ["identity", "trim-blank-lines", "collapse-whitespace", "list-item"];
  var LINK_TYPES = ["implements", "resolves", "supersedes", "exempts", "verifies", "derived-from", "annotates", "depends-on", "story", "impacts", "challenges"];
  var MAX_SOURCES = 32;

  // Finding code -> the next corrective action, in plain words.
  var GUIDANCE = {
    "missing-statement": "Add a labeled Problem or Outcome section to the primary source, add an explicit mapping for it below, or tick 'Defer both statements'.",
    "empty-field": "The labeled section is empty: add its text to the source or map the field explicitly.",
    "ambiguous-field": "The source labels this field more than once; add an explicit mapping naming the text to use. The importer never picks one.",
    "multiple-targets": "The source holds more than one top-level document; narrow the line range to one.",
    "unsupported-structure": "The primary has no recognizable heading structure for this format; choose another format or map the fields explicitly.",
    "missing-evidence": "Select the evidence kinds for this criterion below (your selection, never inferred).",
    "unresolved-coverage": "Map the remaining bytes, or tick 'Retain every unmapped byte' to acknowledge them as retained-only source.",
    "source-id-requires-mapping": "The item carries its own id; add an explicit mapping that preserves or resolves it.",
    "invalid-candidate": "The composed spec fails validation; correct the named field or target.",
    "existing-corpus-finding": "A pre-existing corpus finding, disclosed separately; it was not caused by this import.",
    "statements-deferred": "Disclosure only: a TODO placeholder stands in for this statement until it is replaced on the board.",
  };
  var ERROR_GUIDANCE = {
    "dirty-context": "Commit or remove the named paths in the serving checkout, then preview again; nothing was reset.",
    "stale-preview": "The server recomputed a different digest; preview again and confirm the fresh preview.",
    "target-exists": "That spec name is already taken on this store; choose a different name. The existing branch was left untouched, never renamed or republished.",
    "unresolved": "The preview still has blocking findings; correct them and preview again.",
    "policy-forbidden": "The store's adopted policy refuses this write.",
    "actor-forbidden": "This write's actor was refused.",
    "invalid-request": "Correct the named input and preview again.",
    "invalid-source": "The named source cannot be honored as selected (bytes, range or encoding).",
    "unsupported-format": "Choose one of the listed formats; the F13 profile only accepts its pinned primary bytes.",
  };

  var state = {
    sources: [], // {id, label, data, size, startLine, endLine}
    primary: "",
    mappings: [], // {target, sourceId, start, end, transform, text, evidence}
    links: [], // {type, ref}
    epoch: 0,
    previewSeq: 0,
    preview: null, // {result, request, epoch}
    created: null,
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
  function labelOf(sourceId) {
    for (var i = 0; i < state.sources.length; i++) {
      if (state.sources[i].id === sourceId) return state.sources[i].label;
    }
    return sourceId;
  }
  function isAC(target) {
    return typeof target === "string" && target.indexOf("ac-") === 0;
  }
  function slugOf(specRef) {
    return String(specRef || "").replace(/^spec\//, "");
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
      return { label: file.name, data: base64Of(new Uint8Array(buf)), size: buf.byteLength };
    });
  }

  // -- invalidation -------------------------------------------------------------
  // Every edit lands here: the epoch moves, the confirmation is cleared and
  // disabled, the shown preview (if any) is marked stale, and the created
  // panel is retired.
  function invalidate() {
    state.epoch++;
    state.created = null;
    confirmEl.checked = false;
    confirmEl.disabled = true;
    applyBtn.disabled = true;
    createdEl.hidden = true;
    if (state.preview) {
      resultEl.setAttribute("data-stale", "true");
      staleNote.hidden = false;
    }
    setNextAction();
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
  function setNextAction(text) {
    if (text) {
      nextAction.textContent = text;
      return;
    }
    if (state.created) {
      nextAction.textContent = "Created. Open the board to continue, or the source record to inspect the original copied content.";
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
        var src = { id: sourceIdFor(read[i].label), label: read[i].label, data: read[i].data, size: read[i].size, startLine: 0, endLine: 0 };
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
      sourceList.appendChild(li);
    });
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

  function renderMappings() {
    clear(mappingList);
    state.mappings.forEach(function (m, index) {
      var li = el("li", { "data-testid": "import-mapping-" + index, "data-mapping-index": String(index), "data-mapping-target": m.target });
      var head = el("div", { class: "import-inline" });
      var targetLabel = el("label");
      targetLabel.appendChild(document.createTextNode("Target "));
      var target = el("input", { class: "import-mapping-target", "data-testid": "import-mapping-target-" + index, spellcheck: "false", placeholder: "problem, outcome or ac-1" });
      target.value = m.target;
      targetLabel.appendChild(target);
      head.appendChild(targetLabel);
      var sourceLabel = el("label");
      sourceLabel.appendChild(document.createTextNode("Source "));
      var source = el("select", { class: "import-mapping-source" });
      source.appendChild(el("option", { value: "" }, "(none — user text only)"));
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
        lab.appendChild(document.createTextNode(" " + kind));
        ev.appendChild(lab);
      });
      li.appendChild(ev);
      mappingList.appendChild(li);
    });
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
    var sourceRow = t.closest("#import-source-list li");
    if (sourceRow) {
      if (t.name === "import-primary") state.primary = t.value;
      syncSourceRow(sourceRow);
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
  form.addEventListener("click", function (e) {
    var t = e.target;
    if (!t || !t.closest) return;
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
      target: { slug: $("import-slug").value.trim(), class: $("import-class").value, title: $("import-title").value.trim() },
      format: $("import-format").value,
      primary: state.primary,
      sources: state.sources.map(function (s) {
        var o = { id: s.id, label: s.label, data: s.data };
        if (s.startLine) o.start_line = s.startLine;
        if (s.endLine) o.end_line = s.endLine;
        return o;
      }),
      defer_statements: $("import-defer").checked,
      retain_unmapped: $("import-retain").checked,
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
  function showError(failure) {
    var code = (failure && failure.code) || "transport";
    var detail = (failure && (failure.error || failure.message)) || "no detail";
    errorEl.setAttribute("data-code", code);
    errorEl.textContent = "Refused (" + code + "): " + detail + (ERROR_GUIDANCE[code] ? " — " + ERROR_GUIDANCE[code] : "");
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
          showError(r.data);
          setNextAction();
          return;
        }
        state.preview = { result: r.data, request: body, epoch: epoch };
        renderPreview(r.data);
        setNextAction();
      })
      .catch(function (err) {
        if (epoch !== state.epoch) return;
        showError({ code: "transport", error: err.message });
        setNextAction();
      });
  }

  function renderPreview(result) {
    resultEl.hidden = false;
    resultEl.setAttribute("data-stale", "false");
    resultEl.setAttribute("data-ready", String(!!result.ready));
    staleNote.hidden = true;
    var blocking = blockingCount(result);
    readyEl.setAttribute("data-ready", String(!!result.ready));
    readyEl.textContent = result.ready
      ? "Ready: no blocking findings. Confirm below to create exactly this proposal."
      : "Not ready: " + blocking + " blocking finding" + (blocking === 1 ? "" : "s") + ". Correct them, then preview again.";
    digestEl.textContent = result.digest || "";

    clear(findingsEl);
    (result.findings || []).forEach(function (f) {
      var li = el("li", { "data-code": f.code, "data-target": f.target || "", "data-blocking": String(!!f.blocking) });
      li.appendChild(el("strong", null, f.code + (f.blocking ? " (blocking)" : " (disclosure)")));
      li.appendChild(document.createTextNode(" "));
      if (f.target) li.appendChild(el("span", { class: "import-finding-target" }, f.target + ": "));
      li.appendChild(el("span", { class: "import-finding-message" }, f.message));
      if (GUIDANCE[f.code]) li.appendChild(el("span", { class: "import-finding-next" }, "Next: " + GUIDANCE[f.code]));
      findingsEl.appendChild(li);
    });
    if (!(result.findings || []).length) findingsEl.appendChild(el("li", { class: "empty" }, "None."));

    clear(fieldsEl);
    var editable = $("import-format").value !== "native";
    (result.fields || []).forEach(function (f) {
      fieldsEl.appendChild(renderField(f, editable));
    });
    if (!(result.fields || []).length) fieldsEl.appendChild(el("p", { class: "empty" }, "No fields resolved yet."));

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
  }

  function renderField(f, editable) {
    var art = el("article", { class: "import-field", "data-testid": "import-field-" + f.target, "data-target": f.target, "data-origin": f.origin });
    var h = el("h4", null, f.target);
    h.appendChild(el("span", { class: "import-origin", "data-testid": "import-field-origin-" + f.target }, f.origin));
    art.appendChild(h);
    art.appendChild(el("pre", { class: "import-field-text", "data-testid": "import-field-text-" + f.target }, f.text));
    var spans = el("p", { class: "import-field-spans", "data-testid": "import-field-spans-" + f.target });
    if (f.origin === "generated-deferral") {
      spans.textContent = "Generated TODO placeholder — visibly incomplete, not source text.";
    } else if (f.spans && f.spans.length) {
      spans.textContent = f.spans
        .map(function (sp) {
          return "from " + labelOf(sp.source_id) + " bytes [" + sp.start + "," + sp.end + ")" + (sp.transform ? " transform " + sp.transform : "");
        })
        .join("; ");
    } else {
      spans.textContent = "User-supplied text; no source span.";
    }
    art.appendChild(spans);

    if (isAC(f.target)) {
      var ev = el("fieldset", { class: "import-evidence" });
      ev.appendChild(el("legend", null, "Evidence kinds (your selection)"));
      EVIDENCE_KINDS.forEach(function (kind) {
        var lab = el("label");
        var box = el("input", { type: "checkbox", "data-evidence-target": f.target, "data-kind": kind, "data-testid": "import-evidence-" + f.target + "-" + kind });
        box.checked = (f.evidence || []).indexOf(kind) >= 0;
        lab.appendChild(box);
        lab.appendChild(document.createTextNode(" " + kind));
        ev.appendChild(lab);
      });
      ev.appendChild(el("button", { type: "button", class: "import-evidence-all", "data-evidence-target": f.target, "data-testid": "import-evidence-all-" + f.target }, "Use these kinds for every criterion"));
      art.appendChild(ev);
    }
    if (editable) {
      art.appendChild(el("button", { type: "button", class: "import-edit", "data-edit-target": f.target, "data-testid": "import-edit-" + f.target }, "Edit text"));
      var ta = el("textarea", { class: "import-edit-text", "data-edit-target": f.target, "data-testid": "import-edit-text-" + f.target, "aria-label": "Edited text for " + f.target });
      ta.value = f.text;
      ta.hidden = true;
      art.appendChild(ta);
    }
    art._field = f;
    return art;
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
    var art = fieldsEl.querySelector('article[data-target="' + target + '"]');
    return art ? art._field : null;
  }

  // Preview-side controls: evidence selection, apply-to-all, text edits.
  fieldsEl.addEventListener("change", function (e) {
    var t = e.target;
    if (!t || !t.getAttribute || !t.getAttribute("data-evidence-target") || t.tagName !== "INPUT") return;
    var target = t.getAttribute("data-evidence-target");
    setEvidence(target, checkedKinds(target));
    renderMappings();
    invalidate();
  });
  fieldsEl.addEventListener("input", function (e) {
    var t = e.target;
    if (!t || !t.classList || !t.classList.contains("import-edit-text")) return;
    var field = fieldOf(t.getAttribute("data-edit-target"));
    if (!field) return;
    setText(field, t.value);
    renderMappings();
    invalidate();
  });
  fieldsEl.addEventListener("click", function (e) {
    var t = e.target;
    if (!t || !t.closest) return;
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
      var ta = fieldsEl.querySelector('textarea[data-edit-target="' + edit.getAttribute("data-edit-target") + '"]');
      if (ta) {
        ta.hidden = false;
        ta.focus();
      }
    }
  });

  // -- confirmation and apply ------------------------------------------------------
  confirmEl.addEventListener("change", function () {
    applyBtn.disabled = !(confirmEl.checked && previewCurrent() && state.preview.result.ready);
    setNextAction();
  });
  applyBtn.addEventListener("click", function () {
    if (!previewCurrent() || !state.preview.result.ready || !confirmEl.checked) return;
    var epoch = state.epoch;
    var digest = state.preview.result.digest;
    var body = state.preview.request; // the exact bytes previewed, never rebuilt
    hideError();
    applyBtn.disabled = true;
    setNextAction("Creating…");
    post("/design/import/apply", body, { "X-Verdi-Import-Preview": digest })
      .then(function (r) {
        if (epoch !== state.epoch) return;
        if (r.status !== 200) {
          showError(r.data);
          if (r.data && (r.data.code === "stale-preview" || r.data.code === "dirty-context" || r.data.code === "unresolved")) {
            resultEl.setAttribute("data-stale", "true");
            staleNote.hidden = false;
            confirmEl.checked = false;
            confirmEl.disabled = true;
          } else {
            applyBtn.disabled = !confirmEl.checked;
          }
          setNextAction();
          return;
        }
        state.created = r.data;
        renderCreated(r.data);
        confirmEl.disabled = true;
        applyBtn.disabled = true;
        setNextAction();
      })
      .catch(function (err) {
        if (epoch !== state.epoch) return;
        showError({ code: "transport", error: err.message + ". Nothing favorable is assumed; preview again and retry." });
        setNextAction();
      });
  });

  function renderCreated(res) {
    createdEl.hidden = false;
    createdEl.setAttribute("data-status", res.status || "");
    var typed = "spec/" + $("import-slug").value.trim();
    var summary = (res.status === "already-created" ? "Already created earlier: " : "Created: ") + res.spec_ref + " on branch " + res.branch + " at commit " + res.commit + ", from preview " + res.preview_digest + ".";
    if (res.spec_ref !== typed) {
      summary += " Note: the published name " + res.spec_ref + " differs from the name typed here (" + typed + ").";
    } else {
      summary += " The name is exactly the one you confirmed; nothing was renamed.";
    }
    if (res.statements_deferred) {
      summary += " Both statements were deferred: TODO placeholders stand in until they are replaced on the board.";
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

  setNextAction();
})();
