"use client";

import { useEffect, useSyncExternalStore } from "react";

import { apiGet, errorCode, type DocSpace, type KnowledgeFiles, type KnowledgeTreeLevel } from "./api";
import { onCatalogChange } from "./catalogEvents";
import { staleCache, type Entry } from "./staleCache";

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
  spaces.ensure(path, () => apiGet<DocSpace>(`/v1/docs/space?path=${encodeURIComponent(path)}`), retry);
}

export function ensureFiles(projectId: string, branch: string, retry = false) {
  files.ensure(`${projectId}:${branch}`, () => apiGet<KnowledgeFiles>(`/v1/catalog/nodes/${projectId}/knowledge/files?branch=${encodeURIComponent(branch)}`), retry);
}

function levelKey(projectId: string, branch: string | null, dir: string): string {
  return `${projectId}:${branch ?? ""}:${dir}`;
}

export function ensureLevel(projectId: string, branch: string | null, dir: string, retry = false) {
  const q = new URLSearchParams();
  if (branch) q.set("branch", branch);
  if (dir) q.set("dir", dir);
  const qs = q.toString();
  levels.ensure(levelKey(projectId, branch, dir), () => apiGet<KnowledgeTreeLevel>(`/v1/catalog/nodes/${projectId}/knowledge/tree${qs ? `?${qs}` : ""}`), retry);
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
  return s.node ? [...s.ancestors.map((a) => a.slug), s.node.slug] : [];
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
