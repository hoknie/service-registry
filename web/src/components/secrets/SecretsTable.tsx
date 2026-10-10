"use client";

import { KeyRound, Pencil, Plus, Trash } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, apiSend, errorCode, type Items, type Secret } from "@/lib/api";
import { secretFrom, secretsPath } from "@/lib/secrets";

import { catalogHref } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { Tooltip } from "../ui/Tooltip";
import { SecretDialog } from "./SecretDialog";

type Props = { locale: Locale; nodeId: string | null; canManage: boolean };

export function SecretsTable({ locale, nodeId, canManage }: Props) {
  const { secrets: t, errors, common } = useUiText();
  const [items, setItems] = useState<Secret[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<Secret | null | "new">(null);
  const [deleting, setDeleting] = useState<Secret | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const load = useCallback(async () => {
    try {
      setItems((await apiGet<Items<Secret>>(secretsPath(nodeId))).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [nodeId, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const own = (s: Secret) => canManage && s.own;
  return (
    <>
      <PageHeader
        glyph={KeyRound}
        title={t.title}
        lead={nodeId ? t.nodeIntro : t.globalIntro}
        actions={
          canManage && (
            <Button variant="primary" onClick={() => setEditing("new")}>
              <Plus aria-hidden="true" />
              {t.newSecret}
            </Button>
          )
        }
      />
      {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}
      {!items && !error && <SkeletonTable rows={3} label={common.loading} />}
      {items && items.length === 0 && <EmptyState icon={KeyRound} title={t.empty} />}
      {items && items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <Th>{t.name}</Th>
              <Th>{t.storage}</Th>
              <Th>{t.from}</Th>
              <Th numeric>{t.usedBy}</Th>
              <Th>
                <span className="sr-only">{common.actions}</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((s) => (
              <Tr key={s.id}>
                <Td>
                  <span className="block font-medium text-ink">{s.name}</span>
                  {s.description && <span className="block text-xs text-muted">{s.description}</span>}
                </Td>
                <Td className="text-ink-2">
                  {s.storage === "stored" ? (
                    <>
                      {t.stored} <code className="text-xs">{s.fingerprint}</code>
                    </>
                  ) : (
                    <>
                      {t.reference} <code className="text-xs">{s.ref}</code>
                    </>
                  )}
                </Td>
                <Td className="text-ink-2">
                  {!s.own && s.from ? (
                    <Link href={catalogHref(locale, s.from.id, "settings", null, null, "secrets")} className="text-signal hover:underline">
                      {secretFrom(s, t)}
                    </Link>
                  ) : (
                    secretFrom(s, t)
                  )}
                </Td>
                <Td numeric className="text-ink-2">
                  {s.used_by}
                </Td>
                <Td className="text-right whitespace-nowrap">
                  {own(s) && (
                    <>
                      <Button size="sm" variant="ghost" onClick={() => setEditing(s)}>
                        <Pencil aria-hidden="true" />
                        {t.edit}
                      </Button>
                      <Tooltip content={s.used_by > 0 ? format(t.inUse, { n: String(s.used_by) }) : null}>
                        <span>
                          <Button size="sm" variant="danger-ghost" disabled={s.used_by > 0} onClick={() => setDeleting(s)}>
                            <Trash aria-hidden="true" />
                            {t.delete}
                          </Button>
                        </span>
                      </Tooltip>
                    </>
                  )}
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
      {editing && (
        <SecretDialog
          nodeId={nodeId}
          secret={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null);
            void load();
          }}
        />
      )}
      {deleting && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setDeleting(null)}
          title={format(t.deleteTitle, { name: deleting.name })}
          text={t.deleteText}
          confirm={t.delete}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `${secretsPath(nodeId)}/${deleting.id}` as `/${string}`);
              toast.success(t.deleted);
              setDeleting(null);
              void load();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </>
  );
}
