import type { Tone } from "../ui/Badge";
import type { CatalogLabels } from "../catalog/shared";
import type { SyncRun } from "@/lib/api";

export type RepoSyncLabels = CatalogLabels["repoSync"];

export const runTone: Record<SyncRun["status"], Tone> = {
  running: "neutral",
  succeeded: "signal",
  failed: "danger",
  rate_limited: "amber",
};

export function splitPatterns(text: string): string[] {
  return text
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);
}
