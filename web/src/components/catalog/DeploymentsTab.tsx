"use client";

import { Boxes, ExternalLink, GitBranch, GitCommitHorizontal, Plug, Rocket, RotateCcw, TriangleAlert, UserRound } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { ago, when } from "@/i18n/time";
import {
  apiGet,
  errorCode,
  type Deployment,
  type DirectoryEnvironment,
  type Items,
  type Observation,
  type Page,
  type ServiceEnvironment,
} from "@/lib/api";
import { cn } from "@/lib/cn";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { Pager } from "../ui/Pager";
import { SkeletonCards } from "../ui/Skeleton";
import type { CatalogLabels } from "./shared";

const PAGE = 50;
const FILTERS = ["service", "environment", "branch"] as const;
type Filters = Record<(typeof FILTERS)[number], string>;
const NO_FILTERS: Filters = { service: "", environment: "", branch: "" };

type Props = {
  projectId: string;
  branch?: string | null;
  locale: Locale;
  labels: CatalogLabels;
  onConnect: () => void;
};

const short = (sha: string | null) => (sha ? sha.slice(0, 7) : "—");

function Meta({ icon: Icon, children, title }: { icon: typeof GitBranch; children: ReactNode; title: string }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5" title={title}>
      <Icon aria-hidden="true" className="size-3.5 shrink-0 text-muted" />
      <span className="sr-only">{title}: </span>
      <span className="truncate">{children}</span>
    </span>
  );
}

type DeploymentLabels = CatalogLabels["deployments"];

const rolloutTone = { complete: "signal", progressing: "cobalt", stalled: "danger" } as const;
const runTone = { succeeded: "signal", failed: "danger", active: "cobalt" } as const;

function ObservedRow({ o, locale, t }: { o: Observation; locale: Locale; t: DeploymentLabels }) {
  return (
    <li className={cn("grid gap-1 px-4 py-2 text-xs text-ink-2", o.gone && "opacity-60")}>
      <span className="flex flex-wrap items-center gap-x-2 gap-y-1">
        <Boxes aria-hidden="true" className="size-3.5 shrink-0 text-muted" />
        <span className="min-w-0 truncate font-mono text-ink" title={`${o.cluster} / ${o.namespace} / ${o.kind} ${o.workload}`}>
          {o.cluster}/{o.namespace}/{o.workload}
        </span>
        {o.version && <code className="rounded bg-surface-3 px-1 text-ink">{o.version}</code>}
        {o.gone && <Badge tone="outline">{t.gone}</Badge>}
      </span>
      <span className="flex flex-wrap items-center gap-x-3 gap-y-1 pl-5">
        {o.replicas && (
          <span>
            {t.ready}: <span className="tabular-nums text-ink">{`${o.replicas.ready}/${o.replicas.desired}`}</span>
          </span>
        )}
        {o.rollout && <Badge tone={rolloutTone[o.rollout]}>{t.rollout[o.rollout]}</Badge>}
        {o.restarts !== undefined && o.restarts > 0 && (
          <span>
            {t.restarts}: <span className="tabular-nums text-ink">{o.restarts}</span>
          </span>
        )}
        {o.kind === "CronJob" && (
          <>
            {o.schedule && (
              <span title={t.schedule}>
                <code>{o.schedule}</code>
              </span>
            )}
            {o.suspended && <Badge tone="outline">{t.suspended}</Badge>}
            {o.last_run ? (
              <span className="inline-flex items-center gap-1">
                {t.lastRun}: <Badge tone={runTone[o.last_run.status]}>{t.runs[o.last_run.status]}</Badge>
                {o.last_run.started_at && <span title={when(o.last_run.started_at, locale, "")}>{ago(o.last_run.started_at, locale, "")}</span>}
              </span>
            ) : (
              <span>{t.neverRan}</span>
            )}
          </>
        )}
        <time dateTime={o.observed_at} title={`${t.observedAt}: ${when(o.observed_at, locale, "")}`} className="ml-auto text-muted">
          {ago(o.observed_at, locale, "")}
        </time>
      </span>
    </li>
  );
}

