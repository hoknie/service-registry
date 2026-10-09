import { CircleAlert, CircleCheck } from "lucide-react";

import { cn } from "@/lib/cn";

export type Note = { kind: "error" | "success"; text: string } | null;

export function Message({ note, className }: { note: Note; className?: string }) {
  if (!note) return null;
  const error = note.kind === "error";
  const Icon = error ? CircleAlert : CircleCheck;
  return (
    <p
      role={error ? "alert" : "status"}
      className={cn(
        "flex items-start gap-2 rounded-md px-3 py-2 text-sm",
        error ? "bg-danger-soft text-danger" : "bg-signal-soft text-signal",
        className,
      )}
    >
      <Icon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <span>{note.text}</span>
    </p>
  );
}
