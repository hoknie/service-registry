"use client";

import { Upload } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { ApiError, apiForm, errorCode } from "@/lib/api";
import { can } from "@/lib/rights";
import {
  humanSize,
  manifestVersion,
  nextPatch,
  SKILLS_BASE,
  skillUrl,
  validVersion,
  type Skill,
  type SkillFolder,
} from "@/lib/skills";
import { loadFolder, loadSkill } from "@/lib/skillsApi";

import { useSession } from "../SessionProvider";
import { Crumbs } from "../shell/crumbs";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Checkbox, Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { PageHeader, Panel } from "../ui/Panel";
import { Segmented } from "../ui/Segmented";
import { Select } from "../ui/Select";
import { useOrganizations } from "../places/spaces";

type Props = { locale: Locale; labels: Messages["skills"] };

type Picked = { files: { file: File; path: string }[]; size: number };

export function UploadForm({ locale, labels: t }: Props) {
  const u = t.uploadPage;
  const params = useSearchParams();
  const router = useRouter();
  const { errors } = useUiText();
  const { session } = useSession();
  const user = session.status === "signed-in" ? session.user : null;
  const folderId = params.get("folder");
  const skillId = params.get("skill");
  const orgs = useOrganizations();
  const [folder, setFolder] = useState<SkillFolder | null>(null);
  const [skill, setSkill] = useState<Skill | null>(null);
  const [mode, setMode] = useState<"zip" | "folder">("zip");
  const [picked, setPicked] = useState<Picked | null>(null);
  const [version, setVersion] = useState("");
  const [scope, setScope] = useState(() =>
    params.get("space") === "global" ? "" : (params.get("space") ?? ""),
  );
  const [publish, setPublish] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const canGlobal = can(user, "skills", "create");
  const effectiveScope = scope || (canGlobal ? "" : (orgs[0]?.id ?? ""));

  useEffect(() => {
    if (folderId)
      loadFolder(folderId)
        .then(setFolder)
        .catch(() => setFolder(null));
    if (skillId)
      loadSkill(skillId)
        .then((s) => {
          setSkill(s);
          setVersion(nextPatch(s.version?.version));
        })
        .catch(() => setSkill(null));
  }, [folderId, skillId]);

  const choose = async (list: FileList | null) => {
    if (!list || list.length === 0) return;
    const files = [...list].map((file) => ({
      file,
      path: file.webkitRelativePath || file.name,
    }));
    setPicked({ files, size: files.reduce((n, f) => n + f.file.size, 0) });
    const manifest = files.find(
      (f) => f.path === "SKILL.md" || /^[^/]+\/SKILL\.md$/.test(f.path),
    );
    if (manifest) {
      const v = manifestVersion(await manifest.file.text());
      if (v) setVersion(v);
    }
  };

  const submit = async () => {
    if (!picked) return;
    setBusy(true);
    setNote(null);
    const form = new FormData();
    if (version) form.append("version", version);
    if (!skillId) {
      if (folderId) form.append("folder_id", folderId);
      else if (effectiveScope) form.append("organization_id", effectiveScope);
      form.append("status", publish ? "published" : "draft");
    }
    for (const f of picked.files)
      form.append(
        mode === "zip" ? "archive" : "file",
        f.file,
        mode === "zip" ? f.file.name : f.path,
      );
    try {
      const saved = skillId
        ? await apiForm<Skill>("PUT", skillUrl(skillId, "/content"), form)
        : await apiForm<Skill>("POST", SKILLS_BASE, form);
      router.push(`/${locale}/skills/skill?id=${saved.id}`);
    } catch (e) {
      let text = errorText(errors, errorCode(e));
      if (e instanceof ApiError && e.code === "conflict.skill_version_exists")
        text = `${text} ${u.bumpVersion}`;
      if (
        e instanceof ApiError &&
        e.code === "validation.skill_manifest_invalid"
      )
        text = `${text} ${format(u.fields, { fields: e.message })}`;
      setNote({ kind: "error", text });
    } finally {
      setBusy(false);
    }
  };

  const crumbs = [{ label: t.title, href: `/${locale}/skills` }];
  if (folder)
    crumbs.push({
      label: folder.name,
      href: `/${locale}/skills?folder=${folder.id}`,
    });
  if (skill)
    crumbs.push({
      label: skill.title,
      href: `/${locale}/skills/skill?id=${skill.id}`,
    });
  const badVersion = version !== "" && !validVersion(version);
  return (
    <>
      <Crumbs
        items={[...crumbs, { label: skillId ? u.newVersion : u.title }]}
      />
      <PageHeader
        glyph={Upload}
        title={skillId ? u.newVersion : u.title}
        lead={u.lead}
      />
      <Panel>
        <div className="grid max-w-2xl gap-5">
          {!skillId && !folderId && (
            <Field label={u.scope}>
              {(f) => (
                <Select
                  id={f.id}
                  value={effectiveScope}
                  onChange={(e) => setScope(e.target.value)}
                >
                  {canGlobal && <option value="">{t.scopeGlobal}</option>}
                  {orgs.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          )}
          {folder && (
            <p className="text-sm text-muted">{`${u.destination}: ${folder.name}`}</p>
          )}
          <Segmented
            label={u.lead}
            value={mode}
            onChange={(v) => {
              setMode(v as "zip" | "folder");
              setPicked(null);
            }}
            options={[
              { value: "zip", label: u.zip },
              { value: "folder", label: u.folder },
            ]}
          />
          <label className="grid cursor-pointer gap-2 rounded-card border border-dashed border-line-strong p-6 text-center text-sm text-muted hover:bg-surface-2">
            <span>{mode === "zip" ? u.chooseZip : u.chooseFolder}</span>
            {mode === "zip" ? (
              <input
                type="file"
                accept=".zip,application/zip"
                className="mx-auto text-sm"
                onChange={(e) => void choose(e.target.files)}
              />
            ) : (
              <input
                type="file"
                multiple
                className="mx-auto text-sm"
                {...{ webkitdirectory: "" }}
                onChange={(e) => void choose(e.target.files)}
              />
            )}
            {picked && (
              <span className="text-ink-2">
                {format(u.files, {
                  count: picked.files.length,
                  size: humanSize(picked.size),
                })}
              </span>
            )}
          </label>
          <Field
            label={u.version}
            hint={u.versionHint}
            error={badVersion ? u.invalidVersion : undefined}
          >
            {(f) => (
              <Input
                {...f}
                className="font-mono"
                value={version}
                onChange={(e) => setVersion(e.target.value.trim())}
              />
            )}
          </Field>
          {!skillId && (
            <Checkbox
              checked={publish}
              onChange={(e) => setPublish(e.target.checked)}
              label={u.publish}
            />
          )}
          <Message note={note} />
          <div>
            <Button
              variant="primary"
              busy={busy}
              disabled={!picked || badVersion}
              onClick={submit}
            >
              <Upload aria-hidden="true" className="size-4" /> {u.submit}
            </Button>
          </div>
        </div>
      </Panel>
    </>
  );
}
