"use client";

import { KeyRound, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import type { Subscription } from "@/lib/notifications";
import { createSigningSecret } from "@/lib/notificationsApi";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { CopyButton } from "../ui/CopyButton";
import { Message, type Note } from "../ui/Message";

type Props = { nodeId: string; sub: Subscription; labels: Messages["notify"]; onChanged: () => void };

export function SigningSecret({ nodeId, sub, labels: t, onChanged }: Props) {
  const { errors } = useUiText();
  const [fingerprint, setFingerprint] = useState(sub.signing?.fingerprint ?? null);
  const [secret, setSecret] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  const create = async () => {
    setBusy(true);
    try {
      const r = await createSigningSecret(nodeId, sub.key);
      setSecret(r.secret);
      setFingerprint(r.fingerprint);
      setNote(null);
      toast.success(t.signing.created);
      onChanged();
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <section aria-label={t.signing.title} className="grid gap-3 rounded-lg border border-line p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="flex items-center gap-2 text-sm text-ink-2">
          <KeyRound aria-hidden="true" className="size-4 text-muted" />
          {fingerprint ? format(t.signing.fingerprint, { fingerprint }) : t.signing.none}
        </p>
        <Button size="sm" busy={busy} disabled={busy} onClick={() => void create()}>
          {fingerprint ? t.signing.replace : t.signing.create}
        </Button>
      </div>
      <Message note={note} />
      {secret && (
        <div className="grid gap-2 rounded-md border border-amber/40 bg-amber-soft/60 p-3">
          <p className="flex items-start gap-2 text-sm text-amber">
            <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
            {t.signing.once}
          </p>
          <div className="flex items-center gap-2">
            <code className="min-w-0 flex-1 truncate rounded-sm bg-surface px-2 py-1 font-mono text-xs text-ink">{secret}</code>
            <CopyButton text={secret} />
          </div>
        </div>
      )}
    </section>
  );
}
