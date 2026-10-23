"use client";

import { TriangleAlert } from "lucide-react";

import { format } from "@/i18n/format";
import type { Snapshot } from "@/lib/api";
import { cn } from "@/lib/cn";
import { snapshotMissing } from "@/lib/snapshotLimits";

import { useSession } from "../SessionProvider";
import { useUiText } from "../UiText";

type Props = { snapshot: Snapshot; canWrite: boolean; settingsHref?: string; className?: string };

export function LimitNotice({ snapshot, canWrite, settingsHref, className }: Props) {
  const { common } = useUiText();
  const { session } = useSession();
  const t = common.docLimits;
  const missing = snapshotMissing(snapshot);
  if (missing === 0) return null;
  const reached = snapshot.limits ?? [];
  const admin = session.user?.is_superadmin === true;
  const detailed = admin || canWrite || reached.length > 0;
  return (
    <div role="status" className={cn("flex items-start gap-2 rounded-md border border-line bg-amber-soft px-3 py-2 text-sm text-ink", className)}>
      <TriangleAlert aria-hidden="true" className="mt-0.5 size-4 shrink-0 text-amber" />
      <div className="grid gap-1">
        <p className="font-medium">
          {t.title}
          {detailed ? `: ${format(t.count, { n: missing })}` : ""}
        </p>
        {reached.map((l) => (
          <p key={l.name}>{format(t.reached, { name: l.name, value: l.value })}</p>
        ))}
        {admin ? (
          <p className="text-ink-2">{t.admin}</p>
        ) : canWrite || reached.length > 0 ? (
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
