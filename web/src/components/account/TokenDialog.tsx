"use client";

import { TriangleAlert } from "lucide-react";
import { useState, type FormEvent } from "react";

import { format } from "@/i18n/format";
import { apiSend, type PersonalToken, type Scope } from "@/lib/api";
import { scopeAllowed } from "@/lib/rights";
import {
  BASE_SCOPES,
  canSubmitNodes,
  CATALOG_ROOT,
  PUBLICATION_SCOPE,
  SKILLS_SCOPE,
  MCP_SERVERS_SCOPE,
  GLOBAL_AREAS,
  isGlobalScope,
  scopesForAccess,
  toggleScope,
  tokenNodesBody,
  type PickedNode,
  type TokenAccess,
} from "@/lib/tokenNodes";

import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Checkbox, Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { ServerSetPicker } from "../mcp/ServerSetPicker";
import { useGatewayOn } from "../mcp/TokenServersDialog";
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

type ScopeBoxProps = { scope: Scope; hint: string; checked: boolean; disabled?: boolean; onChange: (on: boolean) => void };

function ScopeBox({ scope, hint, checked, disabled, onChange }: ScopeBoxProps) {
  return (
    <Checkbox
      checked={checked}
      disabled={disabled}
      onChange={(e) => onChange(e.target.checked)}
      label={
        <span>
          <code className="font-medium text-ink">{scope}</code> <span className="text-muted">— {hint}</span>
        </span>
      }
    />
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
  const [servers, setServers] = useState<string[] | null>(null);
  const gateway = useGatewayOn();
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const { session } = useSession();
  const viewer = session.status === "signed-in" ? session.user : null;
  const allowed = (scope: Scope) => superadmin || (endpoint === "/v1/account/tokens" && scopeAllowed(viewer, scope));
  const offerGlobal = superadmin || allowed(CATALOG_ROOT) || GLOBAL_AREAS.some((area) => allowed(`${area}:read`) || allowed(`${area}:write`));
  const global = scopes.some(isGlobalScope);
  const scoped = access === "nodes";
  const changeAccess = (next: TokenAccess) => {
    setAccess(next);
    setScopes((current) => scopesForAccess(next, current));
  };
  const toggle = (scope: Scope, on: boolean) =>
    setScopes((current) => toggleScope(current, scope, on).filter((s) => !isGlobalScope(s) || allowed(s)));

  const reset = () => {
    setName("");
    setScopes(initialScopes);
    setLifetime("90");
    setCustomDays("");
    setAccess("all");
    setNodes([]);
    setServers(null);
    setNote(null);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNote(null);
    const days = lifetime === "forever" ? null : lifetime === "custom" ? Number(customDays) : Number(lifetime);
    try {
      const token = await apiSend<PersonalToken>("POST", endpoint, { name, scopes, expires_in_days: days, nodes: tokenNodesBody(access, nodes),
        ...(gateway && scopes.includes("mcp") ? { mcp_servers: servers } : {}) });
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
          <Button type="submit" form="issue-token" variant="primary" disabled={busy || scopes.length === 0 || !canSubmitNodes(access, nodes, scopes)}>
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
          {BASE_SCOPES.map((s) => (
            <ScopeBox key={s} scope={s} hint={labels.scopeHints[s]} checked={scopes.includes(s)} onChange={(on) => toggle(s, on)} />
          ))}
          {superadmin && !scoped && !global && <p className="text-xs text-muted">{t.wholeSuperadmin}</p>}
        </fieldset>
        {offerGlobal && (
          <fieldset className="grid gap-2">
            <legend className="mb-1.5 text-sm font-medium text-ink-2">{t.globalScopes}</legend>
            {GLOBAL_AREAS.filter((area) => allowed(`${area}:read`) || allowed(`${area}:write`)).map((area) => (
              <div key={area} className="grid gap-1 sm:grid-cols-[1fr_auto_auto] sm:items-center sm:gap-4">
                <span className="text-sm">
                  <span className="font-medium text-ink">{labels.scopeAreas[area]}</span>
                </span>
                {(["read", "write"] as const).map((action) => {
                  const scope: Scope = `${area}:${action}`;
                  if (!allowed(scope)) return <span key={scope} />;
                  return (
                    <Checkbox
                      key={scope}
                      checked={scopes.includes(scope)}
                      disabled={scoped}
                      title={labels.scopeHints[scope]}
                      onChange={(e) => toggle(scope, e.target.checked)}
                      label={action === "read" ? t.globalRead : t.globalWrite}
                    />
                  );
                })}
              </div>
            ))}
            {allowed(CATALOG_ROOT) && (
              <ScopeBox
                scope={CATALOG_ROOT}
                hint={labels.scopeHints[CATALOG_ROOT]}
                checked={scopes.includes(CATALOG_ROOT)}
                onChange={(on) => toggle(CATALOG_ROOT, on)}
              />
            )}
            {allowed(PUBLICATION_SCOPE) && (
              <Checkbox
                checked={scopes.includes(PUBLICATION_SCOPE)}
                disabled={scoped}
                onChange={(e) => toggle(PUBLICATION_SCOPE, e.target.checked)}
                label={
                  <span>
                    <span className="font-mono">{PUBLICATION_SCOPE}</span> — {labels.scopeHints[PUBLICATION_SCOPE]}
                  </span>
                }
              />
            )}
            {allowed(SKILLS_SCOPE) && (
              <Checkbox
                checked={scopes.includes(SKILLS_SCOPE)}
                disabled={scoped}
                onChange={(e) => toggle(SKILLS_SCOPE, e.target.checked)}
                label={
                  <span>
                    <span className="font-mono">{SKILLS_SCOPE}</span> — {labels.scopeHints[SKILLS_SCOPE]}
                  </span>
                }
              />
            )}
            {allowed(MCP_SERVERS_SCOPE) && (
              <Checkbox
                checked={scopes.includes(MCP_SERVERS_SCOPE)}
                disabled={scoped}
                onChange={(e) => toggle(MCP_SERVERS_SCOPE, e.target.checked)}
                label={
                  <span>
                    <span className="font-mono">{MCP_SERVERS_SCOPE}</span> — {labels.scopeHints[MCP_SERVERS_SCOPE]}
                  </span>
                }
              />
            )}
            {scoped && <p className="text-xs text-muted">{t.nodesGlobal}</p>}
          </fieldset>
        )}
        {global && <Warning>{t.globalWarning}</Warning>}
        <NodeScopePicker
          access={access}
          onAccess={changeAccess}
          nodes={nodes}
          onNodes={setNodes}
          labels={t}
          note={superadmin ? t.nodesSuperadmin : undefined}
        />
        {gateway && scopes.includes("mcp") && endpoint === "/v1/account/tokens" && (
          <ServerSetPicker value={servers} onChange={setServers} labels={labels.gateway} />
        )}
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
