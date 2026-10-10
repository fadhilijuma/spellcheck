import { test } from 'node:test';
import assert from 'node:assert/strict';
import { tokenize, replaceWord, highlightParts, selectionAfterReplacement } from './tokenize.js';

const cases = [
  ['word offsets', 'Teh speling', [{ word: 'Teh', from: 0, to: 3 }, { word: 'speling', from: 4, to: 11 }]],
  ['skipped nonwords', 'NASA 123 a1 me@teh.com HTTPS://teh.com hello', [{ word: 'NASA', from: 0, to: 4 }, { word: 'hello', from: 39, to: 44 }]],
  ['uppercase spelling', 'TEH RECIEVE SPELING', [{ word: 'TEH', from: 0, to: 3 }, { word: 'RECIEVE', from: 4, to: 11 }, { word: 'SPELING', from: 12, to: 19 }]],
  ['normalization and UTF-16 offsets', '🙂 cafe\u0301', [{ word: 'café', from: 3, to: 8 }]],
  ['apostrophes and hyphens', 'don’t well-known', [{ word: 'don’t', from: 0, to: 5 }, { word: 'well', from: 6, to: 10 }, { word: 'known', from: 11, to: 16 }]],
];
const selections = [
  ['caret after corrected word', 'speling is easy', 'speling', 'spelling', { start: 7, end: 7 }, { start: 8, end: 8 }],
  ['caret inside corrected word', 'speling is easy', 'speling', 'spelling', { start: 3, end: 3 }, { start: 8, end: 8 }],
  ['caret before corrected word', 'the speling', 'speling', 'spelling', { start: 2, end: 2 }, { start: 2, end: 2 }],
  ['range spanning replacements', 'speling and speling', 'speling', 'spelling', { start: 0, end: 19 }, { start: 0, end: 21 }],
  ['caret between repeated words', 'speling and speling', 'speling', 'spelling', { start: 10, end: 10 }, { start: 11, end: 11 }],
  ['shorter correction', 'helllo world', 'helllo', 'hello', { start: 12, end: 12 }, { start: 11, end: 11 }],
  ['normalized Unicode offsets', '🙂 cafe\u0301 end', 'café', 'cafe', { start: 12, end: 12 }, { start: 11, end: 11 }],
];
for (const [name, text, word, suggestion, selection, expected] of selections) {
  test(name, () => assert.deepEqual(selectionAfterReplacement(text, word, suggestion, selection), expected));
}
for (const [name, text, expected] of cases) {
  test(name, () => assert.deepEqual(tokenize(text), expected));
}
test('corrections preserve case, boundaries, URLs and punctuation', () => {
  assert.equal(replaceWord('Teh, Teheran teh Teh https://x/Teh', 'Teh', 'The'), 'The, Teheran teh The https://x/Teh');
});
test('normalized correction preserves UTF-16 positions', () => {
  assert.equal(replaceWord('🙂 cafe\u0301 cafe\u0301!', 'café', 'cafe'), '🙂 cafe cafe!');
});
test('highlight parts preserve all text without interpreting markup', () => {
  const text = '<script>Teh</script>';
  const parts = highlightParts(text, [{ word: 'Teh' }]);
  assert.equal(parts.map(part => part.text).join(''), text);
  assert.deepEqual(parts.filter(part => part.error).map(part => part.text), ['Teh']);
});
