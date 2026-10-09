import { Box, Building, Folder } from "lucide-react";

import { cn } from "@/lib/cn";
import type { NodeKind } from "@/lib/api";

const icons = { organization: Building, folder: Folder, project: Box } as const;
const tones = { organization: "text-kind-org", folder: "text-kind-folder", project: "text-kind-project" } as const;
const tiles = {
  organization: "bg-kind-org-soft text-kind-org ring-kind-org/20",
  folder: "bg-kind-folder-soft text-kind-folder ring-kind-folder/20",
  project: "bg-kind-project-soft text-kind-project ring-kind-project/20",
} as const;

export function KindIcon({ kind, className, tile }: { kind: NodeKind; className?: string; tile?: boolean }) {
  const Icon = icons[kind];
  if (tile) {
    return (
      <span aria-hidden="true" className={cn("grid size-10 shrink-0 place-items-center rounded-xl ring-1 ring-inset", tiles[kind], className)}>
        <Icon className="size-5" />
      </span>
    );
  }
  return <Icon aria-hidden="true" className={cn("size-4 shrink-0", tones[kind], className)} />;
}
