/**
 * Client for the server-only École & Culture G API.
 *
 * Every function throws an Error carrying the server message so the panels can
 * show it directly. The endpoints answer 404 on the desktop by design.
 */

async function request(path, options = {}) {
  const response = await fetch(path, {
    headers: { Accept: 'application/json', ...(options.body ? { 'Content-Type': 'application/json' } : {}) },
    ...options,
  });
  let body = null;
  try {
    body = await response.json();
  } catch {
    /* empty body */
  }
  if (!response.ok) {
    throw new Error(body?.error || `${response.status}`);
  }
  return body ?? {};
}

function query(params) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') search.set(key, value);
  }
  const text = search.toString();
  return text ? `?${text}` : '';
}

export const schoolAPI = {
  taxonomy: () => request('/api/school/taxonomy'),
  items: (params = {}) => request(`/api/school/items${query(params)}`),
  item: (uid) => request(`/api/school/items/${encodeURIComponent(uid)}`),
  create: (item) => request('/api/school/items', { method: 'POST', body: JSON.stringify(item) }),
  update: (uid, item) =>
    request(`/api/school/items/${encodeURIComponent(uid)}`, { method: 'PUT', body: JSON.stringify(item) }),
  archive: (uid) => request(`/api/school/items/${encodeURIComponent(uid)}`, { method: 'DELETE' }),
  progress: () => request('/api/school/progress'),
  setProgress: (uid, status, percent = 0) =>
    request(`/api/school/items/${encodeURIComponent(uid)}/progress`, {
      method: 'POST',
      body: JSON.stringify({ status, percent }),
    }),
  attempts: (uid) => request(`/api/school/quizzes/${encodeURIComponent(uid)}/attempts`),
  submitQuiz: (uid, answers) =>
    request(`/api/school/quizzes/${encodeURIComponent(uid)}/attempts`, {
      method: 'POST',
      body: JSON.stringify({ answers }),
    }),
  leaderboard: () => request('/api/school/leaderboard'),
  pending: () => request('/api/admin/school/pending'),
  review: (uid, decision, note = '') =>
    request(`/api/admin/school/items/${encodeURIComponent(uid)}/review`, {
      method: 'POST',
      body: JSON.stringify({ decision, note }),
    }),
  saveTaxonomy: (entry) =>
    request('/api/admin/school/taxonomy', { method: 'POST', body: JSON.stringify(entry) }),
  deleteTaxonomy: (id) => request(`/api/admin/school/taxonomy/${id}`, { method: 'DELETE' }),
};
