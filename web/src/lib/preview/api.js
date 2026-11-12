// Calls to the Go backend (see backend/internal/api).
export function createApi(base) {
  const url = (p) => base.replace(/\/$/, '') + p;

  async function check(res) {
    if (res.ok) return res;
    let message = `HTTP ${res.status}`;
    try { message = (await res.json()).error.message; } catch { /* not JSON */ }
    const err = new Error(message);
    err.status = res.status;
    throw err;
  }

  function form(photo, params) {
    const f = new FormData();
    f.append('photo', photo);
    if (params) f.append('params', JSON.stringify(params));
    return f;
  }

  return {
    defaults: async () => (await check(await fetch(url('/api/defaults')))).json(),
    mesh: async (lod = 0) => (await check(await fetch(url(`/api/mesh?lod=${lod}`)))).json(),
    texture: async (photo, params, size = 2048, signal) =>
      (await check(await fetch(url(`/api/texture?size=${size}`), { method: 'POST', body: form(photo, params), signal }))).blob(),
    build: async (photo, params) =>
      (await check(await fetch(url('/api/build'), { method: 'POST', body: form(photo, params) }))).blob(),
  };
}
