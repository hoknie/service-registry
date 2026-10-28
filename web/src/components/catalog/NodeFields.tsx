"use client";

import type { ReactNode } from "react";

import type { SourceKind } from "@/lib/api";

import { MarkdownField } from "../places/MarkdownField";
import { Checkbox, Field, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import type { Suggest } from "../ui/Suggestions";
import { labelError as labelRule } from "../ui/TagInput";
import { SOURCE_KINDS, SourceFields, type SecretScope, type SourceDraft } from "../knowledge/SourceFields";
import { BranchInput } from "./BranchInput";
import { type CatalogLabels } from "./shared";

export type Repo = { hosting: string; repo_url: string; default_branch: string };
export type RepoMode = "link" | SourceKind;

export const noRepo: Repo = { hosting: "", repo_url: "", default_branch: "" };

export const labelError = (tag: string) => labelRule(tag, "label");

export function slugify(name: string): string {
  return name
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 63);
}

export function RepoSection({
  labels,
  mode,
  onMode,
  repo,
  onRepo,
  draft,
  onDraft,
  scope,
  managed,
  branchSuggest,
  branchNote,
  children,
}: {
  labels: CatalogLabels;
  mode: RepoMode;
  onMode: (m: RepoMode) => void;
  repo: Repo;
  onRepo: (v: Repo) => void;
  draft: SourceDraft;
  onDraft: (v: SourceDraft) => void;
  scope: SecretScope;
  managed?: boolean;
  branchSuggest?: Suggest;
  branchNote?: ReactNode;
  children?: ReactNode;
}) {
  const s = labels.docs.source;
  return (
    <fieldset className="grid gap-4 rounded-lg border border-line p-4">
      <legend className="px-1 text-sm font-medium text-ink-2">
        {labels.repo.section}
        {managed && <span className="ml-2 font-normal text-muted">· {labels.edit.fromRepoSync}</span>}
      </legend>
      {managed && <p className="text-xs text-muted">{labels.repo.managedSource}</p>}
      <fieldset disabled={managed} className="grid gap-4">
        <Field label={labels.repo.mode}>
          {(p) => (
            <Select
              {...p}
              value={mode}
              onChange={(e) => {
                const next = e.target.value as RepoMode;
                onMode(next);
                if (next !== "link") onDraft({ ...draft, kind: next });
              }}
            >
              <option value="link">{labels.repo.link}</option>
              {SOURCE_KINDS.map((k) => (
                <option key={k} value={k}>
                  {s.kinds[k]}
                </option>
              ))}
            </Select>
          )}
        </Field>
        {mode === "link" ? (
          <>
            <Field label={labels.repo.hosting}>
              {(p) => (
                <Select {...p} value={repo.hosting} onChange={(e) => onRepo({ ...repo, hosting: e.target.value })}>
                  <option value="">{labels.repo.noHosting}</option>
                  <option value="github">GitHub</option>
                  <option value="gitlab">GitLab</option>
                  <option value="forgejo">Forgejo</option>
                  <option value="gitea">Gitea</option>
                </Select>
              )}
            </Field>
            <Field label={labels.repo.url}>
              {(p) => <Input {...p} type="url" value={repo.repo_url} onChange={(e) => onRepo({ ...repo, repo_url: e.target.value })} />}
            </Field>
          </>
        ) : (
          <SourceFields value={draft} onChange={onDraft} labels={s} scope={scope} />
        )}
        {branchNote}
        <Field label={labels.repo.branch} hint={managed ? labels.edit.fromRepoSync : labels.repo.branchHint}>
          {(p) => (
            <BranchInput
              {...p}
              readOnly={managed}
              suggest={branchSuggest}
              value={repo.default_branch}
              onChange={(v) => onRepo({ ...repo, default_branch: v })}
            />
          )}
        </Field>
      </fieldset>
      {children}
    </fieldset>
  );
}

export function ObservationField({ labels, value, onChange }: { labels: CatalogLabels; value: boolean; onChange: (v: boolean) => void }) {
  return (
    <div className="grid gap-1">
      <Checkbox label={labels.repo.clusterObservation} checked={value} onChange={(e) => onChange(e.target.checked)} aria-describedby="cluster-observation-hint" />
      <p id="cluster-observation-hint" className="text-xs text-muted">
        {labels.repo.clusterObservationHint}
      </p>
    </div>
  );
}

export function DescriptionField({
  labels,
  value,
  onChange,
  readOnly,
  hint,
  rows = 6,
}: {
  labels: CatalogLabels;
  value: string;
  onChange: (v: string) => void;
  readOnly?: boolean;
  hint?: string;
  rows?: number;
}) {
  return <MarkdownField label={labels.create.description} value={value} onChange={onChange} readOnly={readOnly} hint={hint} rows={rows} />;
}
