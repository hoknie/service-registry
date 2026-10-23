"use client";

import { ArrowDown, ArrowUp, Plus, Trash } from "lucide-react";
import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { errorCode, type Contact, type ContactKind, type Group } from "@/lib/api";
import { moveItem } from "@/lib/owners";
import { putGroupContacts, type SaveContacts } from "@/lib/ownersApi";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Panel } from "../ui/Panel";
import { Select } from "../ui/Select";

type Labels = Messages["admin"]["groups"]["contacts"];
type Row = { kind: ContactKind; value: string; label: string };

const KINDS: ContactKind[] = ["email", "chat", "url", "phone"];
const MAX = 10;

const rowOf = (c: Contact): Row => ({ kind: c.kind, value: c.value, label: c.label ?? "" });

type Props = { group: Group; labels: Labels; onSaved: () => void; save?: SaveContacts; plain?: boolean };

export function GroupContacts({ group, labels, onSaved, save: send = putGroupContacts, plain }: Props) {
  const { errors } = useUiText();
  const [rows, setRows] = useState<Row[]>(() => group.contacts.map(rowOf));
  const [onCall, setOnCall] = useState(group.on_call_url ?? "");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<{ field: "contacts" | "onCall"; text: string } | null>(null);

  const change = (i: number, patch: Partial<Row>) => setRows((r) => r.map((row, j) => (j === i ? { ...row, ...patch } : row)));

  const save = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    try {
      const contacts = rows.map((r) => ({ kind: r.kind, value: r.value.trim(), label: r.label.trim() || null }));
      const saved = await send(group.id, contacts, onCall.trim() || null);
      setRows(saved.contacts.map(rowOf));
      setOnCall(saved.on_call_url ?? "");
      setProblem(null);
      toast.success(labels.saved);
      onSaved();
    } catch (e) {
      const code = errorCode(e);
      setProblem({ field: code === "validation.invalid_on_call_url" ? "onCall" : "contacts", text: errorText(errors, code) });
    } finally {
      setBusy(false);
    }
  };

  const form = (
    <form className="grid gap-4" onSubmit={save}>
      {rows.length === 0 ? (
        <p className="text-sm text-muted">{labels.empty}</p>
      ) : (
        <ol className="grid gap-3">
          {rows.map((r, i) => (
            <li key={i} className="grid gap-2 rounded-lg border border-line p-3 sm:grid-cols-[9rem_minmax(0,1fr)_minmax(0,12rem)_auto] sm:items-end">
              <Field label={labels.kind}>
                {(p) => (
                  <Select {...p} value={r.kind} onChange={(e) => change(i, { kind: e.target.value as ContactKind })}>
                    {KINDS.map((k) => (
                      <option key={k} value={k}>
                        {labels.kinds[k]}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
              <Field label={labels.value}>
                {(p) => <Input {...p} required maxLength={2048} value={r.value} onChange={(e) => change(i, { value: e.target.value })} />}
              </Field>
              <Field label={labels.label}>{(p) => <Input {...p} maxLength={100} value={r.label} onChange={(e) => change(i, { label: e.target.value })} />}</Field>
              <span className="flex items-center gap-1">
                <Button type="button" size="icon" variant="ghost" aria-label={labels.up} disabled={i === 0} onClick={() => setRows(moveItem(rows, i, -1))}>
                  <ArrowUp aria-hidden="true" />
                </Button>
                <Button
                  type="button"
                  size="icon"
                  variant="ghost"
                  aria-label={labels.down}
                  disabled={i === rows.length - 1}
                  onClick={() => setRows(moveItem(rows, i, 1))}
                >
                  <ArrowDown aria-hidden="true" />
                </Button>
                <Button type="button" size="icon" variant="danger-ghost" aria-label={labels.remove} onClick={() => setRows(rows.filter((_, j) => j !== i))}>
                  <Trash aria-hidden="true" />
                </Button>
              </span>
            </li>
          ))}
        </ol>
      )}
      {problem?.field === "contacts" && <p className="text-sm text-danger">{problem.text}</p>}
      <Button
        type="button"
        className="justify-self-start"
        disabled={rows.length >= MAX}
        onClick={() => setRows([...rows, { kind: "chat", value: "", label: "" }])}
      >
        <Plus aria-hidden="true" />
        {labels.add}
      </Button>
      <Field label={labels.onCall} hint={labels.onCallHint} error={problem?.field === "onCall" ? problem.text : undefined}>
        {(p) => <Input {...p} inputMode="url" maxLength={2048} value={onCall} onChange={(e) => setOnCall(e.target.value)} placeholder="https://" />}
      </Field>
      <Button type="submit" variant="primary" busy={busy} className="justify-self-start">
        {labels.save}
      </Button>
    </form>
  );

  if (plain) return form;
  return (
    <Panel title={labels.title} description={labels.lead}>
      {form}
    </Panel>
  );
}
