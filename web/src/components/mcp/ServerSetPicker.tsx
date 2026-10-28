"use client";

import { useEffect, useState } from "react";

import { apiGet } from "@/lib/api";
import { MCP_BASE, type McpServerPage, type McpServerRef } from "@/lib/mcp";

import { Checkbox, Input } from "../ui/Field";
import { Segmented } from "../ui/Segmented";

export type PickerLabels = {
  title: string;
  all: string;
  selected: string;
  search: string;
  empty: string;
  none: string;
  hint: string;
};

type Props = {
  value: string[] | null;
  onChange: (v: string[] | null) => void;
  labels: PickerLabels;
  servers?: McpServerRef[];
  disabled?: boolean;
};

export function ServerSetPicker({
  value,
  onChange,
  labels: t,
  servers,
  disabled,
}: Props) {
  const [loaded, setLoaded] = useState<McpServerRef[] | null>(servers ?? null);
  const [q, setQ] = useState("");
  useEffect(() => {
    if (servers) return;
    apiGet<McpServerPage>(`${MCP_BASE}/servers?limit=200`)
      .then((p) =>
        setLoaded(
          p.items
            .filter((s) => s.status !== "disabled")
            .map((s) => ({ id: s.id, slug: s.slug, title: s.title })),
        ),
      )
      .catch(() => setLoaded([]));
  }, [servers]);
  const list = (servers ?? loaded ?? []).filter((s) =>
    `${s.slug} ${s.title}`.toLowerCase().includes(q.trim().toLowerCase()),
  );
  return (
    <fieldset className="grid gap-2" disabled={disabled}>
      <legend className="mb-1 text-sm font-medium text-ink">{t.title}</legend>
      <Segmented
        label={t.title}
        value={value === null ? "all" : "selected"}
        options={[
          { value: "all", label: t.all },
          { value: "selected", label: t.selected },
        ]}
        onChange={(v) => onChange(v === "all" ? null : (value ?? []))}
      />
      {value !== null && (
        <div className="grid gap-2 rounded-lg border border-line p-3">
          <Input
            type="search"
            placeholder={t.search}
            aria-label={t.search}
            value={q}
            onChange={(e) => setQ(e.target.value)}
          />
          <ul className="grid max-h-56 gap-1 overflow-y-auto">
            {list.map((s) => (
              <li key={s.id}>
                <Checkbox
                  label={
                    <span>
                      {s.title}{" "}
                      <span className="font-mono text-xs text-muted">
                        {s.slug}
                      </span>
                    </span>
                  }
                  checked={value.includes(s.id)}
                  onChange={(e) =>
                    onChange(
                      e.target.checked
                        ? [...value, s.id]
                        : value.filter((x) => x !== s.id),
                    )
                  }
                />
              </li>
            ))}
            {list.length === 0 && (
              <li className="text-sm text-muted">{t.empty}</li>
            )}
          </ul>
          <p className="text-xs text-muted">
            {value.length === 0 ? t.none : t.hint}
          </p>
        </div>
      )}
    </fieldset>
  );
}
