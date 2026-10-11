import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkDocument, getDefaultLanguage } from './api.js';

test('deduplicates words and forwards the abort signal', async () => {
  const signal = new AbortController().signal;
  const result = await checkDocument('Teh Teh quick', { signal, fetcher: async (url, options) => {
    assert.equal(url, '/v1/check');
    assert.equal(options.method, 'POST');
    assert.equal(options.signal, signal);
    assert.deepEqual(JSON.parse(options.body), { words: ['Teh', 'quick'] });
    return Response.json({ misspelled: [{ word: 'Teh', suggestions: ['The'] }] });
  } });
  assert.deepEqual(result, { issues: [{ word: 'Teh', suggestions: ['The'] }], skipped: 0 });
});
for (const language of ['en-US', 'en-GB']) {
  test(`reads configured default ${language}`, async () => {
    const result = await getDefaultLanguage({ fetcher: async url => {
      assert.equal(url, '/v1/config');
      return Response.json({ defaultLanguage: language });
    } });
    assert.equal(result, language);
  });
}
for (const [name, response] of [
  ['failed configuration request', () => new Response('', { status: 503 })],
  ['unsupported configured convention', () => Response.json({ defaultLanguage: 'fr-FR' })],
  ['missing configured convention', () => Response.json({})],
]) {
  test(name, async () => {
    await assert.rejects(getDefaultLanguage({ fetcher: async () => response() }));
  });
}
for (const language of ['en-US', 'en-GB']) {
  test(`sends ${language} in every batch`, async () => {
    const words = Array.from({ length: 201 }, (_, n) => 'zz' + String.fromCharCode(97 + Math.floor(n / 26), 97 + n % 26));
    const languages = [];
    await checkDocument(words.join(' '), { language, fetcher: async (_url, options) => {
      languages.push(JSON.parse(options.body).language);
      return Response.json({ misspelled: [] });
    } });
    assert.deepEqual(languages, [language, language]);
  });
}
test('batches large documents within both API limits', async () => {
  const words = Array.from({ length: 601 }, (_, n) => 'zz' + String.fromCharCode(97 + Math.floor(n / 26), 97 + n % 26));
  const sizes = [];
  await checkDocument(words.join(' '), { fetcher: async (_url, options) => {
    const body = JSON.parse(options.body);
    sizes.push(body.words.length);
    assert.ok(Buffer.byteLength(options.body) < 64 * 1024);
    return Response.json({ misspelled: [] });
  } });
  assert.deepEqual(sizes, [200, 200, 200, 1]);
});
test('skips long words and does not submit an empty batch', async () => {
  const result = await checkDocument('a'.repeat(65), { fetcher: () => { throw new Error('unexpected request'); } });
  assert.deepEqual(result, { issues: [], skipped: 1 });
});
for (const [name, response] of [
  ['busy API', () => new Response('busy', { status: 503 })],
  ['malformed JSON', () => new Response('not json')],
  ['malformed shape', () => Response.json({ misspelled: null })],
  ['unknown word', () => Response.json({ misspelled: [{ word: 'wrong', suggestions: [] }] })],
  ['malformed suggestions', () => Response.json({ misspelled: [{ word: 'teh', suggestions: [42] }] })],
]) {
  test(name, async () => {
    await assert.rejects(checkDocument('teh', { fetcher: async () => response() }));
  });
}
test('network failure is not an all-valid result', async () => {
  await assert.rejects(checkDocument('teh', { fetcher: async () => { throw new Error('offline'); } }));
});
test('honors cancellation even if the transport ignores it', async () => {
  const controller = new AbortController();
  await assert.rejects(checkDocument('teh', { signal: controller.signal, fetcher: async () => {
    controller.abort();
    return Response.json({ misspelled: [] });
  } }));
});
