import { apiGet, apiSend } from "./api";
import {
  MCP_BASE,
  serverUrl,
  type McpAccess,
  type McpCall,
  type McpFolder,
  type McpOrganizationSet,
  type McpProxySetting,
  type McpServer,
  type McpServerPage,
  type McpSpace,
  type McpTool,
  type McpBuiltin,
  type McpVisibility,
} from "./mcp";

export const loadServer = (id: string) => apiGet<McpServer>(serverUrl(id));

export const loadServers = (query: `/${string}`) =>
  apiGet<McpServerPage>(query);

export const loadFolder = (id: string) =>
  apiGet<McpFolder>(`${MCP_BASE}/folders/${encodeURIComponent(id)}`);

export const loadFolders = (parent: string | null) =>
  apiGet<{ items: McpFolder[] }>(
    `${MCP_BASE}/folders${parent ? `?parent_id=${encodeURIComponent(parent)}` : ""}`,
  );

export const loadTools = (id: string) =>
  apiGet<{ items: McpTool[] }>(serverUrl(id, "/tools"));

export const disableTools = (id: string, tools: string[]) =>
  apiSend<{ items: McpTool[] }>("PUT", serverUrl(id, "/tools/disabled"), {
    tools,
  });

export const checkServer = (id: string) =>
  apiSend<McpServer>("POST", serverUrl(id, "/check"));

export const loadAccess = (id: string) =>
  apiGet<McpAccess>(serverUrl(id, "/access"));

export const saveAccess = (
  id: string,
  body: { everyone: boolean; groups: string[]; users: string[] },
) => apiSend<McpAccess>("PUT", serverUrl(id, "/access"), body);

export const loadCalls = (id: string, query: string) =>
  apiGet<{ items: McpCall[]; next_cursor: string | null }>(
    serverUrl(id, `/calls${query ? `?${query}` : ""}`),
  );

export type ProxyOwner = "organizations" | "folders" | "servers";

export const proxyUrl = (owner: ProxyOwner, id: string): `/${string}` =>
  `${MCP_BASE}/${owner}/${encodeURIComponent(id)}/proxy` as `/${string}`;

export const loadProxy = (owner: ProxyOwner, id: string) =>
  apiGet<McpProxySetting>(proxyUrl(owner, id));

export const loadSpace = (org: string) =>
  apiGet<McpSpace>(`${MCP_BASE}/organizations/${encodeURIComponent(org)}`);

export const loadOrgSet = (org: string) =>
  apiGet<McpOrganizationSet>(
    `${MCP_BASE}/organizations/${encodeURIComponent(org)}/servers`,
  );

export const saveOrgSet = (
  org: string,
  body: { mode: "all" | "selected"; servers: string[] },
) =>
  apiSend<McpOrganizationSet>(
    "PUT",
    `${MCP_BASE}/organizations/${encodeURIComponent(org)}/servers`,
    body,
  );

export const loadVisibility = (org?: string | null) =>
  apiGet<McpVisibility>(
    `${MCP_BASE}/visibility${org ? `?organization_id=${encodeURIComponent(org)}` : ""}`,
  );

export const loadBuiltin = () => apiGet<McpBuiltin>(`${MCP_BASE}/builtin`);

export const putBuiltinDisabled = (tools: string[]) =>
  apiSend<McpBuiltin>("PUT", `${MCP_BASE}/builtin/tools/disabled`, { tools });
