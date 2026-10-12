"use client";

import { ChevronRight, FileText, Folder, FolderOpen } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import type { KnowledgeFile } from "@/lib/api";
import { cn } from "@/lib/cn";

import type { CatalogLabels } from "../catalog/shared";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../ui/Collapsible";

type Props = {
  files: KnowledgeFile[];
  open: string | null;
  labels: CatalogLabels["docs"];
  onOpen: (path: string) => void;
  depth?: number;
};

type Dir = { name: string; path: string; dirs: Dir[]; files: KnowledgeFile[] };

function buildTree(files: KnowledgeFile[]): Dir {
  const root: Dir = { name: "", path: "", dirs: [], files: [] };
  for (const f of files) {
    const parts = f.path.split("/");
    let dir = root;
    for (const name of parts.slice(0, -1)) {
      const path = dir.path ? `${dir.path}/${name}` : name;
      let next = dir.dirs.find((d) => d.name === name);
      if (!next) {
        next = { name, path, dirs: [], files: [] };
        dir.dirs.push(next);
      }
      dir = next;
    }
    dir.files.push(f);
  }
  return root;
}

function dirGit(dir: Dir): "untracked" | "ignored" | null {
  const statuses = [...dir.files.map((f) => f.git_status ?? null), ...dir.dirs.map(dirGit)];
  if (statuses.length === 0) return null;
  if (statuses.includes("untracked")) return "untracked";
  return statuses.every((s) => s === "ignored") ? "ignored" : null;
}

function ancestors(path: string | null): string[] {
  if (!path) return [];
  const parts = path.split("/").slice(0, -1);
  return parts.map((_, i) => parts.slice(0, i + 1).join("/"));
}

const indent = (depth: number) => ({ marginLeft: `${depth * 0.9}rem`, width: `calc(100% - ${depth * 0.9}rem)` });
const row = "motion-control flex min-w-0 items-center gap-1.5 rounded-md py-1 pr-2 pl-2 text-left text-sm";

export function DocsTree({ files, open, labels: t, onOpen, depth: base = 0 }: Props) {
  const tree = useMemo(() => buildTree(files), [files]);
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set([...tree.dirs.map((d) => d.path), ...ancestors(open)]));

  useEffect(() => {
    const way = ancestors(open);
    if (way.length === 0) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setExpanded((prev) => (way.every((p) => prev.has(p)) ? prev : new Set([...prev, ...way])));
  }, [open]);

  const toggle = (path: string) =>
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return next;
    });

  const render = (dir: Dir, depth: number): React.ReactNode[] => [
    ...dir.dirs.map((d) => {
      const isOpen = expanded.has(d.path);
      const git = dirGit(d);
      return (
        <Collapsible asChild key={`d:${d.path}`} open={isOpen} onOpenChange={() => toggle(d.path)}>
          <li className="min-w-0">
            <CollapsibleTrigger asChild>
              <button
                type="button"
                title={git ? `${d.path} — ${t.gitStatusHint[git]}` : d.path}
                className={cn(row, "motion-control text-ink-2 hover:bg-surface-2", git === "untracked" && "text-amber", git === "ignored" && "text-muted italic")}
                style={indent(depth)}
              >
                <ChevronRight aria-hidden="true" className={cn("motion-control size-3.5 shrink-0 text-muted", isOpen && "rotate-90")} />
                {isOpen ? <FolderOpen aria-hidden="true" className="size-3.5 shrink-0 text-muted" /> : <Folder aria-hidden="true" className="size-3.5 shrink-0 text-muted" />}
                <span className="min-w-0 flex-1 truncate font-medium">{d.name}</span>
                {git && (
                  <span role="img" aria-label={t.gitStatus[git]} className={cn("size-1.5 shrink-0 rounded-full", git === "untracked" ? "bg-amber" : "bg-line-strong")} />
                )}
              </button>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <ul className="grid min-w-0 gap-0.5 pt-0.5">{render(d, depth + 1)}</ul>
            </CollapsibleContent>
          </li>
        </Collapsible>
      );
    }),
    ...dir.files.map((f) => {
      const name = f.path.slice(f.path.lastIndexOf("/") + 1);
      const current = open === f.path;
      const git = f.git_status ?? null;
      const hint = [f.path, f.skip_reason ? t.skipped[f.skip_reason] : null, git ? t.gitStatusHint[git] : null].filter(Boolean).join(" — ");
      return (
        <li key={`f:${f.path}`} className="min-w-0">
          <button
            type="button"
            onClick={() => onOpen(f.path)}
            aria-current={current ? "true" : undefined}
            title={hint}
            className={cn(
              row,
              current ? "bg-signal-soft text-signal" : "text-ink-2 hover:bg-surface-2",
              !current && git === "untracked" && "text-amber",
              !current && (f.skip_reason || git === "ignored") && "text-muted",
              git === "ignored" && "italic",
            )}
            style={indent(depth + 1)}
          >
            <FileText aria-hidden="true" className="size-3.5 shrink-0" />
            <span className="min-w-0 flex-1 truncate">{name}</span>
            {git && (
              <span aria-label={t.gitStatus[git]} className={cn("shrink-0 font-mono text-[0.7rem] font-semibold not-italic", git === "untracked" ? "text-amber" : "text-muted")}>
                {git === "untracked" ? "U" : "I"}
              </span>
            )}
          </button>
        </li>
      );
    }),
  ];

  return <ul className="grid min-w-0 gap-0.5 overflow-hidden">{render(tree, base)}</ul>;
}
