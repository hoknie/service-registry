"use client";

import { useSyncExternalStore } from "react";

import { apiGet, errorCode, type DocSpace, type KnowledgeFiles, type KnowledgeTreeLevel } from "./api";

type Entry<T> = { value?: T; error?: string; loading: boolean };

const spaces = new Map<string, Entry<DocSpace>>();
const files = new Map<string, Entry<KnowledgeFiles>>();
const levels = new Map<string, Entry<KnowledgeTreeLevel>>();
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

function ensure<T>(map: Map<string, Entry<T>>, key: string, load: () => Promise<T>, retry: boolean) {
  const cur = map.get(key);
  if (cur?.loading || (!retry && cur && (cur.value !== undefined || cur.error !== undefined))) return;
  map.set(key, { loading: true });
  queueMicrotask(emit);
  load()
    .then((value) => map.set(key, { value, loading: false }))
    .catch((e: unknown) => map.set(key, { error: errorCode(e), loading: false }))
    .finally(emit);
}

export function ensureSpace(path: string, retry = false) {
  ensure(spaces, path, () => apiGet<DocSpace>(`/v1/docs/space?path=${encodeURIComponent(path)}`), retry);
}

export function ensureFiles(projectId: string, branch: string, retry = false) {
  ensure(files, `${projectId}:${branch}`, () => apiGet<KnowledgeFiles>(`/v1/catalog/nodes/${projectId}/knowledge/files?branch=${encodeURIComponent(branch)}`), retry);
}

function levelKey(projectId: string, branch: string | null, dir: string): string {
  return `${projectId}:${branch ?? ""}:${dir}`;
}

export function ensureLevel(projectId: string, branch: string | null, dir: string, retry = false) {
  const q = new URLSearchParams();
  if (branch) q.set("branch", branch);
  if (dir) q.set("dir", dir);
  const qs = q.toString();
  ensure(levels, levelKey(projectId, branch, dir), () => apiGet<KnowledgeTreeLevel>(`/v1/catalog/nodes/${projectId}/knowledge/tree${qs ? `?${qs}` : ""}`), retry);
}

export function levelEntry(projectId: string, branch: string | null, dir: string): Entry<KnowledgeTreeLevel> | undefined {
  return levels.get(levelKey(projectId, branch, dir));
}

export function spaceEntry(path: string): Entry<DocSpace> | undefined {
  return spaces.get(path);
}

export function filesEntry(projectId: string, branch: string): Entry<KnowledgeFiles> | undefined {
  return files.get(`${projectId}:${branch}`);
}

export function useDocStore(): number {
  return useSyncExternalStore(
    subscribe,
    () => version,
    () => 0,
  );
}

function nodeSlugs(s: DocSpace): string[] {
  return s.node ? [...s.ancestors.map((a) => a.slug), s.node.slug] : [];
}

export function knownSpace(segments: readonly string[]): { space: DocSpace; complete: boolean } | null {
  const exact = spaces.get(segments.join("/"))?.value;
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
    const v = spaces.get(segments.slice(0, i).join("/"))?.value;
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
