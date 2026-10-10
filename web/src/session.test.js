import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createSession } from './session.js';

const flush = () => new Promise(resolve => setImmediate(resolve));
function setup(t, fetcher) {
  t.mock.timers.enable({ apis: ['setTimeout'] });
  const states = [];
  const session = createSession(state => states.push(state), { fetcher });
  t.after(() => session.dispose());
  return { states, session };
}
test('debounces typing and publishes only the latest document', async t => {
  const calls = [];
  const { states, session } = setup(t, async (_url, options) => {
    calls.push(JSON.parse(options.body).words);
    return Response.json({ misspelled: [] });
  });
  session.update('teh');
  t.mock.timers.tick(200);
  session.update('hello');
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
  session.update('teh');
  t.mock.timers.tick(300);
  session.update('hello');
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
  session.update('teh');
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
  session.update('hello');
  t.mock.timers.tick(300);
  await flush();
  assert.equal(states.at(-1).phase, 'offline');
  offline = false;
  session.update('hello world');
  t.mock.timers.tick(300);
  await flush();
  assert.equal(states.at(-1).phase, 'ready');
});
test('composition waits and empty input clears immediately', async t => {
  let calls = 0;
  const { states, session } = setup(t, async () => { calls++; return Response.json({ misspelled: [] }); });
  session.update('teh', true);
  t.mock.timers.tick(300);
  await flush();
  assert.equal(calls, 0);
  session.update('');
  assert.equal(states.at(-1).phase, 'empty');
  assert.deepEqual(states.at(-1).issues, []);
});
test('disposing a session prevents delayed requests', async t => {
  let calls = 0;
  const { states, session } = setup(t, async () => { calls++; return Response.json({ misspelled: [] }); });
  session.update('teh');
  assert.equal(states.at(-1).phase, 'checking');
  session.dispose();
  t.mock.timers.tick(1000);
  await flush();
  assert.equal(calls, 0);
});
