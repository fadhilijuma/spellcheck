import { tokenize } from './tokenize.js';

const api = (import.meta.env?.VITE_SERVICE_API || '/v1').replace(/\/$/, '');

export async function checkDocument(text, { signal, fetcher = fetch } = {}) {
  const words = [...new Set(tokenize(text).map(token => token.word))];
  const skipped = new Set(words.filter(word => [...word].length > 64));
  const allowed = words.filter(word => !skipped.has(word));
  const issues = new Map();
  // Batches of 200 stay well inside the 500-word request limit.
  for (let offset = 0; offset < allowed.length; offset += 200) {
    signal?.throwIfAborted();
    const batch = allowed.slice(offset, offset + 200);
    const response = await fetcher(`${api}/check`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ words: batch }),
      signal,
    });
    if (!response.ok) throw new Error('Spellcheck unavailable');
    const data = await response.json();
    signal?.throwIfAborted();
    if (!data || !Array.isArray(data.misspelled)) throw new Error('Invalid spelling response');
    for (const issue of data.misspelled) {
      const knownWord = issue && batch.includes(issue.word);
      const suggestions = Array.isArray(issue?.suggestions) && issue.suggestions.every(value => typeof value === 'string');
      if (!knownWord || !suggestions) throw new Error('Invalid spelling response');
      issues.set(issue.word, { word: issue.word, suggestions: issue.suggestions });
    }
  }
  return { issues: [...issues.values()], skipped: skipped.size };
}
