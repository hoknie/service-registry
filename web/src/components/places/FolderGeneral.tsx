"use client";

import { FolderInput, Trash2 } from "lucide-react";
import { useState } from "react";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiSend, errorCode } from "@/lib/api";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Field, Input } from "../ui/Field";
import { DangerZone } from "./DangerZone";
import { MarkdownField } from "./MarkdownField";
import { PlaceMoveDialog } from "./PlaceMoveDialog";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";

export type FolderFields = { id: string; slug: string; name: string; description: string; parent_id: string | null; organization_id: string | null };

export type FolderGeneralLabels = {
  name: string;
  slug: string;
  slugHint: string;
  description: string;
  save: string;
  deleteHint: string;
  deleteFolder: string;
  deleteFolderText: string;
};

type Props<F extends FolderFields> = {
  base: "/v1/skills" | "/v1/mcp";
  folder: F;
  labels: FolderGeneralLabels;
  onSaved: (folder: F) => void;
  onDeleted: () => void;
  global: string;
  canGlobal: boolean;
};

export function FolderGeneral<F extends FolderFields>({ base, folder, labels: t, onSaved, onDeleted, global, canGlobal }: Props<F>) {
  const { errors, common } = useUiText();
  const p = common.places;
  const [draft, setDraft] = useState({ slug: folder.slug, name: folder.name, description: folder.description });
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [deleting, setDeleting] = useState(false);
  const [moving, setMoving] = useState(false);
  const url = `${base}/folders/${folder.id}` as const;

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      const saved = await apiSend<F>("PATCH", url, draft);
      setNote({ kind: "success", text: p.saved });
      onSaved(saved);
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const remove = async (): Promise<string | null> => {
    try {
      await apiSend("DELETE", url);
      onDeleted();
      return null;
    } catch (e) {
      return errorText(errors, errorCode(e));
    }
  };

  return (
    <div className="grid gap-4">
      <Panel title={p.general}>
        <form
          className="grid gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            void save();
          }}
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t.name}>{(f) => <Input {...f} required value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} />}</Field>
            <Field label={t.slug} hint={t.slugHint}>
              {(f) => <Input {...f} required className="font-mono" value={draft.slug} onChange={(e) => setDraft({ ...draft, slug: e.target.value.toLowerCase() })} />}
            </Field>
          </div>
          <MarkdownField label={t.description} rows={4} value={draft.description} onChange={(description) => setDraft({ ...draft, description })} />
          <Message note={note} />
          <div>
            <Button type="submit" variant="primary" busy={busy} disabled={!draft.name || !draft.slug}>
              {t.save}
            </Button>
          </div>
        </form>
      </Panel>
      <DangerZone
        title={p.danger}
        rows={[
          {
            key: "move",
            text: p.moveText,
            action: (
              <Button onClick={() => setMoving(true)}>
                <FolderInput aria-hidden="true" />
                {p.move}
              </Button>
            ),
          },
          {
            key: "delete",
            text: t.deleteHint,
            action: (
              <Button variant="danger" onClick={() => setDeleting(true)}>
                <Trash2 aria-hidden="true" />
                {t.deleteFolder}
              </Button>
            ),
          },
        ]}
      />
      <PlaceMoveDialog
        open={moving}
        onOpenChange={setMoving}
        base={base}
        name={folder.name}
        global={global}
        canGlobal={canGlobal}
        current={{ folder: folder.parent_id, organization: folder.organization_id }}
        exclude={folder.id}
        move={async (to) => {
          const saved = await apiSend<F>("PATCH", url, { parent_id: to.folder, organization_id: to.organization });
          onSaved(saved);
        }}
        onMoved={() => undefined}
      />
      <ConfirmDialog
        open={deleting}
        onOpenChange={setDeleting}
        title={t.deleteFolder}
        text={format(t.deleteFolderText, { name: folder.name })}
        confirm={t.deleteFolder}
        onConfirm={remove}
      />
    </div>
  );
}
