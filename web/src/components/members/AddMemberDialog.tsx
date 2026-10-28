"use client";

import { useEffect, useState } from "react";

import { format } from "@/i18n/format";
import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { MEMBER_ROLES, type MemberPerson, type MemberRole } from "@/lib/members";
import { addMember, memberCandidates } from "@/lib/membersApi";
import { cn } from "@/lib/cn";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Select } from "../ui/Select";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  org: string;
  roles: Record<MemberRole, string>;
  labels: Messages["catalog"]["members"];
  onAdded: () => void;
  onInvite: (email: string) => void;
};

export function AddMemberDialog({ open, onOpenChange, org, roles, labels: t, onAdded, onInvite }: Props) {
  const { errors } = useUiText();
  const [query, setQuery] = useState("");
  const [found, setFound] = useState<MemberPerson[] | null>(null);
  const [picked, setPicked] = useState<MemberPerson | null>(null);
  const [role, setRole] = useState<MemberRole>("viewer");
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  useEffect(() => {
    if (!open) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setQuery("");
    setFound(null);
    setPicked(null);
    setRole("viewer");
    setNote(null);
  }, [open]);

  useEffect(() => {
    const q = query.trim();
    if (q.length < 3) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setFound(null);
      return;
    }
    const ctl = new AbortController();
    const timer = setTimeout(() => {
      memberCandidates(org, q, ctl.signal)
        .then((r) => setFound(r.items))
        .catch(() => undefined);
    }, 300);
    return () => {
      clearTimeout(timer);
      ctl.abort();
    };
  }, [query, org]);

  const submit = async () => {
    if (!picked) return;
    setBusy(true);
    setNote(null);
    try {
      await addMember(org, { user_id: picked.id, role });
      onAdded();
      onOpenChange(false);
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const typedEmail = query.includes("@") ? query.trim() : "";
  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t.addTitle}
      footer={
        <Button variant="primary" busy={busy} disabled={!picked} onClick={submit}>
          {t.add}
        </Button>
      }
    >
      <div className="grid gap-4">
        <Field label={t.findUser} hint={t.findHint}>
          {(f) => <Input {...f} type="search" value={query} onChange={(e) => setQuery(e.target.value)} />}
        </Field>
        {found && found.length === 0 && (
          <div className="grid gap-2 text-sm text-muted">
            <p>{t.notFound}</p>
            {typedEmail && (
              <div>
                <Button size="sm" onClick={() => onInvite(typedEmail)}>
                  {format(t.inviteInstead, { email: typedEmail })}
                </Button>
              </div>
            )}
          </div>
        )}
        {found && found.length > 0 && (
          <ul role="listbox" aria-label={t.findUser} className="grid max-h-64 gap-1 overflow-y-auto">
            {found.map((p) => (
              <li key={p.id} role="option" aria-selected={picked?.id === p.id}>
                <button
                  type="button"
                  onClick={() => setPicked(p)}
                  className={cn(
                    "motion-control grid w-full rounded-md px-3 py-2 text-left hover:bg-surface-3",
                    picked?.id === p.id && "bg-signal-soft ring-1 ring-signal",
                  )}
                >
                  <span className="text-sm font-medium text-ink">{p.name || p.email}</span>
                  <span className="text-xs text-muted">{p.email}</span>
                </button>
              </li>
            ))}
          </ul>
        )}
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
        <Message note={note} />
      </div>
    </Dialog>
  );
}
