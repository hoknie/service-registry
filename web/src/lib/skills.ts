export type SkillStatus = "draft" | "published" | "disabled";

export type SkillRef = { id: string; slug: string; name: string };

export type SkillVersion = {
  version: string;
  created_at: string;
  source: "upload" | "git";
  commit: string | null;
  tag: string | null;
  created_by: { id: string; name: string } | null;
  files: number;
  size: number;
  digest: string;
  prerelease: boolean;
  available: boolean;
  yanked: boolean;
};

export type Skill = {
  id: string;
  folder_id: string | null;
  organization_id: string | null;
  organization: SkillRef | null;
  path: string;
  slug: string;
  name: string;
  title: string;
  description: string;
  about: string;
  license: string;
  compatibility: string;
  allowed_tools: string[];
  tags: string[];
  author: string;
  status: SkillStatus;
  image_url: string | null;
  version: SkillVersion | null;
  versions_available: number;
  downloads: number;
  source: { id: string; kind: string; url: string } | null;
  managed: boolean;
  active: boolean;
  disabled_by: SkillRef | null;
  can_write: boolean;
  created_by: { id: string; name: string } | null;
  created_at: string;
  updated_at: string;
};

export type SkillFolder = {
  id: string;
  parent_id: string | null;
  organization_id: string | null;
  organization: SkillRef | null;
  slug: string;
  name: string;
  description: string;
  ancestors: SkillRef[];
  skills: number;
  folders: number;
  disabled: boolean;
  disabled_by: SkillRef | null;
  managed: boolean;
  source_id: string | null;
  can_write: boolean;
};

export type SkillFileKind = "markdown" | "text" | "image" | "binary";

export type SkillFile = {
  path: string;
  size: number;
  kind: SkillFileKind;
  title: string | null;
};

export type SkillFiles = { version: string; items: SkillFile[] };

export type SkillFileContent = {
  path: string;
  kind: SkillFileKind;
  size: number;
  content: string | null;
  version: string;
};

export type SkillPolicy = {
  range: string | null;
  prerelease: boolean | null;
  exclude: string[] | null;
};

export type PolicyOrigin = "skill" | "organization" | "default";

export type SkillPolicyView = {
  own: SkillPolicy;
  effective: { range: string; prerelease: boolean; exclude: string[] };
  from: Record<"range" | "prerelease" | "exclude", PolicyOrigin>;
  can_write: boolean;
};

export type SourceState = "queued" | "running" | "idle" | "failed" | "paused";

export type SourcePausedReason = "folder_disabled" | "skills_disabled" | "polling_off";

export type SourceRun = {
  started_at: string;
  finished_at: string;
  status: "succeeded" | "unchanged" | "failed";
  commit: string | null;
  created: number;
  updated: number;
  removed: number;
  skipped: number;
  error: string | null;
  problems: { code: string; path: string; detail: string }[];
};

export type SkillSourceKind = "remote" | "local_git" | "local_dir";

export type SourceLayout = "flat" | "tree" | "field";

export type SkillSource = {
  id: string;
  folder_id: string;
  kind: SkillSourceKind;
  hosting: string | null;
  url: string | null;
  api_url: string | null;
  path: string | null;
  working_tree: boolean;
  credentials: { secret_id: string; name: string | null } | null;
  branch: string;
  root: string;
  roots: string[];
  layout: SourceLayout;
  group_field: string | null;
  interval_secs: number | null;
  status: SkillStatus;
  versions: "branch" | "tags";
  tag_pattern: string;
  state: SourceState;
  paused_reason: SourcePausedReason | null;
  effective_interval_secs: number;
  next_run_at: string | null;
  last_run: SourceRun | null;
};

export type SkillPage = { items: Skill[]; next_cursor: string | null };

export type SkillFilters = {
  q: string;
  tags: string[];
  status: string;
  active?: boolean;
};

export const SKILLS_BASE = "/v1/skills" as const;

export function apiHref(path: string): string {
  return `/api${path}`;
}

export function skillUrl(id: string, rest = ""): `/${string}` {
  return `${SKILLS_BASE}/${encodeURIComponent(id)}${rest}`;
}

export function withVersion(
  path: string,
  version: string | null | undefined,
  extra: Record<string, string> = {},
): `/${string}` {
  const q = new URLSearchParams(extra);
  if (version) q.set("version", version);
  const s = q.toString();
  return `${path}${s ? `?${s}` : ""}` as `/${string}`;
}

export type SkillPlace = { folder: string | null; space: string | null };

export function placeFrom(search: string): SkillPlace {
  const q = new URLSearchParams(search);
  return { folder: q.get("folder"), space: q.get("space") };
}

export function filtersFrom(search: string): SkillFilters {
  const q = new URLSearchParams(search);
  return {
    q: q.get("q") ?? "",
    tags: q.getAll("tag"),
    status: q.get("status") ?? "",
    active: q.get("active") === "true",
  };
}

export function placeHref(
  locale: string,
  place: SkillPlace,
  f?: SkillFilters,
): string {
  const q = new URLSearchParams();
  if (place.folder) q.set("folder", place.folder);
  else if (place.space) q.set("space", place.space);
  if (f?.q) q.set("q", f.q);
  for (const t of f?.tags ?? []) q.append("tag", t);
  if (f?.status) q.set("status", f.status);
  if (f?.active) q.set("active", "true");
  const s = q.toString();
  return `/${locale}/skills${s ? `?${s}` : ""}`;
}

export function searching(f: SkillFilters): boolean {
  return f.q !== "" || f.tags.length > 0;
}

