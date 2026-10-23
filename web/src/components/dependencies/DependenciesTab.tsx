"use client";

import { Network } from "lucide-react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { CatalogNode } from "@/lib/api";
import { viewOf, depthOf, withView, targetKey, type Dependencies, type DependencyItem, type DependencyView } from "@/lib/dependencies";
import { getDependencies } from "@/lib/dependenciesApi";

import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Segmented } from "../ui/Segmented";
import { SkeletonRows } from "../ui/Skeleton";
import { KindIcon } from "../catalog/KindIcon";
import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { DependencyGraph } from "./DependencyGraph";
import { SourceBadges } from "./SourceBadges";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; canWrite: boolean; fail: (e: unknown) => string };

export function DependenciesTab({ node, locale, labels, canWrite, fail }: Props) {
  const t = labels.dependencies;
  const params = useSearchParams();
  const router = useRouter();
  const container = node.kind !== "project";
  const view = viewOf(params.get("view"), container);
  const depth = depthOf(params.get("depth"));
  const base = catalogHref(locale, node.id, "dependencies");
  const go = (v: DependencyView, d: number) => router.replace(withView(base, v, d), { scroll: false });

  return (
    <div className="grid gap-4">
      {!container && (
        <div className="flex flex-wrap items-center gap-3">
          <Segmented
            label={t.view}
            value={view}
            options={[
              { value: "list", label: t.list },
              { value: "graph", label: t.graph },
            ]}
            onChange={(v) => go(v as DependencyView, depth)}
          />
          {view === "graph" && (
            <Segmented
              label={t.depth}
              value={String(depth)}
              options={["1", "2", "3"].map((d) => ({ value: d, label: `${t.depth} ${d}` }))}
              onChange={(d) => go("graph", Number(d))}
            />
          )}
        </div>
      )}
      {view === "graph" ? (
        <DependencyGraph node={node} depth={container ? 1 : depth} locale={locale} labels={labels} fail={fail} />
      ) : (
        <DependencyLists node={node} locale={locale} labels={labels} canWrite={canWrite} fail={fail} />
      )}
    </div>
  );
}

function DependencyLists({ node, locale, labels, canWrite, fail }: Props) {
  const t = labels.dependencies;
  const { common } = useUiText();
  const [data, setData] = useState<Dependencies | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const abort = new AbortController();
    getDependencies(node.id, abort.signal)
      .then(setData)
      .catch((e: unknown) => {
        if (!abort.signal.aborted) setError(fail(e));
      });
    return () => abort.abort();
  }, [node.id, fail]);

  if (error) return <Message note={{ kind: "error", text: error }} />;
  if (!data) return <SkeletonRows rows={4} label={common.loading} />;
  const hint = (
    <span className="flex flex-wrap justify-center gap-3">
      <Link className="text-signal hover:underline" href={catalogHref(locale, node.id, "settings", null, null, "connect")}>
        {t.howToEvents}
      </Link>
      {canWrite && (
        <Link className="text-signal hover:underline" href={catalogHref(locale, node.id, "settings", null, null, "dependencies")}>
          {t.howToManual}
        </Link>
      )}
    </span>
  );

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Panel title={t.dependsOn}>
        {data.depends_on.length === 0 ? (
          <EmptyState icon={Network} title={t.emptyDependsOn} text={t.howTo} action={hint} />
        ) : (
          <ul className="grid gap-2">
            {data.depends_on.map((item) => (
              <li key={targetKey(item.target) + (item.target.type === "hidden" ? item.sources[0]?.origin : "")} className="flex flex-wrap items-center justify-between gap-2">
                <TargetLabel item={item} locale={locale} labels={labels} />
                <SourceBadges sources={item.sources} locale={locale} labels={labels} />
              </li>
            ))}
          </ul>
        )}
      </Panel>
      <Panel title={t.dependents}>
        {data.dependents.length === 0 && data.hidden_dependents === 0 ? (
          <EmptyState icon={Network} title={t.emptyDependents} />
        ) : (
          <ul className="grid gap-2">
            {data.dependents.map((d) => (
              <li key={d.project.id} className="flex flex-wrap items-center justify-between gap-2">
                <Link href={catalogHref(locale, d.project.id, "dependencies")} className="inline-flex items-center gap-2 text-ink hover:underline">
                  <KindIcon kind="project" />
                  <span className="break-all">{d.project.path}</span>
                </Link>
                <SourceBadges sources={d.sources} locale={locale} labels={labels} />
              </li>
            ))}
            {data.hidden_dependents > 0 && <li className="text-sm text-muted">{format(t.moreHidden, { count: data.hidden_dependents })}</li>}
          </ul>
        )}
      </Panel>
    </div>
  );
}

function TargetLabel({ item, locale, labels }: { item: DependencyItem; locale: Locale; labels: CatalogLabels }) {
  const t = labels.dependencies;
  const target = item.target;
  switch (target.type) {
    case "project":
      return (
        <Link href={catalogHref(locale, target.id, "dependencies")} className="inline-flex items-center gap-2 text-ink hover:underline">
          <KindIcon kind="project" />
          <span className="break-all">{target.path}</span>
        </Link>
      );
    case "external":
      return (
        <span className="inline-flex items-center gap-2">
          <span className="font-medium text-ink">{target.name}</span>
          <span className="text-xs text-muted">{t.external}</span>
        </span>
      );
    case "unresolved":
      return (
        <span className="inline-flex items-center gap-2">
          <span className="break-all text-ink-2">{target.ref}</span>
          <span className="text-xs text-amber">{t.unresolved}</span>
        </span>
      );
  }
  return <span className="text-muted">{t.hidden}</span>;
}
