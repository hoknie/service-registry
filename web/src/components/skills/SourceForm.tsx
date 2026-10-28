"use client";

import { useEffect, useState, type Dispatch, type SetStateAction } from "react";

import type { Locale } from "@/i18n/config";
import type { Messages } from "@/i18n/messages";
import { guessHosting, parseTreeLink, sourceApiUrl } from "@/lib/hostingAddress";
import type { SkillFolder, SkillSource, SkillSourceKind, SourceLayout } from "@/lib/skills";
import { loadSkillDefaults } from "@/lib/skillsApi";
import { toTags } from "@/lib/tags";

import { SecretPicker } from "../secrets/SecretPicker";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import { TagInput } from "../ui/TagInput";

const HOSTINGS = ["github", "gitlab", "gitea", "forgejo"] as const;

export type Draft = {
  kind: SkillSourceKind;
  hosting: string;
  url: string;
  api_url: string;
  path: string;
  secret: string | null;
  branch: string;
  roots: string;
  layout: SourceLayout;
  group_field: string;
  interval: string;
  status: string;
  versions: string;
  tag_pattern: string;
  hostingTouched: boolean;
  apiTouched: boolean;
};

export const emptyDraft: Draft = {
  kind: "remote",
  hosting: "gitlab",
  url: "",
  api_url: "",
  path: "",
  secret: null,
  branch: "",
  roots: "",
  layout: "flat",
  group_field: "",
  interval: "",
  status: "published",
  versions: "branch",
  tag_pattern: "v{version}",
  hostingTouched: false,
  apiTouched: false,
};

export function draftOf(s: SkillSource): Draft {
  const hosting = s.hosting ?? "gitlab";
  return {
    kind: s.kind,
    hosting,
    url: s.url ?? "",
    api_url: s.api_url ?? "",
    path: s.path ?? "",
    secret: s.credentials?.secret_id ?? null,
    branch: s.branch,
    roots: s.roots.filter(Boolean).join("\n"),
    layout: s.layout,
    group_field: s.group_field ?? "",
    interval: s.interval_secs ? String(s.interval_secs) : "",
    status: s.status,
    versions: s.versions,
    tag_pattern: s.tag_pattern,
    hostingTouched: s.hosting !== null,
    apiTouched: !!s.api_url && s.api_url !== sourceApiUrl(hosting, s.url ?? ""),
  };
}

export function withRemote(
  d: Draft,
  patch: Partial<Pick<Draft, "hosting" | "url" | "api_url">>,
): Draft {
  const next = { ...d, ...patch };
  if (patch.hosting !== undefined) next.hostingTouched = true;
  if (patch.api_url !== undefined) next.apiTouched = true;
  const link = patch.url !== undefined ? parseTreeLink(patch.url) : null;
  if (link) {
    next.url = link.url;
    if (!next.branch.trim()) next.branch = link.branch;
    if (!next.roots.trim() && link.root) next.roots = link.root;
  }
  if (patch.url !== undefined && !next.hostingTouched)
    next.hosting = guessHosting(next.url) ?? next.hosting;
  if (
    !next.apiTouched &&
    (patch.url !== undefined || patch.hosting !== undefined)
  )
    next.api_url = sourceApiUrl(next.hosting, next.url) ?? "";
  return next;
}

export function bodyOf(d: Draft) {
  const remote = d.kind === "remote";
  return {
    kind: d.kind,
    hosting: remote ? d.hosting : "",
    url: remote ? d.url : "",
    api_url: remote ? d.api_url : "",
    path: remote ? "" : d.path,
    credentials: remote && d.secret ? { secret_id: d.secret } : null,
    branch: d.branch,
    roots: toTags(d.roots),
    layout: d.layout,
    group_field: d.layout === "field" ? d.group_field.trim() : null,
    interval_secs: d.interval ? Number(d.interval) : null,
    status: d.status,
    versions: d.versions,
    tag_pattern: d.tag_pattern,
  };
}

type Props = {
  draft: Draft;
  setDraft: Dispatch<SetStateAction<Draft>>;
  folder: Pick<SkillFolder, "organization_id"> | null;
  locale: Locale;
  labels: Messages["skills"];
};

export function SourceFields({ draft, setDraft, folder, locale, labels: t }: Props) {
  const s = t.sourcesPage;
  const set = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));
  const [defaultInterval, setDefaultInterval] = useState<number | null>(null);

  useEffect(() => {
    let alive = true;
    loadSkillDefaults()
      .then((d) => alive && setDefaultInterval(d.sync_interval_secs))
      .catch(() => undefined);
    return () => {
      alive = false;
    };
  }, []);
  return (
    <>
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
              <Field label={s.hosting}>
                {(f) => (
                  <Select
                    id={f.id}
                    value={draft.hosting}
                    onChange={(e) =>
                      setDraft((d) => withRemote(d, { hosting: e.target.value }))
                    }
                  >
                    {HOSTINGS.map((x) => (
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
                            api_url: sourceApiUrl(d.hosting, d.url) ?? "",
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
          <Field label={s.roots} hint={s.rootsHint}>
            {(f) => <TagInput id={f.id} value={draft.roots} onChange={(roots) => set({ roots })} placeholder="skills" />}
          </Field>
          <Field label={s.layout}>
            {(f) => (
              <Select id={f.id} value={draft.layout} onChange={(e) => set({ layout: e.target.value as SourceLayout })}>
                <option value="flat">{s.layoutFlat}</option>
                <option value="tree">{s.layoutTree}</option>
                <option value="field">{s.layoutField}</option>
              </Select>
            )}
          </Field>
          {draft.layout === "field" && (
            <Field label={s.groupField} hint={s.groupFieldHint}>
              {(f) => (
                <Input {...f} className="font-mono" value={draft.group_field} onChange={(e) => set({ group_field: e.target.value })} />
              )}
            </Field>
          )}
          <Field label={s.interval} hint={s.intervalHint}>
            {(f) => (
              <Input
                {...f}
                inputMode="numeric"
                placeholder={defaultInterval === null ? undefined : String(defaultInterval)}
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
                <option value="disabled">{t.statusDisabled}</option>
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
    </>
  );
}
