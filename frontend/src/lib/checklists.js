/**
 * Checklist store: the saved checklists, their local execution progress and
 * JSON/CSV import-export.
 *
 * Progress is deliberately local to the device (localStorage), as agreed for
 * the first version: a desktop and a tablet can be at different steps of the
 * same checklist without fighting over a shared state.
 */
import { writable } from 'svelte/store';
import { tNow } from './i18n.js';

export const checklists = writable([]);
export const checklistsLoading = writable(false);
export const checklistsError = writable('');
/** False when the backend runs without persistence. */
export const checklistsEnabled = writable(true);

export async function loadChecklists() {
  checklistsLoading.set(true);
  checklistsError.set('');
  try {
    const response = await fetch('/api/checklists');
    if (!response.ok) throw new Error(`${response.status}`);
    const body = await response.json();
    const enabled = body.enabled !== false;
    checklistsEnabled.set(enabled);
    checklists.set(enabled ? body.checklists ?? [] : []);
  } catch (error) {
    checklistsError.set(tNow('checklist.error', { detail: error.message }));
  } finally {
    checklistsLoading.set(false);
  }
}

/** Create or update a checklist. The server upserts on the uid. */
export async function saveChecklist(checklist) {
  const response = await fetch('/api/checklists', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(checklist),
  });
  if (!response.ok) {
    let detail = `${response.status}`;
    try {
      const body = await response.json();
      if (body?.error) detail = body.error;
    } catch {
      /* keep the status */
    }
    throw new Error(detail);
  }
  const saved = await response.json();
  checklists.update((list) => {
    const index = list.findIndex((item) => item.uid === saved.uid);
    if (index < 0) return [saved, ...list];
    const next = list.slice();
    next[index] = saved;
    return next.sort((a, b) => (b.updatedAt ?? 0) - (a.updatedAt ?? 0));
  });
  return saved;
}

export async function deleteChecklist(uid) {
  const response = await fetch(`/api/checklists/${encodeURIComponent(uid)}`, { method: 'DELETE' });
  if (!response.ok && response.status !== 404) {
    throw new Error(`${response.status}`);
  }
  checklists.update((list) => list.filter((item) => item.uid !== uid));
  clearProgress(uid);
}

// ---------------------------------------------------------------------------
// Local execution progress.
// ---------------------------------------------------------------------------

const PROGRESS_PREFIX = 'dcsmanager.checklist.progress.';

export function loadProgress(uid) {
  try {
    const raw = localStorage.getItem(PROGRESS_PREFIX + uid);
    if (!raw) return { checked: [], current: 0 };
    const parsed = JSON.parse(raw);
    return {
      checked: Array.isArray(parsed.checked) ? parsed.checked : [],
      current: Number.isInteger(parsed.current) ? parsed.current : 0,
    };
  } catch {
    return { checked: [], current: 0 };
  }
}

export function saveProgress(uid, progress) {
  try {
    localStorage.setItem(PROGRESS_PREFIX + uid, JSON.stringify({ ...progress, updatedAt: Date.now() }));
  } catch {
    /* storage full or disabled: execution still works for this session */
  }
}

export function clearProgress(uid) {
  try {
    localStorage.removeItem(PROGRESS_PREFIX + uid);
  } catch {
    /* ignore */
  }
}

// ---------------------------------------------------------------------------
// Import and export.
// ---------------------------------------------------------------------------

/** A blank checklist draft for the editor. */
export function newDraft() {
  return {
    uid: '',
    name: '',
    aircraft: '',
    phase: 'prestart',
    description: '',
    source: 'user',
    visibility: 'private',
    items: [{ uid: '', text: '', response: '', note: '', critical: false }],
  };
}

/** Copy a checklist or a template into an editable draft. */
export function draftFrom(checklist, overrides = {}) {
  return {
    uid: '',
    name: checklist.name ?? '',
    aircraft: checklist.aircraft ?? '',
    phase: checklist.phase ?? 'prestart',
    description: checklist.description ?? '',
    source: overrides.source ?? 'user',
    visibility: checklist.visibility ?? 'private',
    ...overrides,
    items: (checklist.items ?? []).map((item) => ({
      uid: '',
      text: item.text ?? '',
      response: item.response ?? '',
      note: item.note ?? '',
      critical: Boolean(item.critical),
    })),
  };
}

function normalizeDraft(raw) {
  const items = Array.isArray(raw.items) ? raw.items : [];
  return {
    uid: typeof raw.uid === 'string' ? raw.uid : '',
    name: String(raw.name ?? '').trim(),
    aircraft: String(raw.aircraft ?? '').trim(),
    phase: typeof raw.phase === 'string' && raw.phase ? raw.phase : 'prestart',
    description: String(raw.description ?? '').trim(),
    source: 'import',
    visibility: raw.visibility === 'squadron' ? 'squadron' : 'private',
    items: items
      .map((item) => {
        if (typeof item === 'string') return { text: item.trim(), response: '', note: '', critical: false };
        return {
          text: String(item.text ?? item.name ?? '').trim(),
          response: String(item.response ?? item.expected ?? '').trim(),
          note: String(item.note ?? '').trim(),
          critical: Boolean(item.critical),
        };
      })
      .filter((item) => item.text),
  };
}

