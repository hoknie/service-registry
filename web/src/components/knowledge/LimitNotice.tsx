"use client";

import { TriangleAlert } from "lucide-react";

import { format } from "@/i18n/format";
import type { Snapshot } from "@/lib/api";
import { cn } from "@/lib/cn";
import { snapshotMissing } from "@/lib/snapshotLimits";

import { useUiText } from "../UiText";

type Props = { snapshot: Snapshot; canWrite: boolean; settingsHref?: string; className?: string };

export function LimitNotice({ snapshot, canWrite, settingsHref, className }: Props) {
  const { common } = useUiText();
  const t = common.docLimits;
  const missing = snapshotMissing(snapshot);
  if (missing === 0) return null;
  const admin = snapshot.limits !== null && snapshot.limits !== undefined;
  return (
    <div role="status" className={cn("flex items-start gap-2 rounded-md border border-line bg-amber-soft px-3 py-2 text-sm text-ink", className)}>
      <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-amber" />
      <div className="grid gap-1">
        <p className="font-medium">
          {t.title}
          {admin || canWrite ? `: ${format(t.count, { n: missing })}` : ""}
        </p>
        {admin ? (
          <>
            {(snapshot.limits ?? []).map((l) => (
              <p key={l.name}>{format(t.reached, { name: l.name, value: l.value })}</p>
            ))}
            <p className="text-ink-2">{t.admin}</p>
          </>
        ) : canWrite ? (
          <p className="text-ink-2">
            {t.editor}
            {settingsHref && (
              <>
                {" "}
                <a href={settingsHref} className="font-medium text-signal hover:underline">
                  {t.editorLink}
                </a>
              </>
            )}
          </p>
        ) : (
          <p className="text-ink-2">{t.reader}</p>
        )}
      </div>
    </div>
  );
}
