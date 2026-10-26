"use client";

import { FileText, Folder } from "lucide-react";
import { useEffect } from "react";

import { format } from "@/i18n/format";
import { docHref, fileName, splitPath } from "@/lib/docPath";
import { ensureLevel, levelEntry, useDocStore } from "@/lib/docSpaceStore";

import { useUiText } from "../UiText";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import { DocLink } from "./DocLink";

type Props = {
  projectId: string;
  nodePath: string[];
  branch: string | null;
  linkBranch: string | null;
  dir: string;
  labels: { filesCount: string; dirEmpty: string };
};

export function DirectoryPage({ projectId, nodePath, branch, linkBranch, dir, labels: t }: Props) {
  const { common } = useUiText();
  useDocStore();
  useEffect(() => {
    ensureLevel(projectId, branch, dir);
  }, [projectId, branch, dir]);
  const level = levelEntry(projectId, branch, dir)?.value;
  const href = (path: string) => docHref([...nodePath, ...splitPath(path)], linkBranch);
  return (
    <Panel title={fileName(dir)}>
      {!level ? (
        <SkeletonPanel lines={4} label={common.loading} />
      ) : level.dirs.length + level.files.length === 0 ? (
        <p className="text-sm text-muted">{t.dirEmpty}</p>
      ) : (
        <ul className="grid grid-cols-[minmax(0,1fr)] gap-1 text-sm">
          {level.dirs.map((d) => (
            <li key={d.path} className="min-w-0">
              <DocLink href={href(d.path)} className="flex items-center gap-2 rounded-md px-2 py-1.5 hover:bg-surface-2">
                <Folder aria-hidden="true" className="size-4 shrink-0 text-muted" />
                <span className="min-w-0 flex-1 truncate font-medium text-ink">{d.name}</span>
                <span className="text-xs text-muted">{format(t.filesCount, { n: String(d.files) })}</span>
              </DocLink>
            </li>
          ))}
          {level.files.map((f) => (
            <li key={f.path} className="min-w-0">
              <DocLink href={href(f.path)} title={f.path} className="flex items-start gap-2 rounded-md px-2 py-1.5 hover:bg-surface-2">
                <FileText aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-muted" />
                <span className="min-w-0 flex-1 text-signal [overflow-wrap:anywhere]">{f.title || fileName(f.path)}</span>
              </DocLink>
            </li>
          ))}
        </ul>
      )}
    </Panel>
  );
}
