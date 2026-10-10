export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export type User = {
  id: string;
  email: string;
  display_name: string;
  status: "active" | "disabled";
  is_superadmin: boolean;
  is_service: boolean;
  has_password: boolean;
  created_at: string;
  updated_at: string;
};

export type Me = User & { login_method: string };

export type Provider = { key: string; display_name: string };

export type PasswordLogin = "all" | "superadmins" | "off";

export type Providers = { providers: Provider[]; password_login: PasswordLogin };

export type Identity = {
  id: string;
  provider: string;
  display_name: string;
  email: string | null;
  created_at: string;
  last_login_at: string;
};

export function oauthStartUrl(provider: string, options: { next?: string; link?: boolean } = {}): string {
  const query = new URLSearchParams();
  if (options.next) query.set("next", options.next);
  if (options.link) query.set("link", "1");
  const q = query.toString();
  return `/api/v1/auth/oauth/${encodeURIComponent(provider)}/start${q ? `?${q}` : ""}`;
}

export type Page<T> = { items: T[]; total: number; limit: number; offset: number };

export type Group = {
  id: string;
  name: string;
  member_count: number;
  created_at: string;
  updated_at: string;
};

export type Member = Pick<User, "id" | "email" | "display_name" | "status"> & { source: string };

export type GroupDetails = Group & { members: Member[] };

export type NodeKind = "organization" | "folder" | "project";

export type CatalogNode = {
  id: string;
  kind: NodeKind;
  parent_id: string | null;
  slug: string;
  name: string;
  access: "read" | "navigate";
  description?: string;
  labels?: Record<string, string>;
  forge?: ForgeKind | null;
  repo_url?: string | null;
  default_branch?: string | null;
  created_at?: string;
  updated_at?: string;
  depth?: number;
  permissions?: string[];
  path?: CatalogNode[];
  key?: ProjectKey;
  managed?: boolean;
  repository?: Repository | null;
  cluster_observation?: boolean;
};

export type ProcessKind = "collect" | "index" | "forge" | "clusters";
export type ProcessState = "running" | "queued" | "failed" | "idle";
export type Process = { kind: ProcessKind; state: ProcessState; code: string | null; last_at: string | null; pending: number | null };
export type ActivitySummary = { running: number; queued: number; failed: number };
export type NodeActivity = { processes?: Process[]; summary?: ActivitySummary };
export type CatalogTableRow = CatalogNode & {
  children: number;
  match: boolean;
  activity: Process[] | ActivitySummary;
  links: LinkBadge[];
};
export type CatalogTable = Page<CatalogTableRow> & { truncated: boolean };

export type ForgeKind = "github" | "gitlab" | "forgejo" | "gitea";

export type Repository = {
  connection_id: string;
  full_path: string;
  web_url: string;
  description: string;
  topics: string[];
  languages: Record<string, number>;
  default_branch: string | null;
  archived: boolean;
  visibility: "public" | "internal" | "private";
  stars: number;
  license: string | null;
  pushed_at: string | null;
  synced_at: string;
  orphaned_at: string | null;
  has_readme: boolean;
};

export type Readme = { markdown: string; truncated: boolean; web_url: string; default_branch: string | null; kind: ForgeKind };

export type SyncRun = {
  id: string;
  trigger: "schedule" | "manual" | "webhook" | "cli";
  status: "running" | "succeeded" | "failed" | "rate_limited";
  started_at: string;
  finished_at: string | null;
  created: number;
  updated: number;
  orphaned: number;
  skipped: number;
  error_code: string | null;
  error_message: string | null;
  problems: { full_path: string; code: string }[];
};

export type ForgeConnection = {
  id: string;
  node_id: string;
  kind: ForgeKind;
  api_url: string;
  owner_path: string;
  mirror_subgroups: boolean;
  include_archived: boolean;
  include_forks: boolean;
  name_include: string[];
  name_exclude: string[];
  branch_include: string[];
  interval_secs: number;
  credentials: Credentials;
  webhook: { mode: "register" | "manual"; url: string | null } | null;
  last_run: SyncRun | null;
  next_run_at: string;
  created_at: string;
  updated_at: string;
};

