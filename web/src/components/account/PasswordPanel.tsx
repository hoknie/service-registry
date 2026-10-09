"use client";

import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiSend, errorCode } from "@/lib/api";

import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";

type Props = { labels: Messages["account"] };

export function PasswordPanel({ labels }: Props) {
  const { errors } = useUiText();
  const { session, refresh } = useSession();
  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [repeat, setRepeat] = useState("");
  const [message, setMessage] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const t = labels.password;

  if (session.status !== "signed-in") return null;
  const first = !session.user.has_password;
  const mismatch = repeat.length > 0 && next !== repeat;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    if (next !== repeat) {
      setMessage({ kind: "error", text: t.mismatch });
      return;
    }
    setBusy(true);
    setMessage(null);
    try {
      await apiSend("POST", "/v1/auth/password", first ? { new_password: next } : { current_password: current, new_password: next });
      setCurrent("");
      setNext("");
      setRepeat("");
      const done = first ? labels.setPassword.success : t.success;
      setMessage({ kind: "success", text: done });
      toast.success(done);
      if (first) await refresh();
    } catch (e) {
      setMessage({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Panel title={first ? labels.setPassword.title : t.title} description={first ? labels.setPassword.lead : labels.passwordLead}>
      <form className="grid max-w-md gap-4" onSubmit={submit}>
        {!first && (
          <Field label={t.current}>
            {(p) => <Input {...p} type="password" autoComplete="current-password" required value={current} onChange={(e) => setCurrent(e.target.value)} />}
          </Field>
        )}
        <Field label={t.new}>
          {(p) => <Input {...p} type="password" autoComplete="new-password" required value={next} onChange={(e) => setNext(e.target.value)} />}
        </Field>
        <Field label={t.repeat} hint={mismatch ? t.mismatch : undefined}>
          {(p) => (
            <Input
              {...p}
              type="password"
              autoComplete="new-password"
              required
              aria-invalid={mismatch || undefined}
              value={repeat}
              onChange={(e) => setRepeat(e.target.value)}
            />
          )}
        </Field>
        <Message note={message} />
        <div>
          <Button type="submit" variant="primary" disabled={busy}>
            {first ? labels.setPassword.submit : t.submit}
          </Button>
        </div>
      </form>
    </Panel>
  );
}
