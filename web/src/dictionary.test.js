import { test } from 'node:test';
import assert from 'node:assert/strict';
import { personalWord, parsePersonalWords } from './dictionary.js';

for (const row of [
  { name: 'case insensitive names', input: 'Fadhili', expected: 'fadhili' },
  { name: 'apostrophe variants', input: 'O’Connell', expected: "o'connell" },
  { name: 'Unicode letters', input: 'Élodie', expected: 'élodie' },
  { name: 'maximum length', input: 'a'.repeat(64), expected: 'a'.repeat(64) },
  ...['', 'a'.repeat(65), 'two words', 'JWT1', "'word", '<script>', 42, null].map(input => ({
    name: `reject invalid token ${JSON.stringify(input)}`, input, expected: null,
  })),
]) {
  test(row.name, () => assert.equal(personalWord(row.input, 64), row.expected));
}

for (const row of [
  { name: 'personal word obeys a smaller limit', word: 'Fadhili', maxWordLen: 3, expected: null },
  { name: 'personal word above the old limit', word: 'a'.repeat(65), maxWordLen: 80, expected: 'a'.repeat(65) },
  { name: 'personal word Unicode boundary', word: '𐐀é', maxWordLen: 2, expected: '𐐨é' },
]) {
  test(row.name, () => assert.equal(personalWord(row.word, row.maxWordLen), row.expected));
}

for (const row of [
  { name: 'deduplicates and validates stored words', input: '["Fadhili","fadhili","O’Connell",42,"two words"]', expected: ['fadhili', "o'connell"] },
  { name: 'preserves previously accepted long words', input: JSON.stringify(['a'.repeat(65)]), expected: ['a'.repeat(65)] },
  ...[null, '{}', 'null', 'not JSON', '[]'].map(input => ({ name: `ignores invalid or empty storage ${input}`, input, expected: [] })),
]) {
  test(row.name, () => assert.deepEqual(parsePersonalWords(row.input), row.expected));
}
