"use client";

import { KeyRound } from "lucide-react";
import { useSyncExternalStore } from "react";

import { format } from "@/i18n/format";

import { Button } from "../ui/Button";
import { CodeBlock } from "../ui/CodeBlock";
import { CopyButton } from "../ui/CopyButton";
import { Panel } from "../ui/Panel";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "../ui/Tabs";
import { snippets } from "./ingestSnippets";
import type { CatalogLabels } from "./shared";

type Props = { projectId: string; labels: CatalogLabels["howTo"]; onKeys?: () => void };

const noop = () => () => {};

export function IngestHowTo({ projectId, labels, onKeys }: Props) {
  const origin = useSyncExternalStore(noop, () => window.location.origin, () => "");
  if (!origin) return null;
  const s = snippets(origin, projectId);

  const examples = [
    { name: "curl", title: labels.curl, text: s.curl },
    { name: "github", title: labels.github, text: s.github },
    { name: "gitlab", title: labels.gitlab, text: s.gitlab },
  ];

  return (
    <div className="grid gap-6">
      <Panel title={labels.title} description={labels.lead}>
        <dl className="grid gap-3">
          {[
            { term: labels.endpoint, value: `POST ${s.endpoint}`, copy: s.endpoint },
            { term: labels.projectId, value: projectId, copy: projectId },
          ].map((row) => (
            <div key={row.term} className="grid gap-1.5 sm:grid-cols-[10rem_minmax(0,1fr)] sm:items-center">
              <dt className="text-sm text-muted">{row.term}</dt>
              <dd className="flex min-w-0 items-center gap-2 rounded-md border border-line bg-surface-2 py-1 pr-1 pl-3">
                <code className="min-w-0 flex-1 truncate text-sm text-ink" title={row.value}>
                  {row.value}
                </code>
                <CopyButton text={row.copy} variant="ghost" />
              </dd>
            </div>
          ))}
        </dl>
      </Panel>

      <ol className="grid gap-4">
        <li className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3">
          <span aria-hidden="true" className="grid size-7 place-items-center rounded-full border border-line-strong bg-surface text-sm font-medium text-ink-2">
            1
          </span>
          <div className="grid gap-2 pt-0.5">
            <p className="text-ink">{labels.stepKey}</p>
            {onKeys && (
              <Button size="sm" className="justify-self-start" onClick={onKeys}>
                <KeyRound aria-hidden="true" />
                {labels.toKeys}
              </Button>
            )}
          </div>
        </li>
        <li className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3">
          <span aria-hidden="true" className="grid size-7 place-items-center rounded-full border border-line-strong bg-surface text-sm font-medium text-ink-2">
            2
          </span>
          <p className="pt-0.5 text-ink">{format(labels.secrets, { url: origin })}</p>
        </li>
        <li className="grid grid-cols-[2rem_minmax(0,1fr)] gap-3">
          <span aria-hidden="true" className="grid size-7 place-items-center rounded-full border border-line-strong bg-surface text-sm font-medium text-ink-2">
            3
          </span>
          <div className="grid min-w-0 gap-3 pt-0.5">
            <p className="text-ink">{labels.stepSend}</p>
            <Tabs defaultValue="curl">
              <TabsList aria-label={labels.examples} className="mb-3">
                {examples.map((e) => (
                  <TabsTrigger key={e.name} value={e.name}>
                    {e.title}
                  </TabsTrigger>
                ))}
              </TabsList>
              {examples.map((e) => (
                <TabsContent key={e.name} value={e.name}>
                  <CodeBlock title={e.title} code={e.text} />
                </TabsContent>
              ))}
            </Tabs>
            <p className="text-sm text-muted">{labels.idempotency}</p>
          </div>
        </li>
      </ol>
    </div>
  );
}
