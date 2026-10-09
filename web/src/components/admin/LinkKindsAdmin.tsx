"use client";

import { Link2, Pencil, Plus, Trash } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import { localeNames, locales, type Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, LINK_ICONS, type Items, type LinkIcon as Icon, type LinkKind } from "@/lib/api";

import { LinkIcon } from "../links/LinkIcon";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";

type Labels = Messages["admin"]["linkKinds"];
type Props = { locale: Locale; labels: Labels };

type Draft = { mode: "create" | "edit"; key: string; names: Record<Locale, string>; icon: Icon; position: string };

const emptyNames = () => Object.fromEntries(locales.map((l) => [l, ""])) as Record<Locale, string>;

export function LinkKindsAdmin({ locale, labels: t }: Props) {
  const { errors, common } = useUiText();
  const [items, setItems] = useState<LinkKind[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const [deleting, setDeleting] = useState<LinkKind | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  const load = useCallback(async () => {
    try {
      setItems((await apiGet<Items<LinkKind>>("/v1/link-kinds")).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const open = (k?: LinkKind) => {
    setNote(null);
    setDraft(
      k
        ? { mode: "edit", key: k.key, names: { ...k.names }, icon: k.icon, position: String(k.position) }
        : { mode: "create", key: "", names: emptyNames(), icon: "link", position: "0" },
    );
  };

  const save = async () => {
    if (!draft) return;
    setBusy(true);
    setNote(null);
    const position = Number(draft.position);
    const body = { names: draft.names, icon: draft.icon, position: Number.isInteger(position) ? position : -1 };
    try {
      if (draft.mode === "create") {
        await apiSend("POST", "/v1/link-kinds", { key: draft.key, ...body });
        toast.success(t.created);
      } else {
        await apiSend("PATCH", `/v1/link-kinds/${encodeURIComponent(draft.key)}` as `/${string}`, body);
        toast.success(t.saved);
      }
      setDraft(null);
      void load();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <PageHeader glyph={Link2}
        title={t.title}
        lead={t.lead}
        actions={
          <Button variant="primary" onClick={() => open()}>
            <Plus aria-hidden="true" />
            {t.add}
          </Button>
        }
      />
      {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}
      {!items && !error && <SkeletonTable rows={7} label={common.loading} />}
      {items && items.length === 0 && <EmptyState icon={Link2} title={t.empty} />}
      {items && items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <Th>{t.table.icon}</Th>
              <Th>{t.table.key}</Th>
              <Th>{t.table.name}</Th>
              <Th numeric>{t.table.position}</Th>
              <Th>
                <span className="sr-only">{t.table.actions}</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((k) => (
              <Tr key={k.key}>
                <Td>
                  <span className="flex items-center gap-2 text-ink-2">
                    <LinkIcon icon={k.icon} className="text-ink-2" />
                    <span className="text-xs text-muted">{t.icons[k.icon] ?? k.icon}</span>
                  </span>
                </Td>
                <Td>
                  <code className="text-ink">{k.key}</code>
                </Td>
                <Td className="text-ink">{k.names[locale]}</Td>
                <Td numeric className="text-ink-2">{k.position}</Td>
                <Td className="text-right whitespace-nowrap">
                  <Button size="sm" variant="ghost" onClick={() => open(k)}>
                    <Pencil aria-hidden="true" />
                    {t.edit}
                  </Button>
                  <Button size="sm" variant="danger-ghost" onClick={() => setDeleting(k)}>
                    <Trash aria-hidden="true" />
                    {t.delete}
                  </Button>
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
      {draft && (
        <Dialog
          open
          onOpenChange={(o) => !o && setDraft(null)}
          title={draft.mode === "edit" ? format(t.editTitle, { key: draft.key }) : t.createTitle}
          footer={
            <>
              <Button onClick={() => setDraft(null)}>{common.cancel}</Button>
              <Button variant="primary" disabled={busy} onClick={() => void save()}>
                {t.submit}
              </Button>
            </>
          }
        >
          <div className="grid gap-4">
            <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_7rem]">
              <Field label={t.key} hint={draft.mode === "create" ? t.keyHint : undefined}>
                {(p) => (
                  <Input
                    {...p}
                    spellCheck={false}
                    className="font-mono"
                    readOnly={draft.mode === "edit"}
                    value={draft.key}
                    onChange={(e) => setDraft({ ...draft, key: e.target.value })}
                  />
                )}
              </Field>
              <Field label={t.icon}>
                {(p) => (
                  <Select {...p} value={draft.icon} onChange={(e) => setDraft({ ...draft, icon: e.target.value as Icon })}>
                    {LINK_ICONS.map((i) => (
                      <option key={i} value={i}>
                        {t.icons[i]}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
              <Field label={t.position}>
                {(p) => (
                  <Input {...p} type="number" min={0} max={10000} value={draft.position} onChange={(e) => setDraft({ ...draft, position: e.target.value })} />
                )}
              </Field>
            </div>
            <fieldset className="grid gap-3">
              <legend className="mb-1 text-sm font-medium text-ink-2">{t.names}</legend>
              {locales.map((l) => (
                <Field key={l} label={localeNames[l]}>
                  {(p) => (
                    <Input
                      {...p}
                      lang={l}
                      required
                      maxLength={100}
                      value={draft.names[l]}
                      onChange={(e) => setDraft({ ...draft, names: { ...draft.names, [l]: e.target.value } })}
                    />
                  )}
                </Field>
              ))}
            </fieldset>
            {note && <Message note={note} />}
          </div>
        </Dialog>
      )}
      {deleting && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setDeleting(null)}
          title={t.deleteTitle}
          text={format(t.confirmDelete, { key: deleting.key })}
          confirm={t.delete}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `/v1/link-kinds/${encodeURIComponent(deleting.key)}` as `/${string}`);
              toast.success(t.deleted);
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
