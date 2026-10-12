"use client";

import { cn } from "@/lib/cn";

import { SetIcon } from "../ui/SetIcon";

export function LinkIcon({ icon, className }: { icon: string | undefined | null; className?: string }) {
  return <SetIcon name={icon} className={cn("text-muted", className)} />;
}
