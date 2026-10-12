// Match whole words, not substrings; case and apostrophe style are equivalent.
export function personalWord(value, maxWordLen = Infinity) {
  if (typeof value !== 'string' || [...value].length > maxWordLen || !/^\p{L}+(?:['’]\p{L}+)*$/u.test(value)) return null;
  return value.toLowerCase().replaceAll('’', "'");
}

export function parsePersonalWords(value) {
  try {
    const words = JSON.parse(value);
    // Preserve accepted words when a later server configuration lowers its limit.
    return Array.isArray(words) ? [...new Set(words.map(word => personalWord(word)).filter(Boolean))] : [];
  } catch {
    return [];
  }
}
