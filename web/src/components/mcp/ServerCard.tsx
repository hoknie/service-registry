"use client";

import { Activity, Wrench } from "lucide-react";
import { useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import { serverUrl, stateTone, type McpServer } from "@/lib/mcp";

import { Badge } from "../ui/Badge";
import { PlaceCard } from "../places/PlaceCard";
import { SetIcon } from "../ui/SetIcon";

export function ServerIcon({
  server,
  size = "md",
}: {
  server: Pick<McpServer, "id" | "icon_url" | "icon" | "has_icon" | "title">;
  size?: "xs" | "sm" | "md" | "lg";
}) {
  const box =
    size === "lg"
      ? "size-14 rounded-2xl text-2xl shadow-elev-1"
      : size === "sm"
        ? "size-9 text-base"
        : size === "xs"
          ? "size-5 text-[0.65rem] !rounded-md"
          : "size-11 text-lg";
  const src =
    server.icon_url ??
    (!server.icon && server.has_icon
      ? `/api${serverUrl(server.id, "/icon")}`
      : null);
  const [broken, setBroken] = useState<string | null>(null);
  if (src && broken !== src) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={src}
        alt=""
        onError={() => setBroken(src)}
        className={`${box} shrink-0 rounded-xl bg-surface object-contain ${size === "xs" ? "p-0.5" : "p-1.5"} ring-1 ring-line`}
      />
    );
  }
  if (server.icon) {
    return (
      <span
        aria-hidden="true"
        className={`${box} grid shrink-0 place-items-center rounded-xl bg-cobalt-soft text-cobalt ring-1 ring-line ring-inset`}
      >
        <SetIcon
          name={server.icon}
          className={size === "lg" ? "size-8" : size === "sm" ? "size-4" : size === "xs" ? "size-3" : "size-5"}
        />
      </span>
    );
  }
  return (
    <span
      aria-hidden="true"
      className={`${box} grid shrink-0 place-items-center rounded-xl bg-gradient-to-br from-cobalt-soft to-signal-soft font-semibold text-cobalt ring-1 ring-line ring-inset`}
    >
      {server.title.slice(0, 1).toUpperCase()}
    </span>
  );
}

export function StateBadge({
  server,
  labels: t,
}: {
  server: Pick<McpServer, "state">;
  labels: Messages["mcp"];
}) {
  return (
    <Badge tone={stateTone(server.state)} dot>
      {t.states[server.state]}
    </Badge>
  );
}

export function ServerCard({ server, locale, labels: t }: { server: McpServer; locale: Locale; labels: Messages["mcp"] }) {
  const shown = server.tags.slice(0, 5);
  return (
    <PlaceCard href={`/${locale}/mcp/server?id=${server.id}`} icon={<ServerIcon server={server} size="sm" />} name={server.title} kind={t.place.server} slug={server.builtin ? server.slug : `${server.slug}__*`}>
      {server.description && <p className="mt-3 line-clamp-3 text-sm text-ink-2">{server.description}</p>}
      <span className="mt-3 flex flex-wrap gap-1.5">
        <StateBadge server={server} labels={t} />
        {server.builtin && <Badge tone="cobalt">{t.builtin.badge}</Badge>}
        {server.status === "draft" && <Badge tone="amber">{t.statuses.draft}</Badge>}
        {shown.map((tag) => (
          <Badge key={tag} tone="neutral">
            {tag}
          </Badge>
        ))}
      </span>
      <span className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
        <span className="inline-flex items-center gap-1">
          <Wrench aria-hidden="true" className="size-3" />
          {server.tools_total !== undefined
            ? format(t.builtin.toolsOf, { count: server.tools_count, total: server.tools_total })
            : format(t.toolsCount, { count: server.tools_count })}
        </span>
        {!server.builtin && (
          <>
            <span className="inline-flex items-center gap-1">
              <Activity aria-hidden="true" className="size-3" />
              {format(t.callsCount, { count: server.calls })}
            </span>
            <span title={when(server.updated_at, locale, "")}>{format(t.updated, { when: ago(server.updated_at, locale, "") })}</span>
          </>
        )}
        {server.author && <span>{server.author}</span>}
      </span>
    </PlaceCard>
  );
}
