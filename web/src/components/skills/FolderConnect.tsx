"use client";

import { Plug } from "lucide-react";
import Link from "next/link";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { apiHref, folderCurlCommand, isOff, SKILL_TARGETS, SKILLS_BASE, type SkillFolder } from "@/lib/skills";
import { folderArchiveCount } from "@/lib/skillsApi";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { CodeBlock } from "../ui/CodeBlock";
import { Field } from "../ui/Field";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Select } from "../ui/Select";

type Props = { folder: SkillFolder; locale: Locale; labels: Messages["skills"] };

export function FolderConnect({ folder, locale, labels: t }: Props) {
  const c = t.connect;
  const f = t.folderConnect;
  const { errors } = useUiText();
  const [target, setTarget] = useState<string>(SKILL_TARGETS[0]);
  const [count, setCount] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);
  const off = isOff(folder);
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  const path = [...folder.ancestors.map((a) => a.slug), folder.slug].join("/");

  useEffect(() => {
    if (off) return;
    let alive = true;
    folderArchiveCount(folder.id)
      .then((n) => alive && setCount(n))
      .catch((e) => alive && setError(errorText(errors, errorCode(e))));
    return () => {
      alive = false;
    };
  }, [folder.id, off, errors]);

  return (
    <Panel title={f.title} icon={Plug}>
      {off ? (
        <p className="rounded-md bg-amber-soft px-3 py-2 text-sm text-amber">{t.off.folderOff}</p>
      ) : (
        <div className="grid gap-5 text-sm">
          <Message note={error ? { kind: "error", text: error } : null} />
          {count !== null && <p className="text-ink-2">{format(f.count, { count })}</p>}
          <section className="grid gap-2">
            <h3 className="text-base">{c.archive}</h3>
            <p className="text-muted">{c.archiveText}</p>
            <Field label={c.target}>
              {(p) => (
                <Select id={p.id} value={target} onChange={(e) => setTarget(e.target.value)}>
                  {SKILL_TARGETS.map((x) => (
                    <option key={x} value={x}>
                      {x}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
            <CodeBlock code={folderCurlCommand(origin, folder, target)} />
          </section>
          <section className="grid gap-2">
            <h3 className="text-base">{c.mcp}</h3>
            <p className="text-muted">{c.mcpText}</p>
            <CodeBlock title={c.endpoint} code={`${origin}/api/mcp`} />
            <CodeBlock code={format(f.phrase, { path })} />
            <p className="text-muted">
              {f.phraseTool}{" "}
              <Link className="text-signal underline-offset-2 hover:underline" href={`/${locale}/account?tab=tokens`}>
                {c.tokens}
              </Link>
            </p>
          </section>
          <section className="grid gap-2">
            <h3 className="text-base">{c.manual}</h3>
            <div>
              <Button asChild>
                <a href={apiHref(`${SKILLS_BASE}/folders/${encodeURIComponent(folder.id)}/archive`)} download>
                  {t.page.download}
                </a>
              </Button>
            </div>
          </section>
        </div>
      )}
    </Panel>
  );
}
