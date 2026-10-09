"use client";

import { useState, type ReactNode } from "react";

import { useUiText } from "../UiText";
import { Button } from "./Button";
import { Dialog } from "./Dialog";
import { Message, type Note } from "./Message";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  text: ReactNode;
  confirm: string;
  onConfirm: () => Promise<string | null>;
};

export function ConfirmDialog({ open, onOpenChange, title, text, confirm, onConfirm }: Props) {
  const { common } = useUiText();
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  const run = async () => {
    setBusy(true);
    setNote(null);
    const error = await onConfirm();
    setBusy(false);
    if (error) setNote({ kind: "error", text: error });
    else onOpenChange(false);
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) setNote(null);
        onOpenChange(next);
      }}
      size="sm"
      title={title}
      description={text}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)}>{common.cancel}</Button>
          <Button variant="danger" disabled={busy} onClick={() => void run()}>
            {confirm}
          </Button>
        </>
      }
    >
      {note && <Message note={note} />}
    </Dialog>
  );
}
