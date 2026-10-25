"use client";

import { Boxes, ExternalLink, GitBranch, GitCommitHorizontal, TriangleAlert, UserRound } from "lucide-react";
import type { ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import { ago, when } from "@/i18n/time";
import type { Observation, ServiceEnvironment } from "@/lib/api";
import { cn } from "@/lib/cn";

import { Badge } from "../ui/Badge";
import type { CatalogLabels } from "./shared";

export const short = (sha: string | null) => (sha ? sha.slice(0, 7) : "—");

export function Meta({ icon: Icon, children, title }: { icon: typeof GitBranch; children: ReactNode; title: string }) {
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5" title={title}>
      <Icon aria-hidden="true" className="size-3.5 shrink-0 text-muted" />
      <span className="sr-only">{title}: </span>
      <span className="truncate">{children}</span>
    </span>
  );
}

export type DeploymentLabels = CatalogLabels["deployments"];

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

function latest(items: ServiceEnvironment[]) {
  return items.reduce<string | null>((max, c) => (c.occurred_at && (!max || c.occurred_at > max) ? c.occurred_at : max), null);
}

function ServiceBlock({ c, single, locale, t }: { c: ServiceEnvironment; single: boolean; locale: Locale; t: DeploymentLabels }) {
  return (
    <div className="flex flex-1 flex-col">
      <div className="flex flex-1 flex-col gap-3 px-4 py-3">
        <div className="min-w-0">
          <p className="flex items-start gap-2 text-xs text-muted">
            <span className="min-w-0 break-words" title={c.service}>
              <span className="sr-only">{t.service}: </span>
              {c.service}
            </span>
            {!single && c.occurred_at && (
              <time dateTime={c.occurred_at} title={when(c.occurred_at, locale, "")} className="ml-auto shrink-0">
                {ago(c.occurred_at, locale, "—")}
              </time>
            )}
          </p>
          {c.version ? (
            <p className={cn("truncate font-mono font-medium text-ink", single ? "text-xl" : "text-lg")} title={c.version}>
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
        <div className={cn("grid gap-1 text-xs text-ink-2", !single && "sm:flex sm:flex-wrap sm:gap-x-4")}>
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
    </div>
  );
}

type CardProps = { name: string; environment: string; items: ServiceEnvironment[]; locale: Locale; t: DeploymentLabels };

export function EnvironmentCard({ name, environment, items, locale, t }: CardProps) {
  const single = items.length === 1;
  const at = latest(items);
  return (
    <li className="flex flex-col rounded-lg border border-line bg-surface">
      <div className="flex items-center justify-between gap-3 border-b border-line px-4 py-2.5">
        <span className="flex min-w-0 items-center gap-2 text-sm font-medium text-ink">
          <span
            aria-hidden="true"
            className={cn(
              "size-2 shrink-0 rounded-full",
              items.some((c) => c.drift) ? "bg-amber shadow-[0_0_0_3px_var(--amber-soft)]" : "bg-signal shadow-[0_0_0_3px_var(--signal-soft)]",
            )}
          />
          <span className="truncate" title={environment}>
            {name}
          </span>
        </span>
        {at && (
          <time dateTime={at} title={when(at, locale, "")} className="shrink-0 text-xs text-muted">
            {ago(at, locale, "—")}
          </time>
        )}
      </div>
      <div className="flex flex-1 flex-col divide-y divide-line">
        {items.map((c) => (
          <ServiceBlock key={c.service} c={c} single={single} locale={locale} t={t} />
        ))}
      </div>
    </li>
  );
}
