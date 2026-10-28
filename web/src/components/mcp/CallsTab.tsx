"use client";

import { Activity, ChevronRight } from "lucide-react";
import Link from "next/link";
import { Fragment, useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { when } from "@/i18n/time";
import { errorCode } from "@/lib/api";
import { cn } from "@/lib/cn";
import { prettyPayload, type McpCall, type McpServer } from "@/lib/mcp";
import { loadCalls } from "@/lib/mcpApi";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { Select } from "../ui/Select";
import { Table, Td, Th, Tr } from "../ui/Table";

const tones = { ok: "signal", tool_error: "amber", failed: "danger" } as const;

export function CallsTab({
  server,
  locale,
  labels: t,
}: {
  server: McpServer;
  locale: Locale;
  labels: Messages["mcp"];
}) {
  const x = t.calls;
  const { errors } = useUiText();
  const [outcome, setOutcome] = useState("");
  const [items, setItems] = useState<McpCall[] | null>(null);
  const [next, setNext] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [opened, setOpened] = useState<string | null>(null);

  const query = useCallback(
    (cursor?: string | null) => {
      const q = new URLSearchParams();
      if (outcome) q.set("outcome", outcome);
      if (cursor) q.set("cursor", cursor);
      return q.toString();
    },
    [outcome],
  );

  useEffect(() => {
    loadCalls(server.id, query())
      .then((r) => {
        setItems(r.items);
        setNext(r.next_cursor);
        setError(null);
      })
      .catch((e: unknown) => setError(errorText(errors, errorCode(e))));
  }, [server.id, query, errors]);

  const more = async () => {
    if (!next) return;
    const r = await loadCalls(server.id, query(next));
    setItems([...(items ?? []), ...r.items]);
    setNext(r.next_cursor);
  };

  return (
    <div className="grid gap-4">
      <Select
        className="w-56"
        aria-label={x.outcome}
        value={outcome}
        onChange={(e) => setOutcome(e.target.value)}
      >
        <option value="">{x.any}</option>
        <option value="ok">{x.outcomes.ok}</option>
        <option value="tool_error">{x.outcomes.tool_error}</option>
        <option value="failed">{x.outcomes.failed}</option>
      </Select>
      <Message note={error ? { kind: "error", text: error } : null} />
      {items && items.length === 0 && (
        <EmptyState icon={Activity} title={x.empty} />
      )}
      {items && items.length > 0 && (
        <div className="overflow-x-auto rounded-card border border-line bg-surface shadow-elev-1">
          <Table>
            <thead>
              <Tr>
                <Th>{x.at}</Th>
                <Th>{x.tool}</Th>
                <Th>{x.user}</Th>
                <Th>{x.via}</Th>
                <Th numeric>{x.duration}</Th>
                <Th>{x.outcome}</Th>
              </Tr>
            </thead>
            <tbody>
              {items.map((c) => {
                const open = opened === c.id;
                return (
                  <Fragment key={c.id}>
                    <Tr>
                      <Td className="whitespace-nowrap">
                        <button
                          type="button"
                          aria-expanded={open}
                          aria-label={open ? x.hide : x.show}
                          onClick={() => setOpened(open ? null : c.id)}
                          className="motion-control mr-1 inline-grid size-6 place-items-center rounded text-muted hover:bg-surface-3 hover:text-ink"
                        >
                          <ChevronRight
                            aria-hidden="true"
                            className={cn(
                              "size-4 transition-transform",
                              open && "rotate-90",
                            )}
                          />
                        </button>
                        {when(c.at, locale, "")}
                      </Td>
                      <Td className="font-mono">
                        <Link
                          className="text-signal hover:underline"
                          href={`/${locale}/mcp/server?id=${server.id}&tab=tools&tool=${encodeURIComponent(c.tool)}`}
                        >
                          {c.tool}
                        </Link>
                      </Td>
                      <Td>
                        {c.user ? (
                          <span title={c.user.email}>
                            {c.user.name || c.user.email}
                          </span>
                        ) : (
                          "—"
                        )}
                      </Td>
                      <Td>
                        {c.via
                          ? `${x.vias[c.via.kind]}: ${c.via.name ?? c.via.id.slice(0, 8)}`
                          : "—"}
                      </Td>
                      <Td numeric>{c.duration_ms} ms</Td>
                      <Td>
                        <Badge tone={tones[c.outcome]}>
                          {c.code
                            ? errorText(errors, c.code)
                            : x.outcomes[c.outcome]}
                        </Badge>
                      </Td>
                    </Tr>
                    {open && (
                      <tr>
                        <td
                          colSpan={6}
                          className="border-t border-line bg-surface-2 p-4"
                        >
                          {c.request === null && c.response === null ? (
                            <p className="text-sm text-muted">{x.notLogged}</p>
                          ) : (
                            <div className="grid gap-4 lg:grid-cols-2">
                              {(["request", "response"] as const).map((k) => (
                                <div
                                  key={k}
                                  className="grid min-w-0 content-start gap-1.5"
                                >
                                  <h3 className="text-xs font-semibold tracking-wide text-muted uppercase">
                                    {x[k]}
                                  </h3>
                                  <pre className="max-h-96 overflow-auto rounded-md border border-line bg-surface p-3 font-mono text-xs whitespace-pre-wrap text-ink [overflow-wrap:anywhere]">
                                    {c[k]
                                      ? prettyPayload(c[k], c.truncated)
                                      : "—"}
                                  </pre>
                                </div>
                              ))}
                              {c.truncated && (
                                <p className="text-xs text-muted lg:col-span-2">
                                  {x.truncated}
                                </p>
                              )}
                            </div>
                          )}
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </Table>
        </div>
      )}
      {next && (
        <div className="flex justify-center">
          <Button onClick={more}>{t.loadMore}</Button>
        </div>
      )}
    </div>
  );
}
