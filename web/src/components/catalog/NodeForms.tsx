"use client";

import { useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, apiSend, errorCode, type CatalogNode, type KnowledgeSource, type NodeKind, type SourceCheck, type SourceKind } from "@/lib/api";
import { labelsOf, labelsToLines } from "@/lib/tags";
import { useCatalogTree } from "@/lib/catalogTree";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { Checkbox, Field, Input, Textarea } from "../ui/Field";
import { Combobox } from "../ui/Combobox";
import { Segmented } from "../ui/Segmented";
import { Select } from "../ui/Select";
import { hasInvalidTags, LabelsInput, labelError as labelRule } from "../ui/TagInput";
import { Message, type Note } from "../ui/Message";
import { draftOf, emptyDraft, SOURCE_KINDS, SourceFields, sourceBody, secretChanged, type SecretScope, type SourceDraft } from "../knowledge/SourceFields";
import { KeyDialog } from "./KeyDialog";
import { Markdown } from "./Markdown";
import { type CatalogLabels } from "./shared";

type Fail = (e: unknown) => string;
type Repo = { forge: string; repo_url: string; default_branch: string };
type OpenProps = { open: boolean; onOpenChange: (open: boolean) => void };

export function slugify(name: string): string {
  return name
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 63);
}

type RepoMode = "link" | SourceKind;

function RepoSection({
  labels,
  mode,
  onMode,
  repo,
  onRepo,
  draft,
  onDraft,
  scope,
  managed,
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
  children?: ReactNode;
}) {
  const s = labels.docs.source;
  return (
    <fieldset className="grid gap-4 rounded-lg border border-line p-4">
      <legend className="px-1 text-sm font-medium text-ink-2">
        {labels.repo.section}
        {managed && <span className="ml-2 font-normal text-muted">· {labels.edit.fromForge}</span>}
      </legend>
      {managed && <p className="text-xs text-muted">{labels.repo.managedSource}</p>}
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
        <fieldset disabled={managed} className="grid gap-4">
          <Field label={labels.repo.forge}>
            {(p) => (
              <Select {...p} value={repo.forge} onChange={(e) => onRepo({ ...repo, forge: e.target.value })}>
                <option value="">{labels.repo.noForge}</option>
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
        </fieldset>
      ) : (
        <SourceFields value={draft} onChange={onDraft} labels={s} scope={scope} />
      )}
      <Field label={labels.repo.branch}>
        {(p) => (
          <Input
            {...p}
            className="font-mono"
            readOnly={managed}
            value={repo.default_branch}
            onChange={(e) => onRepo({ ...repo, default_branch: e.target.value })}
          />
        )}
      </Field>
      {children}
    </fieldset>
  );
}

const labelError = (tag: string) => labelRule(tag, "label");

const noRepo: Repo = { forge: "", repo_url: "", default_branch: "" };

function ObservationField({ labels, value, onChange }: { labels: CatalogLabels; value: boolean; onChange: (v: boolean) => void }) {
  return (
    <div className="grid gap-1">
      <Checkbox label={labels.repo.clusterObservation} checked={value} onChange={(e) => onChange(e.target.checked)} aria-describedby="cluster-observation-hint" />
      <p id="cluster-observation-hint" className="text-xs text-muted">
        {labels.repo.clusterObservationHint}
      </p>
    </div>
  );
}

function Footer({ form, submit, onCancel, busy, disabled }: { form: string; submit: string; onCancel: () => void; busy?: boolean; disabled?: boolean }) {
  const { common } = useUiText();
  return (
    <>
      {disabled && <span className="mr-auto text-sm text-danger">{common.fixLabels}</span>}
      <Button onClick={onCancel}>{common.cancel}</Button>
      <Button type="submit" form={form} variant="primary" busy={busy} disabled={disabled}>
        {submit}
      </Button>
    </>
  );
}