export type Branch = {
  name: string;
  head_sha: string | null;
  is_default: boolean;
  protected: boolean | null;
  sources: ("forge" | "ingest" | "cluster" | "manual" | "repository")[];
  pinned: boolean;
  stale: boolean;
  first_seen_at: string;
  last_activity_at: string;
  gone_at: string | null;
};

export type PreviewItem = { full_path: string; action: "create" | "update" | "move" | "orphan" | "skip"; code?: string };

export type WebhookSetup = { mode: "register" | "manual"; url: string; secret?: string };

export type Tree = { nodes: CatalogNode[]; truncated: boolean };

export type ProjectKey = {
  id: string;
  prefix: string;
  status: "active" | "expired" | "revoked";
  created_at: string;
  last_used_at: string | null;
  expires_at: string | null;
  revoked_at: string | null;
  secret?: string;
};

export type Scope = "read" | "write" | "admin" | "mcp";

export const SCOPES: readonly Scope[] = ["read", "write", "admin", "mcp"];

export type PersonalToken = {
  id: string;
  name: string;
  prefix: string;
  scopes: Scope[];
  status: "active" | "expired" | "revoked";
  created_at: string;
  expires_at: string | null;
  last_used_at: string | null;
  revoked_at: string | null;
  user_id?: string;
  user_email?: string;
  secret?: string;
};

export type Role = "viewer" | "editor" | "admin";

export type Binding = {
  id: string;
  node_id: string;
  node_name: string;
  subject_kind: "user" | "group";
  subject_id: string;
  subject_name: string;
  role: Role;
  inherited: boolean;
  created_at: string;
};

export type Items<T> = { items: T[] };

export type NodeRef = { id: string; name: string };

export type SecretRef = { id: string; name: string; from: NodeRef | null };

export type Credentials =
  | { kind: "secret"; secret: SecretRef }
  | { kind: "legacy"; storage: "stored" | "reference"; fingerprint?: string; ref?: string }
  | { kind: "none" };

export type Secret = {
  id: string;
  name: string;
  description: string;
  node_id: string | null;
  storage: "stored" | "reference";
  fingerprint: string | null;
  ref: string | null;
  used_by: number;
  from: NodeRef | null;
  own: boolean;
  created_at: string;
  updated_at: string;
};

export type LabelKey = { key: string; count: number };
export type LabelValue = { value: string; count: number };

export type IngestEvent = {
  id: string;
  type: string;
  version: number;
  occurred_at: string;
  received_at: string;
  idempotency_key: string;
  key_prefix: string | null;
  payload: Record<string, unknown>;
};

export type Deployment = {
  id: string;
  service: string;
  environment: string;
  version: string;
  commit_sha: string | null;
  branch: string | null;
  cluster: string | null;
  namespace: string | null;
  url: string | null;
  deployed_by: string | null;
  metadata: Record<string, string>;
  occurred_at: string;
  received_at: string;
  event_id: string | null;
  source: DeploymentSource;
  current: boolean;
};

export type DeploymentSource = "event" | "cluster";

export type Rollout = "complete" | "progressing" | "stalled";

export type Observation = {
  cluster: string;
  namespace: string;
  kind: "Deployment" | "StatefulSet" | "DaemonSet" | "CronJob";
  workload: string;
  branch: string | null;
  version: string | null;
  images: { container: string; image: string; digest: string | null }[];
  replicas?: { desired: number; ready: number; updated: number };
  restarts?: number;
  rollout?: Rollout;
  schedule?: string;
  suspended?: boolean;
  last_run?: { started_at: string | null; status: "succeeded" | "failed" | "active" } | null;
  observed_at: string;
  gone: boolean;
};

export type ServiceEnvironment = {
  service: string;
  environment: string;
  version: string | null;
  commit_sha: string | null;
  branch: string | null;
  url: string | null;
  deployed_by: string | null;
  occurred_at: string | null;
  source: DeploymentSource | null;
  deployment_id: string | null;
  updated_at: string | null;
  observed: Observation[];
  drift: boolean;
};

