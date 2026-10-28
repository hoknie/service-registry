"use client";

import { useState } from "react";

import { Markdown } from "../catalog/Markdown";
import { useUiText } from "../UiText";
import { Textarea } from "../ui/Field";
import { Segmented } from "../ui/Segmented";

type Props = {
  label: string;
  value: string;
  onChange: (value: string) => void;
  readOnly?: boolean;
  hint?: string;
  rows?: number;
  maxLength?: number;
};

export function MarkdownField({ label, value, onChange, readOnly, hint, rows = 6, maxLength }: Props) {
  const p = useUiText().common.places;
  const [preview, setPreview] = useState(false);
  return (
    <div className="grid gap-1.5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm font-medium text-ink-2">{label}</span>
        <Segmented
          label={label}
          value={preview ? "preview" : "text"}
          onChange={(v) => setPreview(v === "preview")}
          options={[
            { value: "text", label: p.markdownText },
            { value: "preview", label: p.markdownPreview },
          ]}
        />
      </div>
      {preview ? (
        <div className="min-h-24 rounded-md border border-line bg-surface p-3">
          {value.trim() ? <Markdown source={value} /> : <p className="text-sm text-muted">{p.markdownEmpty}</p>}
        </div>
      ) : (
        <>
          <Textarea aria-label={label} rows={rows} maxLength={maxLength} readOnly={readOnly} value={value} onChange={(e) => onChange(e.target.value)} />
          <p className="text-xs text-muted">{hint ?? p.markdownHint}</p>
        </>
      )}
    </div>
  );
}
