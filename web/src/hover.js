export function hitWord(x, y, words) {
  return words.find(word => word.rects.some(rect => x >= rect.left && x <= rect.right && y >= rect.top && y <= rect.bottom)) || null;
}

export function popupPosition(rect, viewport) {
  const left = Math.max(8, Math.min(rect.left, viewport.width - 248));
  const below = rect.bottom + 8;
  const top = below + 120 <= viewport.height - 8 ? below : Math.max(8, rect.top - 128);
  return { left, top };
}
