// walldrawer.js: the wall's record drawer, its tabs, and the ⋯ menu that
// opens them (spec/wall-strip-and-drawer-v2 ac-4, ac-5, co-1, co-2, dc-2;
// ledger SI-368 (2), (9) to (12), (15), (16), (21), (24)(f), (24)(h);
// lane F3b). A new asset for the new behaviour (parent co-1: boardspec.js
// does not grow), built on the board's seams — the selection
// (wallselect.js: window.__WALLSELECT__) and the transport's revision
// (boardspecasd.js: window.__verdiASD) — and owning no data.
//
// The drawer and the menu are the server's markup at the body level,
// outside the swapped region in every mode (walldrawerrender.go), so no
// refresh wipes them (SI-368 (9)). Each tab renders one projection as
// prose and rows, never as JSON (ac-5): Readiness is the server's own
// fragment (/board/spec/{name}/readiness, the shared readiness renderer,
// the policy guide beside it), loaded when the tab opens and again on
// every revision change while it stays open (SI-368 (21)); Provenance,
// Review and Context call their existing on-demand operations when their
// tab opens (co-1; SI-368 (10)); Repo reads the bar's posture facts with
// no fetch; Moves and Keys are static, Keys' yarn key copied from the
// region's own source. A projection that cannot be read says so with its
// reason, never an empty or favourable stand-in. Clicking a Readiness
// item finds its wall target (SI-368 (16)): an object's card or a stub is
// selected and revealed beside the drawer; a strip half or a slot is gone
// to, the drawer shut; a row with no target selects nothing.
//
// The pill opens the Readiness tab (its link stays the no-JavaScript
// path, SI-368 (15)); the ⋯ menu opens the others, its counts loaded only
// when it opens, a failed count reading "unavailable", never 0 (SI-368
// (11)); the toolbar's Yarn key opens Keys (SI-368 (9)). The drawer is a
// dialog without a focus trap: opening it moves the focus to its tab,
// shutting it returns the focus to what opened it, and Escape with the
// focus inside it shuts it (the menu likewise), one layer a press. The
// workbench performs no forge write (co-2): the Review tab names the
// branch and the command, and opens nothing.
(function () {
  "use strict";

  var state = window.__BOARDV2__;
  var drawer = document.getElementById("record-drawer");
  if (!state || !drawer) return;
  var scrim = document.getElementById("record-drawer-scrim");
  var menu = document.getElementById("wall-more-menu");

  var TABS = ["readiness", "provenance", "review", "context", "repo", "moves", "keys"];
  var REVEAL_MARGIN = 40; // px a found card keeps inside the canvas's visible box, the drawer's width set aside

  var spec = state.spec;
  var path = window.location.pathname;
  var prefix = path.indexOf("/b/") === 0 ? path.slice(0, path.indexOf("/board/spec/")) : "";
  function url(rest) {
    return prefix + "/board/spec/" + spec + rest;
  }
  // word is a class id's display word: the payload's renamed words, the
  // seam boardspec.js reads for its own copy (parent co-4).
  function word(id) {
    var w = state.words || {};
    return w[id] || id;
  }

  var current = null; // the open tab, or null while the drawer is shut
  var opener = null; // the control that opened the drawer, which gets the focus back
  var loads = {}; // tab → the sequence number of its newest load
  var seq = 0;
  var readinessRevision = null; // the revision the Readiness tab was last loaded at

  // -- markup helpers ------------------------------------------------------------

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined && text !== null && text !== "") n.textContent = text;
    return n;
  }
  function count(n, one, many) {
    return n + " " + (n === 1 ? one : many);
  }
  function clip(s, max) {
    s = String(s || "");
    return s.length > max ? s.slice(0, max - 1) + "…" : s;
  }
  // short shortens a digest or a commit for the eye; the whole value is
  // its element's title.
  function short(d) {
    d = String(d || "");
    var bare = d.replace(/^sha256:/, "");
    if (bare.length <= 12) return d;
    return (d.indexOf("sha256:") === 0 ? "sha256:" : "") + bare.slice(0, 8) + "…" + bare.slice(-4);
  }
  function plain(s) {
    return String(s || "").replace(/-/g, " ");
  }

  function bodyOf(tab) {
    return drawer.querySelector('[data-record-body="' + tab + '"]');
  }

  // section appends one titled section to into and returns it; its rows
  // and empty state follow.
  function section(into, title, aside, tone) {
    var s = el("section", "record-section");
    if (tone) s.setAttribute("data-tone", tone);
    var head = el("div", "record-section-head");
    head.appendChild(el("h3", "record-section-title", title));
    if (aside !== undefined && aside !== null && aside !== "") head.appendChild(el("span", "record-section-aside", String(aside)));
    s.appendChild(head);
    into.appendChild(s);
    return s;
  }
  // rows appends a list of key and value rows: each {k, v, mono, title,
  // note, badge, tone}.
  function rows(into, list) {
    var dl = el("dl", "record-rows");
    for (var i = 0; i < list.length; i++) {
      var r = list[i];
      var row = el("div", "record-row");
      row.appendChild(el("dt", "record-key", r.k));
      var dd = el("dd", "record-value" + (r.mono ? " record-mono" : ""));
      var v = el("span", "record-text", r.v);
      if (r.title) v.title = r.title;
      dd.appendChild(v);
      if (r.note) dd.appendChild(el("span", "record-note", r.note));
      row.appendChild(dd);
      if (r.badge) {
        var cell = el("dd", "record-badge-cell");
        var badge = el("span", "record-badge", r.badge);
        if (r.tone) badge.setAttribute("data-tone", r.tone);
        cell.appendChild(badge);
        row.appendChild(cell);
      }
      dl.appendChild(row);
    }
    into.appendChild(dl);
    return dl;
  }
  function empty(into, text) {
    into.appendChild(el("p", "record-empty", text));
  }
  // unavailable says, in prose, that a projection could not be read and
  // why: the posture reason, never an empty or favourable stand-in.
  function unavailable(into, subject, reason) {
    var p = el("p", "record-unavailable", subject + " unavailable: " + String(reason).replace(/\.$/, "") + ".");
    p.setAttribute("data-testid", "record-unavailable");
    p.setAttribute("role", "status");
    into.appendChild(p);
  }

  // -- the design operations (co-1: each one an existing on-demand read) --------

  // design posts one read; it resolves to {data} or {reason}, the
  // failure envelope's own code and detail when the core answered one.
  function design(op) {
    return fetch(url("/api/" + op), { method: "POST", headers: { "Content-Type": "application/json" }, body: "{}" }).then(
      function (resp) {
        return resp.text().then(function (text) {
          var data;
          try {
            data = JSON.parse(text);
          } catch (e) {
            return { reason: "the response could not be read (HTTP " + resp.status + ")" };
          }
          if (data && data.schema === "verdi.design-failure/v1") return { reason: data.code + ": " + data.detail };
          if (data && typeof data.error === "string") return { reason: data.error };
          if (!resp.ok || !data || typeof data !== "object") return { reason: "HTTP " + resp.status };
          return { data: data };
        });
      },
      function (err) {
        return { reason: "the request failed: " + err.message };
      }
    );
  }

  // -- Provenance (ProvenanceResult; handoff "Record drawer") --------------------

  var CLASSIFICATIONS = {
    "human-stated": { word: "human", tone: "evidenced" },
    "ai-synthesized": { word: "ai-synthesized", tone: "authored" },
    "ai-inferred": { word: "ai-inferred", tone: "authored" },
    unresolved: { word: "unresolved", tone: "violated" },
  };

  function opLine(op) {
    var line = op.op;
    if (op.op === "add-link" || op.op === "remove-link") {
      line += " " + (op.type || "") + " · " + (op.source || "spec") + " → " + (op.ref || "");
    } else if (op.op === "add-context-ref" || op.op === "remove-context-ref") {
      line += " " + (op.ref || "");
    } else {
      if (op.id || op.slug) line += " " + (op.id || op.slug);
      if (op.after_id || op.after_slug) line += " after " + (op.after_id || op.after_slug);
    }
    if (op.text) line += " · “" + clip(op.text, 140) + "”";
    if (op.spike) line += " · " + word("spike");
    if (op.resolves && op.resolves.length) line += " · resolves " + op.resolves.join(", ");
    if (op.acceptance_criteria && op.acceptance_criteria.length) line += " · claims " + op.acceptance_criteria.join(", ");
    if (op.evidence && op.evidence.length) line += " · evidence " + op.evidence.join(", ");
    return line;
  }

  function entryNote(e) {
    var parts = [];
    var a = e.attribution || {};
    if (a.principal_id) parts.push("by " + a.principal_id);
    else if (a.unauthenticated) parts.push("by an unauthenticated human");
    if (e.policy) parts.push(e.policy.state === "resolved" ? "policy " + short(e.policy.digest) : "no policy applicable");
    else if (e.policy_digest) parts.push("policy " + short(e.policy_digest));
    parts.push(count((e.changes || []).length, "change", "changes"));
    return parts.join(" · ");
  }

  function entryBadge(e) {
    var seen = {};
    var words = [];
    var tone = "";
    var ex = e.excerpts || [];
    for (var i = 0; i < ex.length; i++) {
      var c = CLASSIFICATIONS[ex[i].classification] || { word: ex[i].classification, tone: "" };
      if (seen[c.word]) continue;
      seen[c.word] = true;
      words.push(c.word);
      if (!tone || c.tone === "violated" || (c.tone === "authored" && tone === "evidenced")) tone = c.tone;
    }
    return { badge: words.join(" · "), tone: tone };
  }

  function gapLine(g) {
    if (!g.from_digest) return "The typed chain records nothing before " + short(g.to_digest) + ": all of that content has no recorded author or intent.";
    return "Markdown edited outside the typed core between " + short(g.from_digest) + " and " + short(g.to_digest) + ". Whatever changed in that window has no recorded author or intent.";
  }

  function renderProvenance(into, r) {
    var entries = Array.isArray(r.entries) ? r.entries : [];
    var ops = section(into, "Typed operations", count(entries.length, "entry", "entries"));
    if (!entries.length) {
      empty(ops, "No typed operation is recorded for this spec yet. Edits made through the wall land here; a direct Markdown edit is disclosed by the review packet.");
    } else {
      var list = [];
      for (var i = entries.length - 1; i >= 0; i--) {
        var e = entries[i];
        var lines = [];
        for (var j = 0; j < (e.operations || []).length; j++) lines.push(opLine(e.operations[j]));
        var b = entryBadge(e);
        list.push({ k: "#" + (i + 1), v: lines.join("; ") || "no operation", note: entryNote(e), badge: b.badge, tone: b.tone });
      }
      rows(ops, list);
    }
    var gaps = [];
    for (var k = 0; k < entries.length; k++) if (entries[k].unclassified_gap) gaps.push(entries[k].unclassified_gap);
    var direct = section(into, "Unclassified direct edits", count(gaps.length, "gap", "gaps"), "violated");
    if (!gaps.length) {
      empty(direct, "None recorded between typed operations.");
      return;
    }
    var gapRows = [];
    for (var m = 0; m < gaps.length; m++) gapRows.push({ k: "gap", v: gapLine(gaps[m]), title: gaps[m].from_digest + " → " + gaps[m].to_digest, badge: "unclassified", tone: "violated" });
    rows(direct, gapRows);
  }

  // -- Review (ReviewResult) ----------------------------------------------------------

  var CHANGE_TONES = { added: "evidenced", "relationship-added": "evidenced", replaced: "authored", reordered: "authored", removed: "violated", "relationship-removed": "violated" };

  function renderReview(into, r) {
    var base = r.baseline || {};
    var branch = (r.identity && r.identity.branch) || "";
    var baseSec = section(into, "Review base", base.available ? "available" : "unavailable", "frozen");
    if (base.available) {
      rows(baseSec, [
        { k: "branch", v: base.branch || "", mono: true },
        { k: "commit", v: short(base.commit), title: base.commit, mono: true },
        { k: "this branch", v: branch, mono: true },
      ]);
    } else {
      empty(baseSec, base.reason ? "No review base: " + base.reason : "No review base is named for this packet.");
      rows(baseSec, [{ k: "this branch", v: branch, mono: true }]);
    }

    var changes = r.changes || [];
    var changed = section(into, "Semantic changes", String(changes.length));
    if (!changes.length) {
      empty(changed, base.available ? "None since the review base." : "None derived: there is no review base to compare with.");
    } else {
      var cl = [];
      for (var i = 0; i < changes.length; i++) cl.push({ k: changes[i].target, v: plain(changes[i].change), badge: plain(changes[i].change), tone: CHANGE_TONES[changes[i].change] || "" });
      rows(changed, cl);
    }

    var flags = r.inferred_or_unresolved || [];
    var edits = r.unclassified_edits || [];
    var eye = section(into, "Needs a human eye", String(flags.length + edits.length), "violated");
    if (!flags.length && !edits.length) {
      empty(eye, "Nothing: no object is ai-inferred or unresolved, and no direct edit is unclassified.");
    } else {
      var list = [];
      for (var j = 0; j < flags.length; j++) {
        var c = CLASSIFICATIONS[flags[j].classification] || { word: flags[j].classification, tone: "authored" };
        list.push({ k: flags[j].target, v: "an excerpt for it was recorded as " + c.word + "; a human has not restated it", badge: c.word, tone: c.tone });
      }
      for (var k = 0; k < edits.length; k++) {
        list.push({ k: "direct edit", v: gapLine(edits[k]), title: (edits[k].from_digest || "(none)") + " → " + edits[k].to_digest, badge: "unclassified", tone: "violated" });
      }
      rows(eye, list);
    }

    var warnings = r.warnings || [];
    var warn = section(into, "Material warnings", String(warnings.length));
    if (!warnings.length) {
      empty(warn, "None. The diff since the review base raised no structural warning.");
    } else {
      var wl = [];
      for (var m = 0; m < warnings.length; m++) wl.push({ k: warnings[m].target, v: plain(warnings[m].code), badge: "warning", tone: "pending" });
      rows(warn, wl);
    }

    rows(section(into, "Policy in force", ""), [
      { k: "mode", v: r.policy_mode || "none", mono: true },
      { k: "digest", v: r.policy_digest ? short(r.policy_digest) : "none: no policy is adopted", title: r.policy_digest || "", mono: !!r.policy_digest },
    ]);

    // SI-368 (12): forge-agnostic — the branch and the command, and never
    // a control that opens anything (co-2). Only a wall that takes edits
    // proposes from its branch; any other says so.
    var proposing = state.mode === "authoring" && !state.domainRefusal;
    var pr = section(into, "Open a pull request", proposing ? branch : "");
    pr.setAttribute("data-testid", "record-review-command");
    if (!proposing) {
      empty(pr, "No pull request is proposed from this wall: it takes no edit here, and a proposal is pushed from its own design branch.");
    } else if (branch && branch !== base.branch) {
      pr.appendChild(el("p", "record-prose", "Push the branch, then open a pull request on your forge:"));
      var pre = el("pre", "record-command");
      pre.appendChild(el("code", "", "git push -u origin " + branch));
      pr.appendChild(pre);
    } else {
      empty(pr, branch ? "This wall is on the review base's own branch, " + branch + ": there is no branch to propose from." : "The packet names no branch to propose from.");
    }
  }

  // -- Context (DesignContextResult, CapabilitiesResult) ----------------------------

  var RATIFIED = {
    "accepted-parent-feature": function (p) {
      return "The parent " + word("feature") + " " + (p.source || "") + " is accepted, so its decisions are ratified authority.";
    },
    "no-parent-feature": function () {
      return "This spec declares no parent " + word("feature") + ", so there is no ratified decision source. Its own decisions are proposals and are not echoed here as authority.";
    },
    "parent-feature-not-accepted": function (p) {
      return "The parent " + word("feature") + " " + (p.source || "") + " is " + plain(p.parent_state || "not accepted") + ", so its decisions are proposals, not ratified authority.";
    },
    "parent-declared-missing": function (p) {
      return "This spec declares a parent " + word("feature") + ", " + (p.source || "") + ", that is not in the active zone.";
    },
  };

  function contentLine(d) {
    var n = function (list) {
      return (list || []).length;
    };
    return [
      count(n(d.acceptance_criteria), "acceptance criterion", "acceptance criteria"),
      count(n(d.constraints), "constraint", "constraints"),
      count(n(d.decisions), "decision", "decisions"),
      count(n(d.open_questions), "open question", "open questions"),
      count(n(d.stubs), "stub", "stubs"),
    ].join(" · ");
  }

  function renderCapabilities(into, caps) {
    var agents = section(into, "What agents may do", "");
    if (caps.reason) {
      unavailable(agents, "Design capabilities are", caps.reason);
      return;
    }
    var k = caps.data || {};
    var refusal = k.mutability_refusal || {};
    var line = "Delegated agents may apply typed draft writes here.";
    var badge = "may write";
    var tone = "evidenced";
    if (!k.mutable && refusal.precondition === "policy-mode") {
      line = "Delegated agents cannot write here (" + refusal.precondition + "): " + (refusal.detail || "");
      badge = "agents refused";
      tone = "pending";
    } else if (!k.mutable) {
      line = "Typed draft writes are refused here for humans and agents alike (" + (refusal.precondition || "refused") + "): " + (refusal.detail || "");
      badge = "refused";
      tone = "pending";
    }
    var permitted = (k.permitted_operations || []).length;
    rows(agents, [
      { k: "writes", v: line, badge: badge, tone: tone },
      { k: "operations", v: permitted ? count(permitted, "typed operation is", "typed operations are") + " permitted" : "none permitted" },
      { k: "policy mode", v: k.policy_mode || "none", mono: true },
    ]);
  }

  function renderContext(into, c, caps) {
    var d = c.current_draft || {};
    var links = [];
    for (var i = 0; i < (d.links || []).length; i++) links.push(d.links[i].type + " " + d.links[i].ref);
    rows(section(into, "Current spec", d.id || ""), [
      { k: "title", v: d.title || "" },
      { k: "objects", v: contentLine(d) },
      { k: "links", v: links.length ? links.join(" · ") : "none", mono: links.length > 0 },
    ]);

    var posture = c.ratified_decisions_posture || {};
    var parent = section(into, "Parent " + word("feature"), c.parent_feature ? plain(c.parent_feature.state) : "none declared");
    if (c.parent_feature) {
      rows(parent, [
        { k: "ref", v: c.parent_feature.ref, mono: true },
        { k: "title", v: (c.parent_feature.content && c.parent_feature.content.title) || "" },
      ]);
    }
    var why = RATIFIED[posture.reason];
    if (why && c.parent_feature) parent.appendChild(el("p", "record-prose", why(posture)));
    else if (why) empty(parent, why(posture));
    var ratified = c.ratified_decisions || [];
    if (ratified.length) {
      var rl = [];
      for (var r = 0; r < ratified.length; r++) rl.push({ k: ratified[r].id, v: ratified[r].text });
      rows(parent, rl);
    }

    var ap = c.applicable_policy || {};
    rows(section(into, "Applicable policy", "", "evidenced"), [
      { k: "policy", v: ap.policy_id || "none", mono: true },
      { k: "mode", v: ap.mode || "none", mono: true },
      { k: "layout", v: ap.layout ? "agents may suggest wall layout" : "agents may not suggest wall layout" },
    ]);

    // SI-368 (3): the capabilities families' home.
    renderCapabilities(into, caps);

    var pinned = c.pinned_context || [];
    var pin = section(into, "Pinned context", count(pinned.length, "ref", "refs"), "frozen");
    if (!pinned.length) {
      empty(pin, "This spec declares no pinned context.");
    } else {
      var pl = [];
      for (var p = 0; p < pinned.length; p++) pl.push({ k: pinned[p].kind, v: pinned[p].ref + " — " + pinned[p].title, badge: "frozen", tone: "frozen" });
      rows(pin, pl);
    }

    var vg = c.verdi_go_findings || {};
    var found = section(into, "Verdi-go findings", vg.available ? String((vg.findings || []).length) : "unavailable");
    if (!vg.available) {
      empty(found, vg.reason ? "Omitted, and disclosed: " + vg.reason : "Omitted: no toolchain finding was derived.");
    } else if (!(vg.findings || []).length) {
      empty(found, "None. No service or boundary finding applies.");
    } else {
      var fl = [];
      for (var f = 0; f < vg.findings.length; f++) fl.push({ k: vg.findings[f].id, v: vg.findings[f].text, badge: plain(vg.findings[f].kind), tone: "pending" });
      rows(found, fl);
    }

    rows(section(into, "Digests", ""), [
      { k: "context", v: short(c.context_digest), title: c.context_digest || "", mono: true },
      { k: "policy", v: c.policy_digest ? short(c.policy_digest) : "none", title: c.policy_digest || "", mono: true },
    ]);
  }

  // -- Repo: the bar's posture facts, no fetch (SI-368 (10)) -------------------------

  function barNode(testid) {
    return document.querySelector('[data-testid="topbar"] [data-testid="' + testid + '"]');
  }

  function fact(slug) {
    var dd = barNode("asd-posture-" + slug);
    if (!dd) return null;
    var why = barNode("asd-posture-" + slug + "-why");
    var unproven = dd.getAttribute("data-state") !== "proven";
    return { v: dd.textContent.trim(), note: why ? why.textContent.trim() : "", badge: unproven ? "unproven" : "", tone: unproven ? "pending" : "" };
  }

  function renderRepo(into) {
    var bytes = barNode("asd-posture-bytes");
    var tree = barNode("asd-posture-tree");
    var treeWhy = barNode("asd-posture-tree-why");
    if (!tree && !fact("branch")) {
      empty(into, "This page's bar carries no posture facts.");
      return;
    }
    var add = function (list, k, slug) {
      var f = fact(slug);
      if (f) list.push({ k: k, v: f.v, note: f.note, badge: f.badge, tone: f.tone, mono: true });
    };
    var work = [];
    add(work, "branch", "branch");
    add(work, "worktree HEAD", "worktree-head");
    if (bytes) {
      // The word is the summary's first text; the formal state beside it
      // shows only where it is not the word itself.
      var formal = bytes.querySelector(".asd-posture-formal");
      var shown = formal && !formal.classList.contains("topbar-sr") ? formal.textContent.replace(/[()]/g, "").trim() : "";
      var bytesWord = (bytes.firstChild ? bytes.firstChild.textContent : "").replace(/^displayed bytes:\s*/, "").trim();
      var bytesUnproven = bytes.getAttribute("data-state") === "unproven";
      work.push({ k: "displayed bytes", v: bytesWord, note: shown ? "formal state " + shown : "", badge: bytesUnproven ? "unproven" : "", tone: "pending" });
    }
    var dirty = tree ? tree.getAttribute("data-dirty") || "" : "";
    if (tree) {
      work.push({ k: "working tree", v: tree.textContent.replace(/^working tree:\s*/, "").trim(), note: treeWhy ? treeWhy.textContent.trim() : "", badge: dirty === "unproven" ? "unproven" : dirty === "dirty" ? "uncommitted" : "", tone: "pending" });
    }
    add(work, "checkout", "checkout");
    rows(section(into, "Working tree", dirty, "evidenced"), work);
    var acc = [];
    add(acc, "accepted branch", "accepted-branch");
    add(acc, "accepted HEAD", "accepted-head");
    add(acc, "ahead / behind", "ahead-behind");
    add(acc, "divergence", "divergence");
    add(acc, "base digest", "base-digest");
    var branch = fact("accepted-branch");
    rows(section(into, "Accepted record", branch && !branch.badge ? branch.v : "", "frozen"), acc);
  }

  // -- Keys: the yarn key from the region's own source (SI-368 (9)) ----------------

  function renderKeys(into) {
    var src = document.querySelectorAll('#boardv2-region [data-testid="yarn-key"] li');
    var key = section(into, "Yarn key", "what the colours mean");
    key.setAttribute("data-testid", "record-yarn-key");
    if (!src.length) {
      empty(key, "No yarn on this wall yet: a thread's colour joins this key once one is strung.");
      return;
    }
    var dl = el("dl", "record-rows record-yarn");
    for (var i = 0; i < src.length; i++) {
      var row = el("div", "record-row");
      row.setAttribute("data-layer", src[i].getAttribute("data-layer") || "");
      row.setAttribute("data-edge-type", src[i].getAttribute("data-edge-type") || "");
      var dt = el("dt", "record-key");
      var sw = el("span", "yarn-key-swatch");
      sw.setAttribute("aria-hidden", "true");
      dt.appendChild(sw);
      var type = src[i].querySelector(".yarn-key-type");
      dt.appendChild(document.createTextNode(type ? type.textContent : ""));
      row.appendChild(dt);
      var what = src[i].querySelector(".yarn-key-what");
      row.appendChild(el("dd", "record-value", what ? what.textContent : ""));
      dl.appendChild(row);
    }
    key.appendChild(dl);
  }

  // -- loading a tab -----------------------------------------------------------------

  function revisionNow() {
    var t = window.__verdiASD;
    return t && t.state ? t.state().revision : null;
  }

  function busy(body, text) {
    body.textContent = "";
    body.setAttribute("aria-busy", "true");
    var s = el("p", "record-status", text);
    s.setAttribute("role", "status");
    body.appendChild(s);
  }
  function done(body) {
    body.removeAttribute("aria-busy");
  }
  // fresh reports whether a load is still its tab's newest, so a slow
  // response never paints over a newer one.
  function fresh(tab, n) {
    return loads[tab] === n;
  }

  function load(tab) {
    var body = bodyOf(tab);
    if (!body) return;
    var n = ++seq;
    loads[tab] = n;
    switch (tab) {
      case "readiness":
        loadReadiness(body, n);
        return;
      case "provenance":
      case "review":
        busy(body, "Deriving…");
        design(tab === "provenance" ? "get_design_provenance" : "prepare_design_review").then(function (r) {
          if (!fresh(tab, n)) return;
          body.textContent = "";
          if (r.reason) unavailable(body, tab === "provenance" ? "Provenance is" : "The review packet is", r.reason);
          else if (tab === "provenance") renderProvenance(body, r.data);
          else renderReview(body, r.data);
          done(body);
        });
        return;
      case "context":
        busy(body, "Deriving…");
        Promise.all([design("get_design_context"), design("get_design_capabilities")]).then(function (both) {
          if (!fresh(tab, n)) return;
          body.textContent = "";
          if (both[0].reason) {
            unavailable(body, "The design context is", both[0].reason);
            renderCapabilities(body, both[1]);
          } else {
            renderContext(body, both[0].data, both[1]);
          }
          done(body);
        });
        return;
      case "repo":
        body.textContent = "";
        renderRepo(body);
        return;
      case "keys":
        body.textContent = "";
        renderKeys(body);
        return;
    }
  }

  function loadReadiness(body, n) {
    var at = revisionNow();
    if (!body.firstChild) busy(body, "Deriving readiness…");
    fetch(url("/readiness"))
      .then(function (resp) {
        return resp.text().then(function (text) {
          return { ok: resp.ok, status: resp.status, text: text };
        });
      })
      .then(
        function (r) {
          if (!fresh("readiness", n)) return;
          readinessRevision = at;
          if (!r.ok) {
            body.textContent = "";
            unavailable(body, "Readiness is", "HTTP " + r.status);
          } else {
            var keep = openDisclosures(body);
            body.innerHTML = r.text; // the server's own escaped fragment
            reopen(body, keep);
          }
          done(body);
        },
        function (err) {
          if (!fresh("readiness", n)) return;
          body.textContent = "";
          unavailable(body, "Readiness is", "the request failed: " + err.message);
          done(body);
        }
      );
  }

  // A re-fetch on a revision change keeps the disclosures the reader
  // opened, by the concern or section they sit in.
  function disclosureKey(d) {
    var row = d.closest("[data-concern-id]");
    return (row ? row.getAttribute("data-concern-id") : "") + "|" + d.className;
  }
  function openDisclosures(body) {
    var keys = [];
    var ds = body.querySelectorAll("details[open]");
    for (var i = 0; i < ds.length; i++) keys.push(disclosureKey(ds[i]));
    return keys;
  }
  function reopen(body, keys) {
    if (!keys.length) return;
    var ds = body.querySelectorAll("details");
    for (var i = 0; i < ds.length; i++) if (keys.indexOf(disclosureKey(ds[i])) >= 0) ds[i].open = true;
  }

  // -- opening, choosing and shutting ----------------------------------------------

  function tabButton(tab) {
    return document.getElementById("record-tab-" + tab);
  }

  // zoom is the page's CSS zoom (a page zoomed to 200 % the way the
  // budget tests zoom it): a measured box is in the viewport's pixels, a
  // fixed element's offsets in the zoomed page's.
  function zoom() {
    return document.body.currentCSSZoom || 1;
  }

  // place keeps the drawer and its scrim under the top bar, wherever the
  // bar wraps or the page scrolls it.
  function place() {
    var bar = document.querySelector('[data-testid="topbar"]');
    var top = bar ? Math.max(0, Math.round(bar.getBoundingClientRect().bottom / zoom())) : 0;
    drawer.style.setProperty("--record-drawer-top", top + "px");
    if (scrim) scrim.style.setProperty("--record-drawer-top", top + "px");
  }

  function choose(tab, focus) {
    current = tab;
    drawer.setAttribute("data-tab", tab);
    for (var i = 0; i < TABS.length; i++) {
      var on = TABS[i] === tab;
      var b = tabButton(TABS[i]);
      if (b) {
        b.setAttribute("aria-selected", on ? "true" : "false");
        b.setAttribute("tabindex", on ? "0" : "-1");
      }
      var p = document.getElementById("record-panel-" + TABS[i]);
      if (p) p.hidden = !on;
    }
    load(tab);
    if (focus && tabButton(tab)) tabButton(tab).focus({ preventScroll: true });
  }

  function open(tab, from) {
    if (TABS.indexOf(tab) < 0) return;
    closeMenu(false);
    if (drawer.hidden) {
      opener = from || document.activeElement;
      place();
      drawer.hidden = false;
      if (scrim) scrim.hidden = false;
    }
    choose(tab, true);
  }

  function close() {
    if (drawer.hidden) return;
    drawer.hidden = true;
    if (scrim) scrim.hidden = true;
    current = null;
    drawer.removeAttribute("data-tab");
    var back = opener;
    opener = null;
    if (back && back.isConnected && back.focus) back.focus({ preventScroll: true });
  }

  // -- finding a Readiness item's wall target (SI-368 (16)) ---------------------------

  // reveal scrolls a card into the canvas's visible box, the part the
  // drawer covers set aside (handoff "Keyboard": at least 40 px inside
  // the visible width, minus the drawer when open).
  function reveal(card) {
    var c = document.getElementById("board-canvas");
    if (!c) return;
    var frame = document.querySelector("#boardv2-region .wall-frame");
    if (frame) frame.scrollIntoView({ block: "nearest", inline: "nearest" });
    var covered = drawer.hidden ? 0 : Math.max(0, c.getBoundingClientRect().right - drawer.getBoundingClientRect().left);
    var width = Math.max(c.clientWidth - covered, card.offsetWidth);
    var sl = c.scrollLeft;
    var st = c.scrollTop;
    if (card.offsetLeft - REVEAL_MARGIN < sl) sl = card.offsetLeft - REVEAL_MARGIN;
    else if (card.offsetLeft + card.offsetWidth + REVEAL_MARGIN > sl + width) sl = card.offsetLeft + card.offsetWidth + REVEAL_MARGIN - width;
    if (card.offsetTop - REVEAL_MARGIN < st) st = card.offsetTop - REVEAL_MARGIN;
    else if (card.offsetTop + card.offsetHeight + REVEAL_MARGIN > st + c.clientHeight) st = card.offsetTop + card.offsetHeight + REVEAL_MARGIN - c.clientHeight;
    c.scrollLeft = Math.max(0, sl);
    c.scrollTop = Math.max(0, st);
  }

  function goTo(target) {
    close();
    if (!target) return;
    if (!target.hasAttribute("tabindex") && !/^(BUTTON|A|TEXTAREA|INPUT)$/.test(target.tagName)) target.setAttribute("tabindex", "-1");
    target.scrollIntoView({ block: "nearest", inline: "nearest" });
    target.focus({ preventScroll: true });
  }

  function find(row) {
    var kind = row.getAttribute("data-target-kind");
    var value = row.getAttribute("data-target") || "";
    var seam = window.__WALLSELECT__;
    if (kind === "object" || kind === "stub") {
      var sel = { kind: "card", key: kind === "stub" ? "stub:" + value : value };
      var card = seam ? seam.elementOf(sel) : null;
      if (!card) return;
      seam.select(sel);
      reveal(card);
      var found = bodyOf("readiness").querySelectorAll("[data-found]");
      for (var i = 0; i < found.length; i++) found[i].removeAttribute("data-found");
      row.setAttribute("data-found", "true");
      return;
    }
    if (kind === "strip") {
      var half = document.querySelector('#boardv2-region [data-testid="placard-' + value + '"]');
      goTo(half ? half.querySelector(".placard-text") || half : null);
      return;
    }
    if (kind === "slot") {
      var slot = document.querySelector('#boardv2-region .wall-slot[data-slot-kind="' + value + '"] .wall-slot-open');
      if (slot) goTo(slot);
    }
  }

  drawer.addEventListener("click", function (e) {
    var t = e.target;
    if (!(t instanceof Element)) return;
    var tab = t.closest('[role="tab"][data-record-tab]');
    if (tab) {
      choose(tab.getAttribute("data-record-tab"), false);
      return;
    }
    if (t.closest(".record-drawer-close")) {
      close();
      return;
    }
    var row = t.closest('[data-record-body="readiness"] article[data-target-kind]');
    if (!row || row.getAttribute("data-target-kind") === "none") return;
    // The row's own controls keep their meaning — its disclosure, a link,
    // the CLI vector — except the target button, which finds.
    if (!t.closest(".readiness-target") && t.closest("a, button, summary, details, [data-readiness-cli]")) return;
    find(row);
  });

  if (scrim) scrim.addEventListener("click", close);

  // The tabs' keys (manual activation: the arrows move the focus, Enter
  // or Space opens the tab), and Escape with the focus in the drawer.
  drawer.addEventListener("keydown", function (e) {
    if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      close();
      return;
    }
    var t = e.target;
    if (!(t instanceof Element) || t.getAttribute("role") !== "tab") return;
    var i = TABS.indexOf(t.getAttribute("data-record-tab"));
    var next = -1;
    if (e.key === "ArrowRight") next = (i + 1) % TABS.length;
    else if (e.key === "ArrowLeft") next = (i + TABS.length - 1) % TABS.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = TABS.length - 1;
    if (next < 0) return;
    e.preventDefault();
    tabButton(TABS[next]).focus();
  });

  window.addEventListener("resize", function () {
    if (!drawer.hidden) place();
  });
  window.addEventListener(
    "scroll",
    function () {
      if (!drawer.hidden) place();
    },
    { passive: true }
  );

  // Freshness (SI-368 (21)): while Readiness is open it loads again on
  // each revision change, as the marks follow the poll; Repo and Keys
  // follow the bar's and the region's own swaps, which land in the same
  // pass (the posture right after the region).
  document.addEventListener("wall-region-swapped", function () {
    if (current === "readiness" && revisionNow() !== readinessRevision) load("readiness");
    if (current === "repo" || current === "keys") {
      var tab = current;
      setTimeout(function () {
        if (current === tab) load(tab);
      }, 0);
    }
  });

  // -- the ⋯ menu (SI-368 (11)) ----------------------------------------------------

  function moreButton() {
    return document.querySelector('[data-testid="wall-more"]');
  }
  function items() {
    return menu ? menu.querySelectorAll('[role="menuitem"]') : [];
  }

  function setCount(tab, text, stateWord, why) {
    var c = menu.querySelector('[data-count="' + tab + '"]');
    if (!c) return;
    c.textContent = text;
    c.setAttribute("data-count-state", stateWord);
    if (why) c.title = why;
    else c.removeAttribute("title");
  }

  // loadCounts loads each count when the menu opens, and only then: two
  // design reads, and the bar's own ahead fact.
  function loadCounts() {
    setCount("provenance", "counting…", "loading");
    setCount("review", "counting…", "loading");
    design("get_design_provenance").then(function (r) {
      if (r.reason) setCount("provenance", "unavailable", "unavailable", r.reason);
      else setCount("provenance", count((r.data.entries || []).length, "entry", "entries"), "loaded");
    });
    design("prepare_design_review").then(function (r) {
      if (r.reason) {
        setCount("review", "unavailable", "unavailable", r.reason);
        return;
      }
      var n = (r.data.inferred_or_unresolved || []).length + (r.data.unclassified_edits || []).length;
      setCount("review", n === 1 ? "1 needs a human eye" : n + " need a human eye", n > 0 ? "attention" : "loaded");
    });
    var ab = fact("ahead-behind");
    var m = ab && !ab.badge ? /^(\d+) ahead/.exec(ab.v) : null;
    if (m) setCount("repo", m[1] + " ahead", "loaded");
    else setCount("repo", "unavailable", "unavailable", ab ? ab.note || ab.v : "the bar carries no ahead fact");
  }

  // placeMenu hangs the menu from ⋯'s right edge, inside the viewport:
  // what does not fit below ⋯ scrolls inside the menu.
  function placeMenu() {
    var btn = moreButton();
    if (!btn) return;
    var z = zoom();
    var r = btn.getBoundingClientRect();
    var room = document.documentElement.clientWidth / z;
    var top = Math.round(r.bottom / z + 4);
    menu.style.maxHeight = Math.max(120, Math.round(document.documentElement.clientHeight / z - top - 8)) + "px";
    var left = Math.min(r.right / z - menu.offsetWidth, room - menu.offsetWidth - 8);
    menu.style.top = top + "px";
    menu.style.left = Math.round(Math.max(8, left)) + "px";
  }

  function openMenu() {
    if (!menu || !menu.hidden) return;
    menu.hidden = false;
    placeMenu();
    var btn = moreButton();
    if (btn) btn.setAttribute("aria-expanded", "true");
    loadCounts();
    var first = items()[0];
    if (first) first.focus({ preventScroll: true });
  }

  function closeMenu(refocus) {
    if (!menu || menu.hidden) return;
    menu.hidden = true;
    var btn = moreButton();
    if (btn) {
      btn.setAttribute("aria-expanded", "false");
      if (refocus) btn.focus({ preventScroll: true });
    }
  }

  if (menu) {
    menu.addEventListener("click", function (e) {
      var item = e.target instanceof Element ? e.target.closest('[role="menuitem"][data-record-tab]') : null;
      if (item) open(item.getAttribute("data-record-tab"), moreButton());
    });
    // The menu's keys: the arrows, Home and End move between its items;
    // Escape shuts it and returns the focus to ⋯; Tab leaves it at ⋯.
    menu.addEventListener("keydown", function (e) {
      var list = items();
      var at = Array.prototype.indexOf.call(list, document.activeElement);
      var next = -1;
      if (e.key === "Escape" || e.key === "Tab") {
        e.preventDefault();
        e.stopPropagation();
        closeMenu(true);
        return;
      }
      if (e.key === "ArrowDown") next = (at + 1) % list.length;
      else if (e.key === "ArrowUp") next = (at + list.length - 1) % list.length;
      else if (e.key === "Home") next = 0;
      else if (e.key === "End") next = list.length - 1;
      if (next < 0) return;
      e.preventDefault();
      list[next].focus();
    });
    document.addEventListener("pointerdown", function (e) {
      if (menu.hidden) return;
      var t = e.target;
      if (t instanceof Element && (t.closest("#wall-more-menu") || t.closest('[data-testid="wall-more"]'))) return;
      closeMenu(false);
    });
    document.addEventListener("focusout", function (e) {
      if (menu.hidden) return;
      var to = e.relatedTarget;
      if (to instanceof Element && (menu.contains(to) || to.closest('[data-testid="wall-more"]'))) return;
      setTimeout(function () {
        var a = document.activeElement;
        if (!menu.hidden && a && a !== document.body && !menu.contains(a) && !a.closest('[data-testid="wall-more"]')) closeMenu(false);
      }, 0);
    });
    window.addEventListener("resize", function () {
      if (!menu.hidden) placeMenu();
    });
  }

  // The bar's openers: ⋯ toggles the menu; the pill — a link to the
  // readiness page without JavaScript — opens the Readiness tab (a
  // modified click keeps the link's own meaning).
  document.addEventListener("click", function (e) {
    var t = e.target;
    if (!(t instanceof Element)) return;
    if (t.closest('[data-testid="wall-more"]')) {
      if (menu && menu.hidden) openMenu();
      else closeMenu(false);
      return;
    }
    var pill = t.closest("a[data-drawer-tab]");
    if (!pill || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    e.preventDefault();
    open(pill.getAttribute("data-drawer-tab"), pill);
  });

  window.__WALLDRAWER__ = {
    open: open,
    close: close,
    current: function () {
      return current;
    },
  };
})();
