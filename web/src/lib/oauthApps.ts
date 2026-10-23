export function loginHref(locale: string, pathname: string, search: string): string {
  return `/${locale}/login?next=${encodeURIComponent(pathname + search)}`;
}

export function claudeCommand(origin: string, token: string | null): string {
  const base = `claude mcp add --transport http registry ${origin}/api/mcp`;
  return token === null ? base : `${base} \\\n  --header "Authorization: Bearer ${token}"`;
}

export function appName(client: { name: string | null }, unnamed: string): string {
  return client.name ?? unnamed;
}

export function consentParams(search: string): { request: string | null; error: string | null } {
  const q = new URLSearchParams(search);
  return { request: q.get("request"), error: q.get("error") };
}

export const OAUTH_ERRORS = [
  "oauth.invalid_client",
  "oauth.invalid_redirect_uri",
  "oauth.client_metadata_unavailable",
  "oauth.client_metadata_invalid",
  "oauth.client_not_allowed",
] as const;

export function consentError(code: string | null): (typeof OAUTH_ERRORS)[number] | null {
  return OAUTH_ERRORS.find((c) => c === code) ?? null;
}

export function appHost(client: { redirect_host: string; metadata_host?: string | null }): string {
  return client.metadata_host ?? client.redirect_host;
}

export function isLoopbackRedirect(uri: string | undefined): boolean {
  if (!uri) return false;
  try {
    const u = new URL(uri);
    return u.protocol === "http:" && ["localhost", "127.0.0.1", "[::1]"].includes(u.hostname);
  } catch {
    return false;
  }
}
