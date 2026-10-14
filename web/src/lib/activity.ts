"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import type { ActivitySummary, NodeActivity, Process, ProcessKind } from "./api";

export const BUSY_MS = 5000;
export const IDLE_MS = 60000;

export function isSummary(a: Process[] | ActivitySummary | undefined): a is ActivitySummary {
  return !!a && !Array.isArray(a);
}

export function busy(a: NodeActivity | Process[] | ActivitySummary | null | undefined): boolean {
  if (!a) return false;
  if (Array.isArray(a)) return a.some((p) => p.state === "running" || p.state === "queued");
  if ("running" in a) return a.running > 0 || a.queued > 0;
  return (a.processes ? busy(a.processes) : false) || (a.summary ? busy(a.summary) : false);
}

export function pollDelay(a: NodeActivity | null): number {
  return busy(a) ? BUSY_MS : IDLE_MS;
}

export type Settled = ProcessKind | "subtree";

export function settled(prev: NodeActivity | null, next: NodeActivity | null): Settled[] {
  const was = new Set((prev?.processes ?? []).filter((p) => p.state === "running").map((p) => p.kind));
  const done: Settled[] = (next?.processes ?? []).filter((p) => was.has(p.kind) && p.state !== "running").map((p) => p.kind);
  done.push(...[...was].filter((kind) => !(next?.processes ?? []).some((p) => p.kind === kind)));
  if ((prev?.summary?.running ?? 0) > 0 && next?.summary && next.summary.running === 0) done.push("subtree");
  return done;
}

export function visible(processes: Process[]): Process[] {
  return processes.filter((p) => p.state !== "idle");
}

export function useActivity(
  nodeId: string | null,
  load: (id: string) => Promise<NodeActivity>,
  onSettled?: (kinds: Settled[]) => void,
): { activity: NodeActivity | null; refresh: () => void } {
  const [activity, setActivity] = useState<NodeActivity | null>(null);
  const [tick, setTick] = useState(0);
  const last = useRef<NodeActivity | null>(null);
  const settledRef = useRef(onSettled);
  useEffect(() => {
    settledRef.current = onSettled;
  }, [onSettled]);

  useEffect(() => {
    if (!nodeId) return;
    let live = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const run = async () => {
      clearTimeout(timer);
      if (document.visibilityState === "hidden") return;
      let next = last.current;
      try {
        next = await load(nodeId);
      } catch {
      }
      if (!live) return;
      const done = settled(last.current, next);
      last.current = next;
      setActivity(next);
      if (done.length > 0) settledRef.current?.(done);
      timer = setTimeout(() => void run(), pollDelay(next));
    };
    const onVisible = () => {
      if (document.visibilityState === "visible") void run();
      else clearTimeout(timer);
    };
    void run();
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      live = false;
      clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [nodeId, load, tick]);

  const refresh = useCallback(() => setTick((n) => n + 1), []);
  return { activity, refresh };
}
