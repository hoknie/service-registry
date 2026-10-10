"use client";

import { useState } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { apiSend, errorCode, type Secret } from "@/lib/api";
import { secretBody, secretsPath, type SecretDraft } from "@/lib/secrets";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Segmented } from "../ui/Segmented";

type Props = {
  nodeId: string | null;
  secret: Secret | null;
  onClose: () => void;
  onSaved: (s: Secret) => void;
};

export function SecretDialog({ nodeId, secret, onClose, onSaved }: Props) {
  const { secrets: t, errors, common } = useUiText();
  const editing = secret !== null;
  const [draft, setDraft] = useState<SecretDraft>({
    name: secret?.name ?? "",
    description: secret?.description ?? "",
    mode: secret?.storage === "reference" ? "ref" : "value",
    value: "",
    ref: secret?.ref ?? "",
  });
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      const path = secretsPath(nodeId);
      const saved = editing
        ? await apiSend<Secret>("PATCH", `${path}/${secret.id}` as `/${string}`, secretBody(draft, true))
        : await apiSend<Secret>("POST", path, secretBody(draft, false));
      toast.success(editing ? t.updated : t.created);
      onSaved(saved);
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={editing ? t.editTitle : t.createTitle}
      footer={
        <>
          <Button onClick={onClose}>{common.cancel}</Button>
          <Button variant="primary" busy={busy} onClick={() => void save()}>
            {t.save}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <Field label={t.name}>
          {(p) => <Input {...p} required maxLength={100} value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />}
        </Field>
        <Field label={t.description} hint={t.descriptionHint}>
          {(p) => <Input {...p} maxLength={500} value={draft.description} onChange={(e) => setDraft({ ...draft, description: e.target.value })} />}
        </Field>
        <Segmented
          label={t.mode}
          value={draft.mode}
          options={[
            { value: "value", label: t.modeValue },
            { value: "ref", label: t.modeRef },
          ]}
          onChange={(v) => setDraft({ ...draft, mode: v === "ref" ? "ref" : "value" })}
        />
        {draft.mode === "value" ? (
          <Field label={t.value} hint={editing ? t.valueKeep : undefined}>
            {(p) => (
              <Input {...p} type="password" autoComplete="off" required={!editing} value={draft.value} onChange={(e) => setDraft({ ...draft, value: e.target.value })} />
            )}
          </Field>
        ) : (
          <Field label={t.ref} hint={t.refHint}>
            {(p) => <Input {...p} spellCheck={false} className="font-mono" required value={draft.ref} onChange={(e) => setDraft({ ...draft, ref: e.target.value })} />}
          </Field>
        )}
        <Message note={note} />
      </div>
    </Dialog>
  );
}
