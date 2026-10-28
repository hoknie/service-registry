"use client";

import { GitBranch, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago } from "@/i18n/time";
import { apiGet, apiSend, errorCode } from "@/lib/api";
import { guessForge, sourceApiUrl } from "@/lib/forgeAddress";
import {
  SKILLS_BASE,
  type SkillFolder,
  type SkillPage,
  type SkillSource,
  type SkillSourceKind,
} from "@/lib/skills";
import { loadFolder } from "@/lib/skillsApi";

import { SecretPicker } from "../secrets/SecretPicker";
import { useUiText } from "../UiText";
import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Select } from "../ui/Select";

type Props = { locale: Locale; labels: Messages["skills"]; folderId: string };

const FORGES = ["github", "gitlab", "gitea", "forgejo"] as const;

type Draft = {
  kind: SkillSourceKind;
  forge: string;
  url: string;
  api_url: string;
  path: string;
  secret: string | null;
  branch: string;
  root: string;
  interval: string;
  status: string;
  versions: string;
  tag_pattern: string;
  forgeTouched: boolean;
  apiTouched: boolean;
};

const empty: Draft = {
  kind: "remote",
  forge: "gitlab",
  url: "",
  api_url: "",
  path: "",
  secret: null,
  branch: "",
  root: "",
  interval: "",
  status: "published",
  versions: "branch",
  tag_pattern: "v{version}",
  forgeTouched: false,
  apiTouched: false,
};

function draftOf(s: SkillSource): Draft {
  const forge = s.forge ?? "gitlab";
  return {
    kind: s.kind,
    forge,
    url: s.url ?? "",
    api_url: s.api_url ?? "",
    path: s.path ?? "",
    secret: s.credentials?.secret_id ?? null,
    branch: s.branch,
    root: s.root,
    interval: s.interval_secs ? String(s.interval_secs) : "",
    status: s.status,
    versions: s.versions,
    tag_pattern: s.tag_pattern,
    forgeTouched: s.forge !== null,
    apiTouched: !!s.api_url && s.api_url !== sourceApiUrl(forge, s.url ?? ""),
  };
}

export function withRemote(
  d: Draft,
  patch: Partial<Pick<Draft, "forge" | "url" | "api_url">>,
): Draft {
  const next = { ...d, ...patch };
  if (patch.forge !== undefined) next.forgeTouched = true;
  if (patch.api_url !== undefined) next.apiTouched = true;
  if (patch.url !== undefined && !next.forgeTouched)
    next.forge = guessForge(patch.url) ?? next.forge;
  if (
    !next.apiTouched &&
    (patch.url !== undefined || patch.forge !== undefined)
  )
    next.api_url = sourceApiUrl(next.forge, next.url) ?? "";
  return next;
}

function bodyOf(d: Draft) {
  const remote = d.kind === "remote";
  return {
    kind: d.kind,
    forge: remote ? d.forge : "",
    url: remote ? d.url : "",
    api_url: remote ? d.api_url : "",
    path: remote ? "" : d.path,
    credentials: remote && d.secret ? { secret_id: d.secret } : null,
    branch: d.branch,
    root: d.root,
    interval_secs: d.interval ? Number(d.interval) : null,
    status: d.status,
    versions: d.versions,
    tag_pattern: d.tag_pattern,
  };
}

const stateTone: Record<SkillSource["state"], Tone> = {
  queued: "cobalt",
  running: "signal",
  idle: "neutral",
  failed: "danger",
};

