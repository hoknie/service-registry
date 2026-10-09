"use client";

import { useEffect, useState } from "react";

import { errorText } from "@/i18n/errors";
import type { Messages } from "@/i18n/messages";
import { ApiError, apiGet } from "@/lib/api";
import { cn } from "@/lib/cn";

import { useUiText } from "./UiText";

type Labels = Messages["home"]["apiStatus"];
type State = { kind: "checking" | "ok" } | { kind: "unavailable"; code: string };

const lamp = { checking: "bg-muted animate-shimmer", ok: "bg-signal", unavailable: "bg-danger" } as const;

export function ApiStatus({ labels }: { labels: Labels }) {
  const { errors } = useUiText();
  const [state, setState] = useState<State>({ kind: "checking" });

  useEffect(() => {
    let alive = true;
    apiGet<{ status: string }>("/health")
      .then((r) => {
        if (alive) setState(r.status === "ok" ? { kind: "ok" } : { kind: "unavailable", code: "unknown" });
      })
      .catch((e: unknown) => {
        if (alive) setState({ kind: "unavailable", code: e instanceof ApiError ? e.code : "unknown" });
      });
    return () => {
      alive = false;
    };
  }, []);

  return (
    <div role="status" data-state={state.kind} className="flex h-full min-h-32 flex-col justify-between gap-3 rounded-card border border-line bg-surface p-4 shadow-elev-1">
      <p className="text-sm text-muted">{labels.label}</p>
      <div>
        <p className="flex items-center gap-2.5 text-xl font-semibold text-ink">
          <span aria-hidden="true" className={cn("relative size-2.5 rounded-full", lamp[state.kind])}>
            {state.kind === "ok" && <span className="absolute inset-0 animate-ping rounded-full bg-signal opacity-40 motion-reduce:hidden" />}
          </span>
          {labels[state.kind]}
        </p>
        {state.kind === "unavailable" && <p className="mt-1 text-sm text-danger">{errorText(errors, state.code)}</p>}
      </div>
    </div>
  );
}
