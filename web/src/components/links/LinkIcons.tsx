"use client";

import { useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { LinkGlyph } from "@/lib/api";
import { cn } from "@/lib/cn";
import { useLinkKinds } from "@/lib/linkKinds";

import { Badge } from "../ui/Badge";
import { Tooltip } from "../ui/Tooltip";
import { LinkIcon } from "./LinkIcon";
import { kindName } from "./shared";

export type IconLink = { link_key: string; kind_key: string; title: string | null; icon: LinkGlyph; url: string | null; missing?: string[] };

export type IconLabels = { missing: string; noUrl: string; links: string };

export function Glyph({ icon, kindIcon, size = 16, className }: { icon: LinkGlyph; kindIcon?: string; size?: 16 | 20; className?: string }) {
  const [broken, setBroken] = useState(false);
  const box = size === 20 ? "size-5" : "size-4";
  if (icon.kind === "builtin" || broken) {
    return <LinkIcon icon={icon.kind === "builtin" ? icon.name : kindIcon} className={cn(box, "text-ink-2", className)} />;
  }
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={icon.url}
      alt=""
      width={size}
      height={size}
      referrerPolicy="no-referrer"
      loading="lazy"
      onError={() => setBroken(true)}
      className={cn(box, "shrink-0 object-contain", className)}
    />
  );
}

export function LinkIconRow({
  links,
  locale,
  labels,
  max,
  size = 16,
  inert,
  className,
}: {
  links: IconLink[];
  locale: Locale;
  labels: IconLabels;
  max?: number;
  size?: 16 | 20;
  inert?: boolean;
  className?: string;
}) {
  const kinds = useLinkKinds();
  if (links.length === 0) return null;
  const shown = max ? links.slice(0, max) : links;
  return (
    <ul aria-label={labels.links} className={cn("flex flex-wrap items-center gap-1.5", className)}>
      {shown.map((l) => {
        const name = l.title ?? kindName(kinds, l.kind_key, locale);
        const kindIcon = kinds.find((k) => k.key === l.kind_key)?.icon;
        const glyph = <Glyph icon={l.icon} kindIcon={kindIcon} size={size} />;
        const box = cn("grid place-items-center rounded-md ring-1 ring-line ring-inset", size === 20 ? "size-8" : "size-6");
        if (inert) {
          return (
            <li key={l.link_key} title={`${name} — ${l.url ?? labels.noUrl}`} className={cn(box, l.url ? "bg-surface" : "bg-surface-2 opacity-45 grayscale")}>
              {glyph}
              <span className="sr-only">{l.url ? name : `${name}: ${labels.noUrl}`}</span>
            </li>
          );
        }
        if (!l.url) {
          const hint = l.missing && l.missing.length > 0 ? format(labels.missing, { vars: l.missing.join(", ") }) : labels.noUrl;
          return (
            <li key={l.link_key}>
              <Tooltip content={`${name} — ${hint}`}>
                <span tabIndex={0} aria-label={`${name}: ${hint}`} className={cn(box, "bg-surface-2 opacity-45 grayscale")}>
                  {glyph}
                </span>
              </Tooltip>
            </li>
          );
        }
        return (
          <li key={l.link_key}>
            <Tooltip content={`${name} — ${l.url}`}>
              <a href={l.url} target="_blank" rel="noopener noreferrer" aria-label={name} className={cn(box, "motion-control bg-surface hover:ring-signal")}>
                {glyph}
              </a>
            </Tooltip>
          </li>
        );
      })}
      {max && links.length > max && (
        <li>
          <Badge tone="outline">+{links.length - max}</Badge>
        </li>
      )}
    </ul>
  );
}
