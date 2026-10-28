"use client";

import { useEffect, useSyncExternalStore } from "react";

import { apiGet, errorCode, type DocSpace, type DocSpaceNode, type KnowledgeFiles, type KnowledgeTreeLevel } from "./api";
import { onCatalogChange } from "./catalogEvents";
import { staleCache, type Entry } from "./staleCache";
import { refQuery } from "./refs";

const listeners = new Set<() => void>();
let version = 0;

function emit() {
  version++;
  listeners.forEach((l) => l());
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

export type DocSource = { kind: "private" } | { kind: "public"; root: string | null };

let source: DocSource = { kind: "private" };

export function setDocSource(next: DocSource): void {
  source = next;
}

export function isPublicSource(): boolean {
  return source.kind === "public";
}

const PUBLIC_PREFIX = "pub:";

function publicQuery(extra: Record<string, string>): string {
  const q = new URLSearchParams();
  if (source.kind === "public" && source.root) q.set("root", source.root);
  for (const [k, v] of Object.entries(extra)) q.set(k, v);
  return q.toString();
}

function projectPathOf(projectId: string): string {
  return projectId.startsWith(PUBLIC_PREFIX) ? projectId.slice(PUBLIC_PREFIX.length) : projectId;
}

type PublicVersion = { name: string; kind: "branch" | "tag"; collected_at: string; release: boolean };
type PublicNode = { kind: DocSpaceNode["kind"]; slug: string; name: string; description?: string; has_docs?: boolean };
type PublicSpace = {
  entry: { name: string; title: string; kind: string; host: string | null };
  projects: { path: string; name: string }[];
  node: PublicNode;
  ancestors: PublicNode[];
  children: PublicNode[];
  docs: { default: PublicVersion | null; branches: PublicVersion[]; tags: PublicVersion[]; release: string | null } | null;
  doc_path: string | null;
};

function publicNode(n: PublicNode, path: string[]): DocSpaceNode {
  return { id: PUBLIC_PREFIX + path.join("/"), kind: n.kind, slug: n.slug, name: n.name, description: n.description, access: "read", has_docs: n.has_docs };
}

export function fromPublicSpace(p: PublicSpace): DocSpace {
  const nodePath = p.ancestors.slice(1).map((a) => a.slug);
  const isEntry = p.ancestors.length === 0;
  if (!isEntry) nodePath.push(p.node.slug);
  const docs = p.docs
    ? {
        source: null,
        default_branch: p.docs.default?.kind === "branch" ? p.docs.default.name : (p.docs.branches[0]?.name ?? ""),
        branches: p.docs.branches.map((b) => ({ name: b.name, collected_at: b.collected_at })),
        tags: p.docs.tags.map((t) => ({ name: t.name, collected_at: t.collected_at, release: t.release })),
        release: p.docs.release,
        default_key: p.docs.default ? (p.docs.default.kind === "tag" ? `tag:${p.docs.default.name}` : p.docs.default.name) : null,
      }
    : null;
  return {
    node: publicNode(p.node, nodePath),
    ancestors: p.ancestors.slice(1).map((a, i) => publicNode(a, nodePath.slice(0, i + 1))),
    children: p.children.map((c) => publicNode(c, [...nodePath, c.slug])),
    node_path: nodePath,
    entry: { name: p.entry.name, title: p.entry.title, host: p.entry.host, projects: p.projects },
    docs,
    doc_path: p.doc_path,
  };
}

export function nodePathOf(space: DocSpace): string[] {
  if (space.node_path) return space.node_path;
  return space.node ? [...space.ancestors.map((a) => a.slug), space.node.slug] : [];
}

export function fileUrl(projectId: string, branch: string, path: string): `/${string}` {
  if (source.kind === "public") return `/public/docs/file?${publicQuery({ path: projectPathOf(projectId), file: path, ...Object.fromEntries(refQuery(branch)) })}`;
  return `/v1/catalog/nodes/${projectId}/knowledge/file?${refQuery(branch, { path }).toString()}`;
}

export function apiDocUrl(projectId: string, branch: string, path: string): `/${string}` {
  if (source.kind === "public")
    return `/public/docs/api?${publicQuery({ path: projectPathOf(projectId), file: path, ...Object.fromEntries(refQuery(branch)) })}`;
  return `/v1/catalog/nodes/${projectId}/knowledge/api?${refQuery(branch, { path }).toString()}`;
}

export function searchUrl(q: string): `/${string}` {
  return `/public/docs/search?${publicQuery({ q })}`;
}

const spaces = staleCache<DocSpace>(emit, errorCode);
const files = staleCache<KnowledgeFiles>(emit, errorCode);
const levels = staleCache<KnowledgeTreeLevel>(emit, errorCode);

export function revalidate() {
  spaces.revalidate();
  files.revalidate();
  levels.revalidate();
  emit();
}

export function ensureSpace(path: string, retry = false) {
  if (source.kind === "public") {
    spaces.ensure(path, () => apiGet<PublicSpace>(`/public/docs/space?${publicQuery({ path })}`).then(fromPublicSpace), retry);
    return;
  }
  spaces.ensure(path, () => apiGet<DocSpace>(`/v1/docs/space?path=${encodeURIComponent(path)}`), retry);
}

export function filesUrl(projectId: string, branch: string): `/${string}` {
  return source.kind === "public"
    ? `/public/docs/files?${publicQuery({ path: projectPathOf(projectId), ...Object.fromEntries(refQuery(branch)) })}`
    : `/v1/catalog/nodes/${projectId}/knowledge/files?${refQuery(branch).toString()}`;
}

export function ensureFiles(projectId: string, branch: string, retry = false) {
  files.ensure(`${projectId}:${branch}`, () => apiGet<KnowledgeFiles>(filesUrl(projectId, branch)), retry);
}

function levelKey(projectId: string, branch: string | null, dir: string): string {
  return `${projectId}:${branch ?? ""}:${dir}`;
}

export function ensureLevel(projectId: string, branch: string | null, dir: string, retry = false) {
  const q = new URLSearchParams();
  if (branch) refQuery(branch).forEach((v, k) => q.set(k, v));
  if (dir) q.set("dir", dir);
  const qs = q.toString();
  const url: `/${string}` =
    source.kind === "public"
      ? `/public/docs/tree?${publicQuery({ path: projectPathOf(projectId), ...Object.fromEntries(q) })}`
      : `/v1/catalog/nodes/${projectId}/knowledge/tree${qs ? `?${qs}` : ""}`;
  levels.ensure(levelKey(projectId, branch, dir), () => apiGet<KnowledgeTreeLevel>(url), retry);
}

export function levelEntry(projectId: string, branch: string | null, dir: string): Entry<KnowledgeTreeLevel> | undefined {
  return levels.read(levelKey(projectId, branch, dir));
}

export function spaceEntry(path: string): Entry<DocSpace> | undefined {
  return spaces.read(path);
}

export function filesEntry(projectId: string, branch: string): Entry<KnowledgeFiles> | undefined {
  return files.read(`${projectId}:${branch}`);
}

export function useDocStore(): number {
  return useSyncExternalStore(
    subscribe,
    () => version,
    () => 0,
  );
}

function nodeSlugs(s: DocSpace): string[] {
  return nodePathOf(s);
}

export function knownSpace(segments: readonly string[]): { space: DocSpace; complete: boolean } | null {
  const exact = spaces.read(segments.join("/"))?.value;
  if (exact) return { space: exact, complete: true };
  for (const e of spaces.values()) {
    const v = e.value;
    if (v?.node?.kind !== "project") continue;
    const slugs = nodeSlugs(v);
    if (slugs.length <= segments.length && slugs.every((s, j) => s === segments[j]?.toLowerCase())) {
      const rest = segments.slice(slugs.length).join("/");
      return { space: { ...v, doc_path: rest || null }, complete: true };
    }
  }
  for (let i = segments.length - 1; i >= 0; i--) {
    const v = spaces.read(segments.slice(0, i).join("/"))?.value;
    if (!v) continue;
    const slugs = nodeSlugs(v);
    const inside = slugs.length > 0 && slugs.every((s, j) => s === segments[j]?.toLowerCase());
    if (v.node?.kind === "project" && inside) {
      const rest = segments.slice(slugs.length).join("/");
      return { space: { ...v, doc_path: rest || null }, complete: true };
    }
    return { space: v, complete: false };
  }
  return null;
}

export const REVALIDATE_MS = 60000;

export function useDocRevalidate(): void {
  useEffect(() => {
    let timer: ReturnType<typeof setInterval> | undefined;
    const start = () => {
      clearInterval(timer);
      timer = setInterval(() => document.visibilityState === "visible" && revalidate(), REVALIDATE_MS);
    };
    const onVisible = () => {
      if (document.visibilityState !== "visible") return clearInterval(timer);
      revalidate();
      start();
    };
    start();
    document.addEventListener("visibilitychange", onVisible);
    const off = onCatalogChange(revalidate);
    return () => {
      clearInterval(timer);
      document.removeEventListener("visibilitychange", onVisible);
      off();
    };
  }, []);
}
