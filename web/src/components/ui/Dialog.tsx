"use client";

import { X } from "lucide-react";
import { Dialog as D } from "radix-ui";
import type { ReactNode } from "react";

import { cn } from "@/lib/cn";

import { useUiText } from "../UiText";

type Props = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: ReactNode;
  description?: ReactNode;
  children?: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg";
  persistent?: boolean;
};

const widths = { sm: "max-w-md", md: "max-w-lg", lg: "max-w-2xl" } as const;

export function Dialog({ open, onOpenChange, title, description, children, footer, size = "md", persistent }: Props) {
  const { common } = useUiText();
  const block = (e: Event) => persistent && e.preventDefault();
  return (
    <D.Root open={open} onOpenChange={onOpenChange}>
      <D.Portal>
        <D.Overlay className="motion-overlay fixed inset-0 z-50 bg-overlay backdrop-blur-[2px]" />
        <D.Content
          onEscapeKeyDown={block}
          onPointerDownOutside={block}
          onInteractOutside={block}
          className={cn(
            "fixed top-1/2 left-1/2 z-50 flex max-h-[calc(100dvh-2rem)] w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 flex-col",
            "motion-dialog rounded-xl border border-line bg-surface shadow-pop focus:outline-none",
            widths[size],
          )}
        >
          <div className="flex items-start justify-between gap-4 px-5 pt-5">
            <div className="min-w-0">
              <D.Title className="text-lg font-semibold text-ink">{title}</D.Title>
              {description ? (
                <D.Description className="mt-1 text-sm text-muted">{description}</D.Description>
              ) : (
                <D.Description className="sr-only">{title}</D.Description>
              )}
            </div>
            {!persistent && (
              <D.Close
                className="motion-control -mt-1 -mr-2 grid size-8 shrink-0 place-items-center rounded-md text-muted hover:bg-surface-3 hover:text-ink"
                aria-label={common.close}
              >
                <X aria-hidden="true" className="size-4" />
              </D.Close>
            )}
          </div>
          {children && <div className="min-h-0 overflow-y-auto px-5 py-4">{children}</div>}
          {footer && <div className="flex shrink-0 flex-wrap items-center justify-end gap-2 border-t border-line px-5 py-3">{footer}</div>}
        </D.Content>
      </D.Portal>
    </D.Root>
  );
}
