"use client";

import { PowerOff, type LucideIcon } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { when } from "@/i18n/time";
import type { RefreshInterval } from "@/lib/autoRefresh";
import { formatUptime, METRICS_REFRESH_KEY, uptimeSeconds, type MetricsView } from "@/lib/metrics";
import { metricsAllowed } from "@/lib/rights";
import { usePolledGet, useRefreshInterval } from "@/lib/useAutoRefresh";

import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { AutoRefresh } from "../ui/AutoRefresh";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";

export type MetricsLabels = Messages["admin"]["metrics"];
export type Live = { view: MetricsView; interval: RefreshInterval; tick: number; seenAt: number };

type Props = { locale: Locale; t: MetricsLabels; title: string; lead: string; glyph: LucideIcon; children: (live: Live) => ReactNode };

export function MetricsShell({ locale, t, title, lead, glyph, children }: Props) {
  const { session } = useSession();
  const user = session.status === "signed-in" ? session.user : null;
  if (!user || !metricsAllowed(user)) return null;
  return (
    <div className="grid gap-6">
      <PageHeader title={title} lead={lead} glyph={glyph} />
      {user.features?.metrics ? (
        <Polled locale={locale} t={t}>
          {children}
        </Polled>
      ) : (
        <EmptyState icon={PowerOff} title={t.disabled.title} text={t.disabled.text} className="mx-auto max-w-lg" />
      )}
    </div>
  );
}

type Seen = { instance: string | null; changed: string | null; at: number };

function Polled({ locale, t, children }: { locale: Locale; t: MetricsLabels; children: (live: Live) => ReactNode }) {
  const { errors, common } = useUiText();
  const [interval, setIntervalSecs] = useRefreshInterval(METRICS_REFRESH_KEY, 10);
  const { page, failure, refreshing, refresh } = usePolledGet<MetricsView>("/v1/system/metrics", interval, errors);
  const [seen, setSeen] = useState<Seen>({ instance: null, changed: null, at: 0 });
  const [tick, setTick] = useState(0);
  const view = page?.data ?? null;

  useEffect(() => {
    if (!view) return;
    const now = Date.now();
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSeen((s) => ({
      instance: view.instance,
      changed: s.instance !== null && s.instance !== view.instance ? view.instance : s.changed,
      at: now,
    }));
  }, [view]);

  const uptime = view && seen.at > 0 ? uptimeSeconds(view.started_at, seen.at) : null;
  return (
    <>
      <section aria-label={t.about} className="flex flex-wrap items-start justify-between gap-4 rounded-card border border-line bg-surface p-4 shadow-elev-1">
        {view ? (
          <dl className="grid min-w-0 flex-1 grid-cols-[repeat(auto-fit,minmax(10rem,1fr))] gap-x-6 gap-y-2 text-sm">
            <Meta label={t.instance} value={view.instance || "—"} />
            <Meta label={t.version} value={view.version || "—"} />
            <Meta label={t.goVersion} value={view.go_version || "—"} />
            <Meta label={t.uptime} value={uptime === null ? "—" : formatUptime(uptime, locale)} />
            <Meta label={t.sampled} value={when(view.sampled_at.runtime, locale, t.never)} />
            <p className="col-span-full text-muted">{format(t.replica, { secs: view.refresh_secs })}</p>
          </dl>
        ) : (
          <span className="flex-1" />
        )}
        <AutoRefresh
          labels={t.refresh}
          value={interval}
          onChange={setIntervalSecs}
          busy={refreshing}
          onRefresh={() => {
            refresh();
            setTick((n) => n + 1);
          }}
        />
      </section>
      {seen.changed && <Message note={{ kind: "error", text: format(t.replicaChanged, { instance: seen.changed }) }} />}
      {failure && <Message note={{ kind: "error", text: failure.text }} />}
      {!view && !failure && <SkeletonTable rows={4} label={common.loading} />}
      {view && children({ view, interval, tick, seenAt: seen.at })}
    </>
  );
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div className="min-w-0">
      <dt className="text-2xs font-semibold tracking-wide text-muted uppercase">{label}</dt>
      <dd className="truncate font-mono text-ink" title={value}>
        {value}
      </dd>
    </div>
  );
}
