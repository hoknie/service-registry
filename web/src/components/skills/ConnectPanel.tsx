"use client";

import { Plug } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import {
  agentPhrase,
  apiHref,
  curlCommand,
  SKILL_TARGETS,
  skillUrl,
  withVersion,
  type Skill,
  type SkillVersion,
} from "@/lib/skills";

import { Button } from "../ui/Button";
import { CodeBlock } from "../ui/CodeBlock";
import { Panel } from "../ui/Panel";
import { Field } from "../ui/Field";
import { Select } from "../ui/Select";

type Props = {
  skill: Skill;
  versions: SkillVersion[];
  locale: Locale;
  labels: Messages["skills"];
};

export function ConnectPanel({ skill, versions, locale, labels: t }: Props) {
  const c = t.connect;
  const [version, setVersion] = useState("");
  const [target, setTarget] = useState<string>(SKILL_TARGETS[0]);
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  const chosen = version || null;
  return (
    <Panel title={c.title} icon={Plug}>
      <div className="grid gap-5 text-sm">
        {skill.status === "draft" && (
          <p className="rounded-md bg-amber-soft px-3 py-2 text-amber">
            {c.draftWarning}
          </p>
        )}
        <Field label={c.version} hint={c.latestHint}>
          {(p) => (
            <Select
              id={p.id}
              value={version}
              onChange={(e) => setVersion(e.target.value)}
            >
              <option value="">{c.latest}</option>
              {versions
                .filter((v) => v.available)
                .map((v) => (
                  <option key={v.version} value={v.version}>
                    {v.version}
                  </option>
                ))}
            </Select>
          )}
        </Field>
        <section className="grid gap-2">
          <h3 className="text-base">{c.mcp}</h3>
          <p className="text-muted">{c.mcpText}</p>
          <CodeBlock title={c.endpoint} code={`${origin}/api/mcp`} />
          <CodeBlock
            code={agentPhrase(c.phrase, c.phraseVersion, skill.name, chosen)}
          />
          <p className="text-muted">
            {c.phraseTool}{" "}
            <Link
              className="text-signal underline-offset-2 hover:underline"
              href={`/${locale}/account?tab=tokens`}
            >
              {c.tokens}
            </Link>
          </p>
        </section>
        <section className="grid gap-2">
          <h3 className="text-base">{c.archive}</h3>
          <p className="text-muted">{c.archiveText}</p>
          <Field label={c.target}>
            {(p) => (
              <Select
                id={p.id}
                value={target}
                onChange={(e) => setTarget(e.target.value)}
              >
                {SKILL_TARGETS.map((x) => (
                  <option key={x} value={x}>
                    {x}
                  </option>
                ))}
              </Select>
            )}
          </Field>
          <CodeBlock code={curlCommand(origin, skill, chosen, target)} />
        </section>
        <section className="grid gap-2">
          <h3 className="text-base">{c.manual}</h3>
          <div>
            <Button asChild>
              <a
                href={apiHref(
                  withVersion(skillUrl(skill.id, "/archive"), chosen),
                )}
                download
              >
                {t.page.download}
              </a>
            </Button>
          </div>
        </section>
      </div>
    </Panel>
  );
}
