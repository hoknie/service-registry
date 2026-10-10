"use client";

import { useParams } from "next/navigation";

import type { Locale } from "@/i18n/config";
import type { KnowledgeSource, SourceKind } from "@/lib/api";

import { catalogHref } from "../catalog/shared";
import { SecretPicker } from "../secrets/SecretPicker";
import { Checkbox, Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import type { CatalogLabels } from "../catalog/shared";

export const FORGES = ["github", "gitlab", "gitea", "forgejo"] as const;
export const SOURCE_KINDS: SourceKind[] = ["remote", "local_dir", "local_git"];

export type SourceDraft = {
  kind: SourceKind;
  forge: (typeof FORGES)[number];
  url: string;
  apiUrl: string;
  secret: string | null;
  current: string | null;
  legacy: boolean;
  noToken: boolean;
  path: string;
  workingTree: boolean;
  includeIgnored: boolean;
};

export const emptyDraft: SourceDraft = {
  kind: "remote",
  forge: "gitlab",
  url: "",
  apiUrl: "",
  secret: null,
  current: null,
  legacy: false,
  noToken: false,
  path: "",
  workingTree: true,
  includeIgnored: false,
};

export function draftOf(s: KnowledgeSource | null): SourceDraft {
  if (!s) return emptyDraft;
  return {
    kind: s.kind,
    forge: s.forge ?? "gitlab",
    url: s.url ?? "",
    apiUrl: s.api_url ?? "",
    secret: s.credentials.secret?.id ?? null,
    current: s.credentials.secret?.id ?? null,
    legacy: s.credentials.mode === "legacy",
    noToken: s.credentials.mode === "none",
    path: s.path ?? "",
    workingTree: s.kind === "local_git" ? s.working_tree : true,
    includeIgnored: s.kind === "local_git" && s.include_ignored,
  };
}

export function sourceBody(d: SourceDraft): Record<string, unknown> {
  if (d.kind === "local_git") {
    return { kind: d.kind, path: d.path, working_tree: d.workingTree, include_ignored: d.workingTree && d.includeIgnored };
  }
  if (d.kind !== "remote") return { kind: d.kind, path: d.path };
  const out: Record<string, unknown> = { kind: d.kind, forge: d.forge, url: d.url, api_url: d.apiUrl };
  if (d.noToken) out.credentials = null;
  else if (d.secret && d.secret !== d.current) out.credentials = { secret_id: d.secret };
  return out;
}

export function secretChanged(d: SourceDraft): boolean {
  return d.secret !== d.current;
}

export type SecretScope = { listOn: string | null; parent: string | null; canCreate: boolean };

export function SourceFields({
  value,
  onChange,
  labels: t,
  scope,
}: {
  value: SourceDraft;
  onChange: (v: SourceDraft) => void;
  labels: CatalogLabels["docs"]["source"];
  scope: SecretScope;
}) {
  const { locale } = useParams<{ locale: string }>();
  const set = (patch: Partial<SourceDraft>) => onChange({ ...value, ...patch });
  if (value.kind !== "remote") {
    return (
      <>
        <Field label={t.path} hint={value.kind === "local_git" ? `${t.pathHint} ${t.gitHint}` : t.pathHint}>
          {(p) => <Input {...p} required className="font-mono" value={value.path} onChange={(e) => set({ path: e.target.value })} />}
        </Field>
        {value.kind === "local_git" && (
          <div className="grid gap-1">
            <Checkbox
              label={t.workingTree}
              checked={value.workingTree}
              onChange={(e) => set({ workingTree: e.target.checked, includeIgnored: e.target.checked && value.includeIgnored })}
            />
            <p className="pl-6.5 text-xs text-muted">{t.workingTreeHint}</p>
            <Checkbox
              label={t.includeIgnored}
              checked={value.workingTree && value.includeIgnored}
              disabled={!value.workingTree}
              onChange={(e) => set({ includeIgnored: e.target.checked })}
              className="mt-1 has-disabled:cursor-not-allowed has-disabled:opacity-60"
            />
            <p className="pl-6.5 text-xs text-muted">{t.includeIgnoredHint}</p>
          </div>
        )}
      </>
    );
  }
  return (
    <>
      <Field label={t.forge}>
        {(p) => (
          <Select {...p} value={value.forge} onChange={(e) => set({ forge: e.target.value as SourceDraft["forge"] })}>
            {FORGES.map((f) => (
              <option key={f} value={f}>
                {f}
              </option>
            ))}
          </Select>
        )}
      </Field>
      <Field label={t.url} hint={t.urlHint}>
        {(p) => <Input {...p} required value={value.url} onChange={(e) => set({ url: e.target.value })} />}
      </Field>
      <Field label={t.apiUrl} hint={t.apiUrlHint}>
        {(p) => <Input {...p} value={value.apiUrl} onChange={(e) => set({ apiUrl: e.target.value })} />}
      </Field>
      <SecretPicker
        listOn={scope.listOn}
        createOn={scope.canCreate && scope.parent ? scope.parent : false}
        value={value.noToken ? null : value.secret}
        onChange={(secret) => set({ secret, noToken: secret === null })}
        manageHref={scope.parent ? catalogHref(locale as Locale, scope.parent, "settings", null, null, "secrets") : `/${locale}/admin/secrets`}
        allowNone
        legacy={value.legacy && !value.noToken}
      />
    </>
  );
}
