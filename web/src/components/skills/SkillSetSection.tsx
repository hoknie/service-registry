"use client";

import { Folder, Sparkles, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, errorCode } from "@/lib/api";
import { SKILLS_BASE, type SkillFolder, type SkillPage, type SkillSet, type SkillSetItem } from "@/lib/skills";
import { loadSkillSet, putSkillSet } from "@/lib/skillsApi";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Combobox } from "../ui/Combobox";
import { Message } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Select } from "../ui/Select";
import { Skeleton } from "../ui/Skeleton";

type Props = { org: string; labels: Messages["skills"] };

type Candidate = { kind: "folder" | "skill"; item: SkillSetItem };

async function globalCandidates(): Promise<Candidate[]> {
  const out: Candidate[] = [];
  const walk = async (parent: string | null, prefix: string) => {
    const level = await apiGet<{ items: SkillFolder[] }>(`${SKILLS_BASE}/folders${parent ? `?parent_id=${parent}` : ""}`);
    for (const f of level.items.filter((x) => x.organization_id === null)) {
      const path = prefix ? `${prefix}/${f.slug}` : f.slug;
      out.push({ kind: "folder", item: { id: f.id, path, name: f.name, skills: f.skills } });
      if (f.folders > 0 && path.split("/").length < 4) await walk(f.id, path);
    }
  };
  await walk(null, "");
  const skills = await apiGet<SkillPage>(`${SKILLS_BASE}?scope=global&limit=100`);
  for (const s of skills.items) out.push({ kind: "skill", item: { id: s.id, path: s.path, name: s.title, status: s.status } });
  return out;
}

export function SkillSetSection({ org, labels: t }: Props) {
  const s = t.set;
  const { errors } = useUiText();
  const [set, setSet] = useState<SkillSet | null>(null);
  const [candidates, setCandidates] = useState<Candidate[]>([]);
  const [pick, setPick] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    try {
      setSet(await loadSkillSet(org));
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  }, [org, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
    globalCandidates()
      .then(setCandidates)
      .catch(() => setCandidates([]));
  }, [load]);

  if (!set) return error ? <Message note={{ kind: "error", text: error }} /> : <Skeleton className="h-40 w-full" />;

  const chosen = new Set([...set.folders, ...set.skills].map((x) => x.id));
  const add = (id: string) => {
    const c = candidates.find((x) => x.item.id === id);
    if (!c || chosen.has(id)) return;
    setSet({ ...set, folders: c.kind === "folder" ? [...set.folders, c.item] : set.folders, skills: c.kind === "skill" ? [...set.skills, c.item] : set.skills });
    setPick("");
  };
  const remove = (id: string) => setSet({ ...set, folders: set.folders.filter((x) => x.id !== id), skills: set.skills.filter((x) => x.id !== id) });
  const save = async () => {
    setBusy(true);
    try {
      setSet(await putSkillSet(org, { mode: set.mode, folders: set.folders.map((x) => x.id), skills: set.skills.map((x) => x.id) }));
      toast.success(s.saved);
    } catch (e) {
      toast.error(errorText(errors, errorCode(e)));
    } finally {
      setBusy(false);
    }
  };
  const row = (x: SkillSetItem, kind: "folder" | "skill") => (
    <li key={x.id} className="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
      <span className="flex min-w-0 items-center gap-2">
        {kind === "folder" ? <Folder aria-hidden="true" className="size-4 shrink-0 text-muted" /> : <Sparkles aria-hidden="true" className="size-4 shrink-0 text-signal" />}
        <span className="min-w-0">
          <span className="block truncate font-medium text-ink">{x.name}</span>
          <span className="block truncate font-mono text-xs text-muted">{x.path}</span>
        </span>
      </span>
      <span className="flex shrink-0 items-center gap-2 text-xs text-muted">
        {kind === "folder" && format(s.count, { count: x.skills ?? 0 })}
        {set.can_manage && (
          <Button size="sm" variant="ghost" aria-label={format(s.remove, { name: x.name })} onClick={() => remove(x.id)}>
            <X aria-hidden="true" />
          </Button>
        )}
      </span>
    </li>
  );
  return (
    <Panel title={s.title} description={s.lead}>
      <div className="grid gap-4 text-sm">
        <p className="text-muted">{s.note}</p>
        <Select aria-label={s.title} className="w-64" value={set.mode} disabled={!set.can_manage} onChange={(e) => setSet({ ...set, mode: e.target.value as SkillSet["mode"] })}>
          <option value="all">{s.all}</option>
          <option value="selected">{s.selected}</option>
        </Select>
        {set.mode === "selected" && (
          <>
            {set.can_manage && (
              <Combobox
                aria-label={s.search}
                placeholder={s.search}
                value={pick}
                onChange={add}
                options={candidates.filter((c) => !chosen.has(c.item.id)).map((c) => ({ value: c.item.id, label: `${c.item.name} — ${c.item.path}` }))}
              />
            )}
            {chosen.size === 0 ? (
              <p className="text-muted">{s.empty}</p>
            ) : (
              <ul className="grid gap-2">
                {set.folders.map((x) => row(x, "folder"))}
                {set.skills.map((x) => row(x, "skill"))}
              </ul>
            )}
          </>
        )}
        {set.can_manage ? (
          <div>
            <Button variant="primary" busy={busy} onClick={save}>
              {s.save}
            </Button>
          </div>
        ) : (
          <p className="text-muted">{s.readonly}</p>
        )}
      </div>
    </Panel>
  );
}
