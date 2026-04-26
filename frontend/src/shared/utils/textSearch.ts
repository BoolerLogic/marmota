export type HighlightSegment = {
  text: string;
  match: boolean;
};

function normalizeQuery(query: string): string {
  return query.trim();
}

export function countMatches(text: string, query: string): number {
  const normalizedQuery = normalizeQuery(query);
  if (normalizedQuery.length === 0) return 0;

  const haystack = text.toLocaleLowerCase();
  const needle = normalizedQuery.toLocaleLowerCase();

  let count = 0;
  let cursor = 0;

  while (cursor < haystack.length) {
    const index = haystack.indexOf(needle, cursor);
    if (index === -1) break;

    count += 1;
    cursor = index + needle.length;
  }

  return count;
}

export function splitHighlightedText(
  text: string,
  query: string
): HighlightSegment[] {
  const normalizedQuery = normalizeQuery(query);
  if (normalizedQuery.length === 0 || text.length === 0) {
    return [{ text, match: false }];
  }

  const haystack = text.toLocaleLowerCase();
  const needle = normalizedQuery.toLocaleLowerCase();
  const segments: HighlightSegment[] = [];
  let cursor = 0;

  while (cursor < text.length) {
    const index = haystack.indexOf(needle, cursor);
    if (index === -1) {
      segments.push({
        text: text.slice(cursor),
        match: false
      });
      break;
    }

    if (index > cursor) {
      segments.push({
        text: text.slice(cursor, index),
        match: false
      });
    }

    segments.push({
      text: text.slice(index, index + needle.length),
      match: true
    });

    cursor = index + needle.length;
  }

  return segments;
}
