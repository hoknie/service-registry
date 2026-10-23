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

const HINTS = ["metadata_hosts", "metadata_disabled", "metadata_unavailable", "metadata_invalid"] as const;

const REASONS = {
  metadata_unavailable: [
    "timeout",
    "dns",
    "tls",
    "blocked_address",
    "connection",
    "redirect",
    "too_large",
    "rate_limited",
    "fetch_failed",
  ],
  metadata_invalid: [
    "not_json",
    "client_id_mismatch",
    "missing_client_name",
    "has_secret",
    "bad_redirect_uri",
    "auth_method",
    "grant_types",
    "response_types",
  ],
} as const;

export type ConsentReason = (typeof REASONS)[keyof typeof REASONS][number] | `status_${number}`;

export type ConsentHint = { kind: (typeof HINTS)[number]; host: string; reason?: ConsentReason };

function hintReason(kind: (typeof HINTS)[number], raw: string | null): ConsentReason | null {
  if (kind === "metadata_unavailable" && raw && /^status_[1-5][0-9]{2}$/.test(raw)) return raw as ConsentReason;
  if (kind !== "metadata_unavailable" && kind !== "metadata_invalid") return null;
  const known: readonly string[] = REASONS[kind];
  return raw && known.includes(raw) ? (raw as ConsentReason) : null;
}

export function consentParams(search: string): { request: string | null; error: string | null; hint: ConsentHint | null } {
  const q = new URLSearchParams(search);
  const kind = HINTS.find((h) => h === q.get("hint"));
  const host = q.get("host");
  let hint: ConsentHint | null = null;
  if (kind && host) {
    const reason = hintReason(kind, q.get("reason"));
    if (kind === "metadata_hosts" || kind === "metadata_disabled") hint = { kind, host };
    else if (reason) hint = { kind, host, reason };
  }
  return { request: q.get("request"), error: q.get("error"), hint };
}

export function reasonText(reasons: Record<string, string>, reason: ConsentReason): string {
  if (reason.startsWith("status_")) return (reasons.status ?? "").replace("{status}", reason.slice("status_".length));
  return reasons[reason] ?? reason;
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
