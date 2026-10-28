"use client";

import { FolderInput, ImageIcon, ListFilter, SlidersHorizontal, Trash2 } from "lucide-react";
import { useState } from "react";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { apiForm, apiSend, errorCode } from "@/lib/api";
import { skillUrl, type Skill } from "@/lib/skills";
import { loadVersions, patchSkill } from "@/lib/skillsApi";

import { DangerZone } from "../places/DangerZone";
import { MarkdownField } from "../places/MarkdownField";
import { PlaceMoveDialog } from "../places/PlaceMoveDialog";
import { pickSection, SettingsSections } from "../places/SettingsSections";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { TagInput } from "../ui/TagInput";
import { SkillImage } from "./SkillCard";
import { VersionPolicyForm } from "./VersionPolicyForm";

type Props = {
  skill: Skill;
  labels: Messages["skills"];
  onSaved: (s: Skill) => void;
  section: string | null;
  sectionHref: (key: string) => string;
  canGlobal: boolean;
  onDelete: () => void;
  onReload: () => void;
};

export function SkillSettings({ skill, labels: t, onSaved, section: asked, sectionHref, canGlobal, onDelete, onReload }: Props) {
  const s = t.settings;
  const { errors, common } = useUiText();
  const p = common.places;
  const [tags, setTags] = useState(skill.tags.join("\n"));
  const [slug, setSlug] = useState(skill.slug);
  const [about, setAbout] = useState(skill.about);
  const [publishing, setPublishing] = useState(false);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const [moving, setMoving] = useState(false);

  const fail = (e: unknown) => setNote({ kind: "error", text: errorText(errors, errorCode(e)) });

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      const body: Record<string, unknown> = { about };
      if (!skill.managed) {
        body.tags = tags.split("\n").filter(Boolean);
        if (slug !== skill.slug) body.slug = slug;
      }
      onSaved(await patchSkill(skill.id, body));
      setNote({ kind: "success", text: s.saved });
    } catch (e) {
      fail(e);
    } finally {
      setBusy(false);
    }
  };

  const publish = async (status: "draft" | "published" | "disabled") => {
    setPublishing(true);
    try {
      onSaved(await patchSkill(skill.id, { status }));
      onReload();
    } catch (e) {
      fail(e);
    } finally {
      setPublishing(false);
    }
  };

  const upload = async (file: File | undefined) => {
    if (!file) return;
    const form = new FormData();
    form.append("image", file);
    try {
      onSaved(await apiForm<Skill>("PUT", skillUrl(skill.id, "/image"), form));
    } catch (e) {
      fail(e);
    }
  };

  const removeImage = async () => {
    try {
      await apiSend("DELETE", skillUrl(skill.id, "/image"));
      onSaved({ ...skill, image_url: null });
    } catch (e) {
      fail(e);
    }
  };

  const sections = [
    { key: "general", label: p.general, icon: SlidersHorizontal },
    { key: "image", label: p.image, icon: ImageIcon },
    { key: "policy", label: p.policy, icon: ListFilter },
  ];
  const section = pickSection(
    sections.map((x) => x.key),
    asked,
  );

  return (
    <SettingsSections label={p.menu} sections={sections} current={section ?? "general"} href={sectionHref}>
      {section === "general" && (
        <div className="grid gap-6">
          <Panel title={s.general}>
            <div className="grid gap-4">
              <Field label={s.tags} hint={skill.managed ? s.tagsManaged : undefined}>
                {(f) => <TagInput id={f.id} value={tags} disabled={skill.managed} onChange={setTags} />}
              </Field>
              <Field label={s.slug}>
                {(f) => <Input {...f} className="font-mono" value={slug} disabled={skill.managed} onChange={(e) => setSlug(e.target.value.toLowerCase())} />}
              </Field>
              <MarkdownField label={s.about} hint={s.aboutHint} rows={5} maxLength={4000} value={about} onChange={setAbout} />
              <Message note={note} />
              <div>
                <Button variant="primary" busy={busy} onClick={save}>
                  {s.save}
                </Button>
              </div>
            </div>
          </Panel>
          <Panel title={s.publication}>
            <div className="flex flex-wrap items-center justify-between gap-3">
              <p className="min-w-0 flex-1 text-sm text-ink-2">
                {skill.status === "draft" ? s.publicationDraft : skill.status === "disabled" ? t.off.disabledText : s.publicationPublished}
              </p>
              <div className="flex flex-wrap gap-2">
                {skill.status !== "disabled" && (
                  <Button busy={publishing} onClick={() => void publish(skill.status === "draft" ? "published" : "draft")}>
                    {skill.status === "draft" ? t.page.publish : t.page.unpublish}
                  </Button>
                )}
                {skill.status !== "draft" && (
                  <Button busy={publishing} onClick={() => void publish(skill.status === "disabled" ? "published" : "disabled")}>
                    {skill.status === "disabled" ? t.off.enable : t.off.disable}
                  </Button>
                )}
              </div>
            </div>
          </Panel>
          {!skill.managed && (
            <DangerZone
              title={p.danger}
              rows={[
                {
                  key: "move",
                  text: p.moveText,
                  action: (
                    <Button onClick={() => setMoving(true)}>
                      <FolderInput aria-hidden="true" />
                      {p.move}
                    </Button>
                  ),
                },
                {
                  key: "delete",
                  text: p.deleteSkill,
                  action: (
                    <Button variant="danger" onClick={onDelete}>
                      <Trash2 aria-hidden="true" />
                      {t.page.delete}
                    </Button>
                  ),
                },
              ]}
            />
          )}
          <PlaceMoveDialog
            open={moving}
            onOpenChange={setMoving}
            base="/v1/skills"
            name={skill.title}
            global={t.scopeGlobal}
            canGlobal={canGlobal}
            current={{ folder: skill.folder_id, organization: skill.organization_id }}
            move={async (to) => onSaved(await patchSkill(skill.id, { folder_id: to.folder, organization_id: to.organization }))}
            onMoved={() => undefined}
          />
        </div>
      )}
      {section === "image" && (
        <Panel title={s.image} description={s.imageHint}>
          <div className="flex flex-wrap items-center gap-4">
            <SkillImage skill={skill} size="lg" />
            <label className="cursor-pointer">
              <span className="sr-only">{s.uploadImage}</span>
              <input type="file" accept="image/png,image/jpeg,image/webp,image/svg+xml" className="text-sm" onChange={(e) => void upload(e.target.files?.[0])} />
            </label>
            {skill.image_url && (
              <Button variant="ghost" onClick={removeImage}>
                {s.removeImage}
              </Button>
            )}
          </div>
          <Message note={note} />
        </Panel>
      )}
      {section === "policy" && (
        <VersionPolicyForm path={skillUrl(skill.id, "/version-policy")} labels={t} versions={[]} preview={async (draft) => (await loadVersions(skill.id, draft)).items} />
      )}
    </SettingsSections>
  );
}
