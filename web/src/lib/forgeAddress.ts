export type ForgeKind = "github" | "gitlab" | "gitea" | "forgejo";

const LOCAL = new Set(["localhost", "127.0.0.1", "[::1]"]);

function parse(raw: string): URL | null {
  const v = raw.trim().replace(/\/+$/, "");
  if (!v || v.length > 2048) return null;
  try {
    const u = new URL(v);
    if (!u.host || u.username || u.password || u.search || u.hash) return null;
    return u;
  } catch {
    return null;
  }
}

export function guessForge(raw: string): ForgeKind | null {
  const u = parse(raw);
  if (!u) return null;
  if (u.hostname === "github.com") return "github";
  if (u.hostname === "gitlab.com") return "gitlab";
  return null;
}

export function cleanRepoUrl(raw: string): string | null {
  const u = parse(raw);
  if (!u) return null;
  if (u.protocol !== "https:" && !(u.protocol === "http:" && LOCAL.has(u.hostname))) return null;
  const p = u.pathname.replace(/^\/+|\/+$/g, "").replace(/\.git$/, "");
  if (!p.includes("/") || p.includes("//") || p.split("/").some((s) => s === "." || s === "..")) return null;
  return `${u.protocol}//${u.host}/${p}`;
}

export function sourceApiUrl(kind: string, repoUrl: string): string | null {
  const web = cleanRepoUrl(repoUrl);
  if (!web || !kind) return null;
  const u = new URL(web);
  const base = `${u.protocol}//${u.host}`;
  if (kind === "github" && u.hostname === "github.com") return "https://api.github.com";
  if (kind === "github") return `${base}/api/v3`;
  if (kind === "gitlab") return `${base}/api/v4`;
  return `${base}/api/v1`;
}

export type OwnerAddress = { kind: ForgeKind | null; apiUrl: string | null; ownerPath: string };

export function parseOwnerUrl(raw: string, kind: string): OwnerAddress | null {
  const u = parse(raw);
  if (!u || (u.protocol !== "https:" && u.protocol !== "http:")) return null;
  const ownerPath = u.pathname.replace(/^\/+|\/+$/g, "");
  if (!ownerPath) return null;
  const guessed = guessForge(raw);
  const k = guessed ?? (kind || null);
  const base = `${u.protocol}//${u.host}`;
  let apiUrl: string | null = null;
  if (k === "github") apiUrl = u.hostname === "github.com" ? "https://api.github.com" : `${base}/api/v3`;
  else if (k === "gitlab") apiUrl = base;
  else if (k === "gitea" || k === "forgejo") apiUrl = `${base}/api/v1`;
  return { kind: guessed, apiUrl, ownerPath };
}

export function ownerUrlOf(apiUrl: string, ownerPath: string): string {
  const u = parse(apiUrl);
  if (!u || !ownerPath) return "";
  const host = u.hostname === "api.github.com" ? "github.com" : u.host;
  return `${u.protocol}//${host}/${ownerPath.replace(/^\/+|\/+$/g, "")}`;
}
