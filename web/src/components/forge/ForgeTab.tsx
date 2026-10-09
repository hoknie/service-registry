"use client";

import { Activity, Eye, GitBranch, Pencil, Plus, RefreshCw, ShieldCheck, Trash, Webhook } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { ago, when } from "@/i18n/time";
import { apiGet, apiSend, errorCode, type ForgeConnection, type Items } from "@/lib/api";

import { FORGE_NAMES, type CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { ConnectionDialog } from "./ConnectionDialog";
import { PreviewDialog } from "./PreviewDialog";
import { RunsDialog } from "./RunsDialog";
import { runTone } from "./shared";
import { WebhookDialog } from "./WebhookDialog";

type Props = { nodeId: string; locale: Locale; labels: CatalogLabels; canWrite: boolean; canAccess: boolean; onSynced: () => void };

type Open =
  | { kind: "create" }
  | { kind: "edit" | "preview" | "runs" | "webhook" | "delete"; connection: ForgeConnection }
  | null;

export function ForgeTab({ nodeId, locale, labels, canWrite, canAccess, onSynced }: Props) {
  const { errors, common } = useUiText();
  const t = labels.forge;
  const [connections, setConnections] = useState<ForgeConnection[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState<Open>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const errorOf = useCallback((code: string) => errorText(errors, code), [errors]);
  const base = `/v1/catalog/nodes/${nodeId}/forge/connections`;

  const load = useCallback(async () => {
    try {
      setConnections((await apiGet<Items<ForgeConnection>>(base as `/${string}`)).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [base, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const act = async (c: ForgeConnection, action: "check" | "sync") => {
    setBusy(c.id + action);
    try {
      await apiSend("POST", `${base}/${c.id}/${action}` as `/${string}`);
      toast.success(action === "check" ? t.checked : t.syncQueued);
      if (action === "sync") {
        window.setTimeout(() => {
          void load();
          onSynced();
        }, 3000);
      }
    } catch (e) {
      toast.error(fail(e));
    } finally {
      setBusy(null);
    }
  };

  const add = canAccess && (
    <Button variant="primary" onClick={() => setOpen({ kind: "create" })}>
      <Plus aria-hidden="true" />
      {t.add}
    </Button>
  );

  return (
    <div className="grid gap-4">
      <Panel title={t.title} description={t.lead} actions={add || undefined}>
        {error && <Message note={{ kind: "error", text: error }} />}
        {!connections && !error && <SkeletonTable rows={3} label={common.loading} />}
        {connections && connections.length === 0 && (
          <EmptyState icon={GitBranch} title={t.empty} text={canAccess ? t.emptyText : undefined} action={add || undefined} />
        )}
        {connections && connections.length > 0 && (
          <ul className="grid gap-3">
            {connections.map((c) => (
              <li key={c.id} className="rounded-lg border border-line p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="flex flex-wrap items-center gap-2 font-medium text-ink">
                      <Badge>{FORGE_NAMES[c.kind]}</Badge>
                      <code className="break-all">{c.owner_path}</code>
                      {c.webhook && (
                        <Badge tone="outline">
                          <Webhook aria-hidden="true" className="size-3" />
                          {t.modes[c.webhook.mode]}
                        </Badge>
                      )}
                    </p>
                    <p className="mt-1 text-xs break-all text-muted">{c.api_url}</p>
                    <p className="mt-1 text-sm text-ink-2">
                      {c.credentials.kind === "token" ? format(t.credToken, { fingerprint: c.credentials.fingerprint }) : format(t.credRef, { ref: c.credentials.ref })}
                      {" · "}
                      {format(t.minutes, { n: Math.round(c.interval_secs / 60) })}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    {canWrite && (
                      <>
                        <Button size="sm" disabled={busy !== null} onClick={() => void act(c, "check")}>
                          <ShieldCheck aria-hidden="true" />
                          {t.check}
                        </Button>
                        <Button size="sm" onClick={() => setOpen({ kind: "preview", connection: c })}>
                          <Eye aria-hidden="true" />
                          {t.preview}
                        </Button>
                        <Button size="sm" variant="primary" disabled={busy !== null} onClick={() => void act(c, "sync")}>
                          <RefreshCw aria-hidden="true" />
                          {t.sync}
                        </Button>
                      </>
                    )}
                    <Button size="sm" variant="ghost" onClick={() => setOpen({ kind: "runs", connection: c })}>
                      <Activity aria-hidden="true" />
                      {t.runs}
                    </Button>
                    {canAccess && (
                      <>
                        <Button size="sm" variant="ghost" onClick={() => setOpen({ kind: "webhook", connection: c })}>
                          <Webhook aria-hidden="true" />
                          {t.webhook}
                        </Button>
                        <Button size="sm" variant="ghost" onClick={() => setOpen({ kind: "edit", connection: c })}>
                          <Pencil aria-hidden="true" />
                          {t.edit}
                        </Button>
                        <Button size="sm" variant="danger-ghost" onClick={() => setOpen({ kind: "delete", connection: c })}>
                          <Trash aria-hidden="true" />
                          {t.delete}
                        </Button>
                      </>
                    )}
                  </div>
                </div>
                <dl className="mt-3 grid grid-cols-[max-content_minmax(0,1fr)] gap-x-6 gap-y-1.5 text-sm">
                  <dt className="text-muted">{t.lastRun}</dt>
                  <dd className="text-ink-2">
                    {c.last_run ? (
                      <span className="flex flex-wrap items-center gap-2">
                        <Badge tone={runTone[c.last_run.status]} dot>
                          {t.statuses[c.last_run.status]}
                        </Badge>
                        <span title={when(c.last_run.started_at, locale, "")}>{ago(c.last_run.started_at, locale, "")}</span>
                        <span className="text-muted">
                          {format(t.counts, {
                            created: c.last_run.created,
                            updated: c.last_run.updated,
                            orphaned: c.last_run.orphaned,
                            skipped: c.last_run.skipped,
                          })}
                        </span>
                        {c.last_run.error_code && <span className="text-danger">{errorOf(c.last_run.error_code)}</span>}
                      </span>
                    ) : (
                      t.never
                    )}
                  </dd>
                  <dt className="text-muted">{t.nextRun}</dt>
                  <dd className="text-ink-2" title={when(c.next_run_at, locale, "")}>
                    {ago(c.next_run_at, locale, t.never)}
                  </dd>
                </dl>
              </li>
            ))}
          </ul>
        )}
      </Panel>

      {(open?.kind === "create" || open?.kind === "edit") && (
        <ConnectionDialog
          nodeId={nodeId}
          connection={open.kind === "edit" ? open.connection : null}
          labels={t}
          fail={fail}
          onClose={() => setOpen(null)}
          onSaved={() => {
            toast.success(open.kind === "edit" ? t.saved : t.created);
            setOpen(null);
            void load();
          }}
        />
      )}
      {open?.kind === "preview" && <PreviewDialog url={`${base}/${open.connection.id}/preview`} labels={t} fail={fail} onClose={() => setOpen(null)} />}
      {open?.kind === "runs" && (
        <RunsDialog url={`${base}/${open.connection.id}/runs`} locale={locale} labels={t} fail={fail} errorOf={errorOf} onClose={() => setOpen(null)} />
      )}
      {open?.kind === "webhook" && (
        <WebhookDialog
          url={`${base}/${open.connection.id}/webhook`}
          connection={open.connection}
          labels={t}
          fail={fail}
          onClose={() => setOpen(null)}
          onChanged={() => void load()}
        />
      )}
      {open?.kind === "delete" && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setOpen(null)}
          title={t.deleteTitle}
          text={format(t.confirmDelete, { owner: open.connection.owner_path })}
          confirm={t.delete}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `${base}/${open.connection.id}` as `/${string}`);
              toast.success(t.deleted);
              void load();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </div>
  );
}
