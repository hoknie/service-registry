import { BellRing, BookOpenCheck, BookText, Bug, ChartLine, Link, ScrollText, Waypoints, type LucideIcon } from "lucide-react";

import type { LinkIcon as Icon } from "@/lib/api";
import { cn } from "@/lib/cn";

const icons: Record<Icon, LucideIcon> = {
  link: Link,
  logs: ScrollText,
  dashboard: ChartLine,
  errors: Bug,
  alerts: BellRing,
  traces: Waypoints,
  runbook: BookOpenCheck,
  docs: BookText,
};

export function LinkIcon({ icon, className }: { icon: string | undefined; className?: string }) {
  const Comp = icons[icon as Icon] ?? Link;
  return <Comp aria-hidden="true" className={cn("size-4 shrink-0 text-muted", className)} />;
}
