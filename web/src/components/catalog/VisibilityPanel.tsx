"use client";

import { useState } from "react";
import { toast } from "sonner";

import { apiSend, type CatalogNode } from "@/lib/api";

import { Checkbox } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import type { CatalogLabels } from "./shared";

type Props = {
  node: CatalogNode;
  labels: CatalogLabels;
  fail: (e: unknown) => string;
  canSwitch: boolean;
  onSaved: () => void;
};

export function VisibilityPanel({ node, labels, fail, canSwitch, onSaved }: Props) {
  const t = labels.visibility;
  const [open, setOpen] = useState(node.visibility === "open");
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const change = async (next: boolean) => {
    setNote(null);
    setBusy(true);
    setOpen(next);
    try {
      await apiSend("PUT", `/v1/catalog/nodes/${node.id}/visibility`, { visibility: next ? "open" : "closed" });
      toast.success(next ? t.opened : t.closed);
      onSaved();
    } catch (e) {
      setOpen(!next);
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Panel title={t.title}>
      <div className="grid gap-2">
        {canSwitch ? (
          <Checkbox
            label={t.open}
            checked={open}
            disabled={busy}
            onChange={(e) => void change(e.target.checked)}
            aria-describedby="visibility-hint"
          />
        ) : (
          <p className="text-sm text-ink">{open ? t.stateOpen : t.stateClosed}</p>
        )}
        <p id="visibility-hint" className="text-xs text-muted">
          {t.hint}
        </p>
        <Message note={note} />
      </div>
    </Panel>
  );
}