export function SourcesView({ locale, labels: t, folderId }: Props) {
  const s = t.sourcesPage;
  const { errors } = useUiText();
  const [folder, setFolder] = useState<SkillFolder | null>(null);
  const [sources, setSources] = useState<SkillSource[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<SkillSource | "new" | null>(null);
  const [draft, setDraft] = useState<Draft>(empty);
  const [busy, setBusy] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<SkillSource | null>(null);
  const [owned, setOwned] = useState(0);

  const askDelete = async (src: SkillSource) => {
    setDeleting(src);
    setOwned(0);
    try {
      const page = await apiGet<SkillPage>(
        `${SKILLS_BASE}?folder_id=${folderId}&limit=100`,
      );
      setOwned(page.items.filter((x) => x.source?.id === src.id).length);
    } catch {
      setOwned(0);
    }
  };

  const load = useCallback(async () => {
    try {
      const [f, list] = await Promise.all([
        loadFolder(folderId),
        apiGet<{ items: SkillSource[] }>(
          `${SKILLS_BASE}/folders/${folderId}/sources`,
        ),
      ]);
      setFolder(f);
      setSources(list.items);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
      setSources([]);
    }
  }, [folderId, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const busySources = (sources ?? []).some(
    (x) => x.state === "queued" || x.state === "running",
  );
  useEffect(() => {
    if (!busySources) return;
    const timer = setInterval(() => void load(), 5000);
    return () => clearInterval(timer);
  }, [busySources, load]);

  const open = (src: SkillSource | "new") => {
    setEditing(src);
    setDraft(src === "new" ? empty : draftOf(src));
    setFormError(null);
  };

  const save = async () => {
    setBusy(true);
    setFormError(null);
    try {
      if (editing === "new")
        await apiSend(
          "POST",
          `${SKILLS_BASE}/folders/${folderId}/sources`,
          bodyOf(draft),
        );
      else if (editing)
        await apiSend(
          "PATCH",
          `${SKILLS_BASE}/sources/${editing.id}`,
          bodyOf(draft),
        );
      setEditing(null);
      await load();
    } catch (e) {
      setFormError(errorText(errors, errorCode(e)));
    } finally {
      setBusy(false);
    }
  };

  const sync = async (src: SkillSource) => {
    try {
      await apiSend("POST", `${SKILLS_BASE}/sources/${src.id}/sync`);
      await load();
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  };

  const remove = async (): Promise<string | null> => {
    if (!deleting) return null;
    try {
      await apiSend("DELETE", `${SKILLS_BASE}/sources/${deleting.id}`);
      await load();
      return null;
    } catch (e) {
      return errorText(errors, errorCode(e));
    }
  };

  const set = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));
  return (
    <>
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <p className="max-w-2xl text-sm text-muted">{s.lead}</p>
        {folder?.can_write && (
          <Button variant="primary" onClick={() => open("new")}>
            <Plus aria-hidden="true" className="size-4" /> {s.add}
          </Button>
        )}
      </div>
      <Message
        note={error ? { kind: "error", text: error } : null}
        className="mb-4"
      />
      {sources && sources.length === 0 && (
        <EmptyState icon={GitBranch} title={s.empty} />
      )}
      <div className="grid gap-4">
        {(sources ?? []).map((src) => (
          <Panel
            key={src.id}
            title={
              <span className="font-mono text-sm [overflow-wrap:anywhere]">
                {src.url ?? src.path}
              </span>
            }
            description={[
              src.branch || null,
              src.root || null,
              src.versions === "tags"
                ? `${s.fromTags}: ${src.tag_pattern}`
                : null,
            ]
              .filter(Boolean)
              .join(" · ")}
            actions={
              folder?.can_write && (
                <>
                  <Button size="sm" onClick={() => sync(src)}>
                    <RefreshCw aria-hidden="true" className="size-4" /> {s.sync}
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    aria-label={s.edit}
                    title={s.edit}
                    onClick={() => open(src)}
                  >
                    <Pencil aria-hidden="true" className="size-4" />
                  </Button>
                  <Button
                    size="sm"
                    variant="ghost"
                    aria-label={s.delete}
                    title={s.delete}
                    onClick={() => void askDelete(src)}
                  >
                    <Trash2 aria-hidden="true" className="size-4" />
                  </Button>
                </>
              )
            }
          >
            <div className="grid gap-2 text-sm">
              <div className="flex flex-wrap items-center gap-2">
                <Badge tone={stateTone[src.state]}>{s.state[src.state]}</Badge>
                {src.next_run_at && src.state !== "running" && (
                  <span className="text-muted">
                    {format(s.nextRun, {
                      when: ago(src.next_run_at, locale, ""),
                    })}
                  </span>
                )}
              </div>
              <div>
                <span className="text-muted">{s.lastRun}: </span>
                {src.last_run ? (
                  <>
                    {s.status[src.last_run.status]} ·{" "}
                    {format(s.counters, {
                      created: src.last_run.created,
                      updated: src.last_run.updated,
                      removed: src.last_run.removed,
                      skipped: src.last_run.skipped,
                    })}
                    {src.last_run.commit && (
                      <span className="ml-2 font-mono text-xs text-muted">
                        {src.last_run.commit.slice(0, 12)}
                      </span>
                    )}
                    {src.last_run.error && (
                      <span className="ml-2 text-danger">
                        {errorText(errors, src.last_run.error)}
                      </span>
                    )}
                  </>
                ) : (
                  s.never
                )}
              </div>
              {src.last_run && src.last_run.problems.length > 0 && (
                <ul className="list-inside list-disc text-xs text-ink-2">
                  {src.last_run.problems.map((p, i) => (
                    <li key={i}>
                      <span>{errorText(errors, p.code)}</span>
                      {p.path && (
                        <span className="ml-1 font-mono">{p.path}</span>
                      )}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </Panel>
        ))}
      </div>
      <Dialog
        open={editing !== null}
        onOpenChange={(o) => !o && setEditing(null)}
        title={editing === "new" ? s.add : s.edit}
        size="lg"
        footer={
          <Button variant="primary" busy={busy} onClick={save}>
            {s.save}
          </Button>
        }
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={s.kind} className="sm:col-span-2">
            {(f) => (
              <Select
                id={f.id}
                value={draft.kind}
                onChange={(e) =>
                  set({ kind: e.target.value as SkillSourceKind })
                }
              >
                <option value="remote">{s.remote}</option>
                <option value="local_git">{s.localGit}</option>
                <option value="local_dir">{s.localDir}</option>
              </Select>
            )}
          </Field>
          {draft.kind === "remote" ? (
            <>
              <Field label={s.forge}>
                {(f) => (
                  <Select
                    id={f.id}
                    value={draft.forge}
                    onChange={(e) =>
                      setDraft((d) => withRemote(d, { forge: e.target.value }))
                    }
                  >
                    {FORGES.map((x) => (
                      <option key={x} value={x}>
                        {x}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
              <Field label={s.url}>
                {(f) => (
                  <Input
                    {...f}
                    value={draft.url}
                    onChange={(e) =>
                      setDraft((d) => withRemote(d, { url: e.target.value }))
                    }
                  />
                )}
              </Field>
              <Field label={s.apiUrl}>
                {(f) => (
                  <div className="flex gap-2">
                    <Input
                      {...f}
                      className="min-w-0 flex-1"
                      value={draft.api_url}
                      onChange={(e) =>
                        setDraft((d) =>
                          withRemote(d, { api_url: e.target.value }),
                        )
                      }
                    />
                    {draft.apiTouched && (
                      <Button
                        type="button"
                        onClick={() =>
                          setDraft((d) => ({
                            ...d,
                            apiTouched: false,
                            api_url: sourceApiUrl(d.forge, d.url) ?? "",
                          }))
                        }
                      >
                        {s.recompute}
                      </Button>
                    )}
                  </div>
                )}
              </Field>
              <div className="sm:col-span-2">
                <SecretPicker
                  listOn={folder?.organization_id ?? null}
                  createOn={false}
                  value={draft.secret}
                  onChange={(id) => set({ secret: id })}
                  manageHref={
                    folder?.organization_id
                      ? `/${locale}/catalog?node=${folder.organization_id}&tab=settings&section=secrets`
                      : `/${locale}/admin/secrets`
                  }
                  allowNone
                />
              </div>
            </>
          ) : (
            <Field label={s.path} className="sm:col-span-2">
              {(f) => (
                <Input
                  {...f}
                  className="font-mono"
                  value={draft.path}
                  onChange={(e) => set({ path: e.target.value })}
                />
              )}
            </Field>
          )}
          <Field label={s.branch} hint={s.branchHint}>
            {(f) => (
              <Input
                {...f}
                value={draft.branch}
                disabled={draft.kind === "local_dir"}
                onChange={(e) => set({ branch: e.target.value })}
              />
            )}
          </Field>
          <Field label={s.root} hint={s.rootHint}>
            {(f) => (
              <Input
                {...f}
                className="font-mono"
                value={draft.root}
                onChange={(e) => set({ root: e.target.value })}
              />
            )}
          </Field>
          <Field label={s.interval} hint={s.intervalHint}>
            {(f) => (
              <Input
                {...f}
                inputMode="numeric"
                value={draft.interval}
                onChange={(e) =>
                  set({ interval: e.target.value.replace(/\D/g, "") })
                }
              />
            )}
          </Field>
          <Field label={s.newStatus}>
            {(f) => (
              <Select
                id={f.id}
                value={draft.status}
                onChange={(e) => set({ status: e.target.value })}
              >
                <option value="published">{t.statusPublished}</option>
                <option value="draft">{t.statusDraft}</option>
              </Select>
            )}
          </Field>
          <Field label={s.versions}>
            {(f) => (
              <Select
                id={f.id}
                value={draft.versions}
                onChange={(e) => set({ versions: e.target.value })}
              >
                <option value="branch">{s.fromBranch}</option>
                {draft.kind !== "local_dir" && (
                  <option value="tags">{s.fromTags}</option>
                )}
              </Select>
            )}
          </Field>
          {draft.versions === "tags" && (
            <Field label={s.tagPattern} hint={s.tagPatternHint}>
              {(f) => (
                <Input
                  {...f}
                  className="font-mono"
                  value={draft.tag_pattern}
                  onChange={(e) => set({ tag_pattern: e.target.value })}
                />
              )}
            </Field>
          )}
          <Message
            note={formError ? { kind: "error", text: formError } : null}
            className="sm:col-span-2"
          />
        </div>
      </Dialog>
      <ConfirmDialog
        open={deleting !== null}
        onOpenChange={(o) => !o && setDeleting(null)}
        title={s.delete}
        text={format(s.deleteText, { count: owned })}
        confirm={s.delete}
        onConfirm={remove}
      />
    </>
  );
}
