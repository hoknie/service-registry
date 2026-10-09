"use client";

import type { KnowledgeSource, SourceKind } from "@/lib/api";

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
  token: string;
  noToken: boolean;
  path: string;
  workingTree: boolean;
};

export const emptyDraft: SourceDraft = {
  kind: "remote",
  forge: "gitlab",
  url: "",
  apiUrl: "",
  token: "",
  noToken: false,
  path: "",
  workingTree: true,
};

export function draftOf(s: KnowledgeSource | null): SourceDraft {
  if (!s) return emptyDraft;
  return {
    kind: s.kind,
    forge: s.forge ?? "gitlab",
    url: s.url ?? "",
    apiUrl: s.api_url ?? "",
    token: "",
    noToken: s.credentials.mode === "none",
    path: s.path ?? "",
    workingTree: s.kind === "local_git" ? s.working_tree : true,
  };
}

export function sourceBody(d: SourceDraft): Record<string, unknown> {
  if (d.kind === "local_git") return { kind: d.kind, path: d.path, working_tree: d.workingTree };
  if (d.kind !== "remote") return { kind: d.kind, path: d.path };
  const out: Record<string, unknown> = { kind: d.kind, forge: d.forge, url: d.url, api_url: d.apiUrl };
  if (d.noToken) out.credentials = null;
  else if (d.token) out.credentials = { token: d.token };
  return out;
}

export function SourceFields({
  value,
  onChange,
  labels: t,
}: {
  value: SourceDraft;
  onChange: (v: SourceDraft) => void;
  labels: CatalogLabels["docs"]["source"];
}) {
  const set = (patch: Partial<SourceDraft>) => onChange({ ...value, ...patch });
  if (value.kind !== "remote") {
    return (
      <>
        <Field label={t.path} hint={value.kind === "local_git" ? `${t.pathHint} ${t.gitHint}` : t.pathHint}>
          {(p) => <Input {...p} required className="font-mono" value={value.path} onChange={(e) => set({ path: e.target.value })} />}
        </Field>
        {value.kind === "local_git" && (
          <div className="grid gap-1">
            <Checkbox label={t.workingTree} checked={value.workingTree} onChange={(e) => set({ workingTree: e.target.checked })} />
            <p className="pl-6.5 text-xs text-muted">{t.workingTreeHint}</p>
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
      <Checkbox label={t.noToken} checked={value.noToken} onChange={(e) => set({ noToken: e.target.checked })} />
      {!value.noToken && (
        <Field label={t.token} hint={t.tokenHint}>
          {(p) => <Input {...p} type="password" autoComplete="off" value={value.token} onChange={(e) => set({ token: e.target.value })} />}
        </Field>
      )}
    </>
  );
}
