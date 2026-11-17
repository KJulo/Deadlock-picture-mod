// The dictionaries in src/i18n/ui.ts. `en: Dict` only checks the keys when something runs the
// TypeScript checker, and `astro build` does not, so these tests are the real guard.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { ui, fmt, homeUrl, useDict, defaultLang } from '../src/i18n/ui.ts';

const { es, en } = ui;

// Walks two values in parallel and reports every place where their shape differs.
function shapeDiff(a: unknown, b: unknown, path = ''): string[] {
  const kind = (v: unknown) => (Array.isArray(v) ? 'array' : typeof v);
  if (kind(a) !== kind(b)) return [`${path || '(root)'}: ${kind(a)} vs ${kind(b)}`];
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return [`${path}: ${a.length} items vs ${b.length}`];
    return a.flatMap((v, i) => shapeDiff(v, b[i], `${path}[${i}]`));
  }
  if (a && typeof a === 'object') {
    const ka = Object.keys(a), kb = Object.keys(b as object);
    const missing = ka.filter((k) => !kb.includes(k)).map((k) => `${path}.${k}: missing in en`);
    const extra = kb.filter((k) => !ka.includes(k)).map((k) => `${path}.${k}: only in en`);
    const shared = ka.filter((k) => kb.includes(k))
      .flatMap((k) => shapeDiff((a as any)[k], (b as any)[k], `${path}.${k}`));
    return [...missing, ...extra, ...shared];
  }
  return [];
}

// Every string in a dictionary, with its path.
function strings(v: unknown, path = ''): [string, string][] {
  if (typeof v === 'string') return [[path, v]];
  if (Array.isArray(v)) return v.flatMap((x, i) => strings(x, `${path}[${i}]`));
  if (v && typeof v === 'object') return Object.entries(v).flatMap(([k, x]) => strings(x, `${path}.${k}`));
  return [];
}
const enByPath = new Map(strings(en));
const pairs = strings(es).map(([path, s]) => [path, s, enByPath.get(path) ?? ''] as const);
const sorted = (s: string, re: RegExp) => [...s.matchAll(re)].map((m) => m[1]).sort();

test('es and en have the same keys, arrays and value types', () => {
  assert.deepEqual(shapeDiff(es, en), []);
});

test('no string is empty', () => {
  const empty = [...strings(es), ...strings(en)].filter(([, s]) => !s.trim()).map(([p]) => p);
  assert.deepEqual(empty, []);
});

test('each translation keeps the same {placeholders}', () => {
  for (const [path, a, b] of pairs) assert.deepEqual(sorted(b, /\{(\w+)\}/g), sorted(a, /\{(\w+)\}/g), path);
});

test('each translation keeps the same inline HTML tags (set:html strings)', () => {
  for (const [path, a, b] of pairs) assert.deepEqual(sorted(b, /<\/?(\w+)/g), sorted(a, /<\/?(\w+)/g), path);
});

test('every error code the backend sends has a translation', () => {
  const go = readFileSync(new URL('../../backend/internal/api/api.go', import.meta.url), 'utf8');
  const codes = [...new Set([...go.matchAll(/apiError\{[^,]+,\s*"([a-z_]+)"/g)].map((m) => m[1]))];
  assert.ok(codes.length > 0, 'found no apiError codes in api.go: did the pattern change?');
  for (const code of codes) {
    assert.ok(code in es.errors, `es.errors.${code} is missing`);
    assert.ok(code in en.errors, `en.errors.${code} is missing`);
  }
});

test('fmt fills known placeholders and leaves unknown ones', () => {
  assert.equal(fmt('{a} y {b}', { a: 1, b: 'dos' }), '1 y dos');
  assert.equal(fmt('{a} {missing}', { a: 'x' }), 'x {missing}');
});

test('homeUrl and useDict follow the default language', () => {
  assert.equal(homeUrl(defaultLang), '/');
  assert.equal(homeUrl('en'), '/en/');
  assert.equal(useDict('en'), en);
  assert.equal(useDict(undefined), ui[defaultLang]);
  assert.equal(useDict('fr'), ui[defaultLang]);
});
