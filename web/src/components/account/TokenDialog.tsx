"use client";

import { TriangleAlert } from "lucide-react";
import { useState, type FormEvent } from "react";

import { format } from "@/i18n/format";
import { apiSend, SCOPES, type PersonalToken, type Scope } from "@/lib/api";
import { canSubmitNodes, scopesForAccess, tokenNodesBody, type PickedNode, type TokenAccess } from "@/lib/tokenNodes";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Checkbox, Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { NodeScopePicker } from "./NodeScopePicker";
import type { TokenLabels } from "./TokenTable";

const PRESETS = [7, 30, 90, 365] as const;
type Lifetime = `${(typeof PRESETS)[number]}` | "custom" | "forever";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  superadmin: boolean;
  labels: TokenLabels;
  fail: (e: unknown) => string;
  onIssued: (token: PersonalToken) => void;
  presetScopes?: Scope[];
  endpoint?: `/${string}`;
};

function Warning({ children }: { children: string }) {
  return (
    <p role="note" className="flex items-start gap-2.5 rounded-md bg-amber-soft px-3 py-2.5 text-sm text-amber">
      <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      {children}
    </p>
  );
}

export function TokenDialog({ open, onOpenChange, superadmin, labels, fail, onIssued, presetScopes, endpoint = "/v1/account/tokens" }: Props) {
  const { common } = useUiText();
  const t = labels.dialog;
  const [name, setName] = useState("");
  const initialScopes = presetScopes ?? ["read"];
  const [scopes, setScopes] = useState<Scope[]>(initialScopes);
  const [lifetime, setLifetime] = useState<Lifetime>("90");
  const [customDays, setCustomDays] = useState("");
  const [access, setAccess] = useState<TokenAccess>("all");
  const [nodes, setNodes] = useState<PickedNode[]>([]);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const admin = scopes.includes("admin");
  const scoped = access === "nodes";
  const available = SCOPES.filter((s) => s !== "admin" || superadmin);
  const changeAccess = (next: TokenAccess) => {
    setAccess(next);
    setScopes((current) => scopesForAccess(next, current));
  };
  const toggle = (scope: Scope, on: boolean) =>
    setScopes((current) => SCOPES.filter((s) => (s === scope ? on : current.includes(s))));

  const reset = () => {
    setName("");
    setScopes(initialScopes);
    setLifetime("90");
    setCustomDays("");
    setAccess("all");
    setNodes([]);
    setNote(null);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNote(null);
    const days = lifetime === "forever" ? null : lifetime === "custom" ? Number(customDays) : Number(lifetime);
    try {
      const token = await apiSend<PersonalToken>("POST", endpoint, { name, scopes, expires_in_days: days, nodes: tokenNodesBody(access, nodes) });
      reset();
      onIssued(token);
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) reset();
        onOpenChange(next);
      }}
      title={t.title}
      footer={
        <>
          <Button onClick={() => onOpenChange(false)}>{common.cancel}</Button>
          <Button type="submit" form="issue-token" variant="primary" disabled={busy || scopes.length === 0 || !canSubmitNodes(access, nodes, admin)}>
            {t.submit}
          </Button>
        </>
      }
    >
      <form id="issue-token" className="grid gap-4" onSubmit={submit}>
        <Field label={t.name}>
          {(p) => <Input {...p} required maxLength={100} placeholder={t.namePlaceholder} value={name} onChange={(e) => setName(e.target.value)} />}
        </Field>
        <fieldset className="grid gap-2">
          <legend className="mb-1.5 text-sm font-medium text-ink-2">{t.scopes}</legend>
          {available.map((s) => (
            <Checkbox
              key={s}
              checked={scopes.includes(s)}
              disabled={s === "admin" && scoped}
              onChange={(e) => toggle(s, e.target.checked)}
              label={
                <span>
                  <code className="font-medium text-ink">{s}</code> <span className="text-muted">— {labels.scopeHints[s]}</span>
                </span>
              }
            />
          ))}
          {superadmin && scoped && <p className="text-xs text-muted">{t.nodesAdmin}</p>}
          {superadmin && !scoped && !admin && <p className="text-xs text-muted">{t.wholeSuperadmin}</p>}
        </fieldset>
        {admin && <Warning>{t.adminWarning}</Warning>}
        <NodeScopePicker
          access={access}
          onAccess={changeAccess}
          nodes={nodes}
          onNodes={setNodes}
          labels={t}
          note={superadmin ? t.nodesSuperadmin : undefined}
        />
        <Field label={t.lifetime}>
          {(p) => (
            <Select {...p} value={lifetime} onChange={(e) => setLifetime(e.target.value as Lifetime)}>
              {PRESETS.map((n) => (
                <option key={n} value={String(n)}>
                  {format(t.days, { n })}
                </option>
              ))}
              <option value="custom">{t.custom}</option>
              {superadmin && <option value="forever">{t.forever}</option>}
            </Select>
          )}
        </Field>
        {lifetime === "custom" && (
          <Field label={t.customDays}>
            {(p) => (
              <Input {...p} type="number" inputMode="numeric" min={1} step={1} required value={customDays} onChange={(e) => setCustomDays(e.target.value)} />
            )}
          </Field>
        )}
        {lifetime === "forever" && <Warning>{t.foreverWarning}</Warning>}
        <Message note={note} />
      </form>
    </Dialog>
  );
}
