export function tokenize(text) {
  const matches = text.matchAll(/https?:\/\/\S+|www\.\S+|[^\s@]+@[^\s@]+|[\p{L}\p{N}][\p{L}\p{M}\p{N}]*(?:['’][\p{L}\p{M}]+)*/giu);
  const tokens = [];
  for (const match of matches) {
    const word = match[0].normalize('NFC');
    if (!/^\p{L}+(?:['’]\p{L}+)*$/u.test(word)) continue;
    tokens.push({ word, from: match.index, to: match.index + match[0].length });
  }
  return tokens;
}

export function replaceWord(text, word, suggestion) {
  for (const token of tokenize(text).reverse()) {
    if (token.word === word) text = text.slice(0, token.from) + suggestion + text.slice(token.to);
  }
  return text;
}

// Map UTF-16 offsets through the same replacements as replaceWord. A caret
// inside a corrected word lands at its end; positions elsewhere stay relative
// to their original text, including when several occurrences change length.
export function selectionAfterReplacement(text, word, suggestion, selection) {
  const matches = tokenize(text).filter(token => token.word === word);
  function mapOffset(offset) {
    let delta = 0;
    for (const token of matches) {
      if (offset <= token.from) break;
      if (offset < token.to) return token.from + delta + suggestion.length;
      delta += suggestion.length - (token.to - token.from);
    }
    return offset + delta;
  }
  return { start: mapOffset(selection.start), end: mapOffset(selection.end) };
}

export function highlightParts(text, issues = []) {
  const errors = new Set(issues.map(issue => issue.word));
  const parts = [];
  let position = 0;
  for (const token of tokenize(text)) {
    if (!errors.has(token.word)) continue;
    parts.push({ text: text.slice(position, token.from), error: false });
    parts.push({ text: text.slice(token.from, token.to), error: true, word: token.word, from: token.from });
    position = token.to;
  }
  parts.push({ text: text.slice(position), error: false });
  return parts;
}
