/* The copy set, laid out as documents.

   Every word and figure on these sheets comes from the tool. The page chooses
   nothing: which particulars belong on which copy, what each copy is for, which
   stamp it carries and what the proof of delivery does and does not evidence
   are all decided in the binary and tested there. This file is a renderer. */

(function () {
  "use strict";

  var sheets = document.getElementById("p-sheets");
  var picker = document.getElementById("p-picker");
  var problem = document.getElementById("p-problem");
  var withheld = document.getElementById("p-withheld");
  var caveatsWrap = document.getElementById("p-caveats-wrap");
  var caveats = document.getElementById("p-caveats");

  var ALL = "all";
  var shown = ALL;
  var built = [];

  function node(tag, className, text) {
    var n = document.createElement(tag);
    if (className) { n.className = className; }
    if (text != null) { n.textContent = text; }
    return n;
  }

  function clear(parent) { while (parent.firstChild) { parent.removeChild(parent.firstChild); } }

  function say(target, message) {
    target.textContent = message || "";
    target.hidden = !message;
  }

  function numberFromURL() {
    var params = new URLSearchParams(window.location.search);
    return (params.get("number") || "").trim();
  }

  /* ---- one copy ------------------------------------------------------ */

  function partyBlock(block) {
    var box = node("div", "party");
    box.appendChild(node("div", "label", block.label));
    (block.lines || []).forEach(function (line) {
      box.appendChild(node("div", "line", line));
    });
    if (!(block.lines || []).length) {
      box.appendChild(node("div", "line", "—"));
    }
    if (block.note) { box.appendChild(node("div", "pnote", block.note)); }
    return box;
  }

  function metaTable(fields) {
    var table = node("table", "meta");
    var body = node("tbody");
    (fields || []).forEach(function (f) {
      if (!f.value) { return; }
      var tr = node("tr");
      tr.appendChild(node("td", "k", f.label));
      var td = node("td", "v" + (f.strong ? " strong" : ""), f.value);
      if (f.note) { td.appendChild(node("span", "n", f.note)); }
      tr.appendChild(td);
      body.appendChild(tr);
    });
    table.appendChild(body);
    return table;
  }

  function goodsTable(copy) {
    var wrap = node("div", "scroller");
    var table = node("table", "acct");

    var head = node("thead");
    var hrow = node("tr");
    [["#", "centre"], ["Packages", "right"], ["Packing", ""],
     ["Description of the goods (said to contain)", ""], ["Weight", "right"]].forEach(function (cell) {
      hrow.appendChild(node("th", cell[1], cell[0]));
    });
    head.appendChild(hrow);
    table.appendChild(head);

    var body = node("tbody");
    (copy.goods || []).forEach(function (row) {
      var tr = node("tr");
      tr.appendChild(node("td", "centre", String(row.no)));
      tr.appendChild(node("td", "right", row.packages));
      tr.appendChild(node("td", null, row.packing || "—"));
      tr.appendChild(node("td", "wrap", row.description));
      tr.appendChild(node("td", "right", row.weight));
      body.appendChild(tr);
    });
    if (!(copy.goods || []).length) {
      var empty = node("tr");
      var td = node("td", "muted", "No goods are described on this note.");
      td.colSpan = 5;
      empty.appendChild(td);
      body.appendChild(empty);
    }
    table.appendChild(body);

    var foot = node("tfoot");
    var frow = node("tr", "strong");
    frow.appendChild(node("td", "centre", ""));
    frow.appendChild(node("td", "right", copy.goodsTotal.packages));
    frow.appendChild(node("td", null, ""));
    frow.appendChild(node("td", null, "Total"));
    frow.appendChild(node("td", "right", copy.goodsTotal.weight));
    foot.appendChild(frow);
    table.appendChild(foot);

    wrap.appendChild(table);
    return wrap;
  }

  function moneyBlock(copy) {
    var section = node("section");
    section.appendChild(node("h2", null, copy.kind === "proof-of-delivery" ? "The freight" : "Freight and charges"));

    var holder = node("div", "totals");
    var table = node("table", "totals-table");
    var body = node("tbody");
    (copy.freight || []).forEach(function (line) {
      var tr = node("tr", (line.rule ? "rule " : "") + (line.strong ? "strong" : ""));
      var k = node("td", "k", line.label);
      if (line.note) { k.appendChild(node("span", "n", line.note)); }
      tr.appendChild(k);
      tr.appendChild(node("td", "v", line.value === "not stated" || line.value === "Nil" ? line.value : "₹" + line.value));
      body.appendChild(tr);
    });
    table.appendChild(body);
    holder.appendChild(table);

    if (copy.freightWords) {
      var words = node("div", "words");
      words.appendChild(node("b", null, "In words"));
      words.appendChild(document.createTextNode(copy.freightWords));
      holder.appendChild(words);
    }
    if (copy.amountToCollect && copy.kind === "consignee") {
      var collect = node("div", "collect");
      collect.appendChild(node("b", null, "To be paid before the goods are released"));
      collect.appendChild(document.createTextNode(copy.amountToCollect));
      holder.appendChild(collect);
    }
    section.appendChild(holder);
    return section;
  }

  function blankPOD(copy) {
    var section = node("section");
    section.appendChild(node("h2", null, "Delivery — to be completed at the destination"));

    var grid = node("div", "podgrid");
    [["Date of delivery", ""], ["Packages received", ""],
     ["Received by (name)", ""], ["In what capacity", ""],
     ["Identification shown", ""], ["Freight collected (₹)", ""]].forEach(function (pair) {
      var box = node("div", "podbox");
      box.appendChild(node("div", "label", pair[0]));
      box.appendChild(node("span", "blank"));
      grid.appendChild(box);
    });
    section.appendChild(grid);

    var conditions = node("div", "conditions");
    (copy.conditions || []).forEach(function (text) {
      conditions.appendChild(node("span", null, text));
    });
    section.appendChild(conditions);

    var remarks = node("div", "podbox");
    remarks.appendChild(node("div", "label", "Remarks — required if anything was short, damaged or refused"));
    remarks.appendChild(node("span", "blank"));
    remarks.appendChild(node("span", "blank"));
    section.appendChild(remarks);
    return section;
  }

  function filledPOD(pod) {
    var section = node("section");
    section.appendChild(node("h2", null, "The delivery record"));

    section.appendChild(node("div", "pod-verdict " + (pod.clean ? "clean" : "qualified"),
      pod.clean ? "A clean proof of delivery" : "A qualified proof of delivery — read the remarks"));

    var grid = node("div", "podgrid");
    function box(label, value) {
      if (!value) { return; }
      var b = node("div", "podbox");
      b.appendChild(node("div", "label", label));
      b.appendChild(node("div", "val", value));
      grid.appendChild(b);
    }
    box("Date of delivery", pod.deliveredLong || "not recorded");
    box("Received by", pod.receivedBy || "nobody named");
    box("In what capacity", pod.receiverRole);
    box("Identification shown", pod.idShown);
    box("Packages", pod.packagesCounted
      ? pod.packagesReceived + " of " + pod.packagesBooked + " received"
      : "not counted at delivery");
    box("Condition", pod.condition);
    if (pod.shortage) { box("Short by", pod.shortage + " package(s)"); }
    if (pod.daysLate) { box("Days late", String(pod.daysLate)); }
    if (pod.remarks) { box("Remarks", pod.remarks); }
    section.appendChild(grid);

    var evidence = node("div", "evidence");
    if ((pod.evidences || []).length) {
      var good = node("div", "blk");
      good.appendChild(node("b", null, "This document evidences"));
      var list = node("ul", "plain");
      pod.evidences.forEach(function (text) { list.appendChild(node("li", null, text)); });
      good.appendChild(list);
      evidence.appendChild(good);
    }
    if ((pod.doesNotEvidence || []).length) {
      var bad = node("div", "blk");
      bad.appendChild(node("b", null, "It does not evidence"));
      var list2 = node("ul", "plain quiet");
      pod.doesNotEvidence.forEach(function (text) { list2.appendChild(node("li", null, text)); });
      bad.appendChild(list2);
      evidence.appendChild(bad);
    }
    section.appendChild(evidence);
    return section;
  }

  // The theme toggle. The kit's own copy of this lives with the explainer
  // site; the embedded interface carries its own so that the binary needs one
  // script per page and no more.
  (function themer() {
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
      button.textContent = dark ? "☀ Light" : "☾ Dark";
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
  })();

  function render(copy) {
    var article = node("article");

    /* the heading */
    var head = node("header", "dochead");
    head.appendChild(node("div", "holder", copy.for));
    head.appendChild(node("h1", null, copy.title));
    head.appendChild(node("p", "purpose", copy.purpose));

    var stamps = node("div", "stamps");
    stamps.appendChild(node("div", "stamp", copy.freightTag));
    stamps.appendChild(node("div", "stamp quiet", copy.taxEndorsement));
    head.appendChild(stamps);

    var parties = node("div", "parties");
    (copy.parties || []).forEach(function (block) { parties.appendChild(partyBlock(block)); });
    head.appendChild(parties);

    head.appendChild(metaTable(copy.reference));
    article.appendChild(head);

    /* the goods */
    var goodsSection = node("section");
    goodsSection.appendChild(node("h2", null, "The goods"));
    goodsSection.appendChild(goodsTable(copy));
    var saidTo = node("ul", "plain quiet");
    saidTo.appendChild(node("li", null, copy.saidToContain));
    goodsSection.appendChild(saidTo);

    if (copy.declaredValue) {
      var declared = node("div", "words");
      declared.appendChild(node("b", null, "Value declared by the consignor"));
      declared.appendChild(document.createTextNode(copy.declaredValue + " — " + copy.declaredValueWords));
      goodsSection.appendChild(declared);
    }
    article.appendChild(goodsSection);

    /* the carriage */
    var carriage = node("section");
    carriage.appendChild(node("h2", null, "The carriage"));
    var carriageFields = [{ label: "Vehicle", value: copy.vehicleLine, strong: true }];
    if (copy.driverLine) { carriageFields.push({ label: "Driver", value: copy.driverLine }); }
    carriage.appendChild(metaTable(carriageFields));
    article.appendChild(carriage);

    /* the money */
    if ((copy.freight || []).length) { article.appendChild(moneyBlock(copy)); }

    /* the delivery */
    if (copy.podBlank) { article.appendChild(blankPOD(copy)); }
    if (copy.pod) { article.appendChild(filledPOD(copy.pod)); }

    /* the notes */
    if ((copy.notes || []).length) {
      var notesSection = node("section");
      notesSection.appendChild(node("h2", null, "On the face of this copy"));
      var list = node("ul", "plain quiet");
      copy.notes.forEach(function (text) { list.appendChild(node("li", null, text)); });
      notesSection.appendChild(list);
      article.appendChild(notesSection);
    }

    /* the signatures */
    var signatures = node("section", "signatures");
    (copy.signatures || []).forEach(function (block) {
      var sig = node("div", "sig");
      sig.appendChild(node("div", "line"));
      sig.appendChild(node("div", "for", block.for));
      sig.appendChild(node("div", "who", block.who));
      if (block.when) { sig.appendChild(node("div", "when", block.when)); }
      signatures.appendChild(sig);
    });
    article.appendChild(signatures);

    var foot = node("footer", "docfoot");
    foot.appendChild(node("p", null, copy.foot + " Prepared on this machine; not filed anywhere."));
    article.appendChild(foot);

    return article;
  }

  /* ---- the picker ---------------------------------------------------- */

  function show(kind) {
    shown = kind;
    built.forEach(function (entry) {
      entry.article.hidden = !(shown === ALL || shown === entry.kind);
    });
    Array.prototype.forEach.call(picker.children, function (btn) {
      btn.setAttribute("aria-pressed", btn.dataset.kind === shown ? "true" : "false");
    });
  }

  function buildPicker() {
    clear(picker);
    function add(kind, label) {
      var btn = node("button", null, label);
      btn.type = "button";
      btn.dataset.kind = kind;
      btn.addEventListener("click", function () { show(kind); });
      picker.appendChild(btn);
    }
    add(ALL, "All of them, one per page");
    built.forEach(function (entry) { add(entry.kind, entry.label); });
  }

  /* ---- loading -------------------------------------------------------- */

  var number = numberFromURL();
  if (!number) {
    say(problem, "No consignment note was named. Go back to the register and open one.");
    return;
  }

  fetch("/api/copies?number=" + encodeURIComponent(number), { headers: { "Accept": "application/json" } })
    .then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (data) {
        if (!res.ok) { throw new Error(data.error || ("the tool answered " + res.status)); }
        return data;
      });
    })
    .then(function (data) {
      var set = data.set || {};
      document.title = "Consignment note " + (set.number || number) + " · Open MSME";

      clear(sheets);
      built = (set.copies || []).map(function (copy) {
        var article = render(copy);
        sheets.appendChild(article);
        return { kind: copy.kind, label: copy.for, article: article };
      });

      if (!built.length) {
        say(problem, "There is nothing to print for " + number + " yet.");
        return;
      }

      buildPicker();
      show(ALL);
      // The lede explains an empty page; once there are sheets on it, it is in
      // the way of them.
      document.getElementById("p-lede").hidden = true;

      var missing = Object.keys(set.withheld || {});
      if (missing.length) {
        say(withheld, missing.map(function (kind) { return set.withheld[kind]; }).join(" "));
      }

      if ((data.caveats || []).length) {
        caveatsWrap.hidden = false;
        clear(caveats);
        data.caveats.forEach(function (text) { caveats.appendChild(node("li", null, text)); });
      }
    })
    .catch(function (err) {
      say(problem, "Could not load " + number + ": " + err.message +
        ". Is the tool still running in the terminal you started it from?");
    });

  document.getElementById("p-print").addEventListener("click", function () { window.print(); });
})();
