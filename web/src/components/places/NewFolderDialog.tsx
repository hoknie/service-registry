"use client";

import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import { apiSend, errorCode } from "@/lib/api";
import { slugOf } from "@/lib/slug";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Dialog } from "../ui/Dialog";
import { Field, Input } from "../ui/Field";
import { Message } from "../ui/Message";

export type NewFolderLabels = { createTitle: string; name: string; slug: string; slugHint: string; create: string };

type Props<F> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  base: "/v1/skills" | "/v1/mcp";
  parent: string | null;
  space: string | null;
  labels: NewFolderLabels;
  onSaved: (folder: F) => void;
};

export function NewFolderDialog<F>({ open, onOpenChange, base, parent, space, labels: t, onSaved }: Props<F>) {
  const { errors } = useUiText();
  const [slug, setSlug] = useState("");
  const [slugTouched, setSlugTouched] = useState(false);
  const [name, setName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSlug("");
    setSlugTouched(false);
    setName("");
    setError(null);
  }, [open]);

  const submit = async () => {
    setBusy(true);
    setError(null);
    try {
      const saved = await apiSend<F>("POST", `${base}/folders`, {
        slug,
        name,
        parent_id: parent,
        organization_id: parent || space === "global" ? null : space,
      });
      onSaved(saved);
      onOpenChange(false);
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
      title={t.createTitle}
      footer={
        <Button variant="primary" busy={busy} onClick={submit} disabled={!slug || !name}>
          {t.create}
        </Button>
      }
    >
      <div className="grid gap-4">
        <Field label={t.name}>
          {(p) => (
            <Input
              {...p}
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                if (!slugTouched) setSlug(slugOf(e.target.value));
              }}
            />
          )}
        </Field>
        <Field label={t.slug} hint={t.slugHint}>
          {(p) => (
            <Input
              {...p}
              className="font-mono"
              value={slug}
              onChange={(e) => {
                setSlugTouched(true);
                setSlug(e.target.value.toLowerCase());
              }}
            />
          )}
        </Field>
        <Message note={error ? { kind: "error", text: error } : null} />
      </div>
    </Dialog>
  );
}
