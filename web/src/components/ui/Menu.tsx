"use client";

import { Check } from "lucide-react";
import { DropdownMenu as M } from "radix-ui";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "@/lib/cn";

export const Menu = M.Root;
export const MenuTrigger = M.Trigger;

export function MenuContent({ className, align = "end", ...rest }: ComponentProps<typeof M.Content>) {
  return (
    <M.Portal>
      <M.Content
        align={align}
        sideOffset={6}
        collisionPadding={8}
        className={cn(
          "motion-pop z-50 min-w-48 rounded-lg border border-line bg-surface p-1 text-sm shadow-pop",
          className,
        )}
        {...rest}
      />
    </M.Portal>
  );
}

const item =
  "motion-control flex cursor-pointer items-center gap-2.5 rounded-md px-2.5 py-1.5 text-ink outline-none select-none " +
  "data-[highlighted]:bg-surface-3 data-[disabled]:pointer-events-none data-[disabled]:opacity-50 [&_svg]:size-4 [&_svg]:text-muted";

export function MenuItem({ className, danger, ...rest }: ComponentProps<typeof M.Item> & { danger?: boolean }) {
  return <M.Item className={cn(item, danger && "text-danger [&_svg]:text-danger", className)} {...rest} />;
}

export function MenuCheckItem({ checked, children, className, ...rest }: ComponentProps<typeof M.Item> & { checked: boolean }) {
  return (
    <M.Item className={cn(item, "pr-8", checked && "font-medium", className)} {...rest}>
      {children}
      {checked && <Check aria-hidden="true" className="ml-auto !text-signal" />}
    </M.Item>
  );
}

export function MenuLabel({ children }: { children: ReactNode }) {
  return <M.Label className="px-2.5 pt-1.5 pb-1 text-xs text-muted">{children}</M.Label>;
}

export function MenuSeparator() {
  return <M.Separator className="my-1 h-px bg-line" />;
}

export const MenuRadioGroup = M.RadioGroup;

export function MenuRadioItem({ children, className, ...rest }: ComponentProps<typeof M.RadioItem>) {
  return (
    <M.RadioItem className={cn(item, "pr-8 data-[state=checked]:font-medium", className)} {...rest}>
      {children}
      <M.ItemIndicator className="ml-auto">
        <Check aria-hidden="true" className="!text-signal" />
      </M.ItemIndicator>
    </M.RadioItem>
  );
}
