"use client";

import { Archive, ExternalLink, Star, TriangleAlert } from "lucide-react";

import type { Locale } from "@/i18n/config";
import { ago, when } from "@/i18n/time";
import type { CatalogNode, Repository } from "@/lib/api";

import { Badge } from "../ui/Badge";
import { Panel } from "../ui/Panel";
import { FORGE_NAMES, type CatalogLabels } from "../catalog/shared";
import { Readme } from "./Readme";

type Props = { node: CatalogNode; repository: Repository; locale: Locale; labels: CatalogLabels };

const BAR = ["bg-signal", "bg-cobalt", "bg-amber", "bg-danger", "bg-ink-2"] as const;

export function RepositoryPanel({ node, repository: r, locale, labels }: Props) {
  const t = labels.repoMeta;
  const languages = Object.entries(r.languages).sort((a, b) => b[1] - a[1]);
  return (
    <>
      <Panel
        title={t.title}
        actions={
          <a href={r.web_url} target="_blank" rel="noreferrer noopener" className="inline-flex items-center gap-1.5 text-sm text-signal hover:underline">
            {t.open}
            <ExternalLink aria-hidden="true" className="size-3.5" />
          </a>
        }
      >
        <div className="grid gap-4">
          {(r.archived || r.orphaned_at) && (
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
          {r.orphaned_at && <p className="text-sm text-ink-2">{t.orphanedText}</p>}
          {r.topics.length > 0 && (
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
          <dl className="grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-2.5 text-sm">
            <dt className="text-muted">{labels.repo.forge}</dt>
            <dd className="text-ink">{node.forge ? FORGE_NAMES[node.forge] : t.none}</dd>
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
            <dt className="text-muted">{t.synced}</dt>
            <dd className="text-ink" title={when(r.synced_at, locale, "")}>
              {ago(r.synced_at, locale, t.none)}
            </dd>
          </dl>
        </div>
      </Panel>
      {r.has_readme && <Readme projectId={node.id} labels={t} />}
    </>
  );
}
