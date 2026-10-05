"use strict";

// ---- tiny helpers ----------------------------------------------------------

const $ = (sel) => document.querySelector(sel);

async function getJSON(url) {
  const r = await fetch(url, { cache: "no-store" });
  if (!r.ok) throw new Error(url + " -> " + r.status);
  return r.json();
}

function fmtNum(v) {
  if (v == null || v === "") return "—";
  const n = Number(v);
  if (!Number.isFinite(n)) return String(v);
  return Number.isInteger(n) ? n.toLocaleString("fr-FR") : (Math.round(n * 100) / 100).toLocaleString("fr-FR");
}

function objCount(o) {
  if (!o || typeof o !== "object") return 0;
  return Object.values(o).reduce((a, b) => a + (Number(b) || 0), 0);
}

function fmtDate(ms) {
  if (!ms) return "—";
  return new Intl.DateTimeFormat("fr-FR", { dateStyle: "short", timeStyle: "short" }).format(new Date(ms));
}

function escapeHTML(v) {
  return String(v).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

// ---- state -----------------------------------------------------------------

const state = {
  summary: {},
  missions: [],
  sort: {},   // tableId -> { key, dir }
  filter: {}, // tableId -> string
};

// The plugin reads the manager's PostgreSQL database directly: no instance
// selector, no scope, no query parameter to add.
function rowsFor(kind) {
  const d = state.summary[kind];
  if (!d) return [];
  switch (kind) {
    case "pilots": return d.pilots || [];
    case "weapons": return d.weapons || [];
    case "engines": return d.engines || [];
    case "network": return d.network || [];
    default: return [];
  }
}

// ---- generic sortable/filterable table -------------------------------------

// cols: [{ key, label, num?, fmt?, html?, get? }]
function renderTable(tableId, rows, cols) {
  const table = document.getElementById(tableId);
  if (!table) return;

  const filter = (state.filter[tableId] || "").toLowerCase();
  const filtered = filter
    ? rows.filter((r) =>
        cols.some((c) => String((c.get ? c.get(r) : r[c.key]) ?? "").toLowerCase().includes(filter)))
    : rows;

  const s = state.sort[tableId];
  const sorted = s ? [...filtered].sort((a, b) => cmp(a, b, cols, s)) : filtered;

  const thead = "<tr>" + cols.map((c) => {
    const cls = [c.num ? "num" : "", "sortable",
      s && s.key === c.key ? (s.dir > 0 ? "sorted-asc" : "sorted-desc") : ""].join(" ").trim();
    return `<th class="${cls}" data-key="${c.key}">${c.label}</th>`;
  }).join("") + "</tr>";

  const tbody = sorted.map((r) =>
    "<tr>" + cols.map((c) => {
      const v = c.get ? c.get(r) : r[c.key];
      if (c.html) return `<td class="${c.num ? "num" : ""}">${c.fmt ? c.fmt(r) : ""}</td>`;
      const text = c.fmt ? c.fmt(r) : fmtNum(v);
      return `<td class="${c.num ? "num" : ""}">${escapeHTML(text)}</td>`;
    }).join("") + "</tr>").join("") ||
    `<tr><td colspan="${cols.length}" class="faint">aucune donnée</td></tr>`;

  table.innerHTML = `<thead>${thead}</thead><tbody>${tbody}</tbody>`;

  table.querySelectorAll("th.sortable").forEach((th) => {
    th.addEventListener("click", () => {
      const key = th.dataset.key;
      const cur = state.sort[tableId];
      state.sort[tableId] = cur && cur.key === key ? { key, dir: -cur.dir } : { key, dir: -1 };
      renderAll();
    });
  });
}

function cmp(a, b, cols, s) {
  const col = cols.find((c) => c.key === s.key);
  const va = col.get ? col.get(a) : a[col.key];
  const vb = col.get ? col.get(b) : b[col.key];
  const na = Number(va), nb = Number(vb);
  let r;
  if (Number.isFinite(na) && Number.isFinite(nb)) r = na - nb;
  else r = String(va ?? "").localeCompare(String(vb ?? ""));
  return r * s.dir;
}

// ---- column definitions ----------------------------------------------------

const pilotCols = [
  { key: "name", label: "Pilote" },
  { key: "ucid", label: "UCID", fmt: (r) => r.ucid || "—" },
  { key: "missions", label: "Missions", num: true },
  { key: "score", label: "Score", num: true },
  { key: "kills", label: "Kills", num: true },
  { key: "killsAir", label: "Air", num: true },
  { key: "killsCar", label: "Sol", num: true },
  { key: "killsShip", label: "Navire", num: true },
  { key: "deaths", label: "Morts", num: true },
  { key: "kd", label: "K/D", num: true },
  { key: "crashes", label: "Crashs", num: true },
  { key: "ejections", label: "Éjections", num: true },
  { key: "landings", label: "Atterrissages", num: true },
  { key: "friendlyFire", label: "FF", num: true },
  { key: "avgPing", label: "Ping moy.", num: true },
];

const weaponCols = [
  { key: "weapon", label: "Arme" },
  { key: "kills", label: "Kills", num: true },
  { key: "friendlyFire", label: "Fratricide", num: true },
  { key: "victims", label: "Types ciblés", num: true, get: (r) => objCount(r.victimsByType) },
  { key: "killers", label: "Plateformes", num: true, get: (r) => objCount(r.killersByType) },
];

const engineCols = [
  { key: "typeId", label: "Type DCS" },
  { key: "category", label: "Catégorie", html: true, fmt: (r) => `<span class="pill">${escapeHTML(r.category)}</span>` },
  { key: "kills", label: "Kills", num: true },
  { key: "deaths", label: "Pertes", num: true },
  { key: "sorties", label: "Sorties", num: true },
  { key: "kd", label: "K/D", num: true },
];

const networkCols = [
  { key: "name", label: "Pilote" },
  { key: "samples", label: "Échantillons", num: true },
  { key: "avgPing", label: "Ping moy.", num: true },
  { key: "maxPing", label: "Ping max", num: true },
];

const missionCols = [
  { key: "name", label: "Mission" },
  { key: "theatre", label: "Carte", fmt: (r) => r.theatre || "—" },
  { key: "source", label: "Source", html: true, fmt: (r) =>
      r.source === "test" ? `<span class="pill amber">test</span>` : `<span class="pill green">live</span>` },
  { key: "startedAt", label: "Début", fmt: (r) => fmtDate(r.startedAt) },
  { key: "endedAt", label: "Fin", fmt: (r) => (r.endedAt ? fmtDate(r.endedAt) : "—") },
  { key: "winner", label: "Vainqueur", html: true, fmt: (r) =>
      r.winner ? `<span class="pill cyan">${escapeHTML(r.winner)}</span>` : `<span class="faint">—</span>` },
];

// ---- overview --------------------------------------------------------------

function renderKpis(o) {
  const cards = [
    ["Missions", o.missions, "sessions enregistrées"],
    ["Pilotes", o.players, "identités UCID"],
    ["Kills", o.kills, "toutes coalitions"],
    ["Événements", o.events, "dans la base du manager"],
  ];
  $("#cards").innerHTML = cards.map(([k, v, d], i) =>
    `<div class="kpi k${i + 1}"><div class="k">${k}</div><div class="v">${fmtNum(v)}</div><div class="d">${d}</div></div>`
  ).join("");
}

function renderCoalitions(o) {
  const rows = o.coalitions || [];
  const total = rows.reduce((a, c) => a + (c.kills || 0), 0) || 1;
  const el = $("#coalition-bars");
  el.innerHTML = rows.length
    ? rows.map((c) => {
        const pct = Math.round((100 * (c.kills || 0)) / total);
        const red = c.coalition === "red";
        const label = c.coalition === "blue" ? "Bleu" : red ? "Rouge" : (c.coalition || "—");
        return `<div class="barrow"><span class="pill ${red ? "red" : "cyan"}">${escapeHTML(label)}</span>` +
          `<span class="bar ${red ? "red" : ""}"><i style="width:${pct}%"></i></span>` +
          `<span class="num">${pct} %</span></div>`;
      }).join("")
    : `<div class="faint">aucune donnée</div>`;
  const kills = rows.reduce((a, c) => a + (c.kills || 0), 0);
  const score = rows.reduce((a, c) => a + (c.score || 0), 0);
  $("#coalition-note").textContent = `${fmtNum(kills)} kills · score ${fmtNum(score)}`;
}

function renderOverview() {
  const o = state.summary.overview || {};
  renderKpis(o);
  renderCoalitions(o);
}

// ---- chart -----------------------------------------------------------------

function drawChart(svg, points) {
  if (!svg) return;
  const vb = (svg.getAttribute("viewBox") || "").split(/[\s,]+/).map(Number);
  const W = vb[2] || 620;
  const H = vb[3] || 220;
  const padL = 46, padR = 14, padT = 16, padB = 30;

  if (!points || points.length < 2) {
    svg.innerHTML = `<text x="${padL}" y="${H / 2}" class="axis-label">aucun événement sur la fenêtre</text>`;
    return;
  }

  const ys = points.map((p) => Number(p.value) || 0);
  const max = Math.max(1, ...ys);
  const X = (i) => padL + (W - padL - padR) * (i / (points.length - 1));
  const Y = (v) => H - padB - (H - padT - padB) * (v / max);

  let g = "";
  for (let j = 0; j <= 4; j++) {
    const v = (max * j) / 4, y = Y(v);
    g += `<line class="grid-line" x1="${padL}" y1="${y.toFixed(1)}" x2="${W - padR}" y2="${y.toFixed(1)}"/>`;
    g += `<text class="axis-label" x="${padL - 6}" y="${(y + 3).toFixed(1)}" text-anchor="end">${Math.round(v)}</text>`;
  }

  const line = points.map((p, i) => `${i ? "L" : "M"}${X(i).toFixed(1)},${Y(ys[i]).toFixed(1)}`).join(" ");
  const area = `${line} L${X(points.length - 1).toFixed(1)},${(H - padB).toFixed(1)} L${X(0).toFixed(1)},${(H - padB).toFixed(1)} Z`;

  const fmtDay = (t) => {
    const d = new Date(t);
    return `${String(d.getDate()).padStart(2, "0")}/${String(d.getMonth() + 1).padStart(2, "0")}`;
  };
  let xl = "";
  [0, Math.floor((points.length - 1) / 2), points.length - 1].forEach((idx, j) => {
    const anchor = j === 0 ? "start" : j === 2 ? "end" : "middle";
    xl += `<text class="axis-label" x="${X(idx).toFixed(1)}" y="${H - 8}" text-anchor="${anchor}">${fmtDay(points[idx].at)}</text>`;
  });

  const grad = "gain";
  svg.innerHTML =
    `<defs><linearGradient id="${grad}" x1="0" y1="0" x2="0" y2="1">` +
    `<stop offset="0" stop-color="#39d0d8" stop-opacity=".30"/>` +
    `<stop offset="1" stop-color="#39d0d8" stop-opacity="0"/></linearGradient></defs>` +
    g +
    `<path d="${area}" fill="url(#${grad})"/>` +
    `<path d="${line}" fill="none" stroke="#39d0d8" stroke-width="2.2"/>` +
    xl;
}

async function loadOverviewSeries() {
  const s = await getJSON("/api/plugin/series?event=kill&days=30");
  const points = s.points || [];
  drawChart($("#chart-overview"), points);
  const total = points.reduce((a, p) => a + p.value, 0);
  const last7 = points.slice(-7).reduce((a, p) => a + p.value, 0);
  $("#overview-chart-info").textContent = `${fmtNum(total)} kills sur 30 jours · ${fmtNum(last7)} sur 7 jours`;
}

// ---- trends ----------------------------------------------------------------

async function loadSeries() {
  const event = $("#event").value;
  const days = $("#days").value;
  const s = await getJSON(`/api/plugin/series?event=${encodeURIComponent(event)}&days=${days}`);
  const points = s.points || [];
  drawChart($("#chart-trends"), points);
  const total = points.reduce((a, p) => a + p.value, 0);
  const label = event || "tous événements";
  $("#chart-info").textContent = points.length
    ? `${label} · ${fmtNum(total)} événements sur ${days} jours`
    : `${label} · aucun événement sur ${days} jours`;
  syncExportLinks();
}

// ---- health & footer -------------------------------------------------------

async function loadHealth() {
  const dot = $("#pg-dot");
  try {
    const h = await getJSON("/api/plugin/health");
    const c = h.counts || {};
    dot.classList.add("on");
    $("#pg-status").textContent = `PostgreSQL · lecture seule${h.includeTest ? " · test inclus" : ""}`;
    $("#foot-missions").textContent = fmtNum(c.missions);
    $("#foot-players").textContent = fmtNum(c.players);
    $("#foot-events").textContent = fmtNum(c.events);
  } catch (e) {
    dot.classList.remove("on");
    $("#pg-status").textContent = "base inaccessible";
    $("#foot-missions").textContent = "—";
    $("#foot-players").textContent = "—";
    $("#foot-events").textContent = "—";
  }
}

async function loadSummary() {
  state.summary = await getJSON("/api/plugin/summary");
  renderAll();
}

async function loadMissions() {
  const d = await getJSON("/api/plugin/missions?limit=500");
  state.missions = d.missions || [];
  renderTable("missions", state.missions, missionCols);
}

// ---- render / views --------------------------------------------------------

function renderAll() {
  renderOverview();
  renderTable("pilots", rowsFor("pilots"), pilotCols);
  renderTable("weapons", rowsFor("weapons"), weaponCols);
  renderTable("engines", rowsFor("engines"), engineCols);
  renderTable("network", rowsFor("network"), networkCols);
  renderTable("missions", state.missions, missionCols);
}

const TITLES = {
  overview: ["Vue d'ensemble", "Instantané le plus récent · carrière"],
  trends: ["Tendances", "Séries temporelles sur les événements du manager"],
  pilots: ["Pilotes", "Classement de carrière, par UCID"],
  weapons: ["Armes", "Efficacité par arme"],
  engines: ["Machines", "Type DCS exact"],
  network: ["Réseau", "Qualité de connexion"],
  missions: ["Missions", "Historique des sessions"],
  export: ["Export", "CSV / JSON"],
};

function selectView(name) {
  document.querySelectorAll("#nav .nav-item").forEach((b) =>
    b.classList.toggle("active", b.dataset.view === name));
  document.querySelectorAll("[data-pane]").forEach((p) =>
    p.classList.toggle("hidden", p.dataset.pane !== name));
  const t = TITLES[name] || ["", ""];
  $("#title").textContent = t[0];
  $("#subtitle").textContent = t[1];
  if (name === "trends") loadSeries().catch(() => {});
  if (name === "missions") loadMissions().catch(() => {});
  if (name === "export") syncExportLinks();
}

// ---- export ----------------------------------------------------------------

function syncExportLinks() {
  const m = $("#missions-csv");
  if (m) m.href = "/api/plugin/export?type=missions&format=csv";
  const s = $("#series-json");
  if (s) s.href = `/api/plugin/export?type=series&event=${encodeURIComponent($("#event").value)}&format=json`;
}

function runExport() {
  const type = $("#export-type").value;
  const format = $("#export-format").value;
  let url = `/api/plugin/export?type=${type}&format=${format}`;
  if (type === "series") url += `&event=${encodeURIComponent($("#event").value)}`;
  window.location.href = url;
}

// ---- wiring ----------------------------------------------------------------

function reload() {
  loadHealth();
  loadSummary().catch(() => {});
  loadOverviewSeries().catch(() => {});
  if (!$('[data-pane="missions"]').classList.contains("hidden")) loadMissions().catch(() => {});
  if (!$('[data-pane="trends"]').classList.contains("hidden")) loadSeries().catch(() => {});
  syncExportLinks();
  $("#updated").textContent = "maj " + new Date().toLocaleTimeString("fr-FR");
}

function bind() {
  $("#nav").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-view]");
    if (b) selectView(b.dataset.view);
  });
  $("#refresh").addEventListener("click", reload);
  $("#event").addEventListener("change", () => loadSeries().catch(() => {}));
  $("#days").addEventListener("change", () => loadSeries().catch(() => {}));
  $("#export-run").addEventListener("click", runExport);

  const bindFilter = (id, tableId) => {
    const el = document.getElementById(id);
    if (el) el.addEventListener("input", () => { state.filter[tableId] = el.value; renderAll(); });
  };
  bindFilter("pilots-filter", "pilots");
  bindFilter("weapons-filter", "weapons");
  bindFilter("engines-filter", "engines");
  bindFilter("missions-filter", "missions");
}

bind();
reload();
setInterval(reload, 30000);
