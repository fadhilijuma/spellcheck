import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createSession } from './session.js';

const limits = { maxWordLen: 64, maxWords: 500, maxBodyBytes: 385564 };
const flush = () => new Promise(resolve => setImmediate(resolve));
function setup(t, fetcher) {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const states = [];
  const session = createSession(state => states.push(state), { fetcher });
  t.after(() => session.dispose());
  return { states, session };
}
for (const row of [
  { name: 'word count', maxWords: 1, maxBodyBytes: 100, batches: [['one'], ['two']] },
  { name: 'body bytes', maxWords: 500, maxBodyBytes: 17, batches: [['one'], ['two']] },
]) {
  test(`session forwards configured ${row.name}`, async t => {
    const batches = [];
    const { states, session } = setup(t, async (_url, options) => {
      assert.ok(Buffer.byteLength(options.body) <= row.maxBodyBytes);
      batches.push(JSON.parse(options.body).words);
      return Response.json({ misspelled: [] });
    });
    session.update('one two', { ...limits, ...row });
    t.mock.timers.tick(300);
    await flush();
    assert.deepEqual(batches, row.batches);
    assert.equal(states.at(-1).phase, 'ready');
  });
}
test('debounces typing and publishes only the latest document', async t => {
  const calls = [];
  const { states, session } = setup(t, async (_url, options) => {
    calls.push(JSON.parse(options.body).words);
    return Response.json({ misspelled: [] });
  });
  session.update('teh', limits);
  t.mock.timers.tick(200);
  session.update('hello', limits);
  t.mock.timers.tick(299);
  assert.equal(calls.length, 0);
  t.mock.timers.tick(1);
  await flush();
  assert.deepEqual(calls, [['hello']]);
  assert.equal(states.at(-1).phase, 'ready');
});
test('late response cannot overwrite newer results', async t => {
  let release;
  let firstSignal;
  const { states, session } = setup(t, async (_url, options) => {
    if (JSON.parse(options.body).words[0] === 'teh') {
      firstSignal = options.signal;
      return new Promise(resolve => { release = resolve; });
    }
    return Response.json({ misspelled: [] });
  });
  session.update('teh', limits);
  t.mock.timers.tick(300);
  session.update('hello', limits);
  assert.equal(firstSignal.aborted, true);
  t.mock.timers.tick(300);
  await flush();
  release(Response.json({ misspelled: [{ word: 'teh', suggestions: ['the'] }] }));
  await flush();
  assert.equal(states.at(-1).phase, 'ready');
  assert.deepEqual(states.at(-1).issues, []);
});
test('deadline reports unavailable instead of waiting forever', async t => {
  const { states, session } = setup(t, (_url, { signal }) => new Promise((_, reject) => {
    signal.addEventListener('abort', () => reject(signal.reason), { once: true });
  }));
  session.update('teh', limits);
  t.mock.timers.tick(300);
  t.mock.timers.tick(10000);
  await flush();
  assert.equal(states.at(-1).phase, 'offline');
});
test('failed checks retry on the next edit', async t => {
  let offline = true;
  const { states, session } = setup(t, async () => {
    if (offline) throw new Error('offline');
    return Response.json({ misspelled: [] });
  });
  session.update('hello', limits);
  t.mock.timers.tick(300);
  await flush();
  assert.equal(states.at(-1).phase, 'offline');
  offline = false;
  session.update('hello world', limits);
  t.mock.timers.tick(300);
  await flush();
  assert.equal(states.at(-1).phase, 'ready');
});
test('composition waits and empty input clears immediately', async t => {
  let calls = 0;
  const { states, session } = setup(t, async () => { calls++; return Response.json({ misspelled: [] }); });
  session.update('teh', { composing: true, ...limits });
  t.mock.timers.tick(300);
  await flush();
  assert.equal(calls, 0);
  session.update('');
  assert.equal(states.at(-1).phase, 'empty');
  assert.deepEqual(states.at(-1).issues, []);
});
for (const [first, second] of [['en-US', 'en-GB'], ['en-GB', 'en-US']]) {
  test(`changing ${first} to ${second} cancels old results for the same draft`, async t => {
    let release;
    let firstSignal;
    const calls = [];
    const { states, session } = setup(t, async (_url, options) => {
      const body = JSON.parse(options.body);
      calls.push(body.language);
      if (calls.length === 1) {
        firstSignal = options.signal;
        return new Promise(resolve => { release = resolve; });
      }
      return Response.json({ misspelled: [] });
    });
    session.update('colour', { language: first, ...limits });
    t.mock.timers.tick(300);
    assert.ok(firstSignal, 'the first dialect check must start');
    session.update('colour', { language: second, ...limits });
    assert.equal(firstSignal.aborted, true);
    assert.deepEqual(states.at(-1).issues, []);
    t.mock.timers.tick(300);
    await flush();
    release(Response.json({ misspelled: [{ word: 'colour', suggestions: ['color'] }] }));
    await flush();
    assert.deepEqual(calls, [first, second]);
    assert.equal(states.at(-1).phase, 'ready');
    assert.deepEqual(states.at(-1).issues, []);
  });
}
test('disposing a session prevents delayed requests', async t => {
  let calls = 0;
  const { states, session } = setup(t, async () => { calls++; return Response.json({ misspelled: [] }); });
  session.update('teh', limits);
  assert.equal(states.at(-1).phase, 'checking');
  session.dispose();
  t.mock.timers.tick(1000);
  await flush();
  assert.equal(calls, 0);
});
