export type ApiFormat = "openapi" | "swagger" | "proto";

export type ApiProblem = { code: string; path: string; line: number | null; message: string };

export type ApiSummary = {
  path: string;
  format: ApiFormat;
  spec_version: string | null;
  title: string | null;
  version: string | null;
  operations: number | null;
  services: number | null;
  methods: number | null;
  messages: number | null;
  skip_reason: string | null;
  problems: number;
  vendored: boolean;
};

export type ApiList = { items: ApiSummary[]; hidden_vendored: number };

export type ApiSchema = { [key: string]: unknown };

export type ApiContent = { media_type: string; schema: ApiSchema | null };

export type ApiOperation = {
  key: string;
  method: string;
  path: string;
  operation_id: string | null;
  summary: string | null;
  description: string | null;
  tags: string[];
  deprecated: boolean;
  parameters: { name: string; in: string; required: boolean; description: string | null; schema: ApiSchema | null }[];
  request_body: { required: boolean; description: string | null; content: ApiContent[] } | null;
  responses: { status: string; description: string | null; content: ApiContent[] }[];
};

export type ProtoMethod = {
  name: string;
  input: string;
  output: string;
  client_streaming: boolean;
  server_streaming: boolean;
  description: string | null;
  deprecated: boolean;
  http: { method: string; path: string; body: string | null } | null;
};

export type ProtoField = {
  name: string;
  number: number;
  type: string;
  label: string | null;
  map: { key: string; value: string } | null;
  oneof: string | null;
  description: string | null;
  deprecated: boolean;
};

export type ProtoMessage = { full_name: string; description: string | null; deprecated: boolean; fields: ProtoField[] };

export type ProtoEnum = { full_name: string; description: string | null; values: { name: string; number: number; description: string | null }[] };

export type ApiDocument = {
  path: string;
  format: ApiFormat;
  spec_version: string | null;
  problems: ApiProblem[];
  info?: { title: string; version: string; description: string | null };
  servers?: { url: string; description: string | null }[];
  tags?: { name: string; description: string | null }[];
  operations?: ApiOperation[];
  schemas?: { name: string; schema: ApiSchema }[];
  package?: string | null;
  imports?: string[];
  options?: Record<string, string>;
  services?: { name: string; full_name: string; description: string | null; methods: ProtoMethod[] }[];
  messages?: ProtoMessage[];
  enums?: ProtoEnum[];
  type_files?: Record<string, string>;
};

export type ApiView = "api" | "tree" | "source";

export const isApiCandidate = (path: string) => /\.(proto|ya?ml|json)$/i.test(path);

export const apiQuery = (path: string, branch: string) => new URLSearchParams({ path, branch }).toString();

export function viewParam(search: string): ApiView {
  const v = new URLSearchParams(search).get("view");
  return v === "tree" || v === "source" ? v : "api";
}

export function withView(href: string, view: ApiView): string {
  const url = new URL(href, "http://x");
  if (view === "api") url.searchParams.delete("view");
  else url.searchParams.set("view", view);
  return url.pathname + url.search + url.hash;
}

const slug = (s: string) => s.replace(/[^A-Za-z0-9_.-]+/g, "-").replace(/^-+|-+$/g, "");

export const operationAnchor = (op: { method: string; path: string }) => `op-${op.method.toLowerCase()}-${slug(op.path)}`;

export const schemaAnchor = (name: string) => `schema-${slug(name)}`;

export const protoAnchor = (fullName: string) => `type-${slug(fullName.replace(/^\./, ""))}`;

export function filterOperations(ops: ApiOperation[], query: string): ApiOperation[] {
  const q = query.trim().toLowerCase();
  if (!q) return ops;
  return ops.filter((o) => [o.path, o.summary ?? "", o.operation_id ?? ""].some((s) => s.toLowerCase().includes(q)));
}

export function groupByTag(tags: { name: string }[], ops: ApiOperation[]): { tag: string | null; operations: ApiOperation[] }[] {
  const groups = tags.map((t) => ({ tag: t.name as string | null, operations: ops.filter((o) => o.tags.includes(t.name)) }));
  const untagged = ops.filter((o) => o.tags.length === 0);
  if (untagged.length > 0) groups.push({ tag: null, operations: untagged });
  return groups.filter((g) => g.operations.length > 0);
}

export function methodTone(method: string): string {
  switch (method.toUpperCase()) {
    case "GET":
      return "get";
    case "POST":
      return "post";
    case "PUT":
    case "PATCH":
      return "put";
    case "DELETE":
      return "delete";
  }
  return "other";
}

let pendingAnchor: string | null = null;

export function rememberAnchor(anchor: string): void {
  pendingAnchor = anchor;
}

export function takeAnchor(hash: string): string | null {
  const anchor = pendingAnchor ?? (hash ? decodeURIComponent(hash.replace(/^#/, "")) : null);
  pendingAnchor = null;
  return anchor;
}

export function importedType(type: string, files: Record<string, string> | undefined): { name: string; path: string } | null {
  const name = type.replace(/^\./, "");
  const path = files?.[name];
  return path ? { name, path } : null;
}

export function localType(type: string, pkg: string | null | undefined, known: Set<string>): string | null {
  const name = type.replace(/^\./, "");
  if (known.has(name)) return name;
  const qualified = pkg ? `${pkg}.${name}` : name;
  return known.has(qualified) ? qualified : null;
}
