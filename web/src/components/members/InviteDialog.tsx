"use client";

import { useEffect, useState } from "react";

import { locales, localeNames, type Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { absoluteLink, MEMBER_ROLES, type IssuedInvitation, type MemberRole } from "@/lib/members";
import { invite } from "@/lib/membersApi";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { CopyButton } from "../ui/CopyButton";
import { Dialog } from "../ui/Dialog";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Select } from "../ui/Select";

type Labels = Messages["catalog"]["members"];

export function IssuedLink({ issued, labels: t }: { issued: IssuedInvitation; labels: Labels }) {
  const link = absoluteLink(issued.link, typeof window === "undefined" ? "" : window.location.origin);
  return (
    <div className="grid gap-2">
      {issued.mail === "sent" ? (
        <Message note={{ kind: "success", text: t.mail.sent }} />
      ) : (
        <p className="text-sm text-muted">{t.mail[issued.mail]}</p>
      )}
      <span className="text-sm font-medium text-ink-2">{t.link}</span>
      <div className="flex items-start gap-2">
        <code className="min-w-0 flex-1 rounded-md bg-surface-2 px-2 py-1.5 font-mono text-xs [overflow-wrap:anywhere] text-ink">{link}</code>
        <CopyButton text={link} label={t.copy} />
      </div>
    </div>
  );
}

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  org: string;
  locale: Locale;
  email?: string;
  roles: Record<MemberRole, string>;
  labels: Labels;
  onInvited: () => void;
};

export function InviteDialog({ open, onOpenChange, org, locale, email: initial, roles, labels: t, onInvited }: Props) {
  const { errors } = useUiText();
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<MemberRole>("viewer");
  const [language, setLanguage] = useState<Locale>(locale);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [issued, setIssued] = useState<IssuedInvitation | null>(null);

  useEffect(() => {
    if (!open) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setEmail(initial ?? "");
    setRole("viewer");
    setLanguage(locale);
    setNote(null);
    setIssued(null);
  }, [open, initial, locale]);

  const submit = async () => {
    setBusy(true);
    setNote(null);
    try {
      setIssued(await invite(org, { email, role, locale: language }));
      onInvited();
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
      title={t.inviteTitle}
      footer={
        issued ? (
          <Button onClick={() => onOpenChange(false)}>{t.done}</Button>
        ) : (
          <Button variant="primary" busy={busy} disabled={!email.trim()} onClick={submit}>
            {t.send}
          </Button>
        )
      }
    >
      {issued ? (
        <IssuedLink issued={issued} labels={t} />
      ) : (
        <div className="grid gap-4">
          <Field label={t.email}>{(f) => <Input {...f} type="email" required value={email} onChange={(e) => setEmail(e.target.value)} />}</Field>
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t.role}>
              {(f) => (
                <Select id={f.id} value={role} onChange={(e) => setRole(e.target.value as MemberRole)}>
                  {MEMBER_ROLES.map((r) => (
                    <option key={r} value={r}>
                      {roles[r]}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
            <Field label={t.language}>
              {(f) => (
                <Select id={f.id} value={language} onChange={(e) => setLanguage(e.target.value as Locale)}>
                  {locales.map((l) => (
                    <option key={l} value={l}>
                      {localeNames[l]}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          </div>
          <Message note={note} />
        </div>
      )}
    </Dialog>
  );
}
