import { test } from 'node:test';
import assert from 'node:assert/strict';
import { hitWord, popupPosition } from './hover.js';

const word = { word: 'speling', rects: [{ left: 20, right: 80, top: 40, bottom: 60 }, { left: 10, right: 30, top: 60, bottom: 80 }] };
for (const [name, x, y, expected] of [
  ['first line', 40, 45, word],
  ['wrapped line', 12, 70, word],
  ['outside the word', 100, 45, null],
]) {
  test(name, () => assert.equal(hitWord(x, y, [word]), expected));
}
test('popup fits the right edge', () => {
  assert.deepEqual(popupPosition({ left: 370, top: 50, bottom: 70 }, { width: 390, height: 844 }), { left: 142, top: 78 });
});
test('popup opens above the bottom edge', () => {
  assert.deepEqual(popupPosition({ left: 20, top: 780, bottom: 800 }, { width: 390, height: 844 }), { left: 20, top: 652 });
});
test('popup clamps narrow viewports', () => {
  assert.deepEqual(popupPosition({ left: -10, top: 5, bottom: 25 }, { width: 200, height: 100 }), { left: 8, top: 8 });
});
