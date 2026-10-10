"use client";

import { FileText, Pencil } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { apiGet, type CatalogNode, type KnowledgeFileContent, type KnowledgeFiles, type Readme } from "@/lib/api";
import { isMarkdownReadme, pickReadme } from "@/lib/readmeFallback";

import { EmptyState } from "../ui/EmptyState";
import { Panel } from "../ui/Panel";
import { Markdown } from "./Markdown";
import { catalogHref, type CatalogLabels } from "./shared";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; canWrite: boolean };

type Fallback = { kind: "docs"; path: string; branch: string; text: string } | { kind: "forge"; text: string };

async function docsReadme(base: `/${string}`): Promise<Fallback | null> {
  const files = await apiGet<KnowledgeFiles>(`${base}/knowledge/files`);
  const readme = pickReadme(files.items.filter((f) => !f.skip_reason));
  if (!readme) return null;
  const file = await apiGet<KnowledgeFileContent>(`${base}/knowledge/file?path=${encodeURIComponent(readme.path)}`);
  return file.content?.trim() ? { kind: "docs", path: readme.path, branch: file.snapshot.branch, text: file.content } : null;
}

async function forgeReadme(base: `/${string}`): Promise<Fallback | null> {
  const r = await apiGet<Readme>(`${base}/readme`);
  return r.markdown.trim() ? { kind: "forge", text: r.markdown } : null;
}

async function loadFallback(node: CatalogNode): Promise<Fallback | null> {
  const base: `/${string}` = `/v1/catalog/nodes/${node.id}`;
  const docs = await docsReadme(base).catch(() => null);
  if (docs || !node.repository?.has_readme) return docs;
  return forgeReadme(base).catch(() => null);
}

export function DescriptionTab({ node, locale, labels, canWrite }: Props) {
  const t = labels.about;
  const description = node.description?.trim() ?? "";
  const [fallback, setFallback] = useState<Fallback | null | undefined>(description ? null : undefined);

  useEffect(() => {
    if (description) return;
    let live = true;
    void loadFallback(node).then((f) => live && setFallback(f));
    return () => {
      live = false;
    };
  }, [description, node]);

  const add = canWrite && (
    <Link
      href={catalogHref(locale, node.id, "settings", null, null, "general")}
      className="inline-flex items-center gap-1.5 text-sm font-medium text-signal hover:underline"
    >
      <Pencil aria-hidden="true" className="size-4" />
      {t.add}
    </Link>
  );

  if (description) {
    return (
      <Panel title={t.title}>
        <Markdown source={description} />
      </Panel>
    );
  }
  if (fallback === undefined) return null;
  if (!fallback) return <EmptyState icon={FileText} title={t.empty} action={add || undefined} />;
  return (
    <Panel title={t.title}>
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-line bg-surface-2/60 px-4 py-2.5 text-sm">
        <p className="text-ink-2">
          {fallback.kind === "docs" ? (
            <>
              {t.fromReadme}{" "}
              <Link href={catalogHref(locale, node.id, "docs", null, fallback.path)} className="font-mono text-signal hover:underline">
                {fallback.path}
              </Link>{" "}
              · {format(t.onBranch, { branch: fallback.branch })}
            </>
          ) : (
            t.fromForge
          )}
        </p>
        {add}
      </div>
      {fallback.kind === "forge" || isMarkdownReadme(fallback.path) ? (
        <Markdown source={fallback.text} />
      ) : (
        <pre className="overflow-x-auto font-mono text-sm whitespace-pre-wrap text-ink-2">{fallback.text}</pre>
      )}
    </Panel>
  );
}
