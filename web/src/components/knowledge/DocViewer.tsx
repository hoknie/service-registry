"use client";

import { useEffect, useRef, useState } from "react";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, errorCode, type CatalogNode, type KnowledgeFileContent } from "@/lib/api";
import { isApiCandidate, viewParam, withView, type ApiDocument, type ApiView as View } from "@/lib/apis";
import { fileUrl, isPublicSource } from "@/lib/docSpaceStore";
import { cn } from "@/lib/cn";
import { structuredKind } from "@/lib/structured";

import { ApiView } from "../apis/ApiView";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import type { CatalogLabels } from "../catalog/shared";
import { StructuredView } from "./StructuredView";
import { refQuery, splitVersion } from "@/lib/refs";

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

type Loaded = { key: string; file: KnowledgeFileContent; html: string | null; api: ApiDocument | null };

export function DocViewer({ node, branch, path, paths, docHref, onOpenDoc, labels: t, docOf = docQuery, onHeadings }: Props) {
  const body = useRef<HTMLDivElement>(null);
  const { errors, common } = useUiText();
  const key = `${node.id}:${branch}:${path}`;
  const [doc, setDoc] = useState<Loaded | null>(null);
  const [failure, setFailure] = useState<{ key: string; text: string } | null>(null);
  const [view, setView] = useState<View>(() => (typeof window === "undefined" ? "api" : viewParam(window.location.search)));

  const changeView = (next: View) => {
    setView(next);
    window.history.replaceState(window.history.state, "", withView(window.location.href, next));
  };

  useEffect(() => {
    let live = true;
    const apiDoc = isApiCandidate(path) && !isPublicSource()
      ? apiGet<ApiDocument>(`/v1/catalog/nodes/${node.id}/knowledge/api?${refQuery(branch, { path }).toString()}`).catch(() => null)
      : Promise.resolve(null);
    apiGet<KnowledgeFileContent>(fileUrl(node.id, branch, path))
      .then(async (f) => {
        let html: string | null = null;
        if (f.content !== null && /\.(md|markdown)$/i.test(f.path)) {
          const { renderReadme } = await import("@/lib/readme");
          const dir = f.path.includes("/") ? f.path.slice(0, f.path.lastIndexOf("/")) : undefined;
          const known = new Set(paths);
          const docLink = (p: string) => (known.has(p) ? docHref(p) : null);
          html = renderReadme(f.content, { webUrl: node.repo_url, branch: splitVersion(branch).name, kind: node.forge, dir, docLink, alerts: common.alerts, anchorLabel: common.headingAnchor });
        }
        const api = await apiDoc;
        if (!live) return;
        setView(viewParam(window.location.search));
        setDoc({ key, file: f, html, api });
        setFailure(null);
      })
      .catch((e: unknown) => live && setFailure({ key, text: errorText(errors, errorCode(e)) }));
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, node.repo_url, node.forge, errors, common.alerts, common.headingAnchor]);

  const current = doc?.key === key ? doc : null;
  const error = failure?.key === key ? failure.text : null;
  const shown = error ? null : (current ?? doc);
  const pending = !current && !error && !!doc;
  const file = shown?.file ?? null;
  const html = shown?.html ?? null;
  const api = shown?.api ?? null;

  useEffect(() => {
    if (!current && !error) return;
    if (current?.html && body.current) {
      onHeadings?.(headingsOf(body.current));
      const hash = decodeURIComponent(window.location.hash.slice(1));
      if (hash) document.getElementById(hash)?.scrollIntoView();
      return;
    }
    onHeadings?.([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current, error]);

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
          <span className="min-w-0 font-mono text-sm [overflow-wrap:anywhere]">{file?.path ?? path}</span>
          {file?.git_status && (
            <Badge tone={file.git_status === "untracked" ? "amber" : "outline"} title={t.gitStatusHint[file.git_status]}>
              {t.gitStatus[file.git_status]}
            </Badge>
          )}
        </span>
      }
    >
      <div aria-busy={pending || undefined} inert={pending} className={cn("motion-control", pending && "opacity-60 delay-150")}>
        {error && <Message note={{ kind: "error", text: error }} />}
        {!shown && !error && <SkeletonPanel lines={6} label={common.loading} />}
        {file && api && (
          <ApiView
            key={shown?.key}
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
        {file && !api && file.content === null && (
          <p className="text-sm text-muted">{format(t.skippedFile, { reason: file.skip_reason ? t.skipped[file.skip_reason] : "" })}</p>
        )}
        {file && !api && file.content !== null && html && (
          <div
            key={shown?.key}
            ref={body}
            className="readme max-w-none animate-rise-in text-sm leading-relaxed text-ink-2"
            onClick={onClick}
            dangerouslySetInnerHTML={{ __html: html }}
          />
        )}
        {file && !api && file.content !== null && !html && structuredKind(file.path) && (
          <StructuredView key={shown?.key} kind={structuredKind(file.path)!} content={file.content} labels={t.viewer} />
        )}
        {file && !api && file.content !== null && !html && !structuredKind(file.path) && (
          <pre className="overflow-x-auto rounded-md bg-surface-2 p-4 font-mono text-xs leading-relaxed whitespace-pre-wrap text-ink-2">{file.content}</pre>
        )}
      </div>
    </Panel>
  );
}
