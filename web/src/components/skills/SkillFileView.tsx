"use client";

import { Download } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, errorCode } from "@/lib/api";
import { cn } from "@/lib/cn";
import {
  apiHref,
  humanSize,
  resolveLink,
  skillUrl,
  withVersion,
  type SkillFile,
  type SkillFileContent,
} from "@/lib/skills";

import type { DocHeading } from "../knowledge/DocViewer";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";

type Props = {
  skillId: string;
  version: string;
  file: SkillFile;
  files: SkillFile[];
  onOpen: (path: string) => void;
  onHeadings: (h: DocHeading[]) => void;
  labels: Messages["skills"];
};

type Loaded = {
  key: string;
  content: SkillFileContent | null;
  html: string | null;
  front: [string, string][];
};

function headingsOf(root: HTMLElement): DocHeading[] {
  const all = [
    ...root.querySelectorAll<HTMLElement>("h1[id], h2[id], h3[id], h4[id]"),
  ].map((h) => {
    const copy = h.cloneNode(true) as HTMLElement;
    copy.querySelectorAll(".heading-anchor").forEach((a) => a.remove());
    return {
      id: h.id,
      text: (copy.textContent ?? "").trim(),
      level: Number(h.tagName.slice(1)),
    };
  });
  const sub = all.filter((h) => h.level > 1);
  return sub.length > 0 ? sub : all;
}

async function frontmatter(
  text: string,
): Promise<{ rows: [string, string][]; body: string }> {
  const m = /^﻿?---\r?\n([\s\S]*?)\r?\n---\r?\n?/.exec(text);
  if (!m) return { rows: [], body: text };
  const { parse } = await import("yaml");
  const rows: [string, string][] = [];
  try {
    const data = parse(m[1] ?? "") as Record<string, unknown> | null;
    const add = (prefix: string, value: unknown) => {
      if (value && typeof value === "object" && !Array.isArray(value)) {
        for (const [k, v] of Object.entries(value))
          add(prefix ? `${prefix}.${k}` : k, v);
      } else
        rows.push([
          prefix,
          Array.isArray(value) ? value.join(", ") : String(value ?? ""),
        ]);
    };
    add("", data ?? {});
  } catch {
    rows.push(["frontmatter", m[1] ?? ""]);
  }
  return { rows, body: text.slice(m[0].length) };
}

export function SkillFileView({
  skillId,
  version,
  file,
  files,
  onOpen,
  onHeadings,
  labels: t,
}: Props) {
  const body = useRef<HTMLDivElement>(null);
  const { errors, common } = useUiText();
  const key = `${skillId}:${version}:${file.path}`;
  const [doc, setDoc] = useState<Loaded | null>(null);
  const [failure, setFailure] = useState<{ key: string; text: string } | null>(
    null,
  );
  const raw = apiHref(
    withVersion(skillUrl(skillId, "/raw"), version, { path: file.path }),
  );
  const known = new Set(files.map((f) => f.path));
  const hrefOf = (p: string) => `#skill-file=${encodeURIComponent(p)}`;

  useEffect(() => {
    let live = true;
    if (file.kind === "image" || file.kind === "binary") {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setDoc({ key, content: null, html: null, front: [] });
      return;
    }
    apiGet<SkillFileContent>(
      withVersion(skillUrl(skillId, "/file"), version, { path: file.path }),
    )
      .then(async (c) => {
        let html: string | null = null;
        let front: [string, string][] = [];
        if (c.kind === "markdown" && c.content !== null) {
          const split = await frontmatter(c.content);
          front = split.rows;
          const { renderReadme } = await import("@/lib/readme");
          const dir = c.path.includes("/")
            ? c.path.slice(0, c.path.lastIndexOf("/"))
            : undefined;
          html = renderReadme(split.body, {
            branch: null,
            dir,
            docLink: (p) => (known.has(p) ? hrefOf(p) : null),
            alerts: common.alerts,
            anchorLabel: common.headingAnchor,
          });
        }
        if (live) {
          setDoc({ key, content: c, html, front });
          setFailure(null);
        }
      })
      .catch(
        (e: unknown) =>
          live && setFailure({ key, text: errorText(errors, errorCode(e)) }),
      );
    return () => {
      live = false;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);

  const current = doc?.key === key ? doc : null;
  const error = failure?.key === key ? failure.text : null;
  const shown = error ? null : (current ?? doc);
  const pending = !current && !error && !!doc;

  useEffect(() => {
    if (current?.html && body.current) onHeadings(headingsOf(body.current));
    else if (current || error) onHeadings([]);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current, error]);

  const onClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const href = (e.target as HTMLElement).closest("a")?.getAttribute("href");
    if (!href || e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    const target = href.startsWith("#skill-file=")
      ? decodeURIComponent(href.slice("#skill-file=".length))
      : resolveLink(file.path, href, known);
    if (!target) return;
    e.preventDefault();
    onOpen(target);
  };

  const lines = shown?.content?.content?.split("\n") ?? [];
  return (
    <Panel
      title={
        <span className="font-mono text-sm [overflow-wrap:anywhere]">
          {file.path}
        </span>
      }
    >
      <div
        aria-busy={pending || undefined}
        inert={pending}
        className={cn("motion-control", pending && "opacity-60 delay-150")}
      >
        {error && <Message note={{ kind: "error", text: error }} />}
        {!shown && !error && <SkeletonPanel lines={6} label={common.loading} />}
        {shown && shown.front.length > 0 && (
          <Table className="mb-5" aria-label={t.page.frontmatter}>
            <thead>
              <Tr>
                <Th>{t.page.field}</Th>
                <Th>{t.page.value}</Th>
              </Tr>
            </thead>
            <tbody>
              {shown.front.map(([k, v]) => (
                <Tr key={k}>
                  <Td className="font-mono text-xs">{k}</Td>
                  <Td className="text-sm [overflow-wrap:anywhere]">{v}</Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        )}
        {shown?.html && (
          <div
            ref={body}
            className="readme max-w-none animate-rise-in text-sm leading-relaxed text-ink-2"
            onClick={onClick}
            dangerouslySetInnerHTML={{ __html: shown.html }}
          />
        )}
        {shown && !shown.html && shown.content?.content != null && (
          <pre className="overflow-x-auto rounded-md bg-surface-2 p-4 font-mono text-xs leading-relaxed text-ink-2">
            {lines.map((line, i) => (
              <span key={i} className="block">
                <span
                  aria-hidden="true"
                  className="mr-4 inline-block w-8 text-right text-muted select-none"
                >
                  {i + 1}
                </span>
                {line}
              </span>
            ))}
          </pre>
        )}
        {shown && file.kind === "image" && (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={raw}
            alt={file.path}
            className="max-h-[32rem] max-w-full rounded-md ring-1 ring-line"
          />
        )}
        {shown && file.kind === "binary" && (
          <div className="flex flex-wrap items-center gap-3 text-sm text-muted">
            <span>{format(t.page.binary, { size: humanSize(file.size) })}</span>
            <Button asChild size="sm">
              <a href={raw} download>
                <Download aria-hidden="true" className="size-4" />{" "}
                {t.page.downloadFile}
              </a>
            </Button>
          </div>
        )}
      </div>
    </Panel>
  );
}
