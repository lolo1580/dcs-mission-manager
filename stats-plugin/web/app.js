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
  return Number.isInteger(n) ? n.toLocaleString() : (Math.round(n * 100) / 100).toLocaleString();
}

function objCount(o) {
  if (!o || typeof o !== "object") return 0;
  return Object.values(o).reduce((a, b) => a + (Number(b) || 0), 0);
}

// ---- state -----------------------------------------------------------------

const state = {
  scope: "career",
  instance: "",
  instances: [],
  summary: {},
  missions: [],
  sort: {}, // tableId -> { key, dir }
  filter: {}, // tableId -> string
};

// Every API call carries the selected instance.
function api(path) {
  const sep = path.includes("?") ? "&" : "?";
  return state.instance ? path + sep + "instance=" + encodeURIComponent(state.instance) : path;
}

// ---- API mapping -----------------------------------------------------------

// The manager returns arrays under a named key: pilots, weapons, engines,
// network; overview is a flat object.
function rowsFor(kind) {
  const d = dataFor(kind);
  if (!d) return [];
  switch (kind) {
    case "pilots": return d.pilots || [];
    case "weapons": return d.weapons || [];
    case "engines": return d.engines || [];
    case "network": return d.network || [];
    default: return [];
  }
}

function dataFor(kind) {
  const key = state.scope === "career" ? kind : kind + ":" + state.scope;
  return state.summary[key];
}

// ---- generic sortable/filterable table -------------------------------------

// cols: [{ key, label, num?, get?, filterText? }]
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
      const text = c.fmt ? c.fmt(r) : fmtNum(v);
      return `<td class="${c.num ? "num" : ""}">${escapeHTML(text)}</td>`;
    }).join("") + "</tr>").join("") ||
    `<tr><td colspan="${cols.length}" class="muted">no data</td></tr>`;

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

