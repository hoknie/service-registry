"use client";

import { Archive, ExternalLink, Star, TriangleAlert } from "lucide-react";
import Link from "next/link";
import { useEffect, useState, type ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { ago, when } from "@/i18n/time";
import { apiGet, type CatalogNode, type RepoSyncConnection, type Items, type KnowledgeSource } from "@/lib/api";

import { Readme } from "../reposync/Readme";
import { Badge } from "../ui/Badge";
import { Panel } from "../ui/Panel";
import { UnlinkRepoSyncButton } from "./UnlinkRepoSyncDialog";
import { catalogHref, HOSTING_NAMES, type CatalogLabels } from "./shared";

type Props = {
  node: CatalogNode;
  locale: Locale;
  labels: CatalogLabels;
  canWrite: boolean;
  fail: (e: unknown) => string;
  onChanged: () => void;
};

const BAR = ["bg-signal", "bg-cobalt", "bg-amber", "bg-danger", "bg-ink-2"] as const;

type Owner = { nodeId: string; owner: string };

export function RepositoryBlock({ node, locale, labels, canWrite, fail, onChanged }: Props) {
  const t = labels.repoMeta;
  const r = node.repository ?? null;
  const [source, setSource] = useState<KnowledgeSource | null | undefined>(undefined);
  const [owner, setOwner] = useState<Owner | null>(null);
  const connectionId = r?.origin === "repo_sync" ? r.connection_id : null;
  const ancestors = (node.path ?? []).map((p) => p.id).join(",");

  useEffect(() => {
    let live = true;
    apiGet<KnowledgeSource | null>(`/v1/catalog/nodes/${node.id}/knowledge/source`)
      .then((s) => live && setSource(s))
      .catch(() => live && setSource(null));
    return () => {
      live = false;
    };
  }, [node.id, node.updated_at]);

  useEffect(() => {
    if (!connectionId || !ancestors) return;
    let live = true;
    Promise.all(
      ancestors.split(",").map((id) =>
        apiGet<Items<RepoSyncConnection>>(`/v1/catalog/nodes/${id}/repo-sync/connections`)
          .then((res) => res.items.find((c) => c.id === connectionId) ?? null)
          .catch(() => null),
      ),
    ).then((found) => {
      const c = found.find((x) => x !== null);
      if (live && c) setOwner({ nodeId: c.node_id, owner: c.owner_path });
    });
    return () => {
      live = false;
    };
  }, [connectionId, ancestors]);

  if (!node.repo_url && !r && !source) return null;

  const webUrl = r?.web_url ?? node.repo_url ?? source?.url ?? null;
  const branch = node.default_branch ?? r?.default_branch ?? null;
  const languages = Object.entries(r?.languages ?? {}).sort((a, b) => b[1] - a[1]);
  const managed = !!node.managed;

  let origin: ReactNode;
  if (r?.origin === "repo_sync" || managed) {
    const name = owner?.owner ?? r?.full_path.split("/").slice(0, -1).join("/") ?? "";
    origin = owner ? (
      <Link href={catalogHref(locale, owner.nodeId, "settings", null, null, "repo-sync")} className="text-signal hover:underline">
        {format(t.originRepoSync, { owner: name })}
      </Link>
    ) : (
      format(t.originRepoSync, { owner: name })
    );
  } else if (source) {
    origin = format(t.originSource, { kind: labels.docs.source.kinds[source.kind] });
  } else {
    origin = t.originLink;
  }

  return (
    <div className="grid gap-4">
      <Panel
        title={labels.repo.title}
        actions={
          webUrl && (
            <a href={webUrl} target="_blank" rel="noreferrer noopener" className="inline-flex items-center gap-1.5 text-sm text-signal hover:underline">
              {t.open}
              <ExternalLink aria-hidden="true" className="size-3.5" />
            </a>
          )
        }
      >
        <div className="grid gap-4">
          {r && (r.archived || r.orphaned_at) && (
            <div className="flex flex-wrap gap-2">
              {r.archived && (
                <Badge tone="neutral">
                  <Archive aria-hidden="true" className="size-3" />
                  {t.archived}
                </Badge>
              )}
              {r.orphaned_at && (
                <Badge tone="danger">
                  <TriangleAlert aria-hidden="true" className="size-3" />
                  {t.orphaned}
                </Badge>
              )}
            </div>
          )}
          {r?.orphaned_at && <p className="text-sm text-ink-2">{t.orphanedText}</p>}
          <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-2.5 text-sm">
            <dt className="text-muted">{labels.repo.hosting}</dt>
            <dd className="text-ink">{node.hosting ? HOSTING_NAMES[node.hosting] : (source?.hosting && HOSTING_NAMES[source.hosting]) || t.none}</dd>
            <dt className="text-muted">{labels.repo.url}</dt>
            <dd className="min-w-0">
              {webUrl ? (
                <a href={webUrl} rel="noreferrer noopener" target="_blank" className="inline-flex max-w-full items-center gap-1.5 text-signal hover:underline">
                  <span className="truncate">{webUrl}</span>
                  <ExternalLink aria-hidden="true" className="size-3.5 shrink-0" />
                </a>
              ) : (
                t.none
              )}
            </dd>
            <dt className="text-muted">{labels.docs.source.apiUrl}</dt>
            <dd className="min-w-0 break-all">{source?.api_url ? <code className="text-ink">{source.api_url}</code> : t.none}</dd>
            <dt className="text-muted">{labels.repo.branch}</dt>
            <dd>
              {branch ? (
                <Link href={catalogHref(locale, node.id, "overview", branch)} className="font-mono text-signal hover:underline">
                  {branch}
                </Link>
              ) : (
                t.none
              )}
            </dd>
            <dt className="text-muted">{t.origin}</dt>
            <dd className="text-ink">{origin}</dd>
            {r && (
              <>
                <dt className="text-muted">{t.stars}</dt>
                <dd className="inline-flex items-center gap-1 text-ink">
                  <Star aria-hidden="true" className="size-3.5 text-amber" />
                  {r.stars}
                </dd>
                <dt className="text-muted">{t.license}</dt>
                <dd className="text-ink">{r.license ?? t.none}</dd>
                <dt className="text-muted">{t.visibility}</dt>
                <dd className="text-ink">{t.visibilities[r.visibility]}</dd>
                <dt className="text-muted">{t.pushed}</dt>
                <dd className="text-ink" title={when(r.pushed_at, locale, "")}>
                  {ago(r.pushed_at, locale, t.none)}
                </dd>
                <dt className="text-muted">{r.origin === "source" ? t.collected : t.synced}</dt>
                <dd className="text-ink" title={when(r.synced_at, locale, "")}>
                  {ago(r.synced_at, locale, t.none)}
                </dd>
              </>
            )}
          </dl>
          {r && r.topics.length > 0 && (
            <ul aria-label={t.topics} className="flex flex-wrap gap-1.5">
              {r.topics.map((topic) => (
                <li key={topic}>
                  <Badge tone="outline">{topic}</Badge>
                </li>
              ))}
            </ul>
          )}
          {languages.length > 0 && (
            <div>
              <p className="mb-1.5 text-sm text-muted">{t.languages}</p>
              <div aria-hidden="true" className="flex h-2 overflow-hidden rounded-full bg-surface-3">
                {languages.map(([name, pct], i) => (
                  <span key={name} className={BAR[i % BAR.length]} style={{ width: `${pct}%` }} />
                ))}
              </div>
              <ul className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-ink-2">
                {languages.map(([name, pct], i) => (
                  <li key={name} className="inline-flex items-center gap-1.5">
                    <span aria-hidden="true" className={`size-2 rounded-full ${BAR[i % BAR.length]}`} />
                    {name} <span className="text-muted">{pct}%</span>
                  </li>
                ))}
              </ul>
            </div>
          )}
          {managed && canWrite && (
            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-line pt-4">
              <p className="text-sm text-ink-2">{labels.unlink.text}</p>
              <UnlinkRepoSyncButton node={node} labels={labels} fail={fail} onUnlinked={onChanged} />
            </div>
          )}
        </div>
      </Panel>
      {r?.has_readme && <Readme key={`${node.id}:${r.synced_at}`} projectId={node.id} labels={t} />}
    </div>
  );
}
