"use client";

import { Pencil, Variable } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { apiGet, apiSend, errorCode, type CatalogNode, type Items, type NodeVar } from "@/lib/api";

import { catalogHref, labelsText, parseLabels, type CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Field } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { hasInvalidTags, KeyValueInput, keyValueError } from "../ui/TagInput";

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; canWrite: boolean; onChanged: () => void };

export function VarsEditor({ node, locale, labels, canWrite, onChanged }: Props) {
  const { errors, common } = useUiText();
  const t = labels.links.vars;
  const [items, setItems] = useState<NodeVar[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const base = `/v1/catalog/nodes/${node.id}/vars` as const;

  const load = useCallback(async () => {
    try {
      setItems((await apiGet<Items<NodeVar>>(base)).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [base, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const names = new Map((node.path ?? []).map((p) => [p.id, p.name]));
  const own = Object.fromEntries((items ?? []).filter((v) => !v.inherited).map((v) => [v.key, v.value]));

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      await apiSend("PUT", base, { vars: parseLabels(draft) });
      toast.success(t.saved);
      setEditing(false);
      void load();
      onChanged();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Panel
      title={t.title}
      description={t.lead}
      actions={
        canWrite && (
          <Button
            size="sm"
            onClick={() => {
              setDraft(labelsText(own));
              setNote(null);
              setEditing(true);
            }}
          >
            <Pencil aria-hidden="true" />
            {t.edit}
          </Button>
        )
      }
    >
      {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}
      {!items && !error && <SkeletonTable rows={2} label={common.loading} />}
      {items && items.length === 0 && <EmptyState icon={Variable} title={t.empty} />}
      {items && items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <Th>{t.table.key}</Th>
              <Th>{t.table.value}</Th>
              <Th>{t.table.source}</Th>
            </tr>
          </thead>
          <tbody>
            {items.map((v) => (
              <Tr key={v.key}>
                <Td>
                  <code className="text-ink">{v.key}</code>
                </Td>
                <Td className="font-mono text-sm break-all text-ink-2">{v.value}</Td>
                <Td className="text-ink-2">
                  {v.inherited ? (
                    <Link href={catalogHref(locale, v.node_id, "links")} className="hover:underline">
                      {names.get(v.node_id) ?? v.node_id}
                    </Link>
                  ) : (
                    labels.links.templates.here
                  )}
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
      {editing && (
        <Dialog
          open
          onOpenChange={setEditing}
          title={t.editTitle}
          footer={
            <>
              <Button onClick={() => setEditing(false)}>{common.cancel}</Button>
              <Button variant="primary" busy={busy} disabled={hasInvalidTags(draft, (tag) => keyValueError(tag, "key=value"))} onClick={() => void save()}>
                {t.submit}
              </Button>
            </>
          }
        >
          <div className="grid gap-3">
            <Field label={t.title} hint={t.editHint}>
              {(p) => <KeyValueInput {...p} value={draft} onChange={setDraft} />}
            </Field>
            {note && <Message note={note} />}
          </div>
        </Dialog>
      )}
    </Panel>
  );
}
