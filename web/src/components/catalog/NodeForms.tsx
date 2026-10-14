"use client";

import { useEffect, useMemo, useState, type FormEvent } from "react";
import { toast } from "sonner";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, apiSend, errorCode, type CatalogNode, type KnowledgeSource, type SourceCheck } from "@/lib/api";
import { labelsOf, labelsToLines } from "@/lib/tags";
import { useCatalogTree } from "@/lib/catalogTree";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { Field, Input } from "../ui/Field";
import { Combobox } from "../ui/Combobox";
import { hasInvalidTags, LabelsInput } from "../ui/TagInput";
import { Message, type Note } from "../ui/Message";
import { draftOf, emptyDraft, sourceBody, secretChanged, type SourceDraft } from "../knowledge/SourceFields";
import { projectBranches } from "./BranchInput";
import { DescriptionField, labelError, ObservationField, RepoSection, type Repo, type RepoMode } from "./NodeFields";
import { type CatalogLabels } from "./shared";

type Fail = (e: unknown) => string;
type OpenProps = { open: boolean; onOpenChange: (open: boolean) => void };

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

  const branchSuggest = useMemo(
    () => (isProject ? projectBranches(node.id, node.default_branch ?? null, labels.branches.badges.default) : undefined),
    [isProject, node.id, node.default_branch, labels.branches.badges.default],
  );

  const sourceChanged = () => {
    if (managed || source?.origin === "forge") return false;
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
        <DescriptionField labels={labels} value={description} onChange={setDescription} readOnly={managed} hint={managed ? t.fromForge : undefined} />
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
            branchSuggest={branchSuggest}
          >
            {mode !== "link" && !managed && (
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
