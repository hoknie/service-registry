import { cn } from "@/lib/cn";

export function Spinner({ className, label }: { className?: string; label?: string }) {
  return (
    <span role={label ? "status" : undefined} className={cn("inline-block size-4 shrink-0", className)}>
      <span aria-hidden="true" className="block size-full animate-spin rounded-full border-2 border-current border-r-transparent opacity-80" />
      {label && <span className="sr-only">{label}</span>}
    </span>
  );
}
