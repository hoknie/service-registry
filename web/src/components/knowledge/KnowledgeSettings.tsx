"use client";

import Link from "next/link";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import { apiGet, apiSend, errorCode, type NodeKnowledgeSettings, type SettingsField } from "@/lib/api";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { SkeletonPanel } from "../ui/Skeleton";
import { catalogHref, type CatalogLabels } from "../catalog/shared";
import { hasInvalidTags } from "../ui/TagInput";
import { INCLUDE_EXAMPLES, PatternField, patternError, patternLines } from "./PatternField";

type Props = {
  nodeId: string;
  project: boolean;
  canWrite: boolean;
  locale: Locale;
  labels: CatalogLabels["docs"]["settings"];
  onSaved?: () => void;
};

const FIELDS: SettingsField[] = ["include", "exclude", "branches"];

type Draft = Record<SettingsField, { set: boolean; text: string }>;

function draftOf(s: NodeKnowledgeSettings): Draft {
  const own = (f: SettingsField) => ({ set: f in s.own, text: (s.own[f] ?? []).join("\n") });
  return { include: own("include"), exclude: own("exclude"), branches: own("branches") };
}

export function KnowledgeSettings({ nodeId, project, canWrite, locale, labels: t, onSaved }: Props) {
  const { errors, common } = useUiText();
  const url = `/v1/catalog/nodes/${nodeId}/knowledge/settings` as const;
  const [settings, setSettings] = useState<NodeKnowledgeSettings | null>(null);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const show = useCallback((s: NodeKnowledgeSettings) => {
    setSettings(s);
    setDraft(draftOf(s));
  }, []);

  useEffect(() => {
    let live = true;
    apiGet<NodeKnowledgeSettings>(url)
      .then((s) => live && show(s))
      .catch((e: unknown) => live && setNote({ kind: "error", text: errorText(errors, errorCode(e)) }));
    return () => {
      live = false;
    };
  }, [url, show, errors]);

  const save = async (event: FormEvent) => {
    event.preventDefault();
    if (!draft) return;
    setBusy(true);
    setNote(null);
    const body: Record<string, string[] | null> = {};
    for (const f of FIELDS) {
      if (!draft[f].set) continue;
      const lines = patternLines(draft[f].text);
      body[f] = f === "include" && lines.length === 0 ? null : lines;
    }
    try {
      show(await apiSend<NodeKnowledgeSettings>("PUT", url, body));
      onSaved?.();
      toast.success(t.saved);
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const labelOf = { include: t.include, exclude: t.exclude, branches: t.branches };
  const hintOf = { include: `${t.includeHint} ${t.hint}`, exclude: t.hint, branches: t.branchesHint };
  const valueText = (f: SettingsField, s: NodeKnowledgeSettings) => {
    const v = s[f];
    if (v === null) return t.readmeRule;
    return v.length ? v.join("\n") : t.none;
  };

  return (
    <Panel id="knowledge-settings" title={t.title} description={canWrite ? (project ? t.lead : t.leadNode) : t.readOnly}>
      {!settings || !draft ? (
        note ? <Message note={note} /> : <SkeletonPanel lines={3} label={common.loading} />
      ) : (
        <form className="grid max-w-2xl gap-6" onSubmit={save}>
          {FIELDS.map((f) => {
            const from = settings.from[f];
            const origin = draft[f].set ? (
              <Badge tone="signal">{t.here}</Badge>
            ) : from ? (
              <Link href={catalogHref(locale, from.id, "docs-settings")} className="text-xs text-ink-2 hover:underline">
                {format(t.inherited, { name: from.name })}
              </Link>
            ) : (
              <Badge tone="outline">{t.default}</Badge>
            );
            const toggle = canWrite && (
              <Button
                type="button"
                size="sm"
                variant="ghost"
                onClick={() =>
                  setDraft({
                    ...draft,
                    [f]: draft[f].set ? { set: false, text: "" } : { set: true, text: (settings[f] ?? []).join("\n") },
                  })
                }
              >
                {draft[f].set ? t.reset : t.override}
              </Button>
            );
            return (
              <section key={f} className="grid gap-2" aria-label={labelOf[f]}>
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-sm font-medium text-ink">{labelOf[f]}</h3>
                  {origin}
                  <span className="ml-auto">{toggle}</span>
                </div>
                {canWrite && draft[f].set ? (
                  <PatternField
                    label={labelOf[f]}
                    hint={hintOf[f]}
                    value={draft[f].text}
                    onChange={(text) => setDraft({ ...draft, [f]: { set: true, text } })}
                    examples={f === "include" ? INCLUDE_EXAMPLES : undefined}
                    examplesLabel={t.examples}
                  />
                ) : (
                  <pre className="overflow-x-auto rounded-md bg-surface-2 px-3 py-2 font-mono text-xs whitespace-pre-wrap text-ink-2">{valueText(f, settings)}</pre>
                )}
              </section>
            );
          })}
          <Message note={note} />
          {canWrite && (
            <div>
              <Button
                type="submit"
                variant="primary"
                busy={busy}
                disabled={FIELDS.some((f) => draft[f].set && hasInvalidTags(draft[f].text, (tag) => patternError(tag, "")))}
              >
                {t.save}
              </Button>
            </div>
          )}
        </form>
      )}
    </Panel>
  );
}
