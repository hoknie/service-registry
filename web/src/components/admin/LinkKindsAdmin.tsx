"use client";

import { GripVertical, Link2, Pencil, Plus, Trash } from "lucide-react";
import { useCallback, useEffect, useRef, useState, type DragEvent } from "react";
import { toast } from "sonner";

import { localeNames, locales, type Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type Items, type LinkKind } from "@/lib/api";
import { copySource, moveKey, namesBody } from "@/lib/linkKindForm";

import { Glyph } from "../links/LinkIcons";
import { kindGlyph } from "../links/shared";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { IconPicker } from "../ui/IconPicker";
import { Message, type Note } from "../ui/Message";
import { PageHeader } from "../ui/Panel";
import { Segmented } from "../ui/Segmented";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";

type Labels = Messages["admin"]["linkKinds"];
type Props = { locale: Locale; labels: Labels };

type IconMode = "set" | "url";

type Draft = { mode: "create" | "edit"; key: string; names: Record<Locale, string>; icon: string; iconMode: IconMode; iconUrl: string };

const emptyNames = () => Object.fromEntries(locales.map((l) => [l, ""])) as Record<Locale, string>;

export function LinkKindsAdmin({ locale, labels: t }: Props) {
  const { errors, common } = useUiText();
  const [items, setItems] = useState<LinkKind[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const [deleting, setDeleting] = useState<LinkKind | null>(null);
  const [ordering, setOrdering] = useState(false);
  const [dragged, setDragged] = useState<string | null>(null);
  const before = useRef<LinkKind[] | null>(null);

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

  const saveOrder = async (next: LinkKind[], previous: LinkKind[]) => {
    setItems(next);
    setOrdering(true);
    try {
      const r = await apiSend<Items<LinkKind>>("PUT", "/v1/link-kinds", { keys: next.map((k) => k.key) });
      setItems(r.items);
      toast.success(t.reordered);
    } catch (e) {
      setItems(previous);
      toast.error(fail(e));
      if (errorCode(e) === "conflict.link_kind_order_stale") void load();
    } finally {
      setOrdering(false);
    }
  };

  const move = (from: number, to: number) => {
    if (!items || ordering) return;
    const keys = moveKey(
      items.map((k) => k.key),
      from,
      to,
    );
    const next = keys.map((key) => items.find((k) => k.key === key)!);
    void saveOrder(next, items);
  };

  const onDragStart = (e: DragEvent<HTMLElement>, key: string) => {
    if (!items || ordering) {
      e.preventDefault();
      return;
    }
    before.current = items;
    setDragged(key);
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", key);
    const row = (e.currentTarget as HTMLElement).closest("tr");
    if (row) e.dataTransfer.setDragImage(row, 16, 16);
  };

  const onDragOver = (e: DragEvent<HTMLTableRowElement>, key: string) => {
    if (!dragged || !items) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    if (key === dragged) return;
    const keys = items.map((k) => k.key);
    const next = moveKey(keys, keys.indexOf(dragged), keys.indexOf(key));
    setItems(next.map((k) => items.find((x) => x.key === k)!));
  };

  const onDragEnd = () => {
    const previous = before.current;
    before.current = null;
    setDragged(null);
    if (!previous || !items) return;
    if (previous.map((k) => k.key).join() === items.map((k) => k.key).join()) return;
    void saveOrder(items, previous);
  };

  const open = (k?: LinkKind) => {
    setNote(null);
    setDraft(
      k
        ? { mode: "edit", key: k.key, names: { ...k.names }, icon: k.icon, iconMode: k.icon_url ? "url" : "set", iconUrl: k.icon_url ?? "" }
        : { mode: "create", key: "", names: emptyNames(), icon: "link", iconMode: "set", iconUrl: "" },
    );
  };

  const save = async () => {
    if (!draft) return;
    setBusy(true);
    setNote(null);
    const body = {
      names: namesBody(draft.names),
      icon: draft.iconMode === "url" ? "link" : draft.icon,
      icon_url: draft.iconMode === "url" && draft.iconUrl.trim() ? draft.iconUrl.trim() : null,
    };
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

  const copy = draft ? copySource(draft.names) : "";
  const ownName = draft ? draft.names[locale].trim() : "";
  const preview = draft ? kindGlyph({ icon: draft.iconMode === "url" ? "link" : draft.icon, icon_url: draft.iconMode === "url" && draft.iconUrl.trim() ? draft.iconUrl.trim() : null }) : null;
  const ordered = [locale, ...locales.filter((l) => l !== locale)];

  return (
    <>
      <PageHeader
        glyph={Link2}
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
        <Table aria-busy={ordering || undefined}>
          <thead>
            <tr>
              <Th>
                <span className="sr-only">{t.table.order}</span>
              </Th>
              <Th>{t.table.icon}</Th>
              <Th>{t.table.key}</Th>
              <Th>{t.table.name}</Th>
              <Th>
                <span className="sr-only">{t.table.actions}</span>
              </Th>
            </tr>
          </thead>
          <tbody>
            {items.map((k, i) => (
              <Tr
                key={k.key}
                onDragOver={(e) => onDragOver(e, k.key)}
                onDrop={(e) => e.preventDefault()}
                className={dragged === k.key ? "opacity-50" : undefined}
              >
                <Td className="w-0">
                  <button
                    type="button"
                    draggable={!ordering}
                    aria-disabled={ordering || undefined}
                    onDragStart={(e) => onDragStart(e, k.key)}
                    onDragEnd={onDragEnd}
                    onKeyDown={(e) => {
                      const to = e.key === "ArrowUp" ? i - 1 : e.key === "ArrowDown" ? i + 1 : -1;
                      if (to < 0 || to >= items.length) return;
                      e.preventDefault();
                      move(i, to);
                    }}
                    title={t.drag}
                    aria-label={format(t.dragKey, { key: k.key })}
                    className={
                      ordering
                        ? "grid size-7 cursor-not-allowed place-items-center rounded-md text-muted"
                        : "motion-control grid size-7 cursor-grab place-items-center rounded-md text-muted hover:bg-surface-3 hover:text-ink active:cursor-grabbing"
                    }
                  >
                    <GripVertical aria-hidden="true" className="size-4" />
                  </button>
                </Td>
                <Td>
                  <span className="flex items-center gap-2 text-ink-2">
                    <Glyph icon={kindGlyph(k)} />
                    <span className="font-mono text-xs text-muted">{k.icon}</span>
                  </span>
                </Td>
                <Td>
                  <code className="text-ink">{k.key}</code>
                </Td>
                <Td className="text-ink">{k.names[locale]}</Td>
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
              <Button variant="primary" disabled={busy || !ownName} onClick={() => void save()}>
                {t.submit}
              </Button>
            </>
          }
        >
          <div className="grid gap-4">
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
            <div className="grid gap-1.5">
              <span className="text-sm font-medium text-ink-2">{t.icon}</span>
              <div className="flex flex-wrap items-center gap-2">
                <span className="grid size-9 place-items-center rounded-md border border-line bg-surface">
                  {preview && <Glyph key={JSON.stringify(preview)} icon={preview} size={20} />}
                </span>
                <Segmented
                  label={t.icon}
                  value={draft.iconMode}
                  onChange={(v) => setDraft({ ...draft, iconMode: v as IconMode })}
                  options={[
                    { value: "set", label: t.iconSet },
                    { value: "url", label: t.iconUrl },
                  ]}
                />
              </div>
              {draft.iconMode === "set" ? (
                <>
                  <IconPicker
                    value={draft.icon}
                    onChange={(icon) => setDraft({ ...draft, icon })}
                    labels={{ search: t.iconSearch, empty: t.iconEmpty, choose: t.iconChoose }}
                  />
                  <p className="text-xs text-muted">{t.iconSetHint}</p>
                </>
              ) : (
                <>
                  <Input
                    aria-label={t.iconUrl}
                    type="url"
                    placeholder="https://"
                    className="font-mono"
                    autoFocus
                    value={draft.iconUrl}
                    onChange={(e) => setDraft({ ...draft, iconUrl: e.target.value })}
                  />
                  <p className="text-xs text-muted">{t.iconUrlHint}</p>
                </>
              )}
            </div>
            <fieldset className="grid gap-3">
              <legend className="mb-1 text-sm font-medium text-ink-2">{t.names}</legend>
              <p className="-mt-1 text-xs text-muted">{t.namesHint}</p>
              {ordered.map((l) => (
                <Field key={l} label={l === locale ? `${localeNames[l]} *` : localeNames[l]}>
                  {(p) => (
                    <Input
                      {...p}
                      lang={l}
                      required={l === locale}
                      maxLength={100}
                      placeholder={l === locale ? undefined : copy}
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
