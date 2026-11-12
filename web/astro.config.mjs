// @ts-check
import { defineConfig } from 'astro/config';

// Static site (Cloudflare Pages / GitHub Pages). The API URL comes from PUBLIC_API_BASE.
export default defineConfig({
  output: 'static',
  server: { port: 4321 }, // the API's default CORS origin is http://localhost:4321
});
