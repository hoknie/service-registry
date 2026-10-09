"use client";

import { Collapsible as C } from "radix-ui";
import type { ComponentProps } from "react";

import { cn } from "@/lib/cn";

export const Collapsible = C.Root;
export const CollapsibleTrigger = C.Trigger;

export function CollapsibleContent({ className, ...rest }: ComponentProps<typeof C.Content>) {
  return <C.Content className={cn("motion-collapse", className)} {...rest} />;
}
