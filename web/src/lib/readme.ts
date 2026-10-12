import MarkdownIt, { type MarkdownIt as Md, type StateCore, type Token } from "markdown-it";

export const ALERT_KINDS = ["note", "tip", "important", "warning", "caution"] as const;
export type AlertKind = (typeof ALERT_KINDS)[number];

export function headingSlug(text: string): string {
  return text
    .trim()
    .toLowerCase()
    .replace(/[^\p{L}\p{N}\s_-]/gu, "")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-");
}

export function uniqueSlug(base: string, used: Map<string, number>): string {
  const n = used.get(base);
  if (n === undefined) {
    used.set(base, 0);
    return base;
  }
  let next = n + 1;
  while (used.has(`${base}-${next}`)) next++;
  used.set(base, next);
  used.set(`${base}-${next}`, 0);
  return `${base}-${next}`;
}

export function taskMarker(text: string): { checked: boolean; rest: string } | null {
  const m = /^\[([ xX])\][ \t]+/.exec(text);
  if (!m) return null;
  return { checked: m[1] !== " ", rest: text.slice(m[0].length) };
}

export function alertKind(firstLine: string): AlertKind | null {
  const m = /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\][ \t]*$/.exec(firstLine);
  return m ? (m[1]!.toLowerCase() as AlertKind) : null;
}

export type ReadmeSource = {
  webUrl?: string | null;
  branch: string | null;
  kind?: "github" | "gitlab" | "forgejo" | "gitea" | null;
  dir?: string;
  docLink?: (path: string) => string | null;
  alerts?: Partial<Record<AlertKind, string>>;
  anchorLabel?: string;
};

type ForgeSource = ReadmeSource & { webUrl: string; kind: NonNullable<ReadmeSource["kind"]> };

const SCHEME = /^[a-z][a-z0-9+.-]*:/i;

export function forgeBases(src: ForgeSource): { blob: string; raw: string } {
  const root = src.webUrl.replace(/\/+$/, "");
  const branch = encodeURIComponent(src.branch ?? "HEAD").replace(/%2F/gi, "/");
  const dir = src.dir ? `${src.dir.split("/").map(encodeURIComponent).join("/")}/` : "";
  switch (src.kind) {
    case "gitlab":
      return { blob: `${root}/-/blob/${branch}/${dir}`, raw: `${root}/-/raw/${branch}/${dir}` };
    case "gitea":
    case "forgejo":
      return { blob: `${root}/src/branch/${branch}/${dir}`, raw: `${root}/raw/branch/${branch}/${dir}` };
    default:
      return { blob: `${root}/blob/${branch}/${dir}`, raw: `${root}/raw/${branch}/${dir}` };
  }
}

function allowed(url: string): boolean {
  const u = url.trim();
  if (!SCHEME.test(u)) return !u.startsWith("//");
  return /^(https?|mailto):/i.test(u);
}

function resolve(url: string, base: string): string {
  if (SCHEME.test(url) || url.startsWith("#")) return url;
  try {
    return new URL(url.replace(/^\.\//, ""), base).toString();
  } catch {
    return url;
  }
}

function isRelative(url: string): boolean {
  return !SCHEME.test(url) && !url.startsWith("#") && !url.startsWith("//");
}

export function repoPath(url: string, dir?: string): string {
  const base = `https://repo.invalid/${dir ? `${dir}/` : ""}`;
  try {
    return decodeURIComponent(new URL(url, base).pathname.slice(1));
  } catch {
    return url;
  }
}

const escapeHtml = (s: string) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]!);

function inlineText(token: Token | undefined): string {
  return (token?.children ?? [])
    .filter((c) => c.type === "text" || c.type === "code_inline")
    .map((c) => c.content)
    .join("");
}

