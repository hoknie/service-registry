"use client";

import { Tabs as T } from "radix-ui";
import type { ComponentProps } from "react";

import { cn } from "@/lib/cn";

export const Tabs = T.Root;

export function TabsContent({ className, ...rest }: ComponentProps<typeof T.Content>) {
  return <T.Content className={cn("motion-tab focus-visible:outline-none", className)} {...rest} />;
}

export function TabsList({ className, ...rest }: ComponentProps<typeof T.List>) {
  return (
    <T.List
      className={cn(
        "mb-6 flex max-w-full gap-1 overflow-x-auto rounded-xl border border-line bg-surface-2/80 p-1 shadow-elev-1 backdrop-blur [scrollbar-width:none]",
        className,
      )}
      {...rest}
    />
  );
}

export function TabsTrigger({ className, ...rest }: ComponentProps<typeof T.Trigger>) {
  return (
    <T.Trigger
      className={cn(
        "motion-control relative inline-flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium whitespace-nowrap text-muted",
        "hover:bg-surface hover:text-ink data-[state=active]:bg-surface data-[state=active]:text-ink data-[state=active]:shadow-elev-2",
        "[&_svg]:size-4 data-[state=active]:[&_svg]:text-signal",
        className,
      )}
      {...rest}
    />
  );
}
