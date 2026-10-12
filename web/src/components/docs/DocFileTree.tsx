"use client";

import { ChevronRight, FileText, Folder, FolderOpen } from "lucide-react";
import { useEffect, useState } from "react";

import type { GitStatus } from "@/lib/api";
import { cn } from "@/lib/cn";
import { docHref, splitPath } from "@/lib/docPath";
import { ensureLevel, levelEntry, useDocStore } from "@/lib/docSpaceStore";

import type { CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Spinner } from "../ui/Spinner";
import { DocLink } from "./DocLink";

type Labels = CatalogLabels["docs"];

type Props = {
  projectId: string;
  projectPath: string[];
  branch: string | null;
  linkBranch: string | null;
  openPath: string | null;
  depth: number;
  labels: Labels;
  emptyLabel: string;
  retryLabel: string;
};

const indent = (depth: number) => ({
  marginLeft: `${depth * 0.9}rem`,
  width: `calc(100% - ${depth * 0.9}rem)`,
});

function ancestorsOf(path: string | null): string[] {
  if (!path) return [];
  const parts = path.split("/").slice(0, -1);
  return parts.map((_, i) => parts.slice(0, i + 1).join("/"));
}

function GitMark({
  git,
  labels,
}: {
  git: GitStatus | null | undefined;
  labels: Labels;
}) {
  if (!git) return null;
  return (
    <span
      role="img"
      aria-label={labels.gitStatus[git]}
      className={cn(
        "shrink-0 font-mono text-[0.7rem] font-semibold not-italic",
        git === "untracked" ? "text-amber" : "text-muted",
      )}
    >
      {git === "untracked" ? "U" : "I"}
    </span>
  );
}

export function DocFileTree({
  projectId,
  projectPath,
  branch,
  linkBranch,
  openPath,
  depth,
  labels: t,
  emptyLabel,
  retryLabel,
}: Props) {
  const { common } = useUiText();
  useDocStore();
  const [opened, setOpened] = useState<Set<string>>(new Set());
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const onPath = new Set(ancestorsOf(openPath));
  const isOpen = (dir: string) =>
    onPath.has(dir) ? !collapsed.has(dir) : opened.has(dir);
  const pathKey = ancestorsOf(openPath).join("|");

  useEffect(() => {
    ensureLevel(projectId, branch, "");
    for (const dir of pathKey ? pathKey.split("|") : [])
      ensureLevel(projectId, branch, dir);
  }, [projectId, branch, pathKey]);

  const toggle = (dir: string) => {
    const opening = !isOpen(dir);
    const flip = (prev: Set<string>, on: boolean) => {
      const next = new Set(prev);
      if (on) next.add(dir);
      else next.delete(dir);
      return next;
    };
    if (onPath.has(dir)) setCollapsed((prev) => flip(prev, !opening));
    else setOpened((prev) => flip(prev, opening));
    if (opening) ensureLevel(projectId, branch, dir);
  };

  const render = (dir: string, level: number): React.ReactNode => {
    const entry = levelEntry(projectId, branch, dir);
    if (entry?.error === "not_found" && !entry.loading && dir === "") {
      return (
        <li className="py-1 text-xs text-muted" style={indent(level + 0.3)}>
          {emptyLabel}
        </li>
      );
    }
    if (entry?.error !== undefined && !entry.loading) {
      return (
        <li>
          <button
            type="button"
            onClick={() => ensureLevel(projectId, branch, dir, true)}
            className="py-1 text-xs text-danger hover:underline"
            style={indent(level + 1.6)}
          >
            {retryLabel}
          </button>
        </li>
      );
    }
    const value = entry?.value;
    if (!value) {
      return dir === "" ? (
        <li
          className="flex items-center gap-2 py-1 text-xs text-muted"
          style={indent(level + 0.3)}
        >
          <Spinner className="size-3.5 text-signal" />
          {common.loading}
        </li>
      ) : null;
    }
    if (dir === "" && value.dirs.length === 0 && value.files.length === 0) {
      return (
        <li className="py-1 text-xs text-muted" style={indent(level + 0.3)}>
          {emptyLabel}
        </li>
      );
    }
    return [
      ...value.dirs.map((d) => {
        const open = isOpen(d.path);
        const busy = !!levelEntry(projectId, branch, d.path)?.loading;
        return (
          <li key={`d:${d.path}`} className="min-w-0">
            <button
              type="button"
              onClick={() => toggle(d.path)}
              aria-expanded={open}
              aria-busy={busy || undefined}
              title={
                d.git_status
                  ? `${d.path} — ${t.gitStatusHint[d.git_status]}`
                  : d.path
              }
              className={cn(
                "motion-control flex min-w-0 items-center gap-1.5 rounded-md py-1 pr-2 pl-1 text-left text-sm text-ink-2 hover:bg-surface-2",
                d.git_status === "untracked" && "text-amber",
                d.git_status === "ignored" && "text-muted italic",
              )}
              style={indent(level)}
            >
              <span className="grid size-4 shrink-0 place-items-center">
                {busy ? (
                  <Spinner
                    className="size-3 text-signal"
                    label={common.loading}
                  />
                ) : (
                  <ChevronRight
                    aria-hidden="true"
                    className={cn(
                      "motion-control size-3.5 text-muted",
                      open && "rotate-90",
                    )}
                  />
                )}
              </span>
              {open ? (
                <FolderOpen
                  aria-hidden="true"
                  className="size-3.5 shrink-0 text-muted"
                />
              ) : (
                <Folder
                  aria-hidden="true"
                  className="size-3.5 shrink-0 text-muted"
                />
              )}
              <span className="min-w-0 flex-1 truncate font-medium">
                {d.name}
              </span>
              {d.git_status && (
                <span
                  role="img"
                  aria-label={t.gitStatus[d.git_status]}
                  className={cn(
                    "size-1.5 shrink-0 rounded-full",
                    d.git_status === "untracked"
                      ? "bg-amber"
                      : "bg-line-strong",
                  )}
                />
              )}
            </button>
            {open && (
              <ul className="grid min-w-0 animate-fade-in gap-0.5 pt-0.5">
                {render(d.path, level + 1)}
              </ul>
            )}
          </li>
        );
      }),
      ...value.files.map((f) => {
        const name = f.path.slice(f.path.lastIndexOf("/") + 1);
        const current = openPath === f.path;
        const git = f.git_status ?? null;
        const hint = [
          f.path,
          f.skip_reason ? t.skipped[f.skip_reason] : null,
          git ? t.gitStatusHint[git] : null,
        ]
          .filter(Boolean)
          .join(" — ");
        return (
          <li key={`f:${f.path}`} className="min-w-0">
            <DocLink
              href={docHref([...projectPath, ...splitPath(f.path)], linkBranch)}
              aria-current={current ? "page" : undefined}
              title={hint}
              className={cn(
                "motion-control flex min-w-0 items-center gap-1.5 rounded-md py-1 pr-2 pl-1.5 text-sm",
                current
                  ? "bg-signal-soft text-signal"
                  : "text-ink-2 hover:bg-surface-2",
                !current && git === "untracked" && "text-amber",
                !current &&
                  (f.skip_reason || git === "ignored") &&
                  "text-muted",
                git === "ignored" && "italic",
              )}
              style={indent(level + 1.1)}
            >
              <FileText aria-hidden="true" className="size-3.5 shrink-0" />
              <span className="min-w-0 flex-1 truncate">{name}</span>
              <GitMark git={git} labels={t} />
            </DocLink>
          </li>
        );
      }),
    ];
  };

  return <ul className="grid min-w-0 gap-0.5 pt-0.5">{render("", depth)}</ul>;
}
