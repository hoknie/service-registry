"use client";

import { ExternalLink, Plus, Trash2 } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type CatalogNode } from "@/lib/api";
import { draftOf, publicationBody, unreachable, type Publication, type PublicationDraft, type PublicationOrigin } from "@/lib/publication";
import type { RefKind } from "@/lib/refs";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Checkbox, Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Segmented } from "../ui/Segmented";
import { Select } from "../ui/Select";
import { SkeletonPanel } from "../ui/Skeleton";
import { catalogHref } from "../catalog/shared";

type Labels = Messages["catalog"]["publication"];

type Props = { node: CatalogNode; locale: Locale; labels: Labels };

type Inherited = "publish" | "versions" | "default_branch" | "current_release";

export function PublicationSettings({ node, locale, labels: t }: Props) {
  const { errors, common } = useUiText();
  const url = `/v1/catalog/nodes/${node.id}/publication` as const;
  const [data, setData] = useState<Publication | null>(null);
  const [draft, setDraft] = useState<PublicationDraft | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const withName = node.kind !== "organization";

  const show = useCallback((p: Publication) => {
    setData(p);
    setDraft(draftOf(p));
  }, []);

  useEffect(() => {
    let live = true;
    apiGet<Publication>(url)
      .then((p) => live && show(p))
      .catch((e: unknown) => live && setNote({ kind: "error", text: errorText(errors, errorCode(e)) }));
    return () => {
      live = false;
    };
  }, [url, show, errors]);

  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (!draft) return;
    setBusy(true);
    setNote(null);
    try {
      show(await apiSend<Publication>("PUT", url, publicationBody(draft, withName)));
      toast.success(t.saved);
    } catch (err) {
      setNote({ kind: "error", text: errorText(errors, errorCode(err)) });
    } finally {
      setBusy(false);
    }
  };

  if (!data || !draft) {
    return <Panel title={t.title}>{note ? <Message note={note} /> : <SkeletonPanel lines={4} label={common.loading} />}</Panel>;
  }
  const edit = data.can_edit;
  const origin = (set: boolean, from: PublicationOrigin): ReactNode =>
    set ? (
      <Badge tone="signal">{t.here}</Badge>
    ) : from ? (
      <Link href={catalogHref(locale, from.id, "settings", null, null, "publication")} className="text-xs text-ink-2 hover:underline">
        {format(t.inherited, { name: from.name })}
      </Link>
    ) : (
      <Badge tone="outline">{t.default}</Badge>
    );
  const toggle = (field: Inherited) => {
    const cur = draft[field];
    if (field === "versions") {
      setDraft({ ...draft, versions: cur.set ? { set: false, rows: [] } : { set: true, rows: data.versions.map((v) => ({ ...v })) } });
      return;
    }
    const value = data[field];
    setDraft({ ...draft, [field]: { set: !cur.set, value } });
  };
  const head = (field: Inherited, label: string) => (
    <div className="flex flex-wrap items-center gap-2">
      <h3 className="text-sm font-medium text-ink">{label}</h3>
      {origin(draft[field].set, data.from[field])}
      {edit && (
        <span className="ml-auto">
          <Button type="button" size="sm" variant="ghost" onClick={() => toggle(field)}>
            {draft[field].set ? t.reset : t.override}
          </Button>
        </span>
      )}
    </div>
  );
  const flag = (field: "default_branch" | "current_release", label: string) => (
    <section className="grid gap-2" aria-label={label}>
      {head(field, label)}
      {edit && draft[field].set ? (
        <Checkbox label={label} checked={draft[field].value} onChange={(e) => setDraft({ ...draft, [field]: { set: true, value: e.target.checked } })} />
      ) : (
        <p className="text-sm text-ink-2">{data[field] ? t.yes : t.no}</p>
      )}
    </section>
  );
  const rows = draft.versions.rows;
  const setRows = (next: typeof rows) => setDraft({ ...draft, versions: { set: true, rows: next } });
  const kindLabel = (k: RefKind) => (k === "tag" ? t.tags : t.branches);

  return (
    <Panel title={t.title} description={edit ? t.lead : t.readOnly}>
      <form className="grid max-w-2xl gap-6" onSubmit={save}>
        <section className="grid gap-2" aria-label={t.publish}>
          {head("publish", t.publish)}
          {edit && draft.publish.set ? (
            <Segmented
              label={t.publish}
              value={draft.publish.value ? "yes" : "no"}
              options={[
                { value: "yes", label: t.yes },
                { value: "no", label: t.no },
              ]}
              onChange={(v) => setDraft({ ...draft, publish: { set: true, value: v === "yes" } })}
              className="w-fit"
            />
          ) : (
            <p className="text-sm text-ink-2">{data.publish ? t.yes : t.no}</p>
          )}
        </section>
        <section className="grid gap-2" aria-label={t.versions}>
          {head("versions", t.versions)}
          {edit && draft.versions.set ? (
            <div className="grid gap-2">
              {rows.map((row, i) => (
                <div key={i} className="flex flex-wrap items-center gap-2">
                  <Input
                    aria-label={t.pattern}
                    className="min-w-40 flex-1 font-mono"
                    value={row.pattern}
                    onChange={(e) => setRows(rows.map((r, j) => (j === i ? { ...r, pattern: e.target.value } : r)))}
                  />
                  <Select
                    aria-label={t.kind}
                    value={row.kind}
                    onChange={(e) => setRows(rows.map((r, j) => (j === i ? { ...r, kind: e.target.value as RefKind } : r)))}
                    className="w-32"
                  >
                    <option value="branch">{t.branches}</option>
                    <option value="tag">{t.tags}</option>
                  </Select>
                  <Button type="button" size="sm" variant="ghost" aria-label={t.remove} onClick={() => setRows(rows.filter((_, j) => j !== i))}>
                    <Trash2 aria-hidden="true" />
                  </Button>
                </div>
              ))}
              <div>
                <Button type="button" size="sm" onClick={() => setRows([...rows, { pattern: "", kind: "tag" }])}>
                  <Plus aria-hidden="true" />
                  {t.add}
                </Button>
              </div>
            </div>
          ) : data.versions.length ? (
            <ul className="grid gap-1 rounded-md bg-surface-2 px-3 py-2 text-xs text-ink-2">
              {data.versions.map((v) => (
                <li key={`${v.kind}:${v.pattern}`}>
                  <span className="font-mono">{v.pattern}</span> — {kindLabel(v.kind)}
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-sm text-ink-2">{t.noVersions}</p>
          )}
          <p className="text-xs text-muted">{t.versionsHint}</p>
        </section>
        {flag("default_branch", t.defaultBranch)}
        {flag("current_release", t.currentRelease)}
        {withName && (
          <Field label={t.name} hint={t.nameHint}>
            {(p) => <Input {...p} disabled={!edit} value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} className="font-mono" />}
          </Field>
        )}
        <Field label={t.host} hint={t.hostHint}>
          {(p) => <Input {...p} disabled={!edit} value={draft.host} onChange={(e) => setDraft({ ...draft, host: e.target.value })} className="font-mono" />}
        </Field>
        <section className="grid gap-1.5" aria-label={t.addresses}>
          <h3 className="text-sm font-medium text-ink">{t.addresses}</h3>
          {data.entries.length === 0 ? (
            <p className="text-sm text-muted">{unreachable(data) ? t.unreachable : t.notPublished}</p>
          ) : (
            <ul className="grid gap-1 text-sm">
              {data.entries.map((e) => (
                <li key={e.url}>
                  <a href={e.url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 font-mono text-signal hover:underline">
                    {e.url}
                    <ExternalLink aria-hidden="true" className="size-3" />
                  </a>
                </li>
              ))}
            </ul>
          )}
        </section>
        <Message note={note} />
        {edit && (
          <div>
            <Button type="submit" variant="primary" busy={busy}>
              {t.save}
            </Button>
          </div>
        )}
      </form>
    </Panel>
  );
}
