export const DOC_ROOT = "/doc";
export const DOC_NAVIGATE = "svcr:doc-navigate";

export type DocLocation = { segments: string[]; branch: string | null };

export function parseDocLocation(pathname: string, search: string): DocLocation {
  const rest = pathname === DOC_ROOT ? "" : pathname.startsWith(`${DOC_ROOT}/`) ? pathname.slice(DOC_ROOT.length + 1) : "";
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
  const branch = new URLSearchParams(search).get("branch");
  return { segments, branch: branch === "" ? null : branch };
}

export function docHref(segments: readonly string[], branch?: string | null): string {
  const path = segments.map((s) => encodeURIComponent(s)).join("/");
  const query = branch ? `?branch=${encodeURIComponent(branch)}` : "";
  return `${DOC_ROOT}${path ? `/${path}` : ""}${query}`;
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
