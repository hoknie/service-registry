export const WORKTREE_MARK = "+worktree:";

export function shortCommit(commit: string | null | undefined): { short: string; worktree: boolean } {
  if (!commit) return { short: "—", worktree: false };
  const at = commit.indexOf(WORKTREE_MARK);
  if (at >= 0) return { short: commit.slice(0, Math.min(7, at)), worktree: true };
  if (commit.startsWith("sha256:")) return { short: commit.slice(7, 14), worktree: false };
  return { short: commit.slice(0, 7), worktree: false };
}
