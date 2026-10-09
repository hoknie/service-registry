import type { ReactNode } from "react";

import type { NodeKind } from "@/lib/api";
import { cn } from "@/lib/cn";

const washes = {
  organization: "from-kind-org-soft",
  folder: "from-kind-folder-soft",
  project: "from-kind-project-soft",
} as const;
const auras = { organization: "bg-kind-org", folder: "bg-kind-folder", project: "bg-kind-project" } as const;

type Props = {
  kind: NodeKind;
  icon: ReactNode;
  title: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
  children?: ReactNode;
};

export function NodeHero({ kind, icon, title, meta, actions, children }: Props) {
  return (
    <header
      className={cn(
        "relative mb-6 animate-rise-in overflow-hidden rounded-card border border-line bg-gradient-to-br to-hero-to px-5 py-5 shadow-elev-1 sm:px-6",
        washes[kind],
      )}
    >
      <span aria-hidden="true" className={cn("pointer-events-none absolute -top-24 right-6 size-56 rounded-full opacity-[0.14] blur-3xl", auras[kind])} />
      <div className="relative flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 items-start gap-4">
          {icon}
          <div className="min-w-0">
            <h1 className="text-3xl leading-tight break-words">{title}</h1>
            {meta && <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-ink-2">{meta}</div>}
          </div>
        </div>
        {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
      </div>
      {children && <div className="relative mt-4">{children}</div>}
    </header>
  );
}
