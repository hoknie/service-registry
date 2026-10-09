import ts from "typescript";

const DIRECTIVES = [/^\/\/\s*eslint-/, /^\/\*\s*eslint-/, /^\/\/\s*@ts-/, /^\/\/\/\s*<reference/];

export const isDirective = (text) => DIRECTIVES.some((re) => re.test(text));

export function scriptComments(fileName, text) {
  const kind = fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : fileName.endsWith(".ts") ? ts.ScriptKind.TS : ts.ScriptKind.JS;
  const source = ts.createSourceFile(fileName, text, ts.ScriptTarget.Latest, true, kind);
  const seen = new Map();
  const add = (ranges) => {
    for (const r of ranges ?? []) if (!seen.has(r.pos)) seen.set(r.pos, { pos: r.pos, end: r.end, text: text.slice(r.pos, r.end) });
  };
  const visit = (node, inJsxText) => {
    if (!inJsxText) {
      add(ts.getLeadingCommentRanges(text, node.getFullStart()));
      add(ts.getTrailingCommentRanges(text, node.getEnd()));
    }
    for (const child of node.getChildren(source)) visit(child, child.kind === ts.SyntaxKind.JsxText);
  };
  visit(source, false);
  const jsxEmpty = [];
  const jsxText = [];
  const findJsx = (node) => {
    if (node.kind === ts.SyntaxKind.JsxExpression && !node.expression) jsxEmpty.push({ pos: node.getStart(source), end: node.getEnd() });
    if (node.kind === ts.SyntaxKind.JsxText) jsxText.push({ pos: node.pos, end: node.end });
    for (const child of node.getChildren(source)) findJsx(child);
  };
  findJsx(source);
  const inText = (c) => jsxText.some((r) => c.pos >= r.pos && c.pos < r.end);
  const comments = [...seen.values()].filter((c) => !inText(c)).sort((a, b) => a.pos - b.pos);
  return { comments, jsxEmpty };
}

export function cssComments(text) {
  const out = [];
  let i = 0;
  while (i < text.length) {
    const c = text[i];
    if (c === '"' || c === "'") {
      const q = c;
      i++;
      while (i < text.length && text[i] !== q) i += text[i] === "\\" ? 2 : 1;
      i++;
    } else if (c === "/" && text[i + 1] === "*") {
      const end = text.indexOf("*/", i + 2);
      const stop = end < 0 ? text.length : end + 2;
      out.push({ pos: i, end: stop, text: text.slice(i, stop) });
      i = stop;
    } else i++;
  }
  return out;
}