export function listQuery(
  place: SkillPlace,
  f: SkillFilters,
  cursor?: string | null,
): `/${string}` {
  const q = new URLSearchParams();
  const deep = searching(f);
  if (place.folder) {
    q.set("folder_id", place.folder);
    if (deep) q.set("recursive", "true");
  } else {
    if (place.space) q.set("scope", place.space);
    if (!deep) q.set("folder_id", "root");
  }
  if (f.q) q.set("q", f.q);
  for (const t of f.tags) q.append("tag", t);
  if (f.status) q.set("status", f.status);
  if (f.active) q.set("active", "true");
  if (cursor) q.set("cursor", cursor);
  return `${SKILLS_BASE}?${q.toString()}` as `/${string}`;
}

export function spaceOf(organizationId: string | null): string {
  return organizationId ?? "global";
}

export type TreeNode = {
  name: string;
  path: string;
  file: SkillFile | null;
  children: TreeNode[];
};

export function fileTree(files: SkillFile[]): TreeNode[] {
  const root: TreeNode = { name: "", path: "", file: null, children: [] };
  for (const f of files) {
    const parts = f.path.split("/");
    let node = root;
    parts.forEach((part, i) => {
      const path = parts.slice(0, i + 1).join("/");
      let next = node.children.find(
        (c) =>
          c.name === part && (i === parts.length - 1) === (c.file !== null),
      );
      if (!next) {
        next = {
          name: part,
          path,
          file: i === parts.length - 1 ? f : null,
          children: [],
        };
        node.children.push(next);
      }
      node = next;
    });
  }
  const sort = (n: TreeNode) => {
    n.children.sort((a, b) =>
      (a.file === null) !== (b.file === null)
        ? a.file === null
          ? -1
          : 1
        : a.name.localeCompare(b.name),
    );
    n.children.forEach(sort);
  };
  sort(root);
  const manifest = root.children.findIndex((c) => c.path === "SKILL.md");
  if (manifest > 0) root.children.unshift(...root.children.splice(manifest, 1));
  return root.children;
}

export function resolveLink(
  from: string,
  href: string,
  known: Set<string>,
): string | null {
  if (
    /^[a-z][a-z0-9+.-]*:/i.test(href) ||
    href.startsWith("/") ||
    href.startsWith("#")
  )
    return null;
  const clean = href.split("#")[0]?.split("?")[0] ?? "";
  const dir = from.includes("/")
    ? from.slice(0, from.lastIndexOf("/")).split("/")
    : [];
  for (const part of clean.split("/")) {
    if (part === "" || part === ".") continue;
    if (part === "..") {
      if (dir.length === 0) return null;
      dir.pop();
    } else dir.push(part);
  }
  const out = dir.join("/");
  return known.has(out) ? out : null;
}

const SEMVER =
  /^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/;

export function validVersion(v: string): boolean {
  return SEMVER.test(v.trim());
}

export function nextPatch(top: string | null | undefined): string {
  const m = top ? SEMVER.exec(top) : null;
  if (!m) return "0.1.0";
  if (m[4]) return `${m[1]}.${m[2]}.${m[3]}`;
  return `${m[1]}.${m[2]}.${Number(m[3]) + 1}`;
}

export function manifestVersion(text: string): string | null {
  const front = /^﻿?---\r?\n([\s\S]*?)\r?\n---/.exec(text);
  if (!front?.[1]) return null;
  const m = /^\s+version:\s*["']?([^"'\s]+)["']?\s*$/m.exec(front[1]);
  return m?.[1] && validVersion(m[1]) ? m[1].replace(/^v/, "") : null;
}

export const SKILL_TARGETS = [
  "~/.claude/skills",
  ".claude/skills",
  "~/.config/opencode/skills",
  "~/.codex/skills",
] as const;

export function curlCommand(
  origin: string,
  skill: Pick<Skill, "id" | "name">,
  version: string | null,
  target: string,
): string {
  const url = `${origin}${apiHref(withVersion(skillUrl(skill.id, "/archive"), version))}`;
  const file = `${skill.name}${version ? `-${version}` : ""}.zip`;
  return `curl -fsSL -H "Authorization: Bearer $SVCR_TOKEN" "${url}" -o ${file} && unzip -o ${file} -d ${target}`;
}

export function agentPhrase(
  template: string,
  versionTemplate: string,
  name: string,
  version: string | null,
): string {
  return (
    template.replace("{name}", name) +
    (version ? versionTemplate.replace("{version}", version) : "") +
    "."
  );
}

export function humanSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KiB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MiB`;
}

export type SkillSpace = {
  organization: SkillRef;
  description: string;
  can_write: boolean;
};

export function folderCurlCommand(origin: string, folder: Pick<SkillFolder, "id" | "slug">, target: string): string {
  const url = `${origin}${apiHref(`${SKILLS_BASE}/folders/${encodeURIComponent(folder.id)}/archive`)}`;
  const file = `${folder.slug}.zip`;
  return `curl -fsSL -H "Authorization: Bearer $SVCR_TOKEN" "${url}" -o ${file} && unzip -o ${file} -d ${target}`;
}

export function isOff(item: { status?: SkillStatus; disabled?: boolean; disabled_by: SkillRef | null }): boolean {
  return item.status === "disabled" || item.disabled === true || item.disabled_by !== null;
}

export function archiveErrorCode(status: number): string {
  if (status === 409) return "conflict.skill_folder_empty";
  if (status === 413) return "validation.skill_folder_too_large";
  return "unknown";
}

export type SkillSetItem = { id: string; path: string; name: string; skills?: number; status?: SkillStatus };

export type SkillSet = {
  mode: "all" | "selected";
  folders: SkillSetItem[];
  skills: SkillSetItem[];
  can_manage: boolean;
};
