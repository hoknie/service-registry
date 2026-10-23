import type { Snapshot } from "./api";

export function snapshotMissing(s: Pick<Snapshot, "skipped_limit" | "skipped_too_large">): number {
  return (s.skipped_limit ?? 0) + (s.skipped_too_large ?? 0);
}
