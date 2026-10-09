"use client";

import { Ban, FileCode2, Pencil, Plus, Trash } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, apiSend, errorCode, type CatalogNode, type Items, type LinkKind, type LinkTemplate, type ProjectLink, type Tree } from "@/lib/api";

import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Dialog } from "../ui/Dialog";
import { EmptyState } from "../ui/EmptyState";
import { Field, Input, Textarea } from "../ui/Field";
import { Combobox } from "../ui/Combobox";
import { Select } from "../ui/Select";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonTable } from "../ui/Skeleton";
import { Table, Td, Th, Tr } from "../ui/Table";
import { LinkIcon } from "./LinkIcon";
import { kindName } from "./shared";

type Props = {
  node: CatalogNode;
  kinds: LinkKind[];
  locale: Locale;
  labels: CatalogLabels;
  canWrite: boolean;
  onChanged: () => void;
};

type Draft = { mode: "create" | "edit"; kind_key: string; link_key: string; template: string; position: string };

export function TemplatesEditor({ node, kinds, locale, labels, canWrite, onChanged }: Props) {
  const { errors, common } = useUiText();
  const t = labels.links.templates;
  const [items, setItems] = useState<LinkTemplate[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [deleting, setDeleting] = useState<LinkTemplate | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);
  const base = `/v1/catalog/nodes/${node.id}/link-templates` as const;

  const load = useCallback(async () => {
    try {
      setItems((await apiGet<Items<LinkTemplate>>(base)).items);
      setError(null);
    } catch (e) {
      setError(fail(e));
    }
  }, [base, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const changed = () => {
    void load();
    onChanged();
  };

  const names = new Map((node.path ?? []).map((p) => [p.id, p.name]));

  const disable = async (tpl: LinkTemplate) => {
    try {
      await apiSend("PUT", `${base}/${encodeURIComponent(tpl.link_key)}` as `/${string}`, {
        kind_key: tpl.kind_key,
        disabled: true,
        position: tpl.position,
      });
      toast.success(t.disabledDone);
      changed();
    } catch (e) {
      toast.error(fail(e));
    }
  };

  const open = (mode: Draft["mode"], tpl?: LinkTemplate) =>
    setDraft({
      mode,
      kind_key: tpl?.kind_key ?? kinds[0]?.key ?? "",
      link_key: tpl?.link_key ?? kinds[0]?.key ?? "",
      template: tpl?.template ?? "",
      position: String(tpl?.position ?? 0),
    });

  return (
    <Panel
      title={t.title}
      description={t.lead}
      actions={
        canWrite && (
          <Button size="sm" onClick={() => open("create")} disabled={kinds.length === 0}>
            <Plus aria-hidden="true" />
            {t.add}
          </Button>
        )
      }
    >
      {error && <Message note={{ kind: "error", text: error }} className="mb-4" />}
      {!items && !error && <SkeletonTable rows={3} label={common.loading} />}
      {items && items.length === 0 && <EmptyState icon={FileCode2} title={t.empty} />}
      {items && items.length > 0 && (
        <Table>
          <thead>
            <tr>
              <Th>{t.table.kind}</Th>
              <Th>{t.table.key}</Th>
              <Th>{t.table.template}</Th>
              <Th>{t.table.source}</Th>
              {canWrite && (
                <Th>
                  <span className="sr-only">{t.table.actions}</span>
                </Th>
              )}
            </tr>
          </thead>
          <tbody>
            {items.map((tpl) => (
              <Tr key={tpl.link_key}>
                <Td>
                  <span className="flex items-center gap-2 text-ink">
                    <LinkIcon icon={kinds.find((k) => k.key === tpl.kind_key)?.icon} />
                    {kindName(kinds, tpl.kind_key, locale)}
                  </span>
                </Td>
                <Td>
                  <code className="text-ink-2">{tpl.link_key}</code>
                </Td>
                <Td className="max-w-[28rem]">
                  {tpl.disabled ? <Badge tone="outline">{t.disabled}</Badge> : <code className="text-xs break-all text-ink-2">{tpl.template}</code>}
                </Td>
                <Td className="text-ink-2">
                  {tpl.inherited ? (
                    <Link href={catalogHref(locale, tpl.node_id, "links")} className="hover:underline">
                      {names.get(tpl.node_id) ?? tpl.node_id}
                    </Link>
                  ) : (
                    t.here
                  )}
                </Td>
                {canWrite && (
                  <Td className="text-right whitespace-nowrap">
                    {tpl.inherited ? (
                      <>
                        {!tpl.disabled && (
                          <Button size="sm" variant="ghost" onClick={() => open("create", tpl)}>
                            <Pencil aria-hidden="true" />
                            {t.override}
                          </Button>
                        )}
                        {!tpl.disabled && (
                          <Button size="sm" variant="ghost" onClick={() => void disable(tpl)}>
                            <Ban aria-hidden="true" />
                            {t.disable}
                          </Button>
                        )}
                      </>
                    ) : (
                      <>
                        {!tpl.disabled && (
                          <Button size="sm" variant="ghost" onClick={() => open("edit", tpl)}>
                            <Pencil aria-hidden="true" />
                            {t.edit}
                          </Button>
                        )}
                        <Button size="sm" variant="danger-ghost" onClick={() => setDeleting(tpl)}>
                          <Trash aria-hidden="true" />
                          {t.delete}
                        </Button>
                      </>
                    )}
                  </Td>
                )}
              </Tr>
            ))}
          </tbody>
        </Table>
      )}
      {draft && (
        <TemplateDialog
          node={node}
          kinds={kinds}
          locale={locale}
          labels={labels}
          initial={draft}
          onClose={() => setDraft(null)}
          onSaved={() => {
            setDraft(null);
            toast.success(t.saved);
            changed();
          }}
        />
      )}
      {deleting && (
        <ConfirmDialog
          open
          onOpenChange={(o) => !o && setDeleting(null)}
          title={t.deleteTitle}
          text={format(t.confirmDelete, { key: deleting.link_key })}
          confirm={t.delete}
          onConfirm={async () => {
            try {
              await apiSend("DELETE", `${base}/${encodeURIComponent(deleting.link_key)}` as `/${string}`);
              toast.success(t.deleted);
              changed();
              return null;
            } catch (e) {
              return fail(e);
            }
          }}
        />
      )}
    </Panel>
  );
}

type DialogProps = {
  node: CatalogNode;
  kinds: LinkKind[];
  locale: Locale;
  labels: CatalogLabels;
  initial: Draft;
  onClose: () => void;
  onSaved: () => void;
};

function TemplateDialog({ node, kinds, locale, labels, initial, onClose, onSaved }: DialogProps) {
  const { errors, common } = useUiText();
  const t = labels.links.templates.dialog;
  const [draft, setDraft] = useState(initial);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const project = node.kind === "project";
  const [projects, setProjects] = useState<CatalogNode[]>([]);
  const [projectId, setProjectId] = useState(project ? node.id : "");
  const [preview, setPreview] = useState<ProjectLink[] | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const fail = useCallback((e: unknown) => errorText(errors, errorCode(e)), [errors]);

  useEffect(() => {
    if (project) return;
    apiGet<Tree>(`/v1/catalog/tree?root=${node.id}&depth=10`)
      .then((tree) => {
        const found = tree.nodes.filter((n) => n.kind === "project" && n.access === "read");
        setProjects(found);
        setProjectId((cur) => cur || found[0]?.id || "");
      })
      .catch(() => setProjects([]));
  }, [node.id, project]);

  useEffect(() => {
    if (!draft.template.trim() || !projectId) return;
    const timer = window.setTimeout(() => {
      apiSend<Items<ProjectLink>>("POST", `/v1/catalog/nodes/${node.id}/link-templates/preview`, {
        template: draft.template,
        project_id: projectId,
      })
        .then((r) => {
          setPreview(r.items);
          setPreviewError(null);
        })
        .catch((e: unknown) => {
          setPreview(null);
          setPreviewError(fail(e));
        });
    }, 300);
    return () => window.clearTimeout(timer);
  }, [draft.template, projectId, node.id, fail]);

  const set = (patch: Partial<Draft>) => setDraft((d) => ({ ...d, ...patch }));
  const previewing = !!draft.template.trim() && !!projectId;

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      const position = Number(draft.position);
      await apiSend("PUT", `/v1/catalog/nodes/${node.id}/link-templates/${encodeURIComponent(draft.link_key.trim())}` as `/${string}`, {
        kind_key: draft.kind_key,
        template: draft.template,
        position: Number.isInteger(position) ? position : -1,
      });
      onSaved();
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(o) => !o && onClose()}
      size="lg"
      title={draft.mode === "edit" ? format(t.editTitle, { key: draft.link_key }) : t.createTitle}
      footer={
        <>
          <Button onClick={onClose}>{common.cancel}</Button>
          <Button variant="primary" disabled={busy || !draft.link_key.trim() || !draft.template.trim()} onClick={() => void save()}>
            {t.submit}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_7rem]">
          <Field label={t.kind}>
            {(p) => (
              <Combobox
                {...p}
                value={draft.kind_key}
                onChange={(v) => set(draft.mode === "create" && draft.link_key === draft.kind_key ? { kind_key: v, link_key: v } : { kind_key: v })}
                options={kinds.map((k) => ({ value: k.key, label: k.names[locale], detail: k.key, keywords: [k.key] }))}
              />
            )}
          </Field>
          <Field label={t.key} hint={t.keyHint}>
            {(p) => (
              <Input
                {...p}
                spellCheck={false}
                className="font-mono"
                readOnly={draft.mode === "edit"}
                value={draft.link_key}
                onChange={(e) => set({ link_key: e.target.value })}
              />
            )}
          </Field>
          <Field label={t.position}>
            {(p) => <Input {...p} type="number" min={0} max={10000} value={draft.position} onChange={(e) => set({ position: e.target.value })} />}
          </Field>
        </div>
        <Field label={t.template} hint={t.templateHint}>
          {(p) => (
            <Textarea
              {...p}
              rows={3}
              spellCheck={false}
              className="font-mono text-sm"
              placeholder="https://grafana.example/d/svc?var-ns={namespace}"
              value={draft.template}
              onChange={(e) => set({ template: e.target.value })}
            />
          )}
        </Field>
        {note && <Message note={note} />}
        <section aria-labelledby="template-preview" className="grid gap-2 rounded-md border border-line bg-surface-2 p-3">
          <div className="flex flex-wrap items-end justify-between gap-2">
            <h3 id="template-preview" className="text-sm font-medium text-ink-2">
              {t.preview}
            </h3>
            {!project && (
              <Field label={t.previewProject} className="w-64 max-w-full">
                {(p) => (
                  <Select {...p} value={projectId} onChange={(e) => setProjectId(e.target.value)}>
                    {projects.length === 0 && <option value="">{t.previewChoose}</option>}
                    {projects.map((n) => (
                      <option key={n.id} value={n.id}>
                        {n.name}
                      </option>
                    ))}
                  </Select>
                )}
              </Field>
            )}
          </div>
          {previewing && previewError && <Message note={{ kind: "error", text: previewError }} />}
          {(!previewing || (!previewError && !preview)) && <p className="text-sm text-muted">{t.previewEmpty}</p>}
          {previewing && !previewError && preview && (
            <ul className="grid gap-1.5">
              {preview.map((l, i) => (
                <li key={i} className="flex flex-wrap items-center gap-2 text-sm">
                  {l.environment && <Badge tone="cobalt">{l.environment}</Badge>}
                  {l.url ? (
                    <code className="break-all text-ink">{l.url}</code>
                  ) : (
                    <span className="text-muted">{format(labels.links.missing, { vars: l.missing.join(", ") })}</span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </Dialog>
  );
}