/** Parse CSV text into rows of fields, honouring quotes and CRLF. */
export function parseCSV(text) {
  const rows = [];
  let row = [];
  let field = '';
  let quoted = false;
  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i];
    if (quoted) {
      if (ch === '"') {
        if (text[i + 1] === '"') {
          field += '"';
          i += 1;
        } else {
          quoted = false;
        }
      } else {
        field += ch;
      }
      continue;
    }
    if (ch === '"') {
      quoted = true;
    } else if (ch === ',') {
      row.push(field);
      field = '';
    } else if (ch === '\n') {
      row.push(field);
      rows.push(row);
      row = [];
      field = '';
    } else if (ch !== '\r') {
      field += ch;
    }
  }
  if (field !== '' || row.length > 0) {
    row.push(field);
    rows.push(row);
  }
  return rows.filter((cells) => cells.some((cell) => cell.trim() !== ''));
}

function csvImport(text, filename) {
  const rows = parseCSV(text);
  if (rows.length === 0) return [];
  const header = rows[0].map((cell) => cell.trim().toLowerCase());
  const hasHeader = header.includes('item') || header.includes('name') || header.includes('checklist');
  const column = (row, name) => {
    const index = header.indexOf(name);
    return index >= 0 ? (row[index] ?? '').trim() : '';
  };

  if (!hasHeader) {
    // A plain list: one item per line, named after the file.
    const name = filename.replace(/\.[^.]+$/, '') || 'Imported checklist';
    return [normalizeDraft({
      name,
      phase: 'prestart',
      items: rows.map((cells) => ({ text: cells.join(',').trim() })),
    })];
  }

  const groups = new Map();
  for (const row of rows.slice(1)) {
    const name = column(row, 'name') || column(row, 'checklist');
    if (!name) continue;
    const aircraft = column(row, 'aircraft');
    const phase = column(row, 'phase') || 'prestart';
    const key = `${name}\u0000${aircraft}\u0000${phase}`;
    if (!groups.has(key)) {
      groups.set(key, normalizeDraft({
        name,
        aircraft,
        phase,
        description: column(row, 'description'),
        items: [],
      }));
    }
    const item = column(row, 'item') || row[0] || '';
    if (!item.trim()) continue;
    groups.get(key).items.push({
      text: item.trim(),
      response: column(row, 'response'),
      note: column(row, 'note'),
      critical: /^(1|true|yes|oui)$/i.test(column(row, 'critical')),
    });
  }
  return [...groups.values()].filter((checklist) => checklist.items.length > 0);
}

/**
 * Parse an imported file (JSON, CSV or a plain text list) into checklist
 * drafts. Throws an Error with a readable message when the file is unusable.
 */
export function parseImport(text, filename = '') {
  const content = text.replace(/^\uFEFF/, '').trim();
  if (!content) throw new Error(tNow('checklist.importEmpty'));

  const lower = filename.toLowerCase();
  const looksJSON = lower.endsWith('.json') || content.startsWith('{') || content.startsWith('[');
  if (looksJSON) {
    let parsed;
    try {
      parsed = JSON.parse(content);
    } catch {
      throw new Error(tNow('checklist.importInvalidJSON'));
    }
    const source = Array.isArray(parsed) ? parsed : Array.isArray(parsed?.checklists) ? parsed.checklists : [parsed];
    const drafts = source
      .filter((entry) => entry && typeof entry === 'object')
      .map(normalizeDraft)
      .filter((checklist) => checklist.name && checklist.items.length > 0);
    if (drafts.length === 0) throw new Error(tNow('checklist.importNoChecklist'));
    return drafts;
  }

  const drafts = csvImport(content, filename);
  if (drafts.length === 0) throw new Error(tNow('checklist.importNoChecklist'));
  return drafts;
}

function csvEscape(value) {
  const text = String(value ?? '');
  return /[",\r\n]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text;
}

/** Serialise one checklist as CSV: one row per item. */
export function toCSV(checklist) {
  const rows = [['name', 'aircraft', 'phase', 'description', 'item', 'response', 'note', 'critical']];
  for (const item of checklist.items ?? []) {
    rows.push([
      checklist.name,
      checklist.aircraft ?? '',
      checklist.phase ?? '',
      checklist.description ?? '',
      item.text,
      item.response ?? '',
      item.note ?? '',
      item.critical ? 'true' : 'false',
    ]);
  }
  return rows.map((row) => row.map(csvEscape).join(',')).join('\r\n');
}

/** Serialise checklists as the JSON export format. */
export function toJSON(checklistsToExport) {
  return JSON.stringify({ version: 1, checklists: checklistsToExport }, null, 2);
}

/** Trigger a browser download for generated text. */
export function downloadText(filename, text, type = 'application/json') {
  const blob = new Blob([text], { type: `${type};charset=utf-8` });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}
