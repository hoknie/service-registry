"use client";

import { Download, GitBranch } from "lucide-react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import type { Skill } from "@/lib/skills";

import { Badge } from "../ui/Badge";
import { PlaceCard } from "../places/PlaceCard";
import { DisabledBadge } from "./DisabledBadge";

export function SkillImage({
  skill,
  size = "md",
}: {
  skill: Pick<Skill, "image_url" | "title" | "name">;
  size?: "sm" | "md" | "lg" | "hero";
}) {
  const box = size === "hero" ? "size-14 rounded-2xl text-2xl shadow-elev-1" : size === "lg" ? "size-16 text-2xl" : size === "sm" ? "size-9 text-base" : "size-11 text-lg";
  if (skill.image_url) {
    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img src={skill.image_url} alt="" className={`${box} shrink-0 rounded-xl object-cover ring-1 ring-line`} />
    );
  }
  return (
    <span
      aria-hidden="true"
      className={`${box} grid shrink-0 place-items-center rounded-xl bg-gradient-to-br from-signal-soft to-cobalt-soft font-semibold text-signal ring-1 ring-line ring-inset`}
    >
      {(skill.title || skill.name).slice(0, 1).toUpperCase()}
    </span>
  );
}

export function SkillBadges({ skill, labels: t, max = 5 }: { skill: Skill; labels: Messages["skills"]; max?: number }) {
  const shown = skill.tags.slice(0, max);
  if (shown.length === 0 && skill.status !== "draft" && !skill.managed && skill.active) return null;
  return (
    <span className="flex flex-wrap gap-1.5">
      {skill.status === "draft" && <Badge tone="amber">{t.draft}</Badge>}
      <DisabledBadge item={skill} labels={t.off} />
      {skill.managed && (
        <Badge tone="outline">
          <GitBranch aria-hidden="true" className="size-3" /> {t.fromGit}
        </Badge>
      )}
      {shown.map((tag) => (
        <Badge key={tag} tone="neutral">
          {tag}
        </Badge>
      ))}
      {skill.tags.length > shown.length && <Badge tone="outline">{format(t.more, { count: skill.tags.length - shown.length })}</Badge>}
    </span>
  );
}

export function SkillCard({ skill, locale, labels: t }: { skill: Skill; locale: Locale; labels: Messages["skills"] }) {
  return (
    <PlaceCard href={`/${locale}/skills/skill?id=${skill.id}`} icon={<SkillImage skill={skill} size="sm" />} name={skill.title} kind={t.place.skill} slug={skill.name}>
      {skill.description && <p className="mt-3 line-clamp-3 text-sm text-ink-2">{skill.description}</p>}
      <span className="mt-3 block empty:hidden">
        <SkillBadges skill={skill} labels={t} />
      </span>
      <span className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted">
        <span className="font-mono">{skill.version ? `v${skill.version.version}` : t.noVersion}</span>
        <span title={when(skill.updated_at, locale, "")}>{format(t.updated, { when: ago(skill.updated_at, locale, "") })}</span>
        {skill.author && <span>{skill.author}</span>}
        <span className="inline-flex items-center gap-1">
          <Download aria-hidden="true" className="size-3" />
          {format(t.downloads, { count: skill.downloads })}
        </span>
      </span>
    </PlaceCard>
  );
}
