"use client";

import type { ReactNode } from "react";

import type { NodeKnowledgeSettings } from "@/lib/api";
import { cn } from "@/lib/cn";

import { Button } from "../ui/Button";
import type { CatalogLabels } from "../catalog/shared";
import { API_PATH_EXAMPLES, PatternField } from "./PatternField";

export type ApiPathsDraft = { set: boolean; text: string };

type Props = {
  labels: CatalogLabels["docs"]["settings"];
  settings: NodeKnowledgeSettings;
  value: ApiPathsDraft;
  canWrite: boolean;
  origin: ReactNode;
  apisOn: boolean;
  onChange: (v: ApiPathsDraft) => void;
};

export function ApiPathsField({ labels: t, settings, value, canWrite, origin, apisOn, onChange }: Props) {
  const toggle = () => onChange(value.set ? { set: false, text: "" } : { set: true, text: settings.api_paths.join("\n") });
  return (
    <div className={cn("grid gap-2", !apisOn && "opacity-60")}>
      <div className="flex flex-wrap items-center gap-2">
        <h4 className="text-sm text-ink">{t.apiPaths}</h4>
        {origin}
        {canWrite && (
          <span className="ml-auto">
            <Button type="button" size="sm" variant="ghost" onClick={toggle}>
              {value.set ? t.reset : t.override}
            </Button>
          </span>
        )}
      </div>
      {!apisOn && <p className="text-xs text-muted">{t.apiPathsInactive}</p>}
      {canWrite && value.set ? (
        <PatternField
          label={t.apiPaths}
          hint={value.text.trim() ? t.hint : `${t.apiPathsEmpty}. ${t.hint}`}
          value={value.text}
          onChange={(text) => onChange({ set: true, text })}
          examples={API_PATH_EXAMPLES}
          examplesLabel={t.examples}
          extra={
            <Button type="button" size="sm" onClick={() => onChange({ set: true, text: settings.defaults.api_paths.join("\n") })}>
              {t.apiPathsDefault}
            </Button>
          }
        />
      ) : (
        <pre className="overflow-x-auto rounded-md bg-surface-2 px-3 py-2 font-mono text-xs whitespace-pre-wrap text-ink-2">
          {settings.api_paths.length ? settings.api_paths.join("\n") : t.apiPathsEmpty}
        </pre>
      )}
    </div>
  );
}
