"use client";

import { ExternalLink, Link2, RefreshCw } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { ago, when } from "@/i18n/time";
import { apiGet, apiSend, errorCode, type Items, type LinkKind, type ProjectLink, type ServiceEnvironment } from "@/lib/api";

import type { CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Field } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { LinkIcon } from "./LinkIcon";
import { kindName, linkQuery, statusTone } from "./shared";

type Props = {
  projectId: string;
  branch: string | null;
  kinds: LinkKind[];
  locale: Locale;
  labels: CatalogLabels;
};

const sameLink = (a: ProjectLink, b: ProjectLink) =>
  a.link_key === b.link_key && a.service === b.service && a.environment === b.environment;

export function LinksPanel({ projectId, branch, kinds, locale, labels }: Props) {
  const { errors, common } = useUiText();
  const t = labels.links;
  const [environment, setEnvironment] = useState("");
  const [environments, setEnvironments] = useState<string[]>([]);
  const [items, setItems] = useState<ProjectLink[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [checking, setChecking] = useState<string | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const base = `/v1/catalog/nodes/${projectId}/links`;

  const load = useCallback(async () => {
    try {
      setItems((await apiGet<Items<ProjectLink>>(`${base}${linkQuery(branch, environment)}` as `/${string}`)).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [base, branch, environment, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  useEffect(() => {
    apiGet<Items<ServiceEnvironment>>(`/v1/catalog/nodes/${projectId}/environments`)
      .then((r) => setEnvironments([...new Set(r.items.map((e) => e.environment))].sort()))
      .catch(() => setEnvironments([]));
  }, [projectId]);

  const check = async (link: ProjectLink) => {
    const id = `${link.link_key}/${link.service}/${link.environment}`;
    setChecking(id);
    try {
      const path = `${base}/${encodeURIComponent(link.link_key)}/check${linkQuery(branch, environment)}` as `/${string}`;
      const fresh = (await apiSend<Items<ProjectLink>>("POST", path)).items;
      setItems((prev) => prev?.map((l) => fresh.find((f) => sameLink(f, l)) ?? l) ?? prev);
    } catch (e) {
      toast.error(fail(e));
    } finally {
      setChecking(null);
    }
  };

  return (
    <Panel
      title={t.title}
      description={t.lead}
      actions={
        environments.length > 0 && (
          <Field label={t.environment} className="w-48">
            {(p) => (
              <Select {...p} value={environment} onChange={(e) => setEnvironment(e.target.value)}>
                <option value="">{t.allEnvironments}</option>
                {environments.map((env) => (
                  <option key={env} value={env}>
                    {env}
                  </option>
                ))}
              </Select>
            )}
          </Field>
        )
      }
    >
      {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}
      {!items && !error && <SkeletonTable rows={3} label={common.loading} />}
      {items && items.length === 0 && <EmptyState icon={Link2} title={t.empty} text={t.emptyText} />}
      {items && items.length > 0 && (
        <ul className="grid divide-y divide-line">
          {items.map((l) => {
            const id = `${l.link_key}/${l.service}/${l.environment}`;
            const kind = kinds.find((k) => k.key === l.kind_key);
            const name = kindName(kinds, l.kind_key, locale);
            return (
              <li key={id} className="flex flex-wrap items-center gap-x-3 gap-y-1.5 py-2.5">
                <LinkIcon icon={kind?.icon} className={l.url ? "text-ink-2" : undefined} />
                <span className="min-w-0 flex-1">
                  {l.url ? (
                    <a
                      href={l.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      title={t.open}
                      className="inline-flex max-w-full items-center gap-1.5 font-medium text-signal hover:underline"
                    >
                      <span className="truncate">{name}</span>
                      <ExternalLink aria-hidden="true" className="size-3.5 shrink-0" />
                    </a>
                  ) : (
                    <span className="font-medium text-muted" aria-disabled="true">
                      {name}
                    </span>
                  )}
                  <span className="ml-2 inline-flex flex-wrap gap-1 align-middle">
                    {l.link_key !== l.kind_key && <code className="text-xs text-ink-2">{l.link_key}</code>}
                    {l.environment && <Badge tone="cobalt">{l.environment}</Badge>}
                    {l.service && (
                      <Badge tone="outline" title={t.service}>
                        {l.service}
                      </Badge>
                    )}
                  </span>
                  {l.url ? (
                    <span className="block truncate font-mono text-xs text-muted">{l.url}</span>
                  ) : (
                    <span className="block text-xs text-muted">{format(t.missing, { vars: l.missing.join(", ") })}</span>
                  )}
                </span>
                {l.url && (
                  <span className="flex items-center gap-2">
                    {l.check ? (
                      <span className="flex items-center gap-1.5 text-xs text-muted" title={when(l.check.checked_at, locale, "")}>
                        <Badge tone={statusTone(l.check.status)} dot>
                          {t.statuses[l.check.status]}
                          {l.check.http_status !== null && <span className="font-mono">{l.check.http_status}</span>}
                        </Badge>
                        {ago(l.check.checked_at, locale, "")}
                      </span>
                    ) : (
                      <span className="text-xs text-muted">{t.notChecked}</span>
                    )}
                    <Button size="sm" variant="ghost" disabled={checking === id} onClick={() => void check(l)}>
                      <RefreshCw aria-hidden="true" className={checking === id ? "animate-spin" : undefined} />
                      {t.check}
                    </Button>
                  </span>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </Panel>
  );
}
