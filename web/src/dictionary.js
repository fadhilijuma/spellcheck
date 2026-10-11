// Match whole words, not substrings; case and apostrophe style are equivalent.
export function personalWord(value) {
  if (typeof value !== 'string' || [...value].length > 64 || !/^\p{L}+(?:['’]\p{L}+)*$/u.test(value)) return null;
  return value.toLowerCase().replaceAll('’', "'");
}

export function parsePersonalWords(value) {
  try {
    const words = JSON.parse(value);
    return Array.isArray(words) ? [...new Set(words.map(personalWord).filter(Boolean))] : [];
  } catch {
    return [];
  }
}
