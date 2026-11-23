// The client for the Go backend (src/lib/preview/api.js), against a fake fetch.
import { test, beforeEach } from 'node:test';
import assert from 'node:assert/strict';
import { createApi } from '../src/lib/preview/api.js';

let calls, reply;
beforeEach(() => {
  calls = [];
  reply = () => Response.json({});
  globalThis.fetch = async (url, init = {}) => { calls.push({ url, init }); return reply(); };
});

test('the base URL may end with a slash', async () => {
  await createApi('http://api.test/').defaults();
  assert.equal(calls[0].url, 'http://api.test/api/defaults');
});

test('mesh puts lod, shape and cap in the query, encoding the shape', async () => {
  await createApi('http://api.test').mesh(2, 'odd shape', 30);
  assert.equal(calls[0].url, 'http://api.test/api/mesh?lod=2&shape=odd%20shape&cap_deg=30');
});

test('texture posts every photo (photo, photo1, …) and the params as JSON', async () => {
  reply = () => new Response(new Blob(['png']));
  const photos = [new Blob(['a']), new Blob(['b']), new Blob(['c'])];
  const params = { shape: 'cube', seals: [] };
  const blob = await createApi('http://api.test').texture(photos, params, 512);
  const { url, init } = calls[0];
  assert.equal(url, 'http://api.test/api/texture?size=512');
  assert.equal(init.method, 'POST');
  assert.deepEqual([...init.body.keys()], ['photo', 'photo1', 'photo2', 'params']);
  assert.deepEqual(JSON.parse(init.body.get('params')), params);
  assert.equal(await blob.text(), 'png');
});

test('build accepts a single photo', async () => {
  reply = () => new Response(new Blob(['vpk']));
  await createApi('http://api.test').build(new Blob(['a']), { shape: 'sphere' });
  assert.equal(calls[0].url, 'http://api.test/api/build');
  assert.deepEqual([...calls[0].init.body.keys()], ['photo', 'params']);
});

test('normal sends JSON params without uploading photos (legacy signature)', async () => {
  reply = () => new Response(new Blob(['normal-png']));
  const ac = new AbortController();
  const params = { relief: 0.6, shape: 'cube' };
  const blob = await createApi('http://api.test').normal([new Blob(['ignored'])], params, 512, ac.signal);
  const { url, init } = calls[0];
  assert.equal(url, 'http://api.test/api/texture?size=512&map=normal');
  assert.equal(init.method, 'POST');
  assert.deepEqual(init.headers, { 'Content-Type': 'application/json' });
  assert.equal(typeof init.body, 'string');
  assert.deepEqual(JSON.parse(init.body), params);
  assert.equal(init.signal, ac.signal);
  assert.equal(await blob.text(), 'normal-png');
});

test('normal also works when called directly with (params, size, signal)', async () => {
  reply = () => new Response(new Blob(['normal-png']));
  const ac = new AbortController();
  const params = { relief: 0.3, layout: 'cube6' };
  await createApi('http://api.test').normal(params, 1024, ac.signal);
  assert.equal(calls[0].url, 'http://api.test/api/texture?size=1024&map=normal');
  assert.deepEqual(calls[0].init.headers, { 'Content-Type': 'application/json' });
  assert.deepEqual(JSON.parse(calls[0].init.body), params);
  assert.equal(calls[0].init.signal, ac.signal);

  await createApi('http://api.test').normal(params);
  assert.equal(calls[1].url, 'http://api.test/api/texture?size=2048&map=normal');
  assert.deepEqual(JSON.parse(calls[1].init.body), params);
});

test('a backend error keeps its message, stable code and status', async () => {
  reply = () => Response.json({ error: { code: 'busy', message: 'the server is busy' } }, { status: 503 });
  await assert.rejects(createApi('http://api.test').defaults(),
    (e) => e.message === 'the server is busy' && e.code === 'busy' && e.status === 503);
});

test('an error without a JSON body falls back to the HTTP status', async () => {
  reply = () => new Response('Bad Gateway', { status: 502 });
  await assert.rejects(createApi('http://api.test').defaults(),
    (e) => e.message === 'HTTP 502' && e.code === undefined && e.status === 502);
});
