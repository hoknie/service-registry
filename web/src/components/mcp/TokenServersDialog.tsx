"use client";

import { useEffect, useState } from "react";

import { apiSend, type TokenServer } from "@/lib/api";
import { serverListBody } from "@/lib/mcp";

import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Message, type Note } from "../ui/Message";
import { ServerSetPicker, type PickerLabels } from "./ServerSetPicker";

export type TokenServerLabels = PickerLabels & {
  dialogTitle: string;
  changeHint: string;
  save: string;
};

type Props<T> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  current: TokenServer[] | null | undefined;
  endpoint: `/${string}` | null;
  labels: TokenServerLabels;
  fail: (e: unknown) => string;
  onSaved: (updated: T) => void;
};

export function TokenServersDialog<T>({
  open,
  onOpenChange,
  current,
  endpoint,
  labels: t,
  fail,
  onSaved,
}: Props<T>) {
  const { common } = useUiText();
  const [value, setValue] = useState<string[] | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  useEffect(() => {
    if (!open) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setValue(current ? current.map((s) => s.id) : null);
    setNote(null);
  }, [open, current]);

  const save = async () => {
    if (!endpoint) return;
    setBusy(true);
    setNote(null);
    try {
      onSaved(await apiSend<T>("PUT", endpoint, serverListBody(value)));
      onOpenChange(false);
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.dialogTitle}
      description={t.changeHint}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)}>{common.cancel}</Button>
          <Button variant="primary" busy={busy} onClick={save}>
            {t.save}
          </Button>
        </>
      }
    >
      <div className="grid gap-3">
        <ServerSetPicker value={value} onChange={setValue} labels={t} />
        <Message note={note} />
      </div>
    </Dialog>
  );
}

export function serversLabel(
  servers: TokenServer[] | null | undefined,
  all: string,
  none: string,
): string {
  if (servers === null || servers === undefined) return all;
  if (servers.length === 0) return none;
  return servers.map((s) => s.slug).join(", ");
}

export function useGatewayOn(): boolean {
  const { session } = useSession();
  return (
    session.status === "signed-in" &&
    session.user.features?.mcp_gateway === true
  );
}