export function DeploymentsTab({ projectId, branch, locale, labels, onConnect }: Props) {
  const { errors, common } = useUiText();
  const t = labels.deployments;
  const [current, setCurrent] = useState<ServiceEnvironment[] | null>(null);
  const [directory, setDirectory] = useState<DirectoryEnvironment[]>([]);
  const [history, setHistory] = useState<Page<Deployment> | null>(null);
  const initial: Filters = { ...NO_FILTERS, branch: branch ?? "" };
  const [draft, setDraft] = useState<Filters>(initial);
  const [filters, setFilters] = useState<Filters>(initial);
  const [offset, setOffset] = useState(0);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const q = new URLSearchParams();
    for (const f of FILTERS) if (filters[f].trim()) q.set(f, filters[f].trim());
    q.set("limit", String(PAGE));
    q.set("offset", String(offset));
    try {
      const [envs, page] = await Promise.all([
        apiGet<Items<ServiceEnvironment>>(`/v1/catalog/nodes/${projectId}/environments`),
        apiGet<Page<Deployment>>(`/v1/catalog/nodes/${projectId}/deployments?${q.toString()}`),
      ]);
      setCurrent(envs.items);
      setHistory(page);
      setError(null);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  }, [projectId, filters, offset, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  useEffect(() => {
    apiGet<Items<DirectoryEnvironment>>("/v1/environments")
      .then((r) => setDirectory(r.items))
      .catch(() => setDirectory([]));
  }, []);

  const envName = (key: string) => directory.find((e) => e.key === key)?.names[locale] ?? key;

  const apply = (e: FormEvent) => {
    e.preventDefault();
    setOffset(0);
    setFilters(draft);
  };

  const reset = () => {
    setDraft(NO_FILTERS);
    setFilters(NO_FILTERS);
    setOffset(0);
  };

  if (error && !current) return <Message note={{ kind: "error", text: error }} />;
  if (!current || !history) return <SkeletonCards count={3} label={common.loading} />;

  const filtered = FILTERS.some((f) => filters[f].trim());

  if (current.length === 0 && history.items.length === 0 && !filtered) {
    return (
      <EmptyState
        icon={Rocket}
        title={t.empty}
        text={t.emptyText}
        action={
          <Button onClick={onConnect}>
            <Plug aria-hidden="true" />
            {t.connect}
          </Button>
        }
      />
    );
  }

  return (
    <div className="grid animate-rise-in gap-8">
      {error && <Message note={{ kind: "error", text: error }} />}

      <section aria-labelledby="current-title" className="grid gap-3">
        <h2 id="current-title">{t.current}</h2>
        {current.length === 0 ? (
          <p className="text-sm text-muted">{t.empty}</p>
        ) : (
          <ul className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {current.map((c) => (
              <li key={`${c.service}/${c.environment}`} className="flex flex-col rounded-lg border border-line bg-surface">
                <div className="flex items-center justify-between gap-3 border-b border-line px-4 py-2.5">
                  <span className="flex min-w-0 items-center gap-2 text-sm font-medium text-ink">
                    <span
                      aria-hidden="true"
                      className={cn(
                        "size-2 shrink-0 rounded-full",
                        c.drift ? "bg-amber shadow-[0_0_0_3px_var(--amber-soft)]" : "bg-signal shadow-[0_0_0_3px_var(--signal-soft)]",
                      )}
                    />
                    <span className="truncate" title={c.environment}>
                      {envName(c.environment)}
                    </span>
                  </span>
                  {c.occurred_at && (
                    <time dateTime={c.occurred_at} title={when(c.occurred_at, locale, "")} className="shrink-0 text-xs text-muted">
                      {ago(c.occurred_at, locale, "—")}
                    </time>
                  )}
                </div>
                <div className="flex flex-1 flex-col gap-3 px-4 py-3">
                  <div className="min-w-0">
                    <p className="truncate text-xs text-muted">
                      <span className="sr-only">{t.service}: </span>
                      {c.service}
                    </p>
                    {c.version ? (
                      <p className="truncate font-mono text-xl font-medium text-ink" title={c.version}>
                        <span className="sr-only">{t.version}: </span>
                        {c.version}
                      </p>
                    ) : (
                      <p className="text-sm text-muted">{t.noCiDeployment}</p>
                    )}
                    {(c.drift || c.source === "cluster") && (
                      <span className="mt-1.5 flex flex-wrap gap-1">
                        {c.drift && (
                          <Badge tone="amber" title={t.driftHint}>
                            <TriangleAlert aria-hidden="true" className="size-3" />
                            {t.drift}
                          </Badge>
                        )}
                        {c.source === "cluster" && <Badge tone="cobalt">{t.fromCluster}</Badge>}
                      </span>
                    )}
                  </div>
                  <div className="grid gap-1 text-xs text-ink-2">
                    {c.branch && (
                      <Meta icon={GitBranch} title={t.branch}>
                        <code>{c.branch}</code>
                      </Meta>
                    )}
                    {c.version && (
                      <Meta icon={GitCommitHorizontal} title={t.commit}>
                        <code>{short(c.commit_sha)}</code>
                      </Meta>
                    )}
                    {c.deployed_by && (
                      <Meta icon={UserRound} title={t.deployedBy}>
                        {c.deployed_by}
                      </Meta>
                    )}
                  </div>
                  {c.url && (
                    <a
                      href={c.url}
                      rel="noreferrer noopener"
                      target="_blank"
                      className="mt-auto inline-flex items-center gap-1.5 self-start text-sm font-medium text-signal hover:underline"
                    >
                      {t.link}
                      <ExternalLink aria-hidden="true" className="size-3.5" />
                    </a>
                  )}
                </div>
                {c.observed.length > 0 && (
                  <div className="border-t border-line">
                    <p className="px-4 pt-2 text-xs font-medium text-muted">{t.observed}</p>
                    <ul aria-label={t.observed} className="grid divide-y divide-line">
                      {c.observed.map((o) => (
                        <ObservedRow key={`${o.cluster}/${o.namespace}/${o.kind}/${o.workload}`} o={o} locale={locale} t={t} />
                      ))}
                    </ul>
                  </div>
                )}
              </li>
            ))}
          </ul>
        )}
      </section>

      <section aria-labelledby="history-title" className="grid gap-4">
        <h2 id="history-title">{t.history}</h2>
        <form className="grid gap-3 rounded-lg border border-line bg-surface p-3 sm:grid-cols-[repeat(3,minmax(0,1fr))_auto] sm:items-end" onSubmit={apply}>
          {FILTERS.map((f) => (
            <Field key={f} label={t[f]}>
              {(p) => <Input {...p} value={draft[f]} placeholder={t.any} onChange={(e) => setDraft({ ...draft, [f]: e.target.value })} />}
            </Field>
          ))}
          <div className="flex gap-2">
            <Button type="submit" variant="primary">
              {t.filter}
            </Button>
            {filtered && (
              <Button variant="ghost" onClick={reset}>
                <RotateCcw aria-hidden="true" />
                {t.reset}
              </Button>
            )}
          </div>
        </form>

        {history.items.length === 0 ? (
          <p role="status" className="rounded-lg border border-dashed border-line-strong px-4 py-6 text-center text-sm text-muted">
            {filtered ? t.noMatch : t.empty}
          </p>
        ) : (
          <ol className="rail grid gap-1">
            {history.items.map((d) => (
              <li key={d.id} className="relative grid grid-cols-[1.5rem_minmax(0,1fr)] gap-3">
                <span aria-hidden="true" className="relative z-10 mt-3.5 grid size-6 place-items-center">
                  <span
                    className={cn(
                      "size-3 rounded-full border-2",
                      d.current ? "border-signal bg-signal shadow-[0_0_0_4px_var(--signal-soft)]" : "border-line-strong bg-canvas",
                    )}
                  />
                </span>
                <div className="rounded-lg px-3 py-2.5 transition-colors hover:bg-surface">
                  <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    <code className="text-sm font-medium text-ink">{d.version}</code>
                    <span className="text-sm text-ink-2">
                      <span className="sr-only">{t.service}: </span>
                      {d.service}
                    </span>
                    <Badge tone="outline" title={d.environment}>
                      <span className="sr-only">{t.environment}: </span>
                      {envName(d.environment)}
                    </Badge>
                    {d.source === "cluster" && <Badge tone="cobalt">{t.fromCluster}</Badge>}
                    {d.current && (
                      <Badge tone="signal" dot>
                        {t.isCurrent}
                      </Badge>
                    )}
                    <time dateTime={d.occurred_at} title={when(d.occurred_at, locale, "")} className="ml-auto text-xs text-muted tabular-nums">
                      {when(d.occurred_at, locale, "—")}
                    </time>
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-ink-2">
                    {d.branch && (
                      <Meta icon={GitBranch} title={t.branch}>
                        <code>{d.branch}</code>
                      </Meta>
                    )}
                    <Meta icon={GitCommitHorizontal} title={t.commit}>
                      <code>{short(d.commit_sha)}</code>
                    </Meta>
                    {d.deployed_by && (
                      <Meta icon={UserRound} title={t.deployedBy}>
                        {d.deployed_by}
                      </Meta>
                    )}
                    {d.url && (
                      <a href={d.url} rel="noreferrer noopener" target="_blank" className="inline-flex items-center gap-1 text-signal hover:underline">
                        {t.link}
                        <ExternalLink aria-hidden="true" className="size-3" />
                      </a>
                    )}
                  </div>
                </div>
              </li>
            ))}
          </ol>
        )}
        <Pager offset={offset} shown={history.items.length} total={history.total} size={PAGE} labels={labels.pager} onChange={setOffset} />
      </section>
    </div>
  );
}