export function CreateDialog({
  open,
  onOpenChange,
  parent,
  kinds,
  labels,
  fail,
  onCreated,
}: OpenProps & { parent: CatalogNode | null; kinds: NodeKind[]; labels: CatalogLabels; fail: Fail; onCreated: () => void }) {
  const t = labels.create;
  const [kind, setKind] = useState<NodeKind>(kinds[0] ?? "organization");
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [labelLines, setLabelLines] = useState("");
  const [labelDraft, setLabelDraft] = useState("");
  const [repo, setRepo] = useState(noRepo);
  const [mode, setMode] = useState<RepoMode>("link");
  const [draft, setDraft] = useState<SourceDraft>(emptyDraft);
  const [observe, setObserve] = useState(true);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const [secret, setSecret] = useState<string | null>(null);

  const reset = () => {
    setSlug("");
    setSlugTouched(false);
    setName("");
    setDescription("");
    setLabelLines("");
    setLabelDraft("");
    setRepo(noRepo);
    setMode("link");
    setDraft(emptyDraft);
    setObserve(true);
    setNote(null);
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setNote(null);
    try {
      const created = await apiSend<CatalogNode>("POST", "/v1/catalog/nodes", {
        kind,
        parent_id: parent?.id ?? null,
        slug,
        name,
        description,
        labels: labelsOf(labelLines, labelDraft),
        ...(kind === "project"
          ? {
              ...(mode === "link" ? repo : { default_branch: repo.default_branch, source: sourceBody(draft) }),
              cluster_observation: observe,
            }
          : {}),
      });
      reset();
      onOpenChange(false);
      toast.success(t.done);
      if (created.key?.secret) setSecret(created.key.secret);
      onCreated();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={(next) => {
          if (!next) setNote(null);
          onOpenChange(next);
        }}
        title={parent ? format(t.titleIn, { name: parent.name }) : t.title}
        footer={<Footer form="create-node" submit={t.submit} busy={busy} disabled={hasInvalidTags(labelLines, labelError)} onCancel={() => onOpenChange(false)} />}
      >
        <form id="create-node" className="grid gap-4" onSubmit={submit}>
          {kinds.length > 1 && (
            <Field label={t.kind}>
              {(p) => (
                <Select {...p} value={kind} onChange={(e) => setKind(e.target.value as NodeKind)}>
                  {kinds.map((k) => (
                    <option key={k} value={k}>
                      {labels.kinds[k]}
                    </option>
                  ))}
                </Select>
              )}
            </Field>
          )}
          <Field label={t.name}>
            {(p) => (
              <Input
                {...p}
                required
                value={name}
                onChange={(e) => {
                  setName(e.target.value);
                  if (!slugTouched) setSlug(slugify(e.target.value));
                }}
              />
            )}
          </Field>
          <Field label={t.slug} hint={t.slugHint}>
            {(p) => (
              <Input
                {...p}
                required
                className="font-mono"
                value={slug}
                onChange={(e) => {
                  setSlugTouched(true);
                  setSlug(e.target.value);
                }}
              />
            )}
          </Field>
          <Field label={t.description}>{(p) => <Textarea {...p} rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />}</Field>
          <Field label={t.labels} hint={t.labelsHint}>
            {(p) => <LabelsInput {...p} value={labelLines} onChange={setLabelLines} onDraft={setLabelDraft} />}
          </Field>
          {kind === "project" && (
            <RepoSection
              labels={labels}
              mode={mode}
              onMode={setMode}
              repo={repo}
              onRepo={setRepo}
              draft={draft}
              onDraft={setDraft}
              scope={{ listOn: parent?.id ?? null, parent: parent?.id ?? null, canCreate: !!parent?.permissions?.includes("catalog.access") }}
            />
          )}
          {kind === "project" && <ObservationField labels={labels} value={observe} onChange={setObserve} />}
          <Message note={note} />
        </form>
      </Dialog>
      {secret && <KeyDialog secret={secret} labels={labels.keys.dialog} onClose={() => setSecret(null)} />}
    </>
  );
}

export function EditForm({
  node,
  labels,
  fail,
  onSaved,
  readOnly,
}: {
  node: CatalogNode;
  labels: CatalogLabels;
  fail: Fail;
  onSaved: () => void;
  readOnly: boolean;
}) {
  const t = labels.edit;
  const { errors, common } = useUiText();
  const managed = !!node.managed;
  const [slug, setSlug] = useState(node.slug);
  const [name, setName] = useState(node.name);
  const [description, setDescription] = useState(node.description ?? "");
  const [labelLines, setLabelLines] = useState(labelsToLines(node.labels));
  const [labelDraft, setLabelDraft] = useState("");
  const [repo, setRepo] = useState<Repo>({
    forge: node.forge ?? "",
    repo_url: node.repo_url ?? "",
    default_branch: node.default_branch ?? "",
  });
  const [observe, setObserve] = useState(node.cluster_observation ?? true);
  const [note, setNote] = useState<Note>(null);
  const isProject = node.kind === "project";
  const [source, setSource] = useState<KnowledgeSource | null | undefined>(isProject ? undefined : null);
  const [mode, setMode] = useState<RepoMode>("link");
  const [draft, setDraft] = useState<SourceDraft>(emptyDraft);
  const [busy, setBusy] = useState(false);
  const [preview, setPreview] = useState(false);
  const sourceUrl = `/v1/catalog/nodes/${node.id}/knowledge/source` as const;

  useEffect(() => {
    if (!isProject) return;
    let live = true;
    apiGet<KnowledgeSource | null>(sourceUrl)
      .then((s) => {
        if (!live) return;
        setSource(s);
        setMode(s ? s.kind : "link");
        setDraft(draftOf(s));
      })
      .catch(() => live && setSource(null));
    return () => {
      live = false;
    };
  }, [isProject, sourceUrl]);

  const sourceChanged = () => {
    if (mode === "link") return !!source;
    return !source || secretChanged(draft) || JSON.stringify(sourceBody(draft)) !== JSON.stringify(sourceBody(draftOf(source)));
  };

  const check = async () => {
    setBusy(true);
    setNote(null);
    try {
      const res = await apiSend<SourceCheck>("POST", `${sourceUrl}/check`, sourceBody(draft));
      setNote(
        res.ok
          ? { kind: "success", text: format(labels.docs.source.checkOk, { n: res.branches ?? 0 }) }
          : { kind: "error", text: errorText(errors, res.error_code ?? "unknown") },
      );
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setNote(null);
    setBusy(true);
    try {
      const repoFields = mode === "link" ? repo : { default_branch: repo.default_branch };
      await apiSend("PATCH", `/v1/catalog/nodes/${node.id}`, {
        slug,
        name,
        labels: labelsOf(labelLines, labelDraft),
        ...(managed ? {} : { description }),
        ...(isProject && !managed ? repoFields : {}),
        ...(isProject ? { cluster_observation: observe } : {}),
      });
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
      setBusy(false);
      return;
    }
    toast.success(t.done);
    onSaved();
    try {
      if (isProject && source !== undefined && sourceChanged()) {
        if (mode === "link") await apiSend("DELETE", sourceUrl);
        else await apiSend("PUT", sourceUrl, sourceBody(draft));
        onSaved();
      }
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const invalidLabels = hasInvalidTags(labelLines, labelError);
  return (
    <form id="edit-node" className="grid gap-4" onSubmit={submit}>
      <fieldset disabled={readOnly} className="grid gap-4">
        <Field label={labels.create.name}>{(p) => <Input {...p} required value={name} onChange={(e) => setName(e.target.value)} />}</Field>
        <Field label={labels.create.slug} hint={labels.create.slugHint}>
          {(p) => <Input {...p} required className="font-mono" value={slug} onChange={(e) => setSlug(e.target.value)} />}
        </Field>
        {managed && <p className="rounded-md bg-surface-2 px-3 py-2 text-sm text-ink-2">{t.managedNote}</p>}
        <div className="grid gap-1.5">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="text-sm font-medium text-ink-2">{labels.create.description}</span>
            <Segmented
              label={labels.create.description}
              value={preview ? "preview" : "text"}
              onChange={(v) => setPreview(v === "preview")}
              options={[
                { value: "text", label: labels.about.text },
                { value: "preview", label: labels.about.preview },
              ]}
            />
          </div>
          {preview ? (
            <div className="min-h-24 rounded-md border border-line bg-surface p-3">
              {description.trim() ? <Markdown source={description} /> : <p className="text-sm text-muted">{labels.about.empty}</p>}
            </div>
          ) : (
            <>
              <Textarea aria-label={labels.create.description} rows={6} readOnly={managed} value={description} onChange={(e) => setDescription(e.target.value)} />
              <p className="text-xs text-muted">{managed ? t.fromForge : labels.about.markdownHint}</p>
            </>
          )}
        </div>
        <Field label={labels.create.labels} hint={labels.create.labelsHint}>
          {(p) => <LabelsInput {...p} value={labelLines} onChange={setLabelLines} onDraft={setLabelDraft} />}
        </Field>
        {isProject && (
          <RepoSection
            labels={labels}
            mode={mode}
            onMode={setMode}
            repo={repo}
            onRepo={setRepo}
            draft={draft}
            onDraft={setDraft}
            scope={{ listOn: node.id, parent: node.parent_id, canCreate: !!node.permissions?.includes("catalog.access") }}
            managed={managed}
          >
            {mode !== "link" && (
              <div>
                <Button type="button" onClick={() => void check()} disabled={busy}>
                  {labels.repo.checkSource}
                </Button>
              </div>
            )}
          </RepoSection>
        )}
        {node.kind === "project" && <ObservationField labels={labels} value={observe} onChange={setObserve} />}
      </fieldset>
      <Message note={note} />
      {!readOnly && (
        <div className="flex flex-wrap items-center justify-end gap-3">
          {invalidLabels && <span className="mr-auto text-sm text-danger">{common.fixLabels}</span>}
          <Button type="submit" variant="primary" busy={busy || source === undefined} disabled={invalidLabels}>
            {t.submit}
          </Button>
        </div>
      )}
    </form>
  );
}

export function MoveDialog({
  open,
  onOpenChange,
  node,
  labels,
  fail,
  superadmin,
  onMoved,
}: OpenProps & { node: CatalogNode; labels: CatalogLabels; fail: Fail; superadmin: boolean; onMoved: () => void }) {
  const t = labels.move;
  const [target, setTarget] = useState("");
  const [note, setNote] = useState<Note>(null);
  const { tree, error: treeError } = useCatalogTree(open);

  const targets = useMemo(() => {
    if (!tree) return [];
    const byId = new Map(tree.nodes.map((n) => [n.id, n]));
    const pathOf = (n: CatalogNode): string => {
      const names: string[] = [];
      for (let cur: CatalogNode | undefined = n; cur; cur = cur.parent_id ? byId.get(cur.parent_id) : undefined) names.unshift(cur.name);
      return names.join(" / ");
    };
    return tree.nodes
      .filter(
        (n) =>
          n.id !== node.id &&
          n.id !== node.parent_id &&
          n.access === "read" &&
          (n.kind === "organization" || (n.kind === "folder" && node.kind !== "organization")),
      )
      .map((n) => ({ value: n.id, label: pathOf(n) }))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [tree, node]);
  const shown: Note = note ?? (open && treeError ? { kind: "error", text: fail(treeError) } : null);

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setNote(null);
    try {
      await apiSend("POST", `/v1/catalog/nodes/${node.id}/move`, { parent_id: target === "top" ? null : target });
      onOpenChange(false);
      toast.success(t.done);
      onMoved();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      size="sm"
      title={format(t.titleOf, { name: node.name })}
      footer={<Footer form="move-node" submit={t.submit} onCancel={() => onOpenChange(false)} />}
    >
      <form id="move-node" className="grid gap-4" onSubmit={submit}>
        <Field label={t.target}>
          {(p) => (
            <Combobox
              {...p}
              required
              value={target}
              onChange={setTarget}
              placeholder={t.choose}
              options={[...(superadmin && node.kind === "organization" && node.parent_id !== null ? [{ value: "top", label: t.top }] : []), ...targets]}
            />
          )}
        </Field>
        <Message note={shown} />
      </form>
    </Dialog>
  );
}

export function DeleteDialog({
  open,
  onOpenChange,
  node,
  labels,
  fail,
  onDeleted,
}: OpenProps & { node: CatalogNode; labels: CatalogLabels; fail: Fail; onDeleted: () => void }) {
  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title={labels.remove.title}
      text={format(labels.remove.confirm, { name: node.name })}
      confirm={labels.remove.button}
      onConfirm={async () => {
        try {
          await apiSend("DELETE", `/v1/catalog/nodes/${node.id}`);
          toast.success(labels.remove.done);
          onDeleted();
          return null;
        } catch (e) {
          return fail(e);
        }
      }}
    />
  );
}
