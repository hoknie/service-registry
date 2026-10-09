"use client";

import { useState, type FormEvent } from "react";

import { apiSend, type ForgeConnection, type ForgeKind } from "@/lib/api";

import { FORGE_NAMES } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Checkbox, Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { splitPatterns, type ForgeLabels } from "./shared";

type Props = {
  nodeId: string;
  connection: ForgeConnection | null;
  labels: ForgeLabels;
  fail: (e: unknown) => string;
  onClose: () => void;
  onSaved: () => void;
};

const KINDS: ForgeKind[] = ["github", "gitlab", "forgejo", "gitea"];

export function ConnectionDialog({ nodeId, connection, labels: t, fail, onClose, onSaved }: Props) {
  const { common } = useUiText();
  const editing = connection !== null;
  const [kind, setKind] = useState<ForgeKind>(connection?.kind ?? "github");
  const [apiUrl, setApiUrl] = useState(connection?.api_url ?? "");
  const [owner, setOwner] = useState(connection?.owner_path ?? "");
  const [useRef, setUseRef] = useState(connection?.credentials.kind === "ref");
  const [token, setToken] = useState("");
  const [ref, setRef] = useState(connection?.credentials.kind === "ref" ? connection.credentials.ref : "");
  const [include, setInclude] = useState((connection?.name_include ?? []).join(", "));
  const [exclude, setExclude] = useState((connection?.name_exclude ?? []).join(", "));
  const [branchInclude, setBranchInclude] = useState((connection?.branch_include ?? []).join(", "));
  const [archived, setArchived] = useState(connection?.include_archived ?? false);
  const [forks, setForks] = useState(connection?.include_forks ?? false);
  const [subgroups, setSubgroups] = useState(connection?.mirror_subgroups ?? true);
  const [minutes, setMinutes] = useState(String(Math.round((connection?.interval_secs ?? 900) / 60)));
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const credentials = () => {
    if (useRef) return { token_ref: ref.trim() };
    if (token) return { token };
    return undefined;
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNote(null);
    const body: Record<string, unknown> = {
      owner_path: owner,
      name_include: splitPatterns(include),
      name_exclude: splitPatterns(exclude),
      branch_include: splitPatterns(branchInclude),
      include_archived: archived,
      include_forks: forks,
      mirror_subgroups: kind === "gitlab" && subgroups,
      interval_secs: Math.round(Number(minutes) * 60),
    };
    if (apiUrl.trim()) body.api_url = apiUrl.trim();
    const creds = credentials();
    if (creds || !editing) body.credentials = creds ?? { token: "" };
    try {
      if (editing) {
        await apiSend("PATCH", `/v1/catalog/nodes/${nodeId}/forge/connections/${connection.id}`, body);
      } else {
        await apiSend("POST", `/v1/catalog/nodes/${nodeId}/forge/connections`, { kind, ...body });
      }
      onSaved();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      size="lg"
      onOpenChange={(open) => !open && onClose()}
      title={editing ? t.editTitle : t.createTitle}
      footer={
        <>
          <Button onClick={onClose}>{common.cancel}</Button>
          <Button type="submit" form="forge-connection" variant="primary" disabled={busy}>
            {t.submit}
          </Button>
        </>
      }
    >
      <form id="forge-connection" className="grid gap-4" onSubmit={submit}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t.kind}>
            {(p) => (
              <Select {...p} value={kind} disabled={editing} onChange={(e) => setKind(e.target.value as ForgeKind)}>
                {KINDS.map((k) => (
                  <option key={k} value={k}>
                    {FORGE_NAMES[k]}
                  </option>
                ))}
              </Select>
            )}
          </Field>
          <Field label={t.owner} hint={t.ownerHint}>
            {(p) => <Input {...p} required spellCheck={false} value={owner} onChange={(e) => setOwner(e.target.value)} />}
          </Field>
        </div>
        <Field label={t.apiUrl} hint={t.apiUrlHint}>
          {(p) => (
            <Input
              {...p}
              type="url"
              spellCheck={false}
              required={kind === "forgejo" || kind === "gitea"}
              placeholder={kind === "github" ? "https://api.github.com" : kind === "gitlab" ? "https://gitlab.com" : "https://"}
              value={apiUrl}
              onChange={(e) => setApiUrl(e.target.value)}
            />
          )}
        </Field>
        <fieldset className="grid gap-3 rounded-lg border border-line p-4">
          <legend className="px-1 text-sm font-medium text-ink-2">{t.credentials}</legend>
          <div className="flex flex-wrap gap-4">
            <label className="inline-flex items-center gap-2 text-sm">
              <input type="radio" name="cred" checked={!useRef} onChange={() => setUseRef(false)} />
              {t.useToken}
            </label>
            <label className="inline-flex items-center gap-2 text-sm">
              <input type="radio" name="cred" checked={useRef} onChange={() => setUseRef(true)} />
              {t.useRef}
            </label>
          </div>
          {useRef ? (
            <Field label={t.tokenRef} hint={t.tokenRefHint}>
              {(p) => <Input {...p} required spellCheck={false} placeholder="env:FORGE_TOKEN" className="font-mono" value={ref} onChange={(e) => setRef(e.target.value)} />}
            </Field>
          ) : (
            <Field label={t.token} hint={editing ? t.tokenKeep : undefined}>
              {(p) => <Input {...p} type="password" autoComplete="off" required={!editing || connection.credentials.kind === "ref"} value={token} onChange={(e) => setToken(e.target.value)} />}
            </Field>
          )}
        </fieldset>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t.include} hint={t.patternsHint}>
            {(p) => <Input {...p} spellCheck={false} className="font-mono" value={include} onChange={(e) => setInclude(e.target.value)} />}
          </Field>
          <Field label={t.exclude} hint={t.patternsHint}>
            {(p) => <Input {...p} spellCheck={false} className="font-mono" value={exclude} onChange={(e) => setExclude(e.target.value)} />}
          </Field>
        </div>
        <Field label={t.branchInclude} hint={t.branchIncludeHint}>
          {(p) => <Input {...p} spellCheck={false} className="font-mono" placeholder="main, release/*" value={branchInclude} onChange={(e) => setBranchInclude(e.target.value)} />}
        </Field>
        <div className="flex flex-wrap gap-x-6 gap-y-2">
          <Checkbox label={t.archived} checked={archived} onChange={(e) => setArchived(e.target.checked)} />
          <Checkbox label={t.forks} checked={forks} onChange={(e) => setForks(e.target.checked)} />
          {kind === "gitlab" && <Checkbox label={t.subgroups} checked={subgroups} onChange={(e) => setSubgroups(e.target.checked)} />}
        </div>
        <Field label={t.interval} className="max-w-48">
          {(p) => <Input {...p} type="number" min={1} max={1440} step={1} required value={minutes} onChange={(e) => setMinutes(e.target.value)} />}
        </Field>
        <Message note={note} />
      </form>
    </Dialog>
  );
}
