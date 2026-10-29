"use client";

import { GitBranch, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ago } from "@/i18n/time";
import { apiGet, apiSend, errorCode } from "@/lib/api";
import {
  SKILLS_BASE,
  type SkillFolder,
  type SkillPage,
  type SkillSource,
} from "@/lib/skills";
import { loadFolder } from "@/lib/skillsApi";

import { bodyOf, draftOf, emptyDraft, SourceFields, type Draft } from "./SourceForm";

import { catalogHref } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Badge, type Tone } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";

type Props = { locale: Locale; labels: Messages["skills"]; folderId: string };

const stateTone: Record<SkillSource["state"], Tone> = {
  queued: "cobalt",
  running: "signal",
  idle: "neutral",
  failed: "danger",
  paused: "amber",
};

export function SourcesView({ locale, labels: t, folderId }: Props) {
  const s = t.sourcesPage;
  const { errors } = useUiText();
  const [folder, setFolder] = useState<SkillFolder | null>(null);
  const [sources, setSources] = useState<SkillSource[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState<SkillSource | "new" | null>(null);
  const [draft, setDraft] = useState<Draft>(emptyDraft);
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
    setDraft(src === "new" ? emptyDraft : draftOf(src));
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
              src.roots.filter(Boolean).join(", ") || null,
              src.layout === "field" ? `${s.layoutField}: ${src.group_field}` : src.layout === "tree" ? s.layoutTree : null,
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
                {src.state === "paused" && src.paused_reason && (
                  <span className="text-ink-2">
                    {src.paused_reason === "polling_off" && folder?.organization_id ? (
                      <Link href={catalogHref(locale, folder.organization_id, "settings", null, null, "polling")} className="hover:underline">
                        {s.pausedReason[src.paused_reason]}
                      </Link>
                    ) : (
                      s.pausedReason[src.paused_reason]
                    )}
                  </span>
                )}
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
          <SourceFields draft={draft} setDraft={setDraft} folder={folder} locale={locale} labels={t} />
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
