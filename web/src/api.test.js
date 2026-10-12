import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkDocument as checkWithConfiguration, getConfiguration } from './api.js';

const limits = { maxWordLen: 64, maxWords: 500, maxBodyBytes: 385564 };
const checkDocument = (text, options) => checkWithConfiguration(text, { ...limits, ...options });

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
    const result = await getConfiguration({ fetcher: async url => {
      assert.equal(url, '/v1/config');
      return Response.json({ defaultLanguage: language, ...limits, maxWordLen: 80 });
    } });
    assert.deepEqual(result, { defaultLanguage: language, ...limits, maxWordLen: 80 });
  });
}
for (const [name, response] of [
  ['failed configuration request', () => new Response('', { status: 503 })],
  ['unsupported configured convention', () => Response.json({ defaultLanguage: 'fr-FR' })],
  ['missing configured convention', () => Response.json({})],
  ...Object.keys(limits).flatMap(field => [undefined, 0, -1, 1.5, '64', Number.MAX_SAFE_INTEGER + 1].map(value => [
    `invalid configured ${field} ${value}`, () => Response.json({ defaultLanguage: 'en-US', ...limits, [field]: value }),
  ])),
]) {
  test(name, async () => {
    const calls = [];
    const fetcher = async url => {
      calls.push(url);
      return response();
    };
    await assert.rejects(async () => {
      const configuration = await getConfiguration({ fetcher });
      await checkWithConfiguration('teh', { ...configuration, fetcher });
    });
    assert.deepEqual(calls, ['/v1/config']);
  });
}
for (const language of ['en-US', 'en-GB']) {
  test(`sends ${language} in every batch`, async () => {
    const words = Array.from({ length: 501 }, (_, n) => 'zz' + String.fromCharCode(97 + Math.floor(n / 26), 97 + n % 26));
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
    assert.ok(Buffer.byteLength(options.body) <= limits.maxBodyBytes);
    return Response.json({ misspelled: [] });
  } });
  assert.deepEqual(sizes, [500, 101]);
});
for (const row of [
  { name: 'configured count', text: 'one two three four five', maxWords: 2, maxBodyBytes: 100, batches: [['one', 'two'], ['three', 'four'], ['five']] },
  { name: 'exact byte boundary', text: 'one two', maxWords: 5, maxBodyBytes: Buffer.byteLength(JSON.stringify({ words: ['one', 'two'], language: 'en-GB' })), batches: [['one', 'two']] },
  { name: 'one byte below boundary', text: 'one two', maxWords: 5, maxBodyBytes: Buffer.byteLength(JSON.stringify({ words: ['one', 'two'], language: 'en-GB' })) - 1, batches: [['one'], ['two']] },
  { name: 'UTF-8 bytes rather than code units', text: '𐐀é café été', maxWords: 5, maxBodyBytes: Buffer.byteLength(JSON.stringify({ words: ['𐐀é'], language: 'en-GB' })), batches: [['𐐀é'], ['café'], ['été']] },
]) {
  test(`batches by ${row.name}`, async () => {
    const batches = [];
    await checkDocument(row.text, { ...row, language: 'en-GB', fetcher: async (_url, options) => {
      assert.ok(Buffer.byteLength(options.body) <= row.maxBodyBytes);
      const body = JSON.parse(options.body);
      assert.equal(body.language, 'en-GB');
      batches.push(body.words);
      return Response.json({ misspelled: [] });
    } });
    assert.deepEqual(batches, row.batches);
  });
}
for (const row of [
  { name: 'word cannot fit body', options: { maxBodyBytes: 1 } },
  { name: 'already canceled', options: { signal: AbortSignal.abort() } },
]) {
  test(`rejects without submitting: ${row.name}`, async () => {
    let calls = 0;
    await assert.rejects(checkDocument('teh', { ...row.options, fetcher: async () => {
      calls++;
      return Response.json({ misspelled: [] });
    } }));
    assert.equal(calls, 0);
  });
}
test('skips long words and does not submit an empty batch', async () => {
  const result = await checkDocument('a'.repeat(65), { fetcher: () => { throw new Error('unexpected request'); } });
  assert.deepEqual(result, { issues: [], skipped: 1 });
});
for (const row of [
  { name: 'uses a smaller configured limit', maxWordLen: 3, text: 'the café', words: ['the'], skipped: 1 },
  { name: 'accepts words above the old limit', maxWordLen: 80, text: 'a'.repeat(65), words: ['a'.repeat(65)], skipped: 0 },
  { name: 'counts Unicode code points', maxWordLen: 2, text: '𐐀é 𐐀éa', words: ['𐐀é'], skipped: 1 },
]) {
  test(row.name, async () => {
    const sent = [];
    const result = await checkDocument(row.text, { maxWordLen: row.maxWordLen, fetcher: async (_url, options) => {
      sent.push(...JSON.parse(options.body).words);
      return Response.json({ misspelled: [] });
    } });
    assert.deepEqual(sent, row.words);
    assert.equal(result.skipped, row.skipped);
  });
}
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
