export const MCP_BASE = "/v1/mcp";

export type McpRef = { id: string; slug: string; name: string };

export type McpStatus = "draft" | "published" | "disabled";

export type McpState = "ok" | "failing" | "unknown" | "disabled";

export type McpSecretRef = {
  id: string;
  name: string;
  from: { id: string; name: string } | null;
};

export type McpAuthKind = "none" | "bearer" | "header";

export type McpAuth = {
  kind: McpAuthKind;
  header?: string;
  credentials?: { kind: "secret"; secret: McpSecretRef };
};

export type McpCheck = {
  at: string;
  outcome: "ok" | "failed";
  code: string | null;
  detail: string | null;
  duration_ms: number;
  truncated: boolean;
};

export type McpServer = {
  id: string;
  folder_id: string | null;
  organization_id: string | null;
  organization: McpRef | null;
  path: string;
  ancestors: McpRef[];
  slug: string;
  title: string;
  description: string;
  tags: string[];
  author: string;
  icon_url: string | null;
  icon: string | null;
  has_icon: boolean;
  log_payloads: boolean;
  status: McpStatus;
  state: McpState;
  tools_count: number;
  calls: number;
  auth: McpAuth;
  url?: string;
  transport?: "streamable" | "sse";
  server_info?: {
    name: string;
    title: string;
    version: string;
    protocol_version: string;
    instructions: string;
  };
  last_check?: McpCheck;
  can_write: boolean;
  can_manage: boolean;
  created_at: string;
  updated_at: string;
};

export type McpServerPage = { items: McpServer[]; next_cursor: string | null };

export type McpFolder = {
  id: string;
  parent_id: string | null;
  organization_id: string | null;
  organization: McpRef | null;
  slug: string;
  name: string;
  description: string;
  path: string;
  ancestors: McpRef[];
  servers: number;
  folders: number;
  can_write: boolean;
};

export type McpTool = {
  name: string;
  gateway_name: string;
  title: string;
  description: string;
  input_schema: unknown;
  output_schema: unknown;
  annotations: {
    readOnlyHint?: boolean;
    destructiveHint?: boolean;
    idempotentHint?: boolean;
  } | null;
  enabled: boolean;
  skipped: "name_invalid" | "schema_invalid" | null;
};

export type McpAccess = {
  everyone: boolean;
  groups: { id: string; name: string }[];
  users: { id: string; email: string; name?: string }[];
  can_manage: boolean;
};

export type McpServerRef = {
  id: string;
  slug: string;
  title: string;
  organization_id?: string | null;
  status?: McpStatus;
};

export type McpOrganizationSet = {
  mode: "all" | "selected";
  servers: McpServerRef[];
  can_manage: boolean;
};

export type McpSpace = {
  organization: McpRef;
  description: string;
  can_write: boolean;
  can_manage: boolean;
};

export type McpProxyMode = "direct" | "environment" | "proxy";

export type McpProxy = {
  mode: McpProxyMode;
  url: string | null;
  no_proxy: string | null;
  credentials: { kind: "secret"; secret: McpSecretRef } | null;
};

export type McpProxySetting = {
  own: McpProxy | null;
  effective: McpProxy;
  from: {
    kind: "organization" | "folder" | "server";
    id: string;
    name: string;
  } | null;
};

export type McpCall = {
  id: string;
  at: string;
  tool: string;
  user: { id: string; email: string; name: string } | null;
  via: { kind: "token" | "app"; id: string; name: string | null } | null;
  duration_ms: number;
  outcome: "ok" | "tool_error" | "failed";
  code: string | null;
  request: string | null;
  response: string | null;
  truncated: boolean;
};

export type McpVisibility = {
  organizations: {
    id: string;
    slug: string;
    name: string;
    mode: "all" | "selected";
    can_manage: boolean;
  }[];
  servers: {
    id: string;
    slug: string;
    title: string;
    organization_id: string | null;
    status: McpStatus;
    state: McpState;
    access: McpAccess;
    in: string[];
    can_manage: boolean;
  }[];
};

export type McpPlace = { folder: string | null; space: string | null };

export type McpFilters = { q: string; tags: string[]; state: string };

export function placeFrom(search: string): McpPlace {
  const q = new URLSearchParams(search);
  return { folder: q.get("folder"), space: q.get("space") };
}

export function filtersFrom(search: string): McpFilters {
  const q = new URLSearchParams(search);
  return {
    q: q.get("q") ?? "",
    tags: q.getAll("tag"),
    state: q.get("state") ?? "",
  };
}

export function searching(f: McpFilters): boolean {
  return f.q !== "" || f.tags.length > 0 || f.state !== "";
}

export function placeHref(
  locale: string,
  place: McpPlace,
  f?: McpFilters,
): string {
  const q = new URLSearchParams();
  if (place.folder) q.set("folder", place.folder);
  else if (place.space) q.set("space", place.space);
  if (f?.q) q.set("q", f.q);
  for (const t of f?.tags ?? []) q.append("tag", t);
  if (f?.state) q.set("state", f.state);
  const s = q.toString();
  return `/${locale}/mcp${s ? `?${s}` : ""}`;
}

export function listQuery(
  place: McpPlace,
  f: McpFilters,
  cursor?: string | null,
): `/${string}` {
  const q = new URLSearchParams();
  const deep = searching(f);
  if (place.folder) q.set("folder_id", place.folder);
  else {
    if (place.space) q.set("organization_id", place.space);
    if (!deep) q.set("folder_id", "root");
  }
  if (f.q) q.set("q", f.q);
  for (const t of f.tags) q.append("tag", t);
  if (f.state) q.set("state", f.state);
  if (cursor) q.set("cursor", cursor);
  return `${MCP_BASE}/servers?${q.toString()}` as `/${string}`;
}

export function spaceOf(organizationId: string | null): string {
  return organizationId ?? "global";
}

export const serverUrl = (id: string, rest = ""): `/${string}` =>
  `${MCP_BASE}/servers/${encodeURIComponent(id)}${rest}` as `/${string}`;

export function gatewayName(slug: string, tool: string): string {
  return `${slug}__${tool}`;
}

export function stateTone(
  state: McpState,
): "signal" | "danger" | "neutral" | "outline" {
  switch (state) {
    case "ok":
      return "signal";
    case "failing":
      return "danger";
    case "disabled":
      return "outline";
  }
  return "neutral";
}

export function mcpEndpoint(origin: string, publicUrl?: string | null): string {
  return `${(publicUrl || origin).replace(/\/+$/, "")}/api/mcp`;
}

export function agentConfig(endpoint: string, kind: "claude" | "json"): string {
  if (kind === "claude")
    return `claude mcp add --transport http registry ${endpoint} --header "Authorization: Bearer $SVCR_TOKEN"`;
  return JSON.stringify(
    {
      mcpServers: {
        registry: {
          type: "http",
          url: endpoint,
          headers: { Authorization: "Bearer ${SVCR_TOKEN}" },
        },
      },
    },
    null,
    2,
  );
}

export function serverListBody(ids: string[] | null): {
  servers: string[] | null;
} {
  return { servers: ids === null ? null : [...new Set(ids)] };
}

export function prettyPayload(text: string, truncated: boolean): string {
  if (truncated) return text;
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}
