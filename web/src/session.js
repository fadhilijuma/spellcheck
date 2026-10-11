import { checkDocument } from './api.js';

export function createSession(onChange, { fetcher, delay = 300, timeout = 10000 } = {}) {
  let timer, deadline, controller;
  let generation = 0;

  function dispose() {
    generation++;
    clearTimeout(timer);
    clearTimeout(deadline);
    controller?.abort();
  }

  function update(text, { composing = false, language } = {}) {
    dispose();
    const current = generation;
    onChange({ phase: text.trim() ? 'checking' : 'empty', issues: [], skipped: 0 });
    if (!text.trim() || composing) return;
    timer = setTimeout(async () => {
      const request = new AbortController();
      controller = request;
      const limit = setTimeout(() => request.abort(), timeout);
      deadline = limit;
      try {
        const result = await checkDocument(text, { signal: request.signal, fetcher, language });
        if (current === generation) onChange({ phase: 'ready', ...result });
      } catch {
        if (current === generation) onChange({ phase: 'offline', issues: [], skipped: 0 });
      } finally {
        clearTimeout(limit);
        if (controller === request) controller = null;
      }
    }, delay);
  }

  return { update, dispose };
}
