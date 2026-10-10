"use client";

import { Plus } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import { apiGet, errorCode, type Items, type Secret } from "@/lib/api";
import { NO_SECRET, secretOptions, secretsPath } from "@/lib/secrets";

import { useUiText } from "../UiText";
import { Button } from "../ui/Button";
import { Combobox } from "../ui/Combobox";
import { Field } from "../ui/Field";
import { SecretDialog } from "./SecretDialog";

type Props = {
  listOn: string | null;
  createOn: string | null | false;
  value: string | null;
  onChange: (id: string | null) => void;
  manageHref: string;
  allowNone?: boolean;
  legacy?: boolean;
  required?: boolean;
};

export function SecretPicker({ listOn, createOn, value, onChange, manageHref, allowNone, legacy, required }: Props) {
  const { secrets: t, errors } = useUiText();
  const [items, setItems] = useState<Secret[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async () => {
    try {
      setItems((await apiGet<Items<Secret>>(secretsPath(listOn))).items);
      setError(null);
    } catch (e) {
      setError(errorText(errors, errorCode(e)));
    }
  }, [listOn, errors]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  const options = secretOptions(items ?? [], t, allowNone ?? false);
  return (
    <Field label={t.picker} error={error ?? undefined} hint={legacy && !value ? t.legacy : items && items.length === 0 ? t.noneAvailable : undefined}>
      {(p) => (
        <div className="grid gap-2">
          <Combobox
            {...p}
            value={value ?? (allowNone && !legacy ? NO_SECRET : "")}
            options={options}
            placeholder={t.pickerPlaceholder}
            required={required}
            onChange={(v) => onChange(v === NO_SECRET ? null : v)}
          />
          <div className="flex flex-wrap items-center gap-3 text-sm">
            <Link href={manageHref} className="text-signal hover:underline">
              {t.manage}
            </Link>
            {createOn !== false && (
              <Button type="button" size="sm" variant="ghost" onClick={() => setCreating(true)}>
                <Plus aria-hidden="true" />
                {t.create}
              </Button>
            )}
          </div>
          {creating && createOn !== false && (
            <SecretDialog
              nodeId={createOn}
              secret={null}
              onClose={() => setCreating(false)}
              onSaved={(s) => {
                setCreating(false);
                void load();
                onChange(s.id);
              }}
            />
          )}
        </div>
      )}
    </Field>
  );
}
