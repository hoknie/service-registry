import type { ReactNode } from "react";

import { Panel } from "../ui/Panel";

export type DangerRow = { key: string; text: ReactNode; action: ReactNode };

export function DangerZone({ title, rows }: { title: string; rows: DangerRow[] }) {
  if (rows.length === 0) return null;
  return (
    <Panel title={title}>
      <div className="grid gap-4">
        {rows.map((r) => (
          <div key={r.key} className="flex flex-wrap items-center justify-between gap-3">
            <p className="min-w-0 flex-1 text-sm text-ink-2">{r.text}</p>
            {r.action}
          </div>
        ))}
      </div>
    </Panel>
  );
}
