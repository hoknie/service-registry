"use client";

import { useState } from "react";
import { toast } from "sonner";

import { localeNames, locales, type Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode, type NodeKind } from "@/lib/api";
import { cn } from "@/lib/cn";
import {
  CHANNELS,
  environmentsOf,
  PRIMARY_EVENTS,
  RECOVERY_OF,
  toggleEvent,
  validEnvironment,
  validKey,
  type Channel,
  type EventType,
  type Subscription,
  type SubscriptionBody,
} from "@/lib/notifications";
import { putSubscription, suggestEnvironments } from "@/lib/notificationsApi";
import { fromTags, toTags } from "@/lib/tags";

import { catalogHref } from "../catalog/shared";
import { SecretPicker } from "../secrets/SecretPicker";
import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Checkbox, Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Select } from "../ui/Select";
import { TagInput } from "../ui/TagInput";
import { SigningSecret } from "./SigningSecret";

export type DialogMode = { kind: "create" } | { kind: "edit"; sub: Subscription } | { kind: "override"; sub: Subscription };

type Props = {
  nodeId: string;
  nodeKind: NodeKind;
  locale: Locale;
  labels: Messages["notify"];
  mode: DialogMode;
  onClose: () => void;
  onSaved: () => void;
};

export function SubscriptionDialog({ nodeId, nodeKind, locale, labels: t, mode, onClose, onSaved }: Props) {
  const { errors, common } = useUiText();
  const source = mode.kind === "create" ? null : mode.sub;
  const [key, setKey] = useState(source?.key ?? "");
  const [channel, setChannel] = useState<Channel>(source?.channel ?? "webhook");
  const [secret, setSecret] = useState<string | null>(source?.secret_id ?? null);
  const [events, setEvents] = useState<EventType[]>(source?.event_types ?? ["deployment.created"]);
  const [environments, setEnvironments] = useState(fromTags(source?.environments ?? []));
  const [lang, setLang] = useState(source?.locale ?? locale);
  const [disabled, setDisabled] = useState(source?.disabled ?? false);
  const [saved, setSaved] = useState<Subscription | null>(mode.kind === "edit" ? mode.sub : null);
  const [busy, setBusy] = useState(false);
  const [note, setNote] = useState<Note>(null);

  const keyError = key && !validKey(key) ? t.dialog.invalidKey : null;
  const title =
    mode.kind === "create" ? t.dialog.createTitle : format(mode.kind === "edit" ? t.dialog.editTitle : t.dialog.overrideTitle, { key: mode.sub.key });
  const toggle = (e: EventType) => setEvents(toggleEvent(events, e));

  const save = async () => {
    if (!validKey(key)) return setNote({ kind: "error", text: t.dialog.invalidKey });
    if (events.length === 0) return setNote({ kind: "error", text: t.dialog.eventsRequired });
    const body: SubscriptionBody = { channel, secret_id: secret, event_types: events, environments: environmentsOf(toTags(environments)), locale: lang, disabled };
    setBusy(true);
    try {
      const result = await putSubscription(nodeId, key.trim().toLowerCase(), body);
      toast.success(t.saved);
      setNote(null);
      onSaved();
      if (result.channel === "webhook" && !result.disabled) setSaved(result);
      else onClose();
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open
      onOpenChange={(o) => !o && onClose()}
      title={title}
      size="lg"
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            {saved ? common.close : common.cancel}
          </Button>
          <Button variant="primary" busy={busy} disabled={busy} onClick={() => void save()}>
            {t.dialog.save}
          </Button>
        </>
      }
    >
      <div className="grid gap-4">
        <Message note={note} />
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t.dialog.key} hint={t.dialog.keyHint} error={keyError ?? undefined}>
            {(p) => (
              <Input {...p} className="font-mono" spellCheck={false} value={key} disabled={mode.kind !== "create"} onChange={(e) => setKey(e.target.value)} />
            )}
          </Field>
          <Field label={t.dialog.channel}>
            {(p) => (
              <Select {...p} value={channel} onChange={(e) => setChannel(e.target.value as Channel)}>
                {CHANNELS.map((c) => (
                  <option key={c} value={c}>
                    {t.channels[c]}
                  </option>
                ))}
              </Select>
            )}
          </Field>
        </div>
        <div className="grid gap-1.5">
          <SecretPicker
            listOn={nodeId}
            createOn={nodeKind === "project" ? false : nodeId}
            value={secret}
            onChange={setSecret}
            manageHref={catalogHref(locale, nodeId, "settings", null, null, "secrets")}
            allowNone={disabled}
            required={!disabled}
          />
          <p className="text-xs text-muted">{t.dialog.secretHint}</p>
        </div>
        <fieldset className="grid gap-2">
          <legend className="mb-1.5 text-sm font-medium text-ink-2">{t.dialog.events}</legend>
          <div className="grid gap-2 sm:grid-cols-2">
            {PRIMARY_EVENTS.map((e) => {
              const recovery = RECOVERY_OF[e];
              return (
                <div key={e} className="grid gap-1.5">
                  <Checkbox label={t.events[e]} checked={events.includes(e)} onChange={() => toggle(e)} />
                  {recovery && (
                    <Checkbox
                      className={cn("ms-6", !events.includes(e) && "cursor-not-allowed text-muted")}
                      label={t.events[recovery]}
                      checked={events.includes(recovery)}
                      disabled={!events.includes(e)}
                      onChange={() => toggle(recovery)}
                    />
                  )}
                </div>
              );
            })}
          </div>
        </fieldset>
        <Field label={t.dialog.environments} hint={t.dialog.environmentsHint}>
          {(p) => (
            <TagInput
              {...p}
              value={environments}
              onChange={setEnvironments}
              suggest={suggestEnvironments}
              validate={(tag) => (validEnvironment(tag) ? null : t.dialog.invalidEnvironment)}
            />
          )}
        </Field>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t.dialog.locale}>
            {(p) => (
              <Select {...p} value={lang} onChange={(e) => setLang(e.target.value as Locale)}>
                {locales.map((l) => (
                  <option key={l} value={l}>
                    {localeNames[l]}
                  </option>
                ))}
              </Select>
            )}
          </Field>
          <div className="grid content-end gap-1">
            <Checkbox label={t.dialog.disabled} checked={disabled} onChange={(e) => setDisabled(e.target.checked)} />
            <p className="text-xs text-muted">{t.dialog.disabledHint}</p>
          </div>
        </div>
        {saved && saved.channel === "webhook" && <SigningSecret nodeId={nodeId} sub={saved} labels={t} onChanged={onSaved} />}
      </div>
    </Dialog>
  );
}
