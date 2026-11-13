// Calls to the Go backend (see backend/internal/api).
export function createApi(base) {
  const url = (p) => base.replace(/\/$/, '') + p;

  async function check(res) {
    if (res.ok) return res;
    let message = `HTTP ${res.status}`;
    let code;
    try {
      const e = (await res.json()).error;
      message = e.message || message;
      code = e.code;
    } catch { /* not JSON */ }
    const err = new Error(message);
    err.status = res.status;
    err.code = code; // stable error code; the UI translates it (src/i18n/ui.ts → errors)
    throw err;
  }

  function form(photo, params) {
    const f = new FormData();
    (Array.isArray(photo) ? photo : [photo]).forEach((p, i) => f.append(i ? `photo${i}` : 'photo', p));
    if (params) f.append('params', JSON.stringify(params));
    return f;
  }

  return {
    defaults: async () => (await check(await fetch(url('/api/defaults')))).json(),
    // capDeg only matters for the medallions, whose size follows the seals
    mesh: async (lod = 0, shape = 'sphere', capDeg = 43) =>
      (await check(await fetch(url(`/api/mesh?lod=${lod}&shape=${encodeURIComponent(shape)}&cap_deg=${capDeg}`)))).json(),
    texture: async (photo, params, size = 2048, signal) =>
      (await check(await fetch(url(`/api/texture?size=${size}`), { method: 'POST', body: form(photo, params), signal }))).blob(),
    build: async (photo, params) =>
      (await check(await fetch(url('/api/build'), { method: 'POST', body: form(photo, params) }))).blob(),
  };
}
