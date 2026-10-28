"use client";

import { useId, type ReactNode } from "react";

import type { ChildrenView } from "@/lib/childrenView";

import { Badge } from "../ui/Badge";
import { Segmented } from "../ui/Segmented";

export type ViewLabels = { view: string; cards: string; table: string; tree: string };

type Props = {
  title: string;
  count: ReactNode;
  view: ChildrenView;
  onView: (view: ChildrenView) => void;
  labels: ViewLabels;
  controls: boolean;
  filter?: ReactNode;
  children: ReactNode;
};

export function ChildrenSection({ title, count, view, onView, labels, controls, filter, children }: Props) {
  const id = useId();
  return (
    <section aria-labelledby={id} className="grid gap-4">
      <div className="flex flex-wrap items-center gap-3">
        <h2 id={id} className="flex items-center gap-2">
          {title}
          <Badge>{count}</Badge>
        </h2>
        {controls && (
          <div className="ml-auto flex flex-wrap items-center gap-2">
            {filter}
            <Segmented
              label={labels.view}
              value={view}
              onChange={(v) => onView(v === "table" || v === "tree" ? v : "cards")}
              options={[
                { value: "cards", label: labels.cards },
                { value: "table", label: labels.table },
                { value: "tree", label: labels.tree },
              ]}
            />
          </div>
        )}
      </div>
      {children}
    </section>
  );
}
