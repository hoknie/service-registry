"use client";

import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, errorCode, type CatalogNode, type KnowledgeFileContent } from "@/lib/api";
import { structuredKind } from "@/lib/structured";

import { useUiText } from "../UiText";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import type { CatalogLabels } from "../catalog/shared";
import { StructuredView } from "./StructuredView";

type Props = {
  node: CatalogNode;
  branch: string;
  path: string;
  paths: string[];
  docHref: (path: string) => string;
  onOpenDoc: (path: string) => void;
  labels: CatalogLabels["docs"];
};

export function DocViewer({ node, branch, path, paths, docHref, onOpenDoc, labels: t }: Props) {
  const { errors, common } = useUiText();
  const [file, setFile] = useState<KnowledgeFileContent | null>(null);
  const [html, setHtml] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let live = true;
    const query = new URLSearchParams({ path, branch });
    apiGet<KnowledgeFileContent>(`/v1/catalog/nodes/${node.id}/knowledge/file?${query}`)
      .then(async (f) => {
        if (!live) return;
        setFile(f);
        if (f.content !== null && /\.(md|markdown)$/i.test(f.path)) {
          const { renderReadme } = await import("@/lib/readme");
          const dir = f.path.includes("/") ? f.path.slice(0, f.path.lastIndexOf("/")) : undefined;
          const known = new Set(paths);
          const docLink = (p: string) => (known.has(p) ? docHref(p) : null);
          if (live) setHtml(renderReadme(f.content, { webUrl: node.repo_url, branch, kind: node.forge, dir, docLink }));
        }
      })
      .catch((e: unknown) => live && setError(errorText(errors, errorCode(e))));
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [node.id, node.repo_url, node.forge, branch, path, errors]);

  const onClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const a = (e.target as HTMLElement).closest("a");
    const href = a?.getAttribute("href");
    if (!href || !href.startsWith("/") || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    const doc = new URL(href, window.location.origin).searchParams.get("doc");
    if (doc === null) return;
    e.preventDefault();
    onOpenDoc(doc);
  };

  return (
    <Panel title={<span className="font-mono text-sm">{path}</span>}>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!file && !error && <SkeletonPanel lines={6} label={common.loading} />}
      {file && file.content === null && (
        <p className="text-sm text-muted">{format(t.skippedFile, { reason: file.skip_reason ? t.skipped[file.skip_reason] : "" })}</p>
      )}
      {file && file.content !== null && html && (
        <div className="readme max-w-none animate-rise-in text-sm leading-relaxed text-ink-2" onClick={onClick} dangerouslySetInnerHTML={{ __html: html }} />
      )}
      {file && file.content !== null && !html && structuredKind(file.path) && (
        <StructuredView kind={structuredKind(file.path)!} content={file.content} labels={t.viewer} />
      )}
      {file && file.content !== null && !html && !structuredKind(file.path) && (
        <pre className="overflow-x-auto rounded-md bg-surface-2 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap text-ink-2">{file.content}</pre>
      )}
    </Panel>
  );
}
