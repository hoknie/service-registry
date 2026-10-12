"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { ago, when } from "@/i18n/time";
import { apiGet, errorCode, type CatalogNode, type Items, type LinkTemplate, type ProjectLink } from "@/lib/api";
import { useLinkKinds } from "@/lib/linkKinds";

import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { CopyButton } from "../ui/CopyButton";
import { Message } from "../ui/Message";
import { Skeleton } from "../ui/Skeleton";
import { Glyph } from "./LinkIcons";
import { kindName, linkQuery, statusTone } from "./shared";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; branch: string | null; canWrite: boolean };

function useItems<T>(path: `/${string}` | null) {
  const { errors } = useUiText();
  const [state, setState] = useState<{ path: string; items: T[] | null; error: string | null } | null>(null);
  useEffect(() => {
    if (!path) return;
    let live = true;
    apiGet<Items<T>>(path)
      .then((r) => live && setState({ path, items: r.items, error: null }))
      .catch((e) => live && setState({ path, items: null, error: errorText(errors, errorCode(e)) }));
    return () => {
      live = false;
    };
  }, [path, errors]);
  return state?.path === path ? state : null;
}

function Source({ node, nodeId, locale, here }: { node: CatalogNode; nodeId: string; locale: Locale; here: string }) {
  if (nodeId === node.id) return <>{here}</>;
  const name = (node.path ?? []).find((p) => p.id === nodeId)?.name ?? nodeId;
  return (
    <Link href={catalogHref(locale, nodeId, "details")} className="text-signal hover:underline">
      {name}
    </Link>
  );
}

function Empty({ node, locale, text, labels, canWrite }: { node: CatalogNode; locale: Locale; text: string; labels: CatalogLabels; canWrite: boolean }) {
  return (
    <p className="text-sm text-muted">
      {text}{" "}
      {canWrite && (
        <Link href={catalogHref(locale, node.id, "settings", null, null, "links")} className="text-signal hover:underline">
          {labels.detailsLinks.settings}
        </Link>
      )}
    </p>
  );
}

export function LinksDetails({ node, locale, labels, branch, canWrite }: Props) {
  const project = node.kind === "project";
  const base = `/v1/catalog/nodes/${node.id}`;
  const linksState = useItems<ProjectLink>(project ? (`${base}/links${linkQuery(branch, "")}` as `/${string}`) : null);
  const templatesState = useItems<LinkTemplate>(project ? null : (`${base}/link-templates` as `/${string}`));
  const state = project ? linksState : templatesState;
  const kinds = useLinkKinds();
  const d = labels.detailsLinks;
  const t = labels.links;

  return (
    <section aria-labelledby={`links-${node.id}`} className="grid gap-3">
      <h3 id={`links-${node.id}`} className="text-sm font-semibold text-ink">
        {d.title}
      </h3>
      {!state && <Skeleton className="h-16" />}
      {state?.error && <Message note={{ kind: "error", text: state.error }} />}
      {project && linksState?.items && linksState.items.length === 0 && (
        <Empty node={node} locale={locale} text={d.empty} labels={labels} canWrite={canWrite} />
      )}
      {!project && templatesState?.items && templatesState.items.filter((x) => !x.disabled).length === 0 && (
        <Empty node={node} locale={locale} text={d.emptyTemplates} labels={labels} canWrite={canWrite} />
      )}
      {project && linksState?.items && linksState.items.length > 0 && (
        <ul className="grid divide-y divide-line">
          {linksState.items.map((l) => (
            <li key={`${l.link_key}/${l.service}/${l.environment}`} className="grid gap-1.5 py-3">
              <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                <Glyph icon={l.icon} className={l.url ? undefined : "opacity-45 grayscale"} />
                <span className="font-medium text-ink">{l.title ?? kindName(kinds, l.kind_key, locale)}</span>
                <span className="text-xs text-muted">
                  {d.kind}: {kindName(kinds, l.kind_key, locale)}
                </span>
                {l.environment && <Badge tone="cobalt">{l.environment}</Badge>}
                {l.service && (
                  <Badge tone="outline" title={t.service}>
                    {l.service}
                  </Badge>
                )}
                {l.source === "event" ? (
                  <Badge tone="signal">{t.fromEvent}</Badge>
                ) : (
                  <span className="text-xs text-muted">
                    {d.source}: <Source node={node} nodeId={l.node_id} locale={locale} here={d.here} />
                  </span>
                )}
                {l.check && (
                  <span className="flex items-center gap-1.5 text-xs text-muted" title={when(l.check.checked_at, locale, "")}>
                    <Badge tone={statusTone(l.check.status)} dot>
                      {t.statuses[l.check.status]}
                      {l.check.http_status !== null && <span className="font-mono">{l.check.http_status}</span>}
                    </Badge>
                    {ago(l.check.checked_at, locale, "")}
                  </span>
                )}
              </div>
              {l.url ? (
                <div className="flex items-start gap-2">
                  <a
                    href={l.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="min-w-0 flex-1 font-mono text-xs break-all text-signal hover:underline"
                  >
                    {l.url}
                  </a>
                  <CopyButton text={l.url} label={d.copy} variant="ghost" />
                </div>
              ) : (
                <p className="text-xs text-muted">{format(t.missing, { vars: l.missing.join(", ") })}</p>
              )}
            </li>
          ))}
        </ul>
      )}
      {!project && templatesState?.items && templatesState.items.some((x) => !x.disabled) && (
        <ul className="grid divide-y divide-line">
          {templatesState.items
            .filter((x) => !x.disabled)
            .map((tpl) => (
              <li key={tpl.link_key} className="grid gap-1.5 py-3">
                <div className="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <Glyph icon={tpl.icon} />
                  <span className="font-medium text-ink">{tpl.title ?? kindName(kinds, tpl.kind_key, locale)}</span>
                  <span className="text-xs text-muted">
                    {d.kind}: {kindName(kinds, tpl.kind_key, locale)}
                  </span>
                  <code className="text-xs text-ink-2">{tpl.link_key}</code>
                  <span className="text-xs text-muted">
                    {d.source}: <Source node={node} nodeId={tpl.node_id} locale={locale} here={d.here} />
                  </span>
                </div>
                <div className="flex items-start gap-2">
                  <code className="min-w-0 flex-1 font-mono text-xs break-all text-ink-2" title={d.template}>
                    {tpl.template}
                  </code>
                  <CopyButton text={tpl.template ?? ""} label={d.copyTemplate} variant="ghost" />
                </div>
              </li>
            ))}
        </ul>
      )}
    </section>
  );
}
