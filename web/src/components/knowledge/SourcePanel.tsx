"use client";

import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, apiSend, errorCode, type KnowledgeSource, type SourceCheck, type SourceKind } from "@/lib/api";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Field } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import type { CatalogLabels } from "../catalog/shared";
import { draftOf, emptyDraft, SOURCE_KINDS, SourceFields, sourceBody, type SourceDraft } from "./SourceFields";

type Props = {
  projectId: string;
  synced: boolean;
  canWrite: boolean;
  labels: CatalogLabels["docs"]["source"];
  onChanged: () => void;
};

export function SourcePanel({ projectId, synced, canWrite, labels: t, onChanged }: Props) {
  const { errors } = useUiText();
  const base = `/v1/catalog/nodes/${projectId}/knowledge/source` as const;
  const [source, setSource] = useState<KnowledgeSource | null | undefined>(undefined);
  const [draft, setDraft] = useState<SourceDraft>(emptyDraft);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const [removing, setRemoving] = useState(false);

  useEffect(() => {
    let live = true;
    apiGet<KnowledgeSource | null>(base)
      .then((s) => {
        if (!live) return;
        setSource(s);
        setDraft(draftOf(s));
      })
      .catch(() => live && setSource(null));
    return () => {
      live = false;
    };
  }, [base]);

  const body = () => sourceBody(draft);

  const check = async () => {
    setBusy(true);
    setNote(null);
    try {
      const res = await apiSend<SourceCheck>("POST", `${base}/check`, body());
      setNote(
        res.ok
          ? { kind: "success", text: format(t.checkOk, { n: res.branches ?? 0 }) }
          : { kind: "error", text: errorText(errors, res.error_code ?? "unknown") },
      );
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      const saved = await apiSend<KnowledgeSource>("PUT", base, body());
      setSource(saved);
      setDraft({ ...draft, token: "" });
      toast.success(t.saved);
      onChanged();
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const current =
    source === undefined
      ? null
      : source
        ? `${t.kinds[source.kind]}: ${source.url ?? source.path ?? ""} · ${
            source.kind === "local_git"
              ? source.include_ignored
                ? t.includeIgnoredOn
                : source.working_tree
                  ? t.workingTreeOn
                  : t.workingTreeOff
              : source.credentials.mode === "stored"
                ? format(t.credentials.stored, { fingerprint: source.credentials.fingerprint ?? "" })
                : t.credentials[source.credentials.mode]
          }`
        : synced
          ? t.forgeSync
          : t.none;

  return (
    <Panel title={t.title} description={t.lead}>
      {current && <p className="mb-4 text-sm break-all text-ink-2">{current}</p>}
      {canWrite && (
        <form className="grid max-w-2xl gap-4" onSubmit={save}>
          <Field label={t.kind}>
            {(p) => (
              <Select {...p} value={draft.kind} onChange={(e) => setDraft({ ...draft, kind: e.target.value as SourceKind })}>
                {SOURCE_KINDS.map((k) => (
                  <option key={k} value={k}>
                    {t.kinds[k]}
                  </option>
                ))}
              </Select>
            )}
          </Field>
          <SourceFields value={draft} onChange={setDraft} labels={t} />
          <Message note={note} />
          <div className="flex flex-wrap gap-2">
            <Button type="submit" variant="primary" disabled={busy}>
              {t.save}
            </Button>
            <Button type="button" onClick={() => void check()} disabled={busy}>
              {t.check}
            </Button>
            {source && (
              <Button type="button" variant="ghost" onClick={() => setRemoving(true)}>
                {t.remove}
              </Button>
            )}
          </div>
        </form>
      )}
      {removing && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setRemoving(false)}
          title={t.removeTitle}
          text={t.confirmRemove}
          confirm={t.remove}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", base);
              setSource(null);
              toast.success(t.removed);
              onChanged();
              return null;
            } catch (e) {
              return errorText(errors, errorCode(e));
            }
          }}
        />
      )}
    </Panel>
  );
}
