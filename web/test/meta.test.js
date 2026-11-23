// SEO, hreflang, OpenGraph, and Twitter Card metadata checks on the built site (dist/).
// Run `pnpm build` before `pnpm test`.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync, existsSync, statSync } from 'node:fs';

const siteOrigin = process.env.PUBLIC_SITE_URL || 'https://catmod.pages.dev';
const pages = {
  es: { file: 'dist/index.html', path: '/', locale: 'es_ES' },
  en: { file: 'dist/en/index.html', path: '/en/', locale: 'en_US' },
};

const read = (path) => {
  const file = new URL(`../${path}`, import.meta.url);
  assert.ok(existsSync(file), `${path} not found: run \`pnpm build\` before \`pnpm test\``);
  return readFileSync(file, 'utf8');
};

const metaContent = (html, key, attrName = 'property') => {
  const re = new RegExp(`<meta[^>]*\\b${attrName}="${key}"[^>]*\\bcontent="([^"]+)"`, 'i');
  const rev = new RegExp(`<meta[^>]*\\bcontent="([^"]+)"[^>]*\\b${attrName}="${key}"`, 'i');
  return html.match(re)?.[1] ?? html.match(rev)?.[1] ?? null;
};

const linkHref = (html, rel, extra = '') => {
  const links = [...html.matchAll(/<link\b[^>]*>/gi)].map((m) => m[0]);
  const match = links.find((tag) =>
    new RegExp(`\\brel="${rel}"`, 'i').test(tag) &&
    (!extra || new RegExp(extra, 'i').test(tag))
  );
  return match?.match(/\bhref="([^"]+)"/i)?.[1] ?? null;
};

test('og-preview.png exists in public/ and dist/', () => {
  for (const rel of ['public/og-preview.png', 'dist/og-preview.png']) {
    const file = new URL(`../${rel}`, import.meta.url);
    assert.ok(existsSync(file), `${rel} is missing`);
    assert.ok(statSync(file).size > 0, `${rel} is empty`);
  }
});

for (const [lang, { file, path, locale }] of Object.entries(pages)) {
  const expectedCanonical = new URL(path, siteOrigin).href;
  const expectedImage = new URL('/og-preview.png', siteOrigin).href;

  test(`${lang}: canonical and hreflang links`, () => {
    const html = read(file);
    assert.equal(linkHref(html, 'canonical'), expectedCanonical);
    assert.equal(linkHref(html, 'alternate', '\\bhreflang="es"'), new URL('/', siteOrigin).href);
    assert.equal(linkHref(html, 'alternate', '\\bhreflang="en"'), new URL('/en/', siteOrigin).href);
    assert.equal(linkHref(html, 'alternate', '\\bhreflang="x-default"'), new URL('/', siteOrigin).href);
  });

  test(`${lang}: OpenGraph and Twitter Card metadata`, () => {
    const html = read(file);
    assert.equal(metaContent(html, 'og:type'), 'website');
    assert.equal(metaContent(html, 'og:site_name'), 'Soul Container Mod');
    assert.equal(metaContent(html, 'og:url'), expectedCanonical);
    assert.equal(metaContent(html, 'og:locale'), locale);
    assert.equal(metaContent(html, 'og:image'), expectedImage);
    assert.ok(metaContent(html, 'og:title'), 'og:title is missing');
    assert.ok(metaContent(html, 'og:description'), 'og:description is missing');
    assert.ok(metaContent(html, 'og:image:alt'), 'og:image:alt is missing');

    assert.equal(metaContent(html, 'twitter:card', 'name'), 'summary_large_image');
    assert.equal(metaContent(html, 'twitter:image', 'name'), expectedImage);
    assert.ok(metaContent(html, 'twitter:title', 'name'), 'twitter:title is missing');
    assert.ok(metaContent(html, 'twitter:description', 'name'), 'twitter:description is missing');
  });
}
