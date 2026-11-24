/* The consignment note register.

   Everything this page shows is derived by the tool it is talking to: the page
   holds no rules of its own, keeps no second copy of a figure, and never works
   out a total the binary could have worked out. The tool answers with the whole
   register and everything derived from it, and the page draws that. If the two
   ever disagree, the page is the one that is wrong — which is the only
   arrangement in which "the register says" means anything. */

(function () {
  "use strict";

  var state = null; // the whole register, as the tool last answered
  var open = null;  // the note being worked on, with everything derived from it
  var saveTimer = null;

  var el = {};
  var IDS = [
    "no-script", "saving",
    "new-note", "load-sample", "clear-register",
    "audit", "audit-headline", "audit-breaks", "audit-table", "audit-body",
    "toggle-gaps", "gap-form", "gap-series", "gap-seq", "gap-date", "gap-reason", "gap-cancel",
    "tiles",
    "register-card", "register-count", "register-headline", "register", "register-body", "register-empty",
    "f-text", "f-series", "f-status", "f-extra",
    "note-pane", "form", "note-number", "verdict-box", "verdict", "verdict-sub", "tracker",
    "liable", "tax", "tax-caveats", "findings", "print-copies", "dl-findings",
    "advance", "advance-note", "action-problem", "where",
    "deliver-wrap", "d-date", "d-packages", "d-counted", "d-name", "d-role",
    "d-condition", "d-collected", "d-id", "d-remarks", "do-deliver",
    "cancel-wrap", "c-date", "c-reason", "c-by", "do-cancel",
    "renumber-wrap", "r-seq", "do-renumber",
    "o-name", "o-phone", "o-address", "o-city", "o-state", "o-pin", "o-gstin", "o-pan", "o-terms", "office-problem",
    "books", "b-series", "b-label", "b-start", "b-width", "save-book", "book-problem",
    "t-rcm", "t-fwd5", "t-fwd12", "t-exempt", "t-apply", "save-rates", "reset-rates", "rates-problem",
    "constants"
  ];
  IDS.forEach(function (id) { el[id] = document.getElementById(id); });

  /* ---- figures ------------------------------------------------------- */

  // Money crosses the wire as whole paise and weight as whole grams, so the
  // page converts for display and back for entry, and never holds a fraction
  // of either.
  var PAISE = 100, GRAMS = 1000;

  function groupIndian(whole) {
    var s = String(whole);
    if (s.length <= 3) { return s; }
    var head = s.slice(0, -3), tail = s.slice(-3), parts = [];
    while (head.length > 2) { parts.unshift(head.slice(-2)); head = head.slice(0, -2); }
    if (head) { parts.unshift(head); }
    return parts.join(",") + "," + tail;
  }

  function money(paise) {
    var v = Math.abs(paise || 0);
    var out = groupIndian(Math.floor(v / PAISE)) + "." + String(v % PAISE).padStart(2, "0");
    return (paise < 0 ? "-" : "") + out;
  }

  function moneyInput(paise) {
    if (!paise) { return ""; }
    var v = Math.abs(paise);
    return (paise < 0 ? "-" : "") + Math.floor(v / PAISE) + "." + String(v % PAISE).padStart(2, "0");
  }

  function parseMoney(text) {
    var t = String(text == null ? "" : text).replace(/[₹,\s]/g, "");
    if (!t) { return 0; }
    var n = Number(t);
    return isFinite(n) ? Math.round(n * PAISE) : 0;
  }

  function kgInput(grams) {
    if (!grams) { return ""; }
    var v = Math.abs(grams), frac = String(v % GRAMS).padStart(3, "0").replace(/0+$/, "");
    return (grams < 0 ? "-" : "") + Math.floor(v / GRAMS) + (frac ? "." + frac : "");
  }

  function parseKg(text) {
    var t = String(text == null ? "" : text).replace(/[,\skgKG]/g, "");
    if (!t) { return 0; }
    var n = Number(t);
    return isFinite(n) ? Math.round(n * GRAMS) : 0;
  }

  function pctInput(hundredths) {
    if (!hundredths) { return "0"; }
    var whole = Math.floor(hundredths / 100), part = hundredths % 100;
    if (!part) { return String(whole); }
    return String(whole) + "." + (part % 10 === 0 ? String(part / 10) : String(part).padStart(2, "0"));
  }

  function parsePct(text) {
    var t = String(text == null ? "" : text).replace(/[%\s]/g, "");
    if (!t) { return 0; }
    var n = Number(t);
    return isFinite(n) ? Math.round(n * 100) : 0;
  }

  function plural(n, one, many) { return n === 1 ? "1 " + one : n + " " + many; }

  /* ---- the DOM ------------------------------------------------------- */

  function node(tag, className, text) {
    var n = document.createElement(tag);
    if (className) { n.className = className; }
    if (text != null) { n.textContent = text; }
    return n;
  }

  function clear(parent) { while (parent.firstChild) { parent.removeChild(parent.firstChild); } }

  function options(select, list, selected, blank) {
    clear(select);
    if (blank) { select.appendChild(new Option(blank, "")); }
    list.forEach(function (item) {
      var opt = new Option(item.name || item, item.value != null ? item.value : item);
      select.appendChild(opt);
    });
    if (selected != null) { select.value = selected; }
  }

  function say(target, message) {
    if (!target) { return; }
    target.textContent = message || "";
    target.hidden = !message;
  }

  function saving(message) { el.saving.textContent = message || ""; }

  /* ---- talking to the tool ------------------------------------------- */

  function request(method, path, body) {
    var init = { method: method, headers: { "Accept": "application/json" } };
    if (body !== undefined) {
      init.headers["Content-Type"] = "application/json";
      init.body = JSON.stringify(body);
    }
    return fetch(path, init).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (data) {
        if (!res.ok) {
          var err = new Error(data.error || ("the tool answered " + res.status));
          err.problems = data.problems || [];
          throw err;
        }
        return data;
      });
    });
  }

  function takeState(data) {
    state = data;
    renderAudit();
    renderTiles();
    renderFilters();
    renderRegister();
    renderOffice();
    saving("saved");
    return data;
  }

  function reload() { return request("GET", "/api/state").then(takeState); }

  function takeNote(data) {
    open = data;
    renderNote();
    return reload().then(function () { return data; });
  }

  function problem(err) {
    saving("");
    say(el["action-problem"], err.message);
  }

  /* ---- the serial audit --------------------------------------------- */

  function renderAudit() {
    var audit = state.audit || {};
    el.audit.hidden = false;
    el.audit.className = "audit";
    if (!audit.intact) { el.audit.className = "audit broken"; }
    else if ((audit.breaks || []).length) { el.audit.className = "audit watch"; }

    el["audit-headline"].textContent = audit.headline || "";

    clear(el["audit-breaks"]);
    (audit.breaks || []).forEach(function (text) {
      el["audit-breaks"].appendChild(node("li", null, text));
    });

    clear(el["audit-body"]);
    (audit.series || []).forEach(function (s) {
      var tr = node("tr");
      tr.appendChild(node("td", null, s.label || s.series || "unlabelled"));
      tr.appendChild(node("td", "num", s.first || "—"));
      tr.appendChild(node("td", "num", s.last || "—"));
      tr.appendChild(node("td", "num", s.span || 0));
      tr.appendChild(node("td", "num", s.accounted || 0));
      tr.appendChild(node("td", "num" + (s.missing ? " bad" : ""), s.missing || 0));
      tr.appendChild(node("td", "num", s.next || 1));
      tr.appendChild(node("td", null, s.verdict || ""));
      el["audit-body"].appendChild(tr);
    });

    el["audit-table"].hidden = !(audit.series || []).length;
    el["toggle-gaps"].hidden = !audit.missing;
    if (!audit.missing) { el["gap-form"].hidden = true; }
  }

  /* ---- the summary tiles -------------------------------------------- */

  function tile(parent, key, value, sub, tone) {
    var box = node("div", "tile" + (tone ? " " + tone : ""));
    box.appendChild(node("div", "k", key));
    box.appendChild(node("div", "v", value));
    if (sub) { box.appendChild(node("div", "s", sub)); }
    parent.appendChild(box);
  }

  function renderTiles() {
    var s = state.summary || {};
    el.tiles.hidden = !s.notes;
    if (!s.notes) { return; }

    clear(el.tiles);
    tile(el.tiles, "Notes in the register", s.notes, plural(s.delivered || 0, "delivered", "delivered"));
    tile(el.tiles, "Still out", s.open || 0,
      (s.inTransit || 0) + " in transit, " + (s.arrived || 0) + " arrived",
      s.open ? "" : "ok");
    tile(el.tiles, "To collect at the door", "₹" + money(s.toCollectPaise),
      "on to-pay consignments not yet delivered", s.toCollectPaise ? "warn" : "ok");
    tile(el.tiles, "Freight still owed", "₹" + money(s.outstandingPaise), "across every note");
    if (s.overdue) {
      tile(el.tiles, "Past the expected date", s.overdue, "still not delivered", "bad");
    }
    if (s.withBlockers) {
      tile(el.tiles, "Not ready to issue", s.withBlockers, "something has to be fixed", "bad");
    }
    if (s.taxUnstated) {
      tile(el.tiles, "No tax position stated", s.taxUnstated, "the note will say so on its face", "warn");
    }
  }

  /* ---- the register -------------------------------------------------- */

  function renderFilters() {
    var series = (state.books || []).map(function (b) {
      return { value: b.series, name: b.label || b.series || "unlabelled series" };
    });
    var keepSeries = el["f-series"].value;
    options(el["f-series"], series, keepSeries, "Every series");

    var statuses = (state.reference && state.reference.statuses) || [];
    var keepStatus = el["f-status"].value;
    options(el["f-status"], statuses, keepStatus, "Every status");

    var gapSeries = series.length ? series : [{ value: "", name: "unlabelled series" }];
    options(el["gap-series"], gapSeries, el["gap-series"].value || (gapSeries[0] && gapSeries[0].value));
  }

  function matches(row) {
    var text = el["f-text"].value.trim().toLowerCase();
    if (text) {
      var hay = [row.number, row.consignor, row.consignee, row.route, row.goods,
        row.vehicle, row.driver, row.eWayBill, row.statusName].join(" \u0000 ").toLowerCase();
      if (hay.indexOf(text) < 0) { return false; }
    }
    if (el["f-series"].value && row.series !== el["f-series"].value) { return false; }
    if (el["f-status"].value && row.status !== el["f-status"].value) { return false; }

    switch (el["f-extra"].value) {
      case "outstanding": return row.outstandingPaise > 0;
      case "overdue": return !!row.overdue;
      case "blockers": return row.blockers > 0;
      case "unstated": return !row.taxSettled && row.status !== "void";
      default: return true;
    }
  }

  function statusChip(row) {
    var tone = "dead";
    if (row.status === "delivered") { tone = "done"; }
    else if (row.overdue) { tone = "late"; }
    else if (row.status === "booked" || row.status === "in-transit" || row.status === "arrived") { tone = "open"; }

    var text = row.statusName;
    if (row.overdue) { text += " · " + plural(row.daysLate, "day late", "days late"); }
    else if (row.status === "delivered" && row.daysLate) { text += " · " + plural(row.daysLate, "day late", "days late"); }
    return node("span", "chip " + tone, text);
  }

  function renderRegister() {
    var rows = (state.rows || []).filter(matches);
    el["register-count"].textContent = rows.length === (state.rows || []).length
      ? plural(rows.length, "note", "notes")
      : rows.length + " of " + plural((state.rows || []).length, "note", "notes");
    el["register-headline"].textContent = state.headline || "";

    clear(el["register-body"]);
    rows.forEach(function (row) {
      var tr = node("tr");
      tr.className = (row.status === "void" ? "void " : "") + (row.blockers ? "flagged" : "");
      tr.setAttribute("aria-selected", open && open.number === row.number ? "true" : "false");
      tr.tabIndex = 0;

      var ref = node("td", "ref", row.number);
      if (row.blockers) {
        ref.appendChild(node("span", "sub", plural(row.blockers, "thing to fix", "things to fix")));
      }
      tr.appendChild(ref);
      tr.appendChild(node("td", null, row.datePrint));

      var parties = node("td", null, row.consignor || "—");
      parties.appendChild(node("span", "sub", row.consignee ? "→ " + row.consignee : "no consignee"));
      tr.appendChild(parties);

      var route = node("td", null, row.route || "—");
      if (row.goods) { route.appendChild(node("span", "sub", row.goods)); }
      tr.appendChild(route);

      tr.appendChild(node("td", "num", row.packages || 0));
      tr.appendChild(node("td", "num", row.weight || "—"));

      var terms = node("td", null, row.termsStamp.replace("FREIGHT ", ""));
      if (!row.taxSettled && row.status !== "void") {
        terms.appendChild(node("span", "sub", "tax position not stated"));
      }
      tr.appendChild(terms);

      tr.appendChild(node("td", "num", money(row.grossPaise)));
      tr.appendChild(node("td", "num", row.outstandingPaise ? money(row.outstandingPaise) : "—"));

      var status = node("td");
      status.appendChild(statusChip(row));
      tr.appendChild(status);

      var actions = node("td");
      var btn = node("button", "rowbtn", "Open");
      btn.type = "button";
      actions.appendChild(btn);
      tr.appendChild(actions);

      function openIt() { loadNote(row.number); }
      tr.addEventListener("click", openIt);
      tr.addEventListener("keydown", function (ev) {
        if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); openIt(); }
      });
      el["register-body"].appendChild(tr);
    });

    el["register-empty"].hidden = rows.length > 0 || !(state.rows || []).length;
    el.register.hidden = rows.length === 0;
  }

  function loadNote(number) {
    saving("opening " + number + "…");
    return request("GET", "/api/note?number=" + encodeURIComponent(number))
      .then(function (data) {
        open = data;
        renderNote();
        renderRegister();
        el["note-pane"].scrollIntoView({ block: "start" });
        saving("");
      })
      .catch(problem);
  }

  /* ---- the open note: the form --------------------------------------- */

  function field(parent, label, hint, build) {
    var box = node("div", "field");
    var lab = node("label", null, label);
    if (hint) { lab.appendChild(node("span", "hint", " " + hint)); }
    box.appendChild(lab);
    var input = build(box);
    if (input && input.id) { lab.htmlFor = input.id; }
    parent.appendChild(box);
    return box;
  }

  function textField(parent, label, hint, id, value, onChange, placeholder) {
    return field(parent, label, hint, function (box) {
      var wrap = node("div", "inp");
      var input = node("input");
      input.type = "text";
      input.id = id;
      input.value = value == null ? "" : value;
      if (placeholder) { input.placeholder = placeholder; }
      input.addEventListener("input", function () { onChange(input.value); queueSave(); });
      wrap.appendChild(input);
      box.appendChild(wrap);
      return input;
    });
  }

  function numberField(parent, label, hint, id, value, onChange, prefix, suffix) {
    return field(parent, label, hint, function (box) {
      var wrap = node("div", "inp" + (prefix ? " has-pre" : "") + (suffix ? " has-suf" : ""));
      if (prefix) { wrap.appendChild(node("span", "pre", prefix)); }
      var input = node("input");
      input.type = "text";
      input.inputMode = "decimal";
      input.id = id;
      input.className = "num";
      input.value = value == null ? "" : value;
      input.addEventListener("input", function () { onChange(input.value); queueSave(); });
      wrap.appendChild(input);
      if (suffix) { wrap.appendChild(node("span", "suf", suffix)); }
      box.appendChild(wrap);
      return input;
    });
  }

  function areaField(parent, label, hint, id, value, onChange, placeholder) {
    return field(parent, label, hint, function (box) {
      var area = node("textarea");
      area.id = id;
      area.value = value == null ? "" : value;
      if (placeholder) { area.placeholder = placeholder; }
      area.addEventListener("input", function () { onChange(area.value); queueSave(); });
      box.appendChild(area);
      return area;
    });
  }

  function dateField(parent, label, hint, id, value, onChange) {
    return field(parent, label, hint, function (box) {
      var input = node("input");
      input.type = "date";
      input.id = id;
      input.value = value || "";
      input.addEventListener("change", function () { onChange(input.value); queueSave(); });
      box.appendChild(input);
      return input;
    });
  }

  function selectField(parent, label, hint, id, list, value, onChange, blank) {
    return field(parent, label, hint, function (box) {
      var select = node("select");
      select.id = id;
      options(select, list, value, blank);
      select.addEventListener("change", function () { onChange(select.value); queueSave(); });
      box.appendChild(select);
      return select;
    });
  }

  function card(parent, heading, blurb) {
    var section = node("section", "card");
    var h = node("h2", null, heading);
    section.appendChild(h);
    if (blurb) { section.appendChild(node("p", "blurb", blurb)); }
    parent.appendChild(section);
    return section;
  }

  function partyBlock(parent, heading, blurb, party) {
    var section = card(parent, heading, blurb);
    var top = node("div", "row2");
    textField(top, "Name", null, heading + "-name", party.name, function (v) { party.name = v; });
    textField(top, "Phone", null, heading + "-phone", party.phone, function (v) { party.phone = v; });
    section.appendChild(top);

    areaField(section, "Address", null, heading + "-address", party.address, function (v) { party.address = v; });

    var row = node("div", "row4");
    textField(row, "City", null, heading + "-city", party.city, function (v) { party.city = v; });
    textField(row, "State", null, heading + "-state", party.state, function (v) { party.state = v; });
    textField(row, "PIN", null, heading + "-pin", party.pin, function (v) { party.pin = v; });
    textField(row, "GSTIN", null, heading + "-gstin", party.gstin, function (v) { party.gstin = v; });
    section.appendChild(row);
    return section;
  }

  function renderForm() {
    var n = open.note, ref = state.reference || {};
    clear(el.form);

    /* the note itself */
    var top = card(el.form, "The consignment note",
      "The number cannot be changed here. A note that has to be renumbered goes through the renumber action, which will only move it forwards.");
    var topRow = node("div", "row3");
    field(topRow, "Consignment note no.", null, function (box) {
      var wrap = node("div", "inp");
      var input = node("input");
      input.type = "text";
      input.id = "n-number";
      input.value = open.number;
      input.readOnly = true;
      wrap.appendChild(input);
      box.appendChild(wrap);
      return input;
    });
    dateField(topRow, "Date", null, "n-date", n.date, function (v) { n.date = v; });
    numberField(topRow, "Days in transit", "for the expected date", "n-transit", n.transitDays || "",
      function (v) { n.transitDays = Math.max(0, parseInt(v, 10) || 0); });
    top.appendChild(topRow);

    var routeRow = node("div", "row2");
    textField(routeRow, "From", "the booking station", "n-from", n.from, function (v) { n.from = v; }, "Bengaluru");
    textField(routeRow, "To", "the destination station", "n-to", n.to, function (v) { n.to = v; }, "Hubballi");
    top.appendChild(routeRow);

    var markRow = node("div", "row3");
    textField(markRow, "Private mark", null, "n-mark", n.privateMark, function (v) { n.privateMark = v; });
    textField(markRow, "Consignor's invoice no.", null, "n-inv", n.consignorInvoiceNo, function (v) { n.consignorInvoiceNo = v; });
    textField(markRow, "E-way bill no.", "recorded, not generated", "n-eway", n.eWayBillNumber, function (v) { n.eWayBillNumber = v; });
    top.appendChild(markRow);
    top.appendChild(node("p", "blurb",
      "The e-way bill is a separate document, generated and filed on the government portal. This field records the number it was given so the two can be found together; nothing here raises one."));

    /* the parties */
    partyBlock(el.form, "Consignor", "Whoever handed the goods over.", n.consignor);
    partyBlock(el.form, "Consignee", "Whoever is to receive them. A consignment with nobody to deliver it to cannot be carried.", n.consignee);

    /* the goods */
    var goods = card(el.form, "The goods",
      "Described as the consignor declared them. The carrier has not opened the packages, and every copy says so.");
    var scroll = node("div", "table-scroll");
    var table = node("table", "goods");
    var head = node("thead");
    var hrow = node("tr");
    [["#", ""], ["Description of the goods", "w-desc"], ["Packages", "w-small num"],
     ["Packing", "w-small"], ["Weight (kg)", "w-small num"], ["", ""]].forEach(function (cell) {
      hrow.appendChild(node("th", cell[1], cell[0]));
    });
    head.appendChild(hrow);
    table.appendChild(head);

    var body = node("tbody");
    (n.goods || []).forEach(function (line, i) {
      var tr = node("tr");
      tr.appendChild(node("td", null, String(i + 1)));

      tr.appendChild(cell(function (td) {
        var input = node("input");
        input.type = "text";
        input.value = line.description || "";
        input.setAttribute("aria-label", "Description of goods on line " + (i + 1));
        input.addEventListener("input", function () { line.description = input.value; queueSave(); });
        td.appendChild(input);
      }));

      tr.appendChild(cell(function (td) {
        var input = node("input");
        input.type = "text";
        input.className = "num";
        input.inputMode = "numeric";
        input.value = line.packages || "";
        input.setAttribute("aria-label", "Packages on line " + (i + 1));
        input.addEventListener("input", function () {
          line.packages = Math.max(0, parseInt(input.value, 10) || 0);
          queueSave();
        });
        td.appendChild(input);
      }));

      tr.appendChild(cell(function (td) {
        var select = node("select");
        select.setAttribute("aria-label", "Packing on line " + (i + 1));
        options(select, ref.packingKinds || [], line.packing, "—");
        select.addEventListener("change", function () { line.packing = select.value; queueSave(); });
        td.appendChild(select);
      }));

      tr.appendChild(cell(function (td) {
        var input = node("input");
        input.type = "text";
        input.className = "num";
        input.inputMode = "decimal";
        input.value = kgInput(line.actualGrams);
        input.setAttribute("aria-label", "Weight in kilograms on line " + (i + 1));
        input.addEventListener("input", function () { line.actualGrams = parseKg(input.value); queueSave(); });
        td.appendChild(input);
      }));

      tr.appendChild(cell(function (td) {
        var btn = node("button", "rowbtn danger", "Remove");
        btn.type = "button";
        btn.addEventListener("click", function () {
          n.goods.splice(i, 1);
          save().then(renderForm);
        });
        td.appendChild(btn);
      }));
      body.appendChild(tr);
    });
    table.appendChild(body);

    var foot = node("tfoot");
    var frow = node("tr");
    frow.appendChild(node("td", "label", "Total"));
    frow.appendChild(node("td", "label", ""));
    frow.appendChild(node("td", null, String(open.totals.packages || 0)));
    frow.appendChild(node("td", null, ""));
    frow.appendChild(node("td", null, kgInput(open.totals.actualGrams) || "0"));
    frow.appendChild(node("td", null, ""));
    foot.appendChild(frow);
    table.appendChild(foot);

    scroll.appendChild(table);
    goods.appendChild(scroll);
    if (!(n.goods || []).length) {
      goods.appendChild(node("p", "empty", "No goods described yet. Add a line."));
    }

    var addRow = node("div", "btnrow");
    var add = node("button", "btn ghost", "Add a goods line");
    add.type = "button";
    add.addEventListener("click", function () {
      n.goods = n.goods || [];
      n.goods.push({ description: "", packages: 0, packing: "", actualGrams: 0 });
      renderForm();
    });
    addRow.appendChild(add);
    goods.appendChild(addRow);

    var valueRow = node("div", "row2");
    numberField(valueRow, "Declared value", "the consignor's statement of what the goods are worth", "n-value",
      moneyInput(n.declaredValuePaise), function (v) { n.declaredValuePaise = parseMoney(v); }, "₹");
    textField(valueRow, "Declared by", null, "n-declared-by", n.valueDeclaredByName, function (v) { n.valueDeclaredByName = v; });
    goods.appendChild(valueRow);

    /* the freight */
    var freight = card(el.form, "The freight",
      "The term decides who the carrier can sue for the freight, whether anything is collected at the door, and which copy of the note shows what the carriage cost.");
    selectField(freight, "Freight term", null, "n-terms", ref.terms || [], n.terms.basis,
      function (v) { n.terms.basis = v; });

    var basisRow = node("div", "row3");
    selectField(basisRow, "Charged", null, "n-basis", ref.bases || [], n.charges.basis,
      function (v) { n.charges.basis = v; });
    numberField(basisRow, "Rate", null, "n-rate", moneyInput(n.charges.ratePaise),
      function (v) { n.charges.ratePaise = parseMoney(v); }, "₹");
    numberField(basisRow, "Minimum charged weight", "kg, where the carrier books a minimum", "n-min",
      kgInput(n.charges.minimumChargedGrams), function (v) { n.charges.minimumChargedGrams = parseKg(v); });
    freight.appendChild(basisRow);

    var override = node("label", "check");
    var overrideBox = node("input");
    overrideBox.type = "checkbox";
    overrideBox.checked = !!n.charges.freightOverridden;
    overrideBox.addEventListener("change", function () {
      n.charges.freightOverridden = overrideBox.checked;
      save().then(renderForm);
    });
    override.appendChild(overrideBox);
    override.appendChild(document.createTextNode(" The carriage was quoted as a figure, not worked out from a rate"));
    freight.appendChild(override);

    if (n.charges.freightOverridden) {
      numberField(freight, "Freight quoted", null, "n-quoted", moneyInput(n.charges.freightOverridePaise),
        function (v) { n.charges.freightOverridePaise = parseMoney(v); }, "₹");
    }

    var extras = node("div", "row3");
    numberField(extras, "Loading", null, "n-loading", moneyInput(n.charges.loadingPaise),
      function (v) { n.charges.loadingPaise = parseMoney(v); }, "₹");
    numberField(extras, "Unloading", null, "n-unloading", moneyInput(n.charges.unloadingPaise),
      function (v) { n.charges.unloadingPaise = parseMoney(v); }, "₹");
    numberField(extras, "Door delivery", null, "n-door", moneyInput(n.charges.doorDeliveryPaise),
      function (v) { n.charges.doorDeliveryPaise = parseMoney(v); }, "₹");
    freight.appendChild(extras);

    var extras2 = node("div", "row3");
    numberField(extras2, "Detention", null, "n-detention", moneyInput(n.charges.detentionPaise),
      function (v) { n.charges.detentionPaise = parseMoney(v); }, "₹");
    numberField(extras2, "Other charges", null, "n-other", moneyInput(n.charges.otherPaise),
      function (v) { n.charges.otherPaise = parseMoney(v); }, "₹");
    textField(extras2, "Called", null, "n-other-label", n.charges.otherLabel, function (v) { n.charges.otherLabel = v; }, "Statistical charge");
    freight.appendChild(extras2);

    var payRow = node("div", "row2");
    numberField(payRow, "Paid at booking", null, "n-advance", moneyInput(n.charges.advancePaise),
      function (v) { n.charges.advancePaise = parseMoney(v); }, "₹");
    freight.appendChild(payRow);

    if (n.terms.basis === "billed-to-third-party") {
      partyBlock(el.form, "Freight billed to", "The third party the freight is invoiced to. Nothing is collected at either end, so the note is the only record of who owes it.", n.billTo);
    }

    var working = node("table", "breakdown");
    var wbody = node("tbody");
    function line(label, value, note) {
      var tr = node("tr");
      var td = node("td", null, label);
      if (note) { td.appendChild(node("span", "on", note)); }
      tr.appendChild(td);
      tr.appendChild(node("td", "num", value));
      wbody.appendChild(tr);
    }
    line("Chargeable weight", kgInput(open.totals.chargeableGrams) + " kg",
      open.totals.minimumApplied ? "the carrier's minimum, not the actual weight" : null);
    line("Charged on", open.totals.unitsPrinted, null);
    line("Freight", money(open.totals.freightPaise),
      open.totals.freightFromRate ? null : "quoted for the consignment");
    line("Gross freight", money(open.totals.grossPaise), "the figure the tax position is worked out on");
    if (open.totals.advancePaise) { line("Balance", money(open.totals.balancePaise), null); }
    var wordsRow = node("tr", "sub");
    var wordsCell = node("td", null, open.totals.grossInWords);
    wordsCell.colSpan = 2;
    wordsRow.appendChild(wordsCell);
    wbody.appendChild(wordsRow);
    working.appendChild(wbody);
    freight.appendChild(working);

    /* the vehicle and driver */
    var carriage = card(el.form, "The vehicle and the driver",
      "The note is what says which vehicle carried these goods. The driver's details go on the carrier's copy and the proof of delivery, and not on the consignor's or the consignee's.");
    var vRow = node("div", "row3");
    textField(vRow, "Registration no.", null, "v-number", n.vehicle.number, function (v) { n.vehicle.number = v; }, "KA 25 C 4471");
    selectField(vRow, "Kind", null, "v-kind", ref.vehicleKinds || [], n.vehicle.kind, function (v) { n.vehicle.kind = v; }, "—");
    textField(vRow, "Owner", null, "v-owner", n.vehicle.owner, function (v) { n.vehicle.owner = v; });
    carriage.appendChild(vRow);

    var dRow = node("div", "row3");
    textField(dRow, "Driver", null, "dr-name", n.driver.name, function (v) { n.driver.name = v; });
    textField(dRow, "Licence no.", null, "dr-licence", n.driver.licenceNo, function (v) { n.driver.licenceNo = v; });
    textField(dRow, "Phone", null, "dr-phone", n.driver.phone, function (v) { n.driver.phone = v; });
    carriage.appendChild(dRow);

    /* the tax position */
    var tax = card(el.form, "The tax position on the carriage",
      "A consignment note is what makes this a goods transport agency's service, so this is where the tax question starts. Where it goes next depends on the election this agency has made and on the status of the recipient of the service, and this tool will not decide either. A note with no position stated prints that no position was stated.");
    selectField(tax, "Position", null, "tx-position", ref.positions || [], n.tax.position,
      function (v) { n.tax.position = v; });
    var txRow = node("div", "row2");
    textField(txRow, "How the recipient describes itself", "free text: picking from a list would imply it settles the law", "tx-class",
      n.tax.recipientClass, function (v) { n.tax.recipientClass = v; }, "a registered person under the Central Goods and Services Tax Act");
    numberField(txRow, "Rate for this note only", "leave blank to use the register's figure", "tx-rate",
      n.tax.rateOverride ? pctInput(n.tax.rateOverride) : "", function (v) { n.tax.rateOverride = parsePct(v); }, null, "%");
    tax.appendChild(txRow);
    areaField(tax, "Why no tax is charged", "only where that is the position; state the reason, because it is the reason that will have to stand up", "tx-reason",
      n.tax.reason, function (v) { n.tax.reason = v; });

    /* remarks */
    var extra = card(el.form, "Remarks", "Printed at the foot of every copy.");
    areaField(extra, "Remarks", null, "n-remarks", n.remarks, function (v) { n.remarks = v; });

    markFlagged();
  }

  function cell(build) {
    var td = node("td");
    build(td);
    return td;
  }

  // A finding names the particular it is about, so the field it is about is
  // marked rather than leaving the reader to hunt for it.
  var FIELD_IDS = {
    "date": "n-date", "from": "n-from", "to": "n-to",
    "consignor.name": "Consignor-name", "consignee.name": "Consignee-name",
    "consignor.gstin": "Consignor-gstin", "consignee.gstin": "Consignee-gstin",
    "consignor.pin": "Consignor-pin", "consignee.pin": "Consignee-pin",
    "billTo.name": "Freight billed to-name", "billTo.gstin": "Freight billed to-gstin",
    "vehicle.number": "v-number", "driver.name": "dr-name",
    "charges.ratePaise": "n-rate", "charges.advancePaise": "n-advance", "charges.basis": "n-basis",
    "terms.basis": "n-terms", "declaredValuePaise": "n-value", "valueDeclaredByName": "n-declared-by",
    "tax.position": "tx-position", "tax.reason": "tx-reason",
    "eWayBillNumber": "n-eway", "transitDays": "n-transit"
  };

  function markFlagged() {
    (open.report.findings || []).forEach(function (f) {
      if (f.severity === "note") { return; }
      var id = FIELD_IDS[f.field];
      if (!id) { return; }
      var input = document.getElementById(id);
      if (!input) { return; }
      var box = input.closest(".field");
      if (box) { box.classList.add("flagged"); }
    });
  }

  /* ---- the open note: the verdict panel ------------------------------ */

  function renderNote() {
    if (!open) { el["note-pane"].hidden = true; return; }
    el["note-pane"].hidden = false;
    say(el["action-problem"], "");

    el["note-number"].textContent = open.number;
    el["print-copies"].href = "print.html?number=" + encodeURIComponent(open.number);
    el["dl-findings"].href = "/api/findings.csv?number=" + encodeURIComponent(open.number);
    el.where.textContent = "This register is kept in " + (state.dataFile || "one file") +
      " on this machine. Nothing in it leaves the machine.";

    var report = open.report;
    el["verdict-box"].className = "result-primary " + (report.issuable ? "good" : "bad");
    el.verdict.textContent = report.issuable ? "Ready to issue" : "Not ready";
    el["verdict-sub"].textContent = report.verdict;

    /* the tracker */
    clear(el.tracker);
    var step = open.note.status === "delivered" ? 4
      : open.note.status === "arrived" ? 3
      : open.note.status === "in-transit" ? 2
      : open.note.status === "booked" ? 1 : 0;
    el.tracker.className = "tracker" + (step === 0 ? " off" : "");
    ["Booked", "In transit", "Arrived", "Delivered"].forEach(function (name, i) {
      var cls = i + 1 === step ? "on" : (i + 1 < step ? "past" : "");
      el.tracker.appendChild(node("span", cls, name));
    });
    if (step === 0) {
      el.tracker.appendChild(node("span", "on", open.note.status === "void" ? "Void" : "Cancelled"));
    }

    /* who owes the freight */
    clear(el.liable);
    el.liable.appendChild(node("b", null, open.liable.stamp + ". "));
    el.liable.appendChild(document.createTextNode(open.liable.explanation));

    /* the tax position */
    clear(el.tax);
    el.tax.appendChild(document.createTextNode(open.tax.statement));
    clear(el["tax-caveats"]);
    (open.tax.caveats || []).forEach(function (text) {
      el["tax-caveats"].appendChild(node("li", null, text));
    });

    /* what to look at */
    clear(el.findings);
    (report.findings || []).forEach(function (f) {
      var li = node("li", f.severity);
      li.appendChild(node("span", "what",
        f.severity === "blocker" ? "Has to be fixed" : f.severity === "check" ? "Worth a look" : "On the face of the note"));
      li.appendChild(document.createTextNode(f.what));
      li.appendChild(node("span", "fix", f.fix));
      el.findings.appendChild(li);
    });
    if (!(report.findings || []).length) {
      el.findings.appendChild(node("li", "note", "Nothing to look at."));
    }

    /* what may follow */
    clear(el.advance);
    (open.next || []).forEach(function (status) {
      if (status === "cancelled") { return; } // cancelling has its own block, with the reason
      var label = (((state.reference || {}).statuses || []).filter(function (s) { return s.value === status; })[0] || {}).name || status;
      if (status === "delivered") { return; }  // delivering has its own block, with the record
      var btn = node("button", null, label.split(" —")[0]);
      btn.type = "button";
      btn.addEventListener("click", function () {
        saving("moving it along…");
        request("POST", "/api/note/status?number=" + encodeURIComponent(open.number) + "&to=" + encodeURIComponent(status))
          .then(takeNote).catch(problem);
      });
      el.advance.appendChild(btn);
    });
    say(el["advance-note"], (open.next || []).length ? "" :
      "This note is " + open.note.status.replace("-", " ") + ", and nothing follows that. Its number stays where it is.");

    var terminal = ["delivered", "cancelled", "void"].indexOf(open.note.status) >= 0;
    el["deliver-wrap"].hidden = open.note.status === "cancelled" || open.note.status === "void";
    el["cancel-wrap"].hidden = terminal;
    el["renumber-wrap"].hidden = open.note.status === "void";

    /* the delivery form, pre-filled from whatever is recorded */
    var d = open.note.delivery || {};
    el["d-date"].value = d.date || "";
    el["d-packages"].value = d.packagesReceived || "";
    el["d-counted"].checked = !!d.packagesCounted;
    el["d-name"].value = d.receiverName || "";
    el["d-role"].value = d.receiverRole || "";
    el["d-collected"].value = moneyInput(d.freightCollectedPaise);
    el["d-id"].value = d.idShown || "";
    el["d-remarks"].value = d.remarks || "";
    options(el["d-condition"], (state.reference || {}).conditions || [], d.condition || "good-order");

    var c = open.note.cancellation || {};
    el["c-date"].value = c.date || state.today || "";
    el["c-reason"].value = c.reason || "";
    el["c-by"].value = c.by || "";
    el["r-seq"].value = "";

    renderForm();
  }

  /* ---- saving the open note ----------------------------------------- */

  function queueSave() {
    saving("…");
    if (saveTimer) { clearTimeout(saveTimer); }
    saveTimer = setTimeout(function () { save(); }, 400);
  }

  function save() {
    if (!open) { return Promise.resolve(); }
    if (saveTimer) { clearTimeout(saveTimer); saveTimer = null; }
    saving("saving…");

    // The fields are bound to this object: every handler the form built holds a
    // reference to it. Replacing it with the copy that comes back would leave
    // the form writing into something nobody reads any more, and the next
    // keystroke would be saved against a note that had already moved on. So the
    // answer's derived figures are taken and the note being edited is kept.
    var editing = open.note;

    return request("PUT", "/api/note", editing)
      .then(function (data) {
        data.note = editing;
        open = data;
        updateDerived();
        return reload();
      })
      .catch(function (err) { problem(err); });
  }

  // After a save the figures change but the fields do not, so only the derived
  // parts are redrawn. Rebuilding the form on every keystroke would take the
  // cursor out of the field being typed in.
  function updateDerived() {
    var report = open.report;
    el["verdict-box"].className = "result-primary " + (report.issuable ? "good" : "bad");
    el.verdict.textContent = report.issuable ? "Ready to issue" : "Not ready";
    el["verdict-sub"].textContent = report.verdict;

    clear(el.liable);
    el.liable.appendChild(node("b", null, open.liable.stamp + ". "));
    el.liable.appendChild(document.createTextNode(open.liable.explanation));

    clear(el.tax);
    el.tax.appendChild(document.createTextNode(open.tax.statement));
    clear(el["tax-caveats"]);
    (open.tax.caveats || []).forEach(function (text) {
      el["tax-caveats"].appendChild(node("li", null, text));
    });

    clear(el.findings);
    (report.findings || []).forEach(function (f) {
      var li = node("li", f.severity);
      li.appendChild(node("span", "what",
        f.severity === "blocker" ? "Has to be fixed" : f.severity === "check" ? "Worth a look" : "On the face of the note"));
      li.appendChild(document.createTextNode(f.what));
      li.appendChild(node("span", "fix", f.fix));
      el.findings.appendChild(li);
    });

    document.querySelectorAll(".field.flagged").forEach(function (box) { box.classList.remove("flagged"); });
    markFlagged();
  }

  /* ---- the office, the series and the figures ------------------------ */

  function renderOffice() {
    var c = state.carrier || {};
    el["o-name"].value = c.name || "";
    el["o-phone"].value = c.phone || "";
    el["o-address"].value = c.address || "";
    el["o-city"].value = c.city || "";
    el["o-state"].value = c.state || "";
    el["o-pin"].value = c.pin || "";
    el["o-gstin"].value = c.gstin || "";
    el["o-pan"].value = c.pan || "";
    el["o-terms"].value = c.terms || "";

    clear(el.books);
    (state.books || []).forEach(function (b) {
      var li = node("li");
      li.appendChild(node("span", "ref", b.series || "(unlabelled)"));
      li.appendChild(node("span", null, b.label || ""));
      li.appendChild(node("span", "who sp", "first number " + b.startAt + " · " + b.seqWidth + " digits"));
      el.books.appendChild(li);
    });

    var r = state.rates || {};
    el["t-rcm"].value = pctInput(r.recipientReverseCharge);
    el["t-fwd5"].value = pctInput(r.gtaForwardNoItc);
    el["t-fwd12"].value = pctInput(r.gtaForwardWithItc);
    el["t-exempt"].value = moneyInput(r.exemptSingleCarriageGross);
    el["t-apply"].checked = !!r.applyConsignmentExemption;

    clear(el.constants);
    var list = node("div", "consts");
    (state.constants || []).forEach(function (k) {
      var box = node("div", "c" + (k.applied ? "" : " unapplied"));
      box.appendChild(node("div", "n", k.name));
      box.appendChild(node("div", "v", k.value + (k.applied ? "" : "  (not applied)")));
      box.appendChild(node("div", "w", k.what));
      box.appendChild(node("div", "r", k.reference));
      box.appendChild(node("div", "caution", k.caution));
      list.appendChild(box);
    });
    el.constants.appendChild(list);
  }

  function carrierFromForm() {
    return {
      name: el["o-name"].value, address: el["o-address"].value, city: el["o-city"].value,
      state: el["o-state"].value, pin: el["o-pin"].value, gstin: el["o-gstin"].value,
      pan: el["o-pan"].value, phone: el["o-phone"].value, terms: el["o-terms"].value
    };
  }

  function saveCarrier() {
    saving("saving the office…");
    request("PUT", "/api/carrier", carrierFromForm())
      .then(takeState)
      .then(function () { if (open) { return loadNote(open.number); } })
      .catch(function (err) { say(el["office-problem"], err.message); });
  }

  /* ---- wiring -------------------------------------------------------- */

  // The theme toggle. The kit's own copy of this lives with the explainer
  // site; the embedded interface carries its own so that the binary needs one
  // script per page and no more.
  function themer() {
    var root = document.documentElement;
    var button = document.querySelector("[data-themer]");
    if (!button) { return; }

    function stored() { try { return localStorage.getItem("omsme-theme"); } catch (e) { return null; } }
    function current() {
      var set = root.getAttribute("data-theme");
      if (set) { return set; }
      return (window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches) ? "dark" : "light";
    }
    function paint() {
      var dark = current() === "dark";
      button.textContent = dark ? "\u2600 Light" : "\u263e Dark";
      button.setAttribute("aria-label", dark ? "Switch to the light theme" : "Switch to the dark theme");
    }

    paint();
    button.addEventListener("click", function () {
      var next = current() === "dark" ? "light" : "dark";
      root.setAttribute("data-theme", next);
      try { localStorage.setItem("omsme-theme", next); } catch (e) { /* a private window */ }
      paint();
    });
    if (window.matchMedia) {
      window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", function () {
        if (!stored()) { paint(); }
      });
    }
  }

  function wire() {
    themer();
    el["no-script"].hidden = true;

    el["new-note"].addEventListener("click", function () {
      var series = el["f-series"].value || ((state.books || [])[0] || {}).series || "";
      saving("allocating the next number…");
      request("POST", "/api/notes?series=" + encodeURIComponent(series))
        .then(function (data) {
          open = data;
          return reload().then(function () {
            renderNote();
            el["note-pane"].scrollIntoView({ block: "start" });
          });
        })
        .catch(problem);
    });

    el["load-sample"].addEventListener("click", function () {
      saving("loading the worked register…");
      request("POST", "/api/sample").then(function (data) {
        open = null;
        el["note-pane"].hidden = true;
        return takeState(data);
      }).catch(problem);
    });

    el["clear-register"].addEventListener("click", function () {
      if (!window.confirm("Clear every consignment note from the register? The office's details, the series and the figures are kept. The register is a plain file on this machine: copy it first if you want it back.")) { return; }
      if (!window.confirm("This cannot be undone from here. Clear the register?")) { return; }
      saving("clearing…");
      request("POST", "/api/clear").then(function (data) {
        open = null;
        el["note-pane"].hidden = true;
        return takeState(data);
      }).catch(problem);
    });

    ["f-text", "f-series", "f-status", "f-extra"].forEach(function (id) {
      el[id].addEventListener("input", renderRegister);
      el[id].addEventListener("change", renderRegister);
    });

    el["toggle-gaps"].addEventListener("click", function () {
      el["gap-form"].hidden = !el["gap-form"].hidden;
      if (!el["gap-form"].hidden) {
        el["gap-date"].value = state.today || "";
        var first = ((state.audit || {}).series || []).filter(function (s) { return s.gaps && s.gaps.length; })[0];
        if (first) {
          el["gap-series"].value = first.series;
          el["gap-seq"].value = first.gaps[0].from;
        }
      }
    });
    el["gap-cancel"].addEventListener("click", function () { el["gap-form"].hidden = true; });

    el["gap-form"].addEventListener("submit", function (ev) {
      ev.preventDefault();
      var seq = parseInt(el["gap-seq"].value, 10);
      if (!seq) { return; }
      saving("accounting for the number…");
      request("POST", "/api/void?series=" + encodeURIComponent(el["gap-series"].value) + "&seq=" + seq, {
        date: el["gap-date"].value, reason: el["gap-reason"].value, by: ""
      }).then(function () {
        el["gap-reason"].value = "";
        return reload();
      }).then(function () {
        var next = ((state.audit || {}).series || []).filter(function (s) { return s.gaps && s.gaps.length; })[0];
        if (next) {
          el["gap-series"].value = next.series;
          el["gap-seq"].value = next.gaps[0].from;
        } else {
          el["gap-form"].hidden = true;
        }
      }).catch(function (err) { problem(err); window.alert(err.message); });
    });

    el["do-deliver"].addEventListener("click", function () {
      if (!open) { return; }
      saving("recording the delivery…");
      request("POST", "/api/note/delivery?number=" + encodeURIComponent(open.number), {
        date: el["d-date"].value,
        receiverName: el["d-name"].value,
        receiverRole: el["d-role"].value,
        packagesReceived: Math.max(0, parseInt(el["d-packages"].value, 10) || 0),
        packagesCounted: el["d-counted"].checked,
        condition: el["d-condition"].value,
        remarks: el["d-remarks"].value,
        freightCollectedPaise: parseMoney(el["d-collected"].value),
        idShown: el["d-id"].value
      }).then(takeNote).catch(problem);
    });

    el["do-cancel"].addEventListener("click", function () {
      if (!open) { return; }
      saving("cancelling…");
      request("POST", "/api/note/cancel?number=" + encodeURIComponent(open.number), {
        date: el["c-date"].value, reason: el["c-reason"].value, by: el["c-by"].value
      }).then(takeNote).catch(problem);
    });

    el["do-renumber"].addEventListener("click", function () {
      if (!open) { return; }
      var seq = parseInt(el["r-seq"].value, 10);
      if (!seq) { say(el["action-problem"], "Which number is it being moved to?"); return; }
      saving("renumbering…");
      request("POST", "/api/note/renumber?number=" + encodeURIComponent(open.number) + "&to=" + seq)
        .then(takeNote).catch(problem);
    });

    ["o-name", "o-phone", "o-address", "o-city", "o-state", "o-pin", "o-gstin", "o-pan", "o-terms"].forEach(function (id) {
      el[id].addEventListener("change", saveCarrier);
    });

    el["save-book"].addEventListener("click", function () {
      say(el["book-problem"], "");
      saving("saving the series…");
      request("PUT", "/api/book", {
        series: el["b-series"].value,
        label: el["b-label"].value,
        startAt: Math.max(1, parseInt(el["b-start"].value, 10) || 1),
        seqWidth: Math.min(8, Math.max(1, parseInt(el["b-width"].value, 10) || 4))
      }).then(takeState).catch(function (err) { say(el["book-problem"], err.message); });
    });

    el["save-rates"].addEventListener("click", function () {
      say(el["rates-problem"], "");
      saving("saving the figures…");
      request("PUT", "/api/rates", {
        recipientReverseCharge: parsePct(el["t-rcm"].value),
        gtaForwardNoItc: parsePct(el["t-fwd5"].value),
        gtaForwardWithItc: parsePct(el["t-fwd12"].value),
        applyConsignmentExemption: el["t-apply"].checked,
        exemptSingleCarriageGross: parseMoney(el["t-exempt"].value)
      }).then(takeState)
        .then(function () { if (open) { return loadNote(open.number); } })
        .catch(function (err) { say(el["rates-problem"], err.message); });
    });

    el["reset-rates"].addEventListener("click", function () {
      saving("putting the shipped figures back…");
      request("PUT", "/api/rates", {
        recipientReverseCharge: 0, gtaForwardNoItc: 0, gtaForwardWithItc: 0,
        applyConsignmentExemption: false, exemptSingleCarriageGross: 0
      }).then(takeState)
        .then(function () { if (open) { return loadNote(open.number); } })
        .catch(function (err) { say(el["rates-problem"], err.message); });
    });
  }

  wire();
  reload().catch(function (err) {
    el["no-script"].hidden = false;
    el["no-script"].textContent = "The tool is not answering: " + err.message +
      ". Is it still running in the terminal you started it from?";
  });
})();