export type DirectoryEnvironment = {
  key: string;
  names: Record<"en" | "es" | "ru" | "zh", string>;
  position: number;
  created_at: string;
  updated_at: string;
};

export type ClusterError = { code: string; message: string };

export type Cluster = {
  id: string;
  name: string;
  environment: string;
  in_cluster: boolean;
  api_url: string | null;
  ca_pem: string | null;
  credentials: Credentials | null;
  namespaces: string[];
  rules: { label: string; project: string }[];
  interval_secs: number;
  enabled: boolean;
  status: "ok" | "error" | "never";
  last_error: ClusterError | null;
  last_polled_at: string | null;
  next_poll_at: string;
  workloads: number;
  unmatched: number;
  created_at: string;
  updated_at: string;
};

export type ClusterTest = { ok: boolean; version: string | null; missing: string[]; error?: ClusterError };

export type UnmatchedReason = "no_annotation" | "project_not_found" | "invalid_service" | "invalid_environment";

export type UnmatchedWorkload = {
  namespace: string;
  kind: Observation["kind"];
  name: string;
  reason: UnmatchedReason;
  annotation: string | null;
  observed_at: string;
};

export const LINK_ICONS = ["link", "logs", "dashboard", "errors", "alerts", "traces", "runbook", "docs"] as const;
export type LinkIcon = (typeof LINK_ICONS)[number];

export type LinkKind = {
  key: string;
  names: Record<"en" | "es" | "ru" | "zh", string>;
  icon: LinkIcon;
  position: number;
  created_at: string;
  updated_at: string;
};

export type LinkGlyph = { kind: "builtin"; name: string } | { kind: "url" | "file"; url: string };

export type LinkBadge = { link_key: string; kind_key: string; title: string | null; icon: LinkGlyph; url: string | null };

export type LinkTemplate = {
  link_key: string;
  kind_key: string;
  title: string | null;
  icon: LinkGlyph;
  template: string | null;
  disabled: boolean;
  position: number;
  node_id: string;
  inherited: boolean;
  created_at: string;
  updated_at: string;
};

export type NodeVar = { key: string; value: string; node_id: string; inherited: boolean };

export type LinkStatus =
  | "ok"
  | "redirect"
  | "auth_required"
  | "not_found"
  | "server_error"
  | "unexpected"
  | "timeout"
  | "tls_error"
  | "dns_error"
  | "unreachable"
  | "blocked";

export const LINK_STATUSES: readonly LinkStatus[] = [
  "ok",
  "redirect",
  "auth_required",
  "not_found",
  "server_error",
  "unexpected",
  "timeout",
  "tls_error",
  "dns_error",
  "unreachable",
  "blocked",
];

export type LinkCheck = { status: LinkStatus; http_status: number | null; duration_ms: number; checked_at: string };

export type ProjectLink = {
  link_key: string;
  kind_key: string;
  title: string | null;
  icon: LinkGlyph;
  node_id: string;
  inherited: boolean;
  service: string | null;
  environment: string | null;
  url: string | null;
  missing: string[];
  check: LinkCheck | null;
};

export type KnowledgeSettings = { include: string[] | null; exclude: string[]; branches: string[] };

export type SettingsField = "include" | "exclude" | "branches";

export type NodeKnowledgeSettings = KnowledgeSettings & {
  own: Partial<KnowledgeSettings>;
  from: Record<SettingsField, { id: string; kind: NodeKind; name: string } | null>;
};

export type Snapshot = {
  id: string;
  branch: string;
  commit: string;
  status: "ok" | "partial" | "failed";
  error_code: string | null;
  files: number;
  bytes: number;
  skipped: number;
  truncated: boolean;
  collected_at: string;
};

export type KnowledgeBranch = {
  name: string;
  head_sha: string | null;
  pending: boolean;
  last: Snapshot | null;
  snapshot: Snapshot | null;
};

export type SourceKind = "remote" | "local_dir" | "local_git";

