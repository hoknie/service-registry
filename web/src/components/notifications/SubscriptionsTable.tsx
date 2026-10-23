"use client";

import { Bell, EllipsisVertical, Plus } from "lucide-react";
import Link from "next/link";
import { useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { localeNames } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ApiError, errorCode, type CatalogNode } from "@/lib/api";
import { bodyOf, type Subscription } from "@/lib/notifications";
import { deleteSubscription, putSubscription, testSubscription } from "@/lib/notificationsApi";

import { catalogHref } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Menu, MenuContent, MenuItem, MenuTrigger } from "../ui/Menu";
import { Table, Td, Th, Tr } from "../ui/Table";
import { SubscriptionDialog, type DialogMode } from "./SubscriptionDialog";

type Labels = Messages["notify"];
type Props = { node: CatalogNode; locale: Locale; labels: Labels; items: Subscription[]; reload: () => void };

type CodeBook = { [k: string]: string | CodeBook };

export function codeText(t: Labels, fallback: string, code: string | null): string {
  let node: string | CodeBook | undefined = t.codes as unknown as CodeBook;
  for (const part of (code ?? "").split(".")) node = typeof node === "object" ? node[part] : undefined;
  return typeof node === "string" ? node : (code ?? fallback);
}

export function SubscriptionsTable({ node, locale, labels: t, items, reload }: Props) {
  const { errors, common } = useUiText();
  const [dialog, setDialog] = useState<DialogMode | null>(null);
  const [removing, setRemoving] = useState<Subscription | null>(null);
  const pathName = (id: string) => (node.path ?? []).find((p) => p.id === id)?.name ?? id;

  const sendTest = async (s: Subscription) => {
    try {
      const r = await testSubscription(node.id, s.key);
      if (r.status === "delivered") toast.success(format(t.test.delivered, { status: String(r.http_status ?? ""), ms: String(r.duration_ms) }));
      else {
        const text = codeText(t, errors.unknown, r.error_code);
        toast.error(r.http_status ? format(t.test.failed, { text, status: String(r.http_status) }) : format(t.test.failedNoStatus, { text }));
      }
      reload();
    } catch (e) {
      if (e instanceof ApiError && e.status === 429) toast.error(format(t.test.tooOften, { n: "10" }));
      else toast.error(errorText(errors, errorCode(e)));
    }
  };

  const disableHere = async (s: Subscription) => {
    try {
      await putSubscription(node.id, s.key, bodyOf(s, { disabled: true }));
      toast.success(t.saved);
      reload();
    } catch (e) {
      toast.error(errorText(errors, errorCode(e)));
    }
  };

  return (
    <div className="grid gap-4">
      <div className="flex justify-end">
        <Button variant="primary" onClick={() => setDialog({ kind: "create" })}>
          <Plus aria-hidden="true" />
          {t.add}
        </Button>
      </div>
      {items.length === 0 && <EmptyState icon={Bell} title={t.empty} text={t.emptyText} />}
      {items.length > 0 && (
        <div className="@container min-w-0">
          <Table>
            <thead>
              <tr>
                <Th>{t.columns.key}</Th>
                <Th>{t.columns.channel}</Th>
                <Th className="hidden @2xl:table-cell">{t.columns.secret}</Th>
                <Th>{t.columns.events}</Th>
                <Th className="hidden @3xl:table-cell">{t.columns.environments}</Th>
                <Th className="hidden @3xl:table-cell">{t.columns.locale}</Th>
                <Th>{t.columns.state}</Th>
                <Th className="hidden @xl:table-cell">{t.columns.source}</Th>
                <Th>
                  <span className="sr-only">{common.actions}</span>
                </Th>
              </tr>
            </thead>
            <tbody>
              {items.map((s) => (
                <Tr key={s.key}>
                  <Td className="font-mono text-ink">{s.key}</Td>
                  <Td>{t.channels[s.channel]}</Td>
                  <Td className="hidden text-ink-2 @2xl:table-cell">{s.secret?.name ?? t.noSecret}</Td>
                  <Td>
                    <span className="flex flex-wrap gap-1">
                      {s.event_types.map((e) => (
                        <Badge key={e} tone="neutral">
                          {t.events[e]}
                        </Badge>
                      ))}
                    </span>
                  </Td>
                  <Td className="hidden font-mono text-xs text-ink-2 @3xl:table-cell">{s.environments?.join(", ") ?? t.allEnvironments}</Td>
                  <Td className="hidden @3xl:table-cell">{localeNames[s.locale as Locale] ?? s.locale}</Td>
                  <Td>
                    <Badge tone={s.disabled ? "outline" : "signal"}>{s.disabled ? t.disabled : t.enabled}</Badge>
                  </Td>
                  <Td className="hidden @xl:table-cell">
                    {s.inherited ? (
                      <Link href={catalogHref(locale, s.node_id, "settings", null, null, "notifications")} className="text-signal hover:underline">
                        {pathName(s.node_id)}
                      </Link>
                    ) : (
                      t.here
                    )}
                  </Td>
                  <Td className="text-right">
                    <Menu>
                      <MenuTrigger asChild>
                        <Button size="icon" variant="ghost" aria-label={format(t.actions.menu, { key: s.key })}>
                          <EllipsisVertical aria-hidden="true" />
                        </Button>
                      </MenuTrigger>
                      <MenuContent>
                        <MenuItem onSelect={() => void sendTest(s)}>{t.actions.test}</MenuItem>
                        {s.inherited ? (
                          <>
                            <MenuItem onSelect={() => setDialog({ kind: "override", sub: s })}>{t.actions.override}</MenuItem>
                            <MenuItem onSelect={() => void disableHere(s)}>{t.actions.disableHere}</MenuItem>
                          </>
                        ) : (
                          <>
                            <MenuItem onSelect={() => setDialog({ kind: "edit", sub: s })}>{t.actions.edit}</MenuItem>
                            <MenuItem danger onSelect={() => setRemoving(s)}>
                              {t.actions.remove}
                            </MenuItem>
                          </>
                        )}
                      </MenuContent>
                    </Menu>
                  </Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        </div>
      )}
      {dialog && (
        <SubscriptionDialog nodeId={node.id} nodeKind={node.kind} locale={locale} labels={t} mode={dialog} onClose={() => setDialog(null)} onSaved={reload} />
      )}
      {removing && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setRemoving(null)}
          title={format(t.removeTitle, { key: removing.key })}
          text={t.removeText}
          confirm={t.removeConfirm}
          onConfirm={async () => {
            try {
              await deleteSubscription(node.id, removing.key);
              toast.success(t.removed);
              setRemoving(null);
              reload();
              return null;
            } catch (e) {
              return errorText(errors, errorCode(e));
            }
          }}
        />
      )}
    </div>
  );
}
