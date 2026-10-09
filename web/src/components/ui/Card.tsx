import Link from "next/link";
import type { HTMLAttributes, ReactNode } from "react";

import { cn } from "@/lib/cn";

const base = "group relative block rounded-card border border-line bg-surface p-4 shadow-elev-1 animate-rise-in";
const interactive =
  "motion-control hover:-translate-y-0.5 hover:border-line-strong hover:shadow-elev-2 focus-visible:-translate-y-0.5 focus-visible:shadow-elev-2";

export function Card({
  href,
  className,
  children,
  ...rest
}: HTMLAttributes<HTMLElement> & { href?: string; children: ReactNode }) {
  if (href) {
    return (
      <Link href={href} className={cn(base, interactive, className)} {...rest}>
        {children}
      </Link>
    );
  }
  return (
    <div className={cn(base, className)} {...rest}>
      {children}
    </div>
  );
}
