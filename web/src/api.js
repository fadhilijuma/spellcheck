import { tokenize } from './tokenize.js';

const api = (import.meta.env?.VITE_SERVICE_API || '/v1').replace(/\/$/, '');

export async function getConfiguration({ signal, fetcher = fetch } = {}) {
  const response = await fetcher(`${api}/config`, { signal });
  if (!response.ok) throw new Error('Spellcheck unavailable');
  const data = await response.json();
  signal?.throwIfAborted();
  if (!['en-US', 'en-GB'].includes(data?.defaultLanguage)) throw new Error('Invalid configuration response');
  for (const field of ['maxWordLen', 'maxWords', 'maxBodyBytes']) {
    if (!Number.isSafeInteger(data[field]) || data[field] <= 0) throw new Error('Invalid configuration response');
  }
  return { defaultLanguage: data.defaultLanguage, maxWordLen: data.maxWordLen, maxWords: data.maxWords, maxBodyBytes: data.maxBodyBytes };
}

// Request limits come from getConfiguration, validated once when loaded.
export async function checkDocument(text, { signal, fetcher = fetch, language, maxWordLen, maxWords, maxBodyBytes } = {}) {
	signal?.throwIfAborted();
  const words = [...new Set(tokenize(text).map(token => token.word))];
  const allowed = [];
  let skipped = 0;
  for (const word of words) {
    if ([...word].length > maxWordLen) skipped++;
    else allowed.push(word);
  }
  const issues = new Map();
  const encoder = new TextEncoder();
  const envelopeBytes = encoder.encode(JSON.stringify({ words: [], language })).length;
  let offset = 0;
  while (offset < allowed.length) {
    signal?.throwIfAborted();
    const batch = [];
    let bytes = envelopeBytes;
    // Track encoded-word bytes plus the commas between words.
    while (offset < allowed.length && batch.length < maxWords) {
      const word = allowed[offset];
      const wordBytes = encoder.encode(JSON.stringify(word)).length + (batch.length ? 1 : 0);
      if (bytes + wordBytes > maxBodyBytes) break;
      batch.push(word);
      bytes += wordBytes;
      offset++;
    }
    if (!batch.length) throw new Error('Word exceeds request-body limit');
    const response = await fetcher(`${api}/check`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ words: batch, language }),
      signal,
    });
    if (!response.ok) throw new Error('Spellcheck unavailable');
    const data = await response.json();
    signal?.throwIfAborted();
    if (!data || !Array.isArray(data.misspelled)) throw new Error('Invalid spelling response');
    const sent = new Set(batch);
    for (const issue of data.misspelled) {
      const knownWord = issue && sent.has(issue.word);
      const suggestions = Array.isArray(issue?.suggestions) && issue.suggestions.every(value => typeof value === 'string');
      if (!knownWord || !suggestions) throw new Error('Invalid spelling response');
      issues.set(issue.word, { word: issue.word, suggestions: issue.suggestions });
    }
  }
  return { issues: [...issues.values()], skipped };
}
