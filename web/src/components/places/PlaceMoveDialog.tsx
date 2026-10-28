"use client";

import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, errorCode } from "@/lib/api";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Combobox } from "../ui/Combobox";
import { Dialog } from "../ui/Dialog";
import { Field } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { useOrganizations } from "./spaces";

export type MoveTarget = { folder: string | null; organization: string | null };

type Folder = { id: string; name: string; parent_id: string | null; organization_id: string | null; can_write: boolean };

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  base: "/v1/skills" | "/v1/mcp";
  name: string;
  global: string;
  canGlobal: boolean;
  current: MoveTarget;
  exclude?: string;
  move: (target: MoveTarget) => Promise<void>;
  onMoved: () => void;
};

async function allFolders(base: Props["base"]): Promise<Folder[]> {
  const walk = async (parent: string | null, depth: number): Promise<Folder[]> => {
    if (depth > 8) return [];
    const level = (await apiGet<{ items: Folder[] }>(`${base}/folders${parent ? `?parent_id=${parent}` : ""}`)).items;
    const nested = await Promise.all(level.map((f) => walk(f.id, depth + 1)));
    return level.flatMap((f, i) => [f, ...(nested[i] ?? [])]);
  };
  return walk(null, 0);
}

export function PlaceMoveDialog({ open, onOpenChange, base, name, global, canGlobal, current, exclude, move, onMoved }: Props) {
  const { errors, common } = useUiText();
  const p = common.places;
  const orgs = useOrganizations();
  const [folders, setFolders] = useState<Folder[] | null>(null);
  const [target, setTarget] = useState("");
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!open) return;
    let live = true;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTarget("");
    setNote(null);
    allFolders(base)
      .then((list) => live && setFolders(list))
      .catch((e: unknown) => live && setNote({ kind: "error", text: errorText(errors, errorCode(e)) }));
    return () => {
      live = false;
    };
  }, [open, base, errors]);

  const byId = new Map((folders ?? []).map((f) => [f.id, f]));
  const inside = (f: Folder): boolean => {
    for (let cur: Folder | undefined = f; cur; cur = cur.parent_id ? byId.get(cur.parent_id) : undefined) if (cur.id === exclude) return true;
    return false;
  };
  const spaceName = (org: string | null) => (org ? (orgs.find((o) => o.id === org)?.name ?? "") : global);
  const pathOf = (f: Folder): string => {
    const names: string[] = [];
    for (let cur: Folder | undefined = f; cur; cur = cur.parent_id ? byId.get(cur.parent_id) : undefined) names.unshift(cur.name);
    return [spaceName(f.organization_id), ...names].join(" / ");
  };
  const here = (t: MoveTarget) => t.folder === current.folder && (t.folder !== null || t.organization === current.organization);
  const spaces = [...(canGlobal ? [{ id: null as string | null, name: global }] : []), ...orgs.map((o) => ({ id: o.id as string | null, name: o.name }))];
  const options = [
    ...spaces.filter((s) => !here({ folder: null, organization: s.id })).map((s) => ({ value: `space:${s.id ?? "global"}`, label: `${s.name} / ${p.topLevel}` })),
    ...(folders ?? [])
      .filter((f) => f.can_write && !inside(f) && !here({ folder: f.id, organization: f.organization_id }) && (canGlobal || f.organization_id !== null))
      .map((f) => ({ value: `folder:${f.id}`, label: pathOf(f) })),
  ].sort((a, b) => a.label.localeCompare(b.label));

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const [kind, id] = target.split(":");
    if (!id) return;
    setBusy(true);
    setNote(null);
    try {
      await move(kind === "folder" ? { folder: id, organization: null } : { folder: null, organization: id === "global" ? null : id });
      onOpenChange(false);
      toast.success(p.moveDone);
      onMoved();
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      size="sm"
      title={format(p.moveTitle, { name })}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)}>{common.cancel}</Button>
          <Button type="submit" form="move-place" variant="primary" busy={busy} disabled={!target}>
            {p.move}
          </Button>
        </>
      }
    >
      <form id="move-place" className="grid gap-4" onSubmit={submit}>
        <Field label={p.moveTarget}>{(f) => <Combobox {...f} required value={target} onChange={setTarget} placeholder={p.moveChoose} options={options} />}</Field>
        <Message note={note} />
      </form>
    </Dialog>
  );
}
