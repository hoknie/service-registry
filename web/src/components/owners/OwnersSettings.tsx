"use client";

import { ArrowDown, ArrowUp, Trash, User, UsersRound } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import type { CatalogNode, Owner, OwnerSet } from "@/lib/api";
import { catalogChanged } from "@/lib/catalogEvents";
import { draftOf, moveItem, sameSubject, type DraftOwner } from "@/lib/owners";
import { getOwners, putOwners } from "@/lib/ownersApi";

import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonRows } from "../ui/Skeleton";
import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { OwnerPicker } from "./OwnerPicker";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; canWrite: boolean; fail: (e: unknown) => string };

const ownOf = (set: OwnerSet): DraftOwner[] => set.items.filter((o) => !o.inherited).map(draftOf);

export function OwnersSettings({ node, locale, labels, canWrite, fail }: Props) {
  const t = labels.owners;
  const { common } = useUiText();
  const [set, setSet] = useState<OwnerSet | null>(null);
  const [draft, setDraft] = useState<DraftOwner[]>([]);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  const load = useCallback(async () => {
    try {
      const loaded = await getOwners(node.id);
      setSet(loaded);
      setDraft(ownOf(loaded));
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  }, [node.id, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const save = async () => {
    setBusy(true);
    try {
      const saved = await putOwners(node.id, draft);
      setSet(saved);
      setDraft(ownOf(saved));
      setNote(null);
      toast.success(t.saved);
      catalogChanged();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  if (!set) return note ? <Message note={note} /> : <SkeletonRows rows={3} label={common.loading} />;
  const inherited = set.items.filter((o) => o.inherited);
  const rows: DraftOwner[] = canWrite ? draft : ownOf(set);
  const taken = [...inherited.map(draftOf), ...rows];

  return (
    <Panel title={t.title} description={t.lead}>
      <div className="grid gap-4">
        {inherited.length + rows.length > 0 && (
          <ol className="divide-y divide-line rounded-lg border border-line">
            {inherited.map((o, i) => (
              <InheritedRow key={`${o.subject_kind}:${o.subject_id}`} owner={o} primary={i === 0} node={node} locale={locale} labels={labels} />
            ))}
            {rows.map((o, i) => {
              const Icon = o.subject_kind === "group" ? UsersRound : User;
              return (
                <li key={`${o.subject_kind}:${o.subject}`} className="flex flex-wrap items-center gap-3 px-3 py-2">
                  <Icon aria-label={o.subject_kind === "group" ? t.group : t.user} className="size-4 shrink-0 text-muted" />
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-sm font-medium text-ink">{o.name}</span>
                    {o.detail && <span className="block truncate text-xs text-muted">{o.detail}</span>}
                  </span>
                  {inherited.length === 0 && i === 0 && <Badge tone="signal">{t.primary}</Badge>}
                  <Badge tone="outline">{t.here}</Badge>
                  {canWrite && (
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
        {canWrite ? (
          <>
            <OwnerPicker
              nodeId={node.id}
              label={t.add}
              placeholder={t.addPlaceholder}
              membersLabel={t.members}
              orgLabel={t.orgGroup}
              exclude={taken}
              disabled={rows.length >= 10}
              onPick={(o) => !taken.some((r) => sameSubject(r, o)) && setDraft([...rows, o])}
            />
            <div className="flex flex-wrap gap-2">
              <Button variant="primary" busy={busy} onClick={() => void save()}>
                {t.save}
              </Button>
            </div>
          </>
        ) : (
          <p className="text-sm text-muted">{t.readOnly}</p>
        )}
        <Message note={note} />
      </div>
    </Panel>
  );
}

type RowProps = { owner: Owner; primary: boolean; node: CatalogNode; locale: Locale; labels: CatalogLabels };

function InheritedRow({ owner, primary, node, locale, labels }: RowProps) {
  const t = labels.owners;
  const Icon = owner.subject_kind === "group" ? UsersRound : User;
  const source = (node.path ?? []).find((p) => p.id === owner.node_id);
  return (
    <li className="flex flex-wrap items-center gap-3 px-3 py-2">
      <Icon aria-label={owner.subject_kind === "group" ? t.group : t.user} className="size-4 shrink-0 text-muted" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-sm font-medium text-ink">{owner.name}</span>
        {owner.email && <span className="block truncate text-xs text-muted">{owner.email}</span>}
      </span>
      {primary && <Badge tone="signal">{t.primary}</Badge>}
      {source && (
        <Link href={catalogHref(locale, source.id)} className="text-xs text-signal hover:underline">
          {format(t.inheritedFrom, { name: source.name })}
        </Link>
      )}
    </li>
  );
}
