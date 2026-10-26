"use client";

import { Plus, Table2 } from "lucide-react";
import { useEffect, useState } from "react";

import type { Locale } from "@/i18n/config";
import { format } from "@/i18n/format";
import { addTable, MAX_TABLES, METRIC_TABLES_KEY, readTables, removeTable, updateTable, type MetricTable as Stored } from "@/lib/metricTables";

import { Button } from "../ui/Button";
import { EmptyState } from "../ui/EmptyState";
import { Tooltip } from "../ui/Tooltip";
import { MetricTable } from "./MetricTable";
import { MetricsShell, type Live, type MetricsLabels } from "./MetricsShell";

export function MetricsAdmin({ locale, labels: t }: { locale: Locale; labels: MetricsLabels }) {
  return (
    <MetricsShell locale={locale} t={t} title={t.title} lead={t.lead} glyph={Table2}>
      {(live) => <Tables locale={locale} t={t} live={live} />}
    </MetricsShell>
  );
}

function load(): Stored[] {
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(METRIC_TABLES_KEY);
  } catch {
    raw = null;
  }
  return readTables(raw, () => crypto.randomUUID());
}

function save(tables: Stored[]) {
  try {
    window.localStorage.setItem(METRIC_TABLES_KEY, JSON.stringify(tables));
  } catch {
    return;
  }
}

function Tables({ locale, t, live }: { locale: Locale; t: MetricsLabels; live: Live }) {
  const [tables, setTables] = useState<Stored[] | null>(null);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTables(load());
  }, []);

  if (!tables) return null;
  const change = (next: Stored[]) => {
    setTables(next);
    save(next);
  };
  const full = tables.length >= MAX_TABLES;
  const add = (
    <Button disabled={full} onClick={() => change(addTable(tables, crypto.randomUUID()))}>
      <Plus aria-hidden="true" />
      {t.tables.add}
    </Button>
  );
  return (
    <div className="grid gap-8">
      {tables.length === 0 && <EmptyState icon={Table2} title={t.tables.empty} action={add} />}
      {tables.map((table) => (
        <MetricTable
          key={table.id}
          locale={locale}
          t={t}
          live={live}
          table={table}
          onChange={(patch) => change(updateTable(tables, table.id, patch))}
          onRemove={() => change(removeTable(tables, table.id))}
        />
      ))}
      {tables.length > 0 && (
        <div className="flex justify-center">
          {full ? (
            <Tooltip content={format(t.tables.limit, { n: MAX_TABLES })}>
              <span tabIndex={0}>{add}</span>
            </Tooltip>
          ) : (
            add
          )}
        </div>
      )}
    </div>
  );
}
