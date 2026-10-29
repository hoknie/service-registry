"use client";

import Link from "next/link";
import { useCallback, useEffect, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode, type CatalogNode } from "@/lib/api";
import {
  MAX_INTERVAL_MINUTES,
  MAX_TIMEOUT_SECS,
  MIN_INTERVAL_MINUTES,
  MIN_TIMEOUT_SECS,
  POLLING_MODES,
  draftErrors,
  draftOf,
  minutesOf,
  pollingBody,
  type Polling,
  type PollingDraft,
  type PollingField,
  type PollingMode,
  type PollingOrigin,
} from "@/lib/polling";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Select } from "../ui/Select";
import { SkeletonPanel } from "../ui/Skeleton";
import { catalogHref } from "../catalog/shared";

type Labels = Messages["catalog"]["polling"];

type Props = { node: CatalogNode; locale: Locale; labels: Labels };

const draftKey = { interval_secs: "interval", timeout_secs: "timeout", mode: "mode" } as const;

export function PollingSettings({ node, locale, labels: t }: Props) {
  const { errors, common } = useUiText();
  const url = `/v1/catalog/nodes/${node.id}/polling` as const;
  const [data, setData] = useState<Polling | null>(null);
  const [draft, setDraft] = useState<PollingDraft | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);
  const edit = (node.permissions ?? []).includes("catalog.write");
  const organization = node.kind === "organization" && !node.parent_id;

  const show = useCallback((p: Polling) => {
    setData(p);
    setDraft(draftOf(p, p.defaults.repo_sync_interval_secs));
  }, []);

  useEffect(() => {
    let live = true;
    apiGet<Polling>(url)
      .then((p) => live && show(p))
      .catch((e: unknown) => live && setNote({ kind: "error", text: errorText(errors, errorCode(e)) }));
    return () => {
      live = false;
    };
  }, [url, show, errors]);

  if (!data || !draft) {
    return <Panel title={t.title}>{note ? <Message note={note} /> : <SkeletonPanel lines={4} label={common.loading} />}</Panel>;
  }
  const invalid = draftErrors(draft);

  const save = async (e: FormEvent) => {
    e.preventDefault();
    if (invalid.interval || invalid.timeout) return;
    setBusy(true);
    setNote(null);
    try {
      show(await apiSend<Polling>("PUT", url, pollingBody(draft)));
      toast.success(t.saved);
    } catch (err) {
      setNote({ kind: "error", text: errorText(errors, errorCode(err)) });
    } finally {
      setBusy(false);
    }
  };

  const origin = (set: boolean, from: PollingOrigin): ReactNode =>
    set ? (
      <Badge tone="signal">{t.here}</Badge>
    ) : from ? (
      <Link href={catalogHref(locale, from.id, "settings", null, null, "polling")} className="text-xs text-ink-2 hover:underline">
        {format(t.inherited, { name: from.name })}
      </Link>
    ) : (
      <Badge tone="outline">{t.serverDefault}</Badge>
    );
  const toggle = (field: PollingField) => {
    const key = draftKey[field];
    setDraft({ ...draft, [key]: { ...draft[key], set: !draft[key].set } });
  };
  const head = (field: PollingField, label: string) => (
    <div className="flex flex-wrap items-center gap-2">
      <h3 className="text-sm font-medium text-ink">{label}</h3>
      {origin(draft[draftKey[field]].set, data.from[field])}
      {edit && (
        <span className="ml-auto">
          <Button type="button" size="sm" variant="ghost" onClick={() => toggle(field)}>
            {draft[draftKey[field]].set ? t.reset : t.override}
          </Button>
        </span>
      )}
    </div>
  );
  const modeLabel = (m: PollingMode) => t.modes[m];
  const d = data.defaults;

  return (
    <Panel title={t.title} description={edit ? t.lead : t.readOnly}>
      <form className="grid max-w-2xl gap-6" onSubmit={save}>
        <section className="grid gap-2" aria-label={t.interval}>
          {head("interval_secs", t.interval)}
          {edit && draft.interval.set ? (
            <Field label={t.minutes} hint={format(t.intervalHint, { min: MIN_INTERVAL_MINUTES, max: MAX_INTERVAL_MINUTES })} error={invalid.interval ? t.invalid : undefined}>
              {(p) => (
                <Input
                  {...p}
                  inputMode="numeric"
                  className="w-32"
                  value={draft.interval.minutes}
                  onChange={(e) => setDraft({ ...draft, interval: { set: true, minutes: e.target.value } })}
                />
              )}
            </Field>
          ) : data.interval_secs !== null ? (
            <p className="text-sm text-ink-2">{format(t.everyMinutes, { minutes: minutesOf(data.interval_secs) })}</p>
          ) : (
            <ul className="grid gap-1 rounded-md bg-surface-2 px-3 py-2 text-xs text-ink-2">
              <li>{format(t.defaultRepoSync, { minutes: minutesOf(d.repo_sync_interval_secs) })}</li>
              <li>{format(t.defaultDocs, { minutes: minutesOf(d.docs_interval_secs) })}</li>
              {organization && <li>{format(t.defaultSkills, { minutes: minutesOf(d.skills_interval_secs) })}</li>}
            </ul>
          )}
        </section>
        <section className="grid gap-2" aria-label={t.timeout}>
          {head("timeout_secs", t.timeout)}
          {edit && draft.timeout.set ? (
            <Field label={t.seconds} hint={format(t.timeoutHint, { min: MIN_TIMEOUT_SECS, max: MAX_TIMEOUT_SECS })} error={invalid.timeout ? t.invalid : undefined}>
              {(p) => (
                <Input
                  {...p}
                  inputMode="numeric"
                  className="w-32"
                  value={draft.timeout.seconds}
                  onChange={(e) => setDraft({ ...draft, timeout: { set: true, seconds: e.target.value } })}
                />
              )}
            </Field>
          ) : (
            <p className="text-sm text-ink-2">{format(t.timeoutSeconds, { seconds: data.timeout_secs })}</p>
          )}
        </section>
        <section className="grid gap-2" aria-label={t.mode}>
          {head("mode", t.mode)}
          {edit && draft.mode.set ? (
            <Select aria-label={t.mode} value={draft.mode.value} onChange={(e) => setDraft({ ...draft, mode: { set: true, value: e.target.value as PollingMode } })} className="w-fit">
              {POLLING_MODES.map((m) => (
                <option key={m} value={m}>
                  {modeLabel(m)}
                </option>
              ))}
            </Select>
          ) : (
            <p className="text-sm text-ink-2">{modeLabel(data.mode)}</p>
          )}
        </section>
        <ul className="grid list-disc gap-1 pl-5 text-xs text-muted">
          <li>{organization ? t.appliesOrganization : t.applies}</li>
          <li>{t.offHint}</li>
          <li>{t.publishedHint}</li>
        </ul>
        <Message note={note} />
        {edit && (
          <div>
            <Button type="submit" variant="primary" busy={busy} disabled={invalid.interval || invalid.timeout}>
              {t.save}
            </Button>
          </div>
        )}
      </form>
    </Panel>
  );
}
