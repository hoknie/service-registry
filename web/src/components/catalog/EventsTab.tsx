"use client";

import { ChevronRight, Inbox, Plug, RotateCcw } from "lucide-react";
import { Fragment, useCallback, useEffect, useState, type FormEvent } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { ago, when } from "@/i18n/time";
import { apiGet, errorCode, type IngestEvent, type Page } from "@/lib/api";
import { cn } from "@/lib/cn";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { CodeBlock } from "../ui/CodeBlock";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { Pager } from "../ui/Pager";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import type { CatalogLabels } from "./shared";

const PAGE = 50;

type Props = { projectId: string; locale: Locale; labels: CatalogLabels; onConnect: () => void };

export function EventsTab({ projectId, locale, labels, onConnect }: Props) {
  const { errors, common } = useUiText();
  const t = labels.events;
  const [page, setPage] = useState<Page<IngestEvent> | null>(null);
  const [offset, setOffset] = useState(0);
  const [draft, setDraft] = useState("");
  const [type, setType] = useState("");
  const [open, setOpen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    const q = new URLSearchParams();
    if (type) q.set("type", type);
    q.set("limit", String(PAGE));
    q.set("offset", String(offset));
    try {
      setPage(await apiGet<Page<IngestEvent>>(`/v1/catalog/nodes/${projectId}/events?${q.toString()}`));
      setError(null);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  }, [projectId, offset, type, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const apply = (e: FormEvent) => {
    e.preventDefault();
    setOffset(0);
    setType(draft.trim());
  };

  if (error && !page) return <Message note={{ kind: "error", text: error }} />;
  if (!page) return <SkeletonTable rows={6} label={common.loading} />;

  if (page.items.length === 0 && !type) {
    return (
      <EmptyState
        icon={Inbox}
        title={t.empty}
        text={t.emptyText}
        action={
          <Button onClick={onConnect}>
            <Plug aria-hidden="true" />
            {labels.deployments.connect}
          </Button>
        }
      />
    );
  }

  return (
    <section aria-labelledby="events-title" className="grid gap-4">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <h2 id="events-title">{t.title}</h2>
        <form className="flex flex-wrap items-end gap-2" onSubmit={apply}>
          <Field label={t.filterType}>
            {(p) => <Input {...p} className="w-56 font-mono text-sm" value={draft} placeholder={t.any} onChange={(e) => setDraft(e.target.value)} />}
          </Field>
          <Button type="submit" variant="primary">
            {labels.deployments.filter}
          </Button>
          {type && (
            <Button
              variant="ghost"
              onClick={() => {
                setDraft("");
                setType("");
                setOffset(0);
              }}
            >
              <RotateCcw aria-hidden="true" />
              {labels.deployments.reset}
            </Button>
          )}
        </form>
      </div>
      {error && <Message note={{ kind: "error", text: error }} />}

      {page.items.length === 0 ? (
        <p role="status" className="rounded-lg border border-dashed border-line-strong px-4 py-6 text-center text-sm text-muted">
          {t.noMatch}
        </p>
      ) : (
        <Table>
          <thead>
            <tr>
              <Th className="w-10">
                <span className="sr-only">{t.payload}</span>
              </Th>
              <Th>{t.type}</Th>
              <Th>{t.received}</Th>
              <Th>{t.version}</Th>
              <Th>{t.occurred}</Th>
              <Th>{t.idempotencyKey}</Th>
              <Th>{t.key}</Th>
            </tr>
          </thead>
          <tbody>
            {page.items.map((e) => {
              const expanded = open === e.id;
              return (
                <Fragment key={e.id}>
                  <Tr aria-selected={expanded}>
                    <Td>
                      <Button
                        size="icon"
                        variant="ghost"
                        aria-expanded={expanded}
                        aria-controls={`payload-${e.id}`}
                        aria-label={`${expanded ? t.hide : t.show}: ${e.type}`}
                        onClick={() => setOpen(expanded ? null : e.id)}
                      >
                        <ChevronRight aria-hidden="true" className={cn("transition-transform", expanded && "rotate-90")} />
                      </Button>
                    </Td>
                    <Td>
                      <code className="font-medium text-ink">{e.type}</code>
                    </Td>
                    <Td className="whitespace-nowrap text-ink-2">
                      <time dateTime={e.received_at} title={when(e.received_at, locale, "")}>
                        {ago(e.received_at, locale, "")}
                      </time>
                    </Td>
                    <Td className="tabular-nums">v{e.version}</Td>
                    <Td className="whitespace-nowrap text-ink-2">{when(e.occurred_at, locale, "")}</Td>
                    <Td className="max-w-56">
                      <code className="block truncate text-xs text-ink-2" title={e.idempotency_key}>
                        {e.idempotency_key}
                      </code>
                    </Td>
                    <Td className="whitespace-nowrap">
                      {e.key_prefix ? <code className="text-xs text-ink-2">{e.key_prefix}…</code> : <span className="text-muted">{t.keyGone}</span>}
                    </Td>
                  </Tr>
                  {expanded && (
                    <tr id={`payload-${e.id}`}>
                      <td colSpan={7} className="border-b border-line bg-surface-2 px-4 py-3">
                        <CodeBlock title={t.payload} code={JSON.stringify(e.payload, null, 2)} className="bg-surface" />
                      </td>
                    </tr>
                  )}
                </Fragment>
              );
            })}
          </tbody>
        </Table>
      )}
      <Pager offset={offset} shown={page.items.length} total={page.total} size={PAGE} labels={labels.pager} onChange={setOffset} />
    </section>
  );
}
