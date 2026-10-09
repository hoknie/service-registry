"use client";

import { Button } from "../ui/Button";
import { validPattern } from "@/lib/tags";

import { useUiText } from "../UiText";
import { Field } from "../ui/Field";
import { TagInput } from "../ui/TagInput";

export const INCLUDE_EXAMPLES = ["*.md", "docs/**", "openspec/**/*.md", "**/*.md"] as const;

export function patternLines(text: string): string[] {
  return text
    .split("\n")
    .map((l) => l.trim())
    .filter(Boolean);
}

export function addPattern(text: string, pattern: string): string {
  const lines = patternLines(text);
  if (lines.includes(pattern)) return text;
  return [...lines, pattern].join("\n");
}

export function patternError(tag: string, message: string): string | null {
  return validPattern(tag) ? null : message;
}

export function PatternField({
  label,
  hint,
  value,
  onChange,
  examples,
  examplesLabel,
}: {
  label: string;
  hint: string;
  value: string;
  onChange: (v: string) => void;
  examples?: readonly string[];
  examplesLabel?: string;
}) {
  const { common } = useUiText();
  return (
    <div className="grid gap-2">
      <Field label={label} hint={hint}>
        {(p) => <TagInput {...p} value={value} onChange={onChange} validate={(tag) => patternError(tag, common.invalidPattern)} />}
      </Field>
      {examples && (
        <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted">
          <span>{examplesLabel}</span>
          {examples.map((x) => (
            <Button key={x} type="button" size="sm" className="font-mono" onClick={() => onChange(addPattern(value, x))}>
              {x}
            </Button>
          ))}
        </div>
      )}
    </div>
  );
}
