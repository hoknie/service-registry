"use client";

import { Plus, Trash2 } from "lucide-react";
import type { ReactNode } from "react";

import { format } from "@/i18n/format";
import type { NodeKnowledgeSettings } from "@/lib/api";
import { addExample, duplicateRows, REF_EXAMPLES, withKind, type RefKind, type RefPattern } from "@/lib/refs";
import { validPattern } from "@/lib/tags";

import { useUiText } from "../UiText";
import { Badge } from "../ui/Badge";
import { Button } from "../ui/Button";
import { Checkbox, Input } from "../ui/Field";
import { Select } from "../ui/Select";
import type { CatalogLabels } from "../catalog/shared";

export type RefsDraft = { set: boolean; rows: RefPattern[] };

type Props = {
  labels: CatalogLabels["docs"]["settings"];
  settings: NodeKnowledgeSettings;
  value: RefsDraft;
  canWrite: boolean;
  origin: ReactNode;
  onChange: (v: RefsDraft) => void;
};

export function RefsField({ labels: t, settings, value, canWrite, origin, onChange }: Props) {
  const { common } = useUiText();
  const toggle = () => onChange(value.set ? { set: false, rows: [] } : { set: true, rows: settings.refs.map((r) => ({ ...r })) });
  const rows = value.rows;
  const duplicates = duplicateRows(rows);
  const update = (i: number, row: RefPattern) => onChange({ set: true, rows: rows.map((r, j) => (j === i ? row : r)) });
  const kindLabel = (k: RefKind) => (k === "tag" ? t.refsTags : t.refsBranches);
  const hint = format(t.refsHint, { n: String(settings.max_tags) });
  return (
    <section className="grid gap-2" aria-label={t.refs}>
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="text-sm font-medium text-ink">{t.refs}</h3>
        {origin}
        {canWrite && (
          <span className="ml-auto">
            <Button type="button" size="sm" variant="ghost" onClick={toggle}>
              {value.set ? t.reset : t.override}
            </Button>
          </span>
        )}
      </div>
      {canWrite && value.set ? (
        <div className="grid gap-2">
          {rows.map((row, i) => {
            const bad = row.pattern.trim() !== "" && !validPattern(row.pattern.trim());
            return (
              <div key={i} className="grid gap-1">
                <div className="flex flex-wrap items-center gap-2">
                  <Input
                    aria-label={t.refsPattern}
                    aria-invalid={bad || duplicates.has(i)}
                    className="min-w-40 flex-1 font-mono"
                    value={row.pattern}
                    onChange={(e) => update(i, { ...row, pattern: e.target.value })}
                  />
                  <Select aria-label={t.refsKind} value={row.kind} onChange={(e) => update(i, withKind(row, e.target.value as RefKind))} className="w-32">
                    <option value="branch">{t.refsBranches}</option>
                    <option value="tag">{t.refsTags}</option>
                  </Select>
                  <Checkbox
                    label={t.refsRelease}
                    checked={row.release}
                    disabled={row.kind !== "tag"}
                    onChange={(e) => update(i, { ...row, release: e.target.checked })}
                  />
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    aria-label={t.refsRemove}
                    onClick={() => onChange({ set: true, rows: rows.filter((_, j) => j !== i) })}
                  >
                    <Trash2 aria-hidden="true" />
                  </Button>
                </div>
                {bad && <p className="text-xs text-danger">{common.invalidPattern}</p>}
                {duplicates.has(i) && <p className="text-xs text-danger">{t.refsDuplicate}</p>}
              </div>
            );
          })}
          <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted">
            <Button type="button" size="sm" onClick={() => onChange({ set: true, rows: [...rows, { pattern: "", kind: "branch", release: false }] })}>
              <Plus aria-hidden="true" />
              {t.refsAdd}
            </Button>
            <span>{t.examples}</span>
            {REF_EXAMPLES.map((x) => (
              <Button key={`${x.kind}:${x.pattern}`} type="button" size="sm" className="font-mono" onClick={() => onChange({ set: true, rows: addExample(rows, x) })}>
                {x.pattern} · {kindLabel(x.kind)}
                {x.release ? ` · ${t.refsRelease}` : ""}
              </Button>
            ))}
          </div>
        </div>
      ) : settings.refs.length ? (
        <ul className="grid gap-1 rounded-md bg-surface-2 px-3 py-2 text-xs text-ink-2">
          {settings.refs.map((r) => (
            <li key={`${r.kind}:${r.pattern}`} className="flex flex-wrap items-center gap-2">
              <span className="font-mono">{r.pattern}</span>
              <span>— {kindLabel(r.kind)}</span>
              {r.release && <Badge tone="signal">{t.refsRelease}</Badge>}
            </li>
          ))}
        </ul>
      ) : (
        <p className="text-sm text-ink-2">{t.refsDefaultOnly}</p>
      )}
      <p className="text-xs text-muted">{hint}</p>
      {settings.max_tags === 0 && <p className="text-xs text-muted">{t.refsNoTags}</p>}
    </section>
  );
}
