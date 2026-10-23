"use client";

import { useEffect, useRef, useState } from "react";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, errorCode, type CatalogNode, type KnowledgeFileContent } from "@/lib/api";
import { apiQuery, isApiCandidate, viewParam, withView, type ApiDocument, type ApiView as View } from "@/lib/apis";
import { structuredKind } from "@/lib/structured";

import { ApiView } from "../apis/ApiView";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
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
  docOf?: (href: string) => string | null;
  onHeadings?: (headings: DocHeading[]) => void;
};

export type DocHeading = { id: string; text: string; level: number };

function headingsOf(root: HTMLElement): DocHeading[] {
  const all = [...root.querySelectorAll<HTMLElement>("h1[id], h2[id], h3[id], h4[id]")].map((h) => {
    const copy = h.cloneNode(true) as HTMLElement;
    copy.querySelectorAll(".heading-anchor").forEach((a) => a.remove());
    return { id: h.id, text: (copy.textContent ?? "").trim(), level: Number(h.tagName.slice(1)) };
  });
  const sub = all.filter((h) => h.level > 1);
  return sub.length > 0 ? sub : all;
}

function resolveImport(paths: string[], imp: string): string | null {
  if (paths.includes(imp)) return imp;
  const found = paths.filter((p) => p.endsWith(`/${imp}`));
  return found.length === 1 ? (found[0] ?? null) : null;
}

const docQuery = (href: string) => new URL(href, window.location.origin).searchParams.get("doc");

export function DocViewer({ node, branch, path, paths, docHref, onOpenDoc, labels: t, docOf = docQuery, onHeadings }: Props) {
  const body = useRef<HTMLDivElement>(null);
  const { errors, common } = useUiText();
  const [file, setFile] = useState<KnowledgeFileContent | null>(null);
  const [html, setHtml] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [api, setApi] = useState<ApiDocument | null | undefined>(isApiCandidate(path) ? undefined : null);
  const [view, setView] = useState<View>(() => (typeof window === "undefined" ? "api" : viewParam(window.location.search)));

  useEffect(() => {
    if (!isApiCandidate(path)) return;
    let live = true;
    apiGet<ApiDocument>(`/v1/catalog/nodes/${node.id}/knowledge/api?${apiQuery(path, branch)}`)
      .then((d) => live && setApi(d))
      .catch(() => live && setApi(null));
    return () => {
      live = false;
    };
  }, [node.id, branch, path]);

  const changeView = (next: View) => {
    setView(next);
    window.history.replaceState(window.history.state, "", withView(window.location.href, next));
  };

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
          if (live) setHtml(renderReadme(f.content, { webUrl: node.repo_url, branch, kind: node.forge, dir, docLink, alerts: common.alerts, anchorLabel: common.headingAnchor }));
        }
      })
      .catch((e: unknown) => live && setError(errorText(errors, errorCode(e))));
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [node.id, node.repo_url, node.forge, branch, path, errors, common.alerts, common.headingAnchor]);

  const markdown = !!file && file.content !== null && /\.(md|markdown)$/i.test(file.path);

  useEffect(() => {
    if (html && body.current) {
      onHeadings?.(headingsOf(body.current));
      const hash = decodeURIComponent(window.location.hash.slice(1));
      if (hash) document.getElementById(hash)?.scrollIntoView();
      return;
    }
    if ((file && !markdown) || error) onHeadings?.([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [html, file, markdown, error]);

  const onClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const a = (e.target as HTMLElement).closest("a");
    const href = a?.getAttribute("href");
    if (!href || !href.startsWith("/") || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    const doc = docOf(href);
    if (doc === null) return;
    e.preventDefault();
    onOpenDoc(doc);
  };

  return (
    <Panel
      title={
        <span className="flex flex-wrap items-center gap-2">
          <span className="font-mono text-sm">{path}</span>
          {file?.git_status && (
            <Badge tone={file.git_status === "untracked" ? "amber" : "outline"} title={t.gitStatusHint[file.git_status]}>
              {t.gitStatus[file.git_status]}
            </Badge>
          )}
        </span>
      }
    >
      {error && <Message note={{ kind: "error", text: error }} />}
      {(!file || api === undefined) && !error && <SkeletonPanel lines={6} label={common.loading} />}
      {file && api && (
        <ApiView
          doc={api}
          content={file.content}
          view={view}
          onView={changeView}
          labels={t}
          resolveImport={(imp) => resolveImport(paths, imp)}
          fileHref={docHref}
          onOpenFile={onOpenDoc}
        />
      )}
      {file && api === null && file.content === null && (
        <p className="text-sm text-muted">{format(t.skippedFile, { reason: file.skip_reason ? t.skipped[file.skip_reason] : "" })}</p>
      )}
      {file && api === null && file.content !== null && html && (
        <div
          ref={body}
          className="readme max-w-none animate-rise-in text-sm leading-relaxed text-ink-2"
          onClick={onClick}
          dangerouslySetInnerHTML={{ __html: html }}
        />
      )}
      {file && api === null && file.content !== null && !html && structuredKind(file.path) && (
        <StructuredView kind={structuredKind(file.path)!} content={file.content} labels={t.viewer} />
      )}
      {file && api === null && file.content !== null && !html && !structuredKind(file.path) && (
        <pre className="overflow-x-auto rounded-md bg-surface-2 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap text-ink-2">{file.content}</pre>
      )}
    </Panel>
  );
}
