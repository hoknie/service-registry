"use client";

import { BookOpen, RefreshCw, Settings2 } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { when } from "@/i18n/time";
import { apiGet, apiSend, errorCode, type CatalogNode, type Knowledge, type KnowledgeFiles } from "@/lib/api";
import { shortCommit } from "@/lib/commit";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Field } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { DocsTree } from "./DocsTree";
import { DocViewer } from "./DocViewer";

type Props = {
  node: CatalogNode;
  branch: string | null;
  doc: string | null;
  locale: Locale;
  labels: CatalogLabels;
  canWrite: boolean;
  onOpen: (branch: string | null, doc: string | null) => void;
  reload?: number;
  onCollect?: () => void;
};

const statusTone = { ok: "signal", partial: "amber", failed: "danger" } as const;

export function DocsTab({ node, branch: branchParam, doc, locale, labels, canWrite, onOpen, reload = 0, onCollect }: Props) {
  const { errors, common } = useUiText();
  const t = labels.docs;
  const [overview, setOverview] = useState<Knowledge | null>(null);
  const [files, setFiles] = useState<KnowledgeFiles | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const base = `/v1/catalog/nodes/${node.id}/knowledge`;
  const branch = branchParam ?? overview?.branches[0]?.name ?? node.default_branch ?? null;

  const load = useCallback(async () => {
    try {
      setOverview(await apiGet<Knowledge>(base as `/${string}`));
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [base, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load, reload]);

  useEffect(() => {
    if (!overview?.source || !branch) return;
    let live = true;
    apiGet<KnowledgeFiles>(`${base}/files?branch=${encodeURIComponent(branch)}` as `/${string}`)
      .then((f) => live && setFiles(f))
      .catch(() => live && setFiles(null));
    return () => {
      live = false;
    };
  }, [overview, branch, base]);

  const collect = async () => {
    setBusy(true);
    try {
      await apiSend("POST", `${base}/collect` as `/${string}`);
      onCollect?.();
      toast.success(t.collected);
      await load();
    } catch (e) {
      toast.error(fail(e));
    } finally {
      setBusy(false);
    }
  };

  if (error) return <Message note={{ kind: "error", text: error }} />;
  if (!overview) return <SkeletonPanel lines={4} label={common.loading} />;
  const toSettings = (
    <Link href={catalogHref(locale, node.id, "docs-settings")} className="inline-flex items-center gap-1.5 text-sm font-medium text-signal hover:underline">
      <Settings2 aria-hidden="true" className="size-4" />
      {t.toSettings}
    </Link>
  );
  if (!overview.source) {
    return (
      <div className="grid justify-items-center gap-3">
        <EmptyState icon={BookOpen} title={t.noSource} />
        {toSettings}
      </div>
    );
  }

  const state = overview.branches.find((b) => b.name === branch) ?? null;
  const snapshot = state?.snapshot ?? null;
  const failed = state?.last?.status === "failed" ? state.last : null;
  const readme = files?.items.find((f) => !f.path.includes("/") && /^readme(\..+)?$/i.test(f.path));
  const open = doc ?? readme?.path ?? null;

  const actions = canWrite && (
    <Button onClick={() => void collect()} disabled={busy}>
      <RefreshCw aria-hidden="true" className={busy ? "animate-spin" : undefined} />
      {t.collect}
    </Button>
  );

  return (
    <div className="grid gap-6">
      <Panel title={labels.tabs.docs} description={t.lead} actions={actions}>
        <div className="flex flex-wrap items-end gap-x-6 gap-y-3 text-sm">
          {overview.branches.length > 1 && (
            <Field label={t.branch} className="min-w-48">
              {(p) => (
                <Select {...p} value={branch ?? ""} onChange={(e) => onOpen(e.target.value === overview.branches[0]?.name ? null : e.target.value, null)}>
                  {overview.branches.map((b) => (
                    <option key={b.name} value={b.name}>
                      {b.name}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          )}
          {snapshot ? (
            <dl className="flex flex-wrap items-center gap-x-6 gap-y-2">
              <div className="flex gap-2">
                <dt className="text-muted">{t.commit}</dt>
                <dd className="flex items-center gap-2 font-mono text-ink">
                  {shortCommit(snapshot.commit).short}
                  {shortCommit(snapshot.commit).worktree && (
                    <Badge tone="amber" className="font-sans">
                      {t.workingTree}
                    </Badge>
                  )}
                </dd>
              </div>
              <div className="flex gap-2">
                <dt className="text-muted">{t.collectedAt}</dt>
                <dd className="text-ink">{when(snapshot.collected_at, locale, "—")}</dd>
              </div>
              <dd>
                <Badge tone={statusTone[snapshot.status]}>{t.status[snapshot.status]}</Badge>
              </dd>
            </dl>
          ) : (
            <p className="text-muted">{state?.pending ? t.pending : t.never}</p>
          )}
        </div>
        {snapshot?.status === "partial" && <p className="mt-3 text-sm text-muted">{snapshot.truncated ? t.truncated : t.partialHint}</p>}
        {overview.settings.include === null && (
          <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md bg-surface-2 px-3 py-2 text-sm text-ink-2">
            <span>{t.onlyReadme}</span>
            {toSettings}
          </div>
        )}
        {failed && (
          <Message className="mt-3" note={{ kind: "error", text: format(t.failed, { error: errorText(errors, failed.error_code ?? "unknown") }) }} />
        )}
      </Panel>

      {snapshot && files && (
        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,18rem)_minmax(0,1fr)]">
          <Panel title={t.files} className="min-w-0 overflow-hidden" bodyClassName="p-2 sm:p-2">
            {files.items.length === 0 ? (
              <p className="p-3 text-sm text-muted">{t.filesEmpty}</p>
            ) : (
              <DocsTree files={files.items} open={open} labels={t} onOpen={(path) => onOpen(branchParam, path)} />
            )}
          </Panel>
          {open ? (
            <DocViewer
              key={`${branch}:${open}`}
              node={node}
              branch={branch ?? ""}
              path={open}
              paths={files.items.map((f) => f.path)}
              docHref={(path) => catalogHref(locale, node.id, "docs", branchParam, path)}
              onOpenDoc={(path) => onOpen(branchParam, path)}
              labels={t}
            />
          ) : (
            <EmptyState icon={BookOpen} title={t.noFile} />
          )}
        </div>
      )}

    </div>
  );
}
