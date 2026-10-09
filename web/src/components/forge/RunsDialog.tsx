"use client";

import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { ago, when } from "@/i18n/time";
import { apiGet, type Items, type SyncRun } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Message } from "../ui/Message";
import { SkeletonTable } from "../ui/Skeleton";
import { runTone, type ForgeLabels } from "./shared";

type Props = { url: string; locale: Locale; labels: ForgeLabels; fail: (e: unknown) => string; errorOf: (code: string) => string; onClose: () => void };

export function RunsDialog({ url, locale, labels: t, fail, errorOf, onClose }: Props) {
  const { common } = useUiText();
  const [runs, setRuns] = useState<SyncRun[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    apiGet<Items<SyncRun>>(url as `/${string}`)
      .then((r) => setRuns(r.items))
      .catch((e: unknown) => setError(fail(e)));
  }, [url, fail]);

  return (
    <Dialog open size="lg" onOpenChange={(open) => !open && onClose()} title={t.runsTitle} footer={<Button onClick={onClose}>{t.close}</Button>}>
      {error && <Message note={{ kind: "error", text: error }} />}
      {!runs && !error && <SkeletonTable rows={4} label={common.loading} />}
      {runs && runs.length === 0 && <p className="text-sm text-muted">{t.runsEmpty}</p>}
      {runs && runs.length > 0 && (
        <ol className="grid max-h-[60vh] gap-3 overflow-y-auto">
          {runs.map((r) => (
            <li key={r.id} className="rounded-lg border border-line p-3">
              <div className="flex flex-wrap items-center gap-2 text-sm">
                <Badge tone={runTone[r.status]} dot>
                  {t.statuses[r.status]}
                </Badge>
                <span className="text-ink-2" title={when(r.started_at, locale, "")}>
                  {ago(r.started_at, locale, "")}
                </span>
                <span className="text-muted">· {t.triggers[r.trigger]}</span>
              </div>
              <p className="mt-1.5 text-sm text-ink-2">{format(t.counts, { created: r.created, updated: r.updated, orphaned: r.orphaned, skipped: r.skipped })}</p>
              {r.error_code && <p className="mt-1 text-sm text-danger">{errorOf(r.error_code)}</p>}
              {r.problems.length > 0 && (
                <details className="mt-2 text-sm">
                  <summary className="cursor-pointer text-muted">
                    {t.problems} ({r.problems.length})
                  </summary>
                  <ul className="mt-1 grid gap-1">
                    {r.problems.map((p) => (
                      <li key={p.full_path + p.code}>
                        <code className="text-ink">{p.full_path}</code> — <span className="text-muted">{errorOf(p.code)}</span>
                      </li>
                    ))}
                  </ul>
                </details>
              )}
            </li>
          ))}
        </ol>
      )}
    </Dialog>
  );
}
