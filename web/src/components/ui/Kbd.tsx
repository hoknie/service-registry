import { cn } from "@/lib/cn";

export function Kbd({ children, className }: { children: string; className?: string }) {
  return (
    <kbd className={cn("rounded border border-line-strong bg-surface-2 px-1.5 font-mono text-[0.7rem] leading-5 text-muted", className)}>
      {children}
    </kbd>
  );
}
