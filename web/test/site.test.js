// Smoke tests on the built site (dist/), so run `pnpm build` first. They catch pages that build
// but are broken: a missing element the workshop script looks up, a dead in-page link…
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync } from 'node:fs';

const pages = { es: 'dist/index.html', en: 'dist/en/index.html' };
const read = (path) => {
  const file = new URL(`../${path}`, import.meta.url);
  assert.ok(existsSync(file), `${path} not found: run \`pnpm build\` before \`pnpm test\``);
  return readFileSync(file, 'utf8');
};
const attr = (html, re) => [...html.matchAll(re)].map((m) => m[1]);

for (const [lang, path] of Object.entries(pages)) {
  test(`${lang}: language, title and the other language's link`, () => {
    const html = read(path);
    assert.match(html, new RegExp(`<html[^>]*lang="${lang}"`));
    assert.match(html, /<title>[^<]*Soul Container Mod[^<]*<\/title>/);
    const other = lang === 'es' ? '/en/' : '/';
    assert.ok(attr(html, /href="([^"]*)"/g).includes(other), `no link to ${other}`);
  });

  test(`${lang}: every in-page link points to an element that exists`, () => {
    const html = read(path);
    const ids = new Set(attr(html, /\sid="([^"]+)"/g));
    const dead = attr(html, /href="#([^"]+)"/g).filter((id) => !ids.has(id));
    assert.deepEqual(dead, []);
  });

  test(`${lang}: the workshop has what its script needs`, () => {
    const html = read(path);
    const ids = new Set(attr(html, /\sid="([^"]+)"/g));
    for (const id of ['taller', 'view', 'overlay', 'photo', 'drop', 'seal', 'zoom', 'turn', 'sticker_deg',
      'ring_deg', 'forge', 'status', 'undo', 'redo', 'reset-all', 'legend-move', 'cover-link']) {
      assert.ok(ids.has(id), `#${id} is missing`);
    }
    // the runtime strings the script parses on load
    const raw = html.match(/id="taller"[^>]*data-messages="([^"]*)"/)?.[1];
    assert.ok(raw, 'data-messages is missing on #taller');
    const json = raw.replace(/&quot;/g, '"').replace(/&#39;/g, "'").replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&amp;/g, '&');
    const messages = JSON.parse(json);
    for (const key of ['workshop', 'status', 'errors']) assert.ok(messages[key], `messages.${key} is missing`);
  });
}
