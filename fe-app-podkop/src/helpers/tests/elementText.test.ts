import { readdirSync, readFileSync, statSync } from 'fs';
import { join } from 'path';

// LuCI's E() puts a child given as a single string into innerHTML, and one given in an
// array into a text node. The page shows text that comes from the router (names of
// links, the log, the sing-box configuration): it must never be read as HTML. So every
// E() whose children are not a literal, a translation, an element or an array of them
// gets its text in an array. A variable that holds elements is named rendered…

const SOURCE = join(__dirname, '..', '..');

function sources(directory: string): string[] {
  return readdirSync(directory).flatMap((name) => {
    const path = join(directory, name);
    if (statSync(path).isDirectory()) {
      return sources(path);
    }
    return name.endsWith('.ts') && !name.endsWith('.test.ts') ? [path] : [];
  });
}

// the arguments of a call starting after its opening parenthesis, split at top-level commas
function callArguments(source: string, start: number): string[] {
  const parts: string[] = [];
  let depth = 0;
  let from = start;
  for (let index = start; index < source.length; index++) {
    const character = source[index];
    if (character === "'" || character === '"' || character === '`') {
      index++;
      while (index < source.length && source[index] !== character) {
        if (source[index] === '\\') index++;
        index++;
      }
    } else if ('([{'.includes(character)) {
      depth++;
    } else if (')]}'.includes(character)) {
      if (depth === 0) {
        parts.push(source.slice(from, index));
        return parts;
      }
      depth--;
    } else if (character === ',' && depth === 0) {
      parts.push(source.slice(from, index));
      from = index + 1;
    }
  }
  return parts;
}

function safeChildren(children: string): boolean {
  const text = children.trim();
  return (
    text === '' ||
    text.startsWith('[') ||
    text.startsWith('E(') ||
    /^_\(\s*'[^']*'\s*\)$/.test(text) ||
    /^'[^']*'$/.test(text) ||
    /^render[A-Za-z]*\(/.test(text) ||
    // a list of elements made by the code around it
    /^rendered[A-Za-z]*$/.test(text) ||
    /\.map\(/.test(text)
  );
}

describe('the page shows data as text', () => {
  it('passes no single string to E() as its children', () => {
    const offenders: string[] = [];
    for (const path of sources(SOURCE)) {
      const source = readFileSync(path, 'utf8');
      const call = /\bE\(\s*['"][a-z0-9]+['"]\s*,/g;
      let match: RegExpExecArray | null;
      while ((match = call.exec(source)) !== null) {
        const args = callArguments(source, match.index + 2);
        if (args.length >= 3 && !safeChildren(args[2])) {
          const line = source.slice(0, match.index).split('\n').length;
          offenders.push(
            `${path.slice(SOURCE.length + 1)}:${line}: ${args[2].trim()}`,
          );
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});
