"use client";

import { useEffect, useState } from "react";

import { format } from "@/i18n/format";
import { apiGet } from "@/lib/api";
import type { ApiList } from "@/lib/apis";

import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { DocLink } from "../docs/DocLink";
import type { ApiLabels } from "./ApiView";
import { refQuery } from "@/lib/refs";

type Props = { projectId: string; branch: string; labels: ApiLabels; hrefOf: (path: string) => string };

export function ApiCards({ projectId, branch, labels: t, hrefOf }: Props) {
  const [list, setList] = useState<ApiList>({ items: [], hidden_vendored: 0 });
  const [all, setAll] = useState(false);
  useEffect(() => {
    let live = true;
    const query = refQuery(branch);
    if (all) query.set("all", "1");
    apiGet<ApiList>(`/v1/catalog/nodes/${projectId}/knowledge/apis?${query}`)
      .then((r) => live && setList(r))
      .catch(() => live && setList({ items: [], hidden_vendored: 0 }));
    return () => {
      live = false;
    };
  }, [projectId, branch, all]);
  const { items } = list;
  const vendored = items.filter((a) => a.vendored).length;
  if (items.length === 0 && list.hidden_vendored === 0) return null;
  return (
    <section aria-label={t.block} className="grid gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <h2 className="text-sm font-semibold tracking-wide text-muted uppercase">{t.block}</h2>
        {(list.hidden_vendored > 0 || vendored > 0) && (
          <Button type="button" size="sm" variant="ghost" className="ml-auto" aria-pressed={all} onClick={() => setAll(!all)}>
            {all ? t.hideVendored : format(t.showVendored, { n: String(list.hidden_vendored) })}
          </Button>
        )}
      </div>
      <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
        {items.map((a) => (
          <li key={a.path} className="min-w-0">
            <DocLink href={hrefOf(a.path)} className="grid h-full grid-cols-[minmax(0,1fr)] gap-1 rounded-card border border-line bg-surface p-3 shadow-elev-1 hover:border-line-strong">
              <span className="flex min-w-0 items-center gap-2">
                <Badge tone="outline">{a.format}</Badge>
                <span className="min-w-0 truncate font-medium text-ink">{a.title ?? a.path}</span>
                {a.version && <span className="font-mono text-xs text-muted">{a.version}</span>}
                {a.vendored && <Badge tone="outline">{t.vendored}</Badge>}
              </span>
              <span className="truncate font-mono text-xs text-muted">{a.path}</span>
              <span className="flex flex-wrap items-center gap-2 text-xs text-ink-2">
                {a.skip_reason && t.skippedFile}
                {a.operations !== null && format(t.cardOperations, { n: String(a.operations) })}
                {a.methods !== null && format(t.cardMethods, { services: String(a.services ?? 0), methods: String(a.methods) })}
                {a.problems > 0 && <Badge tone="amber">{format(t.cardProblems, { n: String(a.problems) })}</Badge>}
              </span>
            </DocLink>
          </li>
        ))}
      </ul>
    </section>
  );
}