export type Knowledge = {
  synced: boolean;
  source: SourceKind | "forge" | null;
  settings: KnowledgeSettings;
  branches: KnowledgeBranch[];
};

export type KnowledgeSource = {
  kind: SourceKind;
  forge: "github" | "gitlab" | "gitea" | "forgejo" | null;
  url: string | null;
  api_url: string | null;
  path: string | null;
  credentials: { mode: "secret" | "legacy" | "none"; secret: SecretRef | null; fingerprint: string | null };
  working_tree: boolean;
  include_ignored: boolean;
  updated_at: string;
};

export type SourceCheck = { ok: boolean; branches?: number; error_code?: string };

export type DocKind = "doc" | "spec" | "change" | "adr";
export type SkipReason = "too_large" | "binary" | "limit";

export type KnowledgeFile = { path: string; kind: DocKind; bytes: number; skip_reason: SkipReason | null };

export type KnowledgeFiles = { snapshot: Snapshot; items: KnowledgeFile[] };

export type KnowledgeFileContent = KnowledgeFile & { snapshot: Snapshot; content: string | null };

export type SearchSegment = { text: string; match: boolean };

export type SearchHit = {
  project: { id: string; path: string; name: string };
  branch: string;
  commit: string;
  path: string;
  kind: DocKind;
  snippet: SearchSegment[];
};

export type SearchPage = { items: SearchHit[]; next_cursor: string | null };

export type SearchMode = "text" | "semantic" | "hybrid";

export type SearchModes = {
  engine: string;
  modes: SearchMode[];
  default: SearchMode;
  index: { pending: number; indexed: number } | null;
};

async function call<T>(method: string, path: `/${string}`, body?: unknown, signal?: AbortSignal): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`/api${path}`, {
      method,
      signal,
      credentials: "same-origin",
      headers: {
        accept: "application/json",
        ...(body === undefined || body instanceof FormData ? {} : { "content-type": "application/json" }),
      },
      body: body === undefined ? undefined : body instanceof FormData ? body : JSON.stringify(body),
    });
  } catch (e) {
    if (signal?.aborted) throw e;
    throw new ApiError(0, "network", "network error");
  }
  if (response.status === 204) return undefined as T;
  const parsed: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    const { code, message } = (parsed ?? {}) as { code?: unknown; message?: unknown };
    throw new ApiError(
      response.status,
      typeof code === "string" ? code : "unknown",
      typeof message === "string" ? message : response.statusText,
    );
  }
  return parsed as T;
}

export function apiGet<T>(path: `/${string}`, signal?: AbortSignal): Promise<T> {
  return call<T>("GET", path, undefined, signal);
}

export function apiSend<T = void>(
  method: "POST" | "PUT" | "PATCH" | "DELETE",
  path: `/${string}`,
  body?: unknown,
): Promise<T> {
  return call<T>(method, path, body);
}

export function apiUpload<T>(path: `/${string}`, file: File): Promise<T> {
  const form = new FormData();
  form.append("file", file);
  return call<T>("POST", path, form);
}

export function errorCode(e: unknown): string {
  return e instanceof ApiError ? e.code : "unknown";
}

export type ScanStatus = "ok" | "unchanged" | "warning" | "failed";
export type ScanKind = "collect" | "index";
export type ScanTrigger = "schedule" | "manual";
export type ScanSource = "forge" | "remote" | "local_dir" | "local_git";

export type ScanBranch = {
  name: string;
  commit: string;
  result: "collected" | "unchanged" | "failed";
  files: number;
  skipped: Record<string, number>;
  truncated: boolean;
  working_tree: boolean;
  error: string | null;
};

export type Scan = {
  id: string;
  project: { id: string; path: string; name: string };
  kind: ScanKind;
  trigger: ScanTrigger;
  source: ScanSource | null;
  status: ScanStatus;
  started_at: string;
  last_started_at: string;
  finished_at: string;
  duration_ms: number | null;
  repeats: number;
  branches: ScanBranch[];
  index: { embedded_files: number; embedded_chunks: number; documents: number; engine: string; model: string } | null;
  error: { code: string; detail: string } | null;
  warnings: string[];
};