function escapeHTML(v) {
  return String(v).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

// ---- column definitions ----------------------------------------------------

const pilotCols = [
  { key: "name", label: "Pilot" },
  { key: "missions", label: "Missions", num: true },
  { key: "score", label: "Score", num: true },
  { key: "kills", label: "Kills", num: true },
  { key: "killsAir", label: "Air", num: true },
  { key: "killsCar", label: "Ground", num: true },
  { key: "killsShip", label: "Ship", num: true },
  { key: "deaths", label: "Deaths", num: true },
  { key: "kd", label: "K/D", num: true },
  { key: "crashes", label: "Crashes", num: true },
  { key: "ejections", label: "Ejections", num: true },
  { key: "landings", label: "Landings", num: true },
  { key: "friendlyFire", label: "FF", num: true },
  { key: "avgPing", label: "Avg ping", num: true },
];

const weaponCols = [
  { key: "weapon", label: "Weapon" },
  { key: "kills", label: "Kills", num: true },
  { key: "friendlyFire", label: "Friendly fire", num: true },
  { key: "victims", label: "Victims", num: true, get: (r) => objCount(r.victimsByType) },
  { key: "killers", label: "Platforms", num: true, get: (r) => objCount(r.killersByType) },
];

const engineCols = [
  { key: "typeId", label: "DCS type" },
  { key: "category", label: "Category" },
  { key: "kills", label: "Kills", num: true },
  { key: "deaths", label: "Losses", num: true },
  { key: "sorties", label: "Sorties", num: true },
  { key: "kd", label: "K/D", num: true },
];

const networkCols = [
  { key: "name", label: "Pilot" },
  { key: "samples", label: "Samples", num: true },
  { key: "avgPing", label: "Avg ping", num: true },
  { key: "maxPing", label: "Max ping", num: true },
];

const missionCols = [
  { key: "name", label: "Mission" },
  { key: "theatre", label: "Theatre" },
  { key: "source", label: "Source" },
  { key: "startedAt", label: "Started", fmt: (r) => new Date(r.startedAt).toLocaleString() },
  { key: "endedAt", label: "Ended", fmt: (r) => (r.endedAt ? new Date(r.endedAt).toLocaleString() : "—") },
  { key: "winner", label: "Winner" },
];

// ---- views -----------------------------------------------------------------

function renderOverview() {
  const o = dataFor("overview") || {};
  const cards = [
    ["Missions", o.missions], ["Players", o.players], ["Events", o.events],
    ["Kills", o.kills], ["Deaths", o.deaths], ["Crashes", o.crashes],
    ["Ejections", o.ejections], ["Friendly fire", o.friendlyFire],
  ];
  $("#cards").innerHTML = cards.map(([k, v]) =>
    `<div class="card"><div class="k">${k}</div><div class="v">${fmtNum(v)}</div></div>`).join("");

  const cols = [
    { key: "coalition", label: "Coalition" },
    { key: "score", label: "Score", num: true },
    { key: "kills", label: "Kills", num: true },
    { key: "players", label: "Players", num: true },
  ];
  const rows = o.coalitions || [];
  const t = $("#coalitions");
  t.innerHTML =
    "<thead><tr>" + cols.map((c) => `<th class="${c.num ? "num" : ""}">${c.label}</th>`).join("") + "</tr></thead>" +
    "<tbody>" + (rows.map((r) =>
      "<tr>" + cols.map((c) => `<td class="${c.num ? "num" : ""}">${escapeHTML(fmtNum(r[c.key]))}</td>`).join("") + "</tr>"
    ).join("") || `<tr><td colspan="${cols.length}" class="muted">no data</td></tr>`) + "</tbody>";
}

function renderAll() {
  renderOverview();
  renderTable("pilots", rowsFor("pilots"), pilotCols);
  renderTable("weapons", rowsFor("weapons"), weaponCols);
  renderTable("engines", rowsFor("engines"), engineCols);
  renderTable("network", rowsFor("network"), networkCols);
  renderTable("missions", state.missions, missionCols);
}

// ---- health & header -------------------------------------------------------

async function loadHealth() {
  try {
    const h = await getJSON(api("/api/plugin/health"));
    const ok = h.manager && h.manager.service === "dcsmanager";
    const c = h.counts || {};
    $("#meta").innerHTML =
      `manager <span class="status ${ok ? "ok" : "ko"}">${ok ? "reachable" : "unreachable"}</span>` +
      ` · <span class="muted">${h.managerUrl}</span>` +
      (h.lastSync ? ` · last sync ${new Date(h.lastSync.at).toLocaleString()}` : " · no sync yet") +
      ` · <span class="muted">${fmtNum(c.events)} events, ${fmtNum(c.missions)} missions mirrored</span>`;
    if (Array.isArray(h.instances) && h.instances.length) setInstances(h.instances);
  } catch (e) {
    $("#meta").textContent = String(e);
  }
}

function setInstances(list) {
  const same = list.length === state.instances.length && list.every((v, i) => v === state.instances[i]);
  if (same) return;
  state.instances = list;
  if (!state.instance) state.instance = list[0];
  const sel = $("#instance");
  sel.innerHTML = list.map((n) =>
    `<option value="${escapeHTML(n)}"${n === state.instance ? " selected" : ""}>${escapeHTML(n)}</option>`).join("");
}

async function loadSummary() {
  state.summary = await getJSON(api("/api/plugin/summary"));
  renderAll();
}

async function loadMissions() {
  const d = await getJSON(api("/api/plugin/missions?limit=500"));
  state.missions = d.missions || [];
  renderTable("missions", state.missions, missionCols);
}

// ---- chart -----------------------------------------------------------------

let series = [];

async function loadSeries() {
  const metric = $("#metric").value;
  const s = await getJSON(api(`/api/plugin/series?metric=${metric}&scope=${state.scope}`));
  series = s.points || [];
  drawChart(series, metric);
  syncExportLinks();
}

// syncExportLinks keeps the Trend tab's JSON export and the Missions tab's CSV
// export pointing at the current selection.
function syncExportLinks() {
  const metric = $("#metric") ? $("#metric").value : "kills";
  const m = $("#missions-csv");
  if (m) m.href = api("/api/plugin/export?type=missions&format=csv");
  const s = $("#series-json");
  if (s) s.href = api(`/api/plugin/export?type=series&metric=${metric}&scope=${state.scope}&format=json`);
}

function drawChart(points, metric) {
  const svg = $("#chart");
  const W = 800, H = 220, pad = 28;
  $("#chart-info").textContent = points.length
    ? `${points.length} snapshot(s) of “${metric}” (${state.scope})`
    : "";
  if (points.length < 2) {
    svg.innerHTML = `<text x="20" y="120" fill="#8b95a7" font-size="13">not enough snapshots yet — one accumulates on each sync where the value changes</text>`;
    return;
  }
  const ys = points.map((p) => p.value);
  const min = Math.min(...ys), max = Math.max(...ys);
  const span = max - min || 1;
  const X = (i) => pad + (W - 2 * pad) * (i / (points.length - 1));
  const Y = (v) => H - pad - (H - 2 * pad) * ((v - min) / span);
  const path = points.map((p, i) => `${i ? "L" : "M"}${X(i).toFixed(1)},${Y(p.value).toFixed(1)}`).join(" ");
  const area = `${path} L${X(points.length - 1).toFixed(1)},${H - pad} L${X(0).toFixed(1)},${H - pad} Z`;
  svg.innerHTML = `
    <path d="${area}" fill="rgba(98,183,255,0.12)" />
    <path d="${path}" fill="none" stroke="#62b7ff" stroke-width="2" />
    <text x="${pad}" y="16" fill="#8b95a7" font-size="12">max ${max}</text>
    <text x="${pad}" y="${H - 6}" fill="#8b95a7" font-size="12">min ${min}</text>
    <text x="${W - pad}" y="16" fill="#8b95a7" font-size="12" text-anchor="end">${new Date(points[points.length - 1].at).toLocaleString()}</text>`;
}

// ---- tabs ------------------------------------------------------------------

function selectTab(name) {
  document.querySelectorAll("#tabs button").forEach((b) =>
    b.classList.toggle("active", b.dataset.tab === name));
  document.querySelectorAll(".view").forEach((v) =>
    v.classList.toggle("hidden", v.dataset.view !== name));
  if (name === "trends") loadSeries().catch(() => {});
  if (name === "missions") loadMissions().catch(() => {});
  if (name === "export") syncExportLinks();
}

// ---- wiring ----------------------------------------------------------------

function reload() {
  loadHealth();
  loadSummary().catch(() => {});
  if (!$('[data-view="missions"]').classList.contains("hidden")) loadMissions().catch(() => {});
  if (!$('[data-view="trends"]').classList.contains("hidden")) loadSeries().catch(() => {});
  syncExportLinks();
}

function runExport() {
  const type = $("#export-type").value;
  const format = $("#export-format").value;
  let url = api(`/api/plugin/export?type=${type}&format=${format}`);
  if (type === "events" && $("#export-event").value) {
    url += "&event=" + encodeURIComponent($("#export-event").value);
  }
  if (type === "series") {
    url += `&metric=${encodeURIComponent($("#metric").value)}&scope=${state.scope}`;
  }
  window.location.href = url;
}

function bind() {
  $("#tabs").addEventListener("click", (e) => {
    const b = e.target.closest("button[data-tab]");
    if (b) selectTab(b.dataset.tab);
  });
  $("#scope").addEventListener("change", (e) => {
    state.scope = e.target.value;
    renderAll();
    syncExportLinks();
    if (!$('[data-view="trends"]').classList.contains("hidden")) loadSeries().catch(() => {});
  });
  $("#instance").addEventListener("change", (e) => {
    state.instance = e.target.value;
    reload();
  });
  $("#refresh").addEventListener("click", reload);
  $("#metric").addEventListener("change", () => loadSeries().catch(() => {}));
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
