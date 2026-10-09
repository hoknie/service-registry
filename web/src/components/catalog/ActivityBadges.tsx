"use client";

import { CircleAlert, Clock3, LoaderCircle } from "lucide-react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { ago } from "@/i18n/time";
import { visible } from "@/lib/activity";
import type { ActivitySummary, Process, ProcessState } from "@/lib/api";
import { cn } from "@/lib/cn";

import { useUiText } from "../UiText";
import { Badge, type Tone } from "../ui/Badge";
import { Tooltip } from "../ui/Tooltip";
import type { CatalogLabels } from "./shared";

type Labels = CatalogLabels["activity"];

const tone: Record<ProcessState, Tone> = { running: "cobalt", queued: "neutral", failed: "danger", idle: "outline" };

function StateIcon({ state }: { state: ProcessState }) {
  if (state === "running") return <LoaderCircle aria-hidden="true" className="motion-spin size-3.5" />;
  if (state === "failed") return <CircleAlert aria-hidden="true" className="size-3.5" />;
  return <Clock3 aria-hidden="true" className="size-3.5" />;
}

export function ProcessBadges({
  processes,
  labels: t,
  locale,
  compact,
  className,
}: {
  processes: Process[];
  labels: Labels;
  locale: Locale;
  compact?: boolean;
  className?: string;
}) {
  const { errors } = useUiText();
  const shown = visible(processes);
  if (shown.length === 0) return null;
  return (
    <ul aria-label={t.label} className={cn("flex flex-wrap items-center gap-1.5", className)}>
      {shown.map((p) => {
        const name = t.kinds[p.kind];
        const pending = p.kind === "index" && p.pending ? format(t.pending, { count: p.pending }) : "";
        const text = [`${name}: ${t.states[p.state]}`, pending].filter(Boolean).join(" · ");
        const hint = (
          <span className="grid gap-0.5">
            <span>{text}</span>
            {p.state === "failed" && p.code && (
              <span>
                {errorText(errors, p.code)} <code className="text-muted">{p.code}</code>
              </span>
            )}
            {p.last_at && <span className="text-muted">{format(t.last, { when: ago(p.last_at, locale, "") })}</span>}
          </span>
        );
        return (
          <li key={p.kind}>
            <Tooltip content={hint}>
              <Badge tone={tone[p.state]} tabIndex={0} className={cn(compact && "px-1.5")}>
                <StateIcon state={p.state} />
                {compact ? <span className="sr-only">{text}</span> : <span>{p.kind === "index" && p.pending ? `${name} · ${p.pending}` : name}</span>}
              </Badge>
            </Tooltip>
          </li>
        );
      })}
    </ul>
  );
}

export function SummaryBadges({ summary, labels: t, className }: { summary: ActivitySummary; labels: Labels; className?: string }) {
  const states = (["running", "queued", "failed"] as const).filter((s) => summary[s] > 0);
  if (states.length === 0) return null;
  return (
    <ul aria-label={t.label} className={cn("flex flex-wrap items-center gap-1.5", className)}>
      {states.map((s) => (
        <li key={s}>
          <Badge tone={tone[s]}>
            <StateIcon state={s} />
            {format(t.summary[s], { count: summary[s] })}
          </Badge>
        </li>
      ))}
    </ul>
  );
}
