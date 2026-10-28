"use client";

import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { apiGet, apiSend, errorCode } from "@/lib/api";
import type { SkillPolicy, SkillPolicyView, SkillVersion } from "@/lib/skills";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Field, Input } from "../ui/Field";
import { Message, type Note } from "../ui/Message";
import { Panel } from "../ui/Panel";
import { Select } from "../ui/Select";
import { TagInput } from "../ui/TagInput";

type Props = {
  path: `/${string}`;
  labels: Messages["skills"];
  preview?: (draft: SkillPolicy) => Promise<SkillVersion[]>;
  versions?: string[];
  lead?: string;
};

export function VersionPolicyForm({
  path,
  labels: t,
  preview,
  versions = [],
  lead,
}: Props) {
  const p = t.settings.policy;
  const { errors } = useUiText();
  const [view, setView] = useState<SkillPolicyView | null>(null);
  const [range, setRange] = useState("");
  const [prerelease, setPrerelease] = useState("");
  const [exclude, setExclude] = useState("");
  const [excludeSet, setExcludeSet] = useState(false);
  const [shown, setShown] = useState<SkillVersion[] | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState(false);

  const apply = (v: SkillPolicyView) => {
    setView(v);
    setRange(v.own.range ?? "");
    setPrerelease(v.own.prerelease === null ? "" : String(v.own.prerelease));
    setExclude((v.own.exclude ?? []).join("\n"));
    setExcludeSet(v.own.exclude !== null);
  };

  useEffect(() => {
    apiGet<SkillPolicyView>(path)
      .then(apply)
      .catch((e: unknown) =>
        setNote({ kind: "error", text: errorText(errors, errorCode(e)) }),
      );
  }, [path, errors]);

  const draft = (): SkillPolicy => ({
    range: range.trim() || null,
    prerelease: prerelease === "" ? null : prerelease === "true",
    exclude: excludeSet ? exclude.split("\n").filter(Boolean) : null,
  });

  const key = `${range}|${prerelease}|${exclude}|${excludeSet}`;
  useEffect(() => {
    if (!preview || !view) return;
    const timer = setTimeout(() => {
      preview(draft())
        .then(setShown)
        .catch(() => setShown(null));
    }, 300);
    return () => clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, view]);

  const save = async () => {
    setBusy(true);
    setNote(null);
    try {
      apply(await apiSend<SkillPolicyView>("PUT", path, draft()));
      setNote({ kind: "success", text: t.settings.saved });
    } catch (e) {
      setNote({ kind: "error", text: errorText(errors, errorCode(e)) });
    } finally {
      setBusy(false);
    }
  };

  const inherited = (
    field: "range" | "prerelease" | "exclude",
    value: string,
  ) =>
    view
      ? format(p.inherited, {
          value: value || p.none,
          from: p.from[view.from[field]],
        })
      : "";
  const disabled = !view?.can_write;
  return (
    <Panel title={p.title} description={lead ?? p.lead}>
      <div className="grid gap-4">
        <Field
          label={p.range}
          hint={`${p.rangeHint}. ${inherited("range", view?.effective.range ?? "")}`}
        >
          {(f) => (
            <Input
              {...f}
              className="font-mono"
              value={range}
              disabled={disabled}
              placeholder={p.inherit}
              onChange={(e) => setRange(e.target.value)}
            />
          )}
        </Field>
        <Field
          label={p.prerelease}
          hint={inherited(
            "prerelease",
            view ? (view.effective.prerelease ? p.yes : p.no) : "",
          )}
        >
          {(f) => (
            <Select
              id={f.id}
              value={prerelease}
              disabled={disabled}
              onChange={(e) => setPrerelease(e.target.value)}
            >
              <option value="">{p.inherit}</option>
              <option value="true">{p.yes}</option>
              <option value="false">{p.no}</option>
            </Select>
          )}
        </Field>
        <Field
          label={p.exclude}
          hint={inherited(
            "exclude",
            (view?.effective.exclude ?? []).join(", "),
          )}
        >
          {(f) => (
            <TagInput
              id={f.id}
              value={exclude}
              disabled={disabled}
              placeholder={p.inherit}
              onChange={(v) => {
                setExclude(v);
                setExcludeSet(v !== "" || excludeSet);
              }}
              suggest={async (d) =>
                versions
                  .filter((v) => v.startsWith(d))
                  .map((v) => ({ value: v, label: v }))
              }
            />
          )}
        </Field>
        {!preview && <p className="text-xs text-muted">{p.overrideHint}</p>}
        {preview && shown && (
          <p className="text-sm">
            <span className="text-muted">{p.preview}: </span>
            <span className="font-mono">
              {shown
                .filter((v) => v.available)
                .map((v) => v.version)
                .join(", ") || p.none}
            </span>
          </p>
        )}
        <Message note={note} />
        {!disabled && (
          <div className="flex gap-2">
            <Button variant="primary" busy={busy} onClick={save}>
              {p.save}
            </Button>
            <Button
              variant="ghost"
              onClick={() => {
                setRange("");
                setPrerelease("");
                setExclude("");
                setExcludeSet(false);
              }}
            >
              {p.inherit}
            </Button>
          </div>
        )}
      </div>
    </Panel>
  );
}
