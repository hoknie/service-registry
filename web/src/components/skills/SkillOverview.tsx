"use client";

import { GitBranch, Pencil } from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { ago, when } from "@/i18n/time";
import type { Skill } from "@/lib/skills";

import { Markdown } from "../catalog/Markdown";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Panel } from "../ui/Panel";

type Props = { skill: Skill; locale: Locale; labels: Messages["skills"] };

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-3">
      <dt className="text-muted">{label}</dt>
      <dd className="min-w-0 text-right [overflow-wrap:anywhere] text-ink">{children}</dd>
    </div>
  );
}

export function SkillOverview({ skill, locale, labels: t }: Props) {
  const p = t.page;
  const about = skill.about.trim();
  const settingsHref = `/${locale}/skills/skill?id=${skill.id}&tab=settings&section=general`;
  const source = skill.source;
  return (
    <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_20rem]">
      <Panel
        title={p.about}
        actions={
          skill.can_write && (
            <Button asChild size="sm">
              <Link href={settingsHref}>
                <Pencil aria-hidden="true" />
                {about ? p.editAbout : p.addAbout}
              </Link>
            </Button>
          )
        }
      >
        <div className="grid gap-4">
          {about && <Markdown source={about} />}
          {skill.description && (
            <div className={about ? "border-t border-line pt-4" : undefined}>
              {about && <p className="mb-1 text-xs font-medium text-muted">{p.fromSkillMd}</p>}
              <p className="text-sm whitespace-pre-line text-ink-2">{skill.description}</p>
            </div>
          )}
        </div>
      </Panel>
      <div className="grid gap-4">
        {skill.tags.length > 0 && (
          <Panel title={p.tags}>
            <ul className="flex flex-wrap gap-1.5">
              {skill.tags.map((tag) => (
                <li key={tag}>
                  <Badge>{tag}</Badge>
                </li>
              ))}
            </ul>
          </Panel>
        )}
        <Panel title={p.meta}>
          <dl className="grid gap-2 text-sm">
            <Row label={p.latest}>{skill.version ? <span className="font-mono">v{skill.version.version}</span> : t.noVersion}</Row>
            {skill.author && <Row label={p.author}>{skill.author}</Row>}
            <Row label={p.updatedAt}>
              <span title={when(skill.updated_at, locale, "")}>{ago(skill.updated_at, locale, "")}</span>
            </Row>
            <Row label={p.downloadsCount}>{skill.downloads}</Row>
            {skill.license && <Row label={p.license}>{skill.license}</Row>}
            {skill.compatibility && <Row label={p.compatibility}>{skill.compatibility}</Row>}
            {source && (
              <Row label={p.source}>
                <span className="inline-flex items-center gap-1">
                  <GitBranch aria-hidden="true" className="size-3 shrink-0" />
                  {source.url.startsWith("http") ? (
                    <a className="text-signal hover:underline" href={source.url} rel="noreferrer" target="_blank">
                      {source.url}
                    </a>
                  ) : (
                    source.url
                  )}
                </span>
              </Row>
            )}
          </dl>
        </Panel>
      </div>
    </div>
  );
}
