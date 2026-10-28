import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

export function PlaceBody({ pending, children }: { pending: boolean; children: ReactNode }) {
  return (
    <div aria-busy={pending || undefined} inert={pending} className={cn("motion-control", pending && "opacity-60 delay-150")}>
      {children}
    </div>
  );
}
