import MarkdownIt from "markdown-it";

export type ReadmeSource = {
  webUrl?: string | null;
  branch: string | null;
  kind?: "github" | "gitlab" | "forgejo" | "gitea" | null;
  dir?: string;
  docLink?: (path: string) => string | null;
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

export function renderReadme(markdown: string, src: ReadmeSource): string {
  const md = new MarkdownIt({ html: false, linkify: true });
  md.validateLink = allowed;
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
