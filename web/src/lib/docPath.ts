export const DOC_ROOT = "/doc";
export const DOC_NAVIGATE = "svcr:doc-navigate";

let base = DOC_ROOT;

export function setDocBase(next: string): void {
  base = next;
}

export function docBase(): string {
  return base;
}

export function publicRoot(pathname: string): { root: string | null; base: string } {
  const prefix = "/public/docs";
  if (pathname === prefix || pathname.startsWith(`${prefix}/`)) {
    const root = pathname.slice(prefix.length + 1).split("/")[0] ?? "";
    let decoded = root;
    try {
      decoded = decodeURIComponent(root);
    } catch {
      decoded = root;
    }
    return { root: decoded || null, base: root ? `${prefix}/${root}` : prefix };
  }
  return { root: null, base: "" };
}

export type DocLocation = { segments: string[]; branch: string | null };

export function parseDocLocation(pathname: string, search: string): DocLocation {
  const rest = pathname === base ? "" : pathname.startsWith(`${base}/`) ? pathname.slice(base.length + 1) : "";
  const segments = rest
    .split("/")
    .filter((s) => s !== "")
    .map((s) => {
      try {
        return decodeURIComponent(s);
      } catch {
        return s;
      }
    });
  const params = new URLSearchParams(search);
  const tag = params.get("tag");
  if (tag) return { segments, branch: `${TAG_KEY}${tag}` };
  const branch = params.get("branch");
  return { segments, branch: branch === "" ? null : branch };
}

const TAG_KEY = "tag:";

export function docHref(segments: readonly string[], branch?: string | null): string {
  const path = segments.map((s) => encodeURIComponent(s)).join("/");
  const tag = branch?.startsWith(TAG_KEY) ? branch.slice(TAG_KEY.length) : null;
  const query = tag ? `?tag=${encodeURIComponent(tag)}` : branch ? `?branch=${encodeURIComponent(branch)}` : "";
  const href = `${base}${path ? `/${path}` : ""}${query}`;
  return href.startsWith("/") ? href : `/${href}`;
}

export function splitPath(path: string): string[] {
  return path.split("/").filter((s) => s !== "");
}

export function isDocPath(pathname: string): boolean {
  return pathname === DOC_ROOT || pathname.startsWith(`${DOC_ROOT}/`);
}

export function startPage(paths: readonly string[]): string | null {
  const readme = paths.find((p) => !p.includes("/") && p.toLowerCase() === "readme.md");
  if (readme) return readme;
  const md = paths.filter((p) => /\.(md|markdown)$/i.test(p)).sort((a, b) => a.localeCompare(b));
  return md[0] ?? null;
}

export function navigateDoc(href: string, replace = false): void {
  if (replace) window.history.replaceState(null, "", href);
  else window.history.pushState(null, "", href);
  window.dispatchEvent(new Event(DOC_NAVIGATE));
}

export function fileName(path: string): string {
  return path.slice(path.lastIndexOf("/") + 1);
}

export function parentDir(path: string): string {
  const i = path.lastIndexOf("/");
  return i < 0 ? "" : path.slice(0, i);
}

export function isDirectory(paths: readonly string[], docPath: string | null): boolean {
  if (!docPath || paths.includes(docPath)) return false;
  return paths.some((p) => p.startsWith(`${docPath}/`));
}

export function docPathCrumbs(docPath: string | null): { name: string; path: string }[] {
  const parts = docPath ? splitPath(docPath) : [];
  return parts.map((name, i) => ({ name, path: parts.slice(0, i + 1).join("/") }));
}