function extend(md: Md, src: ReadmeSource) {
  md.core.ruler.push("svcr_extensions", (state: StateCore) => {
    const tokens = state.tokens;
    const used = new Map<string, number>();
    for (let i = 0; i < tokens.length; i++) {
      const tok = tokens[i]!;
      if (tok.type === "heading_open") {
        const slug = uniqueSlug(headingSlug(inlineText(tokens[i + 1])) || "section", used);
        tok.attrSet("id", slug);
        const inline = tokens[i + 1];
        if (inline?.type === "inline" && src.anchorLabel !== undefined) {
          const anchor = new state.Token("html_inline", "", 0);
          anchor.content = ` <a class="heading-anchor" href="#${encodeURIComponent(slug)}" aria-label="${escapeHtml(src.anchorLabel)}">#</a>`;
          (inline.children ??= []).push(anchor);
        }
      }
      if (tok.type === "list_item_open" && tokens[i + 1]?.type === "paragraph_open" && tokens[i + 2]?.type === "inline") {
        const inline = tokens[i + 2]!;
        const first = inline.children?.[0];
        const marker = first?.type === "text" ? taskMarker(first.content) : null;
        if (first && marker) {
          first.content = marker.rest;
          inline.content = inline.content.replace(/^\[[ xX]\][ \t]+/, "");
          const box = new state.Token("html_inline", "", 0);
          const label = escapeHtml(inlineText(inline));
          box.content = `<input type="checkbox" class="task-list-item-checkbox" disabled aria-label="${label}"${marker.checked ? " checked" : ""}> `;
          inline.children!.unshift(box);
          tok.attrJoin("class", "task-list-item");
          for (let j = i - 1; j >= 0; j--) {
            const t = tokens[j]!;
            if ((t.type === "bullet_list_open" || t.type === "ordered_list_open") && t.level === tok.level - 1) {
              if (!String(t.attrGet("class") ?? "").includes("contains-task-list")) t.attrJoin("class", "contains-task-list");
              break;
            }
          }
        }
      }
      if (tok.type === "blockquote_open" && tokens[i + 1]?.type === "paragraph_open" && tokens[i + 2]?.type === "inline") {
        const inline = tokens[i + 2]!;
        const kind = alertKind(inline.content.split("\n")[0] ?? "");
        if (!kind) continue;
        const children = inline.children ?? [];
        const brk = children.findIndex((c: Token) => c.type === "softbreak" || c.type === "hardbreak");
        const rest = brk < 0 ? [] : children.slice(brk + 1);
        tok.attrJoin("class", `markdown-alert markdown-alert-${kind}`);
        const title = new state.Token("html_block", "", 0);
        title.content = `<p class="markdown-alert-title">${escapeHtml(src.alerts?.[kind] ?? kind)}</p>\n`;
        if (rest.length === 0) {
          tokens.splice(i + 1, 3, title);
        } else {
          inline.children = rest;
          inline.content = inline.content.split("\n").slice(1).join("\n");
          tokens.splice(i + 1, 0, title);
        }
      }
    }
  });
  md.renderer.rules.table_open = () => '<div class="table-wrap"><table>\n';
  md.renderer.rules.table_close = () => "</table></div>\n";
}

export function renderReadme(markdown: string, src: ReadmeSource): string {
  const md = new MarkdownIt({ html: false, linkify: true });
  md.validateLink = allowed;
  extend(md, src);
  const bases = src.webUrl && src.kind ? forgeBases(src as ForgeSource) : null;
  const rules = md.renderer.rules;
  const dropped: boolean[] = [];
  rules.link_open = (tokens, idx, options, _env, self) => {
    const token = tokens[idx]!;
    const href = String(token.attrGet("href") ?? "");
    if (isRelative(href)) {
      const inApp = src.docLink?.(repoPath(href, src.dir)) ?? null;
      if (inApp !== null) {
        token.attrSet("href", inApp);
        dropped.push(false);
        return self.renderToken(tokens, idx, options);
      }
      if (!bases) {
        dropped.push(true);
        return "";
      }
      token.attrSet("href", resolve(href, bases.blob));
    }
    dropped.push(false);
    if (!href.startsWith("#")) {
      token.attrSet("target", "_blank");
      token.attrSet("rel", "noopener nofollow");
    }
    return self.renderToken(tokens, idx, options);
  };
  rules.link_close = (tokens, idx, options, _env, self) => (dropped.pop() ? "" : self.renderToken(tokens, idx, options));
  const image = rules.image;
  rules.image = (tokens, idx, options, env, self) => {
    const token = tokens[idx]!;
    const url = String(token.attrGet("src") ?? "");
    if (isRelative(url) && !bases) return escapeHtml(self.renderInlineAsText(token.children ?? [], options, env));
    token.attrSet("src", bases ? resolve(url, bases.raw) : url);
    token.attrSet("loading", "lazy");
    return image ? image(tokens, idx, options, env, self) : self.renderToken(tokens, idx, options);
  };
  return md.render(markdown);
}
