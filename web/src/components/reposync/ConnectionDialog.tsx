"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { apiGet, apiSend, type RepoSyncConnection, type RepoSyncExclusion, type HostingKind, type Items } from "@/lib/api";
import { ownerUrlOf, parseOwnerUrl } from "@/lib/hostingAddress";
import { ago } from "@/i18n/time";
import Link from "next/link";

import type { Locale } from "@/i18n/config";

import { catalogHref, HOSTING_NAMES } from "../catalog/shared";
import { SecretPicker } from "../secrets/SecretPicker";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { Checkbox, Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { splitPatterns, type RepoSyncLabels } from "./shared";

type Props = {
  nodeId: string;
  locale: Locale;
  connection: RepoSyncConnection | null;
  labels: RepoSyncLabels;
  fail: (e: unknown) => string;
  onClose: () => void;
  onSaved: () => void;
};

const KINDS: HostingKind[] = ["github", "gitlab", "forgejo", "gitea"];

export function ConnectionDialog({ nodeId, locale, connection, labels: t, fail, onClose, onSaved }: Props) {
  const { common } = useUiText();
  const editing = connection !== null;
  const [kind, setKind] = useState<HostingKind>(connection?.kind ?? "github");
  const [apiUrl, setApiUrl] = useState(connection?.api_url ?? "");
  const [owner, setOwner] = useState(connection?.owner_path ?? "");
  const [ownerUrl, setOwnerUrl] = useState(connection ? ownerUrlOf(connection.api_url, connection.owner_path) : "");
  const [kindTouched, setKindTouched] = useState(editing);
  const [apiTouched, setApiTouched] = useState(editing);
  const [ownerTouched, setOwnerTouched] = useState(editing);
  const [exclusions, setExclusions] = useState<RepoSyncExclusion[] | null>(null);
  const [restoring, setRestoring] = useState<RepoSyncExclusion | null>(null);
  const exclusionsUrl = connection ? (`/v1/catalog/nodes/${nodeId}/repo-sync/connections/${connection.id}/exclusions` as const) : null;

  const loadExclusions = useCallback(() => {
    if (!exclusionsUrl) return;
    apiGet<Items<RepoSyncExclusion>>(exclusionsUrl)
      .then((res) => setExclusions(res.items))
      .catch(() => setExclusions([]));
  }, [exclusionsUrl]);

  useEffect(() => {
    loadExclusions();
  }, [loadExclusions]);

  const derive = (url: string, k: HostingKind, force: boolean) => {
    const parsed = parseOwnerUrl(url, k);
    if (!parsed) return;
    const nextKind = !editing && !kindTouched && parsed.kind ? parsed.kind : k;
    if (nextKind !== k) setKind(nextKind);
    const api = nextKind === k ? parsed.apiUrl : (parseOwnerUrl(url, nextKind)?.apiUrl ?? null);
    if (force || !ownerTouched) setOwner(parsed.ownerPath);
    if ((force || !apiTouched) && api) setApiUrl(api);
  };
  const current = connection?.credentials.kind === "secret" ? connection.credentials.secret.id : null;
  const [secret, setSecret] = useState<string | null>(current);
  const [include, setInclude] = useState((connection?.name_include ?? []).join(", "));
  const [exclude, setExclude] = useState((connection?.name_exclude ?? []).join(", "));
  const [branchInclude, setBranchInclude] = useState((connection?.branch_include ?? []).join(", "));
  const [archived, setArchived] = useState(connection?.include_archived ?? false);
  const [forks, setForks] = useState(connection?.include_forks ?? false);
  const [subgroups, setSubgroups] = useState(connection?.mirror_subgroups ?? true);
  const [minutes, setMinutes] = useState(String(Math.round((connection?.interval_secs ?? 900) / 60)));
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);


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
    if (secret && secret !== current) body.credentials = { secret_id: secret };
    else if (!editing) body.credentials = { secret_id: secret ?? "" };
    try {
      if (editing) {
        await apiSend("PATCH", `/v1/catalog/nodes/${nodeId}/repo-sync/connections/${connection.id}`, body);
      } else {
        await apiSend("POST", `/v1/catalog/nodes/${nodeId}/repo-sync/connections`, { kind, ...body });
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
          <Button type="submit" form="repo-sync-connection" variant="primary" disabled={busy}>
            {t.submit}
          </Button>
        </>
      }
    >
      <form id="repo-sync-connection" className="grid gap-4" onSubmit={submit}>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t.ownerUrl} hint={t.ownerUrlHint} className="sm:col-span-2">
            {(p) => (
              <div className="flex gap-2">
                <Input
                  {...p}
                  type="url"
                  spellCheck={false}
                  className="min-w-0 flex-1"
                  placeholder="https://gitlab.example.com/platform/team"
                  value={ownerUrl}
                  onChange={(e) => {
                    setOwnerUrl(e.target.value);
                    derive(e.target.value, kind, false);
                  }}
                />
                {(apiTouched || ownerTouched) && ownerUrl && (
                  <Button
                    type="button"
                    onClick={() => {
                      setApiTouched(false);
                      setOwnerTouched(false);
                      derive(ownerUrl, kind, true);
                    }}
                  >
                    {t.recompute}
                  </Button>
                )}
              </div>
            )}
          </Field>
          <Field label={t.kind}>
            {(p) => (
              <Select
                {...p}
                value={kind}
                disabled={editing}
                onChange={(e) => {
                  const next = e.target.value as HostingKind;
                  setKind(next);
                  setKindTouched(true);
                  if (!apiTouched && ownerUrl) {
                    const api = parseOwnerUrl(ownerUrl, next)?.apiUrl;
                    if (api) setApiUrl(api);
                  }
                }}
              >
                {KINDS.map((k) => (
                  <option key={k} value={k}>
                    {HOSTING_NAMES[k]}
                  </option>
                ))}
              </Select>
            )}
          </Field>
          <Field label={t.owner} hint={t.ownerHint}>
            {(p) => (
              <Input
                {...p}
                required
                spellCheck={false}
                value={owner}
                onChange={(e) => {
                  setOwner(e.target.value);
                  setOwnerTouched(true);
                }}
              />
            )}
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
              onChange={(e) => {
                setApiUrl(e.target.value);
                setApiTouched(true);
              }}
            />
          )}
        </Field>
        <SecretPicker
          listOn={nodeId}
          createOn={nodeId}
          value={secret}
          onChange={setSecret}
          manageHref={catalogHref(locale, nodeId, "settings", null, null, "secrets")}
          required={!editing}
        />
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
        {editing && (
          <section aria-labelledby="repo-sync-exclusions" className="grid gap-2 border-t border-line pt-4">
            <h3 id="repo-sync-exclusions" className="text-sm font-semibold text-ink">
              {t.exclusions.title}
            </h3>
            {exclusions === null ? null : exclusions.length === 0 ? (
              <p className="text-sm text-muted">{t.exclusions.empty}</p>
            ) : (
              <ul className="grid gap-2">
                {exclusions.map((x) => (
                  <li key={x.external_id} className="flex flex-wrap items-center gap-x-3 gap-y-1 rounded-md border border-line px-3 py-2 text-sm">
                    <code className="min-w-0 truncate text-ink">{x.full_path}</code>
                    <span className="text-xs text-muted">{ago(x.created_at, locale, "")}</span>
                    {x.project && (
                      <Link href={catalogHref(locale, x.project.id)} className="text-signal hover:underline">
                        {x.project.name}
                      </Link>
                    )}
                    <Button type="button" size="sm" className="ml-auto" onClick={() => setRestoring(x)}>
                      {t.exclusions.restore}
                    </Button>
                  </li>
                ))}
              </ul>
            )}
          </section>
        )}
        <Message note={note} />
      </form>
      {restoring && exclusionsUrl && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setRestoring(null)}
          title={t.exclusions.restore}
          text={t.exclusions.confirm}
          confirm={t.exclusions.restore}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `${exclusionsUrl}/${encodeURIComponent(restoring.external_id)}` as `/${string}`);
              toast.success(t.exclusions.restored);
              loadExclusions();
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
