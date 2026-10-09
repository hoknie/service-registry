"use client";

import { TriangleAlert } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { apiSend, type ForgeConnection, type WebhookSetup } from "@/lib/api";

import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { CopyButton } from "../ui/CopyButton";
import { Dialog } from "../ui/Dialog";
import { Message, type Note } from "../ui/Message";
import type { ForgeLabels } from "./shared";

type Props = { url: string; connection: ForgeConnection; labels: ForgeLabels; fail: (e: unknown) => string; onClose: () => void; onChanged: () => void };

function Secret({ label, value, copy }: { label: string; value: string; copy: string }) {
  return (
    <div className="grid gap-1.5">
      <span className="text-sm font-medium text-ink-2">{label}</span>
      <div className="flex flex-wrap items-center gap-2 rounded-lg border border-line bg-surface-2 p-2 pl-3">
        <code className="min-w-0 flex-1 font-mono text-sm break-all text-ink select-all">{value}</code>
        <CopyButton text={value} label={copy} />
      </div>
    </div>
  );
}

export function WebhookDialog({ url, connection, labels: t, fail, onClose, onChanged }: Props) {
  const [setup, setSetup] = useState<WebhookSetup | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const [confirm, setConfirm] = useState(false);

  const configure = async (mode: "register" | "manual") => {
    setBusy(true);
    setNote(null);
    try {
      const result = await apiSend<WebhookSetup>("POST", url as `/${string}`, { mode });
      if (mode === "manual") setSetup(result);
      else {
        toast.success(t.registered);
        onClose();
      }
      onChanged();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog open persistent={!!setup} onOpenChange={(open) => !open && onClose()} title={t.webhook} footer={<Button onClick={onClose}>{t.close}</Button>}>
      {setup ? (
        <div className="grid gap-4">
          <p role="alert" className="flex items-start gap-2.5 rounded-md bg-amber-soft px-3 py-2.5 text-sm text-amber">
            <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            {t.secretOnce}
          </p>
          <Secret label={t.webhookUrl} value={setup.url} copy={t.webhookUrl} />
          {setup.secret && <Secret label={t.webhookSecret} value={setup.secret} copy={t.webhookSecret} />}
        </div>
      ) : (
        <div className="grid gap-4">
          <p className="text-sm text-ink-2">{t.webhookLead}</p>
          <p className="text-sm">
            {connection.webhook ? (
              <>
                <span className="font-medium text-ink">{t.modes[connection.webhook.mode]}</span>
                {connection.webhook.url && <code className="ml-2 text-xs break-all text-muted">{connection.webhook.url}</code>}
              </>
            ) : (
              <span className="text-muted">{t.noWebhook}</span>
            )}
          </p>
          <div className="flex flex-wrap gap-2">
            <Button variant="primary" disabled={busy} onClick={() => void configure("register")}>
              {t.register}
            </Button>
            <Button disabled={busy} onClick={() => void configure("manual")}>
              {t.manual}
            </Button>
            {connection.webhook && (
              <Button variant="danger-ghost" disabled={busy} onClick={() => setConfirm(true)}>
                {t.disableWebhook}
              </Button>
            )}
          </div>
          <p className="text-xs text-muted">{t.registerHint}</p>
          <Message note={note} />
        </div>
      )}
      {confirm && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && setConfirm(false)}
          title={t.disableWebhook}
          text={t.confirmDisable}
          confirm={t.disableWebhook}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", url as `/${string}`);
              toast.success(t.disabled);
              onChanged();
              onClose();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </Dialog>
  );
}
