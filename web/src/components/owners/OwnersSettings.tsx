"use client";

import { ArrowDown, ArrowUp, Trash, User, UsersRound } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { CatalogNode, OwnerSet } from "@/lib/api";
import { catalogChanged } from "@/lib/catalogEvents";
import { draftOf, moveItem, sameSubject, type DraftOwner } from "@/lib/owners";
import { clearOwners, getOwners, putOwners } from "@/lib/ownersApi";

import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonRows } from "../ui/Skeleton";
import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { OwnerPicker } from "./OwnerPicker";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; canWrite: boolean; fail: (e: unknown) => string };

export function OwnersSettings({ node, locale, labels, canWrite, fail }: Props) {
  const t = labels.owners;
  const { common } = useUiText();
  const [set, setSet] = useState<OwnerSet | null>(null);
  const [draft, setDraft] = useState<DraftOwner[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [confirming, setConfirming] = useState(false);

  const load = useCallback(async () => {
    try {
      const loaded = await getOwners(node.id);
      setSet(loaded);
      setDraft(canWrite && loaded.own && !loaded.none ? loaded.items.map(draftOf) : null);
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  }, [node.id, canWrite, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const apply = async (send: () => Promise<OwnerSet>): Promise<string | null> => {
    setBusy(true);
    try {
      const saved = await send();
      setSet(saved);
      setDraft(saved.own && !saved.none ? saved.items.map(draftOf) : null);
      setNote(null);
      toast.success(t.saved);
      catalogChanged();
      return null;
    } catch (e) {
      const text = fail(e);
      setNote({ kind: "error", text });
      return text;
    } finally {
      setBusy(false);
    }
  };
  const save = (next: DraftOwner[]) => void apply(() => putOwners(node.id, next));

  if (!set) return note ? <Message note={note} /> : <SkeletonRows rows={3} label={common.loading} />;
  const from = set.from && !set.own ? (node.path ?? []).find((p) => p.id === set.from) : undefined;
  const editing = canWrite && draft !== null;
  const rows: DraftOwner[] = editing ? draft : set.items.map(draftOf);
  const canClear = canWrite && !(set.own && set.none) && (set.from !== null || rows.length > 0);
  const clearButton = canClear && (
    <Button variant="danger-ghost" busy={busy} onClick={() => setConfirming(true)}>
      {t.clear}
    </Button>
  );

  return (
    <Panel
      title={t.title}
      description={t.lead}
      actions={
        from ? (
          <Link href={catalogHref(locale, from.id)} className="text-sm text-signal hover:underline">
            {format(t.inheritedFrom, { name: from.name })}
          </Link>
        ) : (
          set.own && <Badge tone="signal">{t.here}</Badge>
        )
      }
    >
      <div className="grid gap-4">
        {rows.length === 0 ? (
          <p className="rounded-md border border-dashed border-line-strong px-4 py-5 text-center text-sm text-muted">
            {set.none && !editing ? t.noOwners : editing ? t.empty : t.noneAbove}
          </p>
        ) : (
          <ol className="divide-y divide-line rounded-lg border border-line">
            {rows.map((o, i) => {
              const Icon = o.subject_kind === "group" ? UsersRound : User;
              return (
                <li key={`${o.subject_kind}:${o.subject}`} className="flex flex-wrap items-center gap-3 px-3 py-2">
                  <Icon aria-label={o.subject_kind === "group" ? t.group : t.user} className="size-4 shrink-0 text-muted" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-ink">{o.name}</span>
                    {o.detail && <span className="block truncate text-xs text-muted">{o.detail}</span>}
                  </span>
                  {i === 0 && <Badge tone="signal">{t.primary}</Badge>}
                  {editing && (
                    <span className="flex items-center gap-1">
                      <Button size="icon" variant="ghost" aria-label={`${t.up}: ${o.name}`} disabled={i === 0} onClick={() => setDraft(moveItem(rows, i, -1))}>
                        <ArrowUp aria-hidden="true" />
                      </Button>
                      <Button
                        size="icon"
                        variant="ghost"
                        aria-label={`${t.down}: ${o.name}`}
                        disabled={i === rows.length - 1}
                        onClick={() => setDraft(moveItem(rows, i, 1))}
                      >
                        <ArrowDown aria-hidden="true" />
                      </Button>
                      <Button size="icon" variant="danger-ghost" aria-label={`${t.remove}: ${o.name}`} onClick={() => setDraft(rows.filter((_, j) => j !== i))}>
                        <Trash aria-hidden="true" />
                      </Button>
                    </span>
                  )}
                </li>
              );
            })}
          </ol>
        )}
        {editing ? (
          <>
            <OwnerPicker
              nodeId={node.id}
              label={t.add}
              placeholder={t.addPlaceholder}
              disabled={rows.length >= 10}
              onPick={(o) => !rows.some((r) => sameSubject(r, o)) && setDraft([...rows, o])}
            />
            <div className="flex flex-wrap gap-2">
              <Button variant="primary" busy={busy} onClick={() => save(rows)}>
                {t.save}
              </Button>
              {set.own && (
                <Button busy={busy} onClick={() => save([])}>
                  {t.inherit}
                </Button>
              )}
              {clearButton}
            </div>
          </>
        ) : canWrite ? (
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => setDraft(set.items.map(draftOf))}>{t.own}</Button>
            {set.own && (
              <Button busy={busy} onClick={() => save([])}>
                {t.inherit}
              </Button>
            )}
            {clearButton}
          </div>
        ) : (
          <p className="text-sm text-muted">{t.readOnly}</p>
        )}
        <Message note={note} />
      </div>
      <ConfirmDialog
        open={confirming}
        onOpenChange={setConfirming}
        title={t.clear}
        text={t.clearConfirm}
        confirm={t.clear}
        onConfirm={() => apply(() => clearOwners(node.id))}
      />
    </Panel>
  );
}
