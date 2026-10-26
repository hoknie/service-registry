"use client";

import { versionKey } from "@/lib/refs";

import { Badge } from "../ui/Badge";
import { Field } from "../ui/Field";
import { Select } from "../ui/Select";

export type RefPickerLabels = { version: string; versionBranches: string; versionTags: string; currentRelease: string };

type Props = {
  branches: readonly { name: string }[];
  tags: readonly { name: string; release: boolean }[];
  value: string | null;
  labels: RefPickerLabels;
  onChange: (key: string) => void;
  className?: string;
};

export function RefPicker({ branches, tags, value, labels: t, onChange, className }: Props) {
  if (branches.length + tags.length < 2) return null;
  const grouped = branches.length > 0 && tags.length > 0;
  return (
    <Field label={t.version} className={className ?? "min-w-48"}>
      {(p) => (
        <Select {...p} value={value ?? ""} onChange={(e) => e.target.value && onChange(e.target.value)}>
          {grouped && (
            <option value="__branches" disabled>
              {t.versionBranches}
            </option>
          )}
          {branches.map((b) => (
            <option key={b.name} value={b.name}>
              {b.name}
            </option>
          ))}
          {grouped && (
            <option value="__tags" disabled>
              {t.versionTags}
            </option>
          )}
          {tags.map((tag) => (
            <option key={tag.name} value={versionKey("tag", tag.name)}>
              <span className="inline-flex items-center gap-2">
                {tag.name}
                {tag.release && <Badge tone="signal">{t.currentRelease}</Badge>}
              </span>
            </option>
          ))}
        </Select>
      )}
    </Field>
  );
}
