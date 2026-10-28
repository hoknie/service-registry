"use client";

import { TriangleAlert } from "lucide-react";
import type { ReactNode } from "react";

import { KNOWLEDGE_REFRESH_EVENTS, type KnowledgeRefreshEvent, type NodeKnowledgeSettings } from "@/lib/api";

import { Button } from "../ui/Button";
import { Checkbox } from "../ui/Field";
import type { CatalogLabels } from "../catalog/shared";

export type RefreshEventsDraft = { set: boolean; values: KnowledgeRefreshEvent[] };

type Labels = CatalogLabels["docs"]["settings"];

type Props = {
  labels: Labels;
  settings: NodeKnowledgeSettings;
  value: RefreshEventsDraft;
  canWrite: boolean;
  origin: ReactNode;
  onChange: (v: RefreshEventsDraft) => void;
};

function eventText(t: Labels, e: KnowledgeRefreshEvent): { label: string; hint: string } {
  switch (e) {
    case "service.deployed":
      return { label: t.eventServiceDeployed, hint: t.eventServiceDeployedHint };
    case "cluster.deployed":
      return { label: t.eventClusterDeployed, hint: t.eventClusterDeployedHint };
    case "repo_sync.webhook":
      return { label: t.eventRepoSyncWebhook, hint: t.eventRepoSyncWebhookHint };
    case "repo_sync.manual":
      return { label: t.eventRepoSyncManual, hint: t.eventRepoSyncManualHint };
  }
}

export function RefreshEventsField({ labels: t, settings, value, canWrite, origin, onChange }: Props) {
  const toggle = () => onChange({ set: !value.set, values: [...settings.refresh_events] });
  const pick = (e: KnowledgeRefreshEvent, on: boolean) => {
    const chosen = new Set(value.values);
    if (on) chosen.add(e);
    else chosen.delete(e);
    onChange({ set: true, values: KNOWLEDGE_REFRESH_EVENTS.filter((x) => chosen.has(x)) });
  };
  return (
    <div className="grid gap-2" aria-label={t.refreshEvents}>
      <div className="flex flex-wrap items-center gap-2">
        <h4 className="text-sm text-ink">{t.refreshEvents}</h4>
        {origin}
        {canWrite && (
          <span className="ml-auto">
            <Button type="button" size="sm" variant="ghost" onClick={toggle}>
              {value.set ? t.reset : t.override}
            </Button>
          </span>
        )}
      </div>
      {canWrite && value.set ? (
        <div className="grid gap-2">
          {KNOWLEDGE_REFRESH_EVENTS.map((e) => {
            const text = eventText(t, e);
            return (
              <div key={e} className="grid gap-0.5">
                <Checkbox label={text.label} checked={value.values.includes(e)} onChange={(ev) => pick(e, ev.target.checked)} />
                <p className="pl-6.5 text-xs text-muted">{text.hint}</p>
              </div>
            );
          })}
          {value.values.length === 0 && (
            <p role="status" className="flex items-center gap-1.5 text-xs text-ink-2">
              <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0 text-amber" />
              {t.refreshEventsEmpty}
            </p>
          )}
        </div>
      ) : (
        <p className="text-sm text-ink-2">
          {settings.refresh_events.length ? settings.refresh_events.map((e) => eventText(t, e).label).join(", ") : t.refreshEventsNone}
        </p>
      )}
    </div>
  );
}
