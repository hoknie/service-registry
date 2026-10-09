"use client";

import { GitBranch, Link2, Rocket } from "lucide-react";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { ago } from "@/i18n/time";
import { apiGet, errorCode, type Branch, type Items, type Page, type ProjectLink, type ServiceEnvironment } from "@/lib/api";

import { statusTone } from "../links/shared";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { StatCard } from "../ui/StatCard";
import { catalogHref, type CatalogLabels } from "./shared";

type Load<T> = { data: T | null; error: string | null };
const ROWS = 4;

function useLoad<T>(path: string, fetcher: (path: string) => Promise<T>): Load<T> {
  const { errors } = useUiText();
  const [state, setState] = useState<Load<T> & { path: string | null }>({ data: null, error: null, path: null });
  useEffect(() => {
    let alive = true;
    fetcher(path).then(
      (data) => alive && setState({ data, error: null, path }),
      (e: unknown) => alive && setState({ data: null, error: errorText(errors, errorCode(e)), path }),
    );
    return () => {
      alive = false;
    };
  }, [path, fetcher, errors]);
  return state.path === path ? state : { data: null, error: null };
}

const getEnvironments = (path: string) => apiGet<Items<ServiceEnvironment>>(path as `/${string}`).then((r) => r.items.filter((e) => e.version));
const getBranches = (path: string) =>
  Promise.all([
    apiGet<Page<Branch>>(`${path}?state=active&limit=1` as `/${string}`),
    apiGet<Page<Branch>>(`${path}?state=stale&limit=1` as `/${string}`),
  ]).then(([active, stale]) => ({ active: active.total, stale: stale.total }));
const getLinks = (path: string) =>
  apiGet<Items<ProjectLink>>(path as `/${string}`).then((r) => ({
    total: r.items.length,
    failing: r.items.filter((l) => l.check && statusTone(l.check.status) === "danger").length,
  }));

export function ProjectSummary({ projectId, locale, labels }: { projectId: string; locale: Locale; labels: CatalogLabels }) {
  const t = labels.summary;
  const base = `/v1/catalog/nodes/${projectId}`;
  const envs = useLoad(`${base}/environments`, getEnvironments);
  const branches = useLoad(`${base}/branches`, getBranches);
  const links = useLoad(`${base}/links`, getLinks);

  return (
    <div className="grid gap-4 md:grid-cols-3">
      <StatCard
        icon={Rocket}
        tone="project"
        title={t.environments}
        href={catalogHref(locale, projectId, "deployments")}
        open={t.open}
        loading={!envs.data && !envs.error}
        error={envs.error}
        empty={envs.data && envs.data.length === 0 ? t.empty : undefined}
      >
        <ul className="grid gap-1.5">
          {(envs.data ?? []).slice(0, ROWS).map((e) => (
            <li key={`${e.service}:${e.environment}`} className="flex min-w-0 items-center gap-2 text-sm">
              <Badge tone="signal" dot>
                {e.environment}
              </Badge>
              <code className="min-w-0 truncate font-medium text-ink">{e.version}</code>
              <span className="ml-auto shrink-0 text-xs text-muted">{ago(e.occurred_at ?? e.updated_at ?? "", locale, "")}</span>
            </li>
          ))}
        </ul>
        {envs.data && envs.data.length > ROWS && <p className="mt-2 text-xs text-muted">{format(t.more, { n: envs.data.length - ROWS })}</p>}
      </StatCard>
      <StatCard
        icon={GitBranch}
        tone="cobalt"
        title={t.branches}
        href={catalogHref(locale, projectId, "branches")}
        open={t.open}
        loading={!branches.data && !branches.error}
        error={branches.error}
        empty={branches.data && branches.data.active + branches.data.stale === 0 ? t.empty : undefined}
        value={branches.data?.active}
        caption={branches.data ? format(t.stale, { n: branches.data.stale }) : undefined}
      />
      <StatCard
        icon={Link2}
        tone="amber"
        title={t.links}
        href={catalogHref(locale, projectId, "links")}
        open={t.open}
        loading={!links.data && !links.error}
        error={links.error}
        empty={links.data && links.data.total === 0 ? t.empty : undefined}
        value={links.data?.total}
        caption={links.data ? format(t.failing, { n: links.data.failing }) : undefined}
      />
    </div>
  );
}
