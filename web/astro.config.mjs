// @ts-check
import { defineConfig } from 'astro/config';

// Static site (Cloudflare Pages / GitHub Pages). The API URL comes from PUBLIC_API_BASE.
// Spanish lives at / and English at /en/ (strings in src/i18n/ui.ts).
// Override `site` at build time with PUBLIC_SITE_URL for canonical, hreflang, and OpenGraph URLs.
export default defineConfig({
  site: process.env.PUBLIC_SITE_URL || 'https://catmod.pages.dev',
  output: 'static',
  server: { port: 4321 }, // the API's default CORS origin is http://localhost:4321
  i18n: {
    defaultLocale: 'es',
    locales: ['es', 'en'],
    routing: { prefixDefaultLocale: false },
  },
});
