import { Slot } from "radix-ui";
import type { ButtonHTMLAttributes } from "react";

import { cn } from "@/lib/cn";

import { Spinner } from "./Spinner";

const variants = {
  primary: "bg-primary text-primary-ink hover:bg-primary-hover border border-transparent",
  secondary: "bg-surface text-ink border border-line-strong hover:bg-surface-2 hover:border-field",
  ghost: "bg-transparent text-ink-2 border border-transparent hover:bg-surface-3 hover:text-ink",
  danger: "bg-danger text-white border border-transparent hover:brightness-110 dark:text-canvas",
  "danger-ghost": "bg-transparent text-danger border border-transparent hover:bg-danger-soft",
} as const;

const sizes = {
  sm: "h-8 px-2.5 text-sm gap-1.5",
  md: "h-9 px-3.5 text-sm gap-2",
  icon: "h-8 w-8 justify-center p-0",
} as const;

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: keyof typeof variants;
  size?: keyof typeof sizes;
  asChild?: boolean;
  busy?: boolean;
};

export function Button({ variant = "secondary", size = "md", asChild, busy, className, type, disabled, children, ...rest }: ButtonProps) {
  const Comp = asChild ? Slot.Root : "button";
  return (
    <Comp
      type={asChild ? undefined : (type ?? "button")}
      disabled={asChild ? undefined : disabled || busy}
      aria-busy={busy || undefined}
      className={cn(
        "motion-control inline-flex shrink-0 items-center rounded-md font-medium whitespace-nowrap select-none active:scale-[0.98]",
        "disabled:pointer-events-none disabled:opacity-50 [&_svg]:size-4 [&_svg]:shrink-0",
        variants[variant],
        sizes[size],
        className,
      )}
      {...rest}
    >
      {asChild ? (
        children
      ) : (
        <>
          {busy && <Spinner />}
          {children}
        </>
      )}
    </Comp>
  );
}
