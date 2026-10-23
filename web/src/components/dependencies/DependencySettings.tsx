"use client";

import { Trash } from "lucide-react";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { apiGet, type CatalogNode, type CatalogTable } from "@/lib/api";
import { automaticOf, externalTarget, manualOf, type Dependencies, type DependencyItem } from "@/lib/dependencies";
import { addDependency, getDependencies, removeDependency } from "@/lib/dependenciesApi";
import type { Suggestion } from "@/lib/labelSuggest";
import { nodeHits, nodeSearchQuery } from "@/lib/nodeSearch";

import { Button } from "../ui/Button";
import { ConfirmDialog } from "../ui/ConfirmDialog";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Segmented } from "../ui/Segmented";
import { SkeletonRows } from "../ui/Skeleton";
import { SuggestionList, useSuggestions } from "../ui/Suggestions";
import type { CatalogLabels } from "../catalog/shared";
import { useUiText } from "../UiText";
import { SourceBadges } from "./SourceBadges";

async function findProjects(draft: string, signal: AbortSignal): Promise<Suggestion[]> {
  const path = nodeSearchQuery(draft);
  if (!path) return [];
  const table = await apiGet<CatalogTable>(path as `/${string}`, signal);
  return nodeHits(table)
    .filter((h) => h.node.access === "read" && h.node.kind === "project")
    .map((h) => ({ value: h.node.id, label: h.path ? `${h.path} / ${h.node.name}` : h.node.name }));
}

function nameOf(item: DependencyItem, labels: CatalogLabels): string {
  const t = item.target;
  if (t.type === "project") return t.path;
  if (t.type === "external") return t.name;
  if (t.type === "unresolved") return t.ref;
  return labels.dependencies.hidden;
}

type Props = { node: CatalogNode; locale: Locale; labels: CatalogLabels; fail: (e: unknown) => string };

export function DependencySettings({ node, locale, labels, fail }: Props) {
  const t = labels.dependencies;
  const { common } = useUiText();
  const [data, setData] = useState<Dependencies | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [kind, setKind] = useState<"project" | "external">("project");
  const [draft, setDraft] = useState("");
  const [picked, setPicked] = useState<Suggestion | null>(null);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [focused, setFocused] = useState(false);
  const [removing, setRemoving] = useState<{ id: string; name: string } | null>(null);
  const s = useSuggestions(kind === "project" ? draft : "", findProjects, focused);

  const load = useCallback(async () => {
    try {
      setData(await getDependencies(node.id));
    } catch (e) {
      setNote({ kind: "error", text: fail(e) });
    }
  }, [node.id, fail]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const target = kind === "project" ? picked?.value : externalTarget(draft);
    if (!target) return;
    setBusy(true);
    try {
      await addDependency(node.id, target, text);
      setDraft("");
      setPicked(null);
      setText("");
      setNote(null);
      toast.success(t.added);
      await load();
    } catch (err) {
      setNote({ kind: "error", text: fail(err) });
    } finally {
      setBusy(false);
    }
  };

  if (!data) return note ? <Message note={note} /> : <SkeletonRows rows={3} label={common.loading} />;
  const manual = manualOf(data.depends_on);
  const automatic = automaticOf(data.depends_on);

  return (
    <div className="grid gap-4">
      <Panel title={t.manualTitle} description={t.manualLead}>
        <div className="grid gap-4">
          {manual.length === 0 ? (
            <p className="text-sm text-muted">{t.noManual}</p>
          ) : (
            <ul className="grid gap-2">
              {manual.map(({ item, source }) => (
                <li key={source.id} className="flex flex-wrap items-center justify-between gap-2">
                  <span className="grid">
                    <span className="break-all text-ink">{nameOf(item, labels)}</span>
                    {source.note && <span className="text-sm whitespace-pre-line text-muted">{source.note}</span>}
                  </span>
                  <Button variant="danger-ghost" size="sm" onClick={() => setRemoving({ id: source.id!, name: nameOf(item, labels) })}>
                    <Trash aria-hidden="true" />
                    {t.remove}
                  </Button>
                </li>
              ))}
            </ul>
          )}
          <form onSubmit={submit} className="grid gap-3">
            <Segmented
              label={t.target}
              value={kind}
              options={[
                { value: "project", label: t.project },
                { value: "external", label: t.externalKind },
              ]}
              onChange={(v) => {
                setKind(v as "project" | "external");
                setDraft("");
                setPicked(null);
              }}
            />
            <Field label={t.target}>
              {(f) => (
              <div className="relative">
                <Input
                  {...f}
                  {...(kind === "project" ? s.inputProps : {})}
                  placeholder={kind === "project" ? t.projectPlaceholder : t.externalPlaceholder}
                  value={picked && kind === "project" ? picked.label : draft}
                  onChange={(e) => {
                    setPicked(null);
                    setDraft(e.target.value);
                  }}
                  onFocus={() => setFocused(true)}
                  onBlur={() => setFocused(false)}
                  onKeyDown={(e) =>
                    kind === "project" &&
                    s.onKey(e, (c) => {
                      setPicked(c);
                      s.close();
                    })
                  }
                />
                {kind === "project" && s.open && (
                  <SuggestionList
                    id={s.listId}
                    items={s.items}
                    active={s.active}
                    onActive={s.setActive}
                    onChoose={(c) => {
                      setPicked(c);
                      s.close();
                    }}
                  />
                )}
              </div>
              )}
            </Field>
            <Field label={t.note}>
              {(f) => <Input {...f} placeholder={t.notePlaceholder} value={text} maxLength={500} onChange={(e) => setText(e.target.value)} />}
            </Field>
            {note && <Message note={note} />}
            <div>
              <Button type="submit" variant="primary" busy={busy} disabled={kind === "project" ? !picked : !draft.trim()}>
                {t.add}
              </Button>
            </div>
          </form>
        </div>
      </Panel>
      <Panel title={t.autoTitle} description={t.autoLead}>
        {automatic.length === 0 ? (
          <p className="text-sm text-muted">{t.noAuto}</p>
        ) : (
          <ul className="grid gap-2">
            {automatic.map(({ item, source }, i) => (
              <li key={`${source.source}-${source.origin}-${i}`} className="flex flex-wrap items-center justify-between gap-2">
                <span className="grid">
                  <span className="break-all text-ink">{nameOf(item, labels)}</span>
                  <span className="text-xs break-all text-muted">
                    {t.origin}: {source.origin}
                  </span>
                </span>
                <SourceBadges sources={[source]} locale={locale} labels={labels} />
              </li>
            ))}
          </ul>
        )}
      </Panel>
      <ConfirmDialog
        open={removing !== null}
        onOpenChange={(open) => !open && setRemoving(null)}
        title={t.removeTitle}
        text={removing ? format(t.removeText, { name: removing.name }) : ""}
        confirm={t.remove}
        onConfirm={async () => {
          if (!removing) return null;
          try {
            await removeDependency(node.id, removing.id);
            toast.success(t.removed);
            await load();
            return null;
          } catch (e) {
            return fail(e);
          }
        }}
      />
    </div>
  );
}
