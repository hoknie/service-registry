"use client";

import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { errorText } from "@/i18n/errors";
import { format } from "@/i18n/format";
import type { Messages } from "@/i18n/messages";
import { errorCode } from "@/lib/api";
import { parseTreeLink } from "@/lib/forgeAddress";
import type { SkillFolder } from "@/lib/skills";
import { importSource } from "@/lib/skillsApi";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";
import { bodyOf, emptyDraft, SourceFields, type Draft } from "./SourceForm";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  parent: string | null;
  space: string | null;
  secretsOn: string | null;
  locale: Locale;
  labels: Messages["skills"];
  onImported: (folder: SkillFolder) => void;
};

function repoName(d: Draft): string {
  const raw = d.kind === "remote" ? (parseTreeLink(d.url)?.url ?? d.url) : d.path;
  return raw.replace(/\/+$/, "").split("/").pop()?.replace(/\.git$/, "") ?? "";
}

export function ImportDialog({ open, onOpenChange, parent, space, secretsOn, locale, labels: t, onImported }: Props) {
  const i = t.importDialog;
  const { errors } = useUiText();
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(emptyDraft);
    setName("");
    setError(null);
  }, [open]);

  const organization = parent ? null : space && space !== "global" ? space : null;
  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const out = await importSource({ ...bodyOf(draft), parent_id: parent, organization_id: organization, name: name.trim() });
      onImported(out.folder);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={i.title}
      size="lg"
      footer={
        <Button variant="primary" busy={busy} onClick={submit}>
          {i.submit}
        </Button>
      }
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label={i.name} hint={format(i.nameHint, { name: repoName(draft) || "—" })} className="sm:col-span-2">
          {(f) => <Input {...f} value={name} onChange={(e) => setName(e.target.value)} placeholder={repoName(draft)} />}
        </Field>
        <SourceFields draft={draft} setDraft={setDraft} folder={{ organization_id: secretsOn }} locale={locale} labels={t} />
        <Message note={error ? { kind: "error", text: error } : null} className="sm:col-span-2" />
      </div>
    </Dialog>
  );
}
